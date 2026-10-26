# API Akses Data PASTI: panduan pemakaian untuk pihak luar

Panduan singkat bagi pengembang aplikasi yang ingin menarik data PASTI. Alamat dasar produksi: `https://pasti.kemenkeu.go.id/api/data/v1` (gunakan alamat PASTI yang diberikan administrator).

## 1. Mendaftar

Tidak ada pendaftaran mandiri: **hubungi administrator PASTI** (melalui unit Anda atau surat/nota dinas) untuk didaftarkan sebagai klien API. Siapkan informasi berikut:

- nama aplikasi, instansi dan unit pemilik;
- PIC (nama, jabatan, email, telepon);
- tujuan penggunaan (data apa dan untuk apa);
- dataset yang dibutuhkan (hanya yang benar-benar diperlukan);
- alamat IP keluar (egress) server Anda, bila ingin kunci dibatasi ke alamat itu (disarankan);
- perkiraan kebutuhan permintaan, bila lebih dari bawaan (lihat bagian 4).

Administrator PASTI yang mendaftarkan aplikasi Anda, menentukan dataset dan batas pemakaian, lalu menyerahkan **kunci API** kepada PIC lewat jalur yang aman. Kunci **hanya ditampilkan sekali** dan tidak
dapat dilihat lagi; bila hilang, minta kunci baru (yang lama dicabut). Setelah terdaftar, Anda menerima **ID klien** (`KL-XXXXXXXX`) untuk disebut saat berkomunikasi dengan administrator.

## 2. Autentikasi

Kirim kunci di header setiap permintaan (pilih salah satu):

