import type { PenarikanDataset, PenarikanKuota, PenarikanOtomatis, PenarikanPengaturan, PenarikanStatusTugas, PermintaanPenarikan, PermintaanTugas, TugasBermasalah } from "./api";

// Pembantu murni untuk halaman Penarikan Data dan Dasbor Pengadaan terpadu (tanpa React, supaya bisa diuji dengan node --test).

export const NAMA_BULAN = ["Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"];
export const BULAN_SINGKAT = ["Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"];

// ---- angka ----

const ID = "id-ID";

export function formatAngka(n: number | null | undefined, desimal = 0): string {
  if (n === null || n === undefined || Number.isNaN(n)) return "-";
  return new Intl.NumberFormat(ID, { maximumFractionDigits: desimal, minimumFractionDigits: 0 }).format(n);
}

export function formatPersen(n: number | null | undefined, desimal = 1): string {
  if (n === null || n === undefined || !Number.isFinite(n)) return "-";
  return `${formatAngka(n, desimal)}%`;
}

// Nilai rupiah ringkas untuk kartu dan sumbu: "Rp 1,25 T", "Rp 3,4 M", "Rp 12,5 jt", "Rp 950.000". T = triliun, M = miliar.
export function rupiahRingkas(n: number | null | undefined): string {
  if (n === null || n === undefined || !Number.isFinite(n)) return "-";
  const abs = Math.abs(n);
  const tanda = n < 0 ? "-" : "";
  if (abs >= 1e12) return `${tanda}Rp ${formatAngka(abs / 1e12, 2)} T`;
  if (abs >= 1e9) return `${tanda}Rp ${formatAngka(abs / 1e9, 2)} M`;
  if (abs >= 1e6) return `${tanda}Rp ${formatAngka(abs / 1e6, 1)} jt`;
  return `${tanda}Rp ${formatAngka(Math.round(abs))}`;
}

// Versi tanpa "Rp" untuk sumbu grafik.
export function sumbuRingkas(n: number): string {
  const abs = Math.abs(n);
  if (abs >= 1e12) return `${formatAngka(n / 1e12, 1)} T`;
  if (abs >= 1e9) return `${formatAngka(n / 1e9, 1)} M`;
  if (abs >= 1e6) return `${formatAngka(n / 1e6, 0)} jt`;
  if (abs >= 1e3) return `${formatAngka(n / 1e3, 0)} rb`;
  return formatAngka(n);
}

// Batas atas sumbu yang "bulat": 1, 2, 2.5, 5 atau 10 dikali pangkat sepuluh.
export function batasSumbu(maks: number): number {
  if (!Number.isFinite(maks) || maks <= 0) return 1;
  const pangkat = Math.pow(10, Math.floor(Math.log10(maks)));
  const f = maks / pangkat;
  const langkah = f <= 1 ? 1 : f <= 2 ? 2 : f <= 2.5 ? 2.5 : f <= 5 ? 5 : 10;
  return langkah * pangkat;
}

// Skala sumbu dengan langkah "bulat" (1, 2, 2,5, 5 dikali pangkat sepuluh) dan paling banyak sekitar `target` selang. `bulat` (jumlah paket,
// dst.) melarang langkah pecahan supaya label sumbu tidak berulang (mis. 0, 1, 1, 2, 2). Batas atas selalu >= nilai terbesar.
export function skalaSumbu(maks: number, bulat = false, target = 4): { batas: number; langkah: number } {
  if (!Number.isFinite(maks) || maks <= 0) return { batas: 1, langkah: bulat ? 1 : 0.25 };
  const mentah = maks / target;
  const pangkat = Math.pow(10, Math.floor(Math.log10(mentah)));
  const f = mentah / pangkat;
  const pilihan = bulat ? [1, 2, 5, 10] : [1, 2, 2.5, 5, 10];
  let langkah = (pilihan.find((x) => f <= x + 1e-9) ?? 10) * pangkat;
  if (bulat && langkah < 1) langkah = 1;
  const batas = Math.ceil(maks / langkah - 1e-9) * langkah;
  return { batas, langkah };
}

export function bagi(bagian: number, total: number): number {
  return total > 0 ? (bagian / total) * 100 : 0;
}

// ---- waktu ----

export function formatWaktu(iso: string | null | undefined): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "-";
  return d.toLocaleString(ID, { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit", timeZone: "Asia/Jakarta" }) + " WIB";
}

export function formatTanggal(iso: string | null | undefined): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "-";
  return d.toLocaleDateString(ID, { day: "numeric", month: "short", year: "numeric", timeZone: "Asia/Jakarta" });
}

