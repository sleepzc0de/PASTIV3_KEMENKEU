package digitalisasi

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"pasti-v3-backend/internal/fakesql"
)

// Tes sumber data sinkronisasi: datamart, satu-satunya sumber. Tanpa database sungguhan.

type percobaanSinkron struct {
	mart, pasti *sql.DB
	opt         Options
	key         string
}

// pasangPencatat mengganti fungsi sinkronisasi pengelola dengan pencatat yang selalu sukses, dan mengembalikan daftar panggilannya.
func pasangPencatat(env *managerEnv) *[]percobaanSinkron {
	var mu = &env.mu
	panggilan := &[]percobaanSinkron{}
	env.m.run = func(ctx context.Context, mart, pasti *sql.DB, ds Dataset, opt Options) (Result, error) {
		mu.Lock()
		*panggilan = append(*panggilan, percobaanSinkron{mart: mart, pasti: pasti, opt: opt, key: ds.Key})
		mu.Unlock()
		return Result{Rows: 5, WithCoords: 2, DataPer: waktuUji}, nil
	}
	return panggilan
}

func TestSinkronMembacaDatamartDanMencatatSumberDanDataPer(t *testing.T) {
	env := newManagerEnv(t, ok(5))
	panggilan := pasangPencatat(env)
	if _, err := env.m.Start([]string{"satker", "tanah"}, "admin"); err != nil {
		t.Fatal(err)
	}
	env.waitIdle(t)

	if len(*panggilan) != 2 {
		t.Fatalf("panggilan = %d", len(*panggilan))
	}
	for i, p := range *panggilan {
		if p.mart != env.m.datamart || p.pasti != env.m.pasti {
			t.Errorf("%s: sumber harus datamart dan tujuan PASTI", p.key)
		}
		if p.opt.RunID != int64(i+1) {
			t.Errorf("%s: RunID = %d, want nomor riwayat %d", p.key, p.opt.RunID, i+1)
		}
		if p.opt.Sebelumnya != nil {
			t.Errorf("%s: belum pernah ada sinkronisasi sukses, Sebelumnya harus nil: %v", p.key, p.opt.Sebelumnya)
		}
	}

	// sumber dicatat 'datamart' pada tiap baris riwayat, dan data_per tersimpan saat sukses
	sumber := map[int64]string{}
	var dataPer []interface{}
	for _, ex := range env.pasti.Execs() {
		switch {
		case ex.Query == "UPDATE digitalisasi_sync_log SET sumber = @p1 WHERE id = @p2":
			sumber[ex.Args[1].Value.(int64)] = ex.Args[0].Value.(string)
		case strings.Contains(ex.Query, "data_per = @p5"):
			dataPer = append(dataPer, ex.Args[4].Value)
		}
	}
	if sumber[1] != SumberDatamart || sumber[2] != SumberDatamart {
		t.Errorf("sumber = %v", sumber)
	}
	if len(dataPer) != 2 || dataPer[0] != waktuUji || dataPer[1] != waktuUji {
		t.Errorf("data_per = %v, want %v", dataPer, waktuUji)
	}
	_, msg := env.finals()
	if !strings.Contains(msg[1], "5 baris, 2 berkoordinat") || !strings.Contains(msg[1], "dari datamart, data per 26/10/2026 00.00 WIB") {
		t.Errorf("pesan = %q", msg[1])
	}
	if strings.Contains(msg[1], "sama dengan sinkronisasi sebelumnya") {
		t.Errorf("pesan menyebut 'sama' padahal belum ada sinkronisasi sebelumnya: %q", msg[1])
	}
}

func TestSinkronMeneruskanDataPerSinkronisasiTerakhirDataset(t *testing.T) {
	env := newManagerEnv(t, ok(5))
	panggilan := pasangPencatat(env)
	sebelum := waktuUji.Add(-7 * 24 * time.Hour)
	dasar := env.pasti.OnQuery
	var kunci []interface{}
	env.pasti.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.HasPrefix(q, "SELECT TOP (1) data_per FROM digitalisasi_sync_log") {
			if !strings.Contains(q, "status = @p2") || !strings.Contains(q, "data_per IS NOT NULL") {
				t.Errorf("hanya sinkronisasi SUKSES yang punya data_per yang dihitung: %s", q)
			}
			kunci = append(kunci, args[0].Value, args[1].Value)
			return []string{"data_per"}, [][]driver.Value{{sebelum}}, nil
		}
		return dasar(ctx, q, args)
	}
	if _, err := env.m.Start([]string{"tanah"}, "a"); err != nil {
		t.Fatal(err)
	}
	env.waitIdle(t)
	if len(*panggilan) != 1 || (*panggilan)[0].opt.Sebelumnya == nil || !(*panggilan)[0].opt.Sebelumnya.Equal(sebelum) {
		t.Fatalf("Sebelumnya = %v, want %v", (*panggilan)[0].opt.Sebelumnya, sebelum)
	}
	if kunci[0] != "tanah" || kunci[1] != StatusSukses {
		t.Errorf("parameter = %v", kunci)
	}
}

