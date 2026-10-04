#!/usr/bin/env bash
# =============================================================================
# deploy.sh - Deploy PASTI V3 ke VPS Ubuntu dengan Docker Compose.
#
# Jalankan dari folder repo di VPS. Ada dua environment:
#
#   ./deploy.sh dev     # development  (branch bawaan: development)
#   ./deploy.sh prod    # production   (branch bawaan: production_v3; https://pasti.kemenkeu.go.id)
#   ./deploy.sh         # mengulang environment yang sudah tercatat di folder ini
#   ./deploy.sh --help
#
# Satu folder = satu environment. Untuk dev dan prod di server yang sama, clone repo ke dua folder.
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
CLI_ENV=""          # environment dari argumen: dev | prod
CLI_BRANCH=""       # branch dari --branch (menimpa DEPLOY_BRANCH)
ENVIRONMENT=""      # environment yang dipakai: dev | prod
DEPLOY_BRANCH=""    # branch bawaan environment ini (dari deploy.env)
CONTAINER_PREFIX="pasti"
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
# Sidik deploy.sh saat skrip mulai berjalan; dibandingkan lagi setelah git pull (lihat pastikan_skrip_terbaru).
SCRIPT_SHA_AWAL=$(sha256sum "$APP_DIR/$SCRIPT_NAME" 2>/dev/null | cut -d' ' -f1 || true)

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
Pemakaian: ./deploy.sh <environment> [opsi]

Environment:
  dev, development   server development (mis. http://IP:3001). Branch bawaan: development
  prod, production   server production (https://pasti.kemenkeu.go.id). Branch bawaan: production_v3.
                     Meminta konfirmasi dan memeriksa backend/.env lebih ketat.

Satu folder = satu environment. Folder mengingat environment-nya (ENVIRONMENT di deploy.env),
jadi setelah deploy pertama cukup ./deploy.sh tanpa argumen. Untuk menjalankan dev DAN prod di
server yang sama, clone repo ke dua folder berbeda (mis. ~/pasti-dev dan ~/pasti-prod).

Opsi:
  --branch <nama> deploy dari branch ini, bukan branch bawaan environment
  --no-pull       jangan tarik kode; deploy kode yang ada di server apa adanya
  --no-migrate    lewati migrasi database
  --fresh         build tanpa cache dan tarik base image terbaru (lebih lambat)
  --force         buang perubahan lokal pada file yang dilacak git di server
                  (git reset --hard origin/<branch>). Hati-hati.
  -y, --yes       jawab "ya" otomatis pada konfirmasi (termasuk konfirmasi deploy prod)
  -h, --help      tampilkan bantuan ini

Konfigurasi (dibuat otomatis saat pertama kali dijalankan):
  deploy.env      environment, branch, URL publik backend, port, dan awalan nama container.
                  Contoh isi (prod):
                    ENVIRONMENT=prod
                    DEPLOY_BRANCH=production_v3
                    CONTAINER_PREFIX=pasti-prod
                    NEXT_PUBLIC_API_ROOT_URL=https://pasti.kemenkeu.go.id
                    NEXT_PUBLIC_API_URL=https://pasti.kemenkeu.go.id/api/v1
                    BIND_ADDRESS=127.0.0.1     # hanya lewat Nginx/reverse proxy di server ini
                    BACKEND_PORT=8686
                    FRONTEND_PORT=3000
                    # LOGIN_PATH=/...   (opsional) menimpa alamat halaman login bawaan repo (frontend/login-path.txt)
  backend/.env    rahasia aplikasi, koneksi database, SSO, token Inaproc (dibuat dari
                  backend/.env.example; Anda yang mengisi nilai database & SSO).

Variabel lingkungan:
  HEALTH_TIMEOUT=180   detik menunggu tiap layanan sehat (bawaan 90)
  SKIP_DB_CHECK=1      lewati pemeriksaan keterjangkauan server database sebelum build

Log setiap deploy ditambahkan ke deploy.log.
EOF
}

set_env_arg() {
  [ -z "$CLI_ENV" ] || [ "$CLI_ENV" = "$1" ] || die "Hanya boleh satu environment (dev atau prod)."
  CLI_ENV=$1
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      dev|development) set_env_arg dev ;;
      prod|production) set_env_arg prod ;;
      --branch)
        [ $# -ge 2 ] || die "--branch membutuhkan nama branch."
        CLI_BRANCH=$2
        shift
        ;;
      --branch=*)   CLI_BRANCH=${1#--branch=} ;;
      --no-pull)    NO_PULL=1 ;;
      --no-migrate) NO_MIGRATE=1 ;;
      --fresh)      FRESH=1 ;;
      --force)      FORCE=1 ;;
      -y|--yes)     ASSUME_YES=1 ;;
      -h|--help)    usage; exit 0 ;;
      -*)           die "Opsi tidak dikenal: $1 (lihat --help)" ;;
      *)            die "Argumen tidak dikenal: '$1'. Environment yang valid: dev atau prod. Untuk memilih branch pakai --branch <nama>." ;;
    esac
    shift
  done
  if [ -n "$CLI_BRANCH" ] && ! [[ "$CLI_BRANCH" =~ ^[A-Za-z0-9][A-Za-z0-9._/-]*$ ]]; then
    die "Nama branch tidak valid: $CLI_BRANCH"
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
env_label() { if [ "$ENVIRONMENT" = "prod" ]; then printf 'PRODUCTION'; else printf 'DEVELOPMENT'; fi; }

