package datamart

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"pasti-v3-backend/digitalisasi"
)

// Tes statis tanpa database: script datamart harus selalu mencakup apa yang dipakai query Digitalisasi Aset. Bila sebuah query diubah (kolom, tabel, atau kelompok kode barang
// baru) dan script belum disesuaikan, tes ini gagal, bukan datamart yang diam-diam tidak dapat dipakai saat SLDK sedang bermasalah.

const berkasSkrip = "buat_datamart_sldk.sql"

func bacaSkrip(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(berkasSkrip)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

func rapatkan(s string) string { return strings.Join(strings.Fields(s), " ") }

// tanpaKomentar membuang komentar -- sampai akhir baris (komentar tidak boleh ikut dihitung sebagai kolom atau tabel).
func tanpaKomentar(s string) string {
	return regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(s, "")
}

// tabelSumber: tabel SLDK -> kolom yang disalin skrip (gabungan semua pernyataan SELECT ... INTO ... FROM $(SUMBER).DJKN.<tabel> AS S).
func tabelSumber(t *testing.T, skrip string) map[string]map[string]bool {
	t.Helper()
	re := regexp.MustCompile(`(?is)SELECT\s+(.*?)\s+INTO\s+(\S+)\s+FROM\s+\$\(SUMBER\)\.DJKN\.(\w+)\s+AS\s+S\b`)
	reKol := regexp.MustCompile(`\bS\.(\w+)`)
	out := map[string]map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(tanpaKomentar(skrip), -1) {
		tabel := strings.ToUpper(m[3])
		if out[tabel] == nil {
			out[tabel] = map[string]bool{}
		}
		for _, k := range reKol.FindAllStringSubmatch(m[1], -1) {
			out[tabel][strings.ToLower(k[1])] = true
		}
	}
	if len(out) == 0 {
		t.Fatal("tidak ada pernyataan SELECT ... INTO ... FROM $(SUMBER).DJKN.<tabel> AS S di script")
	}
	return out
}

// rujukan: satu pemakaian kolom tabel SLDK oleh sebuah query dataset.
type rujukan struct {
	dataset string
	tabel   string
	alias   string
	kolom   string
}

// lingkup: satu blok bertanda kurung (atau tingkat teratas) dari sebuah query, terbagi menjadi pernyataan (dipisah titik koma). Alias hanya berlaku di lingkupnya (dan lingkup
// anaknya, untuk subquery berkorelasi); alias yang sama dipakai ulang di CTE atau subquery lain tanpa saling mengganggu.
type lingkup struct {
	induk       *lingkup
	indukPotong int
	potong      []*strings.Builder
}

var (
	reLiteral = regexp.MustCompile(`N?'(?:[^']|'')*'`)
	reIkat    = regexp.MustCompile(`(?i)\b(?:FROM|JOIN|APPLY)\s+((?:\w+\.)?\w+|\(#\d+\))(?:\s+AS)?\s+(\w+)`)
	reRef     = regexp.MustCompile(`\b([A-Za-z_]\w*)\.([A-Za-z_]\w*)\b`)
)

// uraiLingkup memecah query (komentar sudah dibuang) menjadi lingkup. Isi literal teks dikosongkan agar tanda kurung atau titik koma di dalamnya tidak mengacaukan.
func uraiLingkup(q string) []*lingkup {
	q = reLiteral.ReplaceAllString(q, "''")
	akar := &lingkup{potong: []*strings.Builder{{}}}
	semua := []*lingkup{akar}
	sekarang := akar
	for _, r := range q {
		switch r {
		case '(':
			anak := &lingkup{induk: sekarang, indukPotong: len(sekarang.potong) - 1, potong: []*strings.Builder{{}}}
			semua = append(semua, anak)
			sekarang.potong[len(sekarang.potong)-1].WriteString("(#" + strconv.Itoa(len(semua)-1) + ")")
			sekarang = anak
		case ')':
			if sekarang.induk != nil {
				sekarang = sekarang.induk
			}
		case ';':
			sekarang.potong = append(sekarang.potong, &strings.Builder{})
		default:
			sekarang.potong[len(sekarang.potong)-1].WriteRune(r)
		}
	}
	return semua
}

// ikatan: alias (huruf kecil) -> nama tabel SLDK (huruf besar) bila alias itu terikat langsung ke DJKN.<tabel>, atau "" bila terikat ke CTE, tabel sementara, atau subquery.
func (l *lingkup) ikatan(potong int) map[string]string {
	out := map[string]string{}
	for _, m := range reIkat.FindAllStringSubmatch(l.potong[potong].String(), -1) {
		alias := strings.ToLower(m[2])
		switch alias {
		case "on", "where", "inner", "left", "right", "outer", "cross", "full", "with", "option", "group", "order", "union", "join", "set":
			continue // bukan alias (FROM DJKN.X WHERE ..., JOIN Y ON ...)
		}
		if t, ok := strings.CutPrefix(strings.ToUpper(m[1]), "DJKN."); ok {
			out[alias] = t
		} else {
			out[alias] = ""
		}
	}
	return out
}

func (l *lingkup) cari(potong int, alias string) (string, bool) {
	for cur, idx := l, potong; cur != nil; cur, idx = cur.induk, cur.indukPotong {
		if t, ok := cur.ikatan(idx)[alias]; ok {
			return t, true
		}
	}
	return "", false
}

// pemakaianQuery: tabel SLDK yang dirujuk query (DJKN.<tabel>) dan kolom yang dirujuk lewat alias yang terikat langsung ke tabel SLDK pada lingkupnya.
func pemakaianQuery(t *testing.T) (tabel map[string]bool, kolom []rujukan) {
	t.Helper()
	tabel = map[string]bool{}
	reTabel := regexp.MustCompile(`(?i)\bDJKN\.(\w+)`)
	for _, ds := range digitalisasi.Datasets {
		q, err := ds.Query()
		if err != nil {
			t.Fatal(err)
		}
		q = tanpaKomentar(q)
		for _, m := range reTabel.FindAllStringSubmatch(reLiteral.ReplaceAllString(q, "''"), -1) {
			tabel[strings.ToUpper(m[1])] = true
		}
		for _, l := range uraiLingkup(q) {
			for i, sb := range l.potong {
				for _, m := range reRef.FindAllStringSubmatch(sb.String(), -1) {
					alias := strings.ToLower(m[1])
					if alias == "djkn" || alias == "dbo" {
						continue
					}
					if tb, ok := l.cari(i, alias); ok && tb != "" {
						kolom = append(kolom, rujukan{dataset: ds.Key, tabel: tb, alias: alias, kolom: strings.ToLower(m[2])})
					}
				}
			}
		}
	}
	return tabel, kolom
}

func TestSemuaTabelQueryAdaDiDatamart(t *testing.T) {
	skrip := tabelSumber(t, bacaSkrip(t))
	dipakai, _ := pemakaianQuery(t)
	if len(dipakai) == 0 {
		t.Fatal("tidak ada tabel DJKN yang terbaca dari query")
	}
	for tb := range dipakai {
		if skrip[tb] == nil {
			t.Errorf("query memakai DJKN.%s tetapi script datamart tidak menyalinnya", tb)
		}
	}
	for tb := range skrip {
		if !dipakai[tb] {
			t.Errorf("script datamart menyalin DJKN.%s yang tidak dipakai query mana pun (data berlebih; hapus dari script)", tb)
		}
	}
}

func TestSemuaKolomQueryTercakupDatamart(t *testing.T) {
	skrip := tabelSumber(t, bacaSkrip(t))
	_, rujuk := pemakaianQuery(t)
	if len(rujuk) < 50 {
		t.Fatalf("hanya %d rujukan kolom terbaca dari query; pembaca alias rusak?", len(rujuk))
	}
	for _, r := range rujuk {
		if !skrip[r.tabel][r.kolom] {
			t.Errorf("query %s memakai %s.%s (tabel %s) yang tidak disalin script datamart", r.dataset, r.alias, r.kolom, r.tabel)
		}
	}
}

// Tabel rujukan yang `SELECT S.*`-nya tidak boleh bergantung pada kolom yang tidak disalin: satker.sql memakai S.* dari SIMAN2_R_SATKER, dan hanya kolom yang dirujuk
// berikutnya (nama kolom hasil) yang berarti. Tes ini menegaskan bahwa query yang memakai S.* tetap hanya satker.sql.
func TestHanyaSatkerMemakaiBintang(t *testing.T) {
	for _, ds := range digitalisasi.Datasets {
		q, _ := ds.Query()
		q = tanpaKomentar(q)
		if regexp.MustCompile(`(?i)\b\w+\.\*`).MatchString(q) && ds.Key != "satker" {
			t.Errorf("dataset %s memakai alias.* sehingga kolom yang dibutuhkannya tidak dapat dipastikan dari rujukan; tulis kolomnya satu per satu atau tinjau script datamart", ds.Key)
		}
	}
}

// Kriteria "aset inti" (langkah 2) dan "bukan aset inti" (CASE WHEN di langkah 4) harus sama persis, kalau tidak tabel aset di datamart bocor atau terduplikasi.
func TestKriteriaAsetIntiKonsisten(t *testing.T) {
	skrip := tanpaKomentar(bacaSkrip(t))
	reInti := regexp.MustCompile(`(?is)INTO #aset_inti\s+FROM \$\(SUMBER\)\.DJKN\.SIMAN2_M_ASET AS S\s+WHERE\s+(.*?);`)
	reTautan := regexp.MustCompile(`(?is)CASE WHEN\s*\((.*?)\)\s*THEN 1 ELSE 0 END = 0`)
	mi, mt := reInti.FindStringSubmatch(skrip), reTautan.FindStringSubmatch(skrip)
	if mi == nil || mt == nil {
		t.Fatal("kriteria aset inti atau CASE WHEN aset tautan tidak ditemukan di script")
	}
	if a, b := rapatkan(strings.ReplaceAll(mi[1], " ", "")), rapatkan(strings.ReplaceAll(mt[1], " ", "")); a != b {
		t.Errorf("kriteria aset inti berbeda dengan kriteria pada CASE WHEN aset tautan:\n inti   : %s\n tautan : %s", rapatkan(mi[1]), rapatkan(mt[1]))
	}
}

// Setiap penyaring kode barang aset utama pada query (kd_brg LIKE 'x%' atau = 'x') harus tercakup kriteria aset inti, dan kode kendaraan satker.sql harus ada di daftar kendaraan.
func TestKelompokKodeBarangQueryTercakup(t *testing.T) {
	skrip := tanpaKomentar(bacaSkrip(t))
	reInti := regexp.MustCompile(`(?is)INTO #aset_inti\s+FROM \$\(SUMBER\)\.DJKN\.SIMAN2_M_ASET AS S\s+WHERE\s+(.*?);`)
	inti := reInti.FindStringSubmatch(skrip)
	if inti == nil {
		t.Fatal("kriteria aset inti tidak ditemukan")
	}
	var awalan []string
	for _, m := range regexp.MustCompile(`(?i)kd_brg LIKE N'(\d+)%'`).FindAllStringSubmatch(inti[1], -1) {
		awalan = append(awalan, m[1])
	}
	kendaraan := map[string]bool{}
	if m := regexp.MustCompile(`(?is)kd_brg IN \((.*?)\)`).FindStringSubmatch(inti[1]); m != nil {
		for _, k := range regexp.MustCompile(`N'(\d+)'`).FindAllStringSubmatch(m[1], -1) {
			kendaraan[k[1]] = true
		}
	}
	if len(awalan) == 0 || len(kendaraan) == 0 {
		t.Fatalf("awalan kode barang (%d) atau daftar kendaraan (%d) tidak terbaca", len(awalan), len(kendaraan))
	}
	tercakup := func(kode string, tepat bool) bool {
		if tepat && kendaraan[kode] {
			return true
		}
		for _, a := range awalan {
			if strings.HasPrefix(kode, a) {
				return true
			}
		}
		return false
	}
	reKode := regexp.MustCompile(`(?i)\bkd_brg\s+(LIKE|=|IN)\s*(?:N'(\d+)(%?)'|\(([^)]*)\))`)
	periksa := 0
	for _, ds := range digitalisasi.Datasets {
		q, _ := ds.Query()
		q = tanpaKomentar(q)
		for _, m := range reKode.FindAllStringSubmatch(q, -1) {
			// m[1] operator, m[2] kode, m[3] '%' bila awalan, m[4] daftar IN
			var kode []string
			switch {
			case strings.EqualFold(m[1], "IN"):
				for _, k := range regexp.MustCompile(`N'(\d+)'`).FindAllStringSubmatch(m[4], -1) {
					kode = append(kode, k[1])
				}
			default:
				kode = []string{m[2]}
			}
			for _, k := range kode {
				periksa++
				if !tercakup(k, true) {
					t.Errorf("dataset %s menyaring kd_brg %s%s yang tidak tercakup kriteria aset inti script datamart", ds.Key, k, m[3])
				}
			}
		}
	}
	if periksa < 8 {
		t.Fatalf("hanya %d penyaring kode barang terbaca dari query; pembaca regex rusak?", periksa)
	}
	// Semua kode kendaraan di satker.sql ada di daftar script, dan sebaliknya (tanpa kode kendaraan yang tak terpakai).
	q := ""
	for _, ds := range digitalisasi.Datasets {
		if ds.Key == "satker" {
			q, _ = ds.Query()
		}
	}
	dariQuery := map[string]bool{}
	for _, m := range regexp.MustCompile(`N'(3020\d+)'`).FindAllStringSubmatch(tanpaKomentar(q), -1) {
		dariQuery[m[1]] = true
	}
	for k := range dariQuery {
		if !kendaraan[k] {
			t.Errorf("kode kendaraan %s di satker.sql tidak ada di daftar kendaraan script datamart", k)
		}
	}
	for k := range kendaraan {
		if !dariQuery[k] {
			t.Errorf("kode kendaraan %s di script datamart tidak dipakai satker.sql", k)
		}
	}
}

// Awalan satker dan status aset yang disaring query (kd_satker LIKE '015%', status_bmn_yn = 'Y') harus ada di kriteria aset inti: kalau tidak, datamart memuat satker atau
// status yang salah (kosong atau berlebih) walau kolom dan kode barangnya benar. Kriteria satker dan status juga harus berlaku untuk SEMUA kelompok kode barang, bukan satu cabang saja.
func TestPenyaringSatkerDanStatusQueryTercakup(t *testing.T) {
	skrip := tanpaKomentar(bacaSkrip(t))
	inti := regexp.MustCompile(`(?is)INTO #aset_inti\s+FROM \$\(SUMBER\)\.DJKN\.SIMAN2_M_ASET AS S\s+WHERE\s+(.*?);`).FindStringSubmatch(skrip)
	if inti == nil {
		t.Fatal("kriteria aset inti tidak ditemukan")
	}
	k := rapatkan(inti[1])
	awalanSatker := map[string]bool{}
	for _, ds := range digitalisasi.Datasets {
		q, _ := ds.Query()
		q = tanpaKomentar(q)
		for _, m := range regexp.MustCompile(`(?i)\bkd_satker LIKE N'(\d+)%'`).FindAllStringSubmatch(q, -1) {
			awalanSatker[m[1]] = true
		}
		if strings.Contains(q, "DJKN.SIMAN2_M_ASET") && !strings.Contains(q, "status_bmn_yn = N'Y'") {
			t.Errorf("dataset %s tidak lagi menyaring status_bmn_yn = Y; tinjau kriteria aset inti script datamart", ds.Key)
		}
	}
	if len(awalanSatker) != 1 || !awalanSatker["015"] {
		t.Fatalf("awalan satker pada query berubah: %v; sesuaikan script datamart (R_SATKER dan aset inti) lalu tes ini", awalanSatker)
	}
	for _, perlu := range []string{"S.status_bmn_yn = N'Y' AND S.kd_satker LIKE N'015%' AND ("} {
		if !strings.HasPrefix(k, perlu) {
			t.Errorf("kriteria aset inti harus diawali %q (status dan satker berlaku untuk semua kelompok kode barang): %s", perlu, k)
		}
	}
	if !strings.Contains(rapatkan(skrip), "FROM $(SUMBER).DJKN.SIMAN2_R_SATKER AS S WHERE S.kd_satker LIKE N'015%'") {
		t.Error("salinan SIMAN2_R_SATKER harus menyaring kd_satker LIKE '015%'")
	}
}

// SLDK hanya dibaca. Script dijalankan dengan hak baca ke SLDK; setiap kemunculan $(SUMBER) harus berada di klausa FROM (atau literal pencatat), tidak pernah di tujuan perubahan.
func TestSkripHanyaMembacaSLDK(t *testing.T) {
	skrip := tanpaKomentar(bacaSkrip(t))
	n := 0
	for _, baris := range strings.Split(skrip, "\n") {
		if !strings.Contains(baris, "$(SUMBER)") {
			continue
		}
		n++
		if strings.Contains(baris, "N'$(SUMBER)'") { // pencatatan nilai SUMBER pada riwayat
			continue
		}
		if !regexp.MustCompile(`(?i)\bFROM\s+\$\(SUMBER\)\.DJKN\.\w+\s+AS\s+\w+`).MatchString(baris) {
			t.Errorf("$(SUMBER) dipakai bukan di klausa FROM: %q", strings.TrimSpace(baris))
		}
	}
	if n == 0 {
		t.Fatal("$(SUMBER) tidak ditemukan di script")
	}
	for _, larangan := range []string{"DROP DATABASE", "TRUNCATE", "xp_cmdshell", "sp_addlinkedserver", "EXEC AS", "EXECUTE AS", "sp_configure", "GRANT ", "CREATE LOGIN", "CREATE USER"} {
		if strings.Contains(strings.ToUpper(skrip), strings.ToUpper(larangan)) {
			t.Errorf("script memuat %q; datamart tidak boleh mengubah pengaturan server, izin, atau pengguna", larangan)
		}
	}
	// Semua tulisan hanya ke skema DJKN, DJKN_STG, DJKN_OLD, tabel jejak dbo.datamart_sldk_*, tabel sementara (#), dan dua prosedur script ini.
	for _, m := range regexp.MustCompile(`(?i)\b(?:INSERT(?:\s+INTO)?|UPDATE|DELETE\s+FROM|DROP\s+TABLE(?:\s+IF\s+EXISTS)?|CREATE\s+TABLE|INTO)\s+([#\w\[\]\.]+)`).FindAllStringSubmatch(skrip, -1) {
		tujuan := strings.ToLower(m[1])
		ok := strings.HasPrefix(tujuan, "#") || strings.HasPrefix(tujuan, "djkn_stg.") || strings.HasPrefix(tujuan, "djkn_old.") || strings.HasPrefix(tujuan, "dbo.datamart_sldk_")
		if !ok {
			t.Errorf("script menulis ke %q di luar daerah kerja datamart", m[1])
		}
	}
}

// Data pribadi (nama penghuni, SIMAN2_M_ASET_PEMAKAI) hanya untuk rumah negara yang dipakai rumah_negara.sql: kriteria di script harus persis kelompok itu dan mengecualikan mess.
func TestPemakaiHanyaRumahNegara(t *testing.T) {
	skrip := tanpaKomentar(bacaSkrip(t))
	m := regexp.MustCompile(`(?is)INTO DJKN_STG\.SIMAN2_M_ASET_PEMAKAI\s+FROM \$\(SUMBER\)\.DJKN\.SIMAN2_M_ASET_PEMAKAI AS S\s+WHERE\s+(.*?);`).FindStringSubmatch(skrip)
	if m == nil {
		t.Fatal("penyaring SIMAN2_M_ASET_PEMAKAI tidak ditemukan")
	}
	k := rapatkan(m[1])
	for _, perlu := range []string{"I.kd_brg LIKE N'4010201%'", "I.kd_brg LIKE N'4010202%'", "I.kd_brg LIKE N'4010209%'", "I.kd_brg <> N'4010202016'"} {
		if !strings.Contains(k, perlu) {
			t.Errorf("penyaring pemakai tidak memuat %q: %s", perlu, k)
		}
	}
	// kelompok yang sama dengan rumah_negara.sql
	for _, ds := range digitalisasi.Datasets {
		if ds.Key != "rumah_negara" {
			continue
		}
		q, _ := ds.Query()
		q = tanpaKomentar(q)
		for _, perlu := range []string{"A.kd_brg LIKE N'4010201%'", "A.kd_brg LIKE N'4010202%'", "A.kd_brg LIKE N'4010209%'", "A.kd_brg <> N'4010202016'"} {
			if !strings.Contains(q, perlu) {
				t.Errorf("rumah_negara.sql tidak lagi memuat %q: tinjau penyaring pemakai pada script datamart", perlu)
			}
		}
	}
}

func TestHeaderMenyebutSemuaTabelDanPerintah(t *testing.T) {
	mentah := bacaSkrip(t)
	header := mentah[:strings.Index(mentah, ":on error exit")]
	for tb := range tabelSumber(t, mentah) {
		if !strings.Contains(header, tb) {
			t.Errorf("komentar kepala script tidak menjelaskan tabel %s", tb)
		}
	}
	for _, perlu := range []string{"-v SUMBER=", "dbo.datamart_sldk_segarkan", "datamart_sldk_run"} {
		if !strings.Contains(header, perlu) {
			t.Errorf("komentar kepala script tidak menyebut %q", perlu)
		}
	}
}

// Dua prosedur dan pemanggil pertama harus ada, batch dipisah GO, dan galat menghentikan sqlcmd.
func TestStrukturSkrip(t *testing.T) {
	s := tanpaKomentar(bacaSkrip(t))
	for _, perlu := range []string{
		":on error exit", "CREATE OR ALTER PROCEDURE dbo.datamart_sldk_muat", "CREATE OR ALTER PROCEDURE dbo.datamart_sldk_segarkan",
		"sp_getapplock", "ALTER SCHEMA DJKN TRANSFER", "THROW 50020", "THROW 50021", "BEGIN TRY", "EXEC dbo.datamart_sldk_segarkan;",
	} {
		if !strings.Contains(s, perlu) {
			t.Errorf("script tidak memuat %q", perlu)
		}
	}
	// $(SUMBER) tidak boleh punya nilai bawaan tersembunyi: lupa -v harus gagal keras (sqlcmd: scripting variable not defined), bukan membaca database yang salah.
	if regexp.MustCompile(`(?m)^\s*:setvar\s+SUMBER`).MatchString(s) {
		t.Error(":setvar SUMBER aktif di script; harus tetap komentar agar -v berlaku")
	}
	// tabel baru hanya di skema DJKN/DJKN_STG: tiap SELECT ... INTO tabel permanen menuju DJKN_STG
	for _, m := range regexp.MustCompile(`(?i)\bINTO\s+(\S+)`).FindAllStringSubmatch(s, -1) {
		if !strings.HasPrefix(m[1], "#") && !strings.HasPrefix(strings.ToLower(m[1]), "djkn_stg.") {
			t.Errorf("SELECT ... INTO %s: tabel permanen harus dibuat di DJKN_STG lalu ditukar", m[1])
		}
	}
}
