import axios from "axios";
import Cookies from "js-cookie";

export const api = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_URL,
  headers: { "Content-Type": "application/json" },
});

api.interceptors.request.use((config) => {
  const token = Cookies.get("pasti_access_token");
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      Cookies.remove("pasti_access_token");
      if (typeof window !== "undefined") {
        window.location.href = "/login";
      }
    }
    return Promise.reject(error);
  }
);

// ============ Auth ============

export interface LoginResponse {
  success: boolean;
  message: string;
  data: {
    access_token: string;
    expires_in: number;
    user: {
      id: string;
      username: string;
      email: string;
      full_name: string;
      role: string;
    };
  };
}

export interface CaptchaResponse {
  success: boolean;
  message: string;
  data: {
    captcha_id: string;
    captcha_image: string;
  };
}

export async function fetchCaptcha() {
  const res = await api.get<CaptchaResponse>("/auth/captcha");
  return res.data;
}

export async function loginUser(
  username: string,
  password: string,
  captchaId: string,
  captchaAnswer: string
) {
  const res = await api.post<LoginResponse>("/auth/login", {
    username,
    password,
    captcha_id: captchaId,
    captcha_answer: captchaAnswer,
  });
  return res.data;
}

// ============ SLDK Integration ============

export interface SLDKColumn {
  name: string;
  data_type: string;
  is_searchable: boolean;
}

export interface SLDKSearchResponse {
  success: boolean;
  message: string;
  data: {
    columns: SLDKColumn[];
    results: Record<string, unknown>[];
    count: number;
  };
}

export async function searchSLDKAssets(query: string, limit = 50) {
  const res = await api.get<SLDKSearchResponse>("/sldk/assets/search", {
    params: { q: query, limit },
  });
  return res.data;
}

// ============ HRIS2 Integration ============

export interface HRIS2SearchResponse {
  success: boolean;
  message: string;
  data: unknown; // struktur respons HRIS2 belum terdokumentasi, ditangani fleksibel di komponen
}

export async function searchHRIS2Pegawai(query: string) {
  const res = await api.get<HRIS2SearchResponse>("/hris2/pegawai/search", {
    params: { q: query },
  });
  return res.data;
}

// ============ User Management ============

export interface UserListItem {
  id: string;
  username: string;
  email: string;
  full_name: string;
  role: string;
  is_active: boolean;
  auth_provider: string;
  is_protected: boolean;
  nip: string | null;
  jabatan: string | null;
  satker: string | null;
  created_at: string;
}

export interface ListUsersResponse {
  success: boolean;
  message: string;
  data: UserListItem[];
}

export async function listUsers() {
  const res = await api.get<ListUsersResponse>("/users");
  return res.data;
}

export interface CreateUserPayload {
  source: "hris2" | "manual";
  nip?: string;
  username: string;
  password: string;
  email?: string;
  full_name?: string;
  role: "user" | "admin";
}

export async function createUser(payload: CreateUserPayload) {
  const res = await api.post("/users", payload);
  return res.data;
}

export async function searchPegawaiByNIP(nip: string) {
  const res = await api.get<{ success: boolean; message: string; data: Record<string, unknown> }>(
    `/hris2/pegawai/by-nip/${encodeURIComponent(nip)}`
  );
  return res.data;
}

export async function updateUserRole(userId: string, role: string) {
  const res = await api.put(`/users/${userId}/role`, { role });
  return res.data;
}

export async function deactivateUser(userId: string) {
  const res = await api.put(`/users/${userId}/deactivate`);
  return res.data;
}

export async function deleteUser(userId: string) {
  const res = await api.delete(`/users/${userId}`);
  return res.data;
}

export async function getUserDetail(userId: string) {
  const res = await api.get<{ success: boolean; message: string; data: UserListItem }>(`/users/${userId}`);
  return res.data;
}

export interface UpdateUserPayload {
  full_name: string;
  email: string;
  role: "user" | "admin";
  is_active: boolean;
  password?: string;
}

export async function updateUser(userId: string, payload: UpdateUserPayload) {
  const res = await api.put(`/users/${userId}`, payload);
  return res.data;
}

