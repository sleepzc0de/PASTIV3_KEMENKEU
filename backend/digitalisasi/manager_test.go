package digitalisasi

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"pasti-v3-backend/internal/fakesql"
	"strings"
	"sync"
	"testing"
	"time"
)

type managerEnv struct {
	m     *Manager
	pasti *fakesql.DB
	mu    sync.Mutex
	runs  []string // urutan dataset yang dijalankan
}

func (e *managerEnv) ran() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.runs...)
}

func newManagerEnv(t *testing.T, fn func(ctx context.Context, ds Dataset) (Result, error)) *managerEnv {
	t.Helper()
	pastiDB, p := fakesql.New(t)
	sldkDB, _ := fakesql.New(t)
	var nextID int64
	var idMu sync.Mutex
	p.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if !strings.HasPrefix(q, "INSERT INTO digitalisasi_sync_log") {
			return nil, nil, fmt.Errorf("query tak terduga: %s", q)
		}
		idMu.Lock()
		nextID++
		id := nextID
		idMu.Unlock()
		return []string{"id"}, [][]driver.Value{{id}}, nil
	}
	env := &managerEnv{pasti: p}
	env.m = NewManager(pastiDB, sldkDB)
	env.m.run = func(ctx context.Context, _, _ *sql.DB, ds Dataset, opt Options) (Result, error) {
		env.mu.Lock()
		env.runs = append(env.runs, ds.Key)
		env.mu.Unlock()
		return fn(ctx, ds)
	}
	return env
}

