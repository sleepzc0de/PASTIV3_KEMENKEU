package handlers

import (
	"context"
	"database/sql/driver"
	"errors"
	"net/http"
	"strings"
	"testing"

	"pasti-v3-backend/internal/fakesql"
)

// Satu kolom yang kurang lebar (mis. nip_pokja NVARCHAR(50) yang menampung seluruh anggota pokja) tidak boleh menggugurkan seluruh baris.

func TestPotongUTF16(t *testing.T) {
	cases := []struct {
		nama   string
		s      string
		n      int
		want   string
		potong bool
	}{
		{"muat persis", "abcde", 5, "abcde", false},
		{"lebih satu", "abcdef", 5, "abcde", true},
		{"kosong", "", 5, "", false},
		{"banyak byte tetapi sedikit unit", "ééééé", 5, "ééééé", false}, // 10 byte UTF-8, 5 unit UTF-16
		{"dipotong di batas karakter", "éééééé", 5, "ééééé", true},
		{"pasangan pengganti tidak dibelah", "ab😀", 3, "ab", true}, // 😀 = 2 unit UTF-16, sisa lebar hanya 1
		{"pasangan pengganti muat", "ab😀", 4, "ab😀", false},
		{"nol", "abc", 0, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			got, potong := potongUTF16(tc.s, tc.n)
			if got != tc.want || potong != tc.potong {
				t.Errorf("potongUTF16(%q, %d) = (%q, %v), want (%q, %v)", tc.s, tc.n, got, potong, tc.want, tc.potong)
			}
		})
	}
}

// pengumumanDenganPokja: contoh pengumuman dengan nip_pokja dan nama_pokja berisi banyak anggota.
func pengumumanDenganPokja(nip, nama string) string {
	b := strings.Replace(contohTenderPengumuman, `"nip_pokja": "123456789"`, `"nip_pokja": "`+nip+`"`, 1)
	return strings.Replace(b, `"nama_pokja": "Contoh Nama Pokja"`, `"nama_pokja": "`+nama+`"`, 1)
}

func indeksKolomInsert(t *testing.T, e *endpointDatar, kolom string) int {
	t.Helper()
	for i, k := range e.kolomInsert {
		if k == kolom {
			return i
		}
	}
	t.Fatalf("kolom %s tidak ada di INSERT %s", kolom, e.Tabel)
	return -1
}

func jawabLebarKolom(f *fakesql.DB, lebar map[string]int64) {
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if !strings.Contains(q, "INFORMATION_SCHEMA.COLUMNS") {
			return nil, nil, nil
		}
		var rows [][]driver.Value
		for k, n := range lebar {
			rows = append(rows, []driver.Value{k, n})
		}
		return []string{"COLUMN_NAME", "CHARACTER_MAXIMUM_LENGTH"}, rows, nil
	}
}

