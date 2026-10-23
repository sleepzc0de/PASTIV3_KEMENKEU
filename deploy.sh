#!/usr/bin/env bash
# =============================================================================
# deploy.sh - Deploy PASTI V3 ke VPS Ubuntu dengan Docker Compose.
#
# Jalankan dari folder repo di VPS. Ada dua environment:
#
#   ./deploy.sh dev     # development  (branch bawaan: development)
#   ./deploy.sh prod    # production   (branch bawaan: production_v3; https://pasti.kemenkeu.go.id)
#   ./deploy.sh         # bertanya dev/prod (Enter = yang sedang berjalan); dengan --yes langsung yang sedang berjalan
#   ./deploy.sh --help
#
# Tiap environment punya profil konfigurasi sendiri: deploy.<env>.env dan backend/.env.<env>. Folder yang sama bisa
# berganti dev <-> prod (container dan port yang sama diganti); profil kedua dibuat otomatis dari profil pertama,
# tinggal mengisi database-nya. Untuk dev DAN prod berjalan bersamaan di server yang sama, clone repo ke dua folder.
#
# Yang dilakukan, berurutan:
#   1. Cek prasyarat (git, curl, Docker + Compose; menawarkan memasang Docker bila belum ada)
#   2. Siapkan konfigurasi environment terpilih: deploy.<env>.env (URL publik, port) dan backend/.env.<env> (rahasia & DB)
#   3. Tarik kode terbaru (git fetch + fast-forward)
#   4. Build image backend, lalu pastikan server database yang sebenarnya dituju adalah yang benar
#      (nama yang dilaporkan SQL Server sendiri vs EXPECTED_DB_SERVER; alamat DB_HOST bisa dipetakan infrastruktur)
#   5. Build image frontend
#   6. Terapkan migrasi database yang belum diterapkan (dicatat di tabel schema_migrations)
#   7. Jalankan ulang container, lalu cek kesehatannya
#   8. Bila gagal, kembalikan ke image (dan environment) sebelumnya secara otomatis
#
# Catatan: SQL Server TIDAK dipasang oleh skrip ini; skrip hanya terhubung ke
# server database yang diisi di backend/.env.<env>.
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
ATUR_KREDENSIAL=0   # --kredensial: tanyakan ulang semua kredensial environment ini

HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-90}"   # detik menunggu tiap layanan sehat; bisa diubah: HEALTH_TIMEOUT=180 ./deploy.sh
LOCK_FILE="/tmp/pasti-deploy.lock"

SCRIPT_PATH="${BASH_SOURCE[0]}"
APP_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
SCRIPT_NAME="$(basename "$SCRIPT_PATH")"
# Profil environment terpilih (deploy.<env>.env dan backend/.env.<env>); diisi resolve_environment.
DEPLOY_ENV=""
BACKEND_ENV=""
NAMA_DEPLOY_ENV=""      # nama berkas untuk pesan, mis. deploy.prod.env
NAMA_BACKEND_ENV=""     # mis. backend/.env.prod
# Format lama (satu folder = satu environment): dipindahkan menjadi profil environment-nya pada deploy pertama dengan skrip ini.
LEGACY_DEPLOY_ENV="$APP_DIR/deploy.env"
LEGACY_BACKEND_ENV="$APP_DIR/backend/.env"
# Environment yang terakhir berhasil dideploy dari folder ini, yaitu yang sedang berjalan.
ACTIVE_FILE="$APP_DIR/deploy.aktif"
PREV_ENV=""             # isi deploy.aktif sebelum deploy ini
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
                     Meminta konfirmasi dan memeriksa backend/.env.prod lebih ketat.
  (tanpa argumen)    bertanya dev/prod; Enter = environment yang sedang berjalan (tercatat di deploy.aktif).
                     Dengan --yes atau tanpa terminal: langsung environment yang sedang berjalan.

Tiap environment punya profil sendiri: deploy.dev.env + backend/.env.dev, dan deploy.prod.env +
backend/.env.prod. Memilih environment = memakai profilnya; APP_ENV, SSO_ENV, alamat, dan branch ikut.
Folder yang sama bisa berganti dev <-> prod: container dan port yang sama diganti (ditanyakan dulu).
Profil environment kedua dibuat otomatis dari profil pertama (rahasia, port, token ikut sama);
koneksi database-nya dikosongkan, jadi tinggal diisi. Untuk dev DAN prod berjalan bersamaan di
server yang sama, clone repo ke dua folder berbeda (mis. ~/pasti-dev dan ~/pasti-prod).

Opsi:
  --branch <nama> deploy dari branch ini, bukan branch bawaan environment
  --no-pull       jangan tarik kode; deploy kode yang ada di server apa adanya
  --no-migrate    lewati migrasi database
  --fresh         build tanpa cache dan tarik base image terbaru (lebih lambat)
  --force         buang perubahan lokal pada file yang dilacak git di server
                  (git reset --hard origin/<branch>). Hati-hati.
  -y, --yes       jawab "ya" otomatis pada konfirmasi (termasuk konfirmasi deploy prod)
  --kredensial    tanyakan ulang kredensial environment ini di terminal: database, SSO (client ID
                  dan secret), SLDK, dan token Inaproc. Enter = tetap. Tanpa opsi ini, hanya yang
                  kosong yang ditanyakan (profil baru: semuanya).
  -h, --help      tampilkan bantuan ini

Konfigurasi (dibuat otomatis saat environment itu pertama kali dipilih):
  deploy.<env>.env
                  environment, branch, URL publik backend, port, dan awalan nama container.
                  (deploy.env format lama dipindah otomatis menjadi deploy.<env>.env.)
                  Contoh isi deploy.prod.env:
                    ENVIRONMENT=prod
                    DEPLOY_BRANCH=production_v3
                    CONTAINER_PREFIX=pasti-prod
                    NEXT_PUBLIC_API_ROOT_URL=https://pasti.kemenkeu.go.id
                    NEXT_PUBLIC_API_URL=https://pasti.kemenkeu.go.id/api/v1
                    BIND_ADDRESS=127.0.0.1     # hanya lewat Nginx/reverse proxy di server ini
                    BACKEND_PORT=8686
                    FRONTEND_PORT=3000
                    EXPECTED_DB_SERVER=NAMA-SERVER-DB   # nama server database production menurut SQL Server sendiri
                    BACKEND_ENV_FILE=./backend/.env.prod   # diisi otomatis
                    # LOGIN_PATH=/...   (opsional) menimpa alamat halaman login bawaan repo (frontend/login-path.txt)
                    # SSO_CALLBACK_VIA_FRONTEND=1   (opsional, jalan pintas) frontend meneruskan /sso/callback/login ke backend
                  SSO_CALLBACK_VIA_FRONTEND=1: hanya bila proxy di depan domain mengarahkan /sso/callback/login ke frontend
                  (login SSO berakhir di halaman 404) dan belum bisa diperbaiki. Perbaikan yang benar ada di aturan proxy;
                  mengubah nilai ini memerlukan build ulang frontend (./deploy.sh melakukannya).
                  EXPECTED_DB_SERVER: jati diri server database yang benar (hasil SELECT SERVERPROPERTY('ServerName'),
                  bukan IP). Sebelum migrasi, deploy.sh menanyakan nama server yang sebenarnya tersambung dan berhenti
                  bila berbeda, supaya alamat DB_HOST yang dipetakan (IP masking) ke database environment lain tidak
                  ikut termigrasi. Production wajib: bila kosong, deploy interaktif meminta Anda mengetik namanya
                  untuk dikunci. EXPECTED_DB_NAME (opsional) mengunci nama database. Tiap environment
                  menguncinya sendiri.
  backend/.env.<env>
                  rahasia aplikasi, koneksi database, SSO, token Inaproc (dibuat dari
                  backend/.env.example, atau disalin dari profil environment lain; Anda yang mengisi
                  nilai database, dan SSO production untuk prod).

