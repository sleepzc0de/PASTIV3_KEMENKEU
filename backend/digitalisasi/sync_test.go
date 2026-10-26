package digitalisasi

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"pasti-v3-backend/internal/fakesql"
)

// Tes Sync: menyalin tabel DIGITALISASI_* dari datamart ke PASTI. Tanpa SQL Server (fakesql); bukti terhadap SQL Server sungguhan ada di backend/datamart.

const idPenyegaran = 41 // nomor proses penyegaran datamart palsu (id_sinkron di tabel datamart)

var dataPerUji = time.Date(2026, 10, 25, 17, 0, 0, 0, time.UTC)

// datamartBaris membentuk hasil SELECT tabel DIGITALISASI_* datamart palsu: kolom menurut ds.Columns lalu id_sinkron. override memberi nilai khusus per nama kolom.
func datamartBaris(ds Dataset, n int, override func(i int, nama string) (driver.Value, bool)) ([]string, [][]driver.Value) {
	cols := make([]string, 0, len(ds.Columns)+1)
	for _, c := range ds.Columns {
		cols = append(cols, c.Name)
	}
	cols = append(cols, "id_sinkron")
	rows := make([][]driver.Value, n)
	for r := 0; r < n; r++ {
		row := make([]driver.Value, 0, len(cols))
		for _, c := range ds.Columns {
			if v, ok := override(r, c.Name); ok {
				row = append(row, v)
				continue
			}
			switch c.Kind {
			case Int, BigInt:
				row = append(row, int64(r+1))
			case Decimal:
				row = append(row, []byte("1500.2500")) // DECIMAL datang sebagai []byte dari driver SQL Server
			case Coord:
				if strings.EqualFold(c.Name, "Latitude") {
					row = append(row, []byte("-6.2000000"))
				} else {
					row = append(row, []byte("106.8000000"))
				}
			default:
				row = append(row, "teks")
			}
		}
		rows[r] = append(row, int64(idPenyegaran))
	}
	return cols, rows
}

func tanpaOverride(int, string) (driver.Value, bool) { return nil, false }

// pasangDatamart memasang jawaban datamart palsu untuk satu dataset: isi tabel (n baris), jejak penyegaran #idPenyegaran, dan penyegaran sukses terakhir (segar).
func pasangDatamart(f *fakesql.DB, ds Dataset, n int, override func(i int, nama string) (driver.Value, bool), segar time.Time) {
	f.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		switch {
		case strings.HasPrefix(q, "SELECT ["):
			if !strings.Contains(q, "FROM dbo.["+ds.Table+"] ORDER BY [id]") {
				return nil, nil, errors.New("tabel tak terduga: " + q)
			}
			cols, rows := datamartBaris(ds, n, override)
			return cols, rows, nil
		case strings.Contains(q, "WHERE id = @p1"):
			if got := args[0].Value; got != int64(idPenyegaran) {
				return nil, nil, errors.New("nomor proses tak terduga")
			}
			return []string{"mulai"}, [][]driver.Value{{segar}}, nil
		case strings.Contains(q, "MAX(mulai)"):
			return []string{"mulai"}, [][]driver.Value{{segar}}, nil
		}
		return nil, nil, errors.New("query tak terduga ke datamart: " + q)
	}
}

func TestSyncWritesRowsInOneTransaction(t *testing.T) {
	ds, _ := ByKey("tanah")
	martDB, mart := fakesql.New(t)
	pasti, p := fakesql.New(t)
	pasangDatamart(mart, ds, 200, func(i int, nama string) (driver.Value, bool) {
		if nama == "Latitude" && i == 0 {
			return nil, true // baris pertama tanpa koordinat: pasangannya ikut kosong di datamart
		}
		if nama == "Longitude" && i == 0 {
			return nil, true
		}
		return nil, false
	}, dataPerUji)
	p.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		t.Errorf("tidak ada query baca ke PASTI yang diharapkan pada dataset tanpa Keep: %s", q)
		return nil, nil, errors.New("tak terduga")
	}

	var progress []string
	res, err := Sync(context.Background(), martDB, pasti, ds, Options{RunID: 77, Progress: func(m string) { progress = append(progress, m) }})
	if err != nil {
		t.Fatal(err)
	}
	if res.Rows != 200 || res.WithCoords != 199 {
		t.Fatalf("Rows = %d, WithCoords = %d", res.Rows, res.WithCoords)
	}
	if !res.DataPer.Equal(dataPerUji) || res.SamaDenganSebelumnya {
		t.Fatalf("DataPer = %v, Sama = %v", res.DataPer, res.SamaDenganSebelumnya)
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
		// kolom terakhir tiap baris = id_sinkron milik PASTI (nomor riwayat), bukan nomor proses datamart
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
}

