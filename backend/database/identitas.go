package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode"
)

// Identitas adalah jati diri server database menurut SQL Server sendiri. Alamat di DB_HOST tidak cukup untuk memastikan server mana yang dituju: infrastruktur dapat
// memetakan (IP masking/NAT) sebuah alamat ke server lain, misalnya alamat "production" yang ternyata sampai ke SQL Server development. Nama yang dilaporkan server tidak
// ikut berubah oleh pemetaan itu, jadi deploy.sh memakainya untuk membuktikan bahwa database yang akan dimigrasi memang yang dimaksud.
type Identitas struct {
	Server string // SERVERPROPERTY('ServerName'), mis. "SRV-DB-PROD" atau "SRV-DB-PROD\INST1"
	Mesin  string // SERVERPROPERTY('MachineName')
	Basis  string // DB_NAME(): database yang dipakai sambungan ini
	Alamat string // CONNECTIONPROPERTY('local_net_address'): alamat server database menurut server itu sendiri (kosong bila sambungan bukan TCP)
}

const kueriIdentitas = `SELECT
	CAST(SERVERPROPERTY('ServerName') AS NVARCHAR(256)),
	CAST(SERVERPROPERTY('MachineName') AS NVARCHAR(256)),
	CAST(DB_NAME() AS NVARCHAR(256)),
	CAST(CONNECTIONPROPERTY('local_net_address') AS NVARCHAR(128))`

const maksPanjangIdentitas = 256

// BacaIdentitas menanyakan jati diri server yang sedang disambung. Nilai dibersihkan dari karakter kendali dan dipangkas, karena dicetak ke log dan dibaca skrip shell.
func BacaIdentitas(ctx context.Context, db *sql.DB) (Identitas, error) {
	var server, mesin, basis, alamat sql.NullString
	if err := db.QueryRowContext(ctx, kueriIdentitas).Scan(&server, &mesin, &basis, &alamat); err != nil {
		return Identitas{}, fmt.Errorf("gagal membaca identitas server database: %w", err)
	}
	id := Identitas{
		Server: bersihkanIdentitas(server.String),
		Mesin:  bersihkanIdentitas(mesin.String),
		Basis:  bersihkanIdentitas(basis.String),
		Alamat: bersihkanIdentitas(alamat.String),
	}
	if id.Server == "" {
		return Identitas{}, fmt.Errorf("server database tidak melaporkan namanya")
	}
	return id, nil
}

func bersihkanIdentitas(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > maksPanjangIdentitas {
		s = string(r[:maksPanjangIdentitas])
	}
	return s
}

func atauStrip(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// Ringkas: satu baris untuk log (docker logs backend).
func (i Identitas) Ringkas() string {
	return fmt.Sprintf("server=%s mesin=%s database=%s alamat=%s", i.Server, atauStrip(i.Mesin), atauStrip(i.Basis), atauStrip(i.Alamat))
}

// Keluaran: baris KUNCI=nilai yang dibaca deploy.sh dari keluaran "pasti-migrate -identitas". Satu nilai satu baris, tanpa kutip.
func (i Identitas) Keluaran() string {
	return fmt.Sprintf("IDENTITAS_SERVER=%s\nIDENTITAS_MESIN=%s\nIDENTITAS_DATABASE=%s\nIDENTITAS_ALAMAT=%s\n", i.Server, i.Mesin, i.Basis, i.Alamat)
}