func TestJalankanMemotongNilaiYangMelebihiLebarKolomTanpaMenggugurkanBaris(t *testing.T) {
	nipPanjang := strings.Repeat("1", 120)
	p := newInaprocPalsu(t, func(*http.Request) (int, string) {
		return 200, `{"success":true,"data":[` + pengumumanDenganPokja(nipPanjang, "Pokja Pendek") + `],"meta":{"limit":1000,"has_more":false,"cursor":""}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")
	f := pasangDBPalsu(t)
	jawabLebarKolom(f, map[string]int64{"nip_pokja": 50, "nama_pokja": 255, "row_key": 8})

	hasil, err := tenderPengumuman.Jalankan(context.Background(), rencanaUji(t, tenderPengumuman), "")
	if err != nil {
		t.Fatal(err)
	}
	if hasil.TotalSinkron != 1 || hasil.TotalGagal != 0 || hasil.TotalDipotong != 1 {
		t.Fatalf("hasil = %+v, want 1 tersimpan, 0 gagal, 1 nilai dipotong", hasil)
	}

	var sisip, catatan string
	for _, ex := range f.Execs() {
		switch {
		case strings.HasPrefix(ex.Query, "INSERT INTO inaproc_tender_pengumuman"):
			nip, _ := ex.Args[indeksKolomInsert(t, tenderPengumuman, "nip_pokja")].Value.(string)
			nama, _ := ex.Args[indeksKolomInsert(t, tenderPengumuman, "nama_pokja")].Value.(string)
			if nip != nipPanjang[:50] || nama != "Pokja Pendek" {
				t.Errorf("nip_pokja = %q (%d), nama_pokja = %q; want nip dipotong ke 50 dan nama utuh", nip, len(nip), nama)
			}
			// Kunci utama tidak pernah dipotong, walau "lebarnya" dilaporkan lebih kecil.
			if rk, _ := ex.Args[0].Value.(string); len(rk) <= 8 {
				t.Errorf("row_key dipotong: %q", rk)
			}
			sisip = ex.Query
		case strings.Contains(ex.Query, "INTO inaproc_sync_log"):
			catatan, _ = ex.Args[6].Value.(string)
		}
	}
	if sisip == "" {
		t.Fatal("baris tidak disisipkan")
	}
	if !strings.Contains(catatan, "1 nilai dipotong") || strings.Contains(catatan, "gagal") {
		t.Errorf("catatan sync_log = %q, want memuat '1 nilai dipotong' tanpa 'gagal'", catatan)
	}
}

// Tanpa informasi lebar (query skema gagal), nilai disimpan apa adanya dan sinkronisasi tetap jalan seperti biasa.
func TestJalankanTanpaLebarKolomMenyimpanApaAdanya(t *testing.T) {
	nipPanjang := strings.Repeat("1", 120)
	p := newInaprocPalsu(t, func(*http.Request) (int, string) {
		return 200, `{"success":true,"data":[` + pengumumanDenganPokja(nipPanjang, "Pokja") + `],"meta":{"limit":1000,"has_more":false,"cursor":""}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")
	f := pasangDBPalsu(t)
	f.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return nil, nil, errors.New("tidak boleh membaca skema")
	}

	hasil, err := tenderPengumuman.Jalankan(context.Background(), rencanaUji(t, tenderPengumuman), "")
	if err != nil {
		t.Fatal(err)
	}
	if hasil.TotalSinkron != 1 || hasil.TotalDipotong != 0 {
		t.Fatalf("hasil = %+v", hasil)
	}
	for _, ex := range f.Execs() {
		if strings.HasPrefix(ex.Query, "INSERT INTO inaproc_tender_pengumuman") {
			if nip, _ := ex.Args[indeksKolomInsert(t, tenderPengumuman, "nip_pokja")].Value.(string); nip != nipPanjang {
				t.Errorf("nip_pokja = %q, want nilai asli utuh", nip)
			}
		}
	}
}

// Nilai yang dipotong hanya dihitung bila barisnya benar-benar tersimpan; baris yang tetap gagal dihitung sebagai gagal.
func TestJalankanBarisYangTetapGagalTidakDihitungDipotong(t *testing.T) {
	p := newInaprocPalsu(t, func(*http.Request) (int, string) {
		return 200, `{"success":true,"data":[` + pengumumanDenganPokja(strings.Repeat("1", 120), "Pokja") + `],"meta":{"limit":1000,"has_more":false,"cursor":""}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")
	f := pasangDBPalsu(t)
	jawabLebarKolom(f, map[string]int64{"nip_pokja": 50})
	f.OnExec = func(q string, _ []driver.NamedValue) error {
		if strings.HasPrefix(q, "INSERT INTO inaproc_tender_pengumuman") {
			return errors.New("Violation of PRIMARY KEY constraint")
		}
		return nil
	}

	hasil, err := tenderPengumuman.Jalankan(context.Background(), rencanaUji(t, tenderPengumuman), "")
	if err != nil {
		t.Fatal(err)
	}
	if hasil.TotalSinkron != 0 || hasil.TotalGagal != 1 || hasil.TotalDipotong != 0 {
		t.Errorf("hasil = %+v, want 0 tersimpan, 1 gagal, 0 dipotong", hasil)
	}
}

func TestCatatanHasil(t *testing.T) {
	cases := []struct {
		h    HasilSinkron
		want string
	}{
		{HasilSinkron{TotalSinkron: 5}, ""},
		{HasilSinkron{TotalGagal: 2}, "2 baris gagal disimpan"},
		{HasilSinkron{TotalDipotong: 3}, "3 nilai dipotong karena melebihi lebar kolom"},
		{HasilSinkron{TotalGagal: 1, TotalDipotong: 4}, "1 baris gagal disimpan; 4 nilai dipotong karena melebihi lebar kolom"},
	}
	for _, tc := range cases {
		if got := catatanHasil(tc.h); got != tc.want {
			t.Errorf("catatanHasil(%+v) = %q, want %q", tc.h, got, tc.want)
		}
	}
}