// Nilai disalin APA ADANYA: DECIMAL dan koordinat dikirim sebagai teks (bukan []byte, yang akan dibaca SQL Server sebagai varbinary, dan bukan float64 yang kehilangan digit).
func TestSyncCopiesValuesVerbatim(t *testing.T) {
	ds, _ := ByKey("tanah")
	martDB, mart := fakesql.New(t)
	pasti, p := fakesql.New(t)
	pasangDatamart(mart, ds, 1, func(i int, nama string) (driver.Value, bool) {
		switch nama {
		case "Nilai_Tanah":
			return []byte("12345678901234567890123.45"), true // 25 digit: float64 tidak sanggup
		case "Luas_Tanah":
			return []byte("0.0001"), true
		case "Latitude":
			return []byte("-6.1234568"), true
		case "Uraian_tanah":
			return "  tidak dipangkas lagi  ", true // pangkas spasi sudah dikerjakan datamart; aplikasi tidak mengubah nilai
		case "RTRW_tanah":
			return "", true // string kosong tidak diubah jadi NULL oleh aplikasi
		}
		return nil, false
	}, dataPerUji)

	if _, err := Sync(context.Background(), martDB, pasti, ds, Options{RunID: 1}); err != nil {
		t.Fatal(err)
	}
	args := p.Execs()[1].Args
	nilai := map[string]interface{}{}
	for i, c := range ds.Columns {
		nilai[c.Name] = args[i].Value
	}
	for nama, want := range map[string]interface{}{
		"Nilai_Tanah": "12345678901234567890123.45", "Luas_Tanah": "0.0001", "Latitude": "-6.1234568",
		"Uraian_tanah": "  tidak dipangkas lagi  ", "RTRW_tanah": "", "id_aset_tanah": int64(1), "Jumlah_Bangunan": int64(1),
	} {
		if nilai[nama] != want {
			t.Errorf("%s = %#v, want %#v", nama, nilai[nama], want)
		}
	}
}

func TestSyncRejectsUnexpectedColumnType(t *testing.T) {
	ds, _ := ByKey("tanah")
	martDB, mart := fakesql.New(t)
	pasti, p := fakesql.New(t)
	pasangDatamart(mart, ds, 3, func(i int, nama string) (driver.Value, bool) {
		if nama == "Luas_Tanah" {
			return 12.5, true // FLOAT: tabel datamart bukan buatan script
		}
		return nil, false
	}, dataPerUji)
	_, err := Sync(context.Background(), martDB, pasti, ds, Options{})
	if err == nil || !strings.Contains(err.Error(), "Luas_Tanah") || !strings.Contains(err.Error(), "float64") {
		t.Fatalf("err = %v", err)
	}
	if len(p.Events()) != 0 {
		t.Fatalf("PASTI tidak boleh disentuh: %v", p.Events())
	}
}

