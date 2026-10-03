package sldk

import "fmt"

// Rule aturan pemantauan: kondisi data aset yang perlu ditindaklanjuti. Predikatnya dipakai di dua tempat
// supaya hitungan di dashboard dan daftar di pencarian selalu sama: sebagai penghitung di query agregat
// (cmd/sldk-sync) dan sebagai filter pencarian (handlers.buildAssetSearch).
//
// Aturan "rusak berat" tidak ada di sini: kodenya diambil dari tabel referensi kondisi (nama mengandung
// "berat"), bukan ditebak.
type Rule struct {
	Key        string
	Label      string
	Keterangan string
	Predicate  string
	Columns    []string
}

// yes: nilai penanda ya/tidak yang dianggap "ya". Nilai persisnya belum dikonfirmasi (Y/T? 1/0?), jadi
// semua bentuk umum diterima; sejajar dengan isYes() di frontend/components/sldk/asset.ts.
func yes(col string) string {
	return fmt.Sprintf("UPPER(LTRIM(RTRIM(%s))) IN ('Y','YA','YES','1','TRUE')", QuoteIdent(col))
}

var Rules = []Rule{
	{Key: "idle", Label: "BMN idle", Keterangan: "Ditandai idle (status_bmn_idle)", Predicate: yes("status_bmn_idle"), Columns: []string{"status_bmn_idle"}},
	{Key: "hilang", Label: "Barang hilang", Keterangan: "Ditandai hilang (brg_hilang_yn)", Predicate: yes("brg_hilang_yn"), Columns: []string{"brg_hilang_yn"}},
	{Key: "dihentikan", Label: "Dihentikan", Keterangan: "Ditandai dihentikan (dihentikan_yn)", Predicate: yes("dihentikan_yn"), Columns: []string{"dihentikan_yn"}},
	{Key: "nilai_nol", Label: "Nilai buku nol", Keterangan: "Nilai buku 0 padahal nilai perolehan lebih dari 0", Predicate: "[rph_buku] = 0 AND [rph_aset] > 0", Columns: []string{"rph_buku", "rph_aset"}},
	{Key: "dq_tanggal", Label: "Tanggal tidak valid", Keterangan: "Ada tanggal tidak valid menurut pemeriksaan pipeline (dq_tgl_invalid_cnt > 0)", Predicate: "[dq_tgl_invalid_cnt] > 0", Columns: []string{"dq_tgl_invalid_cnt"}},
}

func RuleByKey(key string) (Rule, bool) {
	for _, r := range Rules {
		if r.Key == key {
			return r, true
		}
	}
	return Rule{}, false
}

// CountColumn: nama kolom penghitung aturan di tabel sldk_agregat (aman disisipkan ke SQL: berasal dari daftar tetap).
func (r Rule) CountColumn() string { return "n_" + r.Key }
