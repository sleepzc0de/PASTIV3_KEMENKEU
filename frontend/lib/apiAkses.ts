import { api } from "./api";

// API Akses Data PASTI: pendaftaran aplikasi klien (publik), pengelolaan oleh superadmin, dan pemantauan penarikan data. Fungsi murni (label, format, validasi)
// ada di lib/apiAksesFormat.ts.

export type StatusKlien = "menunggu" | "disetujui" | "ditolak" | "dicabut";

export interface BatasEfektif {
  limit_permintaan: number;
  limit_jendela_detik: number;
  kuota_permintaan: number;
  kuota_periode_detik: number;
}

export interface InfoDataset {
  kode: string;
  label: string;
  deskripsi: string;
}

export interface KunciInfo {
  id: number;
  prefix: string;
  label?: string;
  dibuat_at: string;
  dibuat_oleh?: string;
  kedaluwarsa_at?: string;
  dicabut_at?: string;
  dicabut_oleh?: string;
  alasan_cabut?: string;
  terakhir_dipakai_at?: string;
  terakhir_ip?: string;
  aktif: boolean;
}

export interface Klien {
  id: number;
  kode: string;
  nama_aplikasi: string;
  instansi: string;
  unit?: string;
  pic_nama: string;
  pic_jabatan?: string;
  pic_email: string;
  pic_telepon?: string;
  tujuan: string;
  dataset_diminta: string[];
  dataset_diberikan: string[];
  status: StatusKlien;
  catatan?: string;
  ip_diizinkan: string[];
  ue1_diizinkan: string[];
  didaftarkan_at: string;
  didaftarkan_ip?: string;
  diputuskan_at?: string;
  diputuskan_oleh?: string;
  diubah_at?: string;
  limit_permintaan: number | null;
  limit_jendela_detik: number | null;
  kuota_permintaan: number | null;
  kuota_periode_detik: number | null;
  batas_berlaku?: BatasEfektif;
  kunci_aktif: number;
  terakhir_dipakai?: string;
  kunci?: KunciInfo[];
}

export interface PemakaianKlien {
  klien_id: number;
  kode: string;
  nama: string;
  batas: BatasEfektif;
  limit_sisa: number;
  kuota_terpakai: number;
  kuota_sisa: number;
  kuota_reset: string;
  kuota_persen: number;
  override: boolean;
}

export interface Pengaturan {
  limit_permintaan: number;
  limit_jendela_detik: number;
  kuota_permintaan: number;
  kuota_periode_detik: number;
  pendaftaran_dibuka: boolean;
  kunci_masa_berlaku_hari: number;
  rotasi_tenggang_jam: number;
  halaman_maks: number;
  log_retensi_hari: number;
  agregat_retensi_hari: number;
}

export type RentangPengaturan = Record<string, { min: number; maks: number }>;

export interface Metrik {
  jumlah: number;
  sukses: number;
  galat_klien: number;
  ditolak: number;
  batas_laju: number;
  kuota_habis: number;
  galat_server: number;
  baris: number;
  lat_rata_ms: number;
  lat_maks_ms: number;
  lat_p50_ms: number;
  lat_p95_ms: number;
}

export interface TitikDeret {
  waktu: string;
  jumlah: number;
  gagal: number;
  dibatasi: number;
  lat_rata_ms: number;
}

export interface Kelompok extends Metrik {
  kunci: string;
  klien_id?: number;
  kode?: string;
  nama?: string;
}

export interface StatistikPencatat {
  diterima: number;
  ditulis: number;
  dijatuhkan: number;
  gagal: number;
  antrean: number;
  kapasitas: number;
}

export interface Pemantauan {
  dari: string;
  sampai: string;
  per_hari: boolean;
  ringkasan: Metrik;
  klien_aktif: number;
  deret: TitikDeret[];
  per_klien: Kelompok[];
  per_endpoint: Kelompok[];
  pemakaian: PemakaianKlien[];
  pencatat: StatistikPencatat;
  pengaturan: Pengaturan;
}

export interface RiwayatPermintaan {
  id: number;
  waktu: string;
  klien_id?: number;
  klien_kode?: string;
  klien_nama?: string;
  kunci_prefix?: string;
  request_id?: string;
  metode: string;
  endpoint: string;
  dataset?: string;
  status_http: number;
  hasil: string;
  durasi_ms: number;
  baris?: number;
  ip?: string;
  user_agent?: string;
}

export interface DaftarRiwayat {
  riwayat: RiwayatPermintaan[];
  total: number;
  halaman: number;
  per_halaman: number;
  retensi_hari: number;
}

export interface OpsiPantau {
  klien: { id: number; kode: string; nama: string; status: StatusKlien }[];
  endpoint: { kode: string; label: string }[];
  hasil: { kode: string; label: string }[];
}

