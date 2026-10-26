-- ============================================================
-- Migration 063: riwayat sinkronisasi Digitalisasi Aset mencatat SUMBER datanya
-- Aman dijalankan berulang kali (idempotent)
--
-- Sinkronisasi pernah membaca SLDK dengan datamart sebagai cadangan (kini hanya datamart: migrasi 064 dan docs/datamart-digitalisasi.md). Kolom sumber mencatat dari mana data satu sinkronisasi dibaca:
-- 'sldk' atau 'datamart'. NULL pada riwayat lama = SLDK (sebelum ada datamart). Penjadwal memakainya untuk mencoba ulang lebih cepat sinkronisasi yang berasal dari datamart.
-- ============================================================

IF NOT EXISTS (SELECT * FROM sys.columns WHERE object_id = OBJECT_ID('digitalisasi_sync_log') AND name = 'sumber')
    ALTER TABLE digitalisasi_sync_log ADD sumber NVARCHAR(10) NULL;   -- sldk | datamart