Variabel lingkungan:
  HEALTH_TIMEOUT=180   detik menunggu tiap layanan sehat (bawaan 90)
  SKIP_DB_CHECK=1      lewati pemeriksaan keterjangkauan server database sebelum build
  SKIP_DB_IDENTITY_CHECK=1
                       lewati pemeriksaan jati diri server database (EXPECTED_DB_SERVER). Hanya untuk keadaan
                       darurat; pastikan sendiri bahwa DB_HOST tidak menuju database environment lain

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
      --kredensial) ATUR_KREDENSIAL=1 ;;
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
  exec 7>&1 8>&2   # keluaran asli, dipulihkan sebelum skrip menjalankan dirinya ulang (pastikan_skrip_terbaru)
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
env_label() { if [ "${1:-$ENVIRONMENT}" = "prod" ]; then printf 'PRODUCTION'; else printf 'DEVELOPMENT'; fi; }
env_lain() { if [ "$1" = "prod" ]; then printf 'dev'; else printf 'prod'; fi; }
branch_bawaan() { if [ "$1" = "prod" ]; then printf 'production_v3'; else printf 'development'; fi; }
profil_deploy() { printf '%s/deploy.%s.env' "$APP_DIR" "$1"; }
profil_backend() { printf '%s/backend/.env.%s' "$APP_DIR" "$1"; }

# deploy.sh versi sebelumnya: satu folder = satu environment, dengan deploy.env dan backend/.env. Keduanya dipindah menjadi profil environment
# itu (deploy.<env>.env dan backend/.env.<env>) supaya folder yang sama bisa berganti dev/prod. Isinya tidak diubah selain BACKEND_ENV_FILE.
pindahkan_konfigurasi_lama() {
  [ -f "$LEGACY_DEPLOY_ENV" ] || return 0
  local env tujuan be
  env=$(env_get "$LEGACY_DEPLOY_ENV" ENVIRONMENT)
  env=${env:-dev}   # deploy.env yang dibuat sebelum ada pemisahan environment = development
  case "$env" in dev|prod) ;; *) die "ENVIRONMENT di deploy.env tidak valid: '$env' (harus dev atau prod)." ;; esac
  tujuan=$(profil_deploy "$env")
  be=$(profil_backend "$env")
  if [ -f "$tujuan" ]; then
    warn "deploy.env (format lama) diabaikan karena $(basename "$tujuan") sudah ada. Hapus deploy.env bila sudah tidak dipakai."
    return 0
  fi
  step "Memindahkan konfigurasi format lama menjadi profil $(env_label "$env")"
  mv "$LEGACY_DEPLOY_ENV" "$tujuan"
  env_set "$tujuan" ENVIRONMENT "$env"
  env_set "$tujuan" BACKEND_ENV_FILE "./backend/.env.$env"
  ok "deploy.env -> $(basename "$tujuan")"
  if [ -f "$LEGACY_BACKEND_ENV" ] && [ ! -f "$be" ]; then
    mv "$LEGACY_BACKEND_ENV" "$be"
    ok "backend/.env -> backend/$(basename "$be")"
  fi
  # Container yang sedang berjalan (bila ada) dibangun dari konfigurasi ini.
  if [ ! -f "$ACTIVE_FILE" ]; then
    printf '%s\n' "$env" > "$ACTIVE_FILE"
    own_file "$ACTIVE_FILE"
  fi
}

# Tentukan environment: dari argumen; tanpa argumen, tanya (Enter = yang sedang berjalan), atau langsung yang sedang berjalan bila
# --yes / tanpa terminal. Profil yang dipakai mengikuti environment ini.
resolve_environment() {
  local terakhir="" ans tries=0
  [ -f "$ACTIVE_FILE" ] && terakhir=$(tr -d '[:space:]' < "$ACTIVE_FILE")
  case "$terakhir" in dev|prod) ;; *) terakhir="" ;; esac
  PREV_ENV=$terakhir
  ENVIRONMENT=$CLI_ENV
  if [ -z "$ENVIRONMENT" ] && [ "$ASSUME_YES" -eq 0 ] && [ -t 0 ]; then
    while [ -z "$ENVIRONMENT" ] && [ "$tries" -lt 3 ]; do
      tries=$((tries + 1))
      read -r -p "    Environment yang akan dideploy [dev/prod]${terakhir:+ (Enter = $terakhir, yang sedang berjalan)}: " ans || ans=""
      ans=${ans:-$terakhir}
      case "$ans" in
        dev|development) ENVIRONMENT=dev ;;
        prod|production) ENVIRONMENT=prod ;;
        *) err "Ketik 'dev' atau 'prod'." ;;
      esac
    done
    [ -n "$ENVIRONMENT" ] || die "Environment belum dipilih."
  fi
  ENVIRONMENT=${ENVIRONMENT:-$terakhir}
  [ -n "$ENVIRONMENT" ] || die "Sebutkan environment: ./deploy.sh dev  atau  ./deploy.sh prod"
  DEPLOY_ENV=$(profil_deploy "$ENVIRONMENT")
  BACKEND_ENV=$(profil_backend "$ENVIRONMENT")
  NAMA_DEPLOY_ENV=$(basename "$DEPLOY_ENV")
  NAMA_BACKEND_ENV="backend/$(basename "$BACKEND_ENV")"
}

run_wizard() {
  step "Pengaturan awal $(env_label) (hanya sekali; disimpan di $NAMA_DEPLOY_ENV)"
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
# Dibuat oleh deploy.sh pada $(date '+%F %T'). Aman diedit; jalankan ./deploy.sh $ENVIRONMENT lagi setelah mengubahnya.
# Profil environment $ENVIRONMENT: dipakai setiap kali $ENVIRONMENT dipilih. ENVIRONMENT harus sama dengan nama berkas.
ENVIRONMENT=$ENVIRONMENT
# Berkas konfigurasi backend environment ini (dibaca docker compose). Diisi otomatis.
BACKEND_ENV_FILE=./backend/.env.$ENVIRONMENT
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
# Jati diri SERVER DATABASE yang benar untuk environment ini, menurut SQL Server sendiri (SELECT SERVERPROPERTY('ServerName'); bukan IP). Sebelum migrasi, deploy.sh
# membandingkannya dengan server yang benar-benar tersambung, supaya DB_HOST yang dipetakan infrastruktur (IP masking) ke database environment lain tidak ikut termigrasi.
# Production wajib: bila kosong, deploy production pertama meminta nama ini diketik lalu menyimpannya di sini. EXPECTED_DB_NAME (opsional) mengunci nama database.
EXPECTED_DB_SERVER=
# Jalan pintas (opsional): hapus tanda # dan isi 1 hanya bila proxy di depan domain mengarahkan /sso/callback/login ke frontend sehingga login SSO berakhir di halaman 404.
# SSO_CALLBACK_VIA_FRONTEND=1
EOF
  )
  own_file "$DEPLOY_ENV"
  ok "$NAMA_DEPLOY_ENV dibuat untuk environment $(env_label)."
}

# Profil environment kedua di folder yang sama: disalin dari profil environment lain, tanpa bertanya. Container, port, bind, alamat login, dan
# jalan pintas SSO tetap sama (satu folder = satu stack yang berganti environment; proxy/masking infra mengarah ke port yang sama). Yang khas
# environment diganti: branch, alamat publik, dan kunci server database (database environment ini dikunci ulang).
buat_profil_deploy_dari_lain() {
  local sumber root port
  sumber=$(profil_deploy "$(env_lain "$ENVIRONMENT")")
  step "Membuat $NAMA_DEPLOY_ENV dari $(basename "$sumber")"
  if [ "$ENVIRONMENT" = "prod" ]; then
    root="https://pasti.kemenkeu.go.id"
  else
    port=$(env_get "$sumber" BACKEND_PORT)
    root="http://$(primary_ip):${port:-8686}"
  fi
  (
    umask 077
    cp "$sumber" "$DEPLOY_ENV"
  )
  env_set "$DEPLOY_ENV" ENVIRONMENT "$ENVIRONMENT"
  env_set "$DEPLOY_ENV" BACKEND_ENV_FILE "./backend/.env.$ENVIRONMENT"
  env_set "$DEPLOY_ENV" DEPLOY_BRANCH "$(branch_bawaan "$ENVIRONMENT")"
  env_set "$DEPLOY_ENV" NEXT_PUBLIC_API_ROOT_URL "$root"
  env_set "$DEPLOY_ENV" NEXT_PUBLIC_API_URL "$root/api/v1"
  env_set "$DEPLOY_ENV" EXPECTED_DB_SERVER ""
  if grep -qE '^[[:space:]]*EXPECTED_DB_NAME=' "$DEPLOY_ENV"; then env_set "$DEPLOY_ENV" EXPECTED_DB_NAME ""; fi
  own_file "$DEPLOY_ENV"
  ok "$NAMA_DEPLOY_ENV dibuat: branch $(branch_bawaan "$ENVIRONMENT"), alamat $root; container, port, dan bind sama dengan $(basename "$sumber")."
  log "Bila alamat atau branch-nya lain, ubah $NAMA_DEPLOY_ENV lalu jalankan ulang."
}

