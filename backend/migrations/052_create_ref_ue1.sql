-- ============================================================
-- Migration 052: Referensi Unit Eselon I (UE1)
-- Aman dijalankan berulang kali (idempotent)
--
-- Satu daftar rujukan kode UE1 (5 digit, mis. 01504) -> uraian dan singkatan, dipakai Digitalisasi Aset, Dashboard, dan ekspor.
-- Dikelola admin lewat menu Administrasi > Referensi UE1, jadi bila ada perubahan organisasi cukup diubah di sana.
-- Data awal di bawah hanya DIMASUKKAN bila kodenya belum ada; perubahan admin tidak pernah ditimpa saat migrasi dijalankan ulang.
-- (Referensi SAPA, sapa_ref_ue1, tetap menyimpan sebutan Sekretaris untuk Nota Dinas; nama UE1 di SAPA kini dibaca dari tabel ini.)
-- ============================================================

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'ref_ue1')
BEGIN
    CREATE TABLE ref_ue1 (
        kode NVARCHAR(5) NOT NULL PRIMARY KEY,
        nama NVARCHAR(200) NOT NULL,
        singkatan NVARCHAR(30) NULL,
        urutan INT NOT NULL DEFAULT 0,
        aktif BIT NOT NULL DEFAULT 1,
        diubah_oleh NVARCHAR(100) NULL,
        diubah_pada DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        CONSTRAINT ck_ref_ue1_kode CHECK (LEN(kode) = 5 AND kode NOT LIKE '%[^0-9]%')
    );
END;

GO

INSERT INTO ref_ue1 (kode, nama, singkatan, urutan, diubah_oleh)
SELECT v.kode, v.nama, v.singkatan, v.urutan, N'data awal'
FROM (VALUES
    (N'01501', N'SEKRETARIAT JENDERAL', N'SETJEN', 1),
    (N'01502', N'INSPEKTORAT JENDERAL', N'ITJEN', 2),
    (N'01503', N'DIREKTORAT JENDERAL ANGGARAN', N'DJA', 3),
    (N'01504', N'DIREKTORAT JENDERAL PAJAK', N'DJP', 4),
    (N'01505', N'DIREKTORAT JENDERAL BEA DAN CUKAI', N'DJBC', 5),
    (N'01506', N'DIREKTORAT JENDERAL PERIMBANGAN KEUANGAN', N'DJPK', 6),
    (N'01507', N'DIREKTORAT JENDERAL PEMBIAYAAN DAN RISIKO', N'DJPPR', 7),
    (N'01508', N'DIREKTORAT JENDERAL PERBENDAHARAAN', N'DJPB', 8),
    (N'01509', N'DIREKTORAT JENDERAL KEKAYAAN NEGARA', N'DJKN', 9),
    (N'01511', N'BADAN PENDIDIKAN DAN PELATIHAN KEUANGAN', N'BPPK', 11),
    (N'01512', N'DIREKTORAT JENDERAL STRATEGI EKONOMI DAN FISKAL', N'DJSEF', 12),
    (N'01513', N'LEMBAGA NATIONAL SINGLE WINDOW', N'LNSW', 13),
    (N'01514', N'DIREKTORAT JENDERAL STABILITAS DAN PENGEMBANGAN SEKTOR KEUANGAN', N'DJSPSK', 14),
    (N'01515', N'BADAN TEKNOLOGI, INFORMASI DAN INTELIJEN KEUANGAN', N'BTIIK', 15)
) AS v(kode, nama, singkatan, urutan)
WHERE NOT EXISTS (SELECT 1 FROM ref_ue1 r WHERE r.kode = v.kode);

GO

-- Kode yang sudah terlanjur diisi admin di referensi SAPA tetapi tidak ada pada data awal ikut disalin (singkatan dikosongkan; admin dapat melengkapinya).
-- Tabel SAPA bisa belum ada bila migrasinya belum dijalankan, jadi dicek dulu dan disalin lewat SQL dinamis.
IF OBJECT_ID('dbo.sapa_ref_ue1', 'U') IS NOT NULL
    EXEC(N'INSERT INTO ref_ue1 (kode, nama, urutan, diubah_oleh)
           SELECT s.kode, s.nama, 100, N''disalin dari SAPA''
           FROM sapa_ref_ue1 s
           WHERE s.kode NOT LIKE ''%[^0-9]%'' AND LEN(s.kode) = 5
             AND NOT EXISTS (SELECT 1 FROM ref_ue1 r WHERE r.kode = s.kode)');
