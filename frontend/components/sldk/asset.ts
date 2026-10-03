// Pembaca data aset SLDK (tabel DJKN.SIMAN2_M_ASET). Semua nilai datang longgar: kolom bisa NULL, angka desimal
// datang sebagai string ("1500000.000000000000000000"), dan penanda ya/tidak (*_yn) tidak seragam. Fungsi di sini
// tidak pernah melempar error.

export type Row = Record<string, unknown>;

export function str(v: unknown): string {
  return typeof v === "string" ? v.trim() : "";
}

// Angka dari number atau string desimal; null untuk kosong/bukan angka.
export function num(v: unknown): number | null {
  if (typeof v === "number") return Number.isFinite(v) ? v : null;
  if (typeof v === "string" && v.trim() !== "") {
    const n = Number(v);
    return Number.isFinite(n) ? n : null;
  }
  return null;
}

const YES = /^(y|ya|yes|1|true)$/i;
const NO = /^(t|tidak|n|no|0|false)$/i;

export function isYes(v: unknown): boolean {
  if (typeof v === "boolean") return v;
  if (typeof v === "number") return v === 1;
  return typeof v === "string" && YES.test(v.trim());
}

// "Ya" / "Tidak" bila nilainya jelas; kalau tidak, teks aslinya (jangan menebak arti nilai yang tidak dikenal).
export function yesNo(v: unknown): string {
  if (typeof v === "boolean") return v ? "Ya" : "Tidak";
  if (typeof v === "number") return v === 1 ? "Ya" : v === 0 ? "Tidak" : String(v);
  const s = str(v);
  if (!s) return "";
  if (YES.test(s)) return "Ya";
  if (NO.test(s)) return "Tidak";
  return s;
}

// Tahun dari tanggal ISO ("2021-03-04T00:00:00Z"); kosong bila bukan tanggal.
export function tahunDari(v: unknown): string {
  const s = str(v);
  return /^\d{4}-\d{2}-\d{2}/.test(s) ? s.slice(0, 4) : "";
}

export function formatNumber(n: number | null, maxDecimals = 2): string {
  if (n === null) return "";
  return new Intl.NumberFormat("id-ID", { maximumFractionDigits: maxDecimals }).format(n);
}

export type KondisiTone = "baik" | "ringan" | "berat" | "lain";

// Warna badge kondisi dari NAMA kondisi (bukan kodenya), supaya tidak bergantung pada kode yang belum terkonfirmasi.
export function kondisiTone(nama: string): KondisiTone {
  const n = nama.toLowerCase();
  if (/berat/.test(n)) return "berat";
  if (/ringan/.test(n)) return "ringan";
  if (/baik/.test(n)) return "baik";
  return "lain";
}

export interface Koordinat {
  lat: number;
  lng: number;
  // "ok" = di wilayah Indonesia, "luar" = valid tetapi di luar Indonesia (kemungkinan salah input), "tidak valid".
  status: "ok" | "luar" | "tidak valid";
}

export function koordinat(lat: unknown, lng: unknown): Koordinat | null {
  const a = num(lat);
  const b = num(lng);
  if (a === null || b === null) return null;
  if (a === 0 && b === 0) return null; // 0,0 = belum diisi
  if (Math.abs(a) > 90 || Math.abs(b) > 180) return { lat: a, lng: b, status: "tidak valid" };
  const diIndonesia = a >= -11.5 && a <= 6.5 && b >= 94.5 && b <= 141.5;
  return { lat: a, lng: b, status: diIndonesia ? "ok" : "luar" };
}

export interface AssetSummary {
  idAset: number | null;
  idSatker: number | null;
  kodeRegister: string;
  nama: string;
  merkTipe: string;
  kdJnsBmn: string;
  kdKondisi: string;
  kdStatus: string;
  nilaiBuku: number | null;
  nilaiPerolehan: number | null;
  tahunPerolehan: string;
  lokasi: string;
  flags: string[];
  raw: Row;
}

function idNumber(v: unknown): number | null {
  const n = num(v);
  return n !== null && Number.isInteger(n) ? n : null;
}

// Penanda yang layak jadi sorotan di daftar. Hanya nilai "ya" yang jelas yang ditampilkan.
const FLAGS: [string, string][] = [
  ["brg_hilang_yn", "Hilang"],
  ["brg_rusak_yn", "Ditandai rusak"],
  ["dihentikan_yn", "Dihentikan"],
  ["status_bmn_idle", "Idle"],
  ["rencana_hibah_yn", "Rencana hibah"],
];

export function normalizeAsset(row: Row): AssetSummary {
  const merk = str(row.merk);
  const tipe = str(row.tipe);
  return {
    idAset: idNumber(row.id_aset),
    idSatker: idNumber(row.id_satker),
    kodeRegister: str(row.kode_register),
    nama: str(row.ur_sskel),
    merkTipe: [merk, tipe].filter(Boolean).join(" "),
    kdJnsBmn: String(row.kd_jns_bmn ?? "").trim(),
    kdKondisi: str(row.kd_kondisi),
    kdStatus: str(row.kd_status),
    nilaiBuku: num(row.rph_buku),
    nilaiPerolehan: num(row.rph_aset),
    tahunPerolehan: tahunDari(row.tgl_perlh),
    lokasi: [str(row.ur_kab), str(row.ur_prov)].filter(Boolean).join(", "),
    flags: FLAGS.filter(([key]) => isYes(row[key])).map(([, label]) => label),
    raw: row,
  };
}

// Kode -> nama dari daftar referensi.
export function namaDari(items: { kode: string; nama: string }[] | undefined, kode: string): string {
  if (!kode || !items) return "";
  return items.find((i) => i.kode === kode)?.nama ?? "";
}