// "3 jam lalu", "2 hari lalu"; selisih negatif (jam perangkat tidak cocok) dianggap "baru saja".
export function waktuRelatif(iso: string | null | undefined, sekarang: number = Date.now()): string {
  if (!iso) return "belum pernah";
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "-";
  const detik = Math.max(0, Math.floor((sekarang - t) / 1000));
  if (detik < 60) return "baru saja";
  const menit = Math.floor(detik / 60);
  if (menit < 60) return `${menit} menit lalu`;
  const jam = Math.floor(menit / 60);
  if (jam < 24) return `${jam} jam lalu`;
  const hari = Math.floor(jam / 24);
  return `${hari} hari lalu`;
}

export function durasi(mulai: string | null | undefined, selesai: string | null | undefined): number | null {
  if (!mulai || !selesai) return null;
  const a = new Date(mulai).getTime();
  const b = new Date(selesai).getTime();
  if (Number.isNaN(a) || Number.isNaN(b) || b < a) return null;
  return Math.round((b - a) / 1000);
}

export function formatDurasi(detik: number): string {
  if (detik < 60) return `${detik} dtk`;
  const m = Math.floor(detik / 60);
  if (m < 60) return `${m} mnt ${detik % 60} dtk`;
  const j = Math.floor(m / 60);
  return `${j} jam ${m % 60} mnt`;
}

// ---- status tugas ----

export type NadaStatus = "ok" | "gagal" | "jalan" | "tunggu" | "henti";

export const STATUS_TUGAS: Record<PenarikanStatusTugas, { label: string; nada: NadaStatus }> = {
  antri: { label: "Antri", nada: "tunggu" },
  berjalan: { label: "Berjalan", nada: "jalan" },
  sukses: { label: "Berhasil", nada: "ok" },
  gagal: { label: "Gagal", nada: "gagal" },
  dibatalkan: { label: "Dibatalkan", nada: "henti" },
  dilewati: { label: "Dilewati", nada: "henti" },
};

// "K10/2025" -> "K10 · 2025"; "L2:ABC" -> "Tingkat 2 · ABC"; "2025/COMPLETED/K10" tetap dengan pemisah titik tengah.
export function labelParameter(parameter: string): string {
  if (!parameter) return "";
  if (parameter === "L1") return "Tingkat 1";
  const kat = /^L([23]):(.+)$/.exec(parameter);
  if (kat) return `Tingkat ${kat[1]} · ${kat[2].replace("/", " / ")}`;
  return parameter.split("/").join(" · ");
}

// ---- katalog dataset ----

export interface KelompokDataset {
  id: string;
  nama: string;
  subkelompok: { nama: string; datasets: PenarikanDataset[] }[];
}

// Mengelompokkan dataset menurut kelompok lalu subkelompok, mengikuti urutan dari server.
export function kelompokkanDataset(datasets: PenarikanDataset[], kelompok: { id: string; nama: string }[]): KelompokDataset[] {
  return kelompok
    .map((k) => {
      const subs: { nama: string; datasets: PenarikanDataset[] }[] = [];
      for (const d of datasets) {
        if (d.kelompok !== k.id) continue;
        let s = subs.find((x) => x.nama === d.subkelompok);
        if (!s) {
          s = { nama: d.subkelompok, datasets: [] };
          subs.push(s);
        }
        s.datasets.push(d);
      }
      return { id: k.id, nama: k.nama, subkelompok: subs };
    })
    .filter((k) => k.subkelompok.length > 0);
}

export type KesegaranData = "kosong" | "belum" | "segar" | "lama" | "gagal";

// Keadaan data sebuah dataset untuk penanda di daftar: belum ada baris, belum pernah ditarik, segar (<= batas hari), lama, atau percobaan terakhir gagal.
export function kesegaranDataset(d: PenarikanDataset, sekarang: number = Date.now(), batasHari = 7): KesegaranData {
  const ok = d.terakhir_sukses?.selesai ? new Date(d.terakhir_sukses.selesai).getTime() : null;
  const gagalBaru = d.terakhir && d.terakhir.status === "gagal" && (!d.terakhir_sukses || d.terakhir.id > d.terakhir_sukses.id);
  if (gagalBaru) return "gagal";
  if (ok === null) return d.baris > 0 ? "lama" : "belum";
  if (d.baris === 0) return "kosong";
  return sekarang - ok > batasHari * 86400_000 ? "lama" : "segar";
}

// ---- jadwal ----

export const jam = (n: number) => `${String(n).padStart(2, "0")}.00`;

