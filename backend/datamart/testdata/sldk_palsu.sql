-- SLDK PALSU untuk menguji buat_datamart_sldk.sql. Dijalankan di database kosong KHUSUS TES (bukan SLDK/PASTI asli). Semua data fiktif.
--
-- Isinya meniru bentuk tabel SLDK yang dipakai query Digitalisasi Aset, ditambah kolom yang TIDAK dipakai query (nilai_buku, keterangan, ... harus tidak ikut ke datamart)
-- dan kasus tepi yang penting:
--   * aset berstatus N, aset non-015, dan aset di luar kelompok kode barang (jalan, kendaraan non-015) tidak boleh ikut;
--   * tanah non-015 yang ditautkan ke gedung kantor 015 (aset tautan) harus ikut, karena query gedung kantor membaca tanahnya;
--   * bangunan (kode barang di luar kelompok) di atas tanah 015 harus ikut bila berstatus Y, dan tidak bila berstatus N (dihitung Jumlah_Bangunan);
--   * dua snapshot (TGL_TARIK) untuk mess dan rusunara; dua snapshot status hukum; dua baris satker kembar (dedup menurut updated_at);
--   * nama penghuni hanya untuk rumah negara (bukan mess, bukan tanah); VW_SIMAN_RUANG_GEDUNG_BANGUNAN_015 berupa VIEW; kolom IDENTITY pada photo.
SET NOCOUNT ON;
GO
CREATE SCHEMA DJKN;
GO

CREATE TABLE DJKN.SIMAN2_R_SATKER (
    id_satker INT NOT NULL, kd_satker VARCHAR(21) NOT NULL, ur_satker VARCHAR(200) NULL, alamat_satker VARCHAR(500) NULL,
    nm_kel VARCHAR(100) NULL, nm_kec VARCHAR(100) NULL, nm_kab_kota VARCHAR(100) NULL, nm_prov VARCHAR(100) NULL,
    status_uakpb VARCHAR(10) NULL, updated_at DATETIME NULL, email_satker VARCHAR(100) NULL);
INSERT DJKN.SIMAN2_R_SATKER VALUES
 (1, '015040199119091000KP', 'KPP Induk Uji',   'Jl. Induk 1',  'Kel A', 'Kec A', 'Kota A', 'Prov A', 'AKTIF',    '2025-01-02', 'a@uji.test'),
 (2, '015040199119091001KP', 'KPP Anak Uji',    'Jl. Anak 2',   'Kel B', 'Kec B', 'Kota B', 'Prov B', 'AKTIF',    '2025-01-02', 'b@uji.test'),
 (3, '015050199410640000KP', 'Kantor Bea Uji',  'Jl. Bea 3',    'Kel C', 'Kec C', 'Kota C', 'Prov C', 'AKTIF',    '2025-01-02', 'c@uji.test'),
 (4, '015100199100001000KP', 'Satker Uji',      'Jl. Uji 4',   'Kel D', 'Kec D', 'Kota D', 'Prov D', 'AKTIF',    '2025-01-02', 'd@uji.test'),
 -- baris kembar (heap tanpa PK di SLDK asli) untuk satker tanpa aset: dedup menurut updated_at diuji lewat satker.sql
 (8, '015050199410640000KP', 'Kantor Bea Uji Lama', 'Jl. Bea Lama 3', 'Kel C', 'Kec C', 'Kota C', 'Prov C', 'AKTIF', '2023-01-01', 'c0@uji.test'),
 (5, '015100199100002000KP', 'Satker Nonaktif', 'Jl. Mati 5',   'Kel E', 'Kec E', 'Kota E', 'Prov E', 'NONAKTIF', '2025-01-02', 'e@uji.test'),
 (6, '015010199409294002KP', 'Satker Dikecualikan', 'Jl. 6',    'Kel F', 'Kec F', 'Kota F', 'Prov F', 'AKTIF',    '2025-01-02', 'f@uji.test'),
 (7, '999990199100001000KP', 'Bukan KL 015',    'Jl. Luar 7',   'Kel G', 'Kec G', 'Kota G', 'Prov G', 'AKTIF',    '2025-01-02', 'g@uji.test');

CREATE TABLE DJKN.SIMAN2_R_KONDISI (kd_kondisi VARCHAR(2) NOT NULL, ur_kondisi VARCHAR(50) NULL, urutan INT NULL);
INSERT DJKN.SIMAN2_R_KONDISI VALUES ('1', 'Baik', 1), ('2', 'Rusak Ringan', 2), ('3', 'Rusak Berat', 3);

