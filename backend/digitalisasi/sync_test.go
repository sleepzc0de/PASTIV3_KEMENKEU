package digitalisasi

import (
	"context"
	"database/sql/driver"
	"errors"
	"pasti-v3-backend/internal/fakesql"
	"strings"
	"testing"
)

// sourceRows membentuk hasil query SLDK palsu untuk satu dataset: satu baris per elemen rows, kolom menurut
// nama sumbernya. override memberi nilai khusus per nama kolom sumber; sisanya diisi nilai wajar.
func sourceRows(ds Dataset, n int, override func(i int, source string) (driver.Value, bool)) ([]string, [][]driver.Value) {
	cols := make([]string, len(ds.Columns))
	for i, c := range ds.Columns {
		cols[i] = c.SourceName()
	}
	rows := make([][]driver.Value, n)
	for r := 0; r < n; r++ {
		row := make([]driver.Value, len(ds.Columns))
		for i, c := range ds.Columns {
			if v, ok := override(r, c.SourceName()); ok {
				row[i] = v
				continue
			}
			switch c.Kind {
			case Int, BigInt:
				row[i] = int64(r + 1)
			case Decimal:
				row[i] = []byte("1500.2500") // decimal datang sebagai []byte dari driver SQL Server
			case Coord:
				if strings.EqualFold(c.Name, "Latitude") {
					row[i] = -6.2
				} else {
					row[i] = 106.8
				}
			default:
				row[i] = "teks"
			}
		}
		rows[r] = row
	}
	return cols, rows
}

// sourceResult: sama dengan sourceRows tetapi dengan bentuk kembalian hook onQuery.
func sourceResult(ds Dataset, n int, override func(i int, source string) (driver.Value, bool)) ([]string, [][]driver.Value, error) {
	cols, rows := sourceRows(ds, n, override)
	return cols, rows, nil
}

func noOverride(int, string) (driver.Value, bool) { return nil, false }

func TestSyncWritesRowsInOneTransaction(t *testing.T) {
	ds, _ := ByKey("tanah")
	sldkDB, sldk := fakesql.New(t)
	pasti, p := fakesql.New(t)

	sldk.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if !strings.Contains(q, "#AsetTarget") {
			t.Errorf("query tanah.sql tidak terkirim apa adanya")
		}
		cols, rows := sourceRows(ds, 200, func(i int, s string) (driver.Value, bool) {
			if s == "GPS_Latitude" && i == 0 {
				return "bukan angka", true // koordinat rusak pada baris pertama
			}
			return nil, false
		})
		return cols, rows, nil
	}
	p.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		t.Errorf("tidak ada query baca ke PASTI yang diharapkan pada dataset tanpa Keep: %s", q)
		return nil, nil, errors.New("tak terduga")
	}

	var progress []string
	res, err := Sync(context.Background(), sldkDB, pasti, ds, Options{RunID: 77, Progress: func(m string) { progress = append(progress, m) }})
	if err != nil {
		t.Fatal(err)
	}
	if res.Rows != 200 {
		t.Fatalf("Rows = %d", res.Rows)
	}
	if res.WithCoords != 199 || res.Stats.BadCoord < 1 {
		t.Fatalf("WithCoords = %d, BadCoord = %d; baris pertama harus kehilangan koordinatnya", res.WithCoords, res.Stats.BadCoord)
	}

	ev := p.Events()
	if ev[0] != "BEGIN" || !strings.HasPrefix(ev[1], "EXEC DELETE FROM [DIGITALISASI_TANAH]") || ev[len(ev)-1] != "COMMIT" {
		t.Fatalf("urutan kejadian salah: %v", ev)
	}
	inserts := p.Execs()[1:]
	width := len(ds.Columns) + 1
	total := 0
	for _, e := range inserts {
		if !strings.HasPrefix(e.Query, "INSERT INTO [DIGITALISASI_TANAH] (") {
			t.Fatalf("bukan INSERT: %s", e.Query)
		}
		if len(e.Args) > 2000 {
			t.Fatalf("%d parameter melebihi batas aman", len(e.Args))
		}
		if len(e.Args)%width != 0 {
			t.Fatalf("jumlah parameter %d bukan kelipatan %d kolom", len(e.Args), width)
		}
		total += len(e.Args) / width
		// kolom terakhir tiap baris = id_sinkron
		if got := e.Args[width-1].Value; got != int64(77) {
			t.Fatalf("id_sinkron = %#v", got)
		}
	}
	if total != 200 {
		t.Fatalf("baris tertulis %d, want 200", total)
	}
	if len(inserts) < 2 {
		t.Fatalf("200 baris x %d kolom harus dipecah jadi beberapa INSERT, dapat %d", width, len(inserts))
	}
	if len(progress) == 0 {
		t.Fatal("Progress tidak pernah dipanggil")
	}

	// Decimal dikirim sebagai string (bukan []byte, yang akan dibaca SQL Server sebagai varbinary).
	first := inserts[0].Args
	for i, c := range ds.Columns {
		if c.Kind == Decimal {
			if s, ok := first[i].Value.(string); !ok || s != "1500.25" {
				t.Fatalf("kolom %s = %#v, want string \"1500.25\"", c.Name, first[i].Value)
			}
		}
	}
}

