package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort     string
	AppEnv      string
	FrontendURL string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	JWTSecret            string
	JWTAccessExpireMin   int
	JWTRefreshExpireDays int

	PasswordPepper string

	SSOEnv          string
	SSOClientID     string
	SSOClientSecret string
	SSOScope        string
	SSORedirectURI  string

	SuperadminProtectedEmail string
	SuperadminProtectedNIP   string

	HTTPClientTimeoutSeconds int
	HTTPSProxy               string
	HTTPProxy                string

	// ============ SLDK Integration ============
	SLDKDBHost          string
	SLDKDBPort          string
	SLDKDBUser          string
	SLDKDBPassword      string
	SLDKDBName          string
	SLDKAssetTable      string
	SLDKAssetSearchCols []string
	// SLDKKLKode membatasi data aset ke satu K/L menurut kodenya (mis. 015 = Kementerian Keuangan). Kosong = seluruh K/L.
	SLDKKLKode string

	// ============ Sinkronisasi otomatis Digitalisasi Aset ============
	// Aktif bawaan: tiap 7 hari, mulai antara pukul 01.00 dan 05.00 WIB (di luar jam kerja). Hanya berjalan bila SLDK tersambung.
	DigitalisasiAutoSync         bool
	DigitalisasiAutoIntervalHari int
	DigitalisasiAutoJamMulai     int
	DigitalisasiAutoJamAkhir     int

	// ============ Penarikan data Pengadaan/Tender/E-Katalog otomatis (Inaproc) ============
	// Nilai bawaan penarikan otomatis; pengaturan yang disimpan dari halaman Penarikan Data menggantikannya. Aktif bawaan: tiap
	// 2 hari, mulai antara pukul 01.00 dan 05.00 WIB, tahun berjalan dan satu tahun sebelumnya. Hanya berjalan bila INAPROC_TOKEN terisi.
	InaprocAutoSync         bool
	InaprocAutoIntervalHari int
	InaprocAutoJamMulai     int
	InaprocAutoJamAkhir     int
	InaprocAutoJumlahTahun  int

	// Batas permintaan ke Inaproc. Inaproc membatasi 1.000 permintaan per 60 detik dan kuota 5.000 permintaan yang direset tiap 1 jam; batas
	// di sini sengaja sedikit di bawahnya (bawaan 800 per menit dan 4.500 per jam) supaya penarikan tidak pernah menabrak batas Inaproc.
	InaprocBatasPerMenit int
	InaprocBatasPerJam   int
	// Kebijakan gagal tarik: sebuah tugas dicoba maksimal InaprocMaksPercobaan kali dalam satu siklus, lalu istirahat InaprocIstirahatJam jam
	// sebelum boleh ditarik ulang otomatis.
	InaprocMaksPercobaan int
	InaprocIstirahatJam  int

	TokenEncryptionKey string
	InaprocBaseURL     string
	InaprocToken       string
}

var Cfg *Config

