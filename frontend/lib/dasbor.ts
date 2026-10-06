// Fungsi bantu murni untuk dasbor dan ringkasan (Dashboard Aset dan Pengadaan, Digitalisasi Aset, SAPA): format angka, rupiah ringkas, persen, dan
// waktu. Tidak memakai React atau alias "@/", supaya bisa dites langsung dengan Node.

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

export type Metric = "nilai_buku" | "nilai_perolehan" | "jumlah";

export const METRIC_LABEL: Record<Metric, string> = {
  nilai_buku: "Nilai buku",
  nilai_perolehan: "Nilai perolehan",
  jumlah: "Jumlah aset",
};

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

// Tanggal dan jam dalam zona waktu browser ("3 Okt 2026, 02.10"); waktu sinkronisasi disimpan UTC.
export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString("id-ID", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" });
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

// Bagian-bagian komposisi kondisi aset (satu batang bertumpuk).
export interface KondisiPart {
  key: string;
  label: string;
  tone: KondisiTone;
  value: number;
}
