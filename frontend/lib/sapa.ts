import { api } from "./api";

// SAPA (Sistem Administrasi Pengelolaan Aset): tipe data dan fungsi API untuk alur Penjualan.

export type SapaPeran = "satker" | "kanwil" | "ue1";
export type SapaStatusTahap = "belum" | "draft" | "selesai" | "dilewati";

export interface SapaTahapDef {
  kunci: string;
  peran: SapaPeran;
  label: string;
  keterangan: string;
  jenis: "form" | "eksternal";
  kanal?: string; // aplikasi tempat tahap eksternal dikerjakan (Nadine, SIMAN)
  dokumen?: string[]; // jenis dokumen yang dihasilkan tahap form
  boleh_dilewati: boolean;
  wajib_nomor_tanggal?: boolean;
}

export interface SapaItemDokumen {
  kunci: string;
  label: string;
  otomatis: boolean; // disusun aplikasi dalam berkas Nota Dinas
}

export interface SapaSaya {
  nama: string;
  admin: boolean;
  peran: SapaPeran | "";
  peran_label: string;
  kode_satker: string;
  kode_ue1: string;
  punya_akses: boolean;
  alasan?: string;
  boleh_membuat: boolean;
  tahap: SapaTahapDef[];
  jenis_tim: string[];
  bentuk: string[];
  item_dokumen: SapaItemDokumen[];
}

export interface SapaUsulan {
  id: number;
  noreg: string;
  kode_satker: string;
  nama_satker: string;
  kode_ue1: string;
  dibuat_oleh: string;
  dibuat_pada: string;
  diperbarui_pada: string;
}

export interface SapaRingkasan extends SapaUsulan {
  tahap_saat_ini: string;
  tahap_saat_ini_label: string;
  peran_saat_ini: SapaPeran | "";
  selesai: boolean;
  tahap_selesai: number;
  tahap_total: number;
}

export interface SapaHalaman {
  usulan: SapaRingkasan[];
  total: number;
  halaman: number;
  per_halaman: number;
}

export interface SapaDokumen {
  id: number;
  penjualan_id: number;
  tahap: string;
  jenis: string;
  jenis_label: string;
  nama_file: string;
  ukuran: number;
  peringatan?: string[];
  dibuat_oleh: string;
  dibuat_pada: string;
}

// Isian formulir tiap tahap. Semua nilai berupa teks (angka dan tanggal dikirim apa adanya, divalidasi backend).
export interface SapaAnggota {
  nama: string;
  jabatan: string;
  kedudukan: string;
}

export interface SapaDataTim {
  jabatan_pimpinan: string;
  jenis_tim: string;
  masa_awal: string;
  masa_akhir: string;
  kota: string;
  anggota: SapaAnggota[];
}

export interface SapaDataBA {
  bentuk: string;
  tanggal_penelitian: string;
  nama_tim: string;
}

export interface SapaBarang {
  nama: string;
  kode: string;
  nup: string;
  lokasi: string;
  kondisi: string;
  tahun_perolehan: string;
  nilai_perolehan: string;
  nilai_limit: string;
  keterangan: string;
}

export interface SapaPenandatangan {
  nama: string;
  nip: string;
  jabatan: string;
}

export interface SapaDokPendukung {
  ada: boolean;
  nomor: string;
  tanggal: string;
}

export interface SapaDataNDSatker {
  sudah_rp4: boolean;
  tujuan_surat: string;
  kota: string;
  singkatan_satker: string;
  jenis_bmn: string;
  satuan: string;
  alasan: string;
  tiket_siman: string;
  kepala_kanwil: string;
  penandatangan: SapaPenandatangan;
  barang: SapaBarang[];
  dokumen: Record<string, SapaDokPendukung>;
}

export interface SapaDataNDUE1 {
  nomor_nd: string;
  tanggal_nd: string;
  hal_nd: string;
  sekretaris_ue1: string;
  kepala_kanwil: string;
  pejabat_pengelola: string;
  penandatangan: SapaPenandatangan;
}

// Pada detail usulan, "dokumen" berisi dokumen hasil (bukan jenis dokumen seperti pada definisi tahap); jenis yang
// dihasilkan tahap form ada di jenis_dokumen.
export interface SapaTahapDetail extends Omit<SapaTahapDef, "dokumen"> {
  urutan: number;
  jenis_dokumen?: string;
  status: SapaStatusTahap;
  dapat_dikerjakan: boolean; // semua tahap sebelumnya sudah selesai/dilewati
  alasan_terkunci?: string;
  boleh_aksi: boolean; // peran pengguna sesuai dengan tahap
  dapat_diubah: boolean; // belum ada tahap sesudahnya yang selesai
  data?: unknown; // isian tersimpan (draf atau yang dipakai membuat dokumen)
  saran?: unknown; // nilai awal formulir bila belum ada isian tersimpan
  nomor?: string;
  tanggal?: string;
  catatan?: string;
  diperbarui_oleh?: string;
  diperbarui_pada?: string;
  dokumen: SapaDokumen[];
  template_tersedia?: Record<string, boolean>;
}

export interface SapaDetail {
  usulan: SapaUsulan;
  tahap: SapaTahapDetail[];
  tahap_saat_ini: string;
  selesai: boolean;
}

export interface SapaPenandaInfo {
  nama: string;
  keterangan: string;
  ulang?: boolean;
}

export interface SapaJenisDokumen {
  kunci: string;
  label: string;
  tahap: string;
  bawaan: boolean;
  catatan?: string;
  penanda: SapaPenandaInfo[];
}

