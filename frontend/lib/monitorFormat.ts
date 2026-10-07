// Fungsi murni untuk halaman Monitor Resource (tanpa React dan tanpa axios, jadi bisa diuji dengan Node): format angka gaya Indonesia, label dan warna status, dan
// penyusun deret serta sumbu untuk grafik garis.
import type { Kapasitas, RentangRiwayat, StatusResource, TitikRiwayat } from "./monitor";

export function formatAngka(v: number | null | undefined, desimal = 1): string {
  if (v === null || v === undefined || Number.isNaN(v)) return "-";
  return new Intl.NumberFormat("id-ID", { minimumFractionDigits: desimal, maximumFractionDigits: desimal }).format(v);
}

// Bulat tanpa desimal bila sudah bulat; selain itu satu desimal.
export function formatRingkas(v: number | null | undefined): string {
  if (v === null || v === undefined || Number.isNaN(v)) return "-";
  return formatAngka(v, Number.isInteger(v) ? 0 : 1);
}

export function formatPersen(v: number | null | undefined, desimal = 1): string {
  return v === null || v === undefined || Number.isNaN(v) ? "-" : `${formatAngka(v, desimal)}%`;
}

const SATUAN_BYTE = ["B", "KB", "MB", "GB", "TB", "PB"];

export function formatBytes(b: number | null | undefined): string {
  if (b === null || b === undefined || Number.isNaN(b)) return "-";
  let v = Math.max(0, b);
  let i = 0;
  while (v >= 1024 && i < SATUAN_BYTE.length - 1) {
    v /= 1024;
    i++;
  }
  return `${formatAngka(v, i === 0 ? 0 : 1)} ${SATUAN_BYTE[i]}`;
}

export const formatMB = (mb: number | null | undefined): string => (mb === null || mb === undefined ? "-" : formatBytes(mb * 1024 * 1024));

export function formatMs(ms: number | null | undefined): string {
  if (ms === null || ms === undefined || Number.isNaN(ms)) return "-";
  if (ms >= 1000) return `${formatAngka(ms / 1000, ms >= 10_000 ? 0 : 1)} dtk`;
  return `${formatAngka(ms, ms < 10 ? 1 : 0)} ms`;
}

// 90061 -> "1 hari 1 jam 1 menit"; maksimal tiga satuan terbesar yang tidak nol.
export function formatDurasi(detik: number | null | undefined): string {
  if (detik === null || detik === undefined || Number.isNaN(detik)) return "-";
  let s = Math.max(0, Math.floor(detik));
  if (s < 60) return `${s} detik`;
  const hari = Math.floor(s / 86400);
  s -= hari * 86400;
  const jam = Math.floor(s / 3600);
  s -= jam * 3600;
  const menit = Math.floor(s / 60);
  return [hari && `${hari} hari`, jam && `${jam} jam`, menit && `${menit} menit`].filter(Boolean).slice(0, 3).join(" ") || "0 menit";
}

// ---------------------------------------------------------------- status

export const LABEL_STATUS: Record<StatusResource, string> = {
  cukup: "Cukup",
  perhatian: "Perlu perhatian",
  kritis: "Perlu ditambah",
  tidak_ada_data: "Belum dapat dinilai",
};

// Untuk penilaian kinerja (respons, galat, proses) yang "kritis" berarti perlu diperiksa, bukan ditambah.
export function labelStatus(status: StatusResource, kelompok?: Kapasitas["kelompok"]): string {
  if (status === "kritis" && kelompok === "kinerja") return "Perlu diperiksa";
  return LABEL_STATUS[status] ?? status;
}

export interface KelasStatus {
  lencana: string; // kelas Tailwind untuk lencana
  titik: string; // warna titik/ikon
  garis: string; // warna isi bilah
  kartu: string; // pinggiran kartu
}

export const KELAS_STATUS: Record<StatusResource, KelasStatus> = {
  cukup: { lencana: "bg-emerald-50 text-emerald-700 ring-emerald-200", titik: "bg-emerald-500", garis: "bg-emerald-500", kartu: "border-slate-200" },
  perhatian: { lencana: "bg-amber-50 text-amber-800 ring-amber-200", titik: "bg-amber-500", garis: "bg-amber-500", kartu: "border-amber-200" },
  kritis: { lencana: "bg-red-50 text-red-700 ring-red-200", titik: "bg-red-500", garis: "bg-red-500", kartu: "border-red-300" },
  tidak_ada_data: { lencana: "bg-slate-100 text-slate-600 ring-slate-200", titik: "bg-slate-400", garis: "bg-slate-400", kartu: "border-slate-200" },
};