export function kalimatJadwal(o: PenarikanOtomatis, p: PenarikanPengaturan): string {
  if (!o.token_ada) return "Penarikan otomatis tidak berjalan: token Inaproc belum dikonfigurasi di server.";
  if (!p.aktif) return "Penarikan otomatis dimatikan. Data hanya diperbarui lewat penarikan manual.";
  const periode = p.interval_hari === 1 ? "setiap hari" : `setiap ${p.interval_hari} hari`;
  const jendela = p.jam_mulai === p.jam_akhir ? "kapan saja" : `dimulai antara pukul ${jam(p.jam_mulai)} dan ${jam(p.jam_akhir)} ${o.zona}`;
  const tahun = p.jumlah_tahun === 1 ? "tahun berjalan" : `tahun berjalan dan ${p.jumlah_tahun - 1} tahun sebelumnya`;
  const next = o.berikutnya ? ` Berikutnya sekitar ${formatWaktu(o.berikutnya)}.` : "";
  return `Otomatis ${periode}, ${jendela}, untuk ${tahun}.${next}`;
}

// Daftar tahun untuk pilihan: tahun ini mundur sebanyak jumlah, terbaru dulu.
export function daftarTahun(tahunIni: number, jumlah = 8): string[] {
  return Array.from({ length: jumlah }, (_, i) => String(tahunIni - i));
}

export function tahunValid(t: string): boolean {
  return /^(19|20)\d{2}$/.test(t.trim());
}

// ---- permintaan penarikan manual ----

export interface PilihanTarik {
  kodeKLPD: string;
  tahun: string[]; // tahun terpilih
  dataset: string[]; // id dataset terpilih
  kode: Record<string, string>; // dataset per kode: satu atau beberapa kode (dipisah spasi, koma, titik koma, atau baris baru)
  opsi: Record<string, { status?: string; kd1?: string; kd2?: string }>;
}

export const MAKS_KODE_PER_DATASET = 50;

export function pisahKode(teks: string): string[] {
  return Array.from(new Set(teks.split(/[\s,;]+/).map((k) => k.trim()).filter(Boolean)));
}

// Status transaksi yang sah di E-Katalog V6; bawaan COMPLETED.
export const STATUS_TRANSAKSI = [
  "COMPLETED", "ON_PROCESS", "ON_ADDENDUM", "ON_NEGOTIATION", "WAITING_PPK_REVIEW", "WAITING_SELLER_CONFIRMATION", "ESIGN_IN_PROGRESS",
  "PAYMENT_OUTSIDE_SYSTEM", "CANCELLED", "CANCELLED_ON_NEGOTIATION", "CANCELLED_ON_REVIEW", "REQUEST_CANCEL_BY_ADMIN",
];

// Menyusun badan POST /inaproc/penarikan dari pilihan di formulir, beserta jumlah tugas dan daftar masalah yang menghalangi (kode belum diisi dst).
// Dataset yang cukup dengan KLPD dan tahun dikirim sebagai matriks (dataset x tahun); yang butuh isian khusus menjadi tugas eksplisit.
export function bangunPermintaan(datasets: PenarikanDataset[], p: PilihanTarik): { permintaan: PermintaanPenarikan; jumlahTugas: number; masalah: string[] } {
  const klpd = p.kodeKLPD.trim();
  const tahun = Array.from(new Set(p.tahun.map((t) => t.trim()).filter(Boolean)));
  const matriks: string[] = [];
  const tugas: PermintaanTugas[] = [];
  const masalah: string[] = [];
  let jumlah = 0;

  const pilih = new Set(p.dataset);
  for (const d of datasets) {
    if (!pilih.has(d.id)) continue;
    const o = p.opsi[d.id] ?? {};
    const status = (o.status ?? "").trim();
    switch (d.mode) {
      case "kode": {
        const kode = pisahKode(p.kode[d.id] ?? "");
        if (kode.length === 0) masalah.push(`${d.nama}: isi kode yang akan dicari`);
        else if (kode.length > MAKS_KODE_PER_DATASET) masalah.push(`${d.nama}: maksimal ${MAKS_KODE_PER_DATASET} kode sekali tarik`);
        else for (const k of kode) tugas.push({ dataset: d.id, kode: k });
        break;
      }
      case "kategori": {
        const kd1 = (o.kd1 ?? "").trim();
        const kd2 = (o.kd2 ?? "").trim();
        if (kd2 && !kd1) masalah.push(`${d.nama}: kode kategori tingkat 2 harus disertai tingkat 1`);
        else tugas.push({ dataset: d.id, ...(kd1 ? { kd_kategori_1: kd1 } : {}), ...(kd2 ? { kd_kategori_2: kd2 } : {}) });
        break;
      }
      case "klpd":
        matriks.push(d.id);
        jumlah += 1;
        break;
      default: // klpd_tahun dan transaksi
        if (tahun.length === 0) {
          masalah.push(`${d.nama}: pilih minimal satu tahun`);
        } else if (status) {
          for (const t of tahun) tugas.push({ dataset: d.id, tahun: t, status, ...(klpd ? { kode_klpd: klpd } : {}) });
        } else {
          matriks.push(d.id);
          jumlah += tahun.length;
        }
    }
  }
  const permintaan: PermintaanPenarikan = {};
  if (matriks.length > 0) {
    permintaan.datasets = matriks;
    if (klpd) permintaan.kode_klpd = klpd;
    if (tahun.length > 0) permintaan.tahun = tahun;
  }
  if (tugas.length > 0) permintaan.tugas = tugas.map((t) => (t.kode_klpd || !klpd || t.kode ? t : { ...t, kode_klpd: klpd }));
  return { permintaan, jumlahTugas: jumlah + tugas.length, masalah };
}
// ---- kuota dan kebijakan gagal tarik ----

