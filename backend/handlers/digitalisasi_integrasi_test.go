package handlers

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"pasti-v3-backend/digitalisasi"
)

// Tes integrasi (SQL Server sungguhan, lihat inaproc_integrasi_mssql_test.go): unduhan Digitalisasi Aset terhadap tabel DIGITALISASI_* asli
// (migrasi 020) untuk ketujuh dataset dan ketiga format. Data uji bertanda "UJI-DG-" pada Kode_Satker dan dibersihkan sebelum serta sesudah tes;
// ekspor dibatasi dengan pencarian "UJI-DG-<kunci>" sehingga data lain di tabel tidak ikut.

func bersihkanUjiDG(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, ds := range digitalisasi.Datasets {
		if _, err := db.Exec("DELETE FROM " + qc(ds.Table) + " WHERE Kode_Satker LIKE 'UJI-DG-%'"); err != nil {
			t.Fatalf("bersihkan %s: %v", ds.Table, err)
		}
	}
}

// barisUjiDG: satu baris uji berisi kolom-kolom yang berperan (menurut Roles), koordinat, dan satu kolom pribadi.
func barisUjiDG(ds digitalisasi.Dataset, n int) map[string]interface{} {
	m := map[string]interface{}{"Kode_Satker": fmt.Sprintf("UJI-DG-%s-%d", ds.Key, n), "Nama_Satker": "Satker uji " + ds.Key}
	isi := func(kolom string, v interface{}) {
		if kolom != "" {
			m[kolom] = v
		}
	}
	isi(ds.Roles.Uraian, fmt.Sprintf("Uraian uji %s %d", ds.Key, n))
	isi(ds.Roles.Provinsi, "Provinsi Uji")
	isi(ds.Roles.KabKota, "Kab Uji")
	isi(ds.Roles.Kondisi, "Baik")
	isi(ds.Roles.Luas, "1234.5678") // DECIMAL dikirim sebagai teks agar tidak ada selisih float
	isi(ds.Roles.Nilai, "1234567890123.45")
	if ds.Geo {
		m["Latitude"], m["Longitude"] = "-6.2088141", "106.8456000"
	}
	switch ds.Key {
	case "rumah_negara":
		m["Nama_Penghuni"] = "Rahasia Penghuni"
	case "satker":
		m["Jenis_Satker"] = "INDUK SATKER"
		m["Jumlah_KDJ"] = 3
	}
	return m
}

func sisipDG(t *testing.T, db *sql.DB, ds digitalisasi.Dataset, m map[string]interface{}) {
	t.Helper()
	var kolom, ph []string
	var args []interface{}
	for k, v := range m {
		kolom = append(kolom, qc(k))
		args = append(args, v)
		ph = append(ph, fmt.Sprintf("@p%d", len(args)))
	}
	if _, err := db.Exec(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", qc(ds.Table), strings.Join(kolom, ", "), strings.Join(ph, ", ")), args...); err != nil {
		t.Fatalf("sisip %s: %v", ds.Table, err)
	}
}

