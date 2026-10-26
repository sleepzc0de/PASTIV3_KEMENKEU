package datamart

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

// Pembantu tes integrasi SQL Server: hanya berjalan bila PASTI_UJI_MSSQL_DSN diisi, mis.
//
//	sqlserver://localhost:1433?database=master&authenticator=winsspi&encrypt=disable
//
// Tes membuat database sementara bernama uji_dm_* (SLDK palsu, PASTI, dan datamart) dan menghapusnya lagi; pengguna DSN perlu hak CREATE DATABASE. Database yang ada tidak disentuh.

// tanpaKomentar membuang komentar -- sampai akhir baris.
func tanpaKomentar(s string) string {
	return regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(s, "")
}

func bukaServerUji(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PASTI_UJI_MSSQL_DSN")
	if dsn == "" {
		t.Skip("PASTI_UJI_MSSQL_DSN tidak diisi: tes integrasi SQL Server dilewati")
	}
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		t.Fatal(err)
	}
	var err2 error
	for i := 0; i < 4; i++ { // SQL Server lokal kadang menutup sambungan pertama yang baru dibuka
		if err2 = db.Ping(); err2 == nil {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if err2 != nil {
		t.Fatalf("tidak bisa terhubung ke server uji: %v", err2)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func kutipDB(n string) string { return "[" + strings.ReplaceAll(n, "]", "]]") + "]" }

// buatDBUji membuat database sementara dan menghapusnya saat tes selesai.
func buatDBUji(t *testing.T, srv *sql.DB, nama string) string {
	t.Helper()
	ctx := context.Background()
	full := fmt.Sprintf("uji_dm_%s_%d", nama, time.Now().UnixNano()%1_000_000_000)
	if _, err := srv.ExecContext(ctx, "CREATE DATABASE "+kutipDB(full)); err != nil {
		t.Fatalf("tidak bisa membuat database uji (perlu hak CREATE DATABASE): %v", err)
	}
	t.Cleanup(func() {
		srv.ExecContext(ctx, "ALTER DATABASE "+kutipDB(full)+" SET SINGLE_USER WITH ROLLBACK IMMEDIATE")
		srv.ExecContext(ctx, "DROP DATABASE "+kutipDB(full))
	})
	return full
}

var reGO = regexp.MustCompile(`(?mi)^[ \t]*GO[ \t]*\r?$`)

// jalankanBerkas menjalankan berkas SQL di database nama: variabel sqlcmd $(NAMA) diganti, direktif :on error dibuang, dan batch dipisah di baris GO.
func jalankanBerkas(t *testing.T, srv *sql.DB, nama, teks string, vars map[string]string) error {
	t.Helper()
	for k, v := range vars {
		teks = strings.ReplaceAll(teks, "$("+k+")", v)
	}
	if strings.Contains(tanpaKomentar(teks), "$(") {
		t.Fatal("masih ada variabel sqlcmd yang belum diisi")
	}
	var bersih []string
	for _, l := range strings.Split(teks, "\n") {
		if strings.HasPrefix(strings.TrimSpace(strings.ToLower(l)), ":on error") {
			continue
		}
		bersih = append(bersih, l)
	}
	ctx := context.Background()
	conn, err := srv.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "USE "+kutipDB(nama)); err != nil {
		t.Fatal(err)
	}
	for _, batch := range reGO.Split(strings.Join(bersih, "\n"), -1) {
		if strings.TrimSpace(batch) == "" {
			continue
		}
		if _, err := conn.ExecContext(ctx, batch); err != nil {
			return err
		}
	}
	return nil
}

func ekse(t *testing.T, srv *sql.DB, nama, perintah string) error {
	t.Helper()
	return jalankanBerkas(t, srv, nama, perintah, nil)
}

// baca menjalankan query di database nama dan mengembalikan seluruh baris (dari semua hasil) sebagai teks, terurut.
func baca(t *testing.T, srv *sql.DB, nama, query string) [][]string {
	t.Helper()
	out := bacaUrut(t, srv, nama, query)
	sort.Slice(out, func(i, j int) bool { return strings.Join(out[i], "\x00") < strings.Join(out[j], "\x00") })
	return out
}

// bacaUrut: seperti baca, tetapi urutan baris dibiarkan seperti keluaran query.
func bacaUrut(t *testing.T, srv *sql.DB, nama, query string) [][]string {
	t.Helper()
	ctx := context.Background()
	conn, err := srv.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "USE "+kutipDB(nama)); err != nil {
		t.Fatal(err)
	}
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		t.Fatalf("query gagal di %s: %v", nama, err)
	}
	defer rows.Close()
	var out [][]string
	for set := 0; ; set++ {
		cols, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			raw := make([]interface{}, len(cols))
			ptr := make([]interface{}, len(cols))
			for i := range raw {
				ptr[i] = &raw[i]
			}
			if err := rows.Scan(ptr...); err != nil {
				t.Fatal(err)
			}
			baris := make([]string, len(cols))
			for i, v := range raw {
				switch x := v.(type) {
				case nil:
					baris[i] = "NULL"
				case []byte:
					baris[i] = string(x)
				case time.Time:
					baris[i] = x.UTC().Format(time.RFC3339Nano)
				default:
					baris[i] = fmt.Sprint(x)
				}
			}
			out = append(out, baris)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("hasil query terputus di %s: %v", nama, err)
		}
		if !rows.NextResultSet() {
			break
		}
	}
	return out
}

func angka(t *testing.T, srv *sql.DB, nama, query string) int64 {
	t.Helper()
	r := baca(t, srv, nama, query)
	if len(r) != 1 || len(r[0]) != 1 {
		t.Fatalf("query angka mengembalikan %v", r)
	}
	var n int64
	if _, err := fmt.Sscan(r[0][0], &n); err != nil {
		t.Fatalf("%q bukan angka: %v", r[0][0], err)
	}
	return n
}

func bacaBerkas(t *testing.T, nama string) string {
	t.Helper()
	b, err := os.ReadFile(nama)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// bukaDB membuka koneksi ke database nama di server DSN uji; host dan port boleh diganti (untuk mensimulasikan SLDK yang tidak terjangkau).
func bukaDB(t *testing.T, nama, hostPort string) *sql.DB {
	t.Helper()
	u, err := url.Parse(os.Getenv("PASTI_UJI_MSSQL_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("database", nama)
	u.RawQuery = q.Encode()
	if hostPort != "" {
		u.Host = hostPort
	}
	db, err := sql.Open("sqlserver", u.String())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(4)
	t.Cleanup(func() { db.Close() })
	return db
}
