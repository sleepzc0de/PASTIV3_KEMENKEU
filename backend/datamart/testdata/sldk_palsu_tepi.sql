-- Kasus tepi tambahan untuk SLDK palsu (dijalankan SESUDAH sldk_palsu.sql): data yang paling mungkin membuat konversi datamart berbeda dari aplikasi.
-- Dipakai tes yang membandingkan tabel DIGITALISASI_* hasil script datamart dengan hasil digitalisasi.Sync (aplikasi) pada SLDK yang sama.
SET NOCOUNT ON;

-- Perlebar kolom sumber supaya pemotongan ke panjang kolom tujuan (NVARCHAR(500), (200), (100)) benar-benar terjadi.
ALTER TABLE DJKN.SIMAN2_M_ASET ALTER COLUMN ur_sskel NVARCHAR(MAX) NULL;
ALTER TABLE DJKN.SIMAN2_M_ASET ALTER COLUMN alamat NVARCHAR(MAX) NULL;
ALTER TABLE DJKN.SIMAN2_M_ASET ALTER COLUMN merk NVARCHAR(MAX) NULL;
ALTER TABLE DJKN.SIMAN2_M_ASET ALTER COLUMN nm_unit_pengguna NVARCHAR(MAX) NULL;
ALTER TABLE DJKN.SIMAN2_R_KONDISI ALTER COLUMN ur_kondisi NVARCHAR(400) NULL;
ALTER TABLE DJKN.SIMAN2_R_SATKER ALTER COLUMN ur_satker NVARCHAR(MAX) NULL;
ALTER TABLE DJKN.SIMAN2_R_SATKER ALTER COLUMN alamat_satker NVARCHAR(MAX) NULL;
ALTER TABLE DJKN.SIMAN2_M_ASET_PEMAKAI ALTER COLUMN nm_pmk NVARCHAR(MAX) NULL;
GO

DECLARE @panjang NVARCHAR(MAX) = REPLICATE(CAST(N'x' AS NVARCHAR(MAX)), 700);
-- 499 huruf lalu satu emoji (pasangan pengganti U+D83D U+DE00) tepat di batas 500: pemotongan tidak boleh memecah pasangan itu.
DECLARE @emoji NVARCHAR(MAX) = REPLICATE(CAST(N'a' AS NVARCHAR(MAX)), 499) + NCHAR(55357) + NCHAR(56832) + REPLICATE(CAST(N'b' AS NVARCHAR(MAX)), 50);
-- spasi aneka jenis: tab, NBSP, NEL (U+0085), em space (U+2003), CR LF (semua dipangkas Go), dan U+200B (BUKAN spasi menurut Go: tetap).
DECLARE @kotor NVARCHAR(MAX) = NCHAR(9) + NCHAR(160) + N'  Jl. Tepi  ' + NCHAR(13) + NCHAR(10) + NCHAR(133) + NCHAR(8195);

INSERT DJKN.SIMAN2_R_KONDISI VALUES ('4', REPLICATE(CAST(N'K' AS NVARCHAR(MAX)), 150), 4);   -- kondisi 150 huruf: dipotong ke 100

INSERT DJKN.SIMAN2_R_SATKER (id_satker, kd_satker, ur_satker, alamat_satker, nm_kel, nm_kec, nm_kab_kota, nm_prov, status_uakpb, updated_at, email_satker) VALUES
 (20, '015100199100003000KP', NCHAR(160) + N'  Satker Spasi  ' + NCHAR(9), @panjang, N' ', N'', N'Kota Tepi', NCHAR(8195) + N'Prov Tepi', 'AKTIF', '2025-01-02', 'e@uji.test');

-- tanah: teks kotor dan panjang, kondisi panjang, nilai besar, dan koordinat yang beraneka rupa
INSERT DJKN.SIMAN2_M_ASET (id_aset, kode_register, kd_satker, kd_brg, ur_sskel, no_aset, alamat, kd_rtrw, ur_kel, luas, luas_tnhb, kd_kondisi, rph_aset, gps_latitude, gps_longitude, status_bmn_yn, TGL_TARIK) VALUES
 (110, 'REG-T-110', '015100199100001000KP', '2010101001', @emoji, 110, @kotor, N'   ', N'', 1.50, 0.25, '1', 1000.00, '  -6,2  ', '106,8', 'Y', 20250101),
 (121, 'REG-T-121', '015100199100001000KP', '2010101001', N'Tanah kondisi panjang', 121, N'Jl. 121', NULL, NULL, 0, 0, '4', 9999999999999.99, '-6.5', '106.5', 'Y', 20250101),
 (122, 'REG-T-122', '015100199100003000KP', '2010101001', NCHAR(8203) + N'ZWSP depan', 122, @panjang, NULL, NULL, 12345.6789, 0.0001, '2', 123456789012.34, NULL, NULL, 'Y', 20250101);

