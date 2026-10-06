package peran

import (
	"strings"
	"testing"
)

func TestValidasiPeran(t *testing.T) {
	for _, c := range []struct {
		peran, kode, want string
		ok                bool
	}{
		{PenggunaBarang, "", "", true},
		{PenggunaBarang, "  ", "", true},
		{UE1, "01504", "01504", true},
		{UE1, " 01504 ", "01504", true},
		{Kanwil, "015040199", "015040199", true},
		{Satker, "119091", "119091", true},
		{PenggunaBarang, "01504", "", false}, // tanpa kode
		{UE1, "1504", "", false},
		{UE1, "015040", "", false},
		{UE1, "0150A", "", false},
		{Kanwil, "01504", "", false}, // panjangnya milik UE1
		{Satker, "11909", "", false},
		{Satker, "11909x", "", false},
		{Satker, "٠١٢٣٤٥", "", false}, // angka non-ASCII
		{"superadmin", "", "", false}, // diberikan lewat akun, bukan tabel peran
		{"", "", "", false},
		{"admin", "", "", false},
	} {
		got, err := ValidasiPeran(c.peran, c.kode)
		if (err == nil) != c.ok || got != c.want {
			t.Errorf("ValidasiPeran(%q, %q) = %q, %v; want %q ok=%v", c.peran, c.kode, got, err, c.want, c.ok)
		}
	}
}

func TestKondisiSQLDanBolehKodeSatker(t *testing.T) {
	const lengkap = "015040199119091000KP"
	for _, c := range []struct {
		nama    string
		cakupan Cakupan
		sql     string
		boleh   map[string]bool // kode satker lengkap -> diharapkan
	}{
		{"semua", CakupanSemua, "1 = 1", map[string]bool{lengkap: true, "": true}},
		{"nilai nol dianggap semua", Cakupan{}, "1 = 1", map[string]bool{lengkap: true}},
		{"kosong", Cakupan{Tingkat: Kosong}, "1 = 0", map[string]bool{lengkap: false}},
		{"ue1", Cakupan{Tingkat: TingkatUE1, Kode: "01504"}, "LEFT([Kode_Satker], 5) = N'01504'",
			map[string]bool{lengkap: true, "015050199119091000KP": false, "0150": false, "": false}},
		{"kanwil", Cakupan{Tingkat: TingkatKwl, Kode: "015040199"}, "LEFT([Kode_Satker], 9) = N'015040199'",
			map[string]bool{lengkap: true, "015040299119091000KP": false, "015050199119091000KP": false}},
		{"satker", Cakupan{Tingkat: TingkatSatk, Kode: "119091"}, "(LEN([Kode_Satker]) >= 15 AND SUBSTRING([Kode_Satker], 10, 6) = N'119091')",
			map[string]bool{lengkap: true, "015040199119091001KP": true, "015040199119092000KP": false, "119091": false, "01504019911909": false}},
		// kode tidak sah tidak boleh sampai ke SQL: ditutup, bukan dibuka
		{"ue1 rusak", Cakupan{Tingkat: TingkatUE1, Kode: "01504' OR '1'='1"}, "1 = 0", map[string]bool{lengkap: false}},
		{"ue1 terlalu pendek", Cakupan{Tingkat: TingkatUE1, Kode: "0150"}, "1 = 0", map[string]bool{lengkap: false}},
		{"satker bukan angka", Cakupan{Tingkat: TingkatSatk, Kode: "11909;"}, "1 = 0", map[string]bool{lengkap: false}},
		{"tingkat tak dikenal", Cakupan{Tingkat: "lain", Kode: "01504"}, "1 = 0", map[string]bool{lengkap: false}},
	} {
		if got := c.cakupan.KondisiSQL("[Kode_Satker]"); got != c.sql {
			t.Errorf("%s: KondisiSQL = %q, want %q", c.nama, got, c.sql)
		}
		for kode, want := range c.boleh {
			if got := c.cakupan.BolehKodeSatker(kode); got != want {
				t.Errorf("%s: BolehKodeSatker(%q) = %v, want %v", c.nama, kode, got, want)
			}
		}
	}
	if !CakupanSemua.SemuaData() || (Cakupan{Tingkat: Kosong}).SemuaData() || (Cakupan{Tingkat: TingkatUE1, Kode: "01504"}).SemuaData() {
		t.Error("SemuaData hanya benar untuk cakupan semua")
	}
}

func TestBolehSatker6(t *testing.T) {
	if !CakupanSemua.BolehSatker6("123456") {
		t.Error("semua: boleh")
	}
	s := Cakupan{Tingkat: TingkatSatk, Kode: "119091"}
	if !s.BolehSatker6("119091") || s.BolehSatker6("119092") {
		t.Error("satker hanya kodenya sendiri")
	}
	if (Cakupan{Tingkat: TingkatUE1, Kode: "01504"}).BolehSatker6("119091") {
		t.Error("UE1 tidak bisa memastikan satker 6 digit tanpa daftar satker: harus ditolak")
	}
}