CREATE TABLE DJKN.SIMAN2_R_STATUS_HUKUM_NEW (
    id_status_hukum INT NOT NULL, kd_status_hukum VARCHAR(5) NOT NULL, ur_status_hukum VARCHAR(100) NULL, label VARCHAR(50) NULL, TGL_TARIK INT NOT NULL, catatan VARCHAR(100) NULL);
INSERT DJKN.SIMAN2_R_STATUS_HUKUM_NEW VALUES
 (1, '01', 'Bersertipikat (lama)', 'SHM',  20240101, 'snapshot lama'),
 (1, '01', 'Bersertipikat',        'SHM',  20250101, NULL),
 (2, '01', 'Hak Pakai',            'HP',   20250101, NULL),
 (3, '02', 'Belum Bersertipikat',  'BELUM',20250101, NULL),
 (4, '09', 'Status Lama',          'OLD',  20240101, 'hanya ada di snapshot lama');

CREATE TABLE DJKN.SIMAN2_M_ASET (
    id_aset BIGINT NOT NULL, kode_register VARCHAR(60) NULL, kd_satker VARCHAR(21) NULL, kd_brg VARCHAR(10) NULL, ur_sskel VARCHAR(200) NULL, no_aset INT NULL,
    alamat VARCHAR(500) NULL, kd_rtrw VARCHAR(20) NULL, ur_kel VARCHAR(100) NULL, ur_kec VARCHAR(100) NULL, ur_kab VARCHAR(100) NULL, ur_prov VARCHAR(100) NULL,
    luas DECIMAL(18,2) NULL, luas_tnhb DECIMAL(18,2) NULL, kd_kondisi VARCHAR(2) NULL, rph_aset DECIMAL(20,2) NULL, gps_latitude VARCHAR(30) NULL, gps_longitude VARCHAR(30) NULL,
    status_bmn_yn CHAR(1) NULL, kd_status VARCHAR(2) NULL, kd_status_hukum VARCHAR(5) NULL, kd_jns_bmn INT NULL, merk VARCHAR(100) NULL, nm_unit_pengguna VARCHAR(200) NULL,
    TGL_TARIK INT NOT NULL,
    nilai_buku DECIMAL(20,2) NULL, keterangan VARCHAR(500) NULL, dibuat_oleh VARCHAR(50) NULL);   -- tiga kolom terakhir tidak dipakai query

