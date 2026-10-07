# Referensi UE1 dan keterhubungan satker (aset dan pengadaan)

## Referensi Unit Eselon I

Satu daftar kode UE1 (5 digit, mis. `01504`) -> uraian dan singkatan, tabel `ref_ue1` (migrasi `052_create_ref_ue1.sql`). Dikelola superadmin di
**Administrasi > Referensi UE1** (`/dashboard/referensi/ue1`): tambah, ubah uraian/singkatan/urutan, nonaktifkan, atau hapus. Semua pengguna yang punya peran bisa membacanya.

Data awal (14 UE1; hanya dimasukkan bila kodenya belum ada, jadi migrasi yang dijalankan ulang tidak menimpa perubahan superadmin):

| Kode | Singkatan | Uraian |
|---|---|---|
| 01501 | SETJEN | SEKRETARIAT JENDERAL |
| 01502 | ITJEN | INSPEKTORAT JENDERAL |
| 01503 | DJA | DIREKTORAT JENDERAL ANGGARAN |
| 01504 | DJP | DIREKTORAT JENDERAL PAJAK |
| 01505 | DJBC | DIREKTORAT JENDERAL BEA DAN CUKAI |
| 01506 | DJPK | DIREKTORAT JENDERAL PERIMBANGAN KEUANGAN |
| 01507 | DJPPR | DIREKTORAT JENDERAL PEMBIAYAAN DAN RISIKO |
| 01508 | DJPB | DIREKTORAT JENDERAL PERBENDAHARAAN |
| 01509 | DJKN | DIREKTORAT JENDERAL KEKAYAAN NEGARA |
| 01511 | BPPK | BADAN PENDIDIKAN DAN PELATIHAN KEUANGAN |
| 01512 | DJSEF | DIREKTORAT JENDERAL STRATEGI EKONOMI DAN FISKAL |
| 01513 | LNSW | LEMBAGA NATIONAL SINGLE WINDOW |
| 01514 | DJSPSK | DIREKTORAT JENDERAL STABILITAS DAN PENGEMBANGAN SEKTOR KEUANGAN |
| 01515 | BTIIK | BADAN TEKNOLOGI, INFORMASI DAN INTELIJEN KEUANGAN |

Kode `01510` tidak ada pada data awal. Kode UE1 yang ada di data satker Digitalisasi Aset tetapi belum punya referensi dilaporkan di halaman superadmin
("N kode UE1 di data aset belum punya referensi") dengan tombol Tambahkan, dan tampil sebagai `UE1 <kode>` sampai ditambahkan.

Di mana referensi dipakai:

- **Digitalisasi Aset**: label UE1 pada grafik, filter UE1 (Ringkasan, Peta, Data), rincian rekaman (kode + uraian), dan **dua kolom tambahan pada
  unduhan Excel/CSV** (`Singkatan UE1`, `Uraian UE1`, tepat setelah `Kode UE1`; dibaca dari `ref_ue1` saat diunduh). PDF ringkas tidak memuat kolom UE1.
- **Dashboard**: wawasan dan grafik aset memakai label yang sama; tab Satker menampilkan UE1 tiap satker.
- **SAPA**: nama UE1 dibaca dari referensi ini. `sapa_ref_ue1` tetap menyimpan **sebutan Sekretaris** (tujuan Nota Dinas); daftarnya di Pengaturan SAPA
  kini memuat semua UE1 aktif sehingga superadmin cukup mengisi sebutan yang masih kosong.

API: `GET /api/v1/referensi/ue1` (semua pengguna yang punya peran), `PUT /api/v1/referensi/ue1/:kode` dan `DELETE ...` (superadmin).

## Keterhubungan satker: aset dan pengadaan

Kode satker pada data aset (`DIGITALISASI_*.Kode_Satker`) berbentuk 20 karakter, mis. `015040199119091000KP`:

```
015  04  01  99  119091  000  KP
KL   UE1 ...     satker  anak  akhiran
```

Kode UE1 = 5 karakter pertama; **kode satker 6 digit = karakter ke-10 sampai ke-15**; karakter ke-16 sampai ke-18 menandai induk (`000`) atau anak. Kode 6 digit itu
yang sama dengan `kd_satker_str` pada data Inaproc, jadi keduanya dihubungkan lewat:

```sql
SUBSTRING(Kode_Satker, 10, 6)  =  kd_satker_str
```

- Satu kode 6 digit memuat satu induk dan anak-anak satker; semuanya dihitung sebagai satu satker (nama dan UE1 diambil dari induknya).
- Kode Inaproc yang seluruhnya angka dan kurang dari 6 digit dilengkapi nol di depan (`12345` -> `012345`), karena nol di depan sering hilang bila kode diperlakukan sebagai bilangan.
- Tidak ada kolom atau tabel baru: penghubungan dilakukan saat query, jadi data aset dan Inaproc tidak berubah.

Tampilan: **Dashboard > tab Satker** (`GET /api/v1/satker/keterhubungan?tahun=&kode_klpd=`, bawaan KLPD K10 dan tahun terbaru yang punya pengadaan). Menampilkan
jumlah satker pada data aset, satker pengadaan, **terhubung** (punya aset dan pengadaan), **hanya aset** (tidak punya pengadaan pada tahun itu), dan **hanya pengadaan**
(tidak ditemukan pada data aset: kode berbeda atau satker tanpa data BMN). Tabel dapat dicari (kode/nama/UE1), disaring (status, UE1), dan diurutkan; angka aset
menaut ke tab Data Digitalisasi (`?tab=data&dataset=...&q=<kode>`). Petunjuk di atas tabel muncul bila angkanya mencurigakan (mis. tidak ada satker yang cocok).

Pengadaan per satker dihitung seperti Dashboard Pengadaan: paket RUP penyedia yang masih berlaku (jumlah dan pagu), tender dan non-tender (pengumuman versi terbaru). Satu paket
bisa ada di ketiganya, jadi angka itu ukuran keaktifan satker, bukan total paket.

### Belum diverifikasi terhadap data asli

- Bahwa `kd_satker_str` Inaproc memang berisi kode satker 6 digit yang sama. Dasar pemeriksaannya ada di tampilan: bila **terhubung = 0** atau **hanya pengadaan** sangat banyak,
  bandingkan beberapa kode pada kedua sisi dan sesuaikan `kunciInaprocSQL` di `backend/handlers/satker_keterhubungan.go`.
- Bahwa karakter ke-10 sampai ke-15 selalu kode satker pada seluruh kode `Kode_Satker`. Baris yang kodenya kurang dari 15 karakter dihitung di "aset tanpa kode" dan tidak dihubungkan.

## Peran dan pembatasan data per satker

Kunci kode yang sama dipakai untuk peran data (UE1 = 5 digit pertama kode satker, Kanwil = 9 digit pertama, Satker = digit ke-10 sampai ke-15) yang membatasi data yang dilihat
pengguna. Lihat [peran-data.md](peran-data.md).