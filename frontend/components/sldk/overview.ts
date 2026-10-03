// Fungsi bantu murni untuk halaman Ringkasan dan Pemantauan (Data Aset SLDK). Tidak memakai React atau
// alias "@/", supaya bisa dites langsung dengan Node.

import { formatNumber, kondisiTone } from "./asset";
import type { KondisiTone } from "./asset";

export type Metric = "nilai_buku" | "nilai_perolehan" | "jumlah";

export const METRIC_LABEL: Record<Metric, string> = {
  nilai_buku: "Nilai buku",
  nilai_perolehan: "Nilai perolehan",
  jumlah: "Jumlah aset",
};

export interface Group {
  k1: string | null;
  k2?: string | null;
  jumlah: number;
  nilai_perolehan: number;
  nilai_buku: number;
  nilai_susut: number;
  other?: boolean; // baris gabungan hasil topWithOther
}

export interface RefItem {
  kode: string;
  nama: string;
}

export const EMPTY_LABEL = "(kosong)";

function decimals(n: number): number {
  const abs = Math.abs(n);
  return abs < 10 ? 2 : abs < 100 ? 1 : 0;
}

// Rupiah ringkas: "Rp 1,53 triliun". Angka di bawah satu juta ditulis penuh.
export function compactRupiah(n: number): string {
  if (!Number.isFinite(n)) return "-";
  const abs = Math.abs(n);
  const sign = n < 0 ? "-" : "";
  const units: [number, string][] = [
    [1e15, "kuadriliun"],
    [1e12, "triliun"],
    [1e9, "miliar"],
    [1e6, "juta"],
  ];
  for (const [size, name] of units) {
    if (abs >= size) {
      const v = abs / size;
      return `${sign}Rp ${formatNumber(v, decimals(v))} ${name}`;
    }
  }
  return `${sign}Rp ${formatNumber(abs, 0)}`;
}

export function formatMetric(m: Metric, n: number): string {
  return m === "jumlah" ? formatNumber(n, 0) : compactRupiah(n);
}

// Label sumbu yang pendek ("2 T", "500 jt", "40 rb"); satuan Rp/aset ditulis di judul grafik, bukan di tiap angka.
export function axisLabel(n: number): string {
  if (!Number.isFinite(n)) return "";
  const abs = Math.abs(n);
  const units: [number, string][] = [
    [1e12, "T"],
    [1e9, "M"],
    [1e6, "jt"],
    [1e3, "rb"],
  ];
  for (const [size, name] of units) {
    if (abs >= size) return `${formatNumber(n / size, 1)} ${name}`;
  }
  return formatNumber(n, 0);
}

// Tanggal dan jam dalam zona waktu browser ("3 Okt 2026, 02.10"); waktu sinkronisasi disimpan UTC.
export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString("id-ID", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" });
}

// Permintaan membuka tab Pencarian dengan filter terisi (dari kartu Pemantauan).
export interface SearchPreset {
  nonce: number; // naik tiap permintaan, supaya preset yang sama dua kali tetap memicu pencarian
  anomali?: string;
  kdKondisi?: string;
  satker?: { id: number; kode: string; nama: string };
}

export function share(part: number, total: number): number {
  return total > 0 ? part / total : 0;
}

// 0.1234 -> "12%"; di bawah 10% satu desimal ("4,5%"), di bawah 0,1% ditulis "<0,1%".
export function pct(x: number): string {
  if (!Number.isFinite(x) || x <= 0) return "0%";
  if (x < 0.001) return "<0,1%";
  return `${formatNumber(x * 100, x < 0.1 ? 1 : 0)}%`;
}

export function sortByMetric(groups: Group[], m: Metric): Group[] {
  return [...groups].sort((a, b) => b[m] - a[m]);
}

