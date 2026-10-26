package digitalisasi

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestHistoryMigrationFile(t *testing.T) {
	path := filepath.Join("..", "migrations", HistoryMigrationFile)
	want := HistoryMigrationSQL()
	if *update {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (buat dengan: go test ./digitalisasi -run TestHistoryMigrationFile -update)", err)
	}
	if got := strings.ReplaceAll(string(raw), "\r\n", "\n"); got != want {
		t.Fatalf("migrasi %s tidak sejalan dengan riwayat.go; jalankan: go test ./digitalisasi -run TestHistoryMigrationFile -update", HistoryMigrationFile)
	}
}

func TestHistoryMigrationIsIdempotentAndSafe(t *testing.T) {
	sql := HistoryMigrationSQL()
	if !strings.Contains(sql, "IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = '"+HistoryTable+"')") {
		t.Error("CREATE TABLE harus idempoten")
	}
	if strings.Contains(sql, "\nGO") {
		t.Error("migrasi tidak memakai pemisah GO")
	}
	if strings.Count(sql, "BEGIN\n") != strings.Count(sql, "END;") {
		t.Error("BEGIN/END tidak berpasangan")
	}
	// Penjaga bentuk data: periode hanya akhir bulan, data_tambahan hanya JSON sah. Migrasi tidak boleh membuang atau mengubah tabel lain.
	for _, perlu := range []string{"CHECK (periode = EOMONTH(periode))", "ISJSON(data_tambahan) = 1", "CREATE CLUSTERED INDEX"} {
		if !strings.Contains(sql, perlu) {
			t.Errorf("migrasi tidak memuat %q", perlu)
		}
	}
	if regexp.MustCompile(`(?i)\b(DROP|ALTER|TRUNCATE|DELETE|UPDATE)\b`).MatchString(regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(sql, "")) {
		t.Error("migrasi riwayat hanya boleh membuat tabel baru")
	}
}

// Kolom inti tidak boleh memotong nilai dari dataset mana pun: jenis sama, panjang teks cukup, skala desimal sama.
func TestHistoryCoreColumnsFitEverySource(t *testing.T) {
	for _, ds := range Datasets {
		for _, h := range historyColumns {
			src := h.source(ds)
			if src == "" {
				continue
			}
			c, ok := ds.Column(src)
			if !ok {
				t.Errorf("%s: kolom %q untuk %s tidak ada", ds.Key, src, h.Name)
				continue
			}
			if c.Kind != h.Kind {
				t.Errorf("%s.%s (%s): jenis kolom sumber berbeda dengan kolom inti %s", ds.Key, c.Name, c.SQLType(), h.Name)
				continue
			}
			switch h.Kind {
			case Text:
				if h.Size != 0 && (c.Size == 0 || c.Size > h.Size) {
					t.Errorf("%s.%s (%s) lebih panjang daripada kolom inti %s (%s): nilai dapat terpotong", ds.Key, c.Name, c.SQLType(), h.Name, h.sqlType())
				}
			case Decimal:
				if c.Scale != h.Scale {
					t.Errorf("%s.%s: skala %d berbeda dengan kolom inti %s (skala %d)", ds.Key, c.Name, c.Scale, h.Name, h.Scale)
				}
			}
		}
	}
}

// Setiap kolom dataset punya tepat satu nasib: kolom inti, JSON, atau dikecualikan. Kolom baru di datasets.go otomatis masuk JSON kecuali dikecualikan, dan tidak ada yang hilang diam-diam.
func TestHistoryEveryColumnHasExactlyOneDestination(t *testing.T) {
	reserved := map[string]bool{"id": true, "periode": true, "jenis_aset": true, "data_tambahan": true, "data_per": true, "diambil_pada": true}
	seenCore := map[string]bool{}
	for _, h := range historyColumns {
		if reserved[h.Name] || seenCore[h.Name] {
			t.Errorf("kolom inti %q bentrok atau ganda", h.Name)
		}
		seenCore[h.Name] = true
	}
	for _, ds := range Datasets {
		core := map[string]bool{}
		for _, src := range ds.HistoryCoreSources() {
			if core[strings.ToLower(src)] {
				t.Errorf("%s: kolom %s dipetakan ke dua kolom inti", ds.Key, src)
			}
			core[strings.ToLower(src)] = true
		}
		extra := map[string]bool{}
		for _, c := range ds.HistoryExtras() {
			extra[strings.ToLower(c.Name)] = true
		}
		for _, c := range ds.Columns {
			n := strings.ToLower(c.Name)
			tujuan := 0
			for _, ada := range []bool{core[n], extra[n], c.HistoryExcluded()} {
				if ada {
					tujuan++
				}
			}
			// kolom yang dikecualikan tidak boleh ikut inti atau JSON; kolom lain harus tepat satu
			if tujuan != 1 {
				t.Errorf("%s.%s: inti=%v json=%v dikecualikan=%v (harus tepat satu)", ds.Key, c.Name, core[n], extra[n], c.HistoryExcluded())
			}
		}
	}
}

