import { api } from "./api";

// Log audit aktivitas pengguna (khusus superadmin). Pembantu murni (filter, format waktu, terjemahan rincian) ada di lib/auditFilter.ts.

export interface EntriAudit {
  id: number;
  waktu: string; // ISO UTC
  request_id?: string;
  user_id?: string;
  username?: string;
  nama_lengkap?: string;
  peran?: string;
  kode_peran?: string;
  kategori: string;
  aksi: string;
  label: string;
  metode: string;
  rute: string;
  objek_tipe?: string;
  objek_id?: string;
  status_http: number;
  sukses: boolean;
  durasi_ms: number;
  ip?: string;
  user_agent?: string;
  detail?: Record<string, unknown>;
}

export interface DaftarAudit {
  entri: EntriAudit[];
  total: number;
  halaman: number;
  per_halaman: number;
}

export interface JumlahPer {
  kode: string;
  label?: string;
  jumlah: number;
}

export interface TitikWaktu {
  waktu: string;
  jumlah: number;
  gagal: number;
}

export interface RingkasanAudit {
  dari: string;
  sampai: string;
  total: number;
  berhasil: number;
  gagal: number;
  pengguna_aktif: number;
  login_berhasil: number;
  login_gagal: number;
  ditolak: number;
  ekspor: number;
  per_kategori: JumlahPer[];
  aksi_teratas: JumlahPer[];
  login_gagal_ip: JumlahPer[];
  deret: TitikWaktu[];
  per_jam: boolean;
}

export interface StatistikPerekam {
  diterima: number;
  ditulis: number;
  dijatuhkan: number;
  gagal: number;
  antrean: number;
  kapasitas: number;
}

export interface ResAuditRingkasan {
  ringkasan: RingkasanAudit;
  perekam: StatistikPerekam;
  retensi_hari: number;
}

export interface RingkasanPengguna {
  user_id: string;
  username: string;
  nama_lengkap?: string;
  email?: string;
  aktif?: boolean; // tidak ada bila akunnya sudah dihapus
  jumlah: number;
  gagal: number;
  aktif_terakhir: string;
  login_terakhir?: string;
  ip_terakhir?: string;
  peran_terakhir?: string;
}

export interface DaftarPenggunaAudit {
  pengguna: RingkasanPengguna[];
  total: number;
  halaman: number;
  per_halaman: number;
}

export interface OpsiAudit {
  kategori: { kode: string; label: string }[];
  aksi: { kode: string; label: string; kategori: string }[];
}

interface Res<T> {
  success: boolean;
  message: string;
  data: T;
}

export type KueriAudit = Record<string, string>;

export async function getAuditLog(kueri: KueriAudit, signal?: AbortSignal) {
  const res = await api.get<Res<DaftarAudit>>("/audit", { params: kueri, signal });
  return res.data;
}

export async function getAuditRingkasan(kueri: KueriAudit, signal?: AbortSignal) {
  const res = await api.get<Res<ResAuditRingkasan>>("/audit/ringkasan", { params: kueri, signal });
  return res.data;
}

export async function getAuditPengguna(kueri: KueriAudit, signal?: AbortSignal) {
  const res = await api.get<Res<DaftarPenggunaAudit>>("/audit/pengguna", { params: kueri, signal });
  return res.data;
}

export async function getAuditOpsi() {
  const res = await api.get<Res<OpsiAudit>>("/audit/opsi");
  return res.data;
}

// Ekspor CSV butuh header Authorization, jadi diambil lewat axios lalu disimpan dari blob oleh pemanggil.
export async function unduhAuditCsv(kueri: KueriAudit): Promise<{ blob: Blob; nama: string }> {
  const res = await api.get<Blob>("/audit/ekspor", { params: kueri, responseType: "blob", timeout: 120_000 });
  const m = /filename="?([^";]+)"?/i.exec(String(res.headers["content-disposition"] ?? ""));
  return { blob: res.data, nama: m?.[1] ?? "log-audit.csv" };
}