export interface SapaTemplateInfo {
  id: number;
  kunci: string;
  versi: number;
  nama_file: string;
  ukuran: number;
  aktif: boolean;
  catatan?: string;
  diunggah_oleh: string;
  diunggah_pada: string;
}

export interface SapaStatusTemplate {
  jenis: SapaJenisDokumen;
  sumber: "unggahan" | "bawaan" | "belum";
  aktif?: SapaTemplateInfo;
  riwayat: SapaTemplateInfo[];
}

export interface SapaHasilUnggah {
  template: SapaTemplateInfo;
  penanda: string[];
  tidak_dikenal: string[];
  tidak_dipakai: string[];
  peringatan: string[];
}

export interface SapaPeranRow {
  user_id: string;
  username: string;
  nama: string;
  email: string;
  peran_app: string;
  peran: SapaPeran | "";
  kode_satker: string;
  kode_ue1: string;
}

export interface SapaRefUE1 {
  kode: string;
  nama: string;
  sekretaris: string;
}

export interface SapaSatkerRef {
  satker: { kode: string; nama: string; kab_kota: string; provinsi: string; kode_ue1: string } | null;
  ue1: SapaRefUE1 | null;
  kode_ue1: string;
}

type SapaRes<T> = { success: boolean; message: string; data: T };

export async function getSapaSaya() {
  const res = await api.get<SapaRes<SapaSaya>>("/sapa/saya");
  return res.data;
}

export async function cariSapaSatker(kode: string) {
  const res = await api.get<SapaRes<SapaSatkerRef>>("/sapa/referensi/satker", { params: { kode } });
  return res.data;
}

export async function listSapaPenjualan(params: { q?: string; status?: string; halaman?: number; per_halaman?: number }) {
  const res = await api.get<SapaRes<SapaHalaman>>("/sapa/penjualan", { params });
  return res.data;
}

export async function createSapaPenjualan(body: { kode_satker: string; nama_satker?: string }) {
  const res = await api.post<SapaRes<SapaUsulan>>("/sapa/penjualan", body);
  return res.data;
}

export async function getSapaPenjualan(id: number) {
  const res = await api.get<SapaRes<SapaDetail>>(`/sapa/penjualan/${id}`);
  return res.data;
}

export async function saveSapaDraf(id: number, tahap: string, data: unknown) {
  const res = await api.put<SapaRes<null>>(`/sapa/penjualan/${id}/tahap/${tahap}`, data);
  return res.data;
}

export async function generateSapaDokumen(id: number, tahap: string, data: unknown) {
  const res = await api.post<SapaRes<{ dokumen: SapaDokumen; peringatan: string[] }>>(`/sapa/penjualan/${id}/tahap/${tahap}/dokumen`, data);
  return res.data;
}

export async function completeSapaTahap(id: number, tahap: string, body: { nomor: string; tanggal: string; catatan: string }) {
  const res = await api.post<SapaRes<null>>(`/sapa/penjualan/${id}/tahap/${tahap}/selesai`, body);
  return res.data;
}

export async function skipSapaTahap(id: number, tahap: string, catatan: string) {
  const res = await api.post<SapaRes<null>>(`/sapa/penjualan/${id}/tahap/${tahap}/lewati`, { catatan });
  return res.data;
}

export async function reopenSapaTahap(id: number, tahap: string) {
  const res = await api.post<SapaRes<null>>(`/sapa/penjualan/${id}/tahap/${tahap}/buka-ulang`);
  return res.data;
}

// Unduhan membutuhkan header Authorization, jadi berkas diambil lewat axios lalu disimpan dari blob.
async function unduhBerkas(url: string): Promise<{ blob: Blob; disposition: string }> {
  const res = await api.get<Blob>(url, { responseType: "blob" });
  return { blob: res.data, disposition: String(res.headers["content-disposition"] ?? "") };
}

export const unduhSapaDokumen = (id: number) => unduhBerkas(`/sapa/dokumen/${id}/unduh`);
export const unduhSapaTemplate = (kunci: string) => unduhBerkas(`/sapa/template/${kunci}/unduh`);

export async function listSapaTemplate() {
  const res = await api.get<SapaRes<SapaStatusTemplate[]>>("/sapa/template");
  return res.data;
}

export async function uploadSapaTemplate(kunci: string, berkas: File, catatan: string) {
  const form = new FormData();
  form.append("berkas", berkas);
  if (catatan.trim()) form.append("catatan", catatan.trim());
  // Content-Type dikosongkan supaya browser memasang batas multipart sendiri.
  const res = await api.post<SapaRes<SapaHasilUnggah>>(`/sapa/template/${kunci}`, form, { headers: { "Content-Type": undefined } });
  return res.data;
}

export async function listSapaPeran(q: string) {
  const res = await api.get<SapaRes<SapaPeranRow[]>>("/sapa/peran", { params: { q } });
  return res.data;
}

export async function setSapaPeran(userId: string, body: { peran: SapaPeran | ""; kode_satker: string; kode_ue1: string }) {
  const res = await api.put<SapaRes<null>>(`/sapa/peran/${userId}`, body);
  return res.data;
}

export async function listSapaRefUE1() {
  const res = await api.get<SapaRes<SapaRefUE1[]>>("/sapa/ref-ue1");
  return res.data;
}

export async function saveSapaRefUE1(r: SapaRefUE1) {
  const res = await api.put<SapaRes<null>>(`/sapa/ref-ue1/${r.kode}`, { nama: r.nama, sekretaris: r.sekretaris });
  return res.data;
}

export async function deleteSapaRefUE1(kode: string) {
  const res = await api.delete<SapaRes<null>>(`/sapa/ref-ue1/${kode}`);
  return res.data;
}