-- (id, register, satker, brg, uraian, nup, alamat, rtrw, kel, kec, kab, prov, luas, luas_tnhb, kondisi, rph, lat, lng, status, kd_status, hukum, jns, merk, unit, tgl_tarik, nilai_buku, ket, oleh)
INSERT DJKN.SIMAN2_M_ASET VALUES
 -- tanah
 (100, 'REG-T-100', '015100199100001000KP', '2010101001', 'Tanah Kantor A',   1, 'Jl. Tanah 100', '01/02', 'KelT', 'KecT', 'KotaT', 'ProvT', 1000.50, 300.25, '1', 5000000000.00, '-6,2',     '106,8',    'Y', NULL, '01', 1, NULL, NULL, 20250101, 1.00, 'x', 'u'),
 (101, 'REG-T-101', '015040199119091000KP', '2010101001', 'Tanah Kantor B',   2, NULL,            NULL,    NULL,   NULL,   NULL,    NULL,    2000.00, 500.00, '2', 9000000000.00, '-7.25',    '112.75',   'Y', NULL, '02', 1, NULL, NULL, 20250101, 1.00, 'x', 'u'),
 (102, 'REG-T-102', '999990199100001000KP', '2010101001', 'Tanah Pihak Luar',  1, 'Jl. Luar 102', NULL,    NULL,   NULL,   NULL,    NULL,    700.00,  100.00, '1', 1000000000.00, '-1.0',     '101.0',    'Y', NULL, '01', 1, NULL, NULL, 20250101, 1.00, 'x', 'u'),
 (103, 'REG-T-103', '015100199100001000KP', '2010101001', 'Tanah Dihapus',    3, 'Jl. Hapus 103', NULL,   NULL,   NULL,   NULL,    NULL,    10.00,   0,      '1', 1.00,             NULL,       NULL,       'N', NULL, '01', 1, NULL, NULL, 20250101, 1.00, 'x', 'u'),
 -- gedung kantor utama (tautan ke tanah 102 yang non-015) dan gedung lainnya
 (200, 'REG-B-200', '015040199119091000KP', '4010101001', 'Gedung Kantor Utama', 1, 'Jl. Kantor 200', '03/04', 'KelB', 'KecB', 'KotaB', 'ProvB', 1200.00, NULL, '1', 8000000000.00, '-6.3', '106.9', 'Y', NULL, NULL, 1, NULL, 'Unit Pengguna Kantor', 20250101, 1.00, 'x', 'u'),
 (201, 'REG-B-201', '015100199100001000KP', '4010103001', 'Gedung Lainnya',   1, NULL,            NULL,    NULL,   NULL,   NULL,    NULL,    300.00,  NULL, '3', 1500000000.00, NULL,   NULL,    'Y', NULL, '01', 1, 'Gudang', 'Unit Gudang',       20250101, 1.00, 'x', 'u'),
 -- bangunan di atas tanah 101: 202 (kode di luar kelompok, status Y: ikut sebagai tautan), 203 (status N: tidak ikut)
 (202, 'REG-B-202', '015040199119091000KP', '4020101001', 'Bangunan Lain',   1, NULL, NULL, NULL, NULL, NULL, NULL, 50.00, NULL, '1', 100.00, NULL, NULL, 'Y', NULL, NULL, 1, NULL, NULL, 20250101, 1.00, 'x', 'u'),
 (203, 'REG-B-203', '015040199119091000KP', '4010103002', 'Bangunan Dihapus', 2, NULL, NULL, NULL, NULL, NULL, NULL, 50.00, NULL, '1', 100.00, NULL, NULL, 'N', NULL, NULL, 1, NULL, NULL, 20250101, 1.00, 'x', 'u'),
 -- rumah negara dan mess (dua snapshot)
 (300, 'REG-R-300', '015100199100001000KP', '4010201001', 'Rumah Negara A',   1, 'Jl. RN 300', '05/06', 'KelR', 'KecR', 'KotaR', 'ProvR', 120.00, NULL, '1', 700000000.00, '-6.4', '106.7', 'Y', NULL, NULL, 3, NULL, NULL, 20250101, 1.00, 'x', 'u'),
 (301, 'REG-M-301', '015100199100001000KP', '4010202016', 'Mess Rumah Negara', 1, 'Jl. Mess 301 (lama)', NULL, NULL, NULL, NULL, NULL, 80.00, NULL, '2', 400000000.00, '-6.5', '106.6', 'Y', NULL, '01', 3, NULL, NULL, 20240101, 1.00, 'x', 'u'),
 (301, 'REG-M-301', '015100199100001000KP', '4010202016', 'Mess Rumah Negara', 1, 'Jl. Mess 301',        NULL, NULL, NULL, NULL, NULL, 85.00, NULL, '2', 410000000.00, '-6.5', '106.6', 'Y', NULL, '01', 3, NULL, NULL, 20250101, 1.00, 'x', 'u'),
 -- rusunara (dua snapshot)
 (400, 'REG-U-400', '015100199100001000KP', '4010208001', 'Rusunara A', 7, 'Jl. Rusun 400 (lama)', NULL, NULL, NULL, NULL, NULL, 2000.00, NULL, '1', 3000000000.00, '-6.6', '106.5', 'Y', NULL, NULL, 3, NULL, NULL, 20240101, 1.00, 'x', 'u'),
 (400, 'REG-U-400', '015100199100001000KP', '4010208001', 'Rusunara A', 7, 'Jl. Rusun 400',        NULL, NULL, NULL, NULL, NULL, 2100.00, NULL, '1', 3100000000.00, '-6.6', '106.5', 'Y', NULL, NULL, 3, NULL, NULL, 20250101, 1.00, 'x', 'u'),
 -- kendaraan dinas (01 = KDJ, 02 = KDO) dan kendaraan non-015
 (500, 'REG-K-500', '015100199100001000KP', '3020101001', 'Sedan',  1, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '1', 300000000.00, NULL, NULL, 'Y', '01', NULL, 2, 'Camry',  NULL, 20250101, 1.00, 'x', 'u'),
 (501, 'REG-K-501', '015100199100001000KP', '3020101002', 'Jeep',   1, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '2', 400000000.00, NULL, NULL, 'Y', '02', NULL, 2, 'Fortuner', NULL, 20250101, 1.00, 'x', 'u'),
 (502, 'REG-K-502', '015100199100001000KP', '3020101001', 'Sedan',  2, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '1', 300000000.00, NULL, NULL, 'Y', '02', NULL, 2, 'Camry',  NULL, 20250101, 1.00, 'x', 'u'),
 (503, 'REG-K-503', '999990199100001000KP', '3020101001', 'Sedan Luar', 1, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '1', 300000000.00, NULL, NULL, 'Y', '01', NULL, 2, 'Camry', NULL, 20250101, 1.00, 'x', 'u'),
 -- aset di luar kelompok yang dibutuhkan (jalan) dan aset tak dikenal satkernya
 (600, 'REG-J-600', '015100199100001000KP', '5010101001', 'Jalan', 1, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '1', 1.00, NULL, NULL, 'Y', NULL, NULL, 1, NULL, NULL, 20250101, 1.00, 'x', 'u');

