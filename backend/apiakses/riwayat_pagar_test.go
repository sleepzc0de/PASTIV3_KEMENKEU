package apiakses

import (
	"testing"

	"pasti-v3-backend/digitalisasi"
)

// Riwayat bulanan (DIGITALISASI_ASET_ALL) tidak boleh menyimpan lebih banyak daripada yang boleh keluar lewat API: setiap kolom yang ditutup API (data pribadi, foto, nama pengguna)
// juga harus dikecualikan dari riwayat. Dua aturan ini ditulis terpisah (riwayat di paket digitalisasi, API di sini); tes ini menjaga keduanya tidak menyimpang.
func TestRiwayatMengecualikanSemuaKolomYangDitutupAPI(t *testing.T) {
	var ditutup int
	for _, ds := range digitalisasi.Datasets {
		for _, c := range ds.Columns {
			if !kolomTertutup(c) {
				continue
			}
			ditutup++
			if !c.HistoryExcluded() {
				t.Errorf("%s.%s ditutup untuk API tetapi ikut disalin ke riwayat bulanan", ds.Key, c.Name)
			}
		}
	}
	if ditutup == 0 {
		t.Fatal("tidak ada kolom tertutup yang terbaca: tes tidak bermakna")
	}
}