func TestSyncEmptyResultNeverOverwritesExistingData(t *testing.T) {
	ds, _ := ByKey("tanah")
	martDB, mart := fakesql.New(t)
	pasti, p := fakesql.New(t)
	pasangDatamart(mart, ds, 0, tanpaOverride, dataPerUji)
	existing := int64(10)
	p.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if !strings.HasPrefix(q, "SELECT COUNT(*) FROM [DIGITALISASI_TANAH]") {
			t.Errorf("query tak terduga: %s", q)
		}
		return []string{"n"}, [][]driver.Value{{existing}}, nil
	}

	_, err := Sync(context.Background(), martDB, pasti, ds, Options{})
	if err == nil || !strings.Contains(err.Error(), "TIDAK diganti") {
		t.Fatalf("err = %v", err)
	}
	if p.Count("BEGIN") != 0 || p.Count("EXEC") != 0 {
		t.Fatalf("tabel tidak boleh disentuh: %v", p.Events())
	}

	// Tabel masih kosong: hasil kosong sah (mis. belum ada mess), tidak ada yang perlu dilindungi.
	existing = 0
	res, err := Sync(context.Background(), martDB, pasti, ds, Options{})
	if err != nil || res.Rows != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestSyncKeepsManualColumnsWhenSourceIsNull(t *testing.T) {
	ds, _ := ByKey("satker")
	martDB, mart := fakesql.New(t)
	pasti, p := fakesql.New(t)
	pasangDatamart(mart, ds, 3, func(i int, nama string) (driver.Value, bool) {
		switch nama {
		case "Kode_Satker":
			return []string{"S1", "S2", "S3"}[i], true
		case "Status_Gedung_Kantor", "Foto":
			if i == 2 {
				return "dari-datamart", true // sumber mengisi: tidak boleh ditimpa nilai lama
			}
			return nil, true // datamart tidak punya nilai manual untuk satker ini
		}
		return nil, false
	}, dataPerUji)
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

	if _, err := Sync(context.Background(), martDB, pasti, ds, Options{RunID: 1}); err != nil {
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
	if got := byKode["S3"]; got[0] != "dari-datamart" || got[1] != "dari-datamart" {
		t.Errorf("nilai dari sumber tidak boleh ditimpa nilai lama: %v", got)
	}
	if _, ok := byKode["HILANG"]; ok {
		t.Error("satker yang tidak ada di sumber tidak boleh muncul kembali")
	}
}

func TestSyncRollsBackWhenInsertFails(t *testing.T) {
	ds, _ := ByKey("mess_rumah_negara")
	martDB, mart := fakesql.New(t)
	pasti, p := fakesql.New(t)
	pasangDatamart(mart, ds, 5, tanpaOverride, dataPerUji)
	p.OnExec = func(q string, args []driver.NamedValue) error {
		if strings.HasPrefix(q, "INSERT") {
			return errors.New("String or binary data would be truncated")
		}
		return nil
	}
	_, err := Sync(context.Background(), martDB, pasti, ds, Options{})
	if err == nil || !strings.Contains(err.Error(), "DIGITALISASI_MESS_RUMAH_NEGARA") {
		t.Fatalf("err = %v", err)
	}
	if p.Count("ROLLBACK") != 1 || p.Count("COMMIT") != 0 {
		t.Fatalf("harus rollback, tidak commit: %v", p.Events())
	}
}

func TestSyncRowLimit(t *testing.T) {
	ds, _ := ByKey("satker")
	martDB, mart := fakesql.New(t)
	pasti, p := fakesql.New(t)
	pasangDatamart(mart, ds, 10, tanpaOverride, dataPerUji)
	_, err := Sync(context.Background(), martDB, pasti, ds, Options{MaxRows: 5})
	if err == nil || !strings.Contains(err.Error(), "melebihi 5 baris") {
		t.Fatalf("err = %v", err)
	}
	if len(p.Events()) != 0 {
		t.Fatal("PASTI tidak boleh disentuh")
	}
}

func TestSyncReportsDatamartQueryError(t *testing.T) {
	ds, _ := ByKey("tanah")
	martDB, mart := fakesql.New(t)
	pasti, p := fakesql.New(t)
	mart.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return nil, nil, errors.New("mssql: Invalid object name 'dbo.DIGITALISASI_TANAH'.")
	}
	_, err := Sync(context.Background(), martDB, pasti, ds, Options{})
	if err == nil || !strings.Contains(err.Error(), "Invalid object name") || !strings.Contains(err.Error(), "DIGITALISASI_TANAH") {
		t.Fatalf("err = %v", err)
	}
	if len(p.Events()) != 0 {
		t.Fatal("PASTI tidak boleh disentuh")
	}
}

func TestSyncStopsWhenContextCancelled(t *testing.T) {
	ds, _ := ByKey("tanah")
	martDB, mart := fakesql.New(t)
	pasti, p := fakesql.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	mart.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		cancel() // pengguna menekan Batalkan saat tabel datamart dibaca
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}
	_, err := Sync(ctx, martDB, pasti, ds, Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(p.Events()) != 0 {
		t.Fatal("PASTI tidak boleh disentuh")
	}
}

