-- ============================================================
-- Migration 061: API Akses Data hanya dapat didaftarkan oleh superadmin (formulir pendaftaran publik ditiadakan)
-- Aman dijalankan berulang kali (idempotent)
--
-- Formulir publik, captcha pendaftaran, dan pengaturan "pendaftaran_dibuka" dihapus dari aplikasi. Migrasi 060 tidak diubah (sudah diterapkan di server); migrasi ini hanya
-- membersihkan baris pengaturan yang tidak dipakai lagi. Kolom dan status lama (menunggu, ditolak) dibiarkan: pendaftaran berstatus menunggu dari formulir lama, bila ada, tetap dapat
-- diputuskan (setujui atau tolak) oleh superadmin, dan tidak ada kredensial yang pernah terbit untuknya. Klien baru hanya dibuat lewat POST /api/v1/api-akses/klien (superadmin)
-- dan langsung berstatus disetujui.
-- ============================================================

IF EXISTS (SELECT * FROM sys.tables WHERE name = 'api_akses_pengaturan')
    DELETE FROM api_akses_pengaturan WHERE kunci = N'pendaftaran_dibuka';