export interface KunciBaru {
  kunci: string;
  info: KunciInfo;
}

interface Res<T> {
  success: boolean;
  message: string;
  data: T;
}

export interface FormPendaftaran {
  nama_aplikasi: string;
  instansi: string;
  unit: string;
  pic_nama: string;
  pic_jabatan: string;
  pic_email: string;
  pic_telepon: string;
  tujuan: string;
  dataset_diminta: string[];
}

// ---------------------------------------------------------------- publik

export async function getInfoPendaftaran() {
  const res = await api.get<Res<{ dataset: InfoDataset[]; pendaftaran_dibuka: boolean }>>("/api-akses/info");
  return res.data.data;
}

export async function kirimPendaftaran(form: FormPendaftaran, captchaId: string, captchaAnswer: string) {
  const res = await api.post<Res<{ kode: string }>>("/api-akses/pendaftaran", { ...form, captcha_id: captchaId, captcha_answer: captchaAnswer });
  return res.data;
}

// ---------------------------------------------------------------- pengelolaan (superadmin)

export interface DaftarKlien {
  klien: Klien[];
  total: number;
  halaman: number;
  per_status: Record<StatusKlien, number>;
  dataset: InfoDataset[];
}

export async function getDaftarKlien(params: Record<string, string | number>, signal?: AbortSignal) {
  const res = await api.get<Res<DaftarKlien>>("/api-akses/klien", { params, signal });
  return res.data.data;
}

export async function getDetailKlien(id: number) {
  const res = await api.get<Res<{ klien: Klien; pemakaian: PemakaianKlien }>>(`/api-akses/klien/${id}`);
  return res.data.data;
}

export interface BodyCakupan {
  dataset_diberikan: string[];
  ip_diizinkan: string;
  ue1_diizinkan: string;
  limit_permintaan: number | null;
  limit_jendela_detik: number | null;
  kuota_permintaan: number | null;
  kuota_periode_detik: number | null;
  catatan: string;
}

export async function putKlien(id: number, body: BodyCakupan) {
  const res = await api.put<Res<{ klien: Klien }>>(`/api-akses/klien/${id}`, body);
  return res.data;
}

export async function postSetujui(id: number, body: BodyCakupan & { label_kunci: string }) {
  const res = await api.post<Res<{ klien: Klien; kunci_baru?: KunciBaru; kunci_gagal?: boolean }>>(`/api-akses/klien/${id}/setujui`, body);
  return res.data;
}

export async function postTolak(id: number, catatan: string) {
  const res = await api.post<Res<{ klien: Klien }>>(`/api-akses/klien/${id}/tolak`, { catatan });
  return res.data;
}

export async function postCabut(id: number, catatan: string) {
  const res = await api.post<Res<{ klien: Klien }>>(`/api-akses/klien/${id}/cabut`, { catatan });
  return res.data;
}

export async function postAktifkan(id: number, catatan: string) {
  const res = await api.post<Res<{ klien: Klien; kunci_baru?: KunciBaru; kunci_gagal?: boolean }>>(`/api-akses/klien/${id}/aktifkan`, { catatan });
  return res.data;
}

export async function postKunci(id: number, label: string, rotasi: boolean) {
  const res = await api.post<Res<KunciBaru>>(`/api-akses/klien/${id}/kunci`, { label, rotasi });
  return res.data;
}

export async function deleteKunci(id: number, kunciId: number, alasan: string) {
  const res = await api.delete<Res<{ prefix: string }>>(`/api-akses/klien/${id}/kunci/${kunciId}`, { params: alasan ? { alasan } : undefined });
  return res.data;
}

export async function getPengaturan() {
  const res = await api.get<Res<{ pengaturan: Pengaturan; bawaan_pabrik: Pengaturan; rentang: RentangPengaturan }>>("/api-akses/pengaturan");
  return res.data.data;
}

export async function putPengaturan(p: Pengaturan) {
  const res = await api.put<Res<{ pengaturan: Pengaturan }>>("/api-akses/pengaturan", p);
  return res.data;
}

// ---------------------------------------------------------------- pemantauan (superadmin)

export async function getPantau(params: Record<string, string>, signal?: AbortSignal) {
  const res = await api.get<Res<Pemantauan>>("/api-akses/pantau/ringkasan", { params, signal });
  return res.data.data;
}

export async function getRiwayat(params: Record<string, string | number>, signal?: AbortSignal) {
  const res = await api.get<Res<DaftarRiwayat>>("/api-akses/pantau/riwayat", { params, signal });
  return res.data.data;
}

export async function getOpsiPantau() {
  const res = await api.get<Res<OpsiPantau>>("/api-akses/pantau/opsi");
  return res.data.data;
}
