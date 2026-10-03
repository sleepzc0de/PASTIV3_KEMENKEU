package digitalisasi

import (
	"strings"
	"testing"
	"time"
)

func TestConvertText(t *testing.T) {
	c := Column{Name: "X", Kind: Text, Size: 5}
	var st convStats
	cases := []struct {
		in   interface{}
		want interface{}
	}{
		{nil, nil},
		{"", nil},
		{"   ", nil},
		{"  abc  ", "abc"},
		{[]byte("xyz"), "xyz"},
		{true, "1"},
		{false, "0"},
		{int64(42), "42"},
		{float64(1.5), "1.5"},
		{time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), "2026-"}, // dipotong ke 5 karakter
	}
	for _, tc := range cases {
		got := convertValue(c, tc.in, &st)
		if got != tc.want {
			t.Errorf("convertValue(%#v) = %#v, want %#v", tc.in, got, tc.want)
		}
	}
	if st.Truncated != 1 {
		t.Errorf("Truncated = %d, want 1 (hanya tanggal yang melebihi 5 karakter)", st.Truncated)
	}
}

func TestConvertTextTruncatesByUTF16(t *testing.T) {
	// "😀" memakai 2 unit UTF-16, jadi Size 3 hanya muat satu emoji ditambah satu huruf.
	c := Column{Name: "X", Kind: Text, Size: 3}
	var st convStats
	got := convertValue(c, "😀😀a", &st).(string)
	if got != "😀" {
		t.Fatalf("got %q, want satu emoji (emoji kedua tidak muat)", got)
	}
	if utf16Len(got) > 3 {
		t.Fatalf("panjang UTF-16 %d melebihi 3", utf16Len(got))
	}
	if st.Truncated != 1 {
		t.Fatalf("Truncated = %d", st.Truncated)
	}
}

func TestConvertTextMaxHasNoLimit(t *testing.T) {
	c := Column{Name: "X", Kind: Text}
	var st convStats
	long := strings.Repeat("a", 10000)
	if got := convertValue(c, long, &st); got != long || st.Truncated != 0 {
		t.Fatal("NVARCHAR(MAX) tidak boleh dipotong")
	}
}

func TestConvertDecimalAndInt(t *testing.T) {
	var st convStats
	dec := Column{Name: "D", Kind: Decimal, Scale: 2}
	if got := convertValue(dec, []byte("1234.5000"), &st); got != "1234.5" {
		t.Errorf("decimal []byte = %#v", got)
	}
	if got := convertValue(dec, "12,5", &st); got != "12.5" {
		t.Errorf("koma desimal = %#v", got)
	}
	if got := convertValue(dec, float64(0), &st); got != "0" {
		t.Errorf("nol harus tetap 0, bukan NULL: %#v", got)
	}
	if got := convertValue(dec, "abc", &st); got != nil || st.BadNumber != 1 {
		t.Errorf("angka rusak: %#v, BadNumber=%d", got, st.BadNumber)
	}

	i := Column{Name: "I", Kind: Int}
	if got := convertValue(i, int64(7), &st); got != int64(7) {
		t.Errorf("int64 = %#v", got)
	}
	if got := convertValue(i, float64(5), &st); got != int64(5) {
		t.Errorf("float bulat = %#v", got)
	}
	if got := convertValue(i, "3", &st); got != int64(3) {
		t.Errorf("string angka = %#v", got)
	}
	if got := convertValue(i, 2.5, &st); got != nil {
		t.Errorf("bukan bilangan bulat harus NULL: %#v", got)
	}
	if got := convertValue(i, nil, &st); got != nil {
		t.Errorf("NULL harus tetap NULL: %#v", got)
	}
}

func TestConvertCoordinates(t *testing.T) {
	lat := Column{Name: "Latitude", Kind: Coord}
	lng := Column{Name: "Longitude", Kind: Coord}
	var st convStats

	if got := convertValue(lat, "-6,2", &st); got != -6.2 {
		t.Errorf("lat koma = %#v", got)
	}
	if got := convertValue(lng, []byte("106.8456"), &st); got != 106.8456 {
		t.Errorf("lng []byte = %#v", got)
	}
	if got := convertValue(lat, 91.0, &st); got != nil {
		t.Errorf("lintang > 90 harus dibuang: %#v", got)
	}
	if got := convertValue(lng, 181.0, &st); got != nil {
		t.Errorf("bujur > 180 harus dibuang: %#v", got)
	}
	if got := convertValue(lng, 150.0, &st); got != 150.0 {
		t.Errorf("bujur 150 sah (lintang tidak): %#v", got)
	}
	if got := convertValue(lat, "bukan angka", &st); got != nil {
		t.Errorf("teks harus dibuang: %#v", got)
	}
	if st.BadCoord != 3 {
		t.Errorf("BadCoord = %d, want 3", st.BadCoord)
	}
}

func TestFixCoordinates(t *testing.T) {
	var st convStats
	row := []interface{}{-6.2, 106.8}
	fixCoordinates(row, 0, 1, &st)
	if row[0] == nil || row[1] == nil || st.BadCoord != 0 {
		t.Fatal("pasangan sah tidak boleh diubah")
	}

	row = []interface{}{-6.2, nil}
	fixCoordinates(row, 0, 1, &st)
	if row[0] != nil || row[1] != nil {
		t.Fatal("koordinat tidak berpasangan harus dibuang keduanya")
	}

	row = []interface{}{0.0, 0.0}
	fixCoordinates(row, 0, 1, &st)
	if row[0] != nil || row[1] != nil {
		t.Fatal("(0, 0) adalah isian kosong, harus dibuang")
	}

	row = []interface{}{0.0, 106.8} // di khatulistiwa, sah
	fixCoordinates(row, 0, 1, &st)
	if row[0] == nil {
		t.Fatal("lintang 0 dengan bujur valid itu sah")
	}

	row = []interface{}{nil, nil}
	before := st.BadCoord
	fixCoordinates(row, 0, 1, &st)
	if st.BadCoord != before {
		t.Fatal("tanpa koordinat bukan data rusak")
	}

	fixCoordinates([]interface{}{"x"}, -1, -1, &st) // dataset tanpa koordinat: tidak panik
}
