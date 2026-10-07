package sapa

import (
	"context"
	"strings"
	"unicode"
)

// RefKanwil: satu baris referensi Kanwil (tabel ref_kanwil, migrasi 056) untuk saran tembusan Nota Dinas. Kodenya 9 karakter pertama kode satker.
type RefKanwil struct {
	Kode      string `json:"kode"`
	Nama      string `json:"nama"`
	Singkatan string `json:"singkatan"`
	Aktif     bool   `json:"aktif"`
}

// PilihanKanwil: satu baris referensi Kanwil aktif untuk pemilih tembusan pada formulir Nota Dinas, beserta saran teks tembusannya (TembusanKanwil).
type PilihanKanwil struct {
	Kode      string `json:"kode"`
	Nama      string `json:"nama"`
	Singkatan string `json:"singkatan"`
	Tembusan  string `json:"tembusan"`
}

// RefKanwilAktif: daftar Kanwil yang ditawarkan pemilih tembusan (hanya yang aktif), untuk semua pengguna SAPA.
func (l *Layanan) RefKanwilAktif(ctx context.Context, id Identitas) ([]PilihanKanwil, error) {
	if err := Akses(id); err != nil {
		return nil, err
	}
	daftar, err := l.Repo.DaftarRefKanwilAktif(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PilihanKanwil, 0, len(daftar))
	for _, r := range daftar {
		out = append(out, PilihanKanwil{Kode: r.Kode, Nama: r.Nama, Singkatan: r.Singkatan, Tembusan: TembusanKanwil(r.Nama)})
	}
	return out, nil
}

// kode9Sah: tepat 9 digit angka (kode Kanwil = 9 karakter pertama kode satker).
func kode9Sah(s string) bool {
	if len(s) != 9 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Singkatan yang tetap ditulis huruf besar saat uraian Kanwil dari data satker/SLDK (yang seluruhnya huruf besar) diubah menjadi huruf judul.
// Kata tanpa huruf hidup (mis. "DJP", "KPP") juga dipertahankan. Singkatan yang diisi pada referensi sengaja tidak dipakai sebagai petunjuk: singkatan sering memuat kata biasa
// (mis. "KW DJP JKT PUSAT"), yang akan ikut tertulis huruf besar semua.
var singkatanDikenal = map[string]bool{
	"DJA": true, "DJBC": true, "DJP": true, "DJPB": true, "DJKN": true, "DJPK": true, "DJPPR": true, "DJSEF": true, "DJSPSK": true,
	"BPPK": true, "BTIIK": true, "LNSW": true, "ITJEN": true, "SETJEN": true, "KPPN": true, "KPKNL": true, "KPPBC": true, "UPT": true,
	"DKI": true, "DIY": true, "NTB": true, "NTT": true, "RI": true, "BMN": true, "KL": true,
}

var kataPenghubung = map[string]bool{"DAN": true, "DI": true, "KE": true, "DARI": true, "YANG": true, "DENGAN": true, "ATAU": true, "UNTUK": true}

func tanpaHurufHidup(s string) bool { return !strings.ContainsAny(strings.ToUpper(s), "AIUEO") }

// TembusanKanwil menyusun saran baris tembusan "Kepala <uraian Kanwil>" dari uraian pada referensi Kanwil. Uraian yang sudah campuran huruf besar-kecil dipakai apa adanya
// (ditulis benar oleh superadmin); uraian yang seluruhnya huruf besar (hasil ambil dari data satker atau SLDK) diubah ke huruf judul dengan singkatan dipertahankan,
// mis. "KANTOR WILAYAH DJP JAKARTA PUSAT" -> "Kepala Kantor Wilayah DJP Jakarta Pusat". Hanya saran: pengguna bisa mengubahnya di formulir.
func TembusanKanwil(nama string) string {
	nama = strings.Join(strings.Fields(nama), " ")
	if nama == "" {
		return ""
	}
	if nama == strings.ToUpper(nama) {
		kata := strings.Fields(nama)
		for i, w := range kata {
			kata[i] = judulKata(w, i == 0)
		}
		nama = strings.Join(kata, " ")
	}
	if strings.HasPrefix(strings.ToLower(nama), "kepala ") {
		return nama
	}
	return "Kepala " + nama
}

// judulKata mengubah satu kata berhuruf besar menjadi huruf judul; bagian yang dipisah "-" atau "/" diperlakukan sendiri-sendiri.
func judulKata(w string, awal bool) string {
	if i := strings.IndexAny(w, "-/"); i > 0 && i < len(w)-1 {
		return judulKata(w[:i], awal) + w[i:i+1] + judulKata(w[i+1:], false)
	}
	inti := strings.Trim(w, "()[],.;:\"'")
	if inti == "" {
		return w
	}
	var ganti string
	switch {
	case singkatanDikenal[inti] || (len([]rune(inti)) >= 2 && tanpaHurufHidup(inti) && !strings.ContainsAny(inti, "0123456789")):
		ganti = inti
	case !awal && kataPenghubung[inti]:
		ganti = strings.ToLower(inti)
	default:
		r := []rune(strings.ToLower(inti))
		r[0] = unicode.ToUpper(r[0])
		ganti = string(r)
	}
	return strings.Replace(w, inti, ganti, 1)
}