```http
Authorization: Bearer pasti_xxxxxxxxxx_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
X-API-Key: pasti_xxxxxxxxxx_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

- **Jangan** menaruh kunci di alamat (URL/query string): alamat tercatat di log proxy dan riwayat peramban; kunci seperti itu ditolak.
- Simpan kunci sebagai rahasia (variabel lingkungan atau pengelola rahasia), jangan di kode sumber atau aplikasi peramban/seluler. Panggil API dari **server** Anda (tidak ada CORS untuk peramban).
- Bila kunci diduga bocor, segera minta pencabutan. Untuk pergantian rutin, administrator dapat merotasi kunci: kunci lama tetap berlaku sementara (bawaan 24 jam) supaya Anda sempat berganti.
- Administrator dapat membatasi kunci Anda ke **alamat IP** tertentu; permintaan dari alamat lain ditolak (403). Berikan alamat keluar (egress) server Anda kepada administrator saat meminta pendaftaran.

## 3. Endpoint

Semua hanya `GET`, respons JSON `{"success": true, "message": "OK", "data": {...}}`.

| Endpoint | Isi | Kuota |
|---|---|---|
| `/katalog` | dataset dalam scope kunci Anda (aset dan pengadaan) beserta kolom, tipe, dan penyaring yang didukung | tidak |
| `/aset/{dataset}` | satu halaman data aset | ya |
| `/aset/{dataset}/{id}` | satu baris data aset | ya |
| `/pengadaan/{dataset}` | satu halaman data pengadaan (Inaproc) | ya |
| `/pengadaan/{dataset}/{id}` | satu baris data pengadaan | ya |
| `/saya` | profil klien, scope, batas yang berlaku, sisa kuota | tidak |
| `/saya/penggunaan?rentang=24j` | ringkasan pemakaian Anda sendiri (jumlah, galat, 429, latensi, permintaan terakhir); `rentang`: `1j`, `6j`, `24j`, `7h`, `30h` | tidak |

Ada dua kelompok dataset. Kode dataset (di `/katalog`, `/saya`, dan permintaan scope) selalu berawalan kelompoknya; bagian setelah titik adalah `{dataset}` pada alamat.

**Aset** (`aset.{dataset}`, alamat `/aset/{dataset}`): `satker`, `tanah`, `gedung_kantor_utama`, `gedung_lainnya`, `rusunara`, `rumah_negara`, `mess_rumah_negara`.

**Pengadaan** (`pengadaan.{dataset}`, alamat `/pengadaan/{dataset}`), data Inaproc yang disimpan PASTI untuk Kemenkeu:

| Subkelompok | `{dataset}` |
|---|---|
| Perencanaan (RUP) | `rup_paket_penyedia`, `rup_paket_penyedia_terumumkan`, `rup_anggaran_penyedia`, `rup_paket_swakelola`, `rup_paket_swakelola_terumumkan`, `rup_anggaran_swakelola`, `rup_riwayat_kaji_ulang` |
| Tender | `tender_pengumuman`, `tender_peserta`, `tender_jadwal_tahapan`, `tender_kontrak`, `tender_selesai`, `tender_selesai_nilai` |
| Non-tender | `nontender_pengumuman`, `nontender_selesai`, `nontender_kontrak`, `nontender_jadwal_tahapan` |
| Pencatatan | `pencatatan_nontender`, `pencatatan_nontender_realisasi`, `pencatatan_swakelola`, `pencatatan_swakelola_realisasi` |
| E-Katalog | `ekatalog_v5_paket`, `ekatalog_v6_paket`, `ekatalog_instansi_satker` |

Hanya dataset yang diizinkan untuk Anda yang dapat dibuka (lainnya 403); nama kolom dan tipenya ada di `/katalog`.

### Parameter daftar data (`/aset/{dataset}` dan `/pengadaan/{dataset}`)

| Parameter | Arti |
|---|---|
| `per_halaman` | jumlah baris (bawaan 100; maksimal sesuai pengaturan PASTI, bawaan 500) |
| `kursor` | nilai `kursor_berikutnya` dari respons sebelumnya (kosong = mulai dari awal) |
| `hitung=1` | sertakan `total` baris yang cocok (lebih lambat; pakai seperlunya) |
| `q` | pencarian teks, minimal 3 karakter. Aset: nama, alamat, kode. Pengadaan: nama paket, nama satker, dan kode paket/penyedia (kolom persisnya per dataset) |
| `kode_satker` | aset: kode satker 6 digit, atau awalan kode satker lengkap. Pengadaan: kode satker **6 digit** |
| `kode_ue1` | kode UE1 5 digit |
| `provinsi`, `kab_kota` | nama persis; **hanya aset** (pada dataset pengadaan ditolak 400, bukan diabaikan) |
| `kode_klpd` | **hanya pengadaan**: kode KLPD Inaproc, mis. `K10` |
| `tahun` | **hanya pengadaan**: tahun anggaran empat digit, mis. `2025` (dataset `ekatalog_instansi_satker` tidak punya tahun) |

Penyaring yang tidak didukung dataset ditolak 400 supaya Anda tidak menerima data tanpa penyaring yang Anda kira berlaku. Daftar penyaring tiap dataset ada di `penyaring` pada `/katalog`.

Respons: `baris` (daftar objek; nama kolom huruf kecil, nilai desimal sebagai angka), `jumlah`, `ada_berikutnya`, `kursor_berikutnya` (null bila habis), `total` (bila diminta), `disinkronkan_terakhir`. Data PASTI disalin
dari SLDK (aset) dan Inaproc (pengadaan) secara berkala; setiap baris memuat `disinkronkan_pada`. Kolom `id` adalah penanda baris untuk paginasi dan permintaan satu baris:

- **Aset**: `id` bilangan; **berubah setelah tiap sinkronisasi**. Jangan menyimpannya sebagai kunci jangka panjang, pakai kode aset (mis. `kode_register_*`, `kode_satker`, `nup_*`).
- **Pengadaan**: `id` teks buram (jangan ditafsirkan); `kursor_berikutnya` juga teks. Pakai kode paket sebagai kunci jangka panjang (`kd_rup`, `kd_tender`, `kd_nontender`, `no_kontrak`, dan sebagainya). Satu paket dapat punya beberapa baris
  (mis. pengumuman tender per versi, peserta, anggaran per sumber dana), dan `kd_rup` pada beberapa dataset dapat memuat beberapa kode dipisah `;`.

Menarik seluruh data: ulangi permintaan dengan `kursor` dari respons sebelumnya sampai `ada_berikutnya` bernilai `false`.

**Data yang tidak pernah diberikan**:

- Aset: kolom data pribadi (mis. nama penghuni rumah negara), tautan foto, dan nama pengguna bangunan.
- Pengadaan: data orang (nama, NIP, jabatan, SK, username, email, telepon PPK, Pokja, dan petugas lain; wakil sah penyedia, anggota KSO, pelaksana swakelola), identitas pajak dan keuangan (NPWP, nama dan nomor rekening bank),
  serta isian bebas petugas (alasan, keterangan, informasi lainnya), tautan dokumen, dan riwayat BAP/BAST/penilaian kinerja. Nama penyedia dan kode penyedia **diberikan**. Kolom yang diberikan ada di `/katalog`.

**Batas UE1 pada data pengadaan**: Inaproc hanya mencatat kode satker 6 digit, tanpa UE1. Untuk klien yang dibatasi ke UE1, atau bila Anda memakai `kode_ue1`, PASTI menentukan satker milik UE1 itu dari data aset. Akibatnya
pengadaan satker yang tidak ada di data aset PASTI tidak terlihat. Untuk dataset yang tidak memuat kode satker (mis. `tender_selesai_nilai`, `ekatalog_v5_paket`, `ekatalog_v6_paket`, `rup_paket_swakelola`, `pencatatan_swakelola_realisasi`), baris dibatasi
lewat kode paketnya; baris yang paketnya tidak ditemukan pada data satker (mis. paket e-purchasing swasta tanpa kode RUP) **tidak ikut** bagi klien yang dibatasi UE1.

## 4. Batas laju dan kuota

Bawaan: **1.000 permintaan per 60 detik** dan **kuota 5.000 permintaan per jam** (pulih tiap jam penuh UTC). Administrator dapat menetapkan nilai lain untuk Anda; nilai yang berlaku ada di `/saya`.

Setiap respons membawa header (tidak peka huruf besar/kecil):

| Header | Arti |
|---|---|
| `X-RateLimit-Limit` / `-Remaining` / `-Reset` | batas laju: jumlah, sisa, dan waktu pulih penuh (detik Unix UTC) |
| `X-Quota-Limit` / `-Remaining` / `-Reset` | kuota: jumlah, sisa, dan waktu kuota pulih (detik Unix UTC) |
| `Retry-After` | pada 429/503: detik yang harus ditunggu |

Melewati batas menghasilkan **429**:

```json
{"success": false, "message": "Batas permintaan terlampaui. ...", "code": "batas_laju_terlampaui", "retry_after_detik": 12}
```

`code`: `batas_laju_terlampaui` (tunggu `Retry-After`, biasanya detik) atau `kuota_habis` (tunggu sampai `X-Quota-Reset`). Permintaan yang ditolak tidak mengurangi jatah.

Praktik yang baik: baca `X-RateLimit-Remaining`/`X-Quota-Remaining` dan perlambat sebelum habis; hormati `Retry-After` (jangan mengulang segera); gunakan `per_halaman` besar daripada banyak permintaan kecil; tarik data
secukupnya (data berubah hanya saat sinkronisasi).

## 5. Kode galat

| HTTP | `code` | Arti |
|---|---|---|
| 400 | – | parameter tidak valid, atau penyaring yang tidak didukung dataset itu (pesan menyebut parameternya) |
| 401 | `kunci_tidak_valid` | kunci tidak ada, salah, kedaluwarsa, dicabut, atau klien belum/tidak lagi disetujui (sengaja tidak dibedakan) |
| 403 | `alamat_ditolak` | alamat IP tidak ada dalam allowlist kunci Anda |
| 403 | `dataset_ditolak` | dataset di luar scope kunci Anda |
| 404 | – | dataset atau baris tidak ada (termasuk dataset pengadaan yang dibuka lewat alamat `/aset/...` atau sebaliknya) |
| 429 | `batas_laju_terlampaui`, `kuota_habis`, `terlalu_banyak_percobaan` | lihat bagian 4; yang terakhir: terlalu banyak percobaan dengan kunci tidak valid dari alamat Anda |
| 503 | `layanan_tidak_tersedia` | layanan sedang bermasalah; coba lagi setelah `Retry-After` |

Setiap respons membawa `X-Request-ID`; sertakan saat melapor ke administrator agar permintaan dapat dilacak.

## 6. Contoh

```bash
curl -H "Authorization: Bearer $PASTI_API_KEY" \
  "https://pasti.kemenkeu.go.id/api/data/v1/aset/tanah?kode_ue1=01504&per_halaman=500&hitung=1"