func LoadConfig() {
	if err := godotenv.Load(); err != nil {
		log.Println("[WARN] .env file tidak ditemukan, menggunakan environment variable sistem")
	}

	accessExp, _ := strconv.Atoi(getEnv("JWT_ACCESS_EXPIRE_MINUTES", "25"))
	refreshExp, _ := strconv.Atoi(getEnv("JWT_REFRESH_EXPIRE_DAYS", "7"))
	httpTimeout, _ := strconv.Atoi(getEnv("HTTP_CLIENT_TIMEOUT_SECONDS", "30"))

	Cfg = &Config{
		AppPort:     getEnv("APP_PORT", "8686"),
		AppEnv:      getEnv("APP_ENV", "development"),
		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "1433"),
		DBUser:     getEnv("DB_USER", "sa"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "pasti_v3_db"),

		JWTSecret:            getEnv("JWT_SECRET", ""),
		JWTAccessExpireMin:   accessExp,
		JWTRefreshExpireDays: refreshExp,

		PasswordPepper: getEnv("PASSWORD_PEPPER", ""),

		SSOEnv:          getEnv("SSO_ENV", "development"),
		SSOClientID:     getEnv("SSO_CLIENT_ID", ""),
		SSOClientSecret: getEnv("SSO_CLIENT_SECRET", ""),
		SSOScope:        getEnv("SSO_SCOPE", "openid profile"),
		SSORedirectURI:  getEnv("SSO_REDIRECT_URI", ""),

		SuperadminProtectedEmail: getEnv("SUPERADMIN_PROTECTED_EMAIL", ""),
		SuperadminProtectedNIP:   getEnv("SUPERADMIN_PROTECTED_NIP", ""),

		HTTPClientTimeoutSeconds: httpTimeout,
		HTTPSProxy:               getEnv("HTTPS_PROXY", ""),
		HTTPProxy:                getEnv("HTTP_PROXY", ""),

		SLDKDBHost:          getEnv("SLDK_DB_HOST", ""),
		SLDKDBPort:          getEnv("SLDK_DB_PORT", "1433"),
		SLDKDBUser:          getEnv("SLDK_DB_USER", ""),
		SLDKDBPassword:      getEnv("SLDK_DB_PASSWORD", ""),
		SLDKDBName:          getEnv("SLDK_DB_NAME", ""),
		SLDKAssetTable:      getEnv("SLDK_ASSET_TABLE", ""),
		SLDKAssetSearchCols: parseCommaList(getEnv("SLDK_ASSET_SEARCH_COLUMNS", "")),
		SLDKKLKode:          strings.TrimSpace(getEnv("SLDK_KL_KODE", "")),

		DigitalisasiAutoSync:         getEnvBool("DIGITALISASI_AUTO_SYNC", true),
		DigitalisasiAutoIntervalHari: getEnvInt("DIGITALISASI_AUTO_INTERVAL_HARI", 7),
		DigitalisasiAutoJamMulai:     getEnvInt("DIGITALISASI_AUTO_JAM_MULAI", 1),
		DigitalisasiAutoJamAkhir:     getEnvInt("DIGITALISASI_AUTO_JAM_AKHIR", 5),

		InaprocAutoSync:         getEnvBool("INAPROC_AUTO_SYNC", true),
		InaprocAutoIntervalHari: getEnvInt("INAPROC_AUTO_INTERVAL_HARI", 2),
		InaprocAutoJamMulai:     getEnvInt("INAPROC_AUTO_JAM_MULAI", 1),
		InaprocAutoJamAkhir:     getEnvInt("INAPROC_AUTO_JAM_AKHIR", 5),
		InaprocAutoJumlahTahun:  getEnvInt("INAPROC_AUTO_JUMLAH_TAHUN", 2),
		InaprocBatasPerMenit:    getEnvInt("INAPROC_BATAS_PER_MENIT", 800),
		InaprocBatasPerJam:      getEnvInt("INAPROC_BATAS_PER_JAM", 4500),
		InaprocMaksPercobaan:    getEnvInt("INAPROC_MAKS_PERCOBAAN", 3),
		InaprocIstirahatJam:     getEnvInt("INAPROC_ISTIRAHAT_JAM", 8),

		TokenEncryptionKey: getEnv("TOKEN_ENCRYPTION_KEY", ""),
		InaprocBaseURL:     getEnv("INAPROC_BASE_URL", "https://data.inaproc.id"),
		InaprocToken:       getEnv("INAPROC_TOKEN", ""),
	}

	if Cfg.JWTSecret == "" || Cfg.PasswordPepper == "" {
		log.Fatal("[FATAL] JWT_SECRET dan PASSWORD_PEPPER wajib diisi di .env")
	}
	if Cfg.SSOClientID == "" || Cfg.SSOClientSecret == "" || Cfg.SSORedirectURI == "" {
		log.Fatal("[FATAL] Konfigurasi SSO wajib diisi di .env")
	}

	if Cfg.HTTPSProxy != "" {
		os.Setenv("HTTPS_PROXY", Cfg.HTTPSProxy)
	}
	if Cfg.HTTPProxy != "" {
		os.Setenv("HTTP_PROXY", Cfg.HTTPProxy)
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

// getEnvBool membaca nilai ya/tidak: false, 0, off, no, tidak = mati; true, 1, on, yes, ya = hidup; selain itu (atau kosong) = fallback.
func getEnvBool(key string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "false", "0", "off", "no", "tidak":
		return false
	case "true", "1", "on", "yes", "ya":
		return true
	}
	return fallback
}

// getEnvInt membaca bilangan bulat; kosong atau tidak terbaca = fallback.
func getEnvInt(key string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return fallback
	}
	return n
}

type SSOEndpoints struct {
	BaseURL            string
	AuthorizeEndpoint  string
	TokenEndpoint      string
	UserinfoEndpoint   string
	EndSessionEndpoint string
}

func GetSSOEndpoints() SSOEndpoints {
	if Cfg.SSOEnv == "production" {
		return SSOEndpoints{
			BaseURL:            "https://sso.kemenkeu.go.id",
			AuthorizeEndpoint:  "https://sso.kemenkeu.go.id/connect/authorize",
			TokenEndpoint:      "https://sso.kemenkeu.go.id/connect/token",
			UserinfoEndpoint:   "https://sso.kemenkeu.go.id/connect/userinfo",
			EndSessionEndpoint: "https://sso.kemenkeu.go.id/connect/endsession",
		}
	}
	return SSOEndpoints{
		BaseURL:            "https://demo-account.kemenkeu.go.id",
		AuthorizeEndpoint:  "https://demo-account.kemenkeu.go.id/connect/authorize",
		TokenEndpoint:      "https://demo-account.kemenkeu.go.id/connect/token",
		UserinfoEndpoint:   "https://demo-account.kemenkeu.go.id/connect/userinfo",
		EndSessionEndpoint: "https://demo-account.kemenkeu.go.id/connect/endsession",
	}
}

func parseCommaList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// GetHRIS2BaseURL mengembalikan base URL API HRIS2 sesuai environment,
// mengikuti environment SSO (SSO_ENV) karena keduanya satu ekosistem Kemenkeu.
func GetHRIS2BaseURL() string {
	if Cfg.SSOEnv == "production" {
		return "https://apigateway.kemenkeu.go.id/gateway/HrisProfil/2.0"
	}
	return "https://demo-service.kemenkeu.go.id/hris2/profil"
}
