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