func TestSyncMissingColumnFailsBeforeTouchingTable(t *testing.T) {
	ds, _ := ByKey("rusunara")
	sldkDB, sldk := fakesql.New(t)
	pasti, p := fakesql.New(t)
	sldk.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		cols, rows := sourceRows(ds, 3, noOverride)
		// Query yang disunting tanpa kolom Kamar_tipe_C dan koordinat.
		var keep []int
		for i, c := range cols {
			if c != "Kamar_tipe_C" && c != "GPS_Latitude" {
				keep = append(keep, i)
			}
		}
		pick := func(s []string) []string {
			var o []string
			for _, i := range keep {
				o = append(o, s[i])
			}
			return o
		}
		var outRows [][]driver.Value
		for _, r := range rows {
			var o []driver.Value
			for _, i := range keep {
				o = append(o, r[i])
			}
			outRows = append(outRows, o)
		}
		return pick(cols), outRows, nil
	}
	_, err := Sync(context.Background(), sldkDB, pasti, ds, Options{})
	if err == nil {
		t.Fatal("seharusnya gagal")
	}
	for _, want := range []string{"Kamar_tipe_C", "GPS_Latitude", "rusunara.sql"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("pesan galat %q tidak menyebut %q", err, want)
		}
	}
	if len(p.Events()) != 0 {
		t.Fatalf("PASTI tidak boleh disentuh: %v", p.Events())
	}
}

func TestSyncEmptyResultNeverOverwritesExistingData(t *testing.T) {
	ds, _ := ByKey("tanah")
	sldkDB, sldk := fakesql.New(t)
	pasti, p := fakesql.New(t)
	sldk.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		cols, _ := sourceRows(ds, 0, noOverride)
		return cols, nil, nil
	}
	existing := int64(10)
	p.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if !strings.HasPrefix(q, "SELECT COUNT(*) FROM [DIGITALISASI_TANAH]") {
			t.Errorf("query tak terduga: %s", q)
		}
		return []string{"n"}, [][]driver.Value{{existing}}, nil
	}

	_, err := Sync(context.Background(), sldkDB, pasti, ds, Options{})
	if err == nil || !strings.Contains(err.Error(), "TIDAK diganti") {
		t.Fatalf("err = %v", err)
	}
	if p.Count("BEGIN") != 0 || p.Count("EXEC") != 0 {
		t.Fatalf("tabel tidak boleh disentuh: %v", p.Events())
	}

	// Tabel masih kosong: hasil kosong sah (mis. belum ada mess), tidak ada yang perlu dilindungi.
	existing = 0
	res, err := Sync(context.Background(), sldkDB, pasti, ds, Options{})
	if err != nil || res.Rows != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestSyncKeepsManualColumnsWhenSourceIsNull(t *testing.T) {
	ds, _ := ByKey("satker")
	sldkDB, sldk := fakesql.New(t)
	pasti, p := fakesql.New(t)

	sldk.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return sourceResult(ds, 3, func(i int, s string) (driver.Value, bool) {
			switch s {
			case "Kode_Satker":
				return []string{"S1", "S2", "S3"}[i], true
			case "Status_Gedung_Kantor", "Foto":
				if i == 2 {
					return "dari-sldk", true // sumber mengisi: tidak boleh ditimpa nilai lama
				}
				return nil, true // query selalu mengirim NULL
			}
			return nil, false
		})
	}
	p.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if !strings.HasPrefix(q, "SELECT [Kode_Satker], [Status_Gedung_Kantor], [Foto] FROM [DIGITALISASI_SATKER]") {
			t.Errorf("query tak terduga: %s", q)
		}
		return []string{"k", "s", "f"}, [][]driver.Value{
			{"S1", "Milik sendiri", "foto-s1.jpg"},
			{"S3", "Sewa", "foto-s3.jpg"},
			{"HILANG", "Sudah tidak ada", nil},
		}, nil
	}

	if _, err := Sync(context.Background(), sldkDB, pasti, ds, Options{RunID: 1}); err != nil {
		t.Fatal(err)
	}
	ev := p.Events()
	if ev[0] != "BEGIN" || !strings.HasPrefix(ev[1], "QUERY SELECT [Kode_Satker]") || !strings.HasPrefix(ev[2], "EXEC DELETE") {
		t.Fatalf("nilai manual harus dibaca SEBELUM tabel dikosongkan: %v", ev)
	}

	args := p.Execs()[1].Args
	width := len(ds.Columns) + 1
	statusIdx, fotoIdx, kodeIdx := columnIndex(ds, "Status_Gedung_Kantor"), columnIndex(ds, "Foto"), columnIndex(ds, "Kode_Satker")
	byKode := map[string][2]interface{}{}
	for r := 0; r < len(args)/width; r++ {
		row := args[r*width : (r+1)*width]
		byKode[row[kodeIdx].Value.(string)] = [2]interface{}{row[statusIdx].Value, row[fotoIdx].Value}
	}
	if got := byKode["S1"]; got[0] != "Milik sendiri" || got[1] != "foto-s1.jpg" {
		t.Errorf("S1 harus mempertahankan nilai manual: %v", got)
	}
	if got := byKode["S2"]; got[0] != nil || got[1] != nil {
		t.Errorf("S2 baru, tidak punya nilai manual: %v", got)
	}
	if got := byKode["S3"]; got[0] != "dari-sldk" || got[1] != "dari-sldk" {
		t.Errorf("nilai dari sumber tidak boleh ditimpa nilai lama: %v", got)
	}
	if _, ok := byKode["HILANG"]; ok {
		t.Error("satker yang tidak ada di sumber tidak boleh muncul kembali")
	}
}

