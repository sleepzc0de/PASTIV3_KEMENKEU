package handlers

import (
	"database/sql"
	"strconv"
	"strings"
)

// Bantuan SQL kecil yang dipakai bersama oleh handler Digitalisasi Aset dan Inaproc.

// escapeLike menonaktifkan karakter khusus LIKE pada masukan pengguna (dipakai bersama ESCAPE '\').
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`, `[`, `\[`).Replace(s)
}

// scanMoney membaca kolom angka yang dibaca sebagai teks (NUMERIC/DECIMAL dari driver); NULL dan teks yang bukan angka menjadi 0.
func scanMoney(ns sql.NullString) float64 {
	if !ns.Valid {
		return 0
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(ns.String), 64)
	if err != nil {
		return 0
	}
	return f
}

// optStr mengubah NULL menjadi nil agar tampil sebagai null di JSON.
func optStr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

// rowsToMaps memuat seluruh baris menjadi peta kolom -> nilai (bytes dijadikan string).
func rowsToMaps(rows *sql.Rows) ([]map[string]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, col := range cols {
			val := values[i]
			switch v := val.(type) {
			case []byte:
				rowMap[col] = string(v)
			default:
				rowMap[col] = v
			}
		}
		results = append(results, rowMap)
	}

	return results, nil
}
