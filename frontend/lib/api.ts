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

// Alasan yang ditampilkan di halaman login untuk 401 yang membawa kode galat dari backend (field "code").
// 401 tanpa kode (token tidak valid/kedaluwarsa) memakai teks bawaan halaman login.
const SESSION_END_REASONS = new Map<string, string>([
  ["sso_session_expired", "Sesi SSO Kemenkeu Anda telah berakhir. Silakan masuk kembali melalui SSO Kemenkeu."],
  ["account_inactive", "Akun Anda telah dinonaktifkan. Hubungi administrator."],
]);

api.interceptors.response.use(
  (response) => response,
  (error) => {
    // 401 baru berarti "sesi berakhir" bila pengguna memang punya sesi. Tanpa cookie (mis. salah
    // password di halaman login) itu galat biasa milik pemanggil: mengarahkan ulang di sini akan
    // memuat ulang halaman login dan menghapus pesan galat beserta isian form.
    if (error.response?.status === 401 && Cookies.get("pasti_access_token")) {
      Cookies.remove("pasti_access_token");
      if (typeof window !== "undefined") {
        // Halaman login menampilkan alasannya (LoginErrorBanner); tanpa reason dipakai teks bawaannya.
        const reason = SESSION_END_REASONS.get(error.response.data?.code) ?? "";
        window.location.href = "/login?error=session_expired" + (reason ? "&reason=" + encodeURIComponent(reason) : "");
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

export interface SLDKRefItem {
  kode: string;
  nama: string;
}

// Aturan pemantauan (aset idle, hilang, dst.); definisinya hidup di backend (backend/sldk/rules.go).
export interface SLDKRule {
  key: string;
  label: string;
  keterangan: string;
}

export interface SLDKReferences {
  jenis_bmn: SLDKRefItem[];
  kondisi: SLDKRefItem[];
  status_penggunaan: SLDKRefItem[];
  status_hukum: SLDKRefItem[];
  anomali: SLDKRule[];
}

export async function getSLDKReferences() {
  const res = await api.get<{ success: boolean; message: string; data: SLDKReferences }>("/sldk/referensi");
  return res.data;
}

export interface SLDKSatker {
  id: number;
  kode: string;
  nama: string;
}

export async function searchSLDKSatker(q: string) {
  const res = await api.get<{ success: boolean; message: string; data: { items: SLDKSatker[] } }>("/sldk/satker", {
    params: { q },
  });
  return res.data;
}

export interface SLDKAssetSearchParams {
  q?: string;
  by?: "id" | "teks"; // id = kode persis (cepat), teks = mengandung kata (lambat)
  id_satker?: number;
  kd_jns_bmn?: string;
  kd_kondisi?: string;
  kd_status?: string;
  tahun?: string;
  anomali?: string; // kunci aturan pemantauan (SLDKRule.key)
  limit?: number;
}

export interface SLDKAssetSearchData {
  results: Record<string, unknown>[];
  count: number;
  limit: number;
  satker: Record<string, { id: number; kode: string; nama: string }>;
  elapsed_ms: number;
}

// Server membatasi pencarian 25 detik; timeout klien sedikit di atasnya supaya pesan dari server sempat tiba.
export async function searchSLDKAssets(params: SLDKAssetSearchParams) {
  const res = await api.get<{ success: boolean; message: string; data: SLDKAssetSearchData }>("/sldk/assets/search", {
    params,
    timeout: 40000,
  });
  return res.data;
}

export interface SLDKDetailSection {
  rows: Record<string, unknown>[];
  error?: string;
}

export interface SLDKAssetDetailData {
  id_aset: number;
  sections: Record<string, SLDKDetailSection>;
}

export async function getSLDKAssetDetail(idAset: number) {
  const res = await api.get<{ success: boolean; message: string; data: SLDKAssetDetailData }>(
    `/sldk/assets/${idAset}/detail`,
    { timeout: 40000 }
  );
  return res.data;
}

// ----- Ringkasan & Pemantauan: dibaca dari hasil sinkronisasi di database PASTI, bukan dari SLDK langsung -----

export interface SLDKSyncInfo {
  id: number;
  status: "berjalan" | "sukses" | "gagal";
  mulai: string;
  selesai: string | null;
  cakupan: string | null;
  jumlah_baris: number | null;
  total_aset: number | null;
  data_per: string | null;
  pesan: string | null;
}

export interface SLDKOverviewGroup {
  k1: string | null;
  k2?: string | null;
  jumlah: number;
  nilai_perolehan: number;
  nilai_buku: number;
  nilai_susut: number;
}

export interface SLDKOverviewTotal {
  jumlah: number;
  nilai_perolehan: number;
  nilai_buku: number;
  nilai_susut: number;
}

export interface SLDKOverviewAnomaly extends SLDKRule {
  jumlah: number;
  satker_teratas: { id: string; jumlah: number }[];
}

export interface SLDKFlagKey {
  key: string;
  jumlah: number;
}

// tersedia=false: belum ada sinkronisasi yang sukses, hanya sinkron_terakhir (bila ada) yang terisi.
export type SLDKOverviewData =
  | { tersedia: false; sinkron_terakhir: SLDKSyncInfo | null; sinkron_sukses: null }
  | {
      tersedia: true;
      sinkron_terakhir: SLDKSyncInfo | null;
      sinkron_sukses: SLDKSyncInfo;
      definisi: { flag_keys_aktif: string[]; terkonfirmasi: boolean };
      flag_keys: SLDKFlagKey[];
      total: SLDKOverviewTotal;
      jenis: SLDKOverviewGroup[];
      kondisi: SLDKOverviewGroup[];
      status: SLDKOverviewGroup[];
      jenis_kondisi: SLDKOverviewGroup[];
      provinsi: SLDKOverviewGroup[];
      tahun: SLDKOverviewGroup[];
      satker_teratas: SLDKOverviewGroup[];
      anomali: SLDKOverviewAnomaly[];
      satker: Record<string, { id: number; kode: string; nama: string }>;
    };

export async function getSLDKOverview() {
  const res = await api.get<{ success: boolean; message: string; data: SLDKOverviewData }>("/sldk/ringkasan");
  return res.data;
}

// Khusus admin. Daftar kosong = semua baris dihitung sebagai aset.
export async function updateSLDKOverviewSettings(flagKeysAktif: string[]) {
  const res = await api.put<{ success: boolean; message: string; data: { flag_keys_aktif: string[] } }>(
    "/sldk/ringkasan/pengaturan",
    { flag_keys_aktif: flagKeysAktif }
  );
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

// ============ Inaproc - Tender: Jadwal Tahapan Tender ============

export interface JadwalTahapanTenderItem {
  kd_akt: number;
  kd_klpd: string;
  kd_tender: number;
  kd_satker: string;
  kd_satker_str: string;
  nama_akt: string;
  nama_tahapan: string;
  tahun_anggaran: number;
  tgl_akhir: string;
  tgl_awal: string;
}

export interface JadwalTahapanTenderResponse {
  success: boolean;
  data: JadwalTahapanTenderItem[] | null;
  meta: InaprocMeta;
}

export interface JadwalTahapanTenderParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getJadwalTahapanTender(params: JadwalTahapanTenderParams) {
  const res = await api.get<JadwalTahapanTenderResponse>("/inaproc/tender/jadwal-tahapan-tender", { params });
  return res.data;
}

export interface SyncJadwalTahapanTenderPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncJadwalTahapanTender(payload: SyncJadwalTahapanTenderPayload) {
  const res = await api.post("/inaproc/tender/jadwal-tahapan-tender/sync", payload);
  return res.data;
}

// ============ Inaproc - Tender: Non Tender E-Kontrak ============

// Riwayat BAP/BAST (Berita Acara Pembayaran / Serah Terima).
export interface BapBastHistoryItem {
  besar_pembayaran: number | null;
  jabatan_penandatangan_sk: string | null;
  jabatan_wakil_penyedia: string | null;
  no_bap: string | null;
  no_bast: string | null;
  progres_pekerjaan: number | null;
  tgl_bap: string | null;
  tgl_bast: string | null;
  wakil_sah_penyedia: string | null;
}

// Riwayat SPMK/SPP (Surat Perintah Mulai Kerja / Surat Perintah Pelaksanaan).
export interface SpmkSppHistoryItem {
  alamat_pengiriman: string | null;
  jabatan_wakil_penyedia: string | null;
  kota_spmk_spp: string | null;
  no_spmk_spp: string | null;
  tgl_mulai_pekerjaan: string | null;
  tgl_selesai_pekerjaan: string | null;
  tgl_spmk_spp: string | null;
  wakil_sah_penyedia: string | null;
  waktu_penyelesaian: string | null;
}

export interface PenilaianKinerjaPenyediaItem {
  indikator_penilaian: string;
  nilai_indikator: number | null;
}

export interface NonTenderEkontrakItem {
  alamat_satker: string;
  kd_klpd: string;
  kd_tender: number;
  tahun_anggaran: number;
  nama_paket: string;
  // Ketiga field di bawah selalu array dari API ([] kalau kosong).
  bapbast_history_json: BapBastHistoryItem[];
  spmkspp_history_json: SpmkSppHistoryItem[];
  penilaian_kinerja_penyedia: PenilaianKinerjaPenyediaItem[];
}

export interface NonTenderEkontrakResponse {
  success: boolean;
  data: NonTenderEkontrakItem[] | null;
  meta: InaprocMeta;
}

export interface NonTenderEkontrakParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getNonTenderEkontrak(params: NonTenderEkontrakParams) {
  const res = await api.get<NonTenderEkontrakResponse>("/inaproc/tender/non-tender-ekontrak", { params });
  return res.data;
}

export interface SyncNonTenderEkontrakPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncNonTenderEkontrak(payload: SyncNonTenderEkontrakPayload) {
  const res = await api.post("/inaproc/tender/non-tender-ekontrak/sync", payload);
  return res.data;
}

// ============ Inaproc - Tender: Non Tender E-Kontrak Kontrak ============

export interface NonTenderEkontrakKontrakItem {
  // Satker & KLPD
  kd_klpd: string;
  jenis_klpd: string | null;
  nama_klpd: string | null;
  kd_lpse: number | null;
  kd_satker: string | null;
  kd_satker_str: string | null;
  nama_satker: string | null;
  alamat_satker: string | null;

  // Paket
  kd_nontender: number;
  tahun_anggaran: number;
  nama_paket: string | null;
  mtd_pengadaan: string | null;
  lingkup_pekerjaan: string | null;
  informasi_lainnya: string | null;

  // Kontrak
  no_kontrak: string | null;
  no_sppbj: string | null;
  jenis_kontrak: string | null;
  status_kontrak: string | null;
  kota_kontrak: string | null;
  tgl_kontrak: string | null;
  tgl_kontrak_awal: string | null;
  tgl_kontrak_akhir: string | null;
  tgl_penetapan_status_kontrak: string | null;
  alasan_penetapan_status_kontrak: string | null;
  apakah_addendum: string | null;
  versi_addendum: number | null;
  alasan_addendum: string | null;

  // Nilai
  nilai_kontrak: number | null;
  nilai_pdn_kontrak: number | null;
  nilai_umk_kontrak: number | null;
  alasan_ubah_nilai_kontrak: string | null;
  alasan_nilai_kontrak_10_persen: string | null;

  // PPK
  nama_ppk: string | null;
  nip_ppk: string | null;
  jabatan_ppk: string | null;
  no_sk_ppk: string | null;

  // Penyedia
  nama_penyedia: string | null;
  bentuk_usaha_penyedia: string | null;
  tipe_penyedia: string | null;
  npwp_penyedia: string | null;
  npwp16_penyedia: string | null;
  wakil_sah_penyedia: string | null;
  jabatan_wakil_penyedia: string | null;
  anggota_kso: string | null;
  nama_rek_bank: string | null;
  no_rek_bank: string | null;
  nama_pemilik_rek_bank: string | null;
}

export interface NonTenderEkontrakKontrakResponse {
  success: boolean;
  data: NonTenderEkontrakKontrakItem[] | null;
  meta: InaprocMeta;
}

export interface NonTenderEkontrakKontrakParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getNonTenderEkontrakKontrak(params: NonTenderEkontrakKontrakParams) {
  const res = await api.get<NonTenderEkontrakKontrakResponse>("/inaproc/tender/non-tender-ekontrak-kontrak", { params });
  return res.data;
}

export interface SyncNonTenderEkontrakKontrakPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncNonTenderEkontrakKontrak(payload: SyncNonTenderEkontrakKontrakPayload) {
  const res = await api.post("/inaproc/tender/non-tender-ekontrak-kontrak/sync", payload);
  return res.data;
}

// ============ Inaproc - Tender: Non Tender Pengumuman ============

export interface NonTenderPengumumanItem {
  // Satker & KLPD
  kd_klpd: string;
  jenis_klpd: string | null;
  nama_klpd: string | null;
  kd_satker: string | null;
  kd_satker_str: string | null;
  nama_satker: string | null;

  // LPSE
  kd_lpse: number | null;
  nama_lpse: string | null;
  url_lpse: string | null;

  // Paket
  kd_nontender: number;
  kd_pkt_dce: number | null;
  lls_id: number | null;
  kd_rup: string | null;
  tahun_anggaran: number;
  nama_paket: string | null;
  jenis_pengadaan: string | null;
  kualifikasi_paket: string | null;
  kontrak_pembayaran: string | null;
  mtd_pemilihan: string | null;
  sumber_dana: string | null;
  mak: string | null;
  repeat_order: string | null;
  versi_nontender: number | null;

  // Nilai
  pagu: number | null;
  hps: number | null;

  // Status
  status_nontender: string | null;
  ket_ditutup: string | null;
  ket_diulang: string | null;

  // Pelaksana
  nip_nama_ppk: string | null;
  nip_nama_pp: string | null;
  nip_nama_pokja: string | null;

  // Tanggal
  tgl_buat_paket: string | null;
  tgl_kolektif_kolegial: string | null;
  tgl_pengumuman_nontender: string | null;
}

export interface NonTenderPengumumanResponse {
  success: boolean;
  data: NonTenderPengumumanItem[] | null;
  meta: InaprocMeta;
}

export interface NonTenderPengumumanParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getNonTenderPengumuman(params: NonTenderPengumumanParams) {
  const res = await api.get<NonTenderPengumumanResponse>("/inaproc/tender/non-tender-pengumuman", { params });
  return res.data;
}

export interface SyncNonTenderPengumumanPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncNonTenderPengumuman(payload: SyncNonTenderPengumumanPayload) {
  const res = await api.post("/inaproc/tender/non-tender-pengumuman/sync", payload);
  return res.data;
}