// ============ Inaproc Integration (RUP - History Kaji Ulang) ============

export interface KajiUlangItem {
  datamart_id: string;
  tahun_anggaran: string;
  kd_klpd: string;
  nama_klpd: string;
  jenis_klpd: string;
  kd_satker: string;
  kd_satker_str: string;
  nama_satker: string;
  kd_rup_lama: string;
  kd_rup_baru: string;
  jenis_paket: string;
  jenis_revisi: string;
  alasan_kajiulang: string;
  tgl_kaji_ulang: string;
  _event_date: string;
  _inserted_date: string;
}

export interface InaprocMeta {
  limit: number;
  has_more: boolean;
  cursor: string | null;
}

export interface HistoryKajiUlangResponse {
  success: boolean;
  data: KajiUlangItem[] | null;
  meta: InaprocMeta;
}

export interface HistoryKajiUlangParams {
  kode_klpd?: string;
  tahun: number;
  jenis_paket?: string;
  limit?: number;
  cursor?: string;
}

export async function getHistoryKajiUlang(params: HistoryKajiUlangParams) {
  const res = await api.get<HistoryKajiUlangResponse>("/inaproc/rup/history-kaji-ulang", {
    params,
  });
  return res.data;
}

// ============ Inaproc Sync ============

export interface SyncKajiUlangPayload {
  kode_klpd: string;
  tahun: string;
  jenis_paket?: string;
}

export async function syncHistoryKajiUlang(payload: SyncKajiUlangPayload) {
  const res = await api.post("/inaproc/rup/history-kaji-ulang/sync", payload);
  return res.data;
}

export async function listLocalKajiUlang(params: HistoryKajiUlangParams) {
  const res = await api.get<{ success: boolean; data: { results: Record<string, unknown>[]; count: number } }>(
    "/inaproc/rup/history-kaji-ulang/local",
    { params }
  );
  return res.data;
}

export async function getInaprocSyncLog() {
  const res = await api.get<{ success: boolean; data: Record<string, unknown>[] }>("/inaproc/sync-log");
  return res.data;
}

// ============ Inaproc - Paket Anggaran Penyedia ============

export interface PaketAnggaranItem {
  asal_dana: string;
  asal_dana_klpd: string;
  asal_dana_satker: string;
  jenis_klpd: string;
  kd_kegiatan: number;
  kd_klpd: string;
  kd_komponen: number;
  kd_rup: number;
  kd_rup_lokal: number;
  kd_satker: number;
  kd_satker_str: string;
  kd_subkegiatan: number;
  mak: string;
  nama_klpd: string;
  nama_satker: string;
  pagu: number;
  status_aktif_rup: boolean;
  status_delete_rup: boolean;
  status_umumkan_rup: string;
  sumber_dana: string;
  tahun_anggaran: number;
  tahun_anggaran_dana: number;
}

export interface PaketAnggaranResponse {
  success: boolean;
  data: PaketAnggaranItem[] | null;
  meta: InaprocMeta;
}

export interface PaketAnggaranParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getPaketAnggaranPenyedia(params: PaketAnggaranParams) {
  const res = await api.get<PaketAnggaranResponse>("/inaproc/rup/paket-anggaran-penyedia", { params });
  return res.data;
}

export interface SyncPaketAnggaranPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncPaketAnggaranPenyedia(payload: SyncPaketAnggaranPayload) {
  const res = await api.post("/inaproc/rup/paket-anggaran-penyedia/sync", payload);
  return res.data;
}

// ============ Inaproc - Paket Penyedia ============

export interface PaketPenyediaItem {
  datamart_id?: string;
  tahun_anggaran: string;
  kd_klpd: string;
  nama_klpd: string;
  kd_satker: string;
  nama_satker: string;
  kd_rup: string;
  nama_paket: string;
  pagu: string;
  metode_pengadaan: string;
  jenis_pengadaan: string;
  status_umumkan_rup: string;
  nama_ppk: string;
  tgl_awal_pemilihan: string;
  tgl_akhir_pemilihan: string;
}

export interface PaketPenyediaResponse {
  success: boolean;
  data: PaketPenyediaItem[] | null;
  meta: InaprocMeta;
}