load_deploy_config() {
  step "Membaca konfigurasi deploy ($(env_label): $NAMA_DEPLOY_ENV)"
  if [ ! -f "$DEPLOY_ENV" ]; then
    if [ -f "$(profil_deploy "$(env_lain "$ENVIRONMENT")")" ]; then
      buat_profil_deploy_dari_lain
    else
      [ -t 0 ] || die "$NAMA_DEPLOY_ENV belum ada dan terminal tidak interaktif. Buat dulu manual (contoh isi: ./deploy.sh --help)."
      run_wizard
    fi
  fi

  local file_env
  file_env=$(env_get "$DEPLOY_ENV" ENVIRONMENT)
  [ -z "$file_env" ] || [ "$file_env" = "$ENVIRONMENT" ] \
    || die "ENVIRONMENT di $NAMA_DEPLOY_ENV adalah '$file_env', bukan '$ENVIRONMENT'. Profil $ENVIRONMENT harus berisi ENVIRONMENT=$ENVIRONMENT."
  # Docker compose membaca berkas backend dari BACKEND_ENV_FILE; selalu profil environment ini.
  if [ -z "$file_env" ] || [ "$(env_get "$DEPLOY_ENV" BACKEND_ENV_FILE)" != "./backend/.env.$ENVIRONMENT" ]; then
    env_set "$DEPLOY_ENV" ENVIRONMENT "$ENVIRONMENT"
    env_set "$DEPLOY_ENV" BACKEND_ENV_FILE "./backend/.env.$ENVIRONMENT"
    own_file "$DEPLOY_ENV"
  fi

  API_ROOT_URL=$(env_get "$DEPLOY_ENV" NEXT_PUBLIC_API_ROOT_URL)
  API_URL=$(env_get "$DEPLOY_ENV" NEXT_PUBLIC_API_URL)
  BIND_ADDRESS=$(env_get "$DEPLOY_ENV" BIND_ADDRESS); BIND_ADDRESS=${BIND_ADDRESS:-0.0.0.0}
  BACKEND_PORT=$(env_get "$DEPLOY_ENV" BACKEND_PORT); BACKEND_PORT=${BACKEND_PORT:-8686}
  FRONTEND_PORT=$(env_get "$DEPLOY_ENV" FRONTEND_PORT); FRONTEND_PORT=${FRONTEND_PORT:-3000}
  DEPLOY_BRANCH=$(env_get "$DEPLOY_ENV" DEPLOY_BRANCH)   # kosong (profil lama tanpa DEPLOY_BRANCH) = branch yang sedang aktif
  CONTAINER_PREFIX=$(env_get "$DEPLOY_ENV" CONTAINER_PREFIX)
  if [ -z "$CONTAINER_PREFIX" ]; then
    if [ "$ENVIRONMENT" = "prod" ]; then CONTAINER_PREFIX="pasti-prod"; else CONTAINER_PREFIX="pasti"; fi
  fi

  valid_url "$API_ROOT_URL" || die "NEXT_PUBLIC_API_ROOT_URL di ${NAMA_DEPLOY_ENV} tidak valid: '$API_ROOT_URL'"
  valid_api_url "$API_URL"  || die "NEXT_PUBLIC_API_URL di ${NAMA_DEPLOY_ENV} tidak valid: '$API_URL'"
  valid_port "$BACKEND_PORT"  || die "BACKEND_PORT di ${NAMA_DEPLOY_ENV} tidak valid: '$BACKEND_PORT'"
  valid_port "$FRONTEND_PORT" || die "FRONTEND_PORT di ${NAMA_DEPLOY_ENV} tidak valid: '$FRONTEND_PORT'"
  [[ $CONTAINER_PREFIX =~ ^[a-z0-9][a-z0-9_-]*$ ]] || die "CONTAINER_PREFIX di ${NAMA_DEPLOY_ENV} tidak valid: '$CONTAINER_PREFIX' (huruf kecil, angka, - dan _)"
  SSO_VIA_FRONTEND=$(env_get "$DEPLOY_ENV" SSO_CALLBACK_VIA_FRONTEND)
  SSO_VIA_FRONTEND=${SSO_VIA_FRONTEND//[[:space:]]/}
  case "$SSO_VIA_FRONTEND" in
    ""|0|1) ;;
    *) die "SSO_CALLBACK_VIA_FRONTEND di ${NAMA_DEPLOY_ENV} harus 1 (menyalakan) atau 0/kosong (mematikan), bukan '$SSO_VIA_FRONTEND'." ;;
  esac
  [ "$API_URL" = "$API_ROOT_URL/api/v1" ] || warn "NEXT_PUBLIC_API_URL ($API_URL) tidak sama dengan NEXT_PUBLIC_API_ROOT_URL + /api/v1."
  if [ "$ENVIRONMENT" = "prod" ] && [[ $API_ROOT_URL != https://* ]]; then
    die "Environment production wajib memakai https:// (NEXT_PUBLIC_API_ROOT_URL sekarang: $API_ROOT_URL)."
  fi

  ok "Environment: $(env_label)"
  ok "API publik: $API_URL"
  ok "Port: backend $BACKEND_PORT, frontend $FRONTEND_PORT (bind $BIND_ADDRESS); container: ${CONTAINER_PREFIX}-backend / ${CONTAINER_PREFIX}-frontend"
  if [ "$SSO_VIA_FRONTEND" = "1" ]; then
    warn "SSO_CALLBACK_VIA_FRONTEND=1: frontend meneruskan /sso/callback/login ke backend (jalan pintas). Tetap minta tim infrastruktur meneruskan path itu ke backend (port $BACKEND_PORT) di proxy."
  fi
}

app_env_bawaan() { if [ "$ENVIRONMENT" = "prod" ]; then printf 'production'; else printf 'development'; fi; }

# Contoh FRONTEND_URL: bila alamat backend memakai port (akses langsung), frontend ada di host
# yang sama dengan port frontend; bila tanpa port (domain/Nginx), alamatnya sama.
frontend_url_bawaan() {
  local scheme host_port host
  scheme=${API_ROOT_URL%%://*}
  host_port=${API_ROOT_URL#*://}
  host=${host_port%%:*}
  if [[ $host_port == *:* ]]; then printf '%s://%s:%s' "$scheme" "$host" "$FRONTEND_PORT"; else printf '%s' "$API_ROOT_URL"; fi
}

# Nilai yang ditentukan oleh environment dan alamat di deploy.<env>.env: APP_ENV, SSO_ENV, FRONTEND_URL, dan SSO_REDIRECT_URI (callback SSO).
# Disamakan pada setiap deploy, jadi tidak perlu diubah tangan saat berganti dev/prod atau mengubah alamat. "diam" = tanpa laporan (profil baru).
isi_nilai_environment() {
  local key mau lama
  for key in APP_ENV SSO_ENV FRONTEND_URL SSO_REDIRECT_URI; do
    case "$key" in
      APP_ENV|SSO_ENV)  mau=$(app_env_bawaan) ;;
      FRONTEND_URL)     mau=$(frontend_url_bawaan) ;;
      SSO_REDIRECT_URI) mau="$API_ROOT_URL/sso/callback/login" ;;
    esac
    lama=$(env_get "$BACKEND_ENV" "$key")
    [ "$lama" != "$mau" ] || continue
    env_set "$BACKEND_ENV" "$key" "$mau"
    [ "${1:-}" != "diam" ] || continue
    log "$key disesuaikan dengan $(env_label): ${lama:--} -> $mau"
    [ "$key" != "SSO_REDIRECT_URI" ] || warn "Pastikan $mau terdaftar sebagai Redirect URI client SSO $(env_label) di SSO Kemenkeu."
  done
}

