import { api } from "./api";

// Monitor resource (khusus superadmin). Pembantu murni (format angka, status, penyusun grafik) ada di lib/monitorFormat.ts.

export type StatusResource = "cukup" | "perhatian" | "kritis" | "tidak_ada_data";

export interface Kontainer {
  mem_batas: number | null;
  mem_pakai: number | null;
  cpu_batas: number | null;
}

export interface SistemInfo {
  os: string;
  arsitektur: string;
  cpu_jumlah: number;
  cpu_persen: number | null;
  cpu_proses_persen: number | null;
  load1: number | null;
  load5: number | null;
  load15: number | null;
  mem_total: number;
  mem_terpakai: number;
  mem_tersedia: number;
  mem_persen: number;
  swap_total: number;
  swap_terpakai: number;
  disk_jalur: string;
  disk_total: number;
  disk_terpakai: number;
  disk_bebas: number;
  disk_persen: number;
  uptime_detik: number | null;
  kontainer?: Kontainer;
  catatan?: string[];
}

export interface ProsesInfo {
  versi_go: string;
  uptime_detik: number;
  goroutine: number;
  heap_dipakai: number;
  heap_dari_os: number;
  mem_dari_os: number;
  rss: number | null;
  gc_jumlah: number;
  gc_jeda_terakhir_ms: number;
  fd_terbuka: number | null;
}

export interface PoolDB {
  nama: string;
  tersambung: boolean;
  ping_ms: number;
  batas: number;
  terbuka: number;
  dipakai: number;
  menganggur: number;
  menunggu_total: number;
  menunggu_ms_total: number;
  galat?: string;
}

export interface FileDB {
  nama: string;
  jenis: string;
  ukuran_mb: number;
  terpakai_mb: number | null;
  maks_mb: number | null;
}

export interface VolumeDB {
  titik: string;
  total: number;
  bebas: number;
  persen: number;
}

export interface InfoSQL {
  versi: string;
  edisi: string;
  nama_db: string;
  ukuran_mb: number;
  terpakai_mb: number | null;
  file: FileDB[];
  volume: VolumeDB[];
  cpu_jumlah: number | null;
  mem_fisik_mb: number | null;
  mem_proses_mb: number | null;
  page_life_expectancy: number | null;
  buffer_hit_ratio: number | null;
  sesi_pengguna: number | null;
  catatan?: string[];
  diukur_pada: string;
}

export interface DatabaseInfo {
  utama: PoolDB | null;
  sldk?: PoolDB | null;
  sql?: InfoSQL | null;
}

export interface JendelaHTTP {
  nama: string;
  total: number;
  galat_4xx: number;
  galat_5xx: number;
  rata_ms: number;
  p50_ms: number;
  p95_ms: number;
  p99_ms: number;
  per_detik: number;
  persen_galat_5xx: number;
}

export interface RuteLambat {
  rute: string;
  metode: string;
  jumlah: number;
  galat_5xx: number;
  rata_ms: number;
  p95_ms: number;
  maks_ms: number;
  berat: boolean;
}

export interface HttpInfo {
  berjalan: number;
  jendela: JendelaHTTP[];
  rute_lambat: RuteLambat[];
  rute_galat: RuteLambat[];
}

export interface Kapasitas {
  kunci: string;
  nama: string;
  kelompok: "resource" | "kinerja";
  satuan: string;
  sekarang: number | null;
  puncak_24j: number | null;
  puncak_7h: number | null;
  rata_24j: number | null;
  batas: number | null;
  status: StatusResource;
  tindakan: string;
  rincian: string[];
  saran?: string;
}

export interface TugasLatar {
  nama: string;
  berjalan: boolean;
  info?: string;
}

export interface RingkasanMonitor {
  waktu: string;
  status: StatusResource;
  perlu_ditambah: string[];
  sistem: SistemInfo;
  proses: ProsesInfo;
  database: DatabaseInfo;
  http: HttpInfo;
  kapasitas: Kapasitas[];
  tugas: TugasLatar[];
  riwayat_24j?: { jumlah: number } | null;
  riwayat_7h?: { jumlah: number } | null;
  pengukuran: { aktif: boolean; interval_detik: number; mulai_pada: string; retensi_hari: number };
}

export interface TitikRiwayat {
  waktu: string;
  cpu_rata: number | null;
  cpu_maks: number | null;
  mem_persen: number | null;
  disk_persen: number | null;
  load1: number | null;
  rss: number | null;
  heap: number | null;
  goroutine: number | null;
  db_dipakai: number | null;
  db_tunggu: number | null;
  db_ukuran_mb: number | null;
  req_total: number | null;
  req_5xx: number | null;
  lat_rata_ms: number | null;
  lat_p95_ms: number | null;
}

export type RentangRiwayat = "1j" | "24j" | "7h" | "30h";

export interface RiwayatMonitor {
  rentang: RentangRiwayat;
  dari: string;
  sampai: string;
  lebar_titik_menit: number;
  titik: TitikRiwayat[];
}

export interface TabelDB {
  nama: string;
  baris: number;
  ukuran_mb: number;
}

interface Res<T> {
  success: boolean;
  message: string;
  data: T;
}

export async function getMonitorRingkasan(signal?: AbortSignal) {
  const res = await api.get<Res<RingkasanMonitor>>("/monitor/ringkasan", { signal });
  return res.data;
}

export async function getMonitorRiwayat(rentang: RentangRiwayat, signal?: AbortSignal) {
  const res = await api.get<Res<RiwayatMonitor>>("/monitor/riwayat", { params: { rentang }, signal });
  return res.data;
}

export async function getMonitorTabel(signal?: AbortSignal) {
  const res = await api.get<Res<{ tabel: TabelDB[]; catatan: string }>>("/monitor/database", { signal });
  return res.data;
}