func TestSyncRollsBackWhenInsertFails(t *testing.T) {
	ds, _ := ByKey("mess_rumah_negara")
	sldkDB, sldk := fakesql.New(t)
	pasti, p := fakesql.New(t)
	sldk.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return sourceResult(ds, 5, noOverride)
	}
	p.OnExec = func(q string, args []driver.NamedValue) error {
		if strings.HasPrefix(q, "INSERT") {
			return errors.New("String or binary data would be truncated")
		}
		return nil
	}
	_, err := Sync(context.Background(), sldkDB, pasti, ds, Options{})
	if err == nil || !strings.Contains(err.Error(), "DIGITALISASI_MESS_RUMAH_NEGARA") {
		t.Fatalf("err = %v", err)
	}
	if p.Count("ROLLBACK") != 1 || p.Count("COMMIT") != 0 {
		t.Fatalf("harus rollback, tidak commit: %v", p.Events())
	}
}

func TestSyncRowLimit(t *testing.T) {
	ds, _ := ByKey("satker")
	sldkDB, sldk := fakesql.New(t)
	pasti, p := fakesql.New(t)
	sldk.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return sourceResult(ds, 10, noOverride)
	}
	_, err := Sync(context.Background(), sldkDB, pasti, ds, Options{MaxRows: 5})
	if err == nil || !strings.Contains(err.Error(), "melebihi 5 baris") {
		t.Fatalf("err = %v", err)
	}
	if len(p.Events()) != 0 {
		t.Fatal("PASTI tidak boleh disentuh")
	}
}

func TestSyncReportsSLDKQueryError(t *testing.T) {
	ds, _ := ByKey("tanah")
	sldkDB, sldk := fakesql.New(t)
	pasti, p := fakesql.New(t)
	sldk.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return nil, nil, errors.New("mssql: Invalid column name 'gps_latitude'.")
	}
	_, err := Sync(context.Background(), sldkDB, pasti, ds, Options{})
	if err == nil || !strings.Contains(err.Error(), "Invalid column name 'gps_latitude'") {
		t.Fatalf("err = %v", err)
	}
	if len(p.Events()) != 0 {
		t.Fatal("PASTI tidak boleh disentuh")
	}
}

func TestSyncStopsWhenContextCancelled(t *testing.T) {
	ds, _ := ByKey("tanah")
	sldkDB, sldk := fakesql.New(t)
	pasti, p := fakesql.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	sldk.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		cancel() // pengguna menekan Batalkan saat query SLDK berjalan
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}
	_, err := Sync(ctx, sldkDB, pasti, ds, Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(p.Events()) != 0 {
		t.Fatal("PASTI tidak boleh disentuh")
	}
}