// Data pribadi, foto, dan nama pengguna tidak pernah masuk riwayat, baik sebagai kolom inti, JSON, maupun di SQL penyimpanannya.
func TestHistoryNeverCopiesPersonalOrPhotoColumns(t *testing.T) {
	for _, ds := range Datasets {
		sql := HistorySQL(ds)
		dikecualikan := map[string]bool{}
		for _, c := range ds.Columns {
			if c.Sensitive || strings.HasPrefix(strings.ToLower(c.Name), "foto") || strings.EqualFold(c.Name, "Nama_Pengguna") {
				dikecualikan[c.Name] = true
				if !c.HistoryExcluded() {
					t.Errorf("%s.%s harus dikecualikan dari riwayat", ds.Key, c.Name)
				}
			}
		}
		// peran Foto selalu dikecualikan
		if ds.Roles.Foto != "" {
			if c, _ := ds.Column(ds.Roles.Foto); !c.HistoryExcluded() {
				t.Errorf("%s: kolom foto %s harus dikecualikan", ds.Key, ds.Roles.Foto)
			}
		}
		for n := range dikecualikan {
			if regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(n) + `\b`).MatchString(sql) {
				t.Errorf("%s: SQL riwayat menyebut kolom terlarang %s", ds.Key, n)
			}
		}
	}
	// Dataset rumah negara memang punya kolom pribadi; pastikan tes ini bermakna.
	rn, _ := ByKey("rumah_negara")
	if c, ok := rn.Column("Nama_Penghuni"); !ok || !c.Sensitive {
		t.Fatal("asumsi tes: Nama_Penghuni pada rumah_negara bertanda Sensitive")
	}
}

// bagiTingkatAtas memecah daftar dipisah koma pada tingkat teratas (koma di dalam tanda kurung dan teks dilewati).
func bagiTingkatAtas(s string) []string {
	var out []string
	awal, kurung, kutip := 0, 0, false
	for i, r := range s {
		switch {
		case r == '\'':
			kutip = !kutip
		case kutip:
		case r == '(':
			kurung++
		case r == ')':
			kurung--
		case r == ',' && kurung == 0:
			out = append(out, strings.TrimSpace(s[awal:i]))
			awal = i + 1
		}
	}
	return append(out, strings.TrimSpace(s[awal:]))
}

