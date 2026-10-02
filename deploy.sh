#!/usr/bin/env bash
# =============================================================================
# deploy.sh - Deploy PASTI V3 ke VPS Ubuntu dengan Docker Compose.
#
# Jalankan dari folder repo di VPS:
#
#   ./deploy.sh                  # tarik kode terbaru dari branch yang sedang aktif, lalu deploy
#   ./deploy.sh production_v3    # pindah ke branch tertentu, lalu deploy
#   ./deploy.sh --help
#
# Yang dilakukan, berurutan:
#   1. Cek prasyarat (git, curl, Docker + Compose; menawarkan memasang Docker bila belum ada)
#   2. Siapkan konfigurasi: deploy.env (URL publik, port) dan backend/.env (rahasia & DB)
#   3. Tarik kode terbaru (git fetch + fast-forward)
#   4. Build image backend & frontend
#   5. Terapkan migrasi database yang belum diterapkan (dicatat di tabel schema_migrations)
#   6. Jalankan ulang container, lalu cek kesehatannya
#   7. Bila gagal, kembalikan ke image sebelumnya secara otomatis
#
# Catatan: SQL Server TIDAK dipasang oleh skrip ini; skrip hanya terhubung ke
# server database yang diisi di backend/.env.
# =============================================================================

set -Eeuo pipefail

ORIG_ARGS=("$@")
TARGET_BRANCH=""
NO_PULL=0
NO_MIGRATE=0
FORCE=0
FRESH=0
ASSUME_YES=0

HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-90}"   # detik menunggu tiap layanan sehat; bisa diubah: HEALTH_TIMEOUT=180 ./deploy.sh
LOCK_FILE="/tmp/pasti-deploy.lock"

SCRIPT_PATH="${BASH_SOURCE[0]}"
APP_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
SCRIPT_NAME="$(basename "$SCRIPT_PATH")"
DEPLOY_ENV="$APP_DIR/deploy.env"
BACKEND_ENV="$APP_DIR/backend/.env"
BACKEND_ENV_EXAMPLE="$APP_DIR/backend/.env.example"
LOG_FILE="$APP_DIR/deploy.log"

REPO_OWNER=""
OWNER_HOME=""
START_TS=$SECONDS
APT_UPDATED=0
ROLLBACK_IMAGES=()      # nama image yang punya salinan ":rollback"
PREV_COMMIT=""

# ----------------------------------------------------------------------------
# Output
# ----------------------------------------------------------------------------
step() { printf '\n==> %s\n' "$*"; }
log()  { printf '    %s\n' "$*"; }
ok()   { printf '    [OK]    %s\n' "$*"; }
warn() { printf '    [WARN]  %s\n' "$*" >&2; }
err()  { printf '    [GAGAL] %s\n' "$*" >&2; }
die()  { err "$*"; exit 1; }

usage() {
  cat <<'EOF'
Pemakaian: ./deploy.sh [branch] [opsi]

  branch          (opsional) pindah ke branch ini sebelum deploy. Tanpa ini, memakai
                  branch yang sedang aktif di server.

Opsi:
  --no-pull       jangan tarik kode; deploy kode yang ada di server apa adanya
  --no-migrate    lewati migrasi database
  --fresh         build tanpa cache dan tarik base image terbaru (lebih lambat)
  --force         buang perubahan lokal pada file yang dilacak git di server
                  (git reset --hard origin/<branch>). Hati-hati.
  -y, --yes       jawab "ya" otomatis pada konfirmasi (mis. memasang Docker)
  -h, --help      tampilkan bantuan ini

Konfigurasi (dibuat otomatis saat pertama kali dijalankan):
  deploy.env      URL publik backend dan port. Contoh isi:
                    NEXT_PUBLIC_API_ROOT_URL=http://203.0.113.10:8686
                    NEXT_PUBLIC_API_URL=http://203.0.113.10:8686/api/v1
                    BIND_ADDRESS=0.0.0.0     # 127.0.0.1 bila diakses lewat Nginx di server ini
                    BACKEND_PORT=8686
                    FRONTEND_PORT=3000
  backend/.env    rahasia aplikasi, koneksi database, SSO, token Inaproc (dibuat dari
                  backend/.env.example; Anda yang mengisi nilai database & SSO).

Log setiap deploy ditambahkan ke deploy.log.
EOF
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --no-pull)    NO_PULL=1 ;;
      --no-migrate) NO_MIGRATE=1 ;;
      --fresh)      FRESH=1 ;;
      --force)      FORCE=1 ;;
      -y|--yes)     ASSUME_YES=1 ;;
      -h|--help)    usage; exit 0 ;;
      -*)           die "Opsi tidak dikenal: $1 (lihat --help)" ;;
      *)
        [ -z "$TARGET_BRANCH" ] || die "Hanya boleh satu nama branch."
        TARGET_BRANCH="$1"
        ;;
    esac
    shift
  done
  if [ -n "$TARGET_BRANCH" ] && ! [[ "$TARGET_BRANCH" =~ ^[A-Za-z0-9][A-Za-z0-9._/-]*$ ]]; then
    die "Nama branch tidak valid: $TARGET_BRANCH"
  fi
}