export type NadaKuota = "normal" | "waspada" | "melambat" | "habis" | "ditahan";

// Keadaan kuota: normal, waspada (>= 80% jatah jam atau menit), melambat (batas per menit penuh: menunggu hitungan detik), habis (jatah jam
// habis: menunggu sampai jatah longgar), atau ditahan (jeda bersama setelah 429).
export function nadaKuota(k: PenarikanKuota, sekarang: number = Date.now()): NadaKuota {
  if (k.tahan_sampai && new Date(k.tahan_sampai).getTime() > sekarang) return "ditahan";
  if (k.sisa_jam <= 0) return "habis";
  if (k.pulih_sekitar || k.terpakai_menit >= k.batas_per_menit) return "melambat";
  const pakaiJam = k.batas_per_jam > 0 ? k.terpakai_jam / k.batas_per_jam : 0;
  const pakaiMenit = k.batas_per_menit > 0 ? k.terpakai_menit / k.batas_per_menit : 0;
  return Math.max(pakaiJam, pakaiMenit) >= 0.8 ? "waspada" : "normal";
}

export function persenPakai(terpakai: number, batas: number): number {
  if (batas <= 0) return 0;
  return Math.min(100, Math.max(0, (terpakai / batas) * 100));
}

export function kalimatKuota(k: PenarikanKuota, sekarang: number = Date.now()): string {
  switch (nadaKuota(k, sekarang)) {
    case "ditahan":
      return `Inaproc membatasi laju (429), jadi semua permintaan ditahan sampai ${formatWaktu(k.tahan_sampai)}.`;
    case "habis":
      return `Jatah permintaan per jam habis. Penarikan menunggu dan lanjut sekitar ${formatWaktu(k.pulih_sekitar)}.`;
    case "melambat":
      return "Batas per menit sedang penuh; permintaan berikutnya menunggu hitungan detik sampai jendela 60 detik longgar, lalu lanjut sendiri.";
    case "waspada":
      return "Pemakaian mendekati batas. Penarikan melambat sendiri bila perlu supaya tidak ditolak Inaproc.";
  }
  return "Pemakaian wajar.";
}

export function kalimatKebijakan(maks: number, istirahatJam: number): string {
  const jam = Number.isInteger(istirahatJam) ? String(istirahatJam) : formatAngka(istirahatJam, 1);
  return `Tugas yang gagal dicoba ulang otomatis sampai ${maks} kali dalam sehari (jarak minimal 10 menit antar percobaan). Setelah ${maks} kali gagal, tugas istirahat ${jam} jam, lalu bisa ditarik ulang.`;
}

export type NadaBermasalah = "ulang" | "istirahat" | "manual";

// Keadaan satu tugas bermasalah: percobaan ulang terjadwal, istirahat, atau di luar rencana otomatis (tidak dicoba otomatis lagi).
export function keadaanBermasalah(b: TugasBermasalah, sekarang: number = Date.now()): { label: string; nada: NadaBermasalah } {
  const kapan = b.berikutnya_sekitar ? formatWaktu(b.berikutnya_sekitar) : null;
  if (b.istirahat) {
    return {
      label: kapan && b.dalam_rencana ? `Istirahat sampai ${kapan}, lalu ditarik ulang otomatis` : "Istirahat setelah gagal berulang; tarik manual bila perlu",
      nada: "istirahat",
    };
  }
  if (b.berikutnya_sekitar) {
    const lewat = new Date(b.berikutnya_sekitar).getTime() <= sekarang;
    return { label: `Gagal ${b.gagal} dari ${b.maks_percobaan} percobaan; percobaan berikut ${lewat ? "segera" : "sekitar " + kapan}`, nada: "ulang" };
  }
  return { label: `Gagal ${b.gagal} dari ${b.maks_percobaan} percobaan; tidak dicoba otomatis (penarikan otomatis nonaktif atau di luar rencana)`, nada: "manual" };
}