export interface PaketPenyediaParams {
  kode_klpd?: string;
  tahun: number;
  status?: string;
  limit?: number;
  cursor?: string;
}

export async function getPaketPenyedia(params: PaketPenyediaParams) {
  const res = await api.get<PaketPenyediaResponse>("/inaproc/rup/paket-penyedia", { params });
  return res.data;
}

export interface SyncPaketPenyediaPayload {
  kode_klpd: string;
  tahun: string;
  status?: string;
}

export async function syncPaketPenyedia(payload: SyncPaketPenyediaPayload) {
  const res = await api.post("/inaproc/rup/paket-penyedia/sync", payload);
  return res.data;
}
// ============ Inaproc - Paket Swakelola ============

export interface PaketSwakelolaItem {
  kd_klpd: string;
  kd_satker: number;
  kd_rup: number;
  nama_klpd: string;
  nama_satker: string;
  nama_paket: string;
  tahun_anggaran: number;
  status: string;
}

export interface PaketSwakelolaResponse {
  success: boolean;
  data: PaketSwakelolaItem[] | null;
  meta: InaprocMeta;
}

export interface PaketSwakelolaParams {
  kode_klpd?: string;
  tahun: number;
  status?: string;
  limit?: number;
  cursor?: string;
}

export async function getPaketSwakelola(params: PaketSwakelolaParams) {
  const res = await api.get<PaketSwakelolaResponse>("/inaproc/rup/paket-swakelola", { params });
  return res.data;
}

export interface SyncPaketSwakelolaPayload {
  kode_klpd: string;
  tahun: string;
  status?: string;
}

export async function syncPaketSwakelola(payload: SyncPaketSwakelolaPayload) {
  const res = await api.post("/inaproc/rup/paket-swakelola/sync", payload);
  return res.data;
}

// ============ Inaproc - Program Master ============

export interface ProgramMasterItem {
  is_deleted: boolean;
  jenis_klpd: string;
  kd_klpd: string;
  kd_program: number;
  kd_program_lokal: number;
  kd_program_str: string;
  kd_satker: number;
  nama_klpd: string;
  nama_program: string;
  pagu_program: number;
  tahun_anggaran: number;
}

export interface ProgramMasterResponse {
  success: boolean;
  data: ProgramMasterItem[] | null;
  meta?: InaprocMeta;
}

export interface ProgramMasterParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getProgramMaster(params: ProgramMasterParams) {
  const res = await api.get<ProgramMasterResponse>("/inaproc/rup/program-master", { params });
  return res.data;
}

export interface SyncProgramMasterPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncProgramMaster(payload: SyncProgramMasterPayload) {
  const res = await api.post("/inaproc/rup/program-master/sync", payload);
  return res.data;
}

// ============ Inaproc - Paket Swakelola Terumumkan ============

export interface PaketSwakelolaTerumumkanItem {
  jenis_klpd: string;
  kd_klpd: string;
  kd_rup: number;
  kd_satker: number;
  nama_klpd: string;
  nama_paket: string;
  nama_ppk: string;
  nama_satker: string;
  nip_ppk: string;
  pagu: number;
  status_aktif_rup: boolean;
  status_umumkan_rup: string;
  tahun_anggaran: number;
  tgl_awal_pelaksanaan_kontrak: string;
  tgl_akhir_pelaksanaan_kontrak: string;
  volume_pekerjaan: string;
}

export interface PaketSwakelolaTerumumkanResponse {
  success: boolean;
  data: PaketSwakelolaTerumumkanItem[] | null;
  meta?: InaprocMeta;
}

export interface PaketSwakelolaTerumumkanParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getPaketSwakelolaTerumumkan(params: PaketSwakelolaTerumumkanParams) {
  const res = await api.get<PaketSwakelolaTerumumkanResponse>("/inaproc/rup/paket-swakelola-terumumkan", { params });
  return res.data;
}

export interface SyncPaketSwakelolaTerumumkanPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncPaketSwakelolaTerumumkan(payload: SyncPaketSwakelolaTerumumkanPayload) {
  const res = await api.post("/inaproc/rup/paket-swakelola-terumumkan/sync", payload);
  return res.data;
}

