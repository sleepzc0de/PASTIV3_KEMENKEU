package routes

import (
	"context"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"pasti-v3-backend/sapa"
)

// Tes integrasi SQL Server (hanya berjalan bila PASTI_UJI_MSSQL_DSN diisi, lihat bukaDBUji): daftar jenis/satuan BMN hasil migrasi 022 dan 057 sama dengan DefaultRefBMN, migrasi
// 057 aman dijalankan ulang, dan Store membaca referensi Kanwil untuk saran tembusan Nota Dinas.

func TestSapaDaftarBMNHasilMigrasiSamaDenganDefaultDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	s := sapa.NewStore(db)
	ctx := context.Background()

	banding := func(ket string) {
		t.Helper()
		got, err := s.AmbilRefBMN(ctx)
		if err != nil {
			t.Fatal(err)
		}
		want := sapa.DefaultRefBMN()
		if len(got.Satuan) != len(want.Satuan) || len(got.Jenis) != len(want.Jenis) {
			t.Fatalf("%s: %d satuan/%d jenis di DB, want %d/%d", ket, len(got.Satuan), len(got.Jenis), len(want.Satuan), len(want.Jenis))
		}
		for i, w := range want.Satuan {
			if got.Satuan[i].Nama != w.Nama || got.Satuan[i].Urutan != w.Urutan {
				t.Errorf("%s: satuan[%d] = %+v, want %+v", ket, i, got.Satuan[i], w)
			}
		}
		for i, w := range want.Jenis {
			g := got.Jenis[i]
			if g.Nama != w.Nama || g.SatuanBawaan != w.SatuanBawaan || !reflect.DeepEqual(g.Satuan, w.Satuan) {
				t.Errorf("%s: jenis[%d] = %+v, want %+v", ket, i, g, w)
			}
		}
	}
	banding("setelah migrasi")

	// Migrasi 057 dijalankan ulang tidak menggandakan apa pun.
	skrip, err := os.ReadFile("../migrations/057_sapa_satuan_nup_m2.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i, batch := range regexp.MustCompile(`(?im)^[ \t]*GO[ \t]*\r?$`).Split(string(skrip), -1) {
		if strings.TrimSpace(batch) == "" {
			continue
		}
		if _, err := db.Exec(batch); err != nil {
			t.Fatalf("jalankan ulang batch %d: %v", i+1, err)
		}
	}
	banding("setelah migrasi 057 dijalankan ulang")
}

func TestSapaAmbilRefKanwilDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	const kode = "015990198"
	bersih := func() { db.Exec(`DELETE FROM ref_kanwil WHERE kode = N'` + kode + `'`) }
	bersih()
	if os.Getenv("PASTI_UJI_BIARKAN_DATA") != "1" {
		t.Cleanup(bersih)
	}
	s := sapa.NewStore(db)
	ctx := context.Background()

	if r, err := s.AmbilRefKanwil(ctx, kode); err != nil || r != nil {
		t.Fatalf("belum terdaftar: %+v %v, want nil tanpa galat", r, err)
	}
	if _, err := db.Exec(`INSERT INTO ref_kanwil (kode, nama, singkatan, aktif, sumber) VALUES (@p1, N'KANTOR WILAYAH DJP UJI', N'KW DJP UJI', 1, N'manual')`, kode); err != nil {
		t.Fatal(err)
	}
	r, err := s.AmbilRefKanwil(ctx, kode)
	if err != nil || r == nil || r.Kode != kode || r.Nama != "KANTOR WILAYAH DJP UJI" || r.Singkatan != "KW DJP UJI" || !r.Aktif {
		t.Fatalf("terdaftar: %+v %v", r, err)
	}
	if got := sapa.TembusanKanwil(r.Nama); got != "Kepala Kantor Wilayah DJP Uji" {
		t.Errorf("saran tembusan = %q", got)
	}
	// singkatan kosong dibaca sebagai teks kosong, dan status nonaktif ikut terbaca
	if _, err := db.Exec(`UPDATE ref_kanwil SET singkatan = NULL, aktif = 0 WHERE kode = @p1`, kode); err != nil {
		t.Fatal(err)
	}
	if r, err := s.AmbilRefKanwil(ctx, kode); err != nil || r == nil || r.Singkatan != "" || r.Aktif {
		t.Errorf("nonaktif tanpa singkatan: %+v %v", r, err)
	}
}

// Daftar untuk pemilih tembusan hanya memuat Kanwil aktif, urut menurut urutan lalu kode.
func TestSapaDaftarRefKanwilAktifDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	kode := []string{"015990191", "015990192", "015990193"}
	bersih := func() {
		for _, k := range kode {
			db.Exec(`DELETE FROM ref_kanwil WHERE kode = N'` + k + `'`)
		}
	}
	bersih()
	if os.Getenv("PASTI_UJI_BIARKAN_DATA") != "1" {
		t.Cleanup(bersih)
	}
	// 191 urutan 20, 192 urutan 10 (tampil lebih dulu), 193 nonaktif
	for _, r := range []struct {
		kode   string
		urutan int
		aktif  int
	}{{kode[0], 20, 1}, {kode[1], 10, 1}, {kode[2], 5, 0}} {
		if _, err := db.Exec(`INSERT INTO ref_kanwil (kode, nama, singkatan, urutan, aktif, sumber) VALUES (@p1, N'KANTOR WILAYAH UJI ' + @p1, NULL, @p2, @p3, N'manual')`, r.kode, r.urutan, r.aktif); err != nil {
			t.Fatal(err)
		}
	}
	daftar, err := sapa.NewStore(db).DaftarRefKanwilAktif(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var urut []string
	for _, r := range daftar {
		if !r.Aktif {
			t.Errorf("Kanwil nonaktif ikut tampil: %+v", r)
		}
		for _, k := range kode {
			if r.Kode == k {
				urut = append(urut, k)
				if r.Singkatan != "" {
					t.Errorf("singkatan NULL harus terbaca kosong: %+v", r)
				}
			}
		}
	}
	if !reflect.DeepEqual(urut, []string{kode[1], kode[0]}) {
		t.Errorf("urutan uji = %v, want [%s %s] (urutan, lalu kode; nonaktif tidak ikut)", urut, kode[1], kode[0])
	}
}
