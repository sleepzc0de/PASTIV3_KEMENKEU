-- ============================================================
-- Migration 062: API Akses Data mencakup dataset pengadaan (selain aset)
-- Aman dijalankan berulang kali (idempotent)
--
-- Katalog dataset bertambah dengan 24 dataset pengadaan (kode "pengadaan.<dataset>"), jadi satu klien dapat memegang sampai 31 kode dataset. Kolom
-- api_akses_klien.dataset_diminta (daftar kode dipisah koma) sebelumnya 1.000 karakter: daftar penuh sudah mendekati batas itu dan bertambahnya dataset akan membuat
-- pendaftaran gagal dengan galat pemotongan. Dilebarkan ke 4.000 karakter (bukan MAX: kolom ini tidak pernah memuat lebih dari beberapa puluh kode).
-- Tabel api_akses_klien_dataset.dataset (60) dan api_akses_log.dataset (60) sudah cukup untuk kode terpanjang (40 karakter).
-- ============================================================

-- max_length dalam byte: NVARCHAR(1000) = 2000, NVARCHAR(4000) = 8000, NVARCHAR(MAX) = -1.
IF EXISTS (SELECT 1 FROM sys.columns WHERE object_id = OBJECT_ID('api_akses_klien') AND name = 'dataset_diminta' AND max_length > 0 AND max_length < 8000)
    ALTER TABLE api_akses_klien ALTER COLUMN dataset_diminta NVARCHAR(4000) NOT NULL;
