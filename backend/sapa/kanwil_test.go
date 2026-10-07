package sapa

import "testing"

func TestTembusanKanwil(t *testing.T) {
	for _, c := range []struct{ nama, want string }{
		// uraian dari data satker/SLDK (huruf besar semua) -> huruf judul, singkatan dipertahankan
		{"KANTOR WILAYAH DJP JAKARTA PUSAT", "Kepala Kantor Wilayah DJP Jakarta Pusat"},
		{"KANWIL DJBC JAWA TIMUR I", "Kepala Kanwil DJBC Jawa Timur I"},
		{"KANTOR WILAYAH DJKN DKI JAKARTA", "Kepala Kantor Wilayah DJKN DKI Jakarta"},
		{"KANTOR WILAYAH DJPB PROVINSI SULAWESI DAN MALUKU", "Kepala Kantor Wilayah DJPB Provinsi Sulawesi dan Maluku"},
		{"KANTOR PUSAT DJA", "Kepala Kantor Pusat DJA"},
		{"KANTOR WILAYAH KALIMANTAN-TIMUR", "Kepala Kantor Wilayah Kalimantan-Timur"},
		{"KANWIL (DJP) BALI", "Kepala Kanwil (DJP) Bali"},
		{"KANTOR WILAYAH ABCDE BARAT", "Kepala Kantor Wilayah Abcde Barat"}, // kata biasa menjadi huruf judul
		{"  KANTOR   WILAYAH  DJP  ", "Kepala Kantor Wilayah DJP"},          // spasi dirapikan
		// uraian campuran huruf (ditulis superadmin) dipakai apa adanya
		{"Kantor Wilayah DJP Jakarta Pusat", "Kepala Kantor Wilayah DJP Jakarta Pusat"},
		{"Kantor Wilayah DJKN Jawa Barat", "Kepala Kantor Wilayah DJKN Jawa Barat"},
		// sudah berawalan "Kepala" tidak digandakan
		{"Kepala Kantor Wilayah DJKN Jakarta", "Kepala Kantor Wilayah DJKN Jakarta"},
		{"KEPALA KANTOR WILAYAH DJP BALI", "Kepala Kantor Wilayah DJP Bali"},
		{"", ""},
		{"   ", ""},
	} {
		if got := TembusanKanwil(c.nama); got != c.want {
			t.Errorf("TembusanKanwil(%q) = %q, want %q", c.nama, got, c.want)
		}
	}
}