func (e *managerEnv) waitIdle(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for e.m.Active() != nil {
		if time.Now().After(deadline) {
			t.Fatal("antrean tidak selesai dalam 3 detik")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// finals: status akhir dan pesan terakhir tiap baris riwayat (menurut id), dari perintah UPDATE yang tercatat.
func (e *managerEnv) finals() (status map[int64]string, message map[int64]string) {
	status, message = map[int64]string{}, map[int64]string{}
	for _, ex := range e.pasti.Execs() {
		if !strings.HasPrefix(ex.Query, "UPDATE digitalisasi_sync_log SET status = @p1") {
			continue
		}
		switch {
		case strings.Contains(ex.Query, "selesai = SYSUTCDATETIME(), jumlah_baris"): // finishLog
			id := ex.Args[4].Value.(int64)
			status[id] = ex.Args[0].Value.(string)
			message[id] = ex.Args[3].Value.(string)
		case strings.Contains(ex.Query, "mulai = SYSUTCDATETIME()"): // markRunning
			id := ex.Args[2].Value.(int64)
			if _, done := status[id]; !done {
				status[id] = ex.Args[0].Value.(string)
			}
		}
	}
	return status, message
}

func ok(rows int) func(context.Context, Dataset) (Result, error) {
	return func(context.Context, Dataset) (Result, error) { return Result{Rows: rows, WithCoords: rows / 2}, nil }
}

func TestManagerRunsQueueInDatasetOrder(t *testing.T) {
	env := newManagerEnv(t, ok(10))
	info, err := env.m.Start([]string{"tanah", "satker", "tanah"}, "admin1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(info.Datasets, ",") != "satker,tanah" || info.Oleh != "admin1" {
		t.Fatalf("info = %+v; urutan harus mengikuti Datasets dan tanpa duplikat", info)
	}
	env.waitIdle(t)

	if got := strings.Join(env.ran(), ","); got != "satker,tanah" {
		t.Fatalf("urutan jalan = %s", got)
	}
	status, msg := env.finals()
	if status[1] != StatusSukses || status[2] != StatusSukses {
		t.Fatalf("status = %v", status)
	}
	if !strings.HasPrefix(msg[1], "10 baris, 5 berkoordinat") {
		t.Fatalf("pesan = %q", msg[1])
	}
	if n := env.pasti.Count("QUERY INSERT INTO digitalisasi_sync_log"); n != 2 {
		t.Fatalf("baris antrean = %d", n)
	}
}

func TestManagerAllowsOnlyOneRunAtATime(t *testing.T) {
	release := make(chan struct{})
	env := newManagerEnv(t, func(ctx context.Context, ds Dataset) (Result, error) {
		<-release
		return Result{Rows: 1}, nil
	})
	if _, err := env.m.Start([]string{"satker"}, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.m.Start([]string{"tanah"}, "b"); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
	if act := env.m.Active(); act == nil || act.Oleh != "a" || act.Saat != "satker" && act.Saat != "" {
		t.Fatalf("Active = %+v", act)
	}
	close(release)
	env.waitIdle(t)
	if _, err := env.m.Start([]string{"tanah"}, "b"); err != nil {
		t.Fatalf("setelah selesai harus bisa mulai lagi: %v", err)
	}
	env.waitIdle(t)
}

func TestManagerCancelMarksCurrentAndQueued(t *testing.T) {
	started := make(chan struct{}, 1)
	env := newManagerEnv(t, func(ctx context.Context, ds Dataset) (Result, error) {
		started <- struct{}{}
		<-ctx.Done()
		return Result{}, ctx.Err()
	})
	if _, err := env.m.Start([]string{"satker", "tanah", "rusunara"}, "a"); err != nil {
		t.Fatal(err)
	}
	<-started
	if !env.m.Cancel() {
		t.Fatal("Cancel harus berhasil saat ada antrean")
	}
	env.waitIdle(t)

	status, msg := env.finals()
	for id := int64(1); id <= 3; id++ {
		if status[id] != StatusDibatalkan {
			t.Errorf("id %d status = %q, want dibatalkan", id, status[id])
		}
	}
	if !strings.Contains(msg[1], "Dibatalkan oleh pengguna") || !strings.Contains(msg[2], "sebelum dimulai") {
		t.Errorf("pesan = %q / %q", msg[1], msg[2])
	}
	if got := strings.Join(env.ran(), ","); got != "satker" {
		t.Errorf("hanya dataset pertama yang boleh sempat jalan, jalan: %s", got)
	}
	if env.m.Cancel() {
		t.Error("Cancel tanpa antrean harus false")
	}
}

func TestManagerFailureDoesNotStopQueue(t *testing.T) {
	env := newManagerEnv(t, func(ctx context.Context, ds Dataset) (Result, error) {
		if ds.Key == "satker" {
			return Result{}, errors.New("boom")
		}
		return Result{Rows: 3}, nil
	})
	if _, err := env.m.Start([]string{"satker", "tanah"}, "a"); err != nil {
		t.Fatal(err)
	}
	env.waitIdle(t)
	status, msg := env.finals()
	if status[1] != StatusGagal || msg[1] != "boom" {
		t.Errorf("satker: %q %q", status[1], msg[1])
	}
	if status[2] != StatusSukses {
		t.Errorf("tanah tetap harus jalan, status = %q", status[2])
	}
}

func TestManagerTimeout(t *testing.T) {
	env := newManagerEnv(t, func(ctx context.Context, ds Dataset) (Result, error) {
		<-ctx.Done()
		return Result{}, ctx.Err()
	})
	env.m.Timeout = 30 * time.Millisecond
	if _, err := env.m.Start([]string{"satker"}, "a"); err != nil {
		t.Fatal(err)
	}
	env.waitIdle(t)
	status, msg := env.finals()
	if status[1] != StatusGagal || !strings.Contains(msg[1], "batas waktu") {
		t.Fatalf("status = %q, pesan = %q", status[1], msg[1])
	}
}

func TestManagerContainsPanics(t *testing.T) {
	env := newManagerEnv(t, func(ctx context.Context, ds Dataset) (Result, error) { panic("meledak") })
	if _, err := env.m.Start([]string{"satker", "tanah"}, "a"); err != nil {
		t.Fatal(err)
	}
	env.waitIdle(t)
	status, msg := env.finals()
	if status[1] != StatusGagal || status[2] != StatusGagal {
		t.Fatalf("status = %v", status)
	}
	if strings.Contains(msg[1], "meledak") {
		t.Errorf("isi panic tidak boleh bocor ke riwayat: %q", msg[1])
	}
	if _, err := env.m.Start([]string{"satker"}, "a"); err != nil {
		t.Fatalf("manajer harus pulih setelah panic: %v", err)
	}
	env.waitIdle(t)
}

func TestManagerValidation(t *testing.T) {
	env := newManagerEnv(t, ok(1))
	if _, err := env.m.Start(nil, "a"); !errors.Is(err, ErrNothingToRun) {
		t.Errorf("kosong: %v", err)
	}
	if _, err := env.m.Start([]string{"satker", "../etc"}, "a"); !errors.Is(err, ErrUnknownDataset) {
		t.Errorf("tak dikenal: %v", err)
	}
	if env.pasti.Count("QUERY") != 0 || env.pasti.Count("EXEC") != 0 {
		t.Errorf("validasi gagal tidak boleh menulis riwayat: %v", env.pasti.Events())
	}

	noSLDK := NewManager(env.m.pasti, nil)
	if _, err := noSLDK.Start([]string{"satker"}, "a"); !errors.Is(err, ErrNoSLDK) {
		t.Errorf("tanpa SLDK: %v", err)
	}
}

func TestRecoverOrphans(t *testing.T) {
	env := newManagerEnv(t, ok(1))
	if _, err := env.m.RecoverOrphans(context.Background()); err != nil {
		t.Fatal(err)
	}
	ex := env.pasti.Execs()
	if len(ex) != 1 || !strings.Contains(ex[0].Query, "WHERE status IN (@p3, @p4)") {
		t.Fatalf("exec = %+v", ex)
	}
	if ex[0].Args[0].Value != StatusGagal || ex[0].Args[2].Value != StatusAntri || ex[0].Args[3].Value != StatusBerjalan {
		t.Fatalf("args = %+v", ex[0].Args)
	}
}

func TestResultSummary(t *testing.T) {
	r := Result{Rows: 120, WithCoords: 100, Duration: 90 * time.Second, Stats: convStats{BadCoord: 3, Truncated: 2}}
	got := r.Summary()
	for _, want := range []string{"120 baris", "100 berkoordinat", "3 koordinat tidak valid dibuang", "2 teks dipotong", "1m30s"} {
		if !strings.Contains(got, want) {
			t.Errorf("Summary %q tidak memuat %q", got, want)
		}
	}
}