// ============ Inaproc - Paket Penyedia Terumumkan ============

export interface PaketPenyediaTerumumkanItem {
  kd_klpd: string;
  nama_klpd: string;
  kd_satker: number;
  nama_satker: string;
  kd_rup: number;
  nama_paket: string;
  pagu: number;
  metode_pengadaan: string;
  jenis_pengadaan: string;
  status_umumkan_rup: string;
  nama_ppk: string;
  tahun_anggaran: number;
  tgl_awal_pemilihan: string;
  tgl_akhir_pemilihan: string;
}

export interface PaketPenyediaTerumumkanResponse {
  success: boolean;
  data: PaketPenyediaTerumumkanItem[] | null;
  meta: InaprocMeta;
}

export interface PaketPenyediaTerumumkanParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getPaketPenyediaTerumumkan(params: PaketPenyediaTerumumkanParams) {
  const res = await api.get<PaketPenyediaTerumumkanResponse>("/inaproc/rup/paket-penyedia-terumumkan", { params });
  return res.data;
}

export interface SyncPaketPenyediaTerumumkanPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncPaketPenyediaTerumumkan(payload: SyncPaketPenyediaTerumumkanPayload) {
  const res = await api.post("/inaproc/rup/paket-penyedia-terumumkan/sync", payload);
  return res.data;
}


// ============ Inaproc - Paket Anggaran Swakelola ============

export interface PaketAnggaranSwakelolaItem {
  asal_dana: string;
  asal_dana_klpd: string;
  asal_dana_satker: string;
  jenis_klpd: string;
  kd_kegiatan: number;
  kd_klpd: string;
  kd_komponen: number;
  kd_rup: number;
  kd_rup_lokal: number;
  kd_satker: number;
  kd_satker_str: string;
  kd_subkegiatan: number;
  mak: string;
  nama_klpd: string;
  nama_satker: string;
  pagu: number;
  status_aktif_rup: boolean;
  status_delete_rup: boolean;
  status_umumkan_rup: string;
  sumber_dana: string;
  tahun_anggaran: number;
  tahun_anggaran_dana: number;
}

export interface PaketAnggaranSwakelolaResponse {
  success: boolean;
  data: PaketAnggaranSwakelolaItem[] | null;
  meta: InaprocMeta;
}

export interface PaketAnggaranSwakelolaParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getPaketAnggaranSwakelola(params: PaketAnggaranSwakelolaParams) {
  const res = await api.get<PaketAnggaranSwakelolaResponse>("/inaproc/rup/paket-anggaran-swakelola", { params });
  return res.data;
}

export interface SyncPaketAnggaranSwakelolaPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncPaketAnggaranSwakelola(payload: SyncPaketAnggaranSwakelolaPayload) {
  const res = await api.post("/inaproc/rup/paket-anggaran-swakelola/sync", payload);
  return res.data;
}

// ============ Inaproc - Tender: Jadwal Tahapan Non Tender ============

export interface JadwalTahapanNonTenderItem {
  kd_akt: number;
  kd_klpd: string;
  kd_nontender: number;
  kd_satker: string;
  kd_satker_str: string;
  nama_akt: string;
  nama_tahapan: string;
  tahun_anggaran: number;
  tgl_akhir: string;
  tgl_awal: string;
}

export interface JadwalTahapanNonTenderResponse {
  success: boolean;
  data: JadwalTahapanNonTenderItem[] | null;
  meta: InaprocMeta;
}

export interface JadwalTahapanNonTenderParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getJadwalTahapanNonTender(params: JadwalTahapanNonTenderParams) {
  const res = await api.get<JadwalTahapanNonTenderResponse>("/inaproc/tender/jadwal-tahapan-non-tender", { params });
  return res.data;
}

export interface SyncJadwalTahapanNonTenderPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncJadwalTahapanNonTender(payload: SyncJadwalTahapanNonTenderPayload) {
  const res = await api.post("/inaproc/tender/jadwal-tahapan-non-tender/sync", payload);
  return res.data;
}