CREATE TABLE DJKN.SIMAN2_M_ASET_TANAH_BANGUNAN (id_aset_tanah BIGINT NULL, id_aset_bangunan BIGINT NULL, status_data INT NULL, catatan VARCHAR(100) NULL);
INSERT DJKN.SIMAN2_M_ASET_TANAH_BANGUNAN VALUES
 (102, 200, 1, 'tanah non-015 untuk gedung kantor'),
 (101, 202, 1, 'bangunan status Y'),
 (101, 203, 1, 'bangunan status N'),
 (100, 201, 1, 'tanah 015 dengan gedung lainnya'),
 (9998, 9997, 1, 'tautan yang tidak ada hubungannya');

CREATE TABLE DJKN.SIMAN2_M_ASET_PHOTO (
    id_photo INT IDENTITY(1, 1) NOT NULL, id_aset BIGINT NULL, folder VARCHAR(200) NULL, filename VARCHAR(200) NULL, nm_photo VARCHAR(200) NULL,
    photo_utama_yn CHAR(1) NULL, tanggal DATETIME NULL, status_data INT NULL, TGL_TARIK INT NULL, ukuran INT NULL);
INSERT DJKN.SIMAN2_M_ASET_PHOTO (id_aset, folder, filename, nm_photo, photo_utama_yn, tanggal, status_data, TGL_TARIK, ukuran) VALUES
 (100, '/foto/100/a.jpg', NULL, NULL, 'Y', '2025-01-01', 1, 20250101, 10),
 (100, '/foto/100/b.jpg', NULL, NULL, 'N', '2025-02-01', 1, 20250101, 10),
 (100, '/foto/100/hapus.jpg', NULL, NULL, 'Y', '2025-03-01', 0, 20250101, 10),
 (200, '', 'kantor.jpg', NULL, 'Y', '2025-01-01', 1, 20250101, 10),
 (300, NULL, NULL, 'rn300.jpg', 'Y', '2025-01-01', 1, 20250101, 10),
 (301, '/foto/301/m.jpg', NULL, NULL, 'Y', '2025-01-01', 1, 20250101, 10),
 (400, '/foto/400/r.jpg', NULL, NULL, 'Y', '2025-01-01', 1, 20250101, 10),
 (9999, '/foto/9999/x.jpg', NULL, NULL, 'Y', '2025-01-01', 1, 20250101, 10);

CREATE TABLE DJKN.SIMAN2_T_INV_BANGUNAN (
    id_inv_bangunan INT NOT NULL, id_aset BIGINT NULL, alamat VARCHAR(500) NULL, kd_rtrw VARCHAR(20) NULL, ur_kel VARCHAR(100) NULL, ur_kec VARCHAR(100) NULL,
    ur_kab VARCHAR(100) NULL, ur_prov VARCHAR(100) NULL, luas_bdg DECIMAL(18,2) NULL, TGL_TARIK INT NULL, updated_at DATETIME NULL, jml_lantai INT NULL);
INSERT DJKN.SIMAN2_T_INV_BANGUNAN VALUES
 (1, 400, 'Jl. Inventaris Rusun', '07/08', 'KelI', 'KecI', 'KotaI', 'ProvI', 1800.00, 20250101, '2025-01-05', 5),
 (2, 400, 'Jl. Inventaris Lama',  NULL,    NULL,   NULL,   NULL,    NULL,    1700.00, 20240101, '2024-01-05', 5),
 (3, 301, NULL, NULL, NULL, NULL, NULL, NULL, 90.00, 20250101, '2025-01-05', 1),
 (4, 100, 'Jl. Tanah (bukan gedung)', NULL, NULL, NULL, NULL, NULL, 1.00, 20250101, '2025-01-05', 1),
 (5, 9999, 'Tak terkait', NULL, NULL, NULL, NULL, NULL, 1.00, 20250101, '2025-01-05', 1);