# Tentukan environment: dari argumen, atau dari deploy.env yang sudah ada. Bila keduanya tidak ada, tanya.
# Satu folder = satu environment: argumen yang bertentangan dengan deploy.env ditolak, supaya folder dev
# tidak bisa tidak sengaja men-deploy production (dan sebaliknya).
resolve_environment() {
  local file_env="" ans tries=0
  if [ -f "$DEPLOY_ENV" ]; then
    file_env=$(env_get "$DEPLOY_ENV" ENVIRONMENT)
    # deploy.env lama (dibuat sebelum ada pemisahan environment) = development.
    file_env=${file_env:-dev}
  fi
  if [ -n "$CLI_ENV" ] && [ -n "$file_env" ] && [ "$CLI_ENV" != "$file_env" ]; then
    die "Folder ini sudah dikonfigurasi untuk environment '$file_env', bukan '$CLI_ENV'. Satu folder = satu environment: clone repo ke folder lain untuk '$CLI_ENV'."
  fi
  ENVIRONMENT=${CLI_ENV:-$file_env}
  if [ -z "$ENVIRONMENT" ]; then
    [ -t 0 ] || die "Sebutkan environment: ./deploy.sh dev  atau  ./deploy.sh prod"
    while [ -z "$ENVIRONMENT" ] && [ "$tries" -lt 3 ]; do
      tries=$((tries + 1))
      read -r -p "    Environment apa yang akan dideploy dari folder ini? [dev/prod]: " ans || ans=""
      case "$ans" in
        dev|development) ENVIRONMENT=dev ;;
        prod|production) ENVIRONMENT=prod ;;
        *) err "Ketik 'dev' atau 'prod'." ;;
      esac
    done
    [ -n "$ENVIRONMENT" ] || die "Environment belum dipilih."
  fi
}