// Data PASTI tidak boleh mundur: datamart yang LEBIH LAMA daripada sinkronisasi sukses terakhir ditolak; yang sama atau lebih baru dipakai.
func TestSyncRefusesOlderDatamartThanLastSync(t *testing.T) {
	ds, _ := ByKey("tanah")
	martDB, mart := fakesql.New(t)
	pasti, p := fakesql.New(t)
	pasangDatamart(mart, ds, 3, tanpaOverride, dataPerUji)

	lebihBaru := dataPerUji.Add(2 * time.Hour)
	_, err := Sync(context.Background(), martDB, pasti, ds, Options{Sebelumnya: &lebihBaru})
	if err == nil || !strings.Contains(err.Error(), "lebih lama daripada data PASTI") {
		t.Fatalf("err = %v", err)
	}
	if len(p.Events()) != 0 {
		t.Fatalf("PASTI tidak boleh disentuh: %v", p.Events())
	}

	sama := dataPerUji
	res, err := Sync(context.Background(), martDB, pasti, ds, Options{Sebelumnya: &sama})
	if err != nil || !res.SamaDenganSebelumnya {
		t.Fatalf("data per sama harus boleh dan ditandai: res=%+v err=%v", res, err)
	}
	lebihLama := dataPerUji.Add(-48 * time.Hour)
	res, err = Sync(context.Background(), martDB, pasti, ds, Options{Sebelumnya: &lebihLama})
	if err != nil || res.SamaDenganSebelumnya {
		t.Fatalf("datamart yang lebih baru harus dipakai: res=%+v err=%v", res, err)
	}
}

func TestSyncNeedsRefreshTraceOfDatamart(t *testing.T) {
	ds, _ := ByKey("tanah")
	pasti, _ := fakesql.New(t)

	// baris datamart menunjuk proses #41 yang tidak ada di jejak
	martDB, mart := fakesql.New(t)
	mart.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.HasPrefix(q, "SELECT [") {
			cols, rows := datamartBaris(ds, 2, tanpaOverride)
			return cols, rows, nil
		}
		return []string{"mulai"}, nil, nil // tidak ada baris
	}
	if _, err := Sync(context.Background(), martDB, pasti, ds, Options{}); err == nil || !strings.Contains(err.Error(), "#41 tidak ditemukan") {
		t.Fatalf("err = %v", err)
	}

	// tabel kosong dan belum pernah ada penyegaran sukses
	martDB2, mart2 := fakesql.New(t)
	mart2.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.HasPrefix(q, "SELECT [") {
			cols, _ := datamartBaris(ds, 0, tanpaOverride)
			return cols, nil, nil
		}
		return []string{"mulai"}, [][]driver.Value{{nil}}, nil // MAX(mulai) = NULL
	}
	if _, err := Sync(context.Background(), martDB2, pasti, ds, Options{}); err == nil || !strings.Contains(err.Error(), "belum pernah berhasil disegarkan") {
		t.Fatalf("err = %v", err)
	}
}

func TestDataPerFromRowsOfTableNotLatestRun(t *testing.T) {
	// penyegaran #41 menulis baris-baris ini; ada penyegaran lebih baru (#42) yang sedang berjalan atau gagal. Data per mengikuti baris yang dibaca.
	ds, _ := ByKey("tanah")
	martDB, mart := fakesql.New(t)
	pasti, _ := fakesql.New(t)
	lama := dataPerUji.Add(-72 * time.Hour)
	pasangDatamart(mart, ds, 1, tanpaOverride, lama)
	res, err := Sync(context.Background(), martDB, pasti, ds, Options{})
	if err != nil || !res.DataPer.Equal(lama) {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestNilaiDatamart(t *testing.T) {
	txt, dec, big := Column{Name: "T", Kind: Text}, Column{Name: "D", Kind: Decimal, Scale: 2}, Column{Name: "B", Kind: BigInt}
	for _, tc := range []struct {
		c    Column
		in   interface{}
		want interface{}
	}{
		{txt, nil, nil}, {txt, "abc", "abc"}, {txt, "", ""}, {dec, []byte("1.50"), "1.50"}, {big, int64(7), int64(7)}, {big, nil, nil},
	} {
		got, err := nilaiDatamart(tc.c, tc.in)
		if err != nil || got != tc.want {
			t.Errorf("nilaiDatamart(%v, %#v) = %#v, %v; want %#v", tc.c.Name, tc.in, got, err, tc.want)
		}
	}
	for _, in := range []interface{}{1.5, true, time.Now(), int32(3)} {
		if _, err := nilaiDatamart(dec, in); err == nil {
			t.Errorf("tipe %T harus ditolak", in)
		}
	}
}