func TestSelesaikan(t *testing.T) {
	ue1 := Baris{ID: 5, Peran: UE1, Kode: "01504"}
	satker := Baris{ID: 9, Peran: Satker, Kode: "119091"}
	barang := Baris{ID: 2, Peran: PenggunaBarang}

	t.Run("admin tanpa pilihan: peran akun, semua data, hak admin", func(t *testing.T) {
		e := Selesaikan(AkunAdmin, []Baris{ue1}, true)
		if e.Role != AkunAdmin || e.Peran != AkunAdmin || !e.Cakupan.SemuaData() || e.PeranID != 0 {
			t.Errorf("%+v", e)
		}
		e = Selesaikan(AkunSuperadmin, nil, true)
		if e.Role != AkunSuperadmin || e.Peran != AkunSuperadmin || !e.Cakupan.SemuaData() {
			t.Errorf("%+v", e)
		}
	})
	t.Run("admin yang bertindak sebagai peran data kehilangan hak admin", func(t *testing.T) {
		a := ue1
		a.Aktif = true
		e := Selesaikan(AkunAdmin, []Baris{a, satker}, false)
		if e.Role != AkunUser || e.Peran != UE1 || e.Cakupan != (Cakupan{Tingkat: TingkatUE1, Kode: "01504"}) || e.PeranID != 5 || e.AkunRole != AkunAdmin {
			t.Errorf("%+v", e)
		}
	})
	t.Run("pengguna biasa tanpa pilihan memakai peran pertama", func(t *testing.T) {
		e := Selesaikan(AkunUser, []Baris{barang, ue1}, false)
		if e.Peran != PenggunaBarang || !e.Cakupan.SemuaData() || e.Role != AkunUser {
			t.Errorf("%+v", e)
		}
	})
	t.Run("pilihan aktif didahulukan atas urutan", func(t *testing.T) {
		s := satker
		s.Aktif = true
		e := Selesaikan(AkunUser, []Baris{barang, s}, false)
		if e.Peran != Satker || e.Cakupan != (Cakupan{Tingkat: TingkatSatk, Kode: "119091"}) {
			t.Errorf("%+v", e)
		}
	})
	t.Run("tanpa peran: semua data kecuali pembatasan wajib", func(t *testing.T) {
		if e := Selesaikan(AkunUser, nil, false); !e.Cakupan.SemuaData() || e.Peran != "" || e.Role != AkunUser {
			t.Errorf("tidak wajib: %+v", e)
		}
		if e := Selesaikan(AkunUser, nil, true); e.Cakupan.Tingkat != Kosong || e.SemuaData() {
			t.Errorf("wajib: %+v", e)
		}
	})
	t.Run("peran rusak di database tidak membuka data", func(t *testing.T) {
		for _, b := range []Baris{{ID: 1, Peran: UE1, Kode: "abc", Aktif: true}, {ID: 2, Peran: "peretas", Kode: "01504", Aktif: true}, {ID: 3, Peran: Satker, Kode: "", Aktif: true}} {
			if e := Selesaikan(AkunUser, []Baris{b}, false); e.Cakupan.Tingkat != Kosong {
				t.Errorf("%+v -> %+v, want kosong", b, e)
			}
		}
	})
	t.Run("pengguna barang melihat semua data tetapi bukan admin", func(t *testing.T) {
		b := barang
		b.Aktif = true
		e := Selesaikan(AkunSuperadmin, []Baris{b}, true)
		if !e.Cakupan.SemuaData() || e.Role != AkunUser || e.Peran != PenggunaBarang {
			t.Errorf("%+v", e)
		}
	})
}

func TestSaranDariKodeSatker(t *testing.T) {
	got := SaranDariKodeSatker("015040199119091000KP")
	want := []Saran{{UE1, "01504", "UE1"}, {Kanwil, "015040199", "Kanwil"}, {Satker, "119091", "Satker"}}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("saran = %+v, want %+v", got, want)
	}
	if got := SaranDariKodeSatker("  015040199119091  "); len(got) != 3 || got[2].Kode != "119091" {
		t.Errorf("tanpa akhiran = %+v", got)
	}
	if got := SaranDariKodeSatker("119091"); len(got) != 1 || got[0].Peran != Satker || got[0].Kode != "119091" {
		t.Errorf("6 digit = %+v", got)
	}
	for _, k := range []string{"", "12345", "0150401991190", "01504019911909x000KP", "ABCDEFGHIJKLMNOPQRST", strings.Repeat("x", 30)} {
		if got := SaranDariKodeSatker(k); got != nil {
			t.Errorf("SaranDariKodeSatker(%q) = %+v, want nil", k, got)
		}
	}
}

func TestLabel(t *testing.T) {
	for p, want := range map[string]string{AkunSuperadmin: "Super Admin", PenggunaBarang: "Pengguna Barang", UE1: "UE1", Kanwil: "Kanwil", Satker: "Satker", "": "", "lain": "lain"} {
		if got := Label(p); got != want {
			t.Errorf("Label(%q) = %q, want %q", p, got, want)
		}
	}
}
