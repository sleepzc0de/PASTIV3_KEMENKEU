import { api } from "./api";

// Log server (khusus superadmin): hanya galat (error) dan peringatan (warning). Pembantu murni (filter, format, grafik) ada di lib/logServerFilter.ts.

export type LevelLog = "error" | "warn";

export interface BarisLog {
  id: number;
  waktu: string; // kejadian pertama pada baris ini (ISO UTC)
  waktu_akhir: string; // kejadian terakhir
  level: LevelLog;
  sumber: string;
  pesan: string;
  detail?: string; // jejak tumpukan (hanya pada rincian)
  jumlah: number; // berapa kali kejadian yang sama digabung pada baris ini
  rute?: string;
  metode?: string;
  status?: number;
  durasi_ms?: number;
  request_id?: string;
  user_id?: string;
  sidik: string;
}

export interface DaftarLog {
  baris: BarisLog[];
  total: number;
  halaman: number;
  per_halaman: number;
}

export interface TitikJamLog {
  jam: string;
  error: number;
  warn: number;
}

export interface JumlahSumberLog {
  sumber: string;
  level: LevelLog;
  jumlah: number;
}

export interface RingkasanLog {
  error_24_jam: number;
  warn_24_jam: number;
  error_7_hari: number;
  warn_7_hari: number;
  kelompok_24_jam: number;
  total_baris: number;
  terakhir?: string;
  per_jam: TitikJamLog[];
  sumber: JumlahSumberLog[];
  paling_berulang: BarisLog[];
}

export interface StatistikPerekamLog {
  diterima: number;
  ditulis: number;
  dijatuhkan: number;
  gagal: number;
  antrean: number;
  kapasitas: number;
  tertunda: number;
  dibatalkan_klien: number;
}

export interface PengaturanLog {
  level: string;
  simpan: string; // warn (error + warning), error, atau off
  retensi_error_hari: number;
  retensi_warn_hari: number;
  maks_baris: number;
  lambat_ms: number;
}

export interface KetahananServer {
  ditolak_laju: number;
  ditolak_beban: number;
  ditolak_berat: number;
  serentak_sekarang: number;
  serentak_puncak: number;
  berat_sekarang: number;
  berat_antri: number;
  maks_serentak: number;
  maks_berat: number;
  laju_aktif: boolean;
}

export interface ResRingkasanLog {
  ringkasan: RingkasanLog;
  perekam: StatistikPerekamLog;
  pengaturan: PengaturanLog;
  ketahanan: KetahananServer;
}

export interface OpsiLog {
  sumber: string[];
  level: LevelLog[];
}

interface Res<T> {
  success: boolean;
  message: string;
  data: T;
}

export type KueriLog = Record<string, string>;

export async function getServerLog(kueri: KueriLog, signal?: AbortSignal) {
  const res = await api.get<Res<DaftarLog>>("/log-server", { params: kueri, signal });
  return res.data;
}

export async function getServerLogRingkasan(signal?: AbortSignal) {
  const res = await api.get<Res<ResRingkasanLog>>("/log-server/ringkasan", { signal });
  return res.data;
}

export async function getServerLogOpsi() {
  const res = await api.get<Res<OpsiLog>>("/log-server/opsi");
  return res.data;
}

export async function getServerLogDetail(id: number, signal?: AbortSignal) {
  const res = await api.get<Res<BarisLog>>(`/log-server/${id}`, { signal });
  return res.data;
}
