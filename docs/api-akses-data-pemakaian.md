# API Akses Data PASTI: panduan pemakaian untuk pihak luar

Panduan singkat bagi pengembang aplikasi yang ingin menarik data PASTI. Alamat dasar produksi: `https://pasti.kemenkeu.go.id/api/data/v1` (gunakan alamat PASTI yang diberikan administrator).

## 1. Mendaftar

1. Buka `https://pasti.kemenkeu.go.id/pendaftaran-api` dan isi: nama aplikasi, instansi, unit, PIC (nama, email, telepon), tujuan penggunaan, dan dataset yang dibutuhkan. Simpan **nomor pendaftaran** (`KL-XXXXXXXX`).
2. Administrator PASTI menilai pendaftaran. Hanya dataset yang benar-benar diperlukan yang diberikan, dan administrator menentukan batas pemakaian Anda.
3. Bila disetujui, **kunci API** diserahkan kepada PIC lewat jalur yang aman. Kunci **hanya ditampilkan sekali** dan tidak dapat dilihat lagi; bila hilang, minta kunci baru (yang lama dicabut).

## 2. Autentikasi

Kirim kunci di header setiap permintaan (pilih salah satu):

```http
Authorization: Bearer pasti_xxxxxxxxxx_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
X-API-Key: pasti_xxxxxxxxxx_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

- **Jangan** menaruh kunci di alamat (URL/query string): alamat tercatat di log proxy dan riwayat peramban; kunci seperti itu ditolak.
- Simpan kunci sebagai rahasia (variabel lingkungan atau pengelola rahasia), jangan di kode sumber atau aplikasi peramban/seluler. Panggil API dari **server** Anda (tidak ada CORS untuk peramban).
- Bila kunci diduga bocor, segera minta pencabutan. Untuk pergantian rutin, administrator dapat merotasi kunci: kunci lama tetap berlaku sementara (bawaan 24 jam) supaya Anda sempat berganti.
- Administrator dapat membatasi kunci Anda ke **alamat IP** tertentu; permintaan dari alamat lain ditolak (403). Berikan alamat keluar (egress) server Anda saat mendaftar.

## 3. Endpoint

Semua hanya `GET`, respons JSON `{"success": true, "message": "OK", "data": {...}}`.

| Endpoint | Isi | Kuota |
|---|---|---|
| `/katalog` | dataset dalam scope kunci Anda beserta kolom dan tipenya | tidak |
| `/aset/{dataset}` | satu halaman data | ya |
| `/aset/{dataset}/{id}` | satu baris | ya |
| `/saya` | profil klien, scope, batas yang berlaku, sisa kuota | tidak |
| `/saya/penggunaan?rentang=24j` | ringkasan pemakaian Anda sendiri (jumlah, galat, 429, latensi, permintaan terakhir); `rentang`: `1j`, `6j`, `24j`, `7h`, `30h` | tidak |

`{dataset}`: `satker`, `tanah`, `gedung_kantor_utama`, `gedung_lainnya`, `rusunara`, `rumah_negara`, `mess_rumah_negara` (hanya yang diizinkan untuk Anda; lainnya 403).

### Parameter daftar data (`/aset/{dataset}`)

| Parameter | Arti |
|---|---|
| `per_halaman` | jumlah baris (bawaan 100; maksimal sesuai pengaturan PASTI, bawaan 500) |
| `kursor` | nilai `kursor_berikutnya` dari respons sebelumnya (kosong = mulai dari awal) |
| `hitung=1` | sertakan `total` baris yang cocok (lebih lambat; pakai seperlunya) |
| `q` | pencarian teks, minimal 3 karakter (nama, alamat, kode) |
| `kode_satker` | kode satker 6 digit, atau awalan kode satker lengkap |
| `kode_ue1` | kode UE1 5 digit |
| `provinsi`, `kab_kota` | nama persis |

Respons: `baris` (daftar objek; nama kolom huruf kecil, nilai desimal sebagai angka), `jumlah`, `ada_berikutnya`, `kursor_berikutnya` (null bila habis), `total` (bila diminta), `disinkronkan_terakhir`. Data PASTI disalin
dari SLDK secara berkala (umumnya dini hari); setiap baris memuat `disinkronkan_pada`. Kolom `id` adalah penanda baris, dan **berubah setelah tiap sinkronisasi**; jangan menyimpannya sebagai kunci jangka panjang, pakai kode
aset (mis. `kode_register_*`, `kode_satker`, `nup_*`).

Menarik seluruh data: ulangi permintaan dengan `kursor` dari respons sebelumnya sampai `ada_berikutnya` bernilai `false`.

**Data yang tidak pernah diberikan**: kolom data pribadi (mis. nama penghuni rumah negara), tautan foto, dan nama pengguna bangunan.

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
| 400 | – | parameter tidak valid (pesan menyebut parameternya) |
| 401 | `kunci_tidak_valid` | kunci tidak ada, salah, kedaluwarsa, dicabut, atau klien belum/tidak lagi disetujui (sengaja tidak dibedakan) |
| 403 | `alamat_ditolak` | alamat IP tidak ada dalam allowlist kunci Anda |
| 403 | `dataset_ditolak` | dataset di luar scope kunci Anda |
| 404 | – | dataset atau baris tidak ada |
| 429 | `batas_laju_terlampaui`, `kuota_habis`, `terlalu_banyak_percobaan` | lihat bagian 4; yang terakhir: terlalu banyak percobaan dengan kunci tidak valid dari alamat Anda |
| 503 | `layanan_tidak_tersedia` | layanan sedang bermasalah; coba lagi setelah `Retry-After` |

Setiap respons membawa `X-Request-ID`; sertakan saat melapor ke administrator agar permintaan dapat dilacak.

## 6. Contoh

```bash
curl -H "Authorization: Bearer $PASTI_API_KEY" \
  "https://pasti.kemenkeu.go.id/api/data/v1/aset/tanah?kode_ue1=01504&per_halaman=500&hitung=1"
```

```python
import os, time, requests

BASE = "https://pasti.kemenkeu.go.id/api/data/v1"
s = requests.Session()
s.headers["Authorization"] = "Bearer " + os.environ["PASTI_API_KEY"]

def tarik(dataset, **param):
    kursor = None
    while True:
        p = dict(param, per_halaman=500)
        if kursor:
            p["kursor"] = kursor
        r = s.get(f"{BASE}/aset/{dataset}", params=p, timeout=60)
        if r.status_code == 429:                       # hormati Retry-After
            time.sleep(int(r.headers.get("Retry-After", "5")))
            continue
        r.raise_for_status()
        d = r.json()["data"]
        yield from d["baris"]
        if not d["ada_berikutnya"]:
            return
        kursor = d["kursor_berikutnya"]

for baris in tarik("satker"):
    ...
```

Pertanyaan dan permintaan perubahan scope atau batas: hubungi administrator PASTI dengan menyebut **nomor pendaftaran** (`KL-…`).