# ----------------------------------------------------------------------------
# Pembantu umum
# ----------------------------------------------------------------------------

# confirm "pertanyaan" [y|n]  -> status 0 bila jawabannya ya. --yes menjawab ya.
confirm() {
  local prompt=$1 def=${2:-n} ans hint="[y/N]"
  [ "$def" = y ] && hint="[Y/n]"
  [ "$ASSUME_YES" -eq 1 ] && return 0
  # Tanpa terminal interaktif, hanya --yes yang dianggap persetujuan.
  if [ ! -t 0 ]; then return 1; fi
  read -r -p "    $prompt $hint " ans || ans=""
  ans=${ans:-$def}
  [[ "$ans" =~ ^[YyJj] ]]
}

# env_get FILE KEY -> nilai KEY (kosong bila tidak ada). Tidak ada ekspansi shell:
# aman untuk kata sandi yang memuat $, !, ` dan sebagainya.
env_get() {
  local file=$1 key=$2 line val
  [ -f "$file" ] || return 0
  line=$(grep -E "^[[:space:]]*${key}=" "$file" | tail -n 1 || true)
  [ -n "$line" ] || return 0
  val=${line#*=}
  val=${val%$'\r'}
  if [[ ${#val} -ge 2 && $val == \"*\" ]]; then val=${val:1:${#val}-2}
  elif [[ ${#val} -ge 2 && $val == \'*\' ]]; then val=${val:1:${#val}-2}
  fi
  printf '%s' "$val"
}

# env_set FILE KEY VALUE -> ganti baris KEY=... (atau tambahkan di akhir).
env_set() {
  local file=$1 key=$2 value=$3 tmp
  tmp=$(mktemp)
  if grep -qE "^[[:space:]]*${key}=" "$file" 2>/dev/null; then
    KEY="$key" VAL="$value" awk '
      BEGIN { done = 0 }
      { if (!done && $0 ~ ("^[ \t]*" ENVIRON["KEY"] "=")) { print ENVIRON["KEY"] "=" ENVIRON["VAL"]; done = 1 } else print }
    ' "$file" > "$tmp"
  else
    cat "$file" > "$tmp" 2>/dev/null || true
    if [ -s "$tmp" ] && [ -n "$(tail -c1 "$tmp")" ]; then printf '\n' >> "$tmp"; fi
    printf '%s=%s\n' "$key" "$value" >> "$tmp"
  fi
  cat "$tmp" > "$file"   # menimpa isi, mempertahankan izin & pemilik file
  rm -f "$tmp"
}

is_placeholder() {
  case "$1" in
    ganti-dengan*|YourStrongPassword123!|CHANGE_ME*|changeme*|isi-*) return 0 ;;
  esac
  return 1
}

gen_hex()    { openssl rand -hex "$1"; }
gen_key_b64() { openssl rand -base64 32; }

primary_ip() {
  local ip=""
  if command -v ip >/dev/null 2>&1; then
    ip=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i <= NF; i++) if ($i == "src") { print $(i + 1); exit }}')
  fi
  if [ -z "$ip" ] && command -v hostname >/dev/null 2>&1; then
    ip=$(hostname -I 2>/dev/null | awk '{print $1}')
  fi
  printf '%s' "${ip:-IP-VPS-ANDA}"
}

valid_url() { [[ $1 =~ ^https?://[A-Za-z0-9._-]+(:[0-9]{1,5})?$ ]]; }   # alamat dasar, tanpa path
valid_api_url() { [[ $1 =~ ^https?://[A-Za-z0-9._-]+(:[0-9]{1,5})?(/[A-Za-z0-9._~/-]*)?$ ]]; }   # boleh ber-path (/api/v1)
valid_port() { [[ $1 =~ ^[0-9]{1,5}$ ]] && [ "$1" -ge 1 ] && [ "$1" -le 65535 ]; }

# git dijalankan sebagai pemilik repo (bila skrip dijalankan sebagai root) supaya
# berkas di .git tidak berubah kepemilikan dan kunci SSH pemilik tetap terpakai.
g() {
  if [ "$(id -u)" -eq 0 ] && [ "$REPO_OWNER" != "root" ]; then
    runuser -u "$REPO_OWNER" -- env HOME="$OWNER_HOME" git -C "$APP_DIR" -c safe.directory="$APP_DIR" "$@"
  else
    git -C "$APP_DIR" -c safe.directory="$APP_DIR" "$@"
  fi
}

dc() {
  docker compose --env-file "$DEPLOY_ENV" --project-directory "$APP_DIR" -f "$APP_DIR/docker-compose.yml" "$@"
}

own_file() {  # kembalikan kepemilikan file ke pemilik repo bila dibuat sebagai root
  if [ "$(id -u)" -eq 0 ] && [ -n "$REPO_OWNER" ] && [ "$REPO_OWNER" != "root" ]; then
    chown "$REPO_OWNER" "$1" 2>/dev/null || true
  fi
}

apt_install() {
  [ "$(id -u)" -eq 0 ] || die "Butuh hak root untuk memasang paket: $*"
  if [ "$APT_UPDATED" -eq 0 ]; then
    DEBIAN_FRONTEND=noninteractive apt-get update -qq
    APT_UPDATED=1
  fi
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "$@"
}

# ----------------------------------------------------------------------------
# Tahap 0: hak akses, log, kunci
# ----------------------------------------------------------------------------
ensure_privileges() {
  [ "$(id -u)" -eq 0 ] && return 0
  if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then return 0; fi
  # PASTI_DEPLOY_SUDO mencegah perulangan tak berujung bila sudo ternyata tidak menaikkan hak akses.
  if command -v sudo >/dev/null 2>&1 && [ -z "${PASTI_DEPLOY_SUDO:-}" ]; then
    echo "Skrip butuh hak root/Docker; menjalankan ulang dengan sudo..."
    exec sudo -E PASTI_DEPLOY_SUDO=1 bash "$APP_DIR/$SCRIPT_NAME" "${ORIG_ARGS[@]}"
  fi
  die "Jalankan sebagai root, atau sebagai pengguna yang ada di grup docker/sudo."
}

setup_logging() {
  touch "$LOG_FILE" 2>/dev/null || LOG_FILE="/tmp/pasti-deploy.log"
  own_file "$LOG_FILE"
  exec > >(tee -a "$LOG_FILE") 2>&1
  printf '\n#### Deploy %s dimulai %s oleh %s ####\n' "$(date '+%F %T')" "$(hostname)" "${SUDO_USER:-$(id -un)}"
}

acquire_lock() {
  if ! command -v flock >/dev/null 2>&1; then
    warn "flock tidak tersedia; penguncian deploy ganda dilewati."
    return 0
  fi
  exec 9>"$LOCK_FILE" || { warn "Tidak bisa membuat $LOCK_FILE; penguncian dilewati."; return 0; }
  flock -n 9 || die "Deploy lain sedang berjalan (kunci: $LOCK_FILE)."
}

# ----------------------------------------------------------------------------
# Tahap 1: prasyarat sistem
# ----------------------------------------------------------------------------
install_docker() {
  local os_id codename
  os_id=$(. /etc/os-release && echo "${ID:-}")
  codename=$(. /etc/os-release && echo "${UBUNTU_CODENAME:-${VERSION_CODENAME:-}}")
  [ "$os_id" = "ubuntu" ] && [ -n "$codename" ] || die "Pemasangan Docker otomatis hanya untuk Ubuntu. Pasang manual: https://docs.docker.com/engine/install/"

  log "Memasang Docker dari repositori resmi (download.docker.com)..."
  apt_install ca-certificates curl
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $codename stable" \
    > /etc/apt/sources.list.d/docker.list
  APT_UPDATED=0
  apt_install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  systemctl enable --now docker
}

preflight_system() {
  step "Memeriksa prasyarat server"

  local os_id
  os_id=$( ( . /etc/os-release 2>/dev/null && echo "${ID:-?}" ) || echo "?")
  [ "$os_id" = "ubuntu" ] || warn "OS terdeteksi '$os_id', skrip ini diuji untuk Ubuntu."

  local missing=()
  command -v git     >/dev/null 2>&1 || missing+=(git)
  command -v curl    >/dev/null 2>&1 || missing+=(curl)
  command -v openssl >/dev/null 2>&1 || missing+=(openssl)
  command -v flock   >/dev/null 2>&1 || missing+=(util-linux)
  if [ ${#missing[@]} -gt 0 ]; then
    log "Memasang paket dasar: ${missing[*]}"
    apt_install "${missing[@]}" ca-certificates
  fi

  if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
    warn "Docker Engine / plugin Docker Compose belum terpasang."
    confirm "Pasang Docker sekarang dari repositori resmi Docker?" y \
      || die "Docker diperlukan. Pasang manual: https://docs.docker.com/engine/install/ubuntu/"
    install_docker
  fi
  if ! docker info >/dev/null 2>&1; then
    command -v systemctl >/dev/null 2>&1 && systemctl start docker >/dev/null 2>&1 || true
    sleep 2
  fi
  docker info >/dev/null 2>&1 || die "Daemon Docker tidak bisa diakses. Coba: sudo systemctl status docker"
  ok "Docker $(docker --version | awk '{print $3}' | tr -d ,), Compose $(docker compose version --short)"

  # Sumber daya: build Next.js membutuhkan memori & disk yang cukup.
  if command -v df >/dev/null 2>&1; then
    local free_gb
    free_gb=$(df -Pk "$APP_DIR" | awk 'NR==2 {printf "%d", $4/1024/1024}')
    if [ "${free_gb:-0}" -lt 3 ]; then warn "Ruang disk bebas hanya ~${free_gb}GB; build image butuh beberapa GB."; fi
  fi
  if [ -r /proc/meminfo ]; then
    local mem_mb swap_mb
    mem_mb=$(awk '/^MemTotal:/ {printf "%d", $2/1024}' /proc/meminfo)
    swap_mb=$(awk '/^SwapTotal:/ {printf "%d", $2/1024}' /proc/meminfo)
    if [ $((mem_mb + swap_mb)) -lt 1800 ]; then
      warn "Memori+swap ~$((mem_mb + swap_mb))MB. Build frontend (Next.js) bisa gagal karena kehabisan memori; tambahkan swap bila itu terjadi."
    fi
  fi
}

# ----------------------------------------------------------------------------
# Tahap 2: konfigurasi
# ----------------------------------------------------------------------------
run_wizard() {
  step "Pengaturan awal (hanya sekali; disimpan di deploy.env)"
  local ip default_root root bind="0.0.0.0" ans
  ip=$(primary_ip)
  default_root="http://${ip}:8686"

  cat <<EOF
    Masukkan alamat BACKEND/API yang dibuka oleh browser pengguna (tanpa garis miring
    di akhir). Alamat ini ditanam ke bundle frontend saat build.
      - Tanpa Nginx, akses langsung ke port : http://<IP-VPS>:8686
      - Dengan Nginx / domain              : https://pasti.kemenkeu.go.id
EOF
  while :; do
    read -r -p "    Alamat backend [$default_root]: " root || root=""
    root=${root:-$default_root}
    root=${root%/}
    if valid_url "$root"; then break; fi
    err "Format tidak valid. Contoh: http://203.0.113.10:8686 atau https://pasti.kemenkeu.go.id"
  done

  read -r -p "    Apakah aplikasi hanya diakses lewat Nginx di server ini (port dibuka hanya ke 127.0.0.1)? [y/N] " ans || ans=""
  [[ "$ans" =~ ^[YyJj] ]] && bind="127.0.0.1"

  (
    umask 077   # hanya untuk berkas ini; jangan bocor ke langkah lain (mis. git checkout)
    cat > "$DEPLOY_ENV" <<EOF
# Dibuat oleh deploy.sh pada $(date '+%F %T'). Aman diedit; jalankan ./deploy.sh lagi setelah mengubahnya.
# Alamat backend/API yang dibuka BROWSER pengguna. NEXT_PUBLIC_* ditanam ke bundle frontend saat build.
NEXT_PUBLIC_API_ROOT_URL=$root
NEXT_PUBLIC_API_URL=$root/api/v1
# 0.0.0.0 = bisa diakses dari luar server; 127.0.0.1 = hanya lewat Nginx di server ini.
BIND_ADDRESS=$bind
BACKEND_PORT=8686
FRONTEND_PORT=3000
EOF
  )
  own_file "$DEPLOY_ENV"
  ok "deploy.env dibuat."
}

load_deploy_config() {
  step "Membaca konfigurasi deploy"
  if [ ! -f "$DEPLOY_ENV" ]; then
    [ -t 0 ] || die "deploy.env belum ada dan terminal tidak interaktif. Buat dulu manual (contoh isi: ./deploy.sh --help)."
    run_wizard
  fi

  API_ROOT_URL=$(env_get "$DEPLOY_ENV" NEXT_PUBLIC_API_ROOT_URL)
  API_URL=$(env_get "$DEPLOY_ENV" NEXT_PUBLIC_API_URL)
  BIND_ADDRESS=$(env_get "$DEPLOY_ENV" BIND_ADDRESS); BIND_ADDRESS=${BIND_ADDRESS:-0.0.0.0}
  BACKEND_PORT=$(env_get "$DEPLOY_ENV" BACKEND_PORT); BACKEND_PORT=${BACKEND_PORT:-8686}
  FRONTEND_PORT=$(env_get "$DEPLOY_ENV" FRONTEND_PORT); FRONTEND_PORT=${FRONTEND_PORT:-3000}

  valid_url "$API_ROOT_URL" || die "NEXT_PUBLIC_API_ROOT_URL di deploy.env tidak valid: '$API_ROOT_URL'"
  valid_api_url "$API_URL"  || die "NEXT_PUBLIC_API_URL di deploy.env tidak valid: '$API_URL'"
  valid_port "$BACKEND_PORT"  || die "BACKEND_PORT di deploy.env tidak valid: '$BACKEND_PORT'"
  valid_port "$FRONTEND_PORT" || die "FRONTEND_PORT di deploy.env tidak valid: '$FRONTEND_PORT'"
  [ "$API_URL" = "$API_ROOT_URL/api/v1" ] || warn "NEXT_PUBLIC_API_URL ($API_URL) tidak sama dengan NEXT_PUBLIC_API_ROOT_URL + /api/v1."

  ok "API publik: $API_URL"
  ok "Port: backend $BACKEND_PORT, frontend $FRONTEND_PORT (bind $BIND_ADDRESS)"
}

create_backend_env() {
  [ -f "$BACKEND_ENV_EXAMPLE" ] || die "backend/.env.example tidak ditemukan."
  step "Membuat backend/.env dari contoh"

  # Contoh FRONTEND_URL: bila alamat backend memakai port (akses langsung), frontend ada di host
  # yang sama dengan port frontend; bila tanpa port (domain/Nginx), alamatnya sama.
  local scheme host_port host frontend_hint
  scheme=${API_ROOT_URL%%://*}
  host_port=${API_ROOT_URL#*://}
  host=${host_port%%:*}
  if [[ $host_port == *:* ]]; then frontend_hint="${scheme}://${host}:${FRONTEND_PORT}"; else frontend_hint="$API_ROOT_URL"; fi
  (
    umask 077
    cp "$BACKEND_ENV_EXAMPLE" "$BACKEND_ENV"
    env_set "$BACKEND_ENV" APP_ENV production
    env_set "$BACKEND_ENV" JWT_SECRET "$(gen_hex 48)"
    env_set "$BACKEND_ENV" PASSWORD_PEPPER "$(gen_hex 48)"
    {
      printf '\n# ============ Ditambahkan otomatis oleh deploy.sh ============\n'
      printf '# Kunci enkripsi token SSO: base64 dari TEPAT 32 byte. JANGAN diganti setelah dipakai.\n'
      printf 'TOKEN_ENCRYPTION_KEY=%s\n' "$(gen_key_b64)"
      printf '# Integrasi Inaproc\nINAPROC_BASE_URL=https://data.inaproc.id\nINAPROC_TOKEN=\n'
      printf '# Integrasi SLDK (opsional; kosongkan SLDK_DB_HOST untuk menonaktifkan)\n'
      printf 'SLDK_DB_HOST=\nSLDK_DB_PORT=1433\nSLDK_DB_USER=\nSLDK_DB_PASSWORD=\nSLDK_DB_NAME=\nSLDK_ASSET_TABLE=\nSLDK_ASSET_SEARCH_COLUMNS=\n'
    } >> "$BACKEND_ENV"
  )
  own_file "$BACKEND_ENV"
  ok "backend/.env dibuat; JWT_SECRET, PASSWORD_PEPPER, dan TOKEN_ENCRYPTION_KEY sudah diisi acak."

  cat <<EOF

    backend/.env baru dibuat dan BELUM siap dipakai. Edit dulu nilai berikut, lalu jalankan ulang ./deploy.sh:

      nano backend/.env

      DB_HOST / DB_PORT / DB_USER / DB_PASSWORD / DB_NAME
          Server SQL Server Anda. Jangan pakai 'localhost' (di dalam container itu artinya
          container itu sendiri). Pakai IP server database, atau host.docker.internal
          bila SQL Server berjalan di VPS yang sama.
      FRONTEND_URL        alamat frontend yang dibuka pengguna, mis. ${frontend_hint}
      SSO_ENV             production (SSO Kemenkeu asli) atau development (SSO demo)
      SSO_CLIENT_ID / SSO_CLIENT_SECRET
      SSO_REDIRECT_URI    ${API_ROOT_URL}/sso/callback/login  (harus terdaftar di SSO Kemenkeu)
      INAPROC_TOKEN       token API Inaproc (kosong = fitur Inaproc nonaktif)

EOF
  exit 1
}

validate_backend_env() {
  step "Memeriksa backend/.env"
  local errors=() warns=() key val
  local required=(DB_HOST DB_USER DB_PASSWORD DB_NAME JWT_SECRET PASSWORD_PEPPER
                  SSO_CLIENT_ID SSO_CLIENT_SECRET SSO_REDIRECT_URI FRONTEND_URL TOKEN_ENCRYPTION_KEY)

  for key in "${required[@]}"; do
    val=$(env_get "$BACKEND_ENV" "$key")
    if [ -z "$val" ]; then
      errors+=("$key kosong")
    elif is_placeholder "$val"; then
      errors+=("$key masih berisi nilai contoh")
    fi
  done

  val=$(env_get "$BACKEND_ENV" DB_HOST)
  case "$val" in
    localhost|127.0.0.1|::1)
      errors+=("DB_HOST=$val tidak bisa dipakai dari dalam container (itu menunjuk ke container itu sendiri); pakai IP server DB atau host.docker.internal") ;;
  esac

  val=$(env_get "$BACKEND_ENV" TOKEN_ENCRYPTION_KEY)
  if [ -n "$val" ]; then
    local nbytes
    nbytes=$(printf '%s' "$val" | base64 -d 2>/dev/null | wc -c | tr -d ' ' || true)
    [ "${nbytes:-0}" = "32" ] || errors+=("TOKEN_ENCRYPTION_KEY harus base64 dari tepat 32 byte (buat: openssl rand -base64 32)")
  fi

  # Nilai rahasia tidak boleh sama dengan contoh di repo.
  for key in JWT_SECRET PASSWORD_PEPPER DB_PASSWORD; do
    if [ -n "$(env_get "$BACKEND_ENV" "$key")" ] && [ "$(env_get "$BACKEND_ENV" "$key")" = "$(env_get "$BACKEND_ENV_EXAMPLE" "$key")" ]; then
      errors+=("$key sama dengan contoh di .env.example")
    fi
  done
  if [ -n "$(env_get "$BACKEND_ENV" SSO_CLIENT_SECRET)" ] && \
     [ "$(env_get "$BACKEND_ENV" SSO_CLIENT_SECRET)" = "$(env_get "$BACKEND_ENV_EXAMPLE" SSO_CLIENT_SECRET)" ]; then
    warns+=("SSO_CLIENT_SECRET sama dengan nilai di .env.example (yang tersimpan di git). Pastikan itu memang secret produksi, atau minta penggantian.")
  fi

  [ "$(env_get "$BACKEND_ENV" APP_ENV)" = "production" ] || warns+=("APP_ENV bukan 'production' (Gin berjalan dalam mode debug).")
  [ "$(env_get "$BACKEND_ENV" SSO_ENV)" = "production" ] || warns+=("SSO_ENV bukan 'production' (memakai SSO demo, bukan SSO Kemenkeu asli).")
  [ -n "$(env_get "$BACKEND_ENV" INAPROC_TOKEN)" ] || warns+=("INAPROC_TOKEN kosong: fitur Inaproc akan membalas 503.")

  for key in FRONTEND_URL SSO_REDIRECT_URI; do
    case "$(env_get "$BACKEND_ENV" "$key")" in
      *localhost*|*127.0.0.1*) warns+=("$key masih menunjuk ke localhost; pengguna di luar server tidak bisa memakainya.") ;;
    esac
  done
  [ "$(env_get "$BACKEND_ENV" SSO_REDIRECT_URI)" = "$API_ROOT_URL/sso/callback/login" ] \
    || warns+=("SSO_REDIRECT_URI sebaiknya $API_ROOT_URL/sso/callback/login (sesuai alamat di deploy.env).")

  # Compose menginterpolasi tanda $ pada nilai env_file (kecuali diapit kutip tunggal);
  # kata sandi berisi $ bisa terpotong. Hanya nama key yang ditampilkan, bukan nilainya.
  local dollar_keys
  dollar_keys=$(grep -E "^[A-Za-z_][A-Za-z0-9_]*=[^'].*[$]" "$BACKEND_ENV" | cut -d= -f1 | tr '\n' ' ' || true)
  [ -z "$dollar_keys" ] || warns+=("Nilai berisi tanda \$ pada: ${dollar_keys}. Compose bisa menafsirkannya sebagai variabel; bungkus nilainya dengan tanda kutip tunggal '...' atau tulis \$\$.")

  local perm
  perm=$(stat -c '%a' "$BACKEND_ENV" 2>/dev/null || echo "")
  case "$perm" in 600|400|"") ;; *) warns+=("Izin backend/.env $perm; disarankan 600 (chmod 600 backend/.env).") ;; esac

  local w
  for w in "${warns[@]+"${warns[@]}"}"; do warn "$w"; done
  if [ ${#errors[@]} -gt 0 ]; then
    for w in "${errors[@]}"; do err "$w"; done
    die "backend/.env belum benar. Perbaiki (nano backend/.env) lalu jalankan ulang."
  fi
  ok "backend/.env lengkap."
}

prepare_backend_env() {
  [ -f "$BACKEND_ENV" ] || create_backend_env   # keluar dengan instruksi
  validate_backend_env
}

# ----------------------------------------------------------------------------
# Tahap 3: kode
# ----------------------------------------------------------------------------
sync_code() {
  step "Menyiapkan kode"
  [ -d "$APP_DIR/.git" ] || die "$APP_DIR bukan repositori git. Clone dulu repo-nya, lalu jalankan skrip ini dari dalamnya."

  PREV_COMMIT=$(g rev-parse --short HEAD)
  local current target
  current=$(g rev-parse --abbrev-ref HEAD)
  target=${TARGET_BRANCH:-$current}
  [ "$target" != "HEAD" ] || die "Repo dalam keadaan detached HEAD. Beri nama branch: ./deploy.sh <branch>"

  if [ "$NO_PULL" -eq 1 ]; then
    ok "--no-pull: memakai kode di server apa adanya ($current @ $PREV_COMMIT)."
    return 0
  fi

  local dirty
  dirty=$(g -c core.fileMode=false status --porcelain --untracked-files=no)
  if [ -n "$dirty" ]; then
    if [ "$FORCE" -eq 1 ]; then
      warn "--force: membuang perubahan lokal berikut:"
      printf '%s\n' "$dirty" | sed 's/^/        /'
      g reset --hard
    else
      err "Ada perubahan lokal pada file yang dilacak git di server:"
      printf '%s\n' "$dirty" | sed 's/^/        /'
      die "Commit/simpan/batalkan perubahan itu dulu, atau jalankan dengan --force untuk membuangnya."
    fi
  fi

  log "git fetch origin ..."
  g fetch --prune origin
  g rev-parse --verify --quiet "origin/$target" >/dev/null || die "Branch 'origin/$target' tidak ada di remote."

  if [ "$target" != "$current" ]; then
    log "Pindah branch: $current -> $target"
    if g rev-parse --verify --quiet "refs/heads/$target" >/dev/null; then
      g checkout "$target"
    else
      g checkout -b "$target" --track "origin/$target"
    fi
  fi

  if [ "$FORCE" -eq 1 ]; then
    g reset --hard "origin/$target"
  else
    g merge --ff-only "origin/$target" >/dev/null \
      || die "Tidak bisa fast-forward ke origin/$target (riwayat lokal menyimpang). Gunakan --force bila yakin."
  fi

  local now
  now=$(g rev-parse --short HEAD)
  if [ "$now" = "$PREV_COMMIT" ] && [ "$target" = "$current" ]; then
    ok "Kode sudah terbaru: $target @ $now"
  else
    ok "Kode: $target  $PREV_COMMIT -> $now"
    g log --oneline --no-decorate -n 15 "$PREV_COMMIT..$now" 2>/dev/null | sed 's/^/        /' || true
  fi
}

# ----------------------------------------------------------------------------
# Tahap 4-6: build, migrasi, jalankan, cek kesehatan
# ----------------------------------------------------------------------------
snapshot_images() {
  # Simpan salinan ":rollback" dari image yang sedang dipakai, untuk jaga-jaga.
  ROLLBACK_IMAGES=()
  local img id repo
  while IFS= read -r img; do
    [ -n "$img" ] || continue
    id=$(docker image inspect -f '{{.Id}}' "$img" 2>/dev/null || true)
    [ -n "$id" ] || continue
    repo=${img%%:*}
    docker tag "$id" "$repo:rollback"
    ROLLBACK_IMAGES+=("$img")
  done < <(dc config --images 2>/dev/null || true)
  if [ ${#ROLLBACK_IMAGES[@]} -gt 0 ]; then
    ok "Image saat ini disimpan untuk rollback: ${ROLLBACK_IMAGES[*]}"
  else
    log "Belum ada image sebelumnya (deploy pertama); rollback tidak tersedia."
  fi
}

build_images() {
  step "Build image (bisa beberapa menit)"
  local args=()
  [ "$FRESH" -eq 1 ] && args=(--no-cache --pull)
  dc build "${args[@]}" || die "Build gagal. Container lama tidak diubah dan tetap berjalan."
  ok "Build selesai."
}

run_migrations() {
  if [ "$NO_MIGRATE" -eq 1 ]; then
    step "Migrasi database dilewati (--no-migrate)"
    return 0
  fi
  step "Migrasi database"
  log "Hanya migrasi yang belum tercatat di schema_migrations yang dijalankan."
  log "(Deploy pertama menjalankan seluruh migrasi; aman karena semuanya idempoten.)"
  dc run --rm -T --no-deps --entrypoint ./pasti-migrate backend \
    || die "Migrasi gagal. Container lama TIDAK diubah dan tetap berjalan. Periksa pesan di atas (koneksi DB di backend/.env?)."
  ok "Migrasi selesai."
}

container_state() { docker inspect -f '{{.State.Status}}' "$1" 2>/dev/null || echo "tidak-ada"; }

wait_http() {  # wait_http NAMA CONTAINER URL
  local name=$1 container=$2 url=$3 elapsed=0
  while [ "$elapsed" -lt "$HEALTH_TIMEOUT" ]; do
    if curl -fsS -m 5 -o /dev/null "$url" 2>/dev/null; then
      ok "$name sehat ($url)"
      return 0
    fi
    case "$(container_state "$container")" in
      restarting|exited|dead) err "$name berhenti/crash-loop (status: $(container_state "$container"))."; return 1 ;;
    esac
    sleep 3
    elapsed=$((elapsed + 3))
  done
  err "$name tidak merespons dalam ${HEALTH_TIMEOUT} detik ($url)."
  return 1
}

health_checks() {
  step "Cek kesehatan"
  local host="127.0.0.1"
  case "$BIND_ADDRESS" in 0.0.0.0|127.0.0.1) ;; *) host="$BIND_ADDRESS" ;; esac
  wait_http "Backend"  pasti-backend  "http://${host}:${BACKEND_PORT}/health" || return 1
  wait_http "Frontend" pasti-frontend "http://${host}:${FRONTEND_PORT}/login" || return 1
}

start_stack() {
  step "Menjalankan container"
  dc up -d --remove-orphans
}

rollback() {
  step "ROLLBACK ke image sebelumnya"
  log "Log terakhir container yang gagal:"
  dc logs --tail 40 backend frontend 2>&1 | sed 's/^/        /' || true

  if [ ${#ROLLBACK_IMAGES[@]} -eq 0 ]; then
    err "Tidak ada image sebelumnya untuk dipulihkan (ini deploy pertama)."
    return 1
  fi
  local img repo
  for img in "${ROLLBACK_IMAGES[@]}"; do
    repo=${img%%:*}
    docker tag "$repo:rollback" "$img"
  done
  dc up -d --no-build --force-recreate
  if health_checks; then
    warn "Aplikasi dipulihkan ke versi sebelumnya (kode: $PREV_COMMIT)."
    warn "Catatan: migrasi database yang sudah diterapkan TIDAK ikut dibatalkan (hanya menambah tabel/kolom, aman untuk versi lama)."
    warn "Kode di folder server tetap versi terbaru (yang bermasalah). Perbaiki dulu, push, lalu jalankan ./deploy.sh lagi."
  else
    err "Rollback pun gagal. Periksa: docker compose logs"
  fi
  return 1
}

cleanup_images() {
  docker image prune -f >/dev/null 2>&1 || true
}

summary() {
  local took=$((SECONDS - START_TS)) frontend_url
  frontend_url=$(env_get "$BACKEND_ENV" FRONTEND_URL)
  step "Selesai dalam $((took / 60)) menit $((took % 60)) detik"
  dc ps 2>/dev/null | sed 's/^/    /' || true
  cat <<EOF

    Kode      : $(g rev-parse --abbrev-ref HEAD) @ $(g rev-parse --short HEAD)
    Aplikasi  : ${frontend_url:-http://$(primary_ip):${FRONTEND_PORT}}
    API       : ${API_URL}
    Log       : docker compose logs -f   (atau: tail -f $LOG_FILE)
    Berikutnya: cukup jalankan ./deploy.sh setiap kali ada pembaruan (kode ditarik otomatis).
EOF
}

# ----------------------------------------------------------------------------
# Alur utama
# ----------------------------------------------------------------------------
main() {
  parse_args "$@"
  ensure_privileges

  # Pemilik folder repo: git dan berkas yang dibuat skrip (log, .env) tetap milik pengguna ini
  # walau skrip berjalan sebagai root lewat sudo.
  REPO_OWNER=$(stat -c '%U' "$APP_DIR")
  OWNER_HOME=$(getent passwd "$REPO_OWNER" 2>/dev/null | cut -d: -f6 || true)
  OWNER_HOME=${OWNER_HOME:-$HOME}

  setup_logging
  acquire_lock
  trap 'err "Gagal tak terduga di baris $LINENO: $BASH_COMMAND"' ERR

  [ -d "$APP_DIR/.git" ] || die "$APP_DIR bukan repositori git."
  [ -f "$APP_DIR/docker-compose.yml" ] || die "docker-compose.yml tidak ditemukan di $APP_DIR."

  preflight_system
  load_deploy_config
  sync_code            # setelah ini kode di disk sudah terbaru
  prepare_backend_env  # membuat/validasi backend/.env (bisa berhenti dengan instruksi)

  snapshot_images
  build_images
  run_migrations

  if ! start_stack || ! health_checks; then
    rollback || true
    die "Deploy GAGAL. Lihat pesan di atas dan $LOG_FILE."
  fi

  cleanup_images
  summary
}

# Hanya jalan bila dieksekusi langsung (bukan di-source), dan seluruh fungsi sudah
# dibaca di atas: aman walau berkas ini berubah karena git pull saat skrip berjalan.
if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  main "$@"
  exit $?
fi
