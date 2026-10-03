package digitalisasi

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// convStats mengumpulkan hal yang diperbaiki saat konversi, untuk dicatat di riwayat sinkronisasi.
type convStats struct {
	Truncated int // nilai teks yang dipotong karena melebihi panjang kolom
	BadNumber int // angka yang tidak terbaca (dibuang jadi NULL)
	BadCoord  int // koordinat yang ada isinya tetapi tidak valid (dibuang jadi NULL)
}

// asString: bentuk teks dari nilai apa pun yang dikembalikan driver SQL Server. decimal/money datang sebagai
// []byte (bukan float), bit sebagai bool.
func asString(v interface{}) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", false
	case string:
		return x, true
	case []byte:
		return string(x), true
	case bool:
		if x {
			return "1", true
		}
		return "0", true
	case int64:
		return strconv.FormatInt(x, 10), true
	case int32:
		return strconv.FormatInt(int64(x), 10), true
	case int:
		return strconv.Itoa(x), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32), true
	case time.Time:
		return x.Format(time.RFC3339), true
	}
	return fmt.Sprint(v), true
}

// utf16Len: panjang menurut SQL Server (NVARCHAR(n) menghitung unit UTF-16).
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func truncateUTF16(s string, max int) string {
	if utf16Len(s) <= max {
		return s
	}
	n := 0
	for i, r := range s {
		w := 1
		if r > 0xFFFF {
			w = 2
		}
		if n+w > max {
			return s[:i]
		}
		n += w
	}
	return s
}

// parseNumber membaca angka dari teks dengan toleransi koma desimal ("-6,2" -> -6.2).
func parseNumber(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if strings.Contains(s, ",") && !strings.Contains(s, ".") {
		s = strings.Replace(s, ",", ".", 1)
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// convertValue mengubah nilai hasil query menjadi nilai yang aman dikirim ke kolom tujuan. NULL tetap NULL;
// teks kosong juga menjadi NULL supaya "tidak ada foto" dan sejenisnya mudah dihitung.
func convertValue(c Column, v interface{}, st *convStats) interface{} {
	s, ok := asString(v)
	if !ok {
		return nil
	}
	switch c.Kind {
	case Int, BigInt:
		f, ok := parseNumber(s)
		if !ok || f != math.Trunc(f) {
			if strings.TrimSpace(s) != "" {
				st.BadNumber++
			}
			return nil
		}
		return int64(f)
	case Decimal:
		f, ok := parseNumber(s)
		if !ok {
			if strings.TrimSpace(s) != "" {
				st.BadNumber++
			}
			return nil
		}
		// Dikirim sebagai teks: SQL Server mengubahnya ke DECIMAL tanpa melalui float.
		return strconv.FormatFloat(f, 'f', -1, 64)
	case Coord:
		f, ok := parseNumber(s)
		limit := 180.0
		if strings.EqualFold(c.Name, "Latitude") {
			limit = 90
		}
		if !ok || math.Abs(f) > limit {
			if strings.TrimSpace(s) != "" {
				st.BadCoord++
			}
			return nil
		}
		return f
	default:
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		if c.Size > 0 && utf16Len(s) > c.Size {
			st.Truncated++
			s = truncateUTF16(s, c.Size)
		}
		if !utf8.ValidString(s) {
			s = strings.ToValidUTF8(s, "")
		}
		return s
	}
}

// fixCoordinates: lintang dan bujur harus berpasangan, dan (0, 0) adalah isian kosong, bukan lokasi.
func fixCoordinates(row []interface{}, lat, lng int, st *convStats) {
	if lat < 0 || lng < 0 {
		return
	}
	a, b := row[lat], row[lng]
	if a == nil && b == nil {
		return
	}
	if a == nil || b == nil {
		row[lat], row[lng] = nil, nil
		st.BadCoord++
		return
	}
	if a.(float64) == 0 && b.(float64) == 0 {
		row[lat], row[lng] = nil, nil
		st.BadCoord++
	}
}