// Warna bilah pemakaian menurut persen (dipakai pada pengukur server).
export function statusDariPersen(p: number | null | undefined, perhatian: number, kritis: number): StatusResource {
  if (p === null || p === undefined || Number.isNaN(p)) return "tidak_ada_data";
  return p >= kritis ? "kritis" : p >= perhatian ? "perhatian" : "cukup";
}

// Kalimat ringkasan di spanduk atas halaman.
export function ringkasanSpanduk(status: StatusResource, perluDitambah: string[], kapasitas: Kapasitas[]): { judul: string; uraian: string } {
  if (perluDitambah.length > 0) {
    return { judul: `Perlu ditambah: ${perluDitambah.join(", ")}`, uraian: "Resource di atas sudah jenuh menurut pengukuran. Lihat kartu resource untuk angka dan alasannya." };
  }
  const perhatian = kapasitas.filter((k) => k.status === "perhatian").map((k) => k.nama);
  const kritisKinerja = kapasitas.filter((k) => k.status === "kritis").map((k) => k.nama);
  if (kritisKinerja.length > 0) return { judul: `Perlu diperiksa: ${kritisKinerja.join(", ")}`, uraian: "Ini masalah kinerja atau ketersambungan; belum tentu teratasi dengan menambah resource." };
  if (perhatian.length > 0) return { judul: `Perlu perhatian: ${perhatian.join(", ")}`, uraian: "Belum perlu menambah resource, tetapi pantau terus." };
  if (status === "tidak_ada_data") return { judul: "Belum cukup data untuk menilai", uraian: "Pengukuran baru mulai; riwayat terkumpul setiap 5 menit." };
  return { judul: "Semua resource cukup", uraian: "Tidak ada yang perlu ditambah saat ini." };
}

// ---------------------------------------------------------------- grafik

export interface DeretGrafik {
  nama: string;
  warna: string;
  nilai: (number | null)[];
}

// Mengambil satu kolom dari titik riwayat. skala membagi nilai (mis. 1024*1024 untuk byte -> MB).
export function ambilKolom(titik: TitikRiwayat[], kunci: keyof TitikRiwayat, skala = 1): (number | null)[] {
  return titik.map((t) => {
    const v = t[kunci];
    return typeof v === "number" && !Number.isNaN(v) ? v / skala : null;
  });
}

// Batas atas sumbu Y yang "bulat": 0-100 untuk persen; selain itu dibulatkan ke 1, 2, 5 x 10^n.
export function batasAtas(nilai: (number | null)[][], tetap?: number): number {
  if (tetap) return tetap;
  let maks = 0;
  for (const d of nilai) for (const v of d) if (v !== null && v > maks) maks = v;
  if (maks <= 0) return 1;
  const pangkat = Math.pow(10, Math.floor(Math.log10(maks)));
  const n = maks / pangkat;
  const bulat = n <= 1 ? 1 : n <= 2 ? 2 : n <= 5 ? 5 : 10;
  return bulat * pangkat;
}

// Jalur SVG untuk satu deret: putus pada nilai kosong supaya celah data tidak tergambar sebagai garis lurus.
export function jalurGaris(nilai: (number | null)[], lebar: number, tinggi: number, maksY: number, kiri = 0): string {
  const n = nilai.length;
  if (n === 0) return "";
  const x = (i: number) => kiri + (n === 1 ? (lebar - kiri) / 2 : (i / (n - 1)) * (lebar - kiri));
  const y = (v: number) => tinggi - Math.min(Math.max(v, 0), maksY) / maksY * tinggi;
  let d = "";
  let buka = false;
  nilai.forEach((v, i) => {
    if (v === null) {
      buka = false;
      return;
    }
    d += `${buka ? "L" : "M"}${x(i).toFixed(1)} ${y(v).toFixed(1)} `;
    buka = true;
  });
  return d.trim();
}

// Jumlah titik data yang ada (bukan kosong) pada deret.
export function jumlahTitik(nilai: (number | null)[]): number {
  return nilai.filter((v) => v !== null).length;
}

export const RENTANG_RIWAYAT: { kunci: RentangRiwayat; label: string; lebar: string }[] = [
  { kunci: "1j", label: "1 jam", lebar: "tiap 30 detik" },
  { kunci: "24j", label: "24 jam", lebar: "tiap 5 menit" },
  { kunci: "7h", label: "7 hari", lebar: "tiap 30 menit" },
  { kunci: "30h", label: "30 hari", lebar: "tiap 2 jam" },
];