func TestSinkronMenandaiBilaDatamartBelumDisegarkanLagi(t *testing.T) {
	env := newManagerEnv(t, func(context.Context, Dataset) (Result, error) {
		return Result{Rows: 3, DataPer: waktuUji, SamaDenganSebelumnya: true}, nil
	})
	if _, err := env.m.Start([]string{"satker"}, "a"); err != nil {
		t.Fatal(err)
	}
	env.waitIdle(t)
	status, msg := env.finals()
	if status[1] != StatusSukses || !strings.Contains(msg[1], "sama dengan sinkronisasi sebelumnya: datamart belum disegarkan lagi") {
		t.Errorf("status %q, pesan %q", status[1], msg[1])
	}
}

func TestSinkronGagalBilaRiwayatTidakDapatDiperiksa(t *testing.T) {
	// tanpa data_per terakhir, pengaman anti-mundur tidak dapat dijamin: lebih baik gagal daripada menyalin diam-diam
	env := newManagerEnv(t, ok(5))
	panggilan := pasangPencatat(env)
	dasar := env.pasti.OnQuery
	env.pasti.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.HasPrefix(q, "SELECT TOP (1) data_per FROM digitalisasi_sync_log") {
			return nil, nil, errors.New("Invalid column name 'data_per'")
		}
		return dasar(ctx, q, args)
	}
	if _, err := env.m.Start([]string{"satker"}, "a"); err != nil {
		t.Fatal(err)
	}
	env.waitIdle(t)
	status, msg := env.finals()
	if status[1] != StatusGagal || !strings.Contains(msg[1], "tidak dapat memeriksa sinkronisasi terakhir") || !strings.Contains(msg[1], "data_per") {
		t.Errorf("status %q, pesan %q", status[1], msg[1])
	}
	if len(*panggilan) != 0 {
		t.Errorf("sinkronisasi tidak boleh jalan: %v", *panggilan)
	}
}

func TestTanpaDatamartTidakAdaSumber(t *testing.T) {
	pasti, _ := fakesql.New(t)
	m := NewManager(pasti, nil)
	if m.AdaSumber() {
		t.Error("tanpa datamart tidak ada sumber")
	}
	if info := m.InfoSumber(context.Background()); info.Tersedia || info.Terjangkau || info.DataPer != nil {
		t.Errorf("info = %+v", info)
	}
	if _, err := m.sinkronDataset(context.Background(), Datasets[0], 1, nil); !errors.Is(err, ErrNoSumber) {
		t.Errorf("err = %v", err)
	}
}

func infoEnv(t *testing.T, jawab func() ([]string, [][]driver.Value, error)) (*Manager, *fakesql.DB) {
	t.Helper()
	pasti, _ := fakesql.New(t)
	mart, f := fakesql.New(t)
	f.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if !strings.Contains(q, "dbo.datamart_digitalisasi_run") {
			t.Errorf("query tak terduga ke datamart: %s", q)
		}
		return jawab()
	}
	return NewManager(pasti, mart), f
}

func TestInfoSumberMembacaJejakDatamartDenganCache(t *testing.T) {
	m, f := infoEnv(t, func() ([]string, [][]driver.Value, error) {
		return []string{"sukses", "status"}, [][]driver.Value{{waktuUji, "gagal"}}, nil
	})
	info := m.InfoSumber(context.Background())
	if !info.Tersedia || !info.Terjangkau || info.DataPer == nil || !info.DataPer.Equal(waktuUji) || info.PenyegaranTerakhir != "gagal" {
		t.Fatalf("info = %+v", info)
	}
	m.InfoSumber(context.Background())
	if n := len(f.Queries()); n != 1 {
		t.Errorf("polling halaman harus memakai cache: %d query ke datamart", n)
	}
	// cache kedaluwarsa: dibaca lagi
	m.mu.Lock()
	m.cacheSumber.dibaca = time.Now().Add(-2 * masaCacheInfoSumber)
	m.mu.Unlock()
	m.InfoSumber(context.Background())
	if n := len(f.Queries()); n != 2 {
		t.Errorf("query setelah cache kedaluwarsa: %d", n)
	}
}

func TestInfoSumberDatamartBelumPernahDisegarkan(t *testing.T) {
	m, _ := infoEnv(t, func() ([]string, [][]driver.Value, error) {
		return []string{"sukses", "status"}, [][]driver.Value{{nil, nil}}, nil
	})
	info := m.InfoSumber(context.Background())
	if !info.Terjangkau || info.DataPer != nil || info.PenyegaranTerakhir != "" {
		t.Errorf("info = %+v", info)
	}
}

func TestInfoSumberDatamartTidakTerjangkau(t *testing.T) {
	m, _ := infoEnv(t, func() ([]string, [][]driver.Value, error) {
		return nil, nil, errors.New("dial tcp: connection refused")
	})
	info := m.InfoSumber(context.Background())
	if !info.Tersedia || info.Terjangkau || info.DataPer != nil {
		t.Errorf("info = %+v", info)
	}
	// galat mentah (bisa memuat nama server) tidak ikut ke JSON halaman
	if strings.Contains(strings.ToLower(strings.Join([]string{info.PenyegaranTerakhir}, "")), "dial") {
		t.Errorf("info memuat galat mentah: %+v", info)
	}
}