func TestEksporDigitalisasiSemuaDatasetDenganSQLServer(t *testing.T) {
	db := dbIntegrasi(t)
	bersihkanUjiDG(t, db)
	t.Cleanup(func() { bersihkanUjiDG(t, db) })
	for _, ds := range digitalisasi.Datasets {
		sisipDG(t, db, ds, barisUjiDG(ds, 1))
		sisipDG(t, db, ds, barisUjiDG(ds, 2))
	}

	for _, ds := range digitalisasi.Datasets {
		t.Run(ds.Key, func(t *testing.T) {
			cari := "?q=UJI-DG-" + ds.Key + "&format="
			for _, role := range []string{"user", "admin"} {
				r := dgRouter(role)
				admin := role == "admin"

				// ---- CSV: semua kolom (bukan hanya kolom tabel daftar) ----
				w := ambilBerkas(r, "/dg/ekspor/"+ds.Key+cari+"csv")
				if w.Code != 200 {
					t.Fatalf("%s csv: status %d: %s", role, w.Code, w.Body)
				}
				baris := strings.Split(strings.TrimRight(strings.TrimPrefix(w.Body.String(), "\xEF\xBB\xBF"), "\r\n"), "\r\n")
				if len(baris) != 3 {
					t.Fatalf("%s csv: %d baris, want 3 (judul + 2 data): %q", role, len(baris), baris)
				}
				nama, _ := dgKolomEkspor(ds, admin, false)
				if got := len(strings.Split(baris[0], ";")); got != len(nama) {
					t.Errorf("%s csv: %d kolom, want %d", role, got, len(nama))
				}
				if has := strings.Contains(w.Body.String(), "Rahasia Penghuni"); has != (ds.Key == "rumah_negara" && admin) {
					t.Errorf("%s csv: memuat data pribadi = %v (dataset %s)", role, has, ds.Key)
				}
				if strings.Contains(baris[0], "id sinkron") || strings.Contains(strings.ToLower(baris[0]), ";id;") {
					t.Errorf("%s csv: kolom teknis ikut diekspor: %s", role, baris[0])
				}

				// ---- Excel: bilangan sungguhan dan koordinat 7 desimal ----
				w = ambilBerkas(r, "/dg/ekspor/"+ds.Key+cari+"xlsx")
				if w.Code != 200 {
					t.Fatalf("%s xlsx: status %d: %s", role, w.Code, w.Body)
				}
				f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
				if err != nil {
					t.Fatalf("%s xlsx tidak sah: %v", role, err)
				}
				sheet := f.GetSheetName(0)
				rows, _ := f.GetRows(sheet)
				if len(rows) != 3 {
					t.Fatalf("%s xlsx: %d baris, want 3", role, len(rows))
				}
				kolom := map[string]int{}
				for i, j := range rows[0] {
					kolom[j] = i
				}
				ambil := func(judul string, mentah bool) string {
					i, ada := kolom[judul]
					if !ada {
						t.Errorf("%s xlsx: judul %q tidak ada", role, judul)
						return ""
					}
					sel, _ := excelize.CoordinatesToCellName(i+1, 2)
					if mentah {
						v, _ := f.GetCellValue(sheet, sel, excelize.Options{RawCellValue: true})
						return v
					}
					v, _ := f.GetCellValue(sheet, sel)
					return v
				}
				if ds.Roles.Luas != "" {
					if v := ambil(labelKolomDigitalisasi(ds.Roles.Luas), true); v != "1234.5678" {
						t.Errorf("%s xlsx: luas mentah = %q, want 1234.5678", role, v)
					}
				}
				if ds.Roles.Nilai != "" {
					if v := ambil(labelKolomDigitalisasi(ds.Roles.Nilai), true); v != "1234567890123.45" {
						t.Errorf("%s xlsx: nilai mentah = %q, want 1234567890123.45", role, v)
					}
				}
				if ds.Geo {
					if v := ambil("Lintang", false); v != "-6.2088141" {
						t.Errorf("%s xlsx: lintang tampil = %q, want -6.2088141", role, v)
					}
					if v := ambil("Bujur", false); v != "106.8456000" {
						t.Errorf("%s xlsx: bujur tampil = %q, want 106.8456000", role, v)
					}
				}
				if _, ada := kolom["Nama Penghuni"]; ada != (ds.Key == "rumah_negara" && admin) {
					t.Errorf("%s xlsx: kolom Nama Penghuni ada = %v", role, ada)
				}
				f.Close()

				// ---- PDF ----
				w = ambilBerkas(r, "/dg/ekspor/"+ds.Key+cari+"pdf")
				if w.Code != 200 || !strings.HasPrefix(w.Body.String(), "%PDF-") {
					t.Errorf("%s pdf: status %d, awal %q", role, w.Code, w.Body.String()[:min(8, w.Body.Len())])
				}
			}
		})
	}

	// Penyaring daftar ikut menyaring berkas: provinsi yang tidak cocok = tidak ada baris data (judul saja).
	w := ambilBerkas(dgRouter("user"), "/dg/ekspor/tanah?format=csv&q=UJI-DG-tanah&provinsi=Provinsi+Lain")
	if w.Code != 200 || strings.Count(strings.TrimRight(w.Body.String(), "\r\n"), "\r\n") != 0 {
		t.Errorf("penyaring provinsi: status %d, isi %q", w.Code, w.Body.String())
	}
	// Dan filter "belum punya koordinat" cocok dengan daftar (baris uji punya koordinat, jadi kosong).
	w = ambilBerkas(dgRouter("user"), "/dg/ekspor/tanah?format=csv&q=UJI-DG-tanah&tanpa_koordinat=1")
	if w.Code != 200 || strings.Count(strings.TrimRight(w.Body.String(), "\r\n"), "\r\n") != 0 {
		t.Errorf("tanpa_koordinat: status %d, isi %q", w.Code, w.Body.String())
	}
}