CREATE TABLE DJKN.SIMAN2_PENGELOLAAN (id_pengelolaan INT NOT NULL, kode_register VARCHAR(60) NULL, is_asuransi VARCHAR(5) NULL, tgl_sk DATETIME NULL, no_sk VARCHAR(50) NULL);
INSERT DJKN.SIMAN2_PENGELOLAAN VALUES
 (1, 'REG-B-200', 'N', '2023-01-01', 'SK-1'),
 (2, 'REG-B-200', 'Y', '2024-01-01', 'SK-2'),
 (3, 'REG-B-201', '0', '2024-01-01', 'SK-3'),
 (4, 'REG-T-100', 'Y', '2024-01-01', 'SK-4'),
 (5, 'REG-X-999', 'Y', '2024-01-01', 'SK-5');

CREATE TABLE DJKN.SIMAN2_M_ASET_PEMAKAI (id_aset_pemakai INT NOT NULL, id_aset BIGINT NULL, jns_pemakai VARCHAR(50) NULL, nm_pmk VARCHAR(200) NULL, tgl_mulai DATETIME NULL, nik VARCHAR(20) NULL);
INSERT DJKN.SIMAN2_M_ASET_PEMAKAI VALUES
 (1, 300, 'PEGAWAI', 'Penghuni Fiktif Lama', '2020-01-01', '0000000000000001'),
 (2, 300, 'PEGAWAI', 'Penghuni Fiktif Baru', '2024-01-01', '0000000000000002'),
 (3, 301, 'PEGAWAI', 'Penghuni Mess Fiktif', '2024-01-01', '0000000000000003'),
 (4, 100, 'PEGAWAI', 'Pemakai Tanah Fiktif', '2024-01-01', '0000000000000004');

CREATE TABLE DJKN.SIMAN2_M_ASET_HUKUM (
    id_aset_hukum INT NOT NULL, id_aset BIGINT NULL, kd_status_hukum VARCHAR(5) NULL, terakhir_yn CHAR(1) NULL, status_data INT NULL, TGL_TARIK INT NULL, updated_at DATETIME NULL, no_dokumen VARCHAR(50) NULL);
INSERT DJKN.SIMAN2_M_ASET_HUKUM VALUES
 (1, 301, '02', 'N', 1, 20240101, '2024-01-01', 'D1'),
 (2, 301, '01', 'Y', 1, 20250101, '2025-01-01', 'D2'),
 (3, 301, '09', 'N', 0, 20250101, '2025-01-01', 'D3'),
 (4, 100, '01', 'Y', 1, 20250101, '2025-01-01', 'D4');

-- Di SLDK asli ini VIEW; sengaja dibuat VIEW di sini juga.
CREATE TABLE DJKN.SIMAN2_RUANG_DASAR (KODE_SATKER VARCHAR(21) NULL, KODE_BARANG VARCHAR(10) NULL, NUP INT NULL, tipe_ruang VARCHAR(50) NULL, kode_ruang VARCHAR(10) NULL, luas_ruang DECIMAL(10,2) NULL);
INSERT DJKN.SIMAN2_RUANG_DASAR VALUES
 ('015100199100001000KP', '4010208001', 7, 'Ruang Tidur', 'E1', 12), ('015100199100001000KP', '4010208001', 7, 'RUANG TIDUR', 'e2', 12),
 ('015100199100001000KP', '4010208001', 7, 'Ruang Tidur', 'D1', 12), ('015100199100001000KP', '4010208001', 7, 'Ruang Tidur', 'C1', 12),
 ('015100199100001000KP', '4010208001', 7, 'Dapur', 'E9', 8),
 ('015100199100001000KP', '4010202016', 1, 'Ruang Tidur', 'E1', 12), ('015100199100001000KP', '4010202016', 1, 'Ruang Tidur', 'D1', 12),
 ('015100199100001000KP', '4010208001', 99, 'Ruang Tidur', 'E1', 12),
 ('015040199119091000KP', '4010101001', 1, 'Ruang Kerja', 'A1', 20);
GO
CREATE VIEW DJKN.VW_SIMAN_RUANG_GEDUNG_BANGUNAN_015 AS
    SELECT KODE_SATKER, KODE_BARANG, NUP, tipe_ruang, kode_ruang, luas_ruang FROM DJKN.SIMAN2_RUANG_DASAR;
GO
