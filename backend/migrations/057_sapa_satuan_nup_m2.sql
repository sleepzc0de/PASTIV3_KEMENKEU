-- ============================================================
-- Migration 057: SAPA - satuan jumlah BMN "NUP" dan "m2"
-- Aman dijalankan berulang kali (idempotent)
--
-- Masukan pengguna: satuan jumlah BMN pada Nota Dinas usulan Satker harus bisa berupa bidang, NUP, unit, m2, dan seterusnya. Migrasi 022 hanya menyemai
-- bidang, unit, buah, set, paket, dan eksemplar; di sini ditambahkan NUP dan m2 beserta pemetaannya ke jenis BMN yang masuk akal (sama dengan DefaultRefBMN di
-- backend/sapa/bmn.go dan dijaga tes):
--   NUP : semua jenis BMN (setiap barang terdaftar dengan nomor urut pendaftaran)
--   m2  : Tanah, Gedung dan Bangunan, Tanah dan Bangunan (ukuran luas)
-- Hanya DITAMBAHKAN bila belum ada, dan hanya pada jenis yang ada: perubahan admin di Pengaturan SAPA tidak tertimpa. Satuan lain (m, m3, dst) ditambahkan admin sendiri.
-- ============================================================

IF NOT EXISTS (SELECT 1 FROM sapa_satuan WHERE nama = 'NUP') INSERT INTO sapa_satuan (nama, urutan) VALUES ('NUP', 7);
IF NOT EXISTS (SELECT 1 FROM sapa_satuan WHERE nama = 'm2') INSERT INTO sapa_satuan (nama, urutan) VALUES ('m2', 8);

-- Tanah
IF EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Tanah') AND NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn_satuan WHERE jenis = 'Tanah' AND satuan = 'm2')
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Tanah', 'm2', 2);
IF EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Tanah') AND NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn_satuan WHERE jenis = 'Tanah' AND satuan = 'NUP')
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Tanah', 'NUP', 3);

-- Gedung dan Bangunan
IF EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Gedung dan Bangunan') AND NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn_satuan WHERE jenis = 'Gedung dan Bangunan' AND satuan = 'm2')
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Gedung dan Bangunan', 'm2', 3);
IF EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Gedung dan Bangunan') AND NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn_satuan WHERE jenis = 'Gedung dan Bangunan' AND satuan = 'NUP')
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Gedung dan Bangunan', 'NUP', 4);

-- Tanah dan Bangunan
IF EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Tanah dan Bangunan') AND NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn_satuan WHERE jenis = 'Tanah dan Bangunan' AND satuan = 'm2')
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Tanah dan Bangunan', 'm2', 4);
IF EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Tanah dan Bangunan') AND NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn_satuan WHERE jenis = 'Tanah dan Bangunan' AND satuan = 'NUP')
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Tanah dan Bangunan', 'NUP', 5);

-- Peralatan dan Mesin
IF EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Peralatan dan Mesin') AND NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn_satuan WHERE jenis = 'Peralatan dan Mesin' AND satuan = 'NUP')
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Peralatan dan Mesin', 'NUP', 5);

-- Kendaraan Bermotor
IF EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Kendaraan Bermotor') AND NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn_satuan WHERE jenis = 'Kendaraan Bermotor' AND satuan = 'NUP')
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Kendaraan Bermotor', 'NUP', 2);

-- Jalan, Irigasi, dan Jaringan
IF EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Jalan, Irigasi, dan Jaringan') AND NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn_satuan WHERE jenis = 'Jalan, Irigasi, dan Jaringan' AND satuan = 'NUP')
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Jalan, Irigasi, dan Jaringan', 'NUP', 3);

-- Aset Tetap Lainnya
IF EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Aset Tetap Lainnya') AND NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn_satuan WHERE jenis = 'Aset Tetap Lainnya' AND satuan = 'NUP')
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Aset Tetap Lainnya', 'NUP', 5);
