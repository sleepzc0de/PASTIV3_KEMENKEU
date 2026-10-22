package database

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"pasti-v3-backend/internal/fakesql"
)

func TestBacaIdentitasMembacaEmpatNilai(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return []string{"a", "b", "c", "d"}, [][]driver.Value{{"SRV-DB-PROD\\INST1", "SRV-DB-PROD", "pasti_v3_db", "10.20.30.40"}}, nil
	}
	id, err := BacaIdentitas(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	want := Identitas{Server: `SRV-DB-PROD\INST1`, Mesin: "SRV-DB-PROD", Basis: "pasti_v3_db", Alamat: "10.20.30.40"}
	if id != want {
		t.Errorf("identitas = %+v, want %+v", id, want)
	}
	// Yang ditanyakan jati diri server (bukan nama yang diketik di DB_HOST).
	q := f.Queries()
	if len(q) != 1 || !strings.Contains(q[0].Query, "SERVERPROPERTY('ServerName')") || !strings.Contains(q[0].Query, "DB_NAME()") {
		t.Errorf("kueri identitas tidak memakai SERVERPROPERTY/DB_NAME: %v", q)
	}
}

func TestBacaIdentitasAlamatKosongDiperbolehkan(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return []string{"a", "b", "c", "d"}, [][]driver.Value{{"SRV", "SRV", "db", nil}}, nil
	}
	id, err := BacaIdentitas(context.Background(), db)
	if err != nil || id.Alamat != "" {
		t.Fatalf("alamat NULL harus diterima: %+v %v", id, err)
	}
	if got := id.Ringkas(); got != "server=SRV mesin=SRV database=db alamat=-" {
		t.Errorf("Ringkas = %q", got)
	}
}

func TestBacaIdentitasMenolakNamaServerKosong(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return []string{"a", "b", "c", "d"}, [][]driver.Value{{nil, "M", "db", "1.2.3.4"}}, nil
	}
	if _, err := BacaIdentitas(context.Background(), db); err == nil {
		t.Error("server tanpa nama harus ditolak: tanpa nama tidak ada yang bisa dicocokkan")
	}
}

func TestBacaIdentitasGalatKueri(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return nil, nil, errors.New("izin ditolak")
	}
	if _, err := BacaIdentitas(context.Background(), db); err == nil || !strings.Contains(err.Error(), "izin ditolak") {
		t.Errorf("galat harus diteruskan: %v", err)
	}
}

// Nilai dari server dicetak ke log dan dibaca skrip shell: baris baru atau karakter kendali tidak boleh menyelipkan baris palsu (mis. IDENTITAS_SERVER kedua).
func TestIdentitasDibersihkanDariKarakterKendali(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return []string{"a", "b", "c", "d"}, [][]driver.Value{{"SRV\nIDENTITAS_SERVER=PROD\r\x00", "M\tX", "  db  ", strings.Repeat("9", 400)}}, nil
	}
	id, err := BacaIdentitas(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if id.Server != "SRVIDENTITAS_SERVER=PROD" || id.Mesin != "MX" || id.Basis != "db" || len(id.Alamat) != maksPanjangIdentitas {
		t.Errorf("pembersihan: %+v", id)
	}
	keluar := id.Keluaran()
	if n := strings.Count(keluar, "\n"); n != 4 {
		t.Errorf("keluaran harus tepat 4 baris, ada %d: %q", n, keluar)
	}
	for _, l := range strings.Split(strings.TrimRight(keluar, "\n"), "\n") {
		if !strings.HasPrefix(l, "IDENTITAS_SERVER=") && !strings.HasPrefix(l, "IDENTITAS_MESIN=") && !strings.HasPrefix(l, "IDENTITAS_DATABASE=") && !strings.HasPrefix(l, "IDENTITAS_ALAMAT=") {
			t.Errorf("baris tak dikenal: %q", l)
		}
	}
}

func TestKeluaranIdentitasFormatTetap(t *testing.T) {
	got := Identitas{Server: "S", Mesin: "M", Basis: "D", Alamat: "A"}.Keluaran()
	want := "IDENTITAS_SERVER=S\nIDENTITAS_MESIN=M\nIDENTITAS_DATABASE=D\nIDENTITAS_ALAMAT=A\n"
	if got != want {
		t.Errorf("Keluaran = %q, want %q (deploy.sh bergantung pada format ini)", got, want)
	}
}
