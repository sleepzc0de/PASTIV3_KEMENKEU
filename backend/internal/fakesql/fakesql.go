// Package fakesql adalah driver database/sql palsu khusus tes: tidak ada SQL Server. Hasil query dan perilaku
// Exec ditentukan lewat hook, dan semua kejadian (BEGIN, QUERY, EXEC, COMMIT, ROLLBACK) dicatat berurutan
// supaya urutan dan isi perintahnya bisa diperiksa. Tidak memverifikasi sintaks T-SQL.
package fakesql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

// Stmt: satu perintah yang diterima driver beserta argumennya.
type Stmt struct {
	Query string
	Args  []driver.NamedValue
}

// DB: satu database palsu. OnQuery menjawab perintah yang mengembalikan baris (SELECT, INSERT ... OUTPUT);
// OnExec boleh mengembalikan galat untuk perintah tanpa hasil.
type DB struct {
	OnQuery func(ctx context.Context, query string, args []driver.NamedValue) (cols []string, rows [][]driver.Value, err error)
	OnExec  func(query string, args []driver.NamedValue) error

	mu      sync.Mutex
	events  []string
	execs   []Stmt
	queries []Stmt
}

var (
	regMu   sync.Mutex
	regSeq  int
	regDBs  = map[string]*DB{}
	regOnce sync.Once
)

type fakeDriver struct{}

func (fakeDriver) Open(name string) (driver.Conn, error) {
	regMu.Lock()
	db := regDBs[name]
	regMu.Unlock()
	if db == nil {
		return nil, errors.New("fakesql: DSN tidak dikenal")
	}
	return &conn{db: db}, nil
}

// New membuat *sql.DB palsu beserta pengendalinya.
func New(t testing.TB) (*sql.DB, *DB) {
	t.Helper()
	regOnce.Do(func() { sql.Register("fakesql", fakeDriver{}) })
	regMu.Lock()
	regSeq++
	dsn := fmt.Sprintf("db%d", regSeq)
	f := &DB{}
	regDBs[dsn] = f
	regMu.Unlock()
	db, err := sql.Open("fakesql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, f
}

func (d *DB) record(ev string) {
	d.mu.Lock()
	d.events = append(d.events, ev)
	d.mu.Unlock()
}

// Events: kejadian berurutan, mis. "BEGIN", "EXEC DELETE FROM ...", "QUERY SELECT ...", "COMMIT".
func (d *DB) Events() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.events...)
}

// Execs: perintah Exec berurutan (kecuali yang dijalankan lewat Query).
func (d *DB) Execs() []Stmt {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]Stmt(nil), d.execs...)
}

// Queries: perintah Query berurutan, dengan teks lengkap.
func (d *DB) Queries() []Stmt {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]Stmt(nil), d.queries...)
}

// Count: jumlah kejadian yang diawali prefix.
func (d *DB) Count(prefix string) int {
	n := 0
	for _, e := range d.Events() {
		if strings.HasPrefix(e, prefix) {
			n++
		}
	}
	return n
}

func short(q string) string {
	q = strings.Join(strings.Fields(q), " ")
	if len(q) > 60 {
		q = q[:60]
	}
	return q
}

type conn struct{ db *DB }

func (c *conn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fakesql: Prepare tidak didukung")
}
func (c *conn) Close() error              { return nil }
func (c *conn) Begin() (driver.Tx, error) { c.db.record("BEGIN"); return &tx{c.db}, nil }
func (c *conn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.db.record("BEGIN")
	return &tx{c.db}, nil
}

func (c *conn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.db.record("QUERY " + short(q))
	c.db.mu.Lock()
	c.db.queries = append(c.db.queries, Stmt{Query: q, Args: append([]driver.NamedValue(nil), args...)})
	c.db.mu.Unlock()
	if c.db.OnQuery == nil {
		return nil, errors.New("fakesql: OnQuery belum diatur")
	}
	cols, rows, err := c.db.OnQuery(ctx, q, args)
	if err != nil {
		return nil, err
	}
	return &result{cols: cols, rows: rows}, nil
}

func (c *conn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.db.record("EXEC " + short(q))
	c.db.mu.Lock()
	c.db.execs = append(c.db.execs, Stmt{Query: q, Args: append([]driver.NamedValue(nil), args...)})
	c.db.mu.Unlock()
	if c.db.OnExec != nil {
		if err := c.db.OnExec(q, args); err != nil {
			return nil, err
		}
	}
	return driver.RowsAffected(1), nil
}

type tx struct{ db *DB }

func (t *tx) Commit() error   { t.db.record("COMMIT"); return nil }
func (t *tx) Rollback() error { t.db.record("ROLLBACK"); return nil }

type result struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *result) Columns() []string { return r.cols }
func (r *result) Close() error      { return nil }
func (r *result) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}