// n kelompok terbesar; sisanya dijumlahkan menjadi satu baris "Lainnya" (kategori nominal tidak boleh digambar
// dengan puluhan baris atau puluhan warna).
export function topWithOther(groups: Group[], n: number, m: Metric, otherLabel = "Lainnya"): Group[] {
  const sorted = sortByMetric(groups, m);
  if (sorted.length <= n) return sorted;
  const head = sorted.slice(0, n);
  const tail = sorted.slice(n);
  const sum = (key: "jumlah" | "nilai_perolehan" | "nilai_buku" | "nilai_susut") => tail.reduce((acc, g) => acc + g[key], 0);
  return [
    ...head,
    { k1: otherLabel, other: true, jumlah: sum("jumlah"), nilai_perolehan: sum("nilai_perolehan"), nilai_buku: sum("nilai_buku"), nilai_susut: sum("nilai_susut") },
  ];
}

export interface Point {
  x: number;
  y: number;
}

// Titik tren per tahun perolehan; tahun yang bukan tahun wajar (data kotor) dibuang.
export function yearPoints(groups: Group[], m: Metric): Point[] {
  const out: Point[] = [];
  for (const g of groups) {
    if (g.k1 && /^\d{4}$/.test(g.k1)) {
      const year = Number(g.k1);
      if (year >= 1900 && year <= 2100) out.push({ x: year, y: g[m] });
    }
  }
  return out.sort((a, b) => a.x - b.x);
}

// Batas atas sumbu yang "bulat": 1, 2, 5 x 10^k.
export function niceMax(v: number): number {
  if (!Number.isFinite(v) || v <= 0) return 1;
  const pow = Math.pow(10, Math.floor(Math.log10(v)));
  const f = v / pow;
  const nice = f <= 1 ? 1 : f <= 2 ? 2 : f <= 5 ? 5 : 10;
  return nice * pow;
}

// Nama untuk label: nama dari referensi, "Kode X" bila tidak ada di referensi, "(kosong)" bila datanya kosong.
export function refLabel(items: RefItem[] | undefined, kode: string | null): string {
  if (kode === null || kode === "") return EMPTY_LABEL;
  return items?.find((i) => i.kode === kode)?.nama || `Kode ${kode}`;
}

export interface KondisiPart {
  key: string;
  label: string;
  tone: KondisiTone;
  value: number;
}

// Bagian-bagian kondisi, diurutkan menurut kode referensi (Baik, Rusak Ringan, Rusak Berat, ...); data kosong terakhir.
export function kondisiParts(groups: Group[], kondisi: RefItem[] | undefined, m: Metric): KondisiPart[] {
  const rank = (kode: string | null) => {
    if (kode === null || kode === "") return Number.MAX_SAFE_INTEGER;
    const i = kondisi?.findIndex((k) => k.kode === kode) ?? -1;
    return i < 0 ? Number.MAX_SAFE_INTEGER - 1 : i;
  };
  return [...groups]
    .sort((a, b) => rank(a.k1) - rank(b.k1) || (a.k1 ?? "").localeCompare(b.k1 ?? ""))
    .map((g) => {
      const label = refLabel(kondisi, g.k1);
      return { key: g.k1 ?? "", label, tone: kondisiTone(label), value: g[m] };
    });
}

// Kode kondisi "Rusak Berat" dari tabel referensi (nama mengandung "berat"), bukan ditebak.
export function kodeRusakBerat(kondisi: RefItem[] | undefined): string | null {
  return kondisi?.find((k) => /berat/i.test(k.nama))?.kode ?? null;
}

// "1|0|1|A" -> "status_data 1 · sts_his 0 · sts_ast 1 · tgl_hapus kosong"
export function flagKeyLabel(key: string): string {
  const [statusData, stsHis, stsAst, hapus] = key.split("|");
  if (hapus === undefined) return key;
  const dash = (v: string) => (v === "-" ? "kosong" : v);
  return `status_data ${dash(statusData)} · sts_his ${dash(stsHis)} · sts_ast ${dash(stsAst)} · tgl_hapus ${hapus === "A" ? "kosong" : "terisi"}`;
}