func TestHistorySQLShape(t *testing.T) {
	semuaKolom := append([]string{"periode", "jenis_aset"}, HistoryColumnNames()...)
	semuaKolom = append(semuaKolom, "data_tambahan", "data_per")
	re := regexp.MustCompile(`(?s)INSERT INTO ` + HistoryTable + ` \((.*?)\)\nSELECT (.*)\nFROM (\S+) s\nORDER BY s\.id;`)
	for _, ds := range Datasets {
		sql := HistorySQL(ds)
		m := re.FindStringSubmatch(sql)
		if m == nil {
			t.Fatalf("%s: bentuk INSERT tidak dikenali:\n%s", ds.Key, sql)
		}
		kolom, nilai := bagiTingkatAtas(m[1]), bagiTingkatAtas(m[2])
		if !reflect.DeepEqual(kolom, semuaKolom) {
			t.Errorf("%s: daftar kolom INSERT %v, want %v", ds.Key, kolom, semuaKolom)
		}
		if len(nilai) != len(kolom) {
			t.Errorf("%s: %d nilai untuk %d kolom", ds.Key, len(nilai), len(kolom))
		}
		if m[3] != ds.Table {
			t.Errorf("%s: sumber %s, want %s", ds.Key, m[3], ds.Table)
		}
		if nilai[0] != "@periode" || nilai[1] != "N'"+ds.Key+"'" || nilai[len(nilai)-1] != "@data_per" {
			t.Errorf("%s: nilai awal/akhir salah: %v ... %v", ds.Key, nilai[:2], nilai[len(nilai)-1])
		}
		// setiap s.Kolom yang dibaca ada pada dataset
		for _, r := range regexp.MustCompile(`\bs\.(\w+)`).FindAllStringSubmatch(sql, -1) {
			if r[1] == "id" {
				continue
			}
			if _, ok := ds.Column(r[1]); !ok {
				t.Errorf("%s: SQL membaca kolom %s yang tidak ada", ds.Key, r[1])
			}
		}
		for _, perlu := range []string{
			"SET XACT_ABORT ON;", "BEGIN TRANSACTION;", "COMMIT TRANSACTION;",
			"DELETE FROM " + HistoryTable + " WHERE periode = @periode AND jenis_aset = N'" + ds.Key + "';",
			"SELECT @baris AS baris;",
		} {
			if !strings.Contains(sql, perlu) {
				t.Errorf("%s: SQL tidak memuat %q", ds.Key, perlu)
			}
		}
		// hanya membaca tabel dataset itu dan hanya menulis ke tabel riwayat
		if strings.Count(sql, "INSERT INTO") != 1 || strings.Count(sql, "DELETE FROM") != 1 || regexp.MustCompile(`(?i)\b(DROP|ALTER|TRUNCATE|UPDATE|MERGE)\b`).MatchString(sql) {
			t.Errorf("%s: SQL riwayat memuat perintah yang tidak semestinya", ds.Key)
		}
	}
}

func TestHistoryMappingExamples(t *testing.T) {
	tanah, _ := ByKey("tanah")
	if got := tanah.HistoryCoreSources(); got["luas"] != "Luas_Tanah" || got["nilai"] != "Nilai_Tanah" || got["kode_register"] != "kode_register_tanah" || got["latitude"] != "Latitude" || got["status_hukum"] != "Status_hukum" {
		t.Errorf("pemetaan tanah salah: %v", got)
	}
	var namaTanah []string
	for _, c := range tanah.HistoryExtras() {
		namaTanah = append(namaTanah, c.Name)
	}
	// luas bangunan di atas tanah dan jumlah bangunan adalah kolom khusus tanah, bukan kolom inti
	for _, perlu := range []string{"id_aset_tanah", "Luas_Bangunan", "Jumlah_Bangunan"} {
		if !contains(namaTanah, perlu) {
			t.Errorf("tanah: %s harus ada di JSON, extras = %v", perlu, namaTanah)
		}
	}

	satker, _ := ByKey("satker")
	if got := satker.HistoryCoreSources(); got["kode_satker"] != "Kode_Satker" || got["kode_barang"] != "" || got["latitude"] != "" {
		t.Errorf("pemetaan satker salah: %v", got)
	}
	var namaSatker []string
	for _, c := range satker.HistoryExtras() {
		namaSatker = append(namaSatker, c.Name)
	}
	for _, perlu := range []string{"Jenis_Satker", "Jumlah_KDJ", "Kondisi_KDJ", "Jumlah_KDO", "Kondisi_KDO", "Status_Gedung_Kantor"} {
		if !contains(namaSatker, perlu) {
			t.Errorf("satker: %s harus ada di JSON, extras = %v", perlu, namaSatker)
		}
	}
	if contains(namaSatker, "Foto") {
		t.Error("satker: Foto tidak boleh masuk JSON")
	}

	rusun, _ := ByKey("rusunara")
	var namaRusun []string
	for _, c := range rusun.HistoryExtras() {
		namaRusun = append(namaRusun, c.Name)
	}
	if !reflect.DeepEqual(namaRusun, []string{"RTRW_Rusun", "Kamar_tipe_E", "Kamar_tipe_D", "Kamar_tipe_C"}) {
		t.Errorf("rusunara: extras = %v", namaRusun)
	}
}

func contains(daftar []string, s string) bool {
	for _, x := range daftar {
		if x == s {
			return true
		}
	}
	return false
}
