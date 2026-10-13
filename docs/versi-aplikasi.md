# Versi Aplikasi

Versi aplikasi tampil di dua tempat:

- **Footer** semua halaman setelah login: `© 2026 PASTI V3 · Pemantauan Aset Terintegrasi · Versi 3.0.0`
- **Halaman login**, tepat di bawah tombol **Masuk**: `PASTI V3 · Versi 3.0.0`

## Aturan penomoran

Bentuknya selalu tiga angka: **3.X.Y**. Angka pertama `3` adalah generasi aplikasi (PASTI V3) dan tidak berubah.

| Jenis pembaruan | Contoh | Perintah |
|---|---|---|
| **Kecil**: perbaikan galat, penyesuaian tampilan, perubahan teks, perbaikan skrip | 3.0.0 → **3.0.1** → 3.0.2 | `npm version patch --no-git-tag-version` |
| **Besar**: fitur atau modul baru, perubahan alur yang terasa oleh pengguna, migrasi database yang penting | 3.0.2 → **3.1.0** → 3.2.0 | `npm version minor --no-git-tag-version` |

Setelah pembaruan besar, angka terakhir kembali ke 0 (3.1.0), lalu pembaruan kecil berikutnya menjadi 3.1.1.

Versi awal (rilis ini) adalah **3.0.0**, yaitu "versi 3.0".

## Cara menaikkan versi

Jalankan di folder `frontend` **sebelum commit** pembaruan, lalu sertakan perubahannya dalam commit yang sama:

```bash
cd frontend
npm version patch --no-git-tag-version
```

Perintah itu mengubah `frontend/package.json` dan `frontend/package-lock.json` sekaligus (jangan diubah dengan tangan; ada tes yang memeriksa keduanya sama). Opsi `--no-git-tag-version` mencegah npm membuat commit dan tag sendiri.

Versi dibaca dari `package.json` **saat build** (`next.config.ts` menyisipkannya ke bundel), jadi angkanya baru berubah di server setelah `./deploy.sh prod` membangun ulang frontend.

## Mengapa di package.json

Satu sumber saja: tidak ada angka ganda yang bisa saling bertentangan, dan `npm version` sudah menghasilkan tepat pola di atas. Tes `lib/versi.test.mjs` menjaga agar versi selalu berbentuk `X.Y.Z`, tetap berawalan `3`, dan sama di `package-lock.json`. Bila versinya hilang atau tidak sah, teks versi tidak ditampilkan sama sekali (bukan menampilkan yang salah).