run_wizard() {
  step "Pengaturan awal $(env_label) (hanya sekali; disimpan di deploy.env)"
  local ip default_root root bind backend_port frontend_port branch prefix ans
  ip=$(primary_ip)

  if [ "$ENVIRONMENT" = "prod" ]; then
    default_root="https://pasti.kemenkeu.go.id"; bind="127.0.0.1"; branch="production_v3"; prefix="pasti-prod"
    cat <<'EOF'
    PRODUCTION: alamat harus https. Aplikasi dibuka lewat reverse proxy (Nginx/WAF) yang meneruskan
    /api, /health, /sso/login, dan /sso/callback/login ke backend, dan sisanya ke frontend.
    Contoh konfigurasi Nginx: deploy/nginx/pasti.kemenkeu.go.id.conf
EOF
  else
    default_root="http://${ip}:8686"; bind="0.0.0.0"; branch="development"; prefix="pasti"
    cat <<'EOF'
    DEVELOPMENT: masukkan alamat BACKEND/API yang dibuka oleh browser pengguna (tanpa garis miring
    di akhir). Alamat ini ditanam ke bundle frontend saat build.
      - Tanpa Nginx, akses langsung ke port : http://<IP-VPS>:8686
      - Dengan Nginx / domain              : https://pasti-dev.contoh.go.id
EOF
  fi

  while :; do
    read -r -p "    Alamat backend/API [$default_root]: " root || root=""
    root=${root:-$default_root}
    root=${root%/}
    if ! valid_url "$root"; then
      err "Format tidak valid. Contoh: http://203.0.113.10:8686 atau https://pasti.kemenkeu.go.id"; continue
    fi
    if [ "$ENVIRONMENT" = "prod" ] && [[ $root != https://* ]]; then
      err "Environment production wajib memakai https://"; continue
    fi
    break
  done

  while :; do
    read -r -p "    Port backend di server [8686]: " backend_port || backend_port=""
    backend_port=${backend_port:-8686}
    valid_port "$backend_port" && break
    err "Port tidak valid (1-65535)."
  done
  while :; do
    read -r -p "    Port frontend di server [3000]: " frontend_port || frontend_port=""
    frontend_port=${frontend_port:-3000}
    valid_port "$frontend_port" && break
    err "Port tidak valid (1-65535)."
  done

  if [ "$ENVIRONMENT" = "prod" ]; then
    read -r -p "    Diakses lewat Nginx/reverse proxy di server ini (port hanya dibuka ke 127.0.0.1)? [Y/n] " ans || ans=""
    [[ "$ans" =~ ^[Nn] ]] && bind="0.0.0.0"
  else
    read -r -p "    Apakah aplikasi hanya diakses lewat Nginx di server ini (port dibuka hanya ke 127.0.0.1)? [y/N] " ans || ans=""
    [[ "$ans" =~ ^[YyJj] ]] && bind="127.0.0.1"
  fi

  (
    umask 077   # hanya untuk berkas ini; jangan bocor ke langkah lain (mis. git checkout)
    cat > "$DEPLOY_ENV" <<EOF
# Dibuat oleh deploy.sh pada $(date '+%F %T'). Aman diedit; jalankan ./deploy.sh lagi setelah mengubahnya.
# Satu folder = satu environment. Jangan diubah setelah dipakai (untuk environment lain, pakai folder lain).
ENVIRONMENT=$ENVIRONMENT
# Branch yang di-deploy (bisa ditimpa sekali jalan dengan --branch).
DEPLOY_BRANCH=$branch
# Awalan nama container. Wajib berbeda bila dev dan prod berjalan di server yang sama.
CONTAINER_PREFIX=$prefix
# Alamat backend/API yang dibuka BROWSER pengguna. NEXT_PUBLIC_* ditanam ke bundle frontend saat build.
NEXT_PUBLIC_API_ROOT_URL=$root
NEXT_PUBLIC_API_URL=$root/api/v1
# 0.0.0.0 = bisa diakses dari luar server; 127.0.0.1 = hanya lewat Nginx/reverse proxy di server ini.
BIND_ADDRESS=$bind
BACKEND_PORT=$backend_port
FRONTEND_PORT=$frontend_port
EOF
  )
  own_file "$DEPLOY_ENV"
  ok "deploy.env dibuat untuk environment $(env_label)."
}

load_deploy_config() {
  step "Membaca konfigurasi deploy ($(env_label))"
  if [ ! -f "$DEPLOY_ENV" ]; then
    [ -t 0 ] || die "deploy.env belum ada dan terminal tidak interaktif. Buat dulu manual (contoh isi: ./deploy.sh --help)."
    run_wizard
  fi

  # deploy.env lama (sebelum ada pemisahan environment): catat environment-nya sekarang.
  if [ -z "$(env_get "$DEPLOY_ENV" ENVIRONMENT)" ]; then
    env_set "$DEPLOY_ENV" ENVIRONMENT "$ENVIRONMENT"
    own_file "$DEPLOY_ENV"
    log "Environment '$ENVIRONMENT' dicatat di deploy.env."
  fi

  API_ROOT_URL=$(env_get "$DEPLOY_ENV" NEXT_PUBLIC_API_ROOT_URL)
  API_URL=$(env_get "$DEPLOY_ENV" NEXT_PUBLIC_API_URL)
  BIND_ADDRESS=$(env_get "$DEPLOY_ENV" BIND_ADDRESS); BIND_ADDRESS=${BIND_ADDRESS:-0.0.0.0}
  BACKEND_PORT=$(env_get "$DEPLOY_ENV" BACKEND_PORT); BACKEND_PORT=${BACKEND_PORT:-8686}
  FRONTEND_PORT=$(env_get "$DEPLOY_ENV" FRONTEND_PORT); FRONTEND_PORT=${FRONTEND_PORT:-3000}
  DEPLOY_BRANCH=$(env_get "$DEPLOY_ENV" DEPLOY_BRANCH)   # kosong (deploy.env lama) = branch yang sedang aktif
  CONTAINER_PREFIX=$(env_get "$DEPLOY_ENV" CONTAINER_PREFIX)
  if [ -z "$CONTAINER_PREFIX" ]; then
    if [ "$ENVIRONMENT" = "prod" ]; then CONTAINER_PREFIX="pasti-prod"; else CONTAINER_PREFIX="pasti"; fi
  fi

  valid_url "$API_ROOT_URL" || die "NEXT_PUBLIC_API_ROOT_URL di deploy.env tidak valid: '$API_ROOT_URL'"
  valid_api_url "$API_URL"  || die "NEXT_PUBLIC_API_URL di deploy.env tidak valid: '$API_URL'"
  valid_port "$BACKEND_PORT"  || die "BACKEND_PORT di deploy.env tidak valid: '$BACKEND_PORT'"
  valid_port "$FRONTEND_PORT" || die "FRONTEND_PORT di deploy.env tidak valid: '$FRONTEND_PORT'"
  [[ $CONTAINER_PREFIX =~ ^[a-z0-9][a-z0-9_-]*$ ]] || die "CONTAINER_PREFIX di deploy.env tidak valid: '$CONTAINER_PREFIX' (huruf kecil, angka, - dan _)"
  [ "$API_URL" = "$API_ROOT_URL/api/v1" ] || warn "NEXT_PUBLIC_API_URL ($API_URL) tidak sama dengan NEXT_PUBLIC_API_ROOT_URL + /api/v1."
  if [ "$ENVIRONMENT" = "prod" ] && [[ $API_ROOT_URL != https://* ]]; then
    die "Environment production wajib memakai https:// (NEXT_PUBLIC_API_ROOT_URL sekarang: $API_ROOT_URL)."
  fi

  ok "Environment: $(env_label)"
  ok "API publik: $API_URL"
  ok "Port: backend $BACKEND_PORT, frontend $FRONTEND_PORT (bind $BIND_ADDRESS); container: ${CONTAINER_PREFIX}-backend / ${CONTAINER_PREFIX}-frontend"
}

create_backend_env() {
  [ -f "$BACKEND_ENV_EXAMPLE" ] || die "backend/.env.example tidak ditemukan."
  step "Membuat backend/.env dari contoh"

  # Contoh FRONTEND_URL: bila alamat backend memakai port (akses langsung), frontend ada di host
  # yang sama dengan port frontend; bila tanpa port (domain/Nginx), alamatnya sama.
  local scheme host_port host frontend_hint app_env sso_env
  scheme=${API_ROOT_URL%%://*}
  host_port=${API_ROOT_URL#*://}
  host=${host_port%%:*}
  if [[ $host_port == *:* ]]; then frontend_hint="${scheme}://${host}:${FRONTEND_PORT}"; else frontend_hint="$API_ROOT_URL"; fi
  if [ "$ENVIRONMENT" = "prod" ]; then app_env="production"; sso_env="production"; else app_env="development"; sso_env="development"; fi
  (
    umask 077
    cp "$BACKEND_ENV_EXAMPLE" "$BACKEND_ENV"
    env_set "$BACKEND_ENV" APP_ENV "$app_env"
    env_set "$BACKEND_ENV" SSO_ENV "$sso_env"
    # URL yang sudah diketahui dari deploy.env langsung diisi.
    env_set "$BACKEND_ENV" FRONTEND_URL "$frontend_hint"
    env_set "$BACKEND_ENV" SSO_REDIRECT_URI "$API_ROOT_URL/sso/callback/login"
    env_set "$BACKEND_ENV" JWT_SECRET "$(gen_hex 48)"
    env_set "$BACKEND_ENV" PASSWORD_PEPPER "$(gen_hex 48)"
    {
      printf '\n# ============ Ditambahkan otomatis oleh deploy.sh ============\n'
      printf '# Kunci enkripsi token SSO: base64 dari TEPAT 32 byte. JANGAN diganti setelah dipakai.\n'
      printf 'TOKEN_ENCRYPTION_KEY=%s\n' "$(gen_key_b64)"
      printf '# Integrasi Inaproc\nINAPROC_BASE_URL=https://data.inaproc.id\nINAPROC_TOKEN=\n'
      printf '# Integrasi SLDK (opsional; kosongkan SLDK_DB_HOST untuk menonaktifkan)\n'
      printf 'SLDK_DB_HOST=\nSLDK_DB_PORT=1433\nSLDK_DB_USER=\nSLDK_DB_PASSWORD=\nSLDK_DB_NAME=\nSLDK_ASSET_TABLE=\nSLDK_ASSET_SEARCH_COLUMNS=\n'
      printf '# Batasi data aset SLDK ke satu K/L menurut kodenya (015 = Kemenkeu); kosong = semua K/L\nSLDK_KL_KODE=015\n'
    } >> "$BACKEND_ENV"
  )
  own_file "$BACKEND_ENV"
  ok "backend/.env dibuat untuk $(env_label); JWT_SECRET, PASSWORD_PEPPER, dan TOKEN_ENCRYPTION_KEY sudah diisi acak."
  ok "Sudah terisi otomatis: APP_ENV=$app_env, SSO_ENV=$sso_env, FRONTEND_URL, SSO_REDIRECT_URI."

  cat <<EOF

    backend/.env baru dibuat dan BELUM siap dipakai. Isi dulu nilai berikut, lalu jalankan ulang ./deploy.sh:

      nano backend/.env

      DB_HOST / DB_PORT / DB_USER / DB_PASSWORD / DB_NAME
          Server SQL Server untuk environment ini (${ENVIRONMENT} sebaiknya memakai database sendiri).
          Jangan pakai 'localhost' (di dalam container itu artinya container itu sendiri). Pakai IP
          server database, atau host.docker.internal bila SQL Server berjalan di VPS yang sama.
      SSO_CLIENT_ID / SSO_CLIENT_SECRET
          Kredensial SSO yang sesuai SSO_ENV=${sso_env}. SSO_REDIRECT_URI (${API_ROOT_URL}/sso/callback/login)
          harus terdaftar di SSO Kemenkeu untuk client tersebut.
      SSO_SCOPE           bawaan 'openid profile'; tambahkan 'hris2 profil.hris' bila memakai fitur Cari Pegawai (HRIS2)
      INAPROC_TOKEN       token API Inaproc (kosong = halaman Pengadaan/Tender membalas 503)
EOF
  if [ "$ENVIRONMENT" = "prod" ]; then
    cat <<EOF

      Production memeriksa lebih ketat: SSO_ENV dan APP_ENV harus 'production', FRONTEND_URL dan
      SSO_REDIRECT_URI harus https://, INAPROC_TOKEN wajib, dan secret SSO tidak boleh sama dengan contoh.
EOF
  fi
  echo
  exit 1
}

# Mencatat satu temuan: kesalahan di environment prod, peringatan di dev. Memakai variabel lokal
# errors/warns milik pemanggil (validate_backend_env).
flag_issue() {
  if [ "$ENVIRONMENT" = "prod" ]; then errors+=("$1"); else warns+=("$1"); fi
}

validate_backend_env() {
  step "Memeriksa backend/.env ($(env_label))"
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
  # Di bawah ini, pada environment PROD pelanggaran = kesalahan (deploy dihentikan); pada dev = peringatan.
  if [ -n "$(env_get "$BACKEND_ENV" SSO_CLIENT_SECRET)" ] && \
     [ "$(env_get "$BACKEND_ENV" SSO_CLIENT_SECRET)" = "$(env_get "$BACKEND_ENV_EXAMPLE" SSO_CLIENT_SECRET)" ]; then
    flag_issue "SSO_CLIENT_SECRET sama dengan nilai di .env.example (yang tersimpan di git): itu kredensial contoh/demo, bukan secret produksi."
  fi

  [ "$(env_get "$BACKEND_ENV" APP_ENV)" = "production" ] || flag_issue "APP_ENV bukan 'production' (Gin berjalan dalam mode debug)."
  [ "$(env_get "$BACKEND_ENV" SSO_ENV)" = "production" ] || flag_issue "SSO_ENV bukan 'production' (memakai SSO demo, bukan SSO Kemenkeu asli)."
  [ -n "$(env_get "$BACKEND_ENV" INAPROC_TOKEN)" ] || flag_issue "INAPROC_TOKEN kosong: fitur Inaproc akan membalas 503."

  for key in FRONTEND_URL SSO_REDIRECT_URI; do
    val=$(env_get "$BACKEND_ENV" "$key")
    case "$val" in
      *localhost*|*127.0.0.1*) flag_issue "$key masih menunjuk ke localhost; pengguna di luar server tidak bisa memakainya." ;;
    esac
    if [ "$ENVIRONMENT" = "prod" ] && [ -n "$val" ] && [[ $val != https://* ]]; then
      errors+=("$key harus diawali https:// pada environment prod (sekarang: $val)")
    fi
  done
  [ "$(env_get "$BACKEND_ENV" SSO_REDIRECT_URI)" = "$API_ROOT_URL/sso/callback/login" ] \
    || flag_issue "SSO_REDIRECT_URI sebaiknya $API_ROOT_URL/sso/callback/login (sesuai alamat di deploy.env)."

  [ "$(env_get "$BACKEND_ENV" DB_USER)" != "sa" ] \
    || warns+=("DB_USER=sa (administrator penuh SQL Server). Sebaiknya pakai login khusus aplikasi dengan hak seperlunya.")

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

# Server database biasanya BERBEDA dari server aplikasi. Pastikan port-nya terjangkau sebelum build
# (5-15 menit) dan sebelum mengubah apa pun. Kegagalan paling umum: firewall server database belum
# mengizinkan IP server ini. Bisa dilewati dengan SKIP_DB_CHECK=1.
check_db_reachable() {
  local host port
  host=$(env_get "$BACKEND_ENV" DB_HOST)
  port=$(env_get "$BACKEND_ENV" DB_PORT); port=${port:-1433}
  step "Memeriksa koneksi ke server database ($host:$port)"
  if [ -n "${SKIP_DB_CHECK:-}" ]; then
    log "SKIP_DB_CHECK: pemeriksaan dilewati."
    return 0
  fi
  if [ "$host" = "host.docker.internal" ]; then
    log "DB_HOST=host.docker.internal (database di server yang sama): diuji dari dalam container saat migrasi."
    return 0
  fi
  valid_port "$port" || die "DB_PORT di backend/.env tidak valid: '$port'"
  # host/port dikirim sebagai argumen (bukan disisipkan ke perintah), jadi nilai aneh di .env tidak dieksekusi.
  if timeout 5 bash -c 'exec 3<>"/dev/tcp/$0/$1"' "$host" "$port" 2>/dev/null; then
    ok "Server database terjangkau ($host:$port)"
    return 0
  fi
  err "Server database $host:$port TIDAK terjangkau dari server ini (IP server ini kemungkinan: $(primary_ip))."
  err "Periksa: (1) firewall server database mengizinkan IP server ini ke port $port;"
  err "         (2) SQL Server mengaktifkan TCP/IP di port $port;  (3) DB_HOST/DB_PORT di backend/.env."
  err "Bila Anda yakin koneksinya benar, lewati pemeriksaan ini:  SKIP_DB_CHECK=1 ./deploy.sh ..."
  die "Belum ada yang diubah di server."
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
  # Prioritas: --branch, lalu DEPLOY_BRANCH di deploy.env, lalu branch yang sedang aktif (deploy.env lama).
  target=${CLI_BRANCH:-${DEPLOY_BRANCH:-$current}}
  [ "$target" != "HEAD" ] || die "Repo dalam keadaan detached HEAD. Beri nama branch: ./deploy.sh <environment> --branch <nama>"

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

# git pull bisa ikut mengganti deploy.sh. Bash sudah membaca versi lama ke memori, jadi bagian skrip yang baru (mis. aturan alamat
# login) tidak berlaku di deploy ini dan hasilnya membingungkan. Karena sync_code berjalan sebelum ada yang dibangun atau diubah
# (hanya kode di disk yang berganti), skrip berhenti di sini dan meminta dijalankan ulang; container lama tidak tersentuh.
pastikan_skrip_terbaru() {
  [ -n "$SCRIPT_SHA_AWAL" ] || return 0
  local sekarang
  sekarang=$(sha256sum "$APP_DIR/$SCRIPT_NAME" 2>/dev/null | cut -d' ' -f1 || true)
  [ -n "$sekarang" ] && [ "$sekarang" != "$SCRIPT_SHA_AWAL" ] || return 0
  err "deploy.sh ikut diperbarui oleh git pull, sedangkan skrip yang sedang berjalan masih versi lama."
  err "Kode di server sudah terbaru dan container lama TIDAK diubah. Jalankan sekali lagi:  ./deploy.sh"
  exit 1
}

# ----------------------------------------------------------------------------
# Tahap 4-6: build, migrasi, jalankan, cek kesehatan
# ----------------------------------------------------------------------------
# Production butuh persetujuan eksplisit sebelum apa pun dibangun/diubah. Ditanyakan SETELAH kode
# ditarik dan .env divalidasi, supaya yang ditampilkan adalah persis apa yang akan dideploy.
confirm_production() {
  [ "$ENVIRONMENT" = "prod" ] || return 0
  step "Konfirmasi deploy PRODUCTION"
  log "Alamat    : $API_ROOT_URL"
  log "Kode      : $(g rev-parse --abbrev-ref HEAD) @ $(g rev-parse --short HEAD)"
  log "Database  : $(env_get "$BACKEND_ENV" DB_HOST) / $(env_get "$BACKEND_ENV" DB_NAME)"
  if [ "$NO_MIGRATE" -eq 0 ]; then
    warn "Migrasi database akan dijalankan di database production. Pastikan backup database sudah dibuat."
  fi
  if [ "$ASSUME_YES" -eq 1 ]; then
    log "--yes: konfirmasi production dilewati."
    return 0
  fi
  [ -t 0 ] || die "Deploy production membutuhkan konfirmasi interaktif (atau jalankan dengan --yes)."
  local ans
  read -r -p "    Ketik 'prod' untuk melanjutkan deploy PRODUCTION: " ans || ans=""
  [ "$ans" = "prod" ] || die "Dibatalkan. Tidak ada yang diubah di server."
}

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

# Alamat halaman login yang berlaku. Bawaannya ada di frontend/login-path.txt (satu sumber dengan next.config.ts untuk lokal dan
# build Docker); LOGIN_PATH di deploy.env (bila ada) menimpanya. Dibaca SETELAH sync_code supaya yang dipakai berasal dari kode
# yang akan dibangun. Diekspor agar docker compose menanamkan nilai yang sama dengan yang dipakai cek kesehatan dan ringkasan.
# "/login" berarti tidak disembunyikan. Bentuk lain harus sama dengan frontend/lib/loginPath.ts: satu segmen 16-128 karakter
# [A-Za-z0-9_=-], bukan nama rute aplikasi.
resolve_login_path() {
  step "Alamat halaman login"
  local berkas="$APP_DIR/frontend/login-path.txt" bawaan="" khusus
  # Versi deploy.sh sebelumnya membuat alamat acak sendiri di deploy.env (diberi tanda komentar di bawah). Nilai itu dibuang supaya
  # bawaan repo yang berlaku; alamat yang Anda tulis sendiri (tanpa tanda itu) tidak disentuh.
  local tanda='# Alamat halaman login (acak, tidak bisa ditebak); /login biasa dijawab 404. Dibuat otomatis oleh deploy.sh.' tmp
  if [ -f "$DEPLOY_ENV" ] && grep -qxF "$tanda" "$DEPLOY_ENV"; then
    tmp=$(mktemp)
    TANDA="$tanda" awk '
      BEGIN { skip = 0 }
      $0 == ENVIRON["TANDA"] { skip = 1; next }
      skip == 1 && index($0, "# Jangan dibagikan di tempat umum.") == 1 { next }
      skip == 1 && index($0, "LOGIN_PATH=") == 1 { skip = 0; next }
      { skip = 0; print }
    ' "$DEPLOY_ENV" > "$tmp"
    cat "$tmp" > "$DEPLOY_ENV"   # menimpa isi, mempertahankan izin & pemilik file
    rm -f "$tmp"
    log "Alamat login acak buatan deploy.sh versi sebelumnya dihapus dari deploy.env; memakai alamat bawaan repo."
  fi
  if [ -f "$berkas" ]; then bawaan=$(tr -d '[:space:]' < "$berkas"); fi
  khusus=$(env_get "$DEPLOY_ENV" LOGIN_PATH)
  khusus=${khusus//[[:space:]]/}
  if [ -n "$khusus" ]; then
    LOGIN_PATH=$khusus
    if [ -n "$bawaan" ] && [ "$khusus" != "$bawaan" ]; then
      warn "deploy.env memakai LOGIN_PATH sendiri ($khusus), bukan bawaan repo ($bawaan). Hapus baris LOGIN_PATH di deploy.env bila ingin memakai bawaan repo."
    fi
  else
    LOGIN_PATH=$bawaan
  fi
  if [ -z "$LOGIN_PATH" ]; then
    warn "frontend/login-path.txt tidak ditemukan dan LOGIN_PATH kosong: halaman login tetap di /login (tidak disembunyikan)."
    LOGIN_PATH="/login"
  fi
  if [ "$LOGIN_PATH" != "/login" ]; then
    if ! [[ $LOGIN_PATH =~ ^/[A-Za-z0-9_=-]{16,128}$ ]] || [[ ${LOGIN_PATH,,} =~ ^/(login|dashboard|kembali-masuk|halaman-tidak-ada|sso|api)$ ]]; then
      die "Alamat halaman login tidak valid: '$LOGIN_PATH' (harus satu segmen 16-128 karakter huruf/angka/_/-/= tanpa titik dan bukan nama rute aplikasi; periksa frontend/login-path.txt atau LOGIN_PATH di deploy.env)."
    fi
  fi
  export LOGIN_PATH
  ok "Halaman login: $LOGIN_PATH"
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

# Port host yang sedang didengarkan proses apa pun (ss bila ada, selain itu uji koneksi).
port_in_use() {
  if command -v ss >/dev/null 2>&1; then
    [ -n "$(ss -H -ltn "sport = :$1" 2>/dev/null)" ]
  else
    (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null
  fi
}

port_owner() {
  local owner=""
  if command -v ss >/dev/null 2>&1; then
    owner=$(ss -H -ltnp "sport = :$1" 2>/dev/null | awk '{print $NF}' | head -1)
  fi
  printf '%s' "${owner:-proses tidak dikenal}"
}

# Container dengan nama yang sama hanya boleh dimiliki folder ini. Bila dev dan prod berjalan di server
# yang sama, keduanya harus memakai CONTAINER_PREFIX berbeda; tanpa pemeriksaan ini, compose gagal
# dengan pesan "container name already in use" yang membingungkan.
check_containers() {
  local cname owner here
  here=$(readlink -f "$APP_DIR" 2>/dev/null || echo "$APP_DIR")
  for cname in "${CONTAINER_PREFIX}-backend" "${CONTAINER_PREFIX}-frontend"; do
    docker inspect "$cname" >/dev/null 2>&1 || continue          # belum ada: aman
    owner=$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$cname" 2>/dev/null || true)
    if [ -z "$owner" ]; then
      die "Container '$cname' sudah ada tetapi bukan dibuat oleh Docker Compose. Hapus dulu (docker rm -f $cname) atau pakai CONTAINER_PREFIX lain di deploy.env. Belum ada yang diubah."
    fi
    if [ "$(readlink -f "$owner" 2>/dev/null || echo "$owner")" != "$here" ]; then
      die "Container '$cname' dimiliki stack di folder lain ($owner), bukan folder ini ($APP_DIR). Bila dev dan prod berjalan di server yang sama, beri CONTAINER_PREFIX berbeda di deploy.env. Belum ada yang diubah."
    fi
  done
}

# Dijalankan SEBELUM mengubah apa pun. Mencegah dua kegagalan yang pernah terjadi di server:
#  1) port di deploy.env dipakai proses lain -> container lama dihapus lalu yang baru gagal start;
#  2) port di deploy.env berbeda dari port stack yang sedang berjalan -> pengguna kehilangan akses.
check_ports() {
  step "Memeriksa container dan port"
  check_containers
  local entry cname cport want label have
  for entry in "${CONTAINER_PREFIX}-backend:8686:${BACKEND_PORT}:Backend" "${CONTAINER_PREFIX}-frontend:3000:${FRONTEND_PORT}:Frontend"; do
    IFS=: read -r cname cport want label <<<"$entry"

    # Port host yang SEKARANG dipakai container ini (kosong bila belum ada / tidak berjalan).
    have=$(docker port "$cname" "$cport/tcp" 2>/dev/null | head -1 | sed 's/.*://' || true)

    if [ -n "$have" ] && [ "$have" != "$want" ]; then
      warn "$label sekarang berjalan di port $have, tetapi deploy.env mengatur port $want. Pengguna yang memakai port $have akan kehilangan akses."
      confirm "Tetap pindah ke port $want?" n \
        || die "Dibatalkan. Samakan port di deploy.env dengan yang sedang dipakai ($have), lalu jalankan ulang."
    fi

    if [ "$have" != "$want" ] && port_in_use "$want"; then
      die "Port $want ($label) sudah dipakai $(port_owner "$want"). Ubah port di deploy.env, atau hentikan proses itu. Belum ada yang diubah di server."
    fi
    ok "$label: port $want siap"
  done
}

wait_http() {  # wait_http NAMA CONTAINER URL [URL...]  (sehat bila salah satu URL menjawab 2xx)
  local name=$1 container=$2 elapsed=0 state url
  shift 2
  while [ "$elapsed" -lt "$HEALTH_TIMEOUT" ]; do
    state=$(container_state "$container")
    case "$state" in
      running)
        # Hanya sehat bila container KITA berjalan: kalau tidak, proses lain yang kebetulan
        # memakai port yang sama bisa membuat cek ini lolos secara keliru.
        for url in "$@"; do
          if curl -fsS -m 5 -o /dev/null "$url" 2>/dev/null; then
            ok "$name sehat ($url)"
            return 0
          fi
        done
        ;;
      restarting) err "$name restart berulang (crash-loop)."; return 1 ;;
      exited|dead|created|tidak-ada) err "$name tidak berjalan (status container $container: $state)."; return 1 ;;
    esac
    sleep 3
    elapsed=$((elapsed + 3))
  done
  err "$name tidak merespons dalam ${HEALTH_TIMEOUT} detik ($*)."
  return 1
}

health_checks() {
  step "Cek kesehatan"
  local host="127.0.0.1"
  case "$BIND_ADDRESS" in 0.0.0.0|127.0.0.1) ;; *) host="$BIND_ADDRESS" ;; esac
  wait_http "Backend"  "${CONTAINER_PREFIX}-backend"  "http://${host}:${BACKEND_PORT}/health" || return 1
  # Halaman login ada di LOGIN_PATH (/login biasa dijawab 404, lihat resolve_login_path). /login tetap diterima sebagai tanda sehat
  # agar rollback ke image lama (sebelum alamat login tersembunyi) tidak dianggap gagal.
  wait_http "Frontend" "${CONTAINER_PREFIX}-frontend" "http://${host}:${FRONTEND_PORT}${LOGIN_PATH}" "http://${host}:${FRONTEND_PORT}/login" || return 1
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
  # Dinyatakan berhasil hanya bila container naik DAN cek kesehatan lolos.
  if dc up -d --no-build --force-recreate && health_checks; then
    warn "Aplikasi dipulihkan ke versi sebelumnya (kode: $PREV_COMMIT)."
    warn "Catatan: migrasi database yang sudah diterapkan TIDAK ikut dibatalkan (hanya menambah tabel/kolom, aman untuk versi lama)."
    warn "Kode di folder server tetap versi terbaru (yang bermasalah). Perbaiki dulu, push, lalu jalankan ./deploy.sh lagi."
  else
    err "ROLLBACK JUGA GAGAL: aplikasi mungkin TIDAK berjalan sekarang."
    err "Periksa:  docker compose --env-file deploy.env ps    dan    docker logs ${CONTAINER_PREFIX}-frontend"
    err "Bila pesannya 'address already in use', ubah port di deploy.env lalu: docker compose --env-file deploy.env up -d"
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

    Environment: $(env_label)
    Kode      : $(g rev-parse --abbrev-ref HEAD) @ $(g rev-parse --short HEAD)
    Aplikasi  : ${frontend_url:-http://$(primary_ip):${FRONTEND_PORT}}
    Halaman login: ${frontend_url:-http://$(primary_ip):${FRONTEND_PORT}}${LOGIN_PATH}
    API       : ${API_URL}
    Log       : docker compose logs -f   (atau: tail -f $LOG_FILE)
    Berikutnya: cukup jalankan ./deploy.sh setiap kali ada pembaruan (kode ditarik otomatis).
EOF
  if [ "$LOGIN_PATH" != "/login" ]; then
    log "Alamat /login biasa dijawab 404; bagikan alamat halaman login di atas hanya kepada pengguna yang berhak."
  fi
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

  resolve_environment  # dev atau prod: dari argumen atau deploy.env
  setup_logging
  acquire_lock
  trap 'err "Gagal tak terduga di baris $LINENO: $BASH_COMMAND"' ERR
  step "Environment: $(env_label)"

  [ -d "$APP_DIR/.git" ] || die "$APP_DIR bukan repositori git."
  [ -f "$APP_DIR/docker-compose.yml" ] || die "docker-compose.yml tidak ditemukan di $APP_DIR."

  preflight_system
  load_deploy_config
  check_ports          # container & port, sebelum mengubah apa pun di server
  sync_code            # setelah ini kode di disk sudah terbaru
  pastikan_skrip_terbaru  # deploy.sh ikut berubah oleh git pull? hentikan dan minta jalankan ulang (belum ada yang diubah)
  resolve_login_path  # alamat halaman login (bawaan repo atau deploy.env), dari kode yang baru ditarik
  prepare_backend_env  # membuat/validasi backend/.env (bisa berhenti dengan instruksi)
  check_db_reachable   # server database terjangkau? (gagal cepat, sebelum build)
  confirm_production   # hanya untuk prod

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