create_backend_env() {
  [ -f "$BACKEND_ENV_EXAMPLE" ] || die "backend/.env.example tidak ditemukan."
  step "Membuat ${NAMA_BACKEND_ENV} dari contoh"

  local app_env sso_env
  app_env=$(app_env_bawaan); sso_env=$app_env
  (
    umask 077
    cp "$BACKEND_ENV_EXAMPLE" "$BACKEND_ENV"
    # URL yang sudah diketahui dari deploy.<env>.env langsung diisi.
    isi_nilai_environment diam
    env_set "$BACKEND_ENV" JWT_SECRET "$(gen_hex 48)"
    env_set "$BACKEND_ENV" PASSWORD_PEPPER "$(gen_hex 48)"
    {
      printf '\n# ============ Ditambahkan otomatis oleh deploy.sh ============\n'
      printf '# Kunci enkripsi token SSO: base64 dari TEPAT 32 byte. JANGAN diganti setelah dipakai.\n'
      printf 'TOKEN_ENCRYPTION_KEY=%s\n' "$(gen_key_b64)"
      printf '# Integrasi Inaproc\nINAPROC_BASE_URL=https://data.inaproc.id\nINAPROC_TOKEN=\n'
      printf '# Integrasi SLDK untuk Digitalisasi Aset (opsional; kosongkan SLDK_DB_HOST untuk menonaktifkan)\n'
      printf 'SLDK_DB_HOST=\nSLDK_DB_PORT=1433\nSLDK_DB_USER=\nSLDK_DB_PASSWORD=\nSLDK_DB_NAME=\n'
      printf '# Peran data (Pengguna Barang, UE1, Kanwil, Satker): true = pengguna yang belum diberi peran tidak melihat data apa pun. Nyalakan setelah semua pengguna diberi peran.\nPERAN_DATA_WAJIB=false\n'
    } >> "$BACKEND_ENV"
  )
  own_file "$BACKEND_ENV"
  ok "${NAMA_BACKEND_ENV} dibuat untuk $(env_label); JWT_SECRET, PASSWORD_PEPPER, dan TOKEN_ENCRYPTION_KEY sudah diisi acak."
  ok "Sudah terisi otomatis: APP_ENV=$app_env, SSO_ENV=$sso_env, FRONTEND_URL, SSO_REDIRECT_URI."
  # Di terminal, kredensialnya langsung ditanyakan (isi_kredensial); tanpa terminal, berhenti dengan petunjuk.
  terminal_interaktif && return 0

  cat <<EOF

    ${NAMA_BACKEND_ENV} baru dibuat dan BELUM siap dipakai. Jalankan ./deploy.sh ${ENVIRONMENT} langsung di terminal server
    supaya kredensialnya ditanyakan, atau isi dulu nilai berikut lalu jalankan ulang ./deploy.sh ${ENVIRONMENT}:

      nano ${NAMA_BACKEND_ENV}

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
  step "Memeriksa ${NAMA_BACKEND_ENV} ($(env_label))"
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
    || flag_issue "SSO_REDIRECT_URI sebaiknya $API_ROOT_URL/sso/callback/login (sesuai alamat di ${NAMA_DEPLOY_ENV})."

  [ "$(env_get "$BACKEND_ENV" DB_USER)" != "sa" ] \
    || warns+=("DB_USER=sa (administrator penuh SQL Server). Sebaiknya pakai login khusus aplikasi dengan hak seperlunya.")

  # Compose menginterpolasi tanda $ pada nilai env_file (kecuali diapit kutip tunggal);
  # kata sandi berisi $ bisa terpotong. Hanya nama key yang ditampilkan, bukan nilainya.
  local dollar_keys
  dollar_keys=$(grep -E "^[A-Za-z_][A-Za-z0-9_]*=[^'].*[$]" "$BACKEND_ENV" | cut -d= -f1 | tr '\n' ' ' || true)
  [ -z "$dollar_keys" ] || warns+=("Nilai berisi tanda \$ pada: ${dollar_keys}. Compose bisa menafsirkannya sebagai variabel; bungkus nilainya dengan tanda kutip tunggal '...' atau tulis \$\$.")

  local perm
  perm=$(stat -c '%a' "$BACKEND_ENV" 2>/dev/null || echo "")
  case "$perm" in 600|400|"") ;; *) warns+=("Izin ${NAMA_BACKEND_ENV} $perm; disarankan 600 (chmod 600 ${NAMA_BACKEND_ENV}).") ;; esac

  local w
  for w in "${warns[@]+"${warns[@]}"}"; do warn "$w"; done
  if [ ${#errors[@]} -gt 0 ]; then
    for w in "${errors[@]}"; do err "$w"; done
    die "${NAMA_BACKEND_ENV} belum benar. Perbaiki (nano ${NAMA_BACKEND_ENV}, atau di terminal: ./deploy.sh ${ENVIRONMENT} --kredensial) lalu jalankan ulang."
  fi
  ok "${NAMA_BACKEND_ENV} lengkap."
}

# Profil backend environment kedua: disalin dari profil environment lain. Rahasia (JWT_SECRET, PASSWORD_PEPPER, TOKEN_ENCRYPTION_KEY), token Inaproc,
# superadmin, dan pengaturan lain ikut sama, sehingga data (sandi, kunci API, token SSO) tetap terbaca bila kedua environment ternyata memakai
# database yang sama (IP masking). Yang ditentukan environment diisi ulang; kredensial yang khas environment (database, SSO, SLDK) dikosongkan
# supaya ditanyakan dan dipilih dengan sadar.
buat_profil_backend_dari_lain() {
  local sumber nama_sumber key
  sumber=$(profil_backend "$(env_lain "$ENVIRONMENT")")
  nama_sumber="backend/$(basename "$sumber")"
  step "Membuat ${NAMA_BACKEND_ENV} dari ${nama_sumber}"
  (
    umask 077
    cp "$sumber" "$BACKEND_ENV"
  )
  isi_nilai_environment diam
  for key in DB_HOST DB_USER DB_PASSWORD DB_NAME SLDK_DB_HOST SLDK_DB_USER SLDK_DB_PASSWORD SLDK_DB_NAME; do env_set "$BACKEND_ENV" "$key" ""; done
  # Kredensial SSO terikat ke SSO_ENV: development memakai client SSO demo (nilai contoh di repo) sebagai bawaan; production diisi sendiri.
  if [ "$ENVIRONMENT" = "dev" ]; then
    env_set "$BACKEND_ENV" SSO_CLIENT_ID "$(env_get "$BACKEND_ENV_EXAMPLE" SSO_CLIENT_ID)"
    env_set "$BACKEND_ENV" SSO_CLIENT_SECRET "$(env_get "$BACKEND_ENV_EXAMPLE" SSO_CLIENT_SECRET)"
  else
    env_set "$BACKEND_ENV" SSO_CLIENT_ID ""
    env_set "$BACKEND_ENV" SSO_CLIENT_SECRET ""
  fi
  own_file "$BACKEND_ENV"
  ok "${NAMA_BACKEND_ENV} dibuat dari ${nama_sumber}: rahasia, token, dan pengaturan lain sama; APP_ENV/SSO_ENV=$(app_env_bawaan), FRONTEND_URL, SSO_REDIRECT_URI menyesuaikan."
  # Di terminal, kredensialnya langsung ditanyakan (isi_kredensial); tanpa terminal, berhenti dengan petunjuk.
  terminal_interaktif && return 0

  cat <<EOF

    Kredensial $(env_label) belum diisi. Jalankan ./deploy.sh ${ENVIRONMENT} langsung di terminal server supaya ditanyakan,
    atau isi di ${NAMA_BACKEND_ENV} (nano ${NAMA_BACKEND_ENV}) lalu jalankan ulang ./deploy.sh ${ENVIRONMENT}:

      DB_HOST / DB_PORT / DB_USER / DB_PASSWORD / DB_NAME   (dikosongkan; DB_PORT ikut ${nama_sumber})
      SLDK_DB_HOST / SLDK_DB_USER / SLDK_DB_PASSWORD / SLDK_DB_NAME   (opsional; kosong = sinkronisasi SLDK nonaktif)
EOF
  if [ "$ENVIRONMENT" = "prod" ]; then
    printf '      SSO_CLIENT_ID / SSO_CLIENT_SECRET   kredensial SSO Kemenkeu PRODUCTION\n'
    [ -n "$(env_get "$BACKEND_ENV" INAPROC_TOKEN)" ] || printf '      INAPROC_TOKEN   wajib di production\n'
  fi
  echo
  exit 1
}

# ----------------------------------------------------------------------------
# Kredensial per environment: database aplikasi, SSO (client ID dan secret), SLDK, dan token Inaproc.
#
# Ditanyakan di terminal lalu disimpan di backend/.env.<env>. Yang kosong selalu ditanyakan; profil yang baru dibuat dan --kredensial menanyakan
# semuanya (Enter = nilai sekarang). Sandi, secret, dan token tidak ditampilkan saat diketik sehingga juga tidak tercatat di deploy.log.
# ----------------------------------------------------------------------------

# Nilai untuk berkas .env: apa adanya bila hanya berisi karakter aman; selain itu diapit kutip tunggal supaya docker compose tidak menafsirkan
# $, #, atau spasi di dalamnya (env_get membuang kutip itu lagi).
nilai_env() {
  if [[ $1 =~ ^[A-Za-z0-9._@:/+=,-]*$ ]]; then printf '%s' "$1"; else printf "'%s'" "$1"; fi
}

# kredensial_kosong KUNCI... -> status 0 bila salah satunya kosong atau masih nilai contoh.
kredensial_kosong() {
  local key val
  for key in "$@"; do
    val=$(env_get "$BACKEND_ENV" "$key")
    if [ -z "$val" ] || is_placeholder "$val"; then return 0; fi
  done
  return 1
}

# tanya_nilai KUNCI "label" JENIS WAJIB  (JENIS: teks | rahasia | host | port; WAJIB: 1 atau 0)
# Enter = nilai sekarang; "-" mengosongkan nilai yang tidak wajib. Langsung ditulis ke backend/.env.<env>.
tanya_nilai() {
  local key=$1 label=$2 jenis=$3 wajib=$4 asli lama baru petunjuk salah tries=0
  asli=$(env_get "$BACKEND_ENV" "$key")
  lama=$asli
  is_placeholder "$lama" && lama=""
  # Secret client SSO demo (contoh di repo) tidak ditawarkan sebagai nilai bawaan di production.
  if [ "$ENVIRONMENT" = "prod" ] && [ "$key" = "SSO_CLIENT_SECRET" ] && [ "$lama" = "$(env_get "$BACKEND_ENV_EXAMPLE" SSO_CLIENT_SECRET)" ]; then lama=""; fi
  [ "$jenis" != "port" ] || [ -n "$lama" ] || lama=1433
  while :; do
    tries=$((tries + 1))
    [ "$tries" -le 3 ] || die "$key belum diisi dengan benar. Container yang berjalan tidak diubah; jalankan ulang ./deploy.sh $ENVIRONMENT."
    petunjuk=""
    if [ "$jenis" = "rahasia" ]; then
      [ -z "$lama" ] || petunjuk=" (Enter = tetap)"
      [ "$wajib" -eq 1 ] || [ -z "$lama" ] || petunjuk=" (Enter = tetap, - = kosongkan)"
      IFS= read -r -s -p "      $label$petunjuk: " baru || baru=""
      printf '\n'
    else
      [ -z "$lama" ] || petunjuk=" [$lama]"
      [ "$wajib" -eq 1 ] || [ -z "$lama" ] || petunjuk="$petunjuk (- = kosongkan)"
      read -r -p "      $label$petunjuk: " baru || baru=""
    fi
    baru=${baru:-$lama}
    if [ "$wajib" -eq 0 ] && [ "$baru" = "-" ]; then baru=""; fi
    salah=""
    if [ -z "$baru" ]; then
      [ "$wajib" -eq 0 ] || salah="wajib diisi."
    elif [[ $baru == *"'"* ]]; then
      salah="tanda kutip tunggal (') tidak didukung di berkas .env."
    elif [ "$jenis" = "port" ] && ! valid_port "$baru"; then
      salah="port tidak valid (1-65535)."
    elif [ "$jenis" = "host" ] && ! [[ $baru =~ ^[A-Za-z0-9._-]+$ ]]; then
      salah="tulis IP atau nama host saja (tanpa port, spasi, atau http://)."
    elif [ "$jenis" = "host" ] && [[ $baru =~ ^(localhost|127\.0\.0\.1)$ ]]; then
      salah="$baru di dalam container berarti container itu sendiri; pakai IP server database (atau host.docker.internal untuk server ini)."
    fi
    if [ -n "$salah" ]; then err "$label: $salah"; continue; fi
    break
  done
  [ "$baru" = "$asli" ] || env_set "$BACKEND_ENV" "$key" "$(nilai_env "$baru")"
}

isi_kredensial() {  # isi_kredensial 1 = profil baru (tanyakan semuanya)
  local semua=${1:-0} db=0 sso=0 sldk=0 inaproc=0 alamat_lama nama_lama
  [ "$ATUR_KREDENSIAL" -eq 0 ] || semua=1
  if [ "$semua" -eq 1 ]; then db=1; sso=1; sldk=1; inaproc=1; fi
  if kredensial_kosong DB_HOST DB_USER DB_PASSWORD DB_NAME; then db=1; fi
  if kredensial_kosong SSO_CLIENT_ID SSO_CLIENT_SECRET; then sso=1; fi
  if [ "$ENVIRONMENT" = "prod" ] && [ "$(env_get "$BACKEND_ENV" SSO_CLIENT_SECRET)" = "$(env_get "$BACKEND_ENV_EXAMPLE" SSO_CLIENT_SECRET)" ]; then sso=1; fi
  if [ -n "$(env_get "$BACKEND_ENV" SLDK_DB_HOST)" ] && kredensial_kosong SLDK_DB_USER SLDK_DB_PASSWORD SLDK_DB_NAME; then sldk=1; fi
  if [ "$ENVIRONMENT" = "prod" ] && kredensial_kosong INAPROC_TOKEN; then inaproc=1; fi
  [ $((db + sso + sldk + inaproc)) -gt 0 ] || return 0
  if ! terminal_interaktif; then
    [ "$ATUR_KREDENSIAL" -eq 0 ] || die "--kredensial membutuhkan terminal interaktif (jalankan langsung di terminal server, tanpa pipe)."
    return 0   # yang kurang disebutkan validate_backend_env
  fi

  step "Kredensial $(env_label) (disimpan di ${NAMA_BACKEND_ENV})"
  log "Enter = memakai nilai dalam [kurung]; sandi, secret, dan token tidak ditampilkan saat diketik."
  alamat_lama="$(env_get "$BACKEND_ENV" DB_HOST):$(env_get "$BACKEND_ENV" DB_PORT)"
  nama_lama=$(env_get "$BACKEND_ENV" DB_NAME)
  if [ "$db" -eq 1 ]; then
    log "Database aplikasi (SQL Server $(env_label)):"
    tanya_nilai DB_HOST "Alamat server database (IP/host)" host 1
    tanya_nilai DB_PORT "Port" port 1
    tanya_nilai DB_NAME "Nama database" teks 1
    tanya_nilai DB_USER "Login" teks 1
    tanya_nilai DB_PASSWORD "Sandi" rahasia 1
  fi
  if [ "$sso" -eq 1 ]; then
    log "SSO Kemenkeu $(env_label) (Redirect URI yang harus terdaftar untuk client ini: $API_ROOT_URL/sso/callback/login):"
    tanya_nilai SSO_CLIENT_ID "Client ID" teks 1
    tanya_nilai SSO_CLIENT_SECRET "Client secret" rahasia 1
  fi
  if [ "$sldk" -eq 1 ]; then
    log "SLDK untuk Digitalisasi Aset (opsional; alamat kosong = sinkronisasi SLDK nonaktif):"
    tanya_nilai SLDK_DB_HOST "Alamat server SLDK (IP/host)" host 0
    if [ -n "$(env_get "$BACKEND_ENV" SLDK_DB_HOST)" ]; then
      tanya_nilai SLDK_DB_PORT "Port" port 1
      tanya_nilai SLDK_DB_NAME "Nama database" teks 1
      tanya_nilai SLDK_DB_USER "Login" teks 1
      tanya_nilai SLDK_DB_PASSWORD "Sandi" rahasia 1
    fi
  fi
  if [ "$inaproc" -eq 1 ]; then
    if [ "$ENVIRONMENT" = "prod" ]; then
      log "Inaproc (wajib di production):"
      tanya_nilai INAPROC_TOKEN "Token API Inaproc" rahasia 1
    else
      log "Inaproc (opsional; kosong = halaman Pengadaan/Tender membalas 503):"
      tanya_nilai INAPROC_TOKEN "Token API Inaproc" rahasia 0
    fi
  fi
  own_file "$BACKEND_ENV"
  ok "Kredensial disimpan di ${NAMA_BACKEND_ENV}."

  # Database diganti dengan sengaja: kunci jati diri server lama tidak berlaku lagi. Server yang baru dikunci saat pemeriksaan jati diri
  # (production: nama servernya diketik setelah ditampilkan).
  if [ "$(env_get "$BACKEND_ENV" DB_HOST):$(env_get "$BACKEND_ENV" DB_PORT)" != "$alamat_lama" ] && [ -n "$(env_get "$DEPLOY_ENV" EXPECTED_DB_SERVER)" ]; then
    env_set "$DEPLOY_ENV" EXPECTED_DB_SERVER ""
    warn "Alamat database berubah: EXPECTED_DB_SERVER di ${NAMA_DEPLOY_ENV} dikosongkan dan dikunci ulang saat pemeriksaan jati diri server database."
  fi
  if [ "$(env_get "$BACKEND_ENV" DB_NAME)" != "$nama_lama" ] && [ -n "$(env_get "$DEPLOY_ENV" EXPECTED_DB_NAME)" ]; then
    env_set "$DEPLOY_ENV" EXPECTED_DB_NAME ""
    warn "Nama database berubah: EXPECTED_DB_NAME di ${NAMA_DEPLOY_ENV} dikosongkan."
  fi
}

prepare_backend_env() {
  local baru=0
  if [ ! -f "$BACKEND_ENV" ]; then
    # Tanpa terminal interaktif keduanya keluar dengan petunjuk; dengan terminal, kredensialnya ditanyakan oleh isi_kredensial.
    if [ -f "$(profil_backend "$(env_lain "$ENVIRONMENT")")" ]; then buat_profil_backend_dari_lain; else create_backend_env; fi
    baru=1
  fi
  isi_nilai_environment   # APP_ENV, SSO_ENV, FRONTEND_URL, SSO_REDIRECT_URI selalu mengikuti environment dan alamatnya
  isi_kredensial "$baru"
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
  valid_port "$port" || die "DB_PORT di ${NAMA_BACKEND_ENV} tidak valid: '$port'"
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
  # Prioritas: --branch, lalu DEPLOY_BRANCH di deploy.<env>.env, lalu branch yang sedang aktif (profil lama tanpa DEPLOY_BRANCH).
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

# git pull (atau berganti branch karena berganti environment) bisa ikut mengganti deploy.sh. Bash sudah membaca versi lama ke memori, jadi
# bagian skrip yang baru tidak berlaku di deploy ini dan hasilnya membingungkan. Karena sync_code berjalan sebelum ada yang dibangun atau
# diubah (hanya kode di disk yang berganti), skrip menjalankan versi barunya sendiri dari awal dengan environment yang sama; container lama
# tidak tersentuh. Hanya sekali: bila berubah lagi (ada push baru saat itu juga), skrip berhenti.
pastikan_skrip_terbaru() {
  [ -n "$SCRIPT_SHA_AWAL" ] || return 0
  local sekarang
  sekarang=$(sha256sum "$APP_DIR/$SCRIPT_NAME" 2>/dev/null | cut -d' ' -f1 || true)
  [ -n "$sekarang" ] && [ "$sekarang" != "$SCRIPT_SHA_AWAL" ] || return 0
  if [ -n "${PASTI_DEPLOY_ULANG:-}" ]; then
    err "deploy.sh berubah lagi setelah dijalankan ulang. Kode di server sudah terbaru dan container lama TIDAK diubah."
    die "Jalankan sekali lagi:  ./deploy.sh $ENVIRONMENT"
  fi
  warn "deploy.sh ikut diperbarui oleh git; menjalankan versi barunya dari awal (belum ada yang diubah di server)."
  exec 1>&7 2>&8   # tanpa tee lama, supaya deploy.log tidak mencatat dua kali
  exec env PASTI_DEPLOY_ULANG=1 bash "$APP_DIR/$SCRIPT_NAME" "${ORIG_ARGS[@]}" "$ENVIRONMENT"
}

# ----------------------------------------------------------------------------
# Tahap 4-6: build, migrasi, jalankan, cek kesehatan
# ----------------------------------------------------------------------------
# Production butuh persetujuan eksplisit sebelum apa pun dibangun/diubah. Ditanyakan SETELAH kode
# ditarik dan .env divalidasi, supaya yang ditampilkan adalah persis apa yang akan dideploy.
# Berganti environment di folder yang sama = container yang sedang berjalan (nama dan port yang sama) diganti. Semua alamat yang mengarah
# ke server ini, termasuk yang dipetakan infrastruktur (IP masking), ikut melayani environment baru, jadi pergantian selalu ditanyakan.
konfirmasi_ganti_environment() {
  [ -n "$PREV_ENV" ] && [ "$PREV_ENV" != "$ENVIRONMENT" ] || return 0
  [ "$(container_state "${CONTAINER_PREFIX}-backend")" != "tidak-ada" ] || [ "$(container_state "${CONTAINER_PREFIX}-frontend")" != "tidak-ada" ] || return 0
  step "Berganti environment: $(env_label "$PREV_ENV") -> $(env_label)"
  warn "Container ${CONTAINER_PREFIX}-backend dan ${CONTAINER_PREFIX}-frontend sekarang menjalankan $(env_label "$PREV_ENV") dan akan diganti $(env_label) (port sama)."
  warn "Alamat yang mengarah ke server ini, termasuk yang dipetakan infrastruktur, akan melayani $(env_label) dengan database di ${NAMA_BACKEND_ENV}."
  confirm "Lanjutkan berganti ke $(env_label)?" n || die "Dibatalkan. Container yang berjalan tidak diubah."
}

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
# build Docker); LOGIN_PATH di deploy.<env>.env (bila ada) menimpanya. Dibaca SETELAH sync_code supaya yang dipakai berasal dari kode
# yang akan dibangun. Diekspor agar docker compose menanamkan nilai yang sama dengan yang dipakai cek kesehatan dan ringkasan.
# "/login" berarti tidak disembunyikan. Bentuk lain harus sama dengan frontend/lib/loginPath.ts: satu segmen 16-128 karakter
# [A-Za-z0-9_=-], bukan nama rute aplikasi.
resolve_login_path() {
  step "Alamat halaman login"
  local berkas="$APP_DIR/frontend/login-path.txt" bawaan="" khusus
  # Versi deploy.sh sebelumnya membuat alamat acak sendiri di deploy.<env>.env (diberi tanda komentar di bawah). Nilai itu dibuang supaya
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
    log "Alamat login acak buatan deploy.sh versi sebelumnya dihapus dari ${NAMA_DEPLOY_ENV}; memakai alamat bawaan repo."
  fi
  if [ -f "$berkas" ]; then bawaan=$(tr -d '[:space:]' < "$berkas"); fi
  khusus=$(env_get "$DEPLOY_ENV" LOGIN_PATH)
  khusus=${khusus//[[:space:]]/}
  if [ -n "$khusus" ]; then
    LOGIN_PATH=$khusus
    if [ -n "$bawaan" ] && [ "$khusus" != "$bawaan" ]; then
      warn "${NAMA_DEPLOY_ENV} memakai LOGIN_PATH sendiri ($khusus), bukan bawaan repo ($bawaan). Hapus baris LOGIN_PATH di ${NAMA_DEPLOY_ENV} bila ingin memakai bawaan repo."
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
      die "Alamat halaman login tidak valid: '$LOGIN_PATH' (harus satu segmen 16-128 karakter huruf/angka/_/-/= tanpa titik dan bukan nama rute aplikasi; periksa frontend/login-path.txt atau LOGIN_PATH di ${NAMA_DEPLOY_ENV})."
    fi
  fi
  export LOGIN_PATH
  ok "Halaman login: $LOGIN_PATH"
}

build_image() {  # build_image LAYANAN
  local args=()
  [ "$FRESH" -eq 1 ] && args=(--no-cache --pull)
  dc build "${args[@]}" "$1" || die "Build $1 gagal. Container lama tidak diubah dan tetap berjalan."
}

# Backend dibangun lebih dulu dan terpisah dari frontend: pasti-migrate (dipakai untuk membaca jati diri database) ada di image backend, sedangkan build frontend
# yang paling lama tidak perlu ditunggu bila ternyata database yang dituju salah.
build_backend() {
  step "Build image backend"
  build_image backend
  ok "Image backend selesai."
}

build_frontend() {
  step "Build image frontend (bisa beberapa menit)"
  build_image frontend
  ok "Image frontend selesai."
}

# ----------------------------------------------------------------------------
# Jati diri server database
#
# Alamat di DB_HOST tidak cukup untuk memastikan server mana yang dituju: infrastruktur dapat memetakan (IP masking/NAT) sebuah alamat ke server lain, mis. alamat yang
# dikira production ternyata sampai ke SQL Server development, sehingga migrasi dan data production masuk ke database development. Pemetaan itu tidak mengubah nama yang
# dilaporkan SQL Server tentang dirinya, jadi nama itulah yang dicocokkan dengan EXPECTED_DB_SERVER (deploy.<env>.env). Dijalankan SEBELUM migrasi dan sebelum container diganti:
# bila tidak cocok, belum ada yang tertulis ke database.
# ----------------------------------------------------------------------------
IDENT_SERVER="" IDENT_MESIN="" IDENT_DATABASE="" IDENT_ALAMAT=""

# ident_nilai KELUARAN KUNCI -> nilai baris IDENTITAS_<KUNCI>= (kosong bila tidak ada). Tidak ada eval: nilai hanya dibaca sebagai teks.
ident_nilai() {
  printf '%s\n' "$1" | sed -n "s/^IDENTITAS_$2=//p" | head -n 1 | tr -d '\r' || true
}

# Menanyakan jati diri server yang sedang tersambung lewat pasti-migrate (satu-satunya keluaran stdout-nya adalah baris IDENTITAS_*; pesan lain ke stderr).
baca_identitas_db() {
  local out
  out=$(dc run --rm -T --no-deps --entrypoint ./pasti-migrate backend -identitas) || return 1
  IDENT_SERVER=$(ident_nilai "$out" SERVER)
  IDENT_MESIN=$(ident_nilai "$out" MESIN)
  IDENT_DATABASE=$(ident_nilai "$out" DATABASE)
  IDENT_ALAMAT=$(ident_nilai "$out" ALAMAT)
  [ -n "$IDENT_SERVER" ]
}

# Nilai EXPECTED_* ditulis tangan di deploy.<env>.env: bentuknya dibatasi (nama server SQL: huruf, angka, titik, garis bawah, minus, dan \ untuk instans bernama).
nama_server_valid() {
  local re='^[A-Za-z0-9._\\-]{1,128}$'
  [[ $1 =~ $re ]]
}

terminal_interaktif() { [ -t 0 ]; }

# Production tanpa EXPECTED_DB_SERVER: belum ada bukti server ini production. Dikunci hanya bila operator mengetik sendiri namanya (bukan sekadar menekan Enter atau --yes),
# setelah melihat nama dan alamat server yang sebenarnya.
kunci_server_db_prod() {
  warn "EXPECTED_DB_SERVER belum diisi di ${NAMA_DEPLOY_ENV}: belum ada bukti bahwa '$IDENT_SERVER' adalah server database PRODUCTION (bukan development)."
  if [[ ${IDENT_SERVER,,}/${IDENT_DATABASE,,} =~ (dev|staging|stg|uat|sandbox|test) ]]; then
    warn "Nama server atau database ('$IDENT_SERVER' / '${IDENT_DATABASE:--}') memuat penanda non-production (dev/staging/uat/test). Periksa dengan tim infrastruktur."
  fi
  if [ "$ASSUME_YES" -eq 1 ] || ! terminal_interaktif; then
    err "Setelah memastikan (tanya tim infrastruktur) bahwa itu server database PRODUCTION, isi di ${NAMA_DEPLOY_ENV} lalu jalankan ulang:"
    err "    EXPECTED_DB_SERVER=$IDENT_SERVER"
    die "Deploy production tanpa EXPECTED_DB_SERVER tidak dilanjutkan; --yes tidak menggantikan pemeriksaan ini. Belum ada yang diubah di server."
  fi
  log "Server database yang akan menerima migrasi dan data: $IDENT_SERVER (alamat ${IDENT_ALAMAT:--}), database ${IDENT_DATABASE:--}"
  local ans tries=0
  while :; do
    tries=$((tries + 1))
    read -r -p "    Ketik NAMA server database PRODUCTION yang benar (nama server, bukan alamat IP) untuk dikunci di ${NAMA_DEPLOY_ENV} (kosong = batal): " ans || ans=""
    [ -n "$ans" ] || die "Dibatalkan. Belum ada yang diubah di server."
    [ "${ans,,}" != "${IDENT_SERVER,,}" ] || break
    if [[ $ans =~ ^[0-9.:]+$ ]] || [ "$ans" = "$IDENT_ALAMAT" ] || [ "$ans" = "$(env_get "$BACKEND_ENV" DB_HOST)" ]; then
      err "Yang diminta NAMA server menurut SQL Server (baris 'Server database yang sebenarnya' di atas), bukan alamat IP."
    else
      err "'$ans' tidak sama dengan server yang sebenarnya dituju ('$IDENT_SERVER')."
    fi
    [ "$tries" -lt 3 ] || die "Dibatalkan: nama server tidak cocok. Bila '$IDENT_SERVER' bukan server production, perbaiki database di ${NAMA_BACKEND_ENV} (./deploy.sh $ENVIRONMENT --kredensial). Belum ada yang diubah di server."
  done
  env_set "$DEPLOY_ENV" EXPECTED_DB_SERVER "$IDENT_SERVER"
  own_file "$DEPLOY_ENV"
  ok "EXPECTED_DB_SERVER=$IDENT_SERVER disimpan di ${NAMA_DEPLOY_ENV}; deploy berikutnya menolak server database lain."
}

verifikasi_identitas_db() {
  step "Memeriksa jati diri server database"
  if [ -n "${SKIP_DB_IDENTITY_CHECK:-}" ]; then
    warn "SKIP_DB_IDENTITY_CHECK: jati diri server database TIDAK diperiksa. Pastikan sendiri bahwa DB_HOST tidak dipetakan ke database environment lain."
    return 0
  fi

  local host want_server want_db wajib=0 salah=0
  host="$(env_get "$BACKEND_ENV" DB_HOST):$(env_get "$BACKEND_ENV" DB_PORT)"
  want_server=$(env_get "$DEPLOY_ENV" EXPECTED_DB_SERVER)
  want_db=$(env_get "$DEPLOY_ENV" EXPECTED_DB_NAME)
  [ -z "$want_server" ] || nama_server_valid "$want_server" || die "EXPECTED_DB_SERVER di ${NAMA_DEPLOY_ENV} tidak valid: '$want_server' (huruf, angka, titik, garis bawah, minus, dan \\ untuk instans bernama)."
  [ -z "$want_db" ] || nama_server_valid "$want_db" || die "EXPECTED_DB_NAME di ${NAMA_DEPLOY_ENV} tidak valid: '$want_db'."
  # Production wajib terbukti; di dev hanya bila EXPECTED_* diisi (selain itu jati diri hanya ditampilkan).
  if [ "$ENVIRONMENT" = "prod" ] || [ -n "$want_server$want_db" ]; then wajib=1; fi

  if ! baca_identitas_db; then
    if [ "$wajib" -eq 1 ]; then
      die "Jati diri server database tidak terbaca (lihat pesan di atas). Tanpa itu tidak bisa dipastikan database mana yang akan dimigrasi. Belum ada yang diubah di server."
    fi
    warn "Jati diri server database tidak terbaca; pemeriksaan dilewati (EXPECTED_DB_SERVER tidak diisi)."
    return 0
  fi
  ok "Server database yang sebenarnya: $IDENT_SERVER (mesin ${IDENT_MESIN:--}, alamat ${IDENT_ALAMAT:--}), database ${IDENT_DATABASE:--}"
  log "DB_HOST di ${NAMA_BACKEND_ENV}: ${host%:} (bisa saja dipetakan infrastruktur; yang dipercaya adalah nama server di atas)"

  if [ -n "$want_server" ] && [ "${IDENT_SERVER,,}" != "${want_server,,}" ]; then
    err "Server database TIDAK sesuai: diharapkan '$want_server' (EXPECTED_DB_SERVER), sebenarnya '$IDENT_SERVER'."
    salah=1
  fi
  if [ -n "$want_db" ] && [ "${IDENT_DATABASE,,}" != "${want_db,,}" ]; then
    err "Database TIDAK sesuai: diharapkan '$want_db' (EXPECTED_DB_NAME), sebenarnya '${IDENT_DATABASE:--}'."
    salah=1
  fi
  if [ "$salah" -eq 1 ]; then
    err "DB_HOST di ${NAMA_BACKEND_ENV} (${host%:}) kemungkinan dipetakan (IP masking) ke server database lain, mis. development. Minta tim infrastruktur alamat server database $(env_label) yang sebenarnya."
    err "Bila server database memang dipindahkan dengan sengaja, ubah EXPECTED_DB_SERVER di ${NAMA_DEPLOY_ENV}."
    die "Deploy dihentikan sebelum migrasi. Container lama tidak diubah dan tidak ada yang ditulis ke database."
  fi
  if [ -n "$want_server" ]; then
    ok "Sesuai dengan EXPECTED_DB_SERVER ($want_server)."
  elif [ "$ENVIRONMENT" = "prod" ]; then
    kunci_server_db_prod
  else
    log "EXPECTED_DB_SERVER tidak diisi: hanya ditampilkan, tidak dibandingkan."
  fi
  [ -z "$want_db" ] || ok "Sesuai dengan EXPECTED_DB_NAME ($want_db)."
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
    || die "Migrasi gagal. Container lama TIDAK diubah dan tetap berjalan. Periksa pesan di atas (koneksi DB di ${NAMA_BACKEND_ENV}?)."
  ok "Migrasi selesai."
}

# Selalu "docker container inspect": "docker inspect" biasa juga mencocokkan IMAGE, dan compose menamai image hasil build
# <proyek>-<layanan> (mis. pasti-prod-backend), yang sama dengan nama container. Tanpa ini, image sisa build dikira container.
container_state() { docker container inspect -f '{{.State.Status}}' "$1" 2>/dev/null || echo "tidak-ada"; }

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
    docker container inspect "$cname" >/dev/null 2>&1 || continue   # belum ada: aman (image bernama sama tidak dihitung)
    owner=$(docker container inspect -f '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$cname" 2>/dev/null || true)
    if [ -z "$owner" ]; then
      die "Container '$cname' sudah ada tetapi bukan dibuat oleh Docker Compose. Hapus dulu (docker rm -f $cname) atau pakai CONTAINER_PREFIX lain di ${NAMA_DEPLOY_ENV}. Belum ada yang diubah."
    fi
    if [ "$(readlink -f "$owner" 2>/dev/null || echo "$owner")" != "$here" ]; then
      die "Container '$cname' dimiliki stack di folder lain ($owner), bukan folder ini ($APP_DIR). Bila dev dan prod berjalan di server yang sama, beri CONTAINER_PREFIX berbeda di ${NAMA_DEPLOY_ENV}. Belum ada yang diubah."
    fi
  done
}

# Dijalankan SEBELUM mengubah apa pun. Mencegah dua kegagalan yang pernah terjadi di server:
#  1) port di deploy.<env>.env dipakai proses lain -> container lama dihapus lalu yang baru gagal start;
#  2) port di deploy.<env>.env berbeda dari port stack yang sedang berjalan -> pengguna kehilangan akses.
check_ports() {
  step "Memeriksa container dan port"
  check_containers
  local entry cname cport want label have
  for entry in "${CONTAINER_PREFIX}-backend:8686:${BACKEND_PORT}:Backend" "${CONTAINER_PREFIX}-frontend:3000:${FRONTEND_PORT}:Frontend"; do
    IFS=: read -r cname cport want label <<<"$entry"

    # Port host yang SEKARANG dipakai container ini (kosong bila belum ada / tidak berjalan).
    have=$(docker port "$cname" "$cport/tcp" 2>/dev/null | head -1 | sed 's/.*://' || true)

    if [ -n "$have" ] && [ "$have" != "$want" ]; then
      warn "$label sekarang berjalan di port $have, tetapi ${NAMA_DEPLOY_ENV} mengatur port $want. Pengguna yang memakai port $have akan kehilangan akses."
      confirm "Tetap pindah ke port $want?" n \
        || die "Dibatalkan. Samakan port di ${NAMA_DEPLOY_ENV} dengan yang sedang dipakai ($have), lalu jalankan ulang."
    fi

    if [ "$have" != "$want" ] && port_in_use "$want"; then
      die "Port $want ($label) sudah dipakai $(port_owner "$want"). Ubah port di ${NAMA_DEPLOY_ENV}, atau hentikan proses itu. Belum ada yang diubah di server."
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
  # Berganti environment: image lama dibangun untuk environment sebelumnya, jadi dijalankan lagi dengan profilnya sendiri (database dan
  # SSO environment itu), bukan profil yang baru gagal. dc membaca DEPLOY_ENV setiap kali dipanggil.
  if [ -n "$PREV_ENV" ] && [ "$PREV_ENV" != "$ENVIRONMENT" ] && [ -f "$(profil_deploy "$PREV_ENV")" ]; then
    DEPLOY_ENV=$(profil_deploy "$PREV_ENV")
    log "Versi sebelumnya dijalankan lagi sebagai $(env_label "$PREV_ENV") ($(basename "$DEPLOY_ENV"))."
  fi
  # Dinyatakan berhasil hanya bila container naik DAN cek kesehatan lolos.
  if dc up -d --no-build --force-recreate && health_checks; then
    warn "Aplikasi dipulihkan ke versi sebelumnya (kode: $PREV_COMMIT${PREV_ENV:+, environment $(env_label "$PREV_ENV")})."
    warn "Catatan: migrasi database yang sudah diterapkan TIDAK ikut dibatalkan (hanya menambah tabel/kolom, aman untuk versi lama)."
    warn "Kode di folder server tetap versi terbaru (yang bermasalah). Perbaiki dulu, push, lalu jalankan ./deploy.sh $ENVIRONMENT lagi."
  else
    err "ROLLBACK JUGA GAGAL: aplikasi mungkin TIDAK berjalan sekarang."
    err "Periksa:  docker compose --env-file $(basename "$DEPLOY_ENV") ps    dan    docker logs ${CONTAINER_PREFIX}-frontend"
    err "Bila pesannya 'address already in use', ubah port di $(basename "$DEPLOY_ENV") lalu: docker compose --env-file $(basename "$DEPLOY_ENV") up -d"
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
    Konfigurasi: ${NAMA_DEPLOY_ENV} dan ${NAMA_BACKEND_ENV}  (ganti kredensial: ./deploy.sh $ENVIRONMENT --kredensial)
    Log       : docker compose --env-file ${NAMA_DEPLOY_ENV} logs -f   (atau: tail -f $LOG_FILE)
    Berikutnya: ./deploy.sh $ENVIRONMENT setiap kali ada pembaruan (kode ditarik otomatis);
                ./deploy.sh $(env_lain "$ENVIRONMENT") untuk berganti ke $(env_label "$(env_lain "$ENVIRONMENT")") (cukup isi database-nya sekali).
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

  setup_logging
  acquire_lock
  trap 'err "Gagal tak terduga di baris $LINENO: $BASH_COMMAND"' ERR
  pindahkan_konfigurasi_lama  # deploy.env/backend/.env format lama -> profil environment-nya
  resolve_environment  # dev atau prod: dari argumen, pertanyaan, atau deploy.aktif; menentukan profil yang dipakai
  step "Environment: $(env_label)${PREV_ENV:+ (yang sedang berjalan: $(env_label "$PREV_ENV"))}"

  [ -d "$APP_DIR/.git" ] || die "$APP_DIR bukan repositori git."
  [ -f "$APP_DIR/docker-compose.yml" ] || die "docker-compose.yml tidak ditemukan di $APP_DIR."

  preflight_system
  load_deploy_config
  check_ports          # container & port, sebelum mengubah apa pun di server
  sync_code            # setelah ini kode di disk sudah terbaru
  pastikan_skrip_terbaru  # deploy.sh ikut berubah oleh git? jalankan versi barunya dari awal (belum ada yang diubah)
  resolve_login_path  # alamat halaman login (bawaan repo atau deploy.<env>.env), dari kode yang baru ditarik
  prepare_backend_env  # membuat/validasi backend/.env.<env> (bisa berhenti dengan instruksi)
  check_db_reachable   # server database terjangkau? (gagal cepat, sebelum build)
  konfirmasi_ganti_environment  # dev <-> prod di folder yang sama: container yang berjalan diganti
  confirm_production   # hanya untuk prod

  snapshot_images
  build_backend
  verifikasi_identitas_db  # jati diri server database vs EXPECTED_DB_SERVER; setelah image backend ada, sebelum frontend dan migrasi
  build_frontend
  run_migrations

  if ! start_stack || ! health_checks; then
    rollback || true
    die "Deploy GAGAL. Lihat pesan di atas dan $LOG_FILE."
  fi

  # Yang sedang berjalan sekarang environment ini (pilihan bawaan ./deploy.sh berikutnya).
  printf '%s\n' "$ENVIRONMENT" > "$ACTIVE_FILE"
  own_file "$ACTIVE_FILE"
  cleanup_images
  summary
}

# Hanya jalan bila dieksekusi langsung (bukan di-source), dan seluruh fungsi sudah
# dibaca di atas: aman walau berkas ini berubah karena git pull saat skrip berjalan.
if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  main "$@"
  exit $?
fi