INSERT DJKN.SIMAN2_M_ASET (id_aset, kode_register, kd_satker, kd_brg, ur_sskel, no_aset, luas, kd_kondisi, rph_aset, gps_latitude, gps_longitude, status_bmn_yn, TGL_TARIK)
SELECT v.id, N'REG-T-' + CAST(v.id AS NVARCHAR(10)), '015100199100001000KP', '2010101001', N'Tanah koordinat ' + CAST(v.id AS NVARCHAR(10)), v.id, 10.5, '1', 1000.00, v.lat, v.lng, 'Y', 20250101
FROM (VALUES
    (111, '-91',          '106'),            -- lintang di luar rentang: keduanya dibuang
    (112, '0',            '0'),              -- (0,0) bukan lokasi
    (113, NULL,           '106'),            -- tidak berpasangan
    (114, 'abc',          '106.5'),          -- bukan angka
    (115, '1e1',          '2E1'),            -- eksponen: 10 dan 20
    (116, '-6.123456789', '106.123456789'),  -- dibulatkan ke 7 desimal
    (117, '91',           '181'),            -- keduanya melampaui batas
    (118, '90',           '180'),            -- tepat di batas: sah
    (119, ' ',            ' '),              -- hanya spasi: kosong
    (120, '1,234.5',      '10'),             -- koma dan titik bersamaan: bukan angka
    (123, '1d5',          '10'),             -- eksponen d: diterima SQL Server tetapi ditolak Go
    (124, '-0',           '0.0'),            -- nol berbentuk lain: tetap (0,0)
    (125, '+7,5',         '+110,25')         -- tanda plus dan koma desimal
) AS v(id, lat, lng);

-- gedung kantor utama dan gedung lainnya dengan teks panjang/kotor
INSERT DJKN.SIMAN2_M_ASET (id_aset, kode_register, kd_satker, kd_brg, ur_sskel, no_aset, alamat, luas, kd_kondisi, rph_aset, merk, nm_unit_pengguna, status_bmn_yn, TGL_TARIK) VALUES
 (220, 'REG-B-220', '015040199119091000KP', '4010101001', @emoji, 220, @kotor, 99.9, '1', 5000000000.50, NULL, NULL, 'Y', 20250101),
 (230, 'REG-B-230', '015100199100001000KP', '4010103001', N'Gudang tepi', 230, N'Jl. 230', 10, '3', 100.00, @panjang, NCHAR(160) + N' Unit  Kotor ' + NCHAR(9), 'Y', 20250101);

-- rumah negara dengan penghuni bernama kotor
INSERT DJKN.SIMAN2_M_ASET (id_aset, kode_register, kd_satker, kd_brg, ur_sskel, no_aset, alamat, luas, kd_kondisi, rph_aset, gps_latitude, gps_longitude, status_bmn_yn, kd_jns_bmn, TGL_TARIK) VALUES
 (310, 'REG-R-310', '015100199100001000KP', '4010201002', N'Rumah Negara tepi', 310, N'Jl. RN 310', 70, '1', 300000000.00, '-6,7', '106,4', 'Y', 3, 20250101);
INSERT DJKN.SIMAN2_M_ASET_PEMAKAI (id_aset_pemakai, id_aset, jns_pemakai, nm_pmk, tgl_mulai, nik) VALUES
 (10, 310, N'  PEGAWAI ', NCHAR(160) + N' Budi Tepi ' + NCHAR(9), '2025-01-01', '0000000000000010');

-- Beberapa rusunara dan mess dengan satker dan NUP yang tidak berurutan menurut id, supaya ORDER BY query (satker, NUP) terlihat pada urutan id tabel tujuan.
-- Salinan baris 400 (rusunara) dan 301 (mess) versi terbaru dengan id, NUP, dan satker lain.
INSERT DJKN.SIMAN2_M_ASET
SELECT v.id, N'REG-U-' + CAST(v.id AS NVARCHAR(10)), v.satker, a.kd_brg, N'Rusunara urut ' + CAST(v.id AS NVARCHAR(10)), v.nup, a.alamat, a.kd_rtrw, a.ur_kel, a.ur_kec, a.ur_kab, a.ur_prov, a.luas, a.luas_tnhb, a.kd_kondisi,
       a.rph_aset, a.gps_latitude, a.gps_longitude, a.status_bmn_yn, a.kd_status, a.kd_status_hukum, a.kd_jns_bmn, a.merk, a.nm_unit_pengguna, a.TGL_TARIK, a.nilai_buku, a.keterangan, a.dibuat_oleh
FROM DJKN.SIMAN2_M_ASET AS a
CROSS JOIN (VALUES (401, '015100199100003000KP', 2), (402, '015100199100001000KP', 3), (403, '015100199100001000KP', 1), (404, '015100199100003000KP', 11)) AS v(id, satker, nup)
WHERE a.id_aset = 400 AND a.TGL_TARIK = 20250101;

INSERT DJKN.SIMAN2_M_ASET
SELECT v.id, N'REG-M-' + CAST(v.id AS NVARCHAR(10)), v.satker, a.kd_brg, N'Mess urut ' + CAST(v.id AS NVARCHAR(10)), v.nup, a.alamat, a.kd_rtrw, a.ur_kel, a.ur_kec, a.ur_kab, a.ur_prov, a.luas, a.luas_tnhb, a.kd_kondisi,
       a.rph_aset, a.gps_latitude, a.gps_longitude, a.status_bmn_yn, a.kd_status, a.kd_status_hukum, a.kd_jns_bmn, a.merk, a.nm_unit_pengguna, a.TGL_TARIK, a.nilai_buku, a.keterangan, a.dibuat_oleh
FROM DJKN.SIMAN2_M_ASET AS a
CROSS JOIN (VALUES (302, '015100199100003000KP', 4), (303, '015100199100001000KP', 9), (304, '015100199100001000KP', 2)) AS v(id, satker, nup)
WHERE a.id_aset = 301 AND a.TGL_TARIK = 20250101;
GO
