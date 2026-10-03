# Sistem desain antarmuka (UI/UX)

Tampilan PASTI V3 dibangun di atas satu sistem desain kecil supaya semua halaman terasa seragam. Semua nilainya ada di
`frontend/tailwind.config.ts` dan `frontend/app/globals.css`.

## Token (tailwind.config.ts)

| Token | Nilai |
|---|---|
| `blue` | Palet biru merek (600 = `#3358e0`), menggantikan biru bawaan Tailwind. Seluruh kelas `bg-blue-*`/`text-blue-*` yang sudah dipakai otomatis ikut berubah. |
| `slate` | Abu-abu netral kebiruan (50 = `#f7f9fc`, 900 = `#0f172a`). |
| Radius | `lg` 10 px, `xl` 14 px, `2xl` 18 px, `3xl` 24 px. |
| Bayangan | `shadow-sm/md/lg/xl` lembut berlapis; `shadow-card` (kartu), `shadow-glow` (cincin fokus kontrol). |
| Animasi | `animate-fade-up`, `fade-in`, `scale-in`, `slide-in-right`, `drawer-in`, `shimmer`, `grow-x`, `pop-in` (semua `backwards`, tanpa menyisakan transform), kurva `ease-smooth`. |
| Font | Tumpukan font sistem (Inter bila terpasang, lalu Segoe UI Variable/SF/Roboto). Tidak ada unduhan font, jadi aman di jaringan yang membatasi akses luar. |

Bila warna merek diganti, ubah palet `blue` **dan** konstanta `BAR` di `frontend/components/sldk/charts.tsx` (grafik memakai hex).

## globals.css

- Dasar: latar `#f4f6fb`, anti-alias font, `::selection` biru, cincin `:focus-visible`, bilah gulir tipis, transisi halus untuk
  tautan/tombol/kontrol, angka tabel rata (`tabular-nums`), judul tabel kapital kecil, aksen `checkbox` biru.
- Komponen: `.card`, `.card-interactive`, `.skeleton` (kilau), `.page-enter`, `.stagger` (anak muncul berurutan).
- Menghormati `prefers-reduced-motion` (animasi dimatikan, putaran indikator tetap berjalan).

## Komponen bersama (frontend/components/ui)

| Komponen | Fungsi |
|---|---|
| `Button` | Varian `primary/secondary/soft/ghost/danger`, ukuran `sm/md/lg`, `isLoading`, `icon`. Bawaan tetap selebar wadah (`fullWidth`); `buttonClasses()` untuk tautan bergaya tombol. |
| `Input` | Label terhubung ke input, ikon, tampil/sembunyi password, pesan galat beranimasi. |
| `Alert` | `tone`: `error` (bawaan), `success`, `warning`, `info`. |
| `ModalShell` | Modal beranimasi (lembar dari bawah di ponsel), Escape/klik luar menutup, kunci gulir halaman, `size`. |
| `ConfirmDialog` | Pengganti `window.confirm`; fokus awal di "Batal". |
| `Toast` (`ToastProvider`, `useToast`) | Pemberitahuan singkat: `success/error/info/warning`; berhenti menghitung mundur saat disorot. |
| `Tabs` | Tab pil dengan penanda geser, panah/Home/End (pola WAI-ARIA); `idPrefix` untuk id panel. |
| `PageHeader`, `PageShell` | Judul halaman (ikon, judul, deskripsi, aksi) dan kerangka header + kartu isi. |
| `Skeleton`, `SkeletonTable`, `SkeletonCards` | Kerangka pemuatan. |
| `EmptyState` | Keadaan kosong dengan ikon, penjelasan, dan aksi. |
| `TopLoader` | Bilah kemajuan di tepi atas saat berpindah halaman atau permintaan API lebih dari ±200 ms (hitungan dari `lib/netActivity.ts`, dipasang di interceptor axios). |

## Kerangka aplikasi

- `components/layout/Sidebar.tsx`: kelompok menu otomatis (Menu / Modul / Administrasi) dari `lib/navigation.ts`, submenu buka-tutup
  halus, ikon-saja saat diciutkan, drawer di ponsel. Menu anak mengikuti pembatasan role.
- `components/layout/Navbar.tsx`: remah roti (dari `breadcrumbsFor`), tombol cari menu, menu pengguna (profil, beranda, keluar).
- `components/layout/CommandPalette.tsx`: **Ctrl/Cmd + K** dari halaman mana pun; ketik sebagian nama menu lalu Enter; halaman yang baru
  dibuka tampil lebih dulu (disimpan di `localStorage`, tidak dikirim ke server).
- `app/dashboard/template.tsx`: isi halaman masuk dengan animasi naik di setiap perpindahan; `loading.tsx` = kerangka seketika;
  `error.tsx` = batas galat yang ramah (sidebar tetap hidup); `app/not-found.tsx` = halaman 404.
- Judul tab browser mengikuti halaman ("Jadwal Tender · PASTI V3").

## Menambah halaman baru

1. Tambahkan menu di `lib/navigation.ts` (otomatis muncul di sidebar, Akses Cepat, pencarian menu, dan remah roti).
2. Bungkus halaman dengan `<PageShell title icon description>` (atau `PageHeader` + kartu sendiri).
3. Gunakan `useToast()` untuk umpan balik singkat, `ConfirmDialog` untuk tindakan merusak, `EmptyState`/`Skeleton*` untuk keadaan kosong dan memuat.

## Yang sengaja tidak diubah

Logika bisnis, API, dan alur data tidak berubah. Mode gelap belum ada (token warna sudah terpusat sehingga bisa ditambahkan nanti).
