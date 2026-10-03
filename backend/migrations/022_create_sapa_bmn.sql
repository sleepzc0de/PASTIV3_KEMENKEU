-- ============================================================
-- Migration 022: SAPA - daftar jenis BMN dan satuan jumlahnya
-- Aman dijalankan berulang kali (idempotent)
--
-- Admin mengatur daftar ini di Pengaturan SAPA. Setiap jenis BMN hanya boleh memakai satuan yang dipetakan padanya
-- (sapa_jenis_bmn_satuan), supaya pasangan yang tidak masuk akal (mis. Peralatan dan Mesin dalam "meter", atau Tanah dalam
-- "unit") tidak mungkin dipilih. Nama menjadi kunci: jenis dan satuan tidak diganti namanya, melainkan dinonaktifkan
-- dan diganti yang baru, sehingga usulan lama (yang menyimpan nama sebagai teks) tetap utuh.
-- ============================================================

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sapa_satuan')
BEGIN
    CREATE TABLE sapa_satuan (
        nama NVARCHAR(30) NOT NULL PRIMARY KEY,
        aktif BIT NOT NULL DEFAULT 1,
        urutan INT NOT NULL DEFAULT 0,
        diubah_oleh NVARCHAR(100) NULL,
        diubah_pada DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );
END;

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sapa_jenis_bmn')
BEGIN
    CREATE TABLE sapa_jenis_bmn (
        nama NVARCHAR(100) NOT NULL PRIMARY KEY,
        aktif BIT NOT NULL DEFAULT 1,
        urutan INT NOT NULL DEFAULT 0,
        satuan_bawaan NVARCHAR(30) NULL,     -- satuan yang dipilih otomatis saat jenis dipilih
        diubah_oleh NVARCHAR(100) NULL,
        diubah_pada DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
    );
END;

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'sapa_jenis_bmn_satuan')
BEGIN
    CREATE TABLE sapa_jenis_bmn_satuan (
        jenis NVARCHAR(100) NOT NULL,
        satuan NVARCHAR(30) NOT NULL,
        urutan INT NOT NULL DEFAULT 0,
        CONSTRAINT pk_sapa_jenis_bmn_satuan PRIMARY KEY (jenis, satuan),
        CONSTRAINT fk_sapa_jbs_jenis FOREIGN KEY (jenis) REFERENCES sapa_jenis_bmn(nama) ON DELETE CASCADE,
        CONSTRAINT fk_sapa_jbs_satuan FOREIGN KEY (satuan) REFERENCES sapa_satuan(nama) ON DELETE CASCADE
    );
END;

-- Data awal (saran yang bisa diubah admin; sama dengan DefaultRefBMN di backend/sapa/bmn.go dan dijaga tes). Ditambahkan hanya
-- bila belum ada, sehingga perubahan admin tidak tertimpa.
IF NOT EXISTS (SELECT 1 FROM sapa_satuan WHERE nama = 'bidang') INSERT INTO sapa_satuan (nama, urutan) VALUES ('bidang', 1);
IF NOT EXISTS (SELECT 1 FROM sapa_satuan WHERE nama = 'unit') INSERT INTO sapa_satuan (nama, urutan) VALUES ('unit', 2);
IF NOT EXISTS (SELECT 1 FROM sapa_satuan WHERE nama = 'buah') INSERT INTO sapa_satuan (nama, urutan) VALUES ('buah', 3);
IF NOT EXISTS (SELECT 1 FROM sapa_satuan WHERE nama = 'set') INSERT INTO sapa_satuan (nama, urutan) VALUES ('set', 4);
IF NOT EXISTS (SELECT 1 FROM sapa_satuan WHERE nama = 'paket') INSERT INTO sapa_satuan (nama, urutan) VALUES ('paket', 5);
IF NOT EXISTS (SELECT 1 FROM sapa_satuan WHERE nama = 'eksemplar') INSERT INTO sapa_satuan (nama, urutan) VALUES ('eksemplar', 6);

IF NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Tanah')
BEGIN
    INSERT INTO sapa_jenis_bmn (nama, urutan, satuan_bawaan) VALUES ('Tanah', 1, 'bidang');
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Tanah', 'bidang', 1);
END;

IF NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Gedung dan Bangunan')
BEGIN
    INSERT INTO sapa_jenis_bmn (nama, urutan, satuan_bawaan) VALUES ('Gedung dan Bangunan', 2, 'unit');
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Gedung dan Bangunan', 'unit', 1), ('Gedung dan Bangunan', 'buah', 2);
END;

IF NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Tanah dan Bangunan')
BEGIN
    INSERT INTO sapa_jenis_bmn (nama, urutan, satuan_bawaan) VALUES ('Tanah dan Bangunan', 3, 'unit');
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Tanah dan Bangunan', 'bidang', 1), ('Tanah dan Bangunan', 'unit', 2), ('Tanah dan Bangunan', 'paket', 3);
END;

IF NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Peralatan dan Mesin')
BEGIN
    INSERT INTO sapa_jenis_bmn (nama, urutan, satuan_bawaan) VALUES ('Peralatan dan Mesin', 4, 'unit');
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Peralatan dan Mesin', 'unit', 1), ('Peralatan dan Mesin', 'buah', 2), ('Peralatan dan Mesin', 'set', 3), ('Peralatan dan Mesin', 'paket', 4);
END;

IF NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Kendaraan Bermotor')
BEGIN
    INSERT INTO sapa_jenis_bmn (nama, urutan, satuan_bawaan) VALUES ('Kendaraan Bermotor', 5, 'unit');
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Kendaraan Bermotor', 'unit', 1);
END;

IF NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Jalan, Irigasi, dan Jaringan')
BEGIN
    INSERT INTO sapa_jenis_bmn (nama, urutan, satuan_bawaan) VALUES ('Jalan, Irigasi, dan Jaringan', 6, 'unit');
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Jalan, Irigasi, dan Jaringan', 'unit', 1), ('Jalan, Irigasi, dan Jaringan', 'paket', 2);
END;

IF NOT EXISTS (SELECT 1 FROM sapa_jenis_bmn WHERE nama = 'Aset Tetap Lainnya')
BEGIN
    INSERT INTO sapa_jenis_bmn (nama, urutan, satuan_bawaan) VALUES ('Aset Tetap Lainnya', 7, 'unit');
    INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES ('Aset Tetap Lainnya', 'unit', 1), ('Aset Tetap Lainnya', 'buah', 2), ('Aset Tetap Lainnya', 'set', 3), ('Aset Tetap Lainnya', 'eksemplar', 4);
END;