# data pengadaan: pengumuman tender KLPD K10 tahun 2025
curl -H "Authorization: Bearer $PASTI_API_KEY" \
  "https://pasti.kemenkeu.go.id/api/data/v1/pengadaan/tender_pengumuman?kode_klpd=K10&tahun=2025&per_halaman=500"
```

```python
import os, time, requests

BASE = "https://pasti.kemenkeu.go.id/api/data/v1"
s = requests.Session()
s.headers["Authorization"] = "Bearer " + os.environ["PASTI_API_KEY"]

def tarik(kelompok, dataset, **param):   # kelompok: "aset" atau "pengadaan"
    kursor = None
    while True:
        p = dict(param, per_halaman=500)
        if kursor:
            p["kursor"] = kursor
        r = s.get(f"{BASE}/{kelompok}/{dataset}", params=p, timeout=60)
        if r.status_code == 429:                       # hormati Retry-After
            time.sleep(int(r.headers.get("Retry-After", "5")))
            continue
        r.raise_for_status()
        d = r.json()["data"]
        yield from d["baris"]
        if not d["ada_berikutnya"]:
            return
        kursor = d["kursor_berikutnya"]

for baris in tarik("aset", "satker"):
    ...

for baris in tarik("pengadaan", "tender_pengumuman", kode_klpd="K10", tahun="2025"):
    ...
```

Pertanyaan dan permintaan perubahan scope atau batas: hubungi administrator PASTI dengan menyebut **ID klien** (`KL-…`).
