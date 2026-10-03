package digitalisasi

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "tulis ulang file migrasi dari datasets.go")

func TestDatasetDefinitions(t *testing.T) {
	seenKey, seenTable := map[string]bool{}, map[string]bool{}
	for _, ds := range Datasets {
		if seenKey[ds.Key] || seenTable[ds.Table] {
			t.Errorf("%s: key/tabel ganda", ds.Key)
		}
		seenKey[ds.Key], seenTable[ds.Table] = true, true
		if !strings.HasPrefix(ds.Table, "DIGITALISASI_") {
			t.Errorf("%s: nama tabel %q harus berawalan DIGITALISASI_", ds.Key, ds.Table)
		}

		names := map[string]bool{}
		for _, c := range ds.Columns {
			lc := strings.ToLower(c.Name)
			if names[lc] {
				t.Errorf("%s: kolom %s ganda", ds.Key, c.Name)
			}
			names[lc] = true
			if lc == "id" || lc == "id_sinkron" || lc == "synced_at" {
				t.Errorf("%s: kolom %s bentrok dengan kolom bawaan tabel", ds.Key, c.Name)
			}
		}

		// Setiap nama peran harus menunjuk kolom yang ada.
		roles := map[string]string{
			"UE1": ds.Roles.UE1, "Satker": ds.Roles.Satker, "NamaSatker": ds.Roles.NamaSatker, "Uraian": ds.Roles.Uraian,
			"Kode": ds.Roles.Kode, "KodeRegister": ds.Roles.KodeRegister, "NUP": ds.Roles.NUP, "Alamat": ds.Roles.Alamat,
			"Kelurahan": ds.Roles.Kelurahan, "Kecamatan": ds.Roles.Kecamatan, "KabKota": ds.Roles.KabKota, "Provinsi": ds.Roles.Provinsi,
			"Luas": ds.Roles.Luas, "Nilai": ds.Roles.Nilai, "Kondisi": ds.Roles.Kondisi, "StatusHukum": ds.Roles.StatusHukum,
			"Foto": ds.Roles.Foto, "Asuransi": ds.Roles.Asuransi, "StatusPenghuni": ds.Roles.StatusPenghuni,
		}
		for role, col := range roles {
			if col == "" {
				continue
			}
			if _, ok := ds.Column(col); !ok {
				t.Errorf("%s: peran %s menunjuk kolom %q yang tidak ada", ds.Key, role, col)
			}
		}
		for _, col := range ds.SearchColumns {
			c, ok := ds.Column(col)
			if !ok {
				t.Errorf("%s: kolom pencarian %q tidak ada", ds.Key, col)
			} else if c.Sensitive {
				t.Errorf("%s: kolom pribadi %q tidak boleh dicari", ds.Key, col)
			}
		}
		for _, col := range append([]string{ds.KeyColumn}, ds.Keep...) {
			if col == "" {
				continue
			}
			if _, ok := ds.Column(col); !ok {
				t.Errorf("%s: kolom Keep/Key %q tidak ada", ds.Key, col)
			}
		}
		if len(ds.Keep) > 0 && ds.KeyColumn == "" {
			t.Errorf("%s: Keep butuh KeyColumn", ds.Key)
		}

		lat, lng := coordIndexes(ds)
		if ds.Geo != (lat >= 0 && lng >= 0) {
			t.Errorf("%s: Geo=%v tidak sejalan dengan kolom Latitude/Longitude", ds.Key, ds.Geo)
		}
	}
}

// Setiap kolom tujuan harus punya alias "AS <nama>" di query bawaannya: menangkap salah ketik di sini maupun
// query yang disunting tanpa menyesuaikan daftar kolom.
func TestEveryColumnHasAnAliasInItsQuery(t *testing.T) {
	for _, ds := range Datasets {
		q, err := ds.Query()
		if err != nil {
			t.Fatalf("%s: %v", ds.Key, err)
		}
		for _, c := range ds.Columns {
			re := regexp.MustCompile(`(?i)\bAS\s+` + regexp.QuoteMeta(c.SourceName()) + `\b`)
			if !re.MatchString(q) {
				t.Errorf("%s: query %s tidak punya alias AS %s", ds.Key, ds.QueryFile, c.SourceName())
			}
		}
	}
}

