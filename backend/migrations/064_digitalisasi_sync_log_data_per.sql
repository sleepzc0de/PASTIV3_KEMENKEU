-- ============================================================
-- Migration 064: riwayat sinkronisasi Digitalisasi Aset mencatat "data per" kapan
-- Aman dijalankan berulang kali (idempotent)
--
-- Sinkronisasi menyalin tabel DIGITALISASI_* dari datamart (docs/datamart-digitalisasi.md). Kolom data_per mencatat awal penyegaran datamart (UTC) yang menghasilkan data
-- sinkronisasi sukses itu. Sinkronisasi berikutnya menolak datamart yang LEBIH LAMA daripada data_per terakhir supaya data PASTI tidak mundur (mis. datamart dipulihkan dari
-- cadangan lama), dan halaman Sinkronisasi menampilkan "data per". NULL pada riwayat lama dan pada sinkronisasi yang tidak sukses.
-- Kolom sumber (migrasi 063) kini selalu 'datamart' untuk sinkronisasi baru; NULL atau 'sldk' pada riwayat lama, dari masa aplikasi masih membaca SLDK langsung.
-- ============================================================

IF NOT EXISTS (SELECT * FROM sys.columns WHERE object_id = OBJECT_ID('digitalisasi_sync_log') AND name = 'data_per')
    ALTER TABLE digitalisasi_sync_log ADD data_per DATETIME2(3) NULL;
