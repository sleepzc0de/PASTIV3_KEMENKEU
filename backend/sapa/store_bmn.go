package sapa

import (
	"context"
	"database/sql"
	"strings"
)

// Penyimpanan SQL daftar jenis BMN dan satuan jumlahnya (migrasi 022).

func (s *Store) AmbilRefBMN(ctx context.Context) (RefBMN, error) {
	ref := RefBMN{Jenis: []JenisBMN{}, Satuan: []SatuanBMN{}}

	rows, err := s.DB.QueryContext(ctx, `SELECT nama, aktif, urutan FROM sapa_satuan ORDER BY urutan, nama`)
	if err != nil {
		return ref, err
	}
	for rows.Next() {
		var x SatuanBMN
		if err := rows.Scan(&x.Nama, &x.Aktif, &x.Urutan); err != nil {
			rows.Close()
			return ref, err
		}
		ref.Satuan = append(ref.Satuan, x)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return ref, err
	}
	rows.Close()

	rows, err = s.DB.QueryContext(ctx, `SELECT nama, aktif, urutan, ISNULL(satuan_bawaan, '') FROM sapa_jenis_bmn ORDER BY urutan, nama`)
	if err != nil {
		return ref, err
	}
	for rows.Next() {
		var j JenisBMN
		if err := rows.Scan(&j.Nama, &j.Aktif, &j.Urutan, &j.SatuanBawaan); err != nil {
			rows.Close()
			return ref, err
		}
		j.Satuan = []string{}
		ref.Jenis = append(ref.Jenis, j)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return ref, err
	}
	rows.Close()

	rows, err = s.DB.QueryContext(ctx, `SELECT jenis, satuan FROM sapa_jenis_bmn_satuan ORDER BY urutan, satuan`)
	if err != nil {
		return ref, err
	}
	defer rows.Close()
	idx := map[string]int{}
	for i, j := range ref.Jenis {
		idx[strings.ToLower(j.Nama)] = i
	}
	for rows.Next() {
		var jenis, satuan string
		if err := rows.Scan(&jenis, &satuan); err != nil {
			return ref, err
		}
		if i, ok := idx[strings.ToLower(jenis)]; ok {
			ref.Jenis[i].Satuan = append(ref.Jenis[i].Satuan, satuan)
		}
	}
	return ref, rows.Err()
}

func (s *Store) SimpanSatuanBMN(ctx context.Context, x SatuanBMN, oleh string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE sapa_satuan WITH (UPDLOCK, SERIALIZABLE)
			    SET aktif = @p2, urutan = @p3, diubah_oleh = @p4, diubah_pada = SYSUTCDATETIME()
			  WHERE nama = @p1;
			 IF @@ROWCOUNT = 0
			     INSERT INTO sapa_satuan (nama, aktif, urutan, diubah_oleh) VALUES (@p1, @p2, @p3, @p4);`,
			x.Nama, x.Aktif, x.Urutan, oleh)
		return err
	})
}

// HapusSatuanBMN menghapus satuan; pemetaannya ke jenis ikut terhapus (ON DELETE CASCADE).
func (s *Store) HapusSatuanBMN(ctx context.Context, nama string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sapa_satuan WHERE nama = @p1`, nama)
	return err
}

// SimpanJenisBMN menyimpan jenis beserta seluruh pemetaan satuannya dalam satu transaksi.
func (s *Store) SimpanJenisBMN(ctx context.Context, j JenisBMN, oleh string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE sapa_jenis_bmn WITH (UPDLOCK, SERIALIZABLE)
			    SET aktif = @p2, urutan = @p3, satuan_bawaan = @p4, diubah_oleh = @p5, diubah_pada = SYSUTCDATETIME()
			  WHERE nama = @p1;
			 IF @@ROWCOUNT = 0
			     INSERT INTO sapa_jenis_bmn (nama, aktif, urutan, satuan_bawaan, diubah_oleh) VALUES (@p1, @p2, @p3, @p4, @p5);`,
			j.Nama, j.Aktif, j.Urutan, nullStr(j.SatuanBawaan), oleh); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM sapa_jenis_bmn_satuan WHERE jenis = @p1`, j.Nama); err != nil {
			return err
		}
		for i, sat := range j.Satuan {
			if _, err := tx.ExecContext(ctx, `INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES (@p1, @p2, @p3)`, j.Nama, sat, i+1); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) HapusJenisBMN(ctx context.Context, nama string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sapa_jenis_bmn WHERE nama = @p1`, nama)
	return err
}