func TestQueriesAreReadOnly(t *testing.T) {
	forbidden := regexp.MustCompile(`(?i)\b(INSERT\s+INTO|UPDATE\s+\S+\s+SET|DELETE\s+FROM|TRUNCATE|MERGE\s|ALTER\s|EXEC(UTE)?\s|CREATE\s+(TABLE|PROC|VIEW|FUNCTION))`)
	for _, ds := range Datasets {
		q, _ := ds.Query()
		code := regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(q, "")
		if m := forbidden.FindString(code); m != "" {
			t.Errorf("%s: query memuat perintah yang mengubah data: %q", ds.Key, m)
		}
		// DROP TABLE hanya untuk tabel #sementara milik sesi, tidak pernah tabel SLDK.
		for _, m := range regexp.MustCompile(`(?i)\bDROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?([^;]+);`).FindAllStringSubmatch(code, -1) {
			for _, name := range strings.Split(m[1], ",") {
				if !strings.HasPrefix(strings.TrimSpace(name), "#") {
					t.Errorf("%s: DROP TABLE %q bukan tabel #sementara", ds.Key, strings.TrimSpace(name))
				}
			}
		}
		// Tabel sementara hanya boleh berupa #temp (milik sesi), tidak menyentuh tabel SLDK.
		for _, m := range regexp.MustCompile(`(?i)\bINTO\s+(\S+)`).FindAllStringSubmatch(code, -1) {
			if !strings.HasPrefix(m[1], "#") {
				t.Errorf("%s: SELECT INTO ke %q bukan tabel #sementara", ds.Key, m[1])
			}
		}
	}
}

func TestGeoQueriesSelectCoordinates(t *testing.T) {
	for _, ds := range Datasets {
		q, _ := ds.Query()
		has := strings.Contains(strings.ToLower(q), "gps_latitude") && strings.Contains(strings.ToLower(q), "gps_longitude")
		if ds.Geo != has {
			t.Errorf("%s: Geo=%v tetapi query memuat koordinat=%v", ds.Key, ds.Geo, has)
		}
	}
}

// Frontend memilih sendiri kolom tabel dan kolom judul per dataset (components/digitalisasi/digitalisasi.ts).
// Tes ini menjaga agar nama kolom di sana tetap ada di definisi backend, tampil di daftar, dan bukan kolom pribadi.
func TestFrontendColumnsExistInBackend(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "frontend", "components", "digitalisasi", "digitalisasi.ts"))
	if err != nil {
		t.Skipf("kode frontend tidak tersedia: %v", err)
	}
	text := string(src)

	titleBlock := regexp.MustCompile(`(?s)export const TITLE_COLUMN[^{]*\{(.*?)\n\};`).FindStringSubmatch(text)
	if titleBlock == nil {
		t.Fatal("blok TITLE_COLUMN tidak ditemukan di digitalisasi.ts")
	}
	for _, ds := range Datasets {
		block := regexp.MustCompile(`(?s)\n  ` + ds.Key + `: \[(.*?)\n  \],`).FindStringSubmatch(text)
		if block == nil {
			t.Errorf("%s: TABLE_COLUMNS tidak ditemukan di digitalisasi.ts", ds.Key)
			continue
		}
		names := regexp.MustCompile(`name: "([^"]+)"`).FindAllStringSubmatch(block[1], -1)
		if len(names) == 0 {
			t.Errorf("%s: TABLE_COLUMNS kosong", ds.Key)
		}
		for _, m := range names {
			c, ok := ds.Column(m[1])
			switch {
			case !ok:
				t.Errorf("%s: frontend memakai kolom %q yang tidak ada di backend", ds.Key, m[1])
			case !c.Display:
				t.Errorf("%s: kolom %q dipakai frontend tetapi tidak dikirim di daftar (bukan Display)", ds.Key, m[1])
			case c.Sensitive:
				t.Errorf("%s: kolom pribadi %q tidak boleh jadi kolom tabel bawaan", ds.Key, m[1])
			}
		}

		title := regexp.MustCompile(`\n  ` + ds.Key + `: "([^"]+)",`).FindStringSubmatch(titleBlock[1])
		if title == nil {
			t.Errorf("%s: TITLE_COLUMN tidak ditemukan", ds.Key)
		} else if _, ok := ds.Column(title[1]); !ok {
			t.Errorf("%s: kolom judul %q tidak ada di backend", ds.Key, title[1])
		}
	}
}

func TestMigrationFile(t *testing.T) {
	path := filepath.Join("..", "migrations", MigrationFile)
	want := MigrationSQL()
	if *update {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (buat dengan: go test ./digitalisasi -run TestMigrationFile -update)", err)
	}
	got := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if got != want {
		t.Fatalf("migrasi %s tidak sejalan dengan datasets.go; jalankan: go test ./digitalisasi -run TestMigrationFile -update", MigrationFile)
	}
}

func TestMigrationIsIdempotentAndSplitsIntoSafeBatches(t *testing.T) {
	sql := MigrationSQL()
	for _, ds := range Datasets {
		if !strings.Contains(sql, "IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = '"+ds.Table+"')") {
			t.Errorf("%s: CREATE TABLE harus idempoten", ds.Table)
		}
	}
	if strings.Contains(sql, "\nGO") {
		t.Error("migrasi tidak memakai pemisah GO")
	}
	if strings.Count(sql, "BEGIN\n") != strings.Count(sql, "END;") {
		t.Error("BEGIN/END tidak berpasangan")
	}
}
