package monitor

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Keadaan database: pool koneksi aplikasi (selalu tersedia) dan SQL Server (bagian yang butuh izin VIEW SERVER STATE / VIEW DATABASE STATE dilewati bila akun database aplikasi
// tidak punya izin itu, dan alasannya dicatat di InfoSQL.Catatan).

// poolDari membaca statistik kolam koneksi. ping: ukur juga waktu satu round-trip ke server (bila false, tidak mengirim apa pun ke database).
func poolDari(ctx context.Context, nama string, db *sql.DB, ping bool) *PoolDB {
	if db == nil {
		return nil
	}
	st := db.Stats()
	p := &PoolDB{
		Nama: nama, Batas: st.MaxOpenConnections, Terbuka: st.OpenConnections, Dipakai: st.InUse, Menganggur: st.Idle,
		Menunggu: st.WaitCount, MenungguMS: float64(st.WaitDuration.Microseconds()) / 1000,
	}
	if ping {
		c, batal := context.WithTimeout(ctx, 4*time.Second)
		defer batal()
		mulai := time.Now()
		if err := db.PingContext(c); err != nil {
			p.Galat = potongPesan(err.Error(), 160)
		} else {
			p.Tersambung = true
			p.PingMS = float64(time.Since(mulai).Microseconds()) / 1000
		}
	} else {
		p.Tersambung = true
	}
	return p
}

func potongPesan(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// bacaInfoSQL membaca keadaan SQL Server. Setiap bagian dijalankan sendiri-sendiri dengan batas waktu supaya satu bagian yang ditolak atau lambat tidak menggagalkan yang lain.
func bacaInfoSQL(ctx context.Context, db *sql.DB) *InfoSQL {
	if db == nil {
		return nil
	}
	info := &InfoSQL{DiukurPada: time.Now().UTC(), File: []FileDB{}, Volume: []VolumeDB{}}
	catat := func(bagian string, err error) {
		info.Catatan = append(info.Catatan, fmt.Sprintf("%s: %s", bagian, potongPesan(err.Error(), 160)))
	}
	coba := func(bagian string, fn func(c context.Context) error) {
		c, batal := context.WithTimeout(ctx, 6*time.Second)
		defer batal()
		if err := fn(c); err != nil {
			catat(bagian, err)
		}
	}

	coba("Versi SQL Server", func(c context.Context) error {
		return db.QueryRowContext(c, `SELECT CAST(SERVERPROPERTY('ProductVersion') AS NVARCHAR(50)), CAST(SERVERPROPERTY('Edition') AS NVARCHAR(100)), DB_NAME()`).
			Scan(&info.Versi, &info.Edisi, &info.NamaDB)
	})

	coba("Ukuran file database", func(c context.Context) error {
		rows, err := db.QueryContext(c, `SELECT name, type_desc, CAST(size AS BIGINT) * 8 / 1024.0, CAST(FILEPROPERTY(name, 'SpaceUsed') AS BIGINT) * 8 / 1024.0,
				CASE WHEN max_size <= 0 THEN NULL ELSE CAST(max_size AS BIGINT) * 8 / 1024.0 END
			FROM sys.database_files`)
		if err != nil {
			return err
		}
		defer rows.Close()
		var terpakai float64
		adaTerpakai := false
		for rows.Next() {
			var f FileDB
			var pakai, maks sql.NullFloat64
			if err := rows.Scan(&f.Nama, &f.Jenis, &f.UkuranMB, &pakai, &maks); err != nil {
				return err
			}
			if pakai.Valid {
				v := pakai.Float64
				f.TerpakaiMB = &v
				terpakai += v
				adaTerpakai = true
			}
			if maks.Valid {
				v := maks.Float64
				f.MaksMB = &v
			}
			info.UkuranMB += f.UkuranMB
			info.File = append(info.File, f)
		}
		if adaTerpakai {
			info.TerpakaiMB = &terpakai
		}
		return rows.Err()
	})

	coba("Ruang disk volume database (butuh izin VIEW SERVER STATE)", func(c context.Context) error {
		rows, err := db.QueryContext(c, `SELECT DISTINCT vs.volume_mount_point, vs.total_bytes, vs.available_bytes
			FROM sys.database_files f CROSS APPLY sys.dm_os_volume_stats(DB_ID(), f.file_id) vs`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var v VolumeDB
			var total, bebas int64
			if err := rows.Scan(&v.Titik, &total, &bebas); err != nil {
				return err
			}
			v.Total, v.Bebas = uint64(total), uint64(bebas)
			if total > 0 {
				v.Persen = float64(total-bebas) / float64(total) * 100
			}
			info.Volume = append(info.Volume, v)
		}
		return rows.Err()
	})

	coba("CPU dan memori server database (butuh izin VIEW SERVER STATE)", func(c context.Context) error {
		var cpu int
		var fisikKB int64
		if err := db.QueryRowContext(c, `SELECT cpu_count, physical_memory_kb FROM sys.dm_os_sys_info`).Scan(&cpu, &fisikKB); err != nil {
			return err
		}
		m := float64(fisikKB) / 1024
		info.CPUJumlah, info.MemFisikMB = &cpu, &m
		var prosesKB int64
		if err := db.QueryRowContext(c, `SELECT physical_memory_in_use_kb FROM sys.dm_os_process_memory`).Scan(&prosesKB); err == nil {
			p := float64(prosesKB) / 1024
			info.MemProsesMB = &p
		}
		return nil
	})

	coba("Page life expectancy dan buffer cache (butuh izin VIEW SERVER STATE)", func(c context.Context) error {
		rows, err := db.QueryContext(c, `SELECT RTRIM(counter_name), cntr_value FROM sys.dm_os_performance_counters
			WHERE counter_name IN (N'Page life expectancy', N'Buffer cache hit ratio', N'Buffer cache hit ratio base')
			  AND object_name LIKE N'%Buffer Manager%'`)
		if err != nil {
			return err
		}
		defer rows.Close()
		var rasio, dasar float64
		for rows.Next() {
			var nama string
			var v int64
			if err := rows.Scan(&nama, &v); err != nil {
				return err
			}
			switch nama {
			case "Page life expectancy":
				f := float64(v)
				info.PLE = &f
			case "Buffer cache hit ratio":
				rasio = float64(v)
			case "Buffer cache hit ratio base":
				dasar = float64(v)
			}
		}
		if dasar > 0 {
			h := rasio / dasar * 100
			info.BufferHitRatio = &h
		}
		return rows.Err()
	})

	coba("Sesi pengguna (hanya sesi akun ini bila tanpa izin VIEW SERVER STATE)", func(c context.Context) error {
		var n int
		if err := db.QueryRowContext(c, `SELECT COUNT(*) FROM sys.dm_exec_sessions WHERE is_user_process = 1`).Scan(&n); err != nil {
			return err
		}
		info.SesiPengguna = &n
		return nil
	})
	return info
}

// bacaTabelTeratas membaca tabel terbesar pada database aplikasi (butuh izin VIEW DATABASE STATE).
func bacaTabelTeratas(ctx context.Context, db *sql.DB, n int) ([]TabelDB, error) {
	if db == nil {
		return nil, nil
	}
	c, batal := context.WithTimeout(ctx, 10*time.Second)
	defer batal()
	rows, err := db.QueryContext(c, fmt.Sprintf(`SELECT TOP (%d) s.name + N'.' + t.name,
			SUM(CASE WHEN ps.index_id IN (0, 1) THEN ps.row_count ELSE 0 END),
			SUM(ps.reserved_page_count) * 8 / 1024.0
		FROM sys.dm_db_partition_stats ps
		JOIN sys.tables t ON t.object_id = ps.object_id
		JOIN sys.schemas s ON s.schema_id = t.schema_id
		GROUP BY s.name, t.name
		ORDER BY 3 DESC, 1`, n))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TabelDB{}
	for rows.Next() {
		var t TabelDB
		if err := rows.Scan(&t.Nama, &t.Baris, &t.UkuranMB); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
