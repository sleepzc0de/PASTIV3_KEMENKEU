package config

import "testing"

func TestGetEnvBool(t *testing.T) {
	cases := []struct {
		nilai    string
		fallback bool
		want     bool
	}{
		{"", true, true}, {"", false, false}, // kosong = bawaan
		{"false", true, false}, {"FALSE", true, false}, {" off ", true, false}, {"0", true, false}, {"no", true, false}, {"tidak", true, false},
		{"true", false, true}, {"True", false, true}, {"1", false, true}, {"on", false, true}, {"yes", false, true}, {"ya", false, true},
		{"mungkin", true, true}, {"mungkin", false, false}, // tidak dikenal = bawaan
	}
	for _, c := range cases {
		t.Setenv("PASTI_UJI_BOOL", c.nilai)
		if got := getEnvBool("PASTI_UJI_BOOL", c.fallback); got != c.want {
			t.Errorf("getEnvBool(%q, %v) = %v, want %v", c.nilai, c.fallback, got, c.want)
		}
	}
}

func TestGetEnvInt(t *testing.T) {
	cases := []struct {
		nilai    string
		fallback int
		want     int
	}{{"", 7, 7}, {"3", 7, 3}, {" 14 ", 7, 14}, {"0", 7, 0}, {"-2", 7, -2}, {"tujuh", 7, 7}, {"1.5", 7, 7}}
	for _, c := range cases {
		t.Setenv("PASTI_UJI_INT", c.nilai)
		if got := getEnvInt("PASTI_UJI_INT", c.fallback); got != c.want {
			t.Errorf("getEnvInt(%q, %d) = %d, want %d", c.nilai, c.fallback, got, c.want)
		}
	}
}

// muatConfigUji memuat konfigurasi dengan rahasia wajib terisi, dan memulihkan Cfg sesudah tes.
func muatConfigUji(t *testing.T) *Config {
	t.Helper()
	lama := Cfg
	t.Cleanup(func() { Cfg = lama })
	t.Setenv("JWT_SECRET", "rahasia-uji-jwt-yang-cukup-panjang")
	t.Setenv("PASSWORD_PEPPER", "pepper-uji-yang-cukup-panjang")
	t.Setenv("SSO_CLIENT_ID", "klien-uji")
	t.Setenv("SSO_CLIENT_SECRET", "rahasia-sso-uji")
	t.Setenv("SSO_REDIRECT_URI", "http://localhost/sso/callback/login")
	LoadConfig()
	return Cfg
}

func TestDatamartDibacaDariEnv(t *testing.T) {
	t.Setenv("DATAMART_DB_HOST", "dm.example")
	t.Setenv("DATAMART_DB_PORT", "1444")
	t.Setenv("DATAMART_DB_USER", "pembaca")
	t.Setenv("DATAMART_DB_PASSWORD", "rahasia")
	t.Setenv("DATAMART_DB_NAME", "pasti_datamart")
	c := muatConfigUji(t)
	if c.DatamartDBHost != "dm.example" || c.DatamartDBPort != "1444" || c.DatamartDBUser != "pembaca" || c.DatamartDBPassword != "rahasia" || c.DatamartDBName != "pasti_datamart" {
		t.Errorf("datamart = %+v", c)
	}
}

func TestDatamartKosongBawaanTanpaSumber(t *testing.T) {
	for _, k := range []string{"DATAMART_DB_HOST", "DATAMART_DB_PORT", "DATAMART_DB_USER", "DATAMART_DB_PASSWORD", "DATAMART_DB_NAME"} {
		t.Setenv(k, "")
	}
	c := muatConfigUji(t)
	if c.DatamartDBHost != "" {
		t.Errorf("bawaan = host %q", c.DatamartDBHost)
	}
}
