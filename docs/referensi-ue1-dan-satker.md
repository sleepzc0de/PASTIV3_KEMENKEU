# Referensi UE1, referensi Kanwil, dan keterhubungan satker (aset dan pengadaan)

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

## Referensi Kantor Wilayah (Kanwil)

Satu daftar kode Kanwil (**9 karakter pertama kode satker** = KL 3 + UE1 2 + wilayah 4, mis. `015040199`) -> uraian dan singkatan, tabel `ref_kanwil` (migrasi `056_create_ref_kanwil.sql`).
Dikelola superadmin di **Administrasi > Referensi Kanwil** (`/dashboard/referensi/kanwil`); pengguna lain yang punya peran dapat membaca daftarnya. Tidak ada data awal.

Isinya dari tiga jalur; kolom `sumber` mencatat asalnya:

| Sumber | Asal | Boleh ditimpa penarikan SLDK? |
|---|---|---|
| `satker` | **Nama satker pada data Digitalisasi Aset** (`DIGITALISASI_SATKER`) yang kode satkernya berawalan kode Kanwil itu | Ya (SLDK lebih otoritatif) |
| `sldk` | **Ditarik dari SLDK**: `DJKN.SIMAN2_R_KORWIL` (`kd_wileselon` -> `ur_korwil`), hanya KL 015 | Ya (diperbarui bila berubah) |
| `manual` | Diisi atau diubah superadmin (menyimpan perubahan apa pun menjadikan barisnya `manual`) | **Tidak pernah** |

- **Kode yang belum punya referensi** dilaporkan di halaman ("N kode Kanwil di data aset belum punya referensi", menurut cakupan peran pembaca) beserta jumlah satkernya dan **saran uraian**:
  satker pada kode itu yang namanya menyebut *kantor wilayah*, *kanwil*, atau *kantor pusat* (induk satker lebih dulu, lalu nama terpendek). Ini **tebakan dari nama**; kode tanpa satker semacam itu tidak diberi saran.
  Tombol **Tambahkan** per kode (uraian terisi saran, bisa diubah) dan **Tambahkan N yang punya saran** (`POST /referensi/kanwil/dari-satker`) memasukkan semua yang punya saran sebagai sumber `satker`.
- **Tarik dari SLDK** (`POST /referensi/kanwil/tarik-sldk`, superadmin): membaca `DJKN.SIMAN2_R_KORWIL` (sekitar 17 ribu baris untuk semua KL; hanya `kd_wileselon LIKE '015%'` yang dibaca, jadi ringan dan langsung dalam satu
  permintaan), menghapus duplikat per kode (baris dengan `updated_at`/`id_korwil` terbaru), lalu dalam **satu transaksi**: menambahkan kode baru, memperbarui baris `satker`/`sldk` yang berbeda, melewati baris `manual`.
  Hasilnya dilaporkan (dibaca, ditambahkan, diperbarui, tanpa perubahan, dilewati karena manual, tidak sah, dipotong). SLDK yang tidak mengembalikan baris tidak mengubah apa pun; hanya satu penarikan dalam satu waktu.
  Kolom pribadi pada tabel itu (`nip`, `nama`, `jabatan`, email, telepon) **tidak dibaca**. `status_korwil` disimpan apa adanya di `status_sldk` sebagai keterangan dan **tidak** mengubah `aktif`.
- Kode Kanwil harus 9 digit angka; uraian wajib (maksimal 200 karakter), singkatan opsional (maksimal 30), urutan 0-9999 (bawaan 100), `aktif` bawaan true. Menghapus referensi tidak mengubah data aset atau peran pengguna.

Di mana referensi dipakai: **Manajemen Pengguna** (keterangan pada lencana peran Kanwil, uraian pada peran yang dimiliki dan saran, serta isian kode peran Kanwil dengan daftar usulan dan nama yang terbaca), dan **rincian rekaman Digitalisasi Aset**
(baris "Kanwil <kode> · <uraian>" di bawah Kode Satker). Kode yang belum terdaftar tampil sebagai `Kanwil <kode>`.
Referensi aktif juga menjadi **saran baris tembusan "Kepala Kantor Wilayah ..." pada Nota Dinas SAPA** (uraian huruf besar diubah ke huruf judul; lihat [sapa.md](sapa.md#tembusan-kanwil-pada-nota-dinas)).

API: `GET /api/v1/referensi/kanwil` (semua pengguna yang punya peran; memuat `daftar`, `belum_terdaftar`, dan `sldk_tersedia`), `PUT|DELETE /api/v1/referensi/kanwil/:kode`, `POST .../dari-satker`, `POST .../tarik-sldk` (superadmin).

### Belum diverifikasi terhadap data asli

- **Tabel sumber SLDK**: dipilih `SIMAN2_R_KORWIL` karena kolom `kd_wileselon` (9 karakter) = `kd_eselonkl_d` (KL+UE1, 5) + `kd_korwil` (4) cocok dengan 9 karakter pertama kode satker (UAPB 3 + UAPPB-E1 2 + UAPPB-W 4 pada `SIMAN2_R_SATKER`).
  Tabel `SIMAN2_R_KANWIL` (17 baris) adalah Kanwil DJKN (`kd_kleselonkanwil` 8 karakter), konsep yang berbeda, jadi tidak dipakai. Belum dijalankan ke SLDK asli (hanya diuji dengan tabel tiruan di SQL Server uji).
  Bila hasil penarikan terlihat salah, bandingkan beberapa kode pada `SIMAN2_R_SATKER` (`uapb`+`uappbe1`+`uappbw`) dengan `kd_wileselon`, lalu sesuaikan `sqlKanwilSLDK` di `backend/handlers/ref_kanwil.go`.
- **Saran uraian dari nama satker** bergantung pada kata kunci di `kataKunciSaranKanwil`; sesuaikan bila nama kantor wilayah pada data asli memakai istilah lain.

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