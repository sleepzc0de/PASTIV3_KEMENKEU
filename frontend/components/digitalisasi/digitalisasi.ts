// Fungsi bantu murni untuk fitur Digitalisasi Aset (tanpa React dan tanpa alias "@/", supaya bisa dites dengan Node).

import type { DGAgg, DGColumn, DGCount, DGDatasetKey, DGProvinsi, DGSyncStatus, DGUE1 } from "@/lib/api";
import { formatNumber } from "../sldk/asset";

export const ASSET_KEYS: DGDatasetKey[] = ["tanah", "gedung_kantor_utama", "gedung_lainnya", "rusunara", "rumah_negara", "mess_rumah_negara"];

export const DATASET_LABEL: Record<DGDatasetKey, string> = {
  satker: "Satuan Kerja",
  tanah: "Tanah",
  gedung_kantor_utama: "Gedung Kantor Utama",
  gedung_lainnya: "Gedung Lainnya",
  rusunara: "Rusunara",
  rumah_negara: "Rumah Negara",
  mess_rumah_negara: "Mess Rumah Negara",
};

export const DATASET_SHORT: Record<DGDatasetKey, string> = {
  satker: "Satker",
  tanah: "Tanah",
  gedung_kantor_utama: "Kantor Utama",
  gedung_lainnya: "Gedung Lain",
  rusunara: "Rusunara",
  rumah_negara: "Rumah Negara",
  mess_rumah_negara: "Mess",
};

// Warna kategori peta: tetap per dataset (warna mengikuti entitas, bukan urutan tampil), diambil dari palet
// kategorikal berurutan. Dengan lebih dari tiga kategori sekaligus, identitas dibawa oleh legenda dan tombol
// pilih-kategori, bukan warna saja.
export const DATASET_COLOR: Record<DGDatasetKey, string> = {
  satker: "#52514e",
  tanah: "#2a78d6",
  gedung_kantor_utama: "#eb6834",
  gedung_lainnya: "#1baf7a",
  rusunara: "#e87ba4",
  rumah_negara: "#4a3aa7",
  mess_rumah_negara: "#eda100",
};

// ---- Kolom tabel daftar (subset yang muat di layar) ----

export interface TableColumn {
  name: string;
  label: string;
}

export const TABLE_COLUMNS: Record<DGDatasetKey, TableColumn[]> = {
  satker: [
    { name: "Kode_Satker", label: "Kode satker" },
    { name: "Nama_Satker", label: "Nama satker" },
    { name: "Jenis_Satker", label: "Jenis" },
    { name: "KabKota_Satker", label: "Kab/Kota" },
    { name: "Provinsi_Satker", label: "Provinsi" },
    { name: "Jumlah_KDJ", label: "KDJ" },
    { name: "Jumlah_KDO", label: "KDO" },
  ],
  tanah: [
    { name: "Nama_Satker", label: "Satker" },
    { name: "Uraian_tanah", label: "Uraian" },
    { name: "KabKota_tanah", label: "Kab/Kota" },
    { name: "Provinsi_tanah", label: "Provinsi" },
    { name: "Luas_Tanah", label: "Luas (m²)" },
    { name: "Kondisi_Tanah", label: "Kondisi" },
    { name: "Nilai_Tanah", label: "Nilai" },
  ],
  gedung_kantor_utama: [
    { name: "Nama_Satker", label: "Satker" },
    { name: "Uraian_bangunan", label: "Uraian" },
    { name: "KabKota_bangunan", label: "Kab/Kota" },
    { name: "Provinsi_bangunan", label: "Provinsi" },
    { name: "Luas_Bangunan", label: "Luas (m²)" },
    { name: "Kondisi_Bangunan", label: "Kondisi" },
    { name: "Nilai_Bangunan", label: "Nilai" },
  ],
  gedung_lainnya: [
    { name: "Nama_Satker", label: "Satker" },
    { name: "Uraian_bangunan", label: "Uraian" },
    { name: "Fungsi", label: "Fungsi" },
    { name: "KabKota_bangunan", label: "Kab/Kota" },
    { name: "Luas_Bangunan", label: "Luas (m²)" },
    { name: "Kondisi_Bangunan", label: "Kondisi" },
    { name: "Nilai_Bangunan", label: "Nilai" },
  ],
  rusunara: [
    { name: "Nama_Satker", label: "Satker" },
    { name: "Uraian_Rusun", label: "Uraian" },
    { name: "KabKota_Rusun", label: "Kab/Kota" },
    { name: "Provinsi_Rusun", label: "Provinsi" },
    { name: "Luas_Rusun", label: "Luas (m²)" },
    { name: "Kondisi_Rusun", label: "Kondisi" },
    { name: "Nilai_Rusun", label: "Nilai" },
  ],
  rumah_negara: [
    { name: "Nama_Satker", label: "Satker" },
    { name: "Uraian_RN", label: "Uraian" },
    { name: "KabKota_RN", label: "Kab/Kota" },
    { name: "Provinsi_RN", label: "Provinsi" },
    { name: "Luas_RN", label: "Luas (m²)" },
    { name: "Status_Penghuni", label: "Penghuni" },
    { name: "Kondisi_RN", label: "Kondisi" },
  ],
  mess_rumah_negara: [
    { name: "Nama_Satker", label: "Satker" },
    { name: "Uraian_mess", label: "Uraian" },
    { name: "KabKota_mess", label: "Kab/Kota" },
    { name: "Provinsi_mess", label: "Provinsi" },
    { name: "Luas_mess", label: "Luas (m²)" },
    { name: "Kondisi_mess", label: "Kondisi" },
    { name: "Nilai_mess", label: "Nilai" },
  ],
};

// Kolom yang jadi judul kartu (layar sempit) dan judul detail.
export const TITLE_COLUMN: Record<DGDatasetKey, string> = {
  satker: "Nama_Satker",
  tanah: "Uraian_tanah",
  gedung_kantor_utama: "Uraian_bangunan",
  gedung_lainnya: "Uraian_bangunan",
  rusunara: "Uraian_Rusun",
  rumah_negara: "Uraian_RN",
  mess_rumah_negara: "Uraian_mess",
};

// ---- Label kolom untuk tampilan detail ----

const TOKEN_LABEL: Record<string, string> = {
  KelurahanDesa: "Kelurahan/Desa",
  KabKota: "Kab/Kota",
  RTRW: "RT/RW",
  UE1: "UE1",
  KDJ: "KDJ",
  KDO: "KDO",
  NUP: "NUP",
  RN: "Rumah Negara",
  id: "ID",
};

// "KelurahanDesa_tanah" -> "Kelurahan/Desa tanah"; "id_aset_tanah" -> "ID aset tanah"
export function columnLabel(name: string): string {
  if (name === "Latitude" || name === "Longitude") return name === "Latitude" ? "Lintang" : "Bujur";
  if (name === "synced_at") return "Disinkronkan";
  if (name === "id_sinkron") return "ID sinkronisasi";
  const label = name
    .split("_")
    .map((t) => TOKEN_LABEL[t] ?? t)
    .join(" ");
  return label.charAt(0).toUpperCase() + label.slice(1);
}

// ---- Format angka ----

export function formatLuas(n: number | null | undefined): string {
  if (n === null || n === undefined) return "-";
  return `${formatNumber(n, 2)} m²`;
}

// Luas ringkas untuk kartu: m² di bawah satu hektare, hektare di atasnya ("1,2 ribu ha" untuk angka besar).
export function compactLuas(n: number): string {
  if (!Number.isFinite(n)) return "-";
  const ha = n / 10000;
  if (Math.abs(ha) < 1) return `${formatNumber(n, 0)} m²`;
  if (Math.abs(ha) >= 1000) return `${formatNumber(ha / 1000, ha >= 10000 ? 0 : 1)} ribu ha`;
  return `${formatNumber(ha, ha < 100 ? 1 : 0)} ha`;
}

export function formatCell(col: DGColumn | undefined, name: string, v: string | number | null | undefined): string {
  if (v === null || v === undefined || v === "") return "-";
  if (typeof v === "number") {
    if (/^Nilai_/.test(name)) return `Rp ${formatNumber(v, v % 1 === 0 ? 0 : 2)}`;
    if (/^Luas_/.test(name)) return formatNumber(v, 2);
    return formatNumber(v, col?.tipe === "desimal" ? 2 : 0);
  }
  return String(v);
}

// ---- Rincian per UE1 / provinsi ----

export type BreakdownMetric = "jumlah" | "luas_tanah" | "nilai" | "satker";

export const BREAKDOWN_LABEL: Record<BreakdownMetric, string> = {
  jumlah: "Jumlah aset",
  luas_tanah: "Luas tanah",
  nilai: "Nilai aset",
  satker: "Jumlah satker",
};

type Per = Partial<Record<DGDatasetKey, DGAgg>>;

export function perJumlah(per: Per): number {
  return ASSET_KEYS.reduce((acc, k) => acc + (per[k]?.jumlah ?? 0), 0);
}

export function perNilai(per: Per): number {
  return ASSET_KEYS.reduce((acc, k) => acc + (per[k]?.nilai ?? 0), 0);
}

