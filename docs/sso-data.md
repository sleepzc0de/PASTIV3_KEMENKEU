# Data SSO Kemenkeu yang disimpan

Setiap login lewat SSO Kemenkeu menyimpan data pegawai selengkap yang dikirim SSO, supaya bisa dipakai kelak (mis. pembatasan data per satker,
peran, atau laporan kepegawaian) tanpa harus meminta ulang ke SSO. Migrasi: `051_sso_data_lengkap.sql`. Kode: `backend/handlers/sso_data.go`.

## Apa yang disimpan

| Tabel / kolom | Isi |
|---|---|
| `employees` (migrasi 002) | Satu baris per pegawai (kunci `sso_sub`): klaim yang punya kolom sendiri (`nip`, `nip9`, `nik`, `name`, `email`, `jabatan`, `satker`, `kode_satker`, `organisasi`, `kode_organisasi`, `kode_kl`, `nama_kl`, ...) dan `raw_claims` = JSON klaim userinfo terakhir. |
| `employees.raw_id_token_claims` | JSON klaim di dalam `id_token` terakhir (bisa memuat klaim yang tidak ada di userinfo). |
| `employees.first_login_at`, `last_login_at`, `login_count` | Login pertama, terakhir, dan jumlah login lewat SSO. |
| `employees.claims_hash` | SHA-256 klaim terakhir (tanpa klaim yang berganti tiap login: `iat`, `exp`, `nonce`, ...). |
| `employee_claims` | **Satu baris per klaim**: `(employee_id, sumber, claim_key, claim_value, tipe)`. `sumber` = `userinfo` atau `id_token`; `tipe` = `string`, `number`, `bool`, `json` (larik/objek), atau `null`. Klaim apa pun, termasuk yang baru ditambah Kemenkeu kelak, langsung bisa dicari tanpa menambah kolom. Diperbarui tiap login (klaim yang tidak dikirim lagi dihapus). |
| `employee_claim_history` | Salinan **lengkap** klaim (userinfo, id_token, scope) setiap kali isinya berubah, mis. mutasi atau jabatan baru. Login tanpa perubahan tidak menambah baris. Baris awal (`claims_hash` NULL) = salinan `raw_claims` yang sudah ada saat migrasi dijalankan. |
| `sso_tokens` (migrasi 004) | Access/refresh token SSO, **terenkripsi**, untuk memanggil HRIS2 atas nama pengguna. Tidak diubah. |

`id_token` hanya dibaca isinya (tanpa memeriksa tanda tangan, sah karena diterima langsung dari token endpoint lewat TLS) dan disimpan sebagai data;
identitas pengguna tetap diambil dari userinfo. Token mentahnya tidak disimpan.

Penyimpanan ini **tidak menggagalkan login**: bila gagal (mis. migrasi belum dijalankan), login tetap berhasil dan galatnya tercatat di log
(`[SSO WARN] gagal menyimpan data SSO lengkap`).

## Contoh pemakaian

```sql
-- Klaim apa saja yang pernah dikirim SSO, dan berapa pegawai yang memilikinya
SELECT sumber, claim_key, tipe, COUNT(*) AS pegawai FROM employee_claims GROUP BY sumber, claim_key, tipe ORDER BY sumber, claim_key;

-- Satu klaim untuk semua pegawai (mis. kode satker)
SELECT e.nip, e.name, c.claim_value AS kode_satker
FROM employees e JOIN employee_claims c ON c.employee_id = e.id AND c.sumber = N'userinfo' AND c.claim_key = N'kode_satker';

-- Riwayat perubahan seorang pegawai
SELECT direkam_pada, userinfo_claims FROM employee_claim_history WHERE employee_id = (SELECT id FROM employees WHERE nip = N'...') ORDER BY id;
```

Kolom `kode_satker` di `employees` dipakai nanti untuk menghubungkan pengguna dengan satkernya: kode satker 6 digit pada data aset dan Inaproc
(lihat [referensi-ue1-dan-satker.md](referensi-ue1-dan-satker.md)). **Format klaim `kode_satker` dari SSO belum diverifikasi**: periksa dengan
query pertama di atas setelah beberapa orang login, lalu sesuaikan cara mencocokkannya.

## Data pribadi

`employees` memuat NIK, nomor telepon, dan klaim lain yang pribadi sifatnya. Batasi akses langsung ke tabelnya, jangan tampilkan di antarmuka
tanpa kebutuhan jelas, dan ikut sertakan tabel `employees`, `employee_claims`, dan `employee_claim_history` dalam kebijakan pencadangan dan retensi data.

## Pengujian

`go test ./handlers -run 'SSO|Klaim|Migrasi051'`. Tes integrasi (`sso_data_integrasi_test.go`) berjalan bila `PASTI_UJI_MSSQL_DSN` diisi dan memeriksa
penyimpanan per klaim (tipe, angka besar, larik, objek, null), riwayat hanya saat data berubah, klaim yang hilang, id_token yang tidak terbaca, dan
penyalinan awal dari `raw_claims` oleh migrasi.