export function ue1Value(u: DGUE1, m: BreakdownMetric): number {
  switch (m) {
    case "jumlah":
      return perJumlah(u.per);
    case "luas_tanah":
      return u.per.tanah?.luas ?? 0;
    case "nilai":
      return perNilai(u.per);
    case "satker":
      return u.satker;
  }
}

export function provinsiValue(p: DGProvinsi, m: Exclude<BreakdownMetric, "satker">): number {
  return ue1Value({ kode: "", label: "", satker: 0, kdj: 0, kdo: 0, per: p.per }, m);
}

export function formatBreakdown(m: BreakdownMetric, n: number): string {
  if (m === "luas_tanah") return compactLuas(n);
  return formatNumber(n, 0);
}

// Judul kolom nilai aset: hanya dataset yang punya kolom nilai ikut dijumlahkan.
export const NILAI_KEYS: DGDatasetKey[] = ASSET_KEYS.filter((k) => k !== "rumah_negara");

// ---- Asuransi ----

const YES = new Set(["1", "Y", "YA", "YES", "TRUE"]);
const NO = new Set(["0", "T", "N", "TIDAK", "NO", "FALSE"]);

// is_asuransi belum terdokumentasi (bit, Y/T, atau teks), jadi nilainya dipetakan longgar dan sisanya ditampilkan apa adanya.
export function asuransiLabel(v: string | null): string {
  if (v === null || v.trim() === "") return "Tidak ada data";
  const u = v.trim().toUpperCase();
  if (YES.has(u)) return "Diasuransikan";
  if (NO.has(u)) return "Belum diasuransikan";
  return v;
}

// Gabungkan hitungan beberapa dataset menurut label (mis. asuransi gedung kantor utama + lainnya).
export function mergeCounts(lists: (DGCount[] | undefined)[], label: (k: string | null) => string): { label: string; jumlah: number }[] {
  const map = new Map<string, number>();
  for (const list of lists) {
    for (const c of list ?? []) {
      const l = label(c.k);
      map.set(l, (map.get(l) ?? 0) + c.jumlah);
    }
  }
  return [...map.entries()].map(([label, jumlah]) => ({ label, jumlah })).sort((a, b) => b.jumlah - a.jumlah);
}

// Status hukum datang sebagai gabungan "Uraian [label] | Uraian [label]": dipendekkan untuk label grafik.
export function shortStatusHukum(k: string | null, max = 70): string {
  if (k === null || k.trim() === "") return "(tidak ada data)";
  const t = k.trim().replace(/\s+\|\s+/g, " · ");
  return t.length > max ? `${t.slice(0, max - 1)}…` : t;
}

// ---- Sinkronisasi ----

export type StatusTone = "ok" | "fail" | "run" | "wait" | "stop";

export const STATUS_META: Record<DGSyncStatus, { label: string; tone: StatusTone }> = {
  sukses: { label: "Sukses", tone: "ok" },
  gagal: { label: "Gagal", tone: "fail" },
  berjalan: { label: "Berjalan", tone: "run" },
  antri: { label: "Antri", tone: "wait" },
  dibatalkan: { label: "Dibatalkan", tone: "stop" },
};

// 3725000 -> "1 j 2 m"; di bawah semenit "42 dtk".
export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return "-";
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s} dtk`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m} m ${s % 60} dtk`;
  const h = Math.floor(m / 60);
  return `${h} j ${m % 60} m`;
}

export function durationBetween(start: string | null, end: string | null): number | null {
  if (!start || !end) return null;
  const a = new Date(start).getTime();
  const b = new Date(end).getTime();
  return Number.isNaN(a) || Number.isNaN(b) ? null : b - a;
}

export function elapsedSince(start: string, now: number): number {
  const t = new Date(start).getTime();
  return Number.isNaN(t) ? 0 : Math.max(0, now - t);
}

// Antrean aktif -> posisi tiap dataset: selesai, sedang jalan, menunggu.
export function queueState(datasets: DGDatasetKey[], current: DGDatasetKey | "", key: DGDatasetKey): "selesai" | "jalan" | "menunggu" {
  if (current === "") return "menunggu";
  const ci = datasets.indexOf(current);
  const ki = datasets.indexOf(key);
  if (ki < ci) return "selesai";
  return ki === ci ? "jalan" : "menunggu";
}

// Persentase aman (0 bila pembagi 0), untuk tabel kelengkapan.
export function ratio(part: number, total: number): number {
  return total > 0 ? part / total : 0;
}

export function dataPer(sinkron: { terakhir_sukses: string | null }[]): string | null {
  let best: string | null = null;
  for (const s of sinkron) {
    if (s.terakhir_sukses && (!best || new Date(s.terakhir_sukses) > new Date(best))) best = s.terakhir_sukses;
  }
  return best;
}
