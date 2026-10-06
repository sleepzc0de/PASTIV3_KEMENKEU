import axios from "axios";
import Cookies from "js-cookie";
import { kutipNipPanjang } from "./angkaBesar";
import { JALUR_KEMBALI } from "./loginPath";
import { netEnd, netStart } from "./netActivity";

export const api = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_URL,
  headers: { "Content-Type": "application/json" },
});

api.interceptors.request.use(
  (config) => {
    const token = Cookies.get("pasti_access_token");
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    netStart(); // bilah kemajuan di atas halaman (TopLoader); setiap netStart dipasangkan dengan netEnd di interceptor respons
    return config;
  },
  (error) => Promise.reject(error)
);

// Alasan yang ditampilkan di halaman login untuk 401 yang membawa kode galat dari backend (field "code").
// 401 tanpa kode (token tidak valid/kedaluwarsa) memakai teks bawaan halaman login.
const SESSION_END_REASONS = new Map<string, string>([
  ["sso_session_expired", "Sesi SSO Kemenkeu Anda telah berakhir. Silakan masuk kembali melalui SSO Kemenkeu."],
  ["account_inactive", "Akun Anda telah dinonaktifkan. Hubungi administrator."],
]);

api.interceptors.response.use(
  (response) => {
    netEnd();
    return response;
  },
  (error) => {
    // Permintaan yang dibatalkan sebelum terkirim tidak melewati interceptor permintaan, jadi tidak pernah netStart.
    if (error?.config) netEnd();
    // 401 baru berarti "sesi berakhir" bila pengguna memang punya sesi. Tanpa cookie (mis. salah
    // password di halaman login) itu galat biasa milik pemanggil: mengarahkan ulang di sini akan
    // memuat ulang halaman login dan menghapus pesan galat beserta isian form.
    if (error.response?.status === 401 && Cookies.get("pasti_access_token")) {
      Cookies.remove("pasti_access_token");
      if (typeof window !== "undefined") {
        // Halaman login menampilkan alasannya (LoginErrorBanner); tanpa reason dipakai teks bawaannya.
        const reason = SESSION_END_REASONS.get(error.response.data?.code) ?? "";
        // Bukan langsung ke halaman login: alamat login disembunyikan, jadi server yang mengarahkannya (JALUR_KEMBALI).
        window.location.href = JALUR_KEMBALI + "?error=session_expired" + (reason ? "&reason=" + encodeURIComponent(reason) : "");
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

// ============ Digitalisasi Aset (hasil sinkronisasi SLDK -> tabel DIGITALISASI_*) ============

export type DGDatasetKey =
  | "satker"
  | "tanah"
  | "gedung_kantor_utama"
  | "gedung_lainnya"
  | "rusunara"
  | "rumah_negara"
  | "mess_rumah_negara";

export interface DGAgg {
  jumlah: number;
  luas: number;
  nilai: number;
}

export interface DGAssetStat {
  key: DGDatasetKey;
  label: string;
  geo: boolean;
  punya_luas: boolean;
  punya_nilai: boolean;
  jumlah: number;
  luas: number;
  nilai: number;
  bertitik: number;
  tanpa_koordinat: number;
  di_luar_indonesia: number;
  tanpa_foto: number;
  tanpa_kondisi: number;
}

export interface DGUE1 {
  kode: string;
  label: string;
  satker: number;
  kdj: number;
  kdo: number;
  per: Partial<Record<DGDatasetKey, DGAgg>>;
}

export interface DGProvinsi {
  nama: string;
  per: Partial<Record<DGDatasetKey, DGAgg>>;
}

export interface DGCount {
  k: string | null;
  jumlah: number;
  nilai: number;
}

export interface DGOverviewData {
  tersedia: true;
  sinkron: { dataset: DGDatasetKey; label: string; jumlah_baris: number; terakhir_sukses: string | null }[];
  satker: { total: number; induk: number; anak: number; kdj: number; kdo: number };
  aset: DGAssetStat[];
  hunian: Record<string, number>;
  ue1: DGUE1[];
  provinsi: DGProvinsi[];
  kondisi: Partial<Record<DGDatasetKey, DGCount[]>>;
  status_hukum: Partial<Record<DGDatasetKey, DGCount[]>>;
  asuransi: Partial<Record<DGDatasetKey, DGCount[]>>;
  status_penghuni: DGCount[];
  kelengkapan: { satker_induk: number; induk_tanpa_kantor_utama: number; induk_tanpa_tanah: number };
}

export type DGOverview = DGOverviewData | { tersedia: false };

export async function getDGOverview() {
  const res = await api.get<{ success: boolean; message: string; data: DGOverview }>("/digitalisasi/ringkasan");
  return res.data;
}

export interface DGMapSet {
  key: DGDatasetKey;
  label: string;
  total: number;
  bertitik: number;
  tanpa_koordinat: number;
  di_luar_indonesia: number;
  terpotong: boolean;
  titik: [number, number, number][]; // [id, lintang, bujur]
}

export async function getDGMap(params: { dataset?: string; ue1?: string }) {
  const res = await api.get<{ success: boolean; message: string; data: { datasets: DGMapSet[] } }>("/digitalisasi/peta", {
    params,
    timeout: 60000,
  });
  return res.data;
}

export interface DGColumn {
  nama: string;
  tipe: "teks" | "bilangan" | "desimal";
  sensitif?: boolean;
}

export type DGRow = Record<string, string | number | null> & { id: number };

export interface DGListParams {
  q?: string;
  ue1?: string;
  provinsi?: string;
  kondisi?: string;
  jenis_satker?: string;
  tanpa_koordinat?: "1";
  page?: number;
  per_page?: number;
}

export interface DGListData {
  dataset: DGDatasetKey;
  label: string;
  geo: boolean;
  kolom: DGColumn[];
  rows: DGRow[];
  total: number;
  page: number;
  per_page: number;
  filter: { ue1?: string[]; provinsi?: string[]; kondisi?: string[] };
}

export async function listDGData(dataset: DGDatasetKey, params: DGListParams) {
  const res = await api.get<{ success: boolean; message: string; data: DGListData }>(`/digitalisasi/data/${dataset}`, { params });
  return res.data;
}

export interface DGDetailData {
  dataset: DGDatasetKey;
  label: string;
  kolom: DGColumn[];
  row: Record<string, string | number | null>;
}

export async function getDGDetail(dataset: DGDatasetKey, id: number) {
  const res = await api.get<{ success: boolean; message: string; data: DGDetailData }>(`/digitalisasi/data/${dataset}/${id}`);
  return res.data;
}

export type DGSyncStatus = "antri" | "berjalan" | "sukses" | "gagal" | "dibatalkan";

export interface DGSyncLog {
  id: number;
  dataset: DGDatasetKey;
  status: DGSyncStatus;
  dibuat: string;
  mulai: string | null;
  selesai: string | null;
  jumlah_baris: number | null;
  jumlah_koordinat: number | null;
  pesan: string | null;
  dijalankan_oleh: string | null;
}

export interface DGDatasetStatus {
  key: DGDatasetKey;
  label: string;
  deskripsi: string;
  tabel: string;
  peta: boolean;
  jumlah_baris: number;
  terakhir: DGSyncLog | null;
  terakhir_sukses: DGSyncLog | null;
}

export interface DGActiveRun {
  datasets: DGDatasetKey[];
  saat_ini: DGDatasetKey | "";
  mulai: string;
  oleh: string;
}

// Sinkronisasi otomatis berkala: dimulai server sendiri (tanpa batas waktu) pada jam malam bila sudah lewat interval sejak sukses terakhir.
export interface DGAutoInfo {
  aktif: boolean;
  interval_hari: number;
  jam_mulai: number; // jam (zona) paling awal sinkronisasi otomatis boleh dimulai
  jam_akhir: number;
  zona: string; // "WIB"
  berikutnya: string | null; // perkiraan mulai berikutnya
}

export interface DGSyncOverview {
  sldk_tersedia: boolean;
  aktif: DGActiveRun | null;
  otomatis: DGAutoInfo;
  datasets: DGDatasetStatus[];
  riwayat: DGSyncLog[];
}

export async function getDGSyncStatus() {
  const res = await api.get<{ success: boolean; message: string; data: DGSyncOverview }>("/digitalisasi/sinkronisasi");
  return res.data;
}

// Khusus admin. Sinkronisasi berjalan di server; respons langsung kembali (202) dan kemajuannya dibaca lewat getDGSyncStatus.
export async function startDGSync(body: { datasets?: DGDatasetKey[]; semua?: boolean }) {
  const res = await api.post<{ success: boolean; message: string; data: { aktif: DGActiveRun } }>("/digitalisasi/sinkronisasi", body);
  return res.data;
}

export async function cancelDGSync() {
  const res = await api.post<{ success: boolean; message: string; data: { dibatalkan: boolean } }>("/digitalisasi/sinkronisasi/batal");
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

// ============ Inaproc - Tender: Non Tender Selesai ============

export interface NonTenderSelesaiItem {
  // Satker & KLPD
  kd_klpd: string | number | null;
  jenis_klpd: string | null;
  nama_klpd: string | null;
  kd_satker: string | null;
  kd_satker_str: string | null;
  nama_satker: string | null;

  // LPSE
  kd_lpse: number | null;
  lpse_id: number | null;
  nama_lpse: string | null;
  url_lpse: string | null;

  // Paket
  kd_nontender: number;
  kd_pkt_dce: number | null;
  kd_rup: string | null;
  tahun_anggaran: number;
  nama_paket: string | null;
  jenis_pengadaan: string | null;
  kualifikasi_paket: string | null;
  kontrak_pembayaran: string | null;
  mtd_pemilihan: string | null;
  sumber_dana: string | null;
  mak: string | null;

  // Penyedia
  kd_penyedia: number | null;
  nama_penyedia: string | null;
  npwp_penyedia: string | null;
  npwp16_penyedia: string | null;

  // Status
  status_nontender: string | null;

  // Nilai
  pagu: number | null;
  hps: number | null;
  nilai_penawaran: number | null;
  nilai_negosiasi: number | null;
  nilai_terkoreksi: number | null;
  nilai_kontrak: number | null;
  nilai_pdn_kontrak: number | null;
  nilai_umk_kontrak: number | null;

  // Tanggal
  tgl_pengumuman_nontender: string | null;
  tgl_selesai_nontender: string | null;
  tgl_penarikan: string | null;
}

export interface NonTenderSelesaiResponse {
  success: boolean;
  data: NonTenderSelesaiItem[] | null;
  meta: InaprocMeta;
}

export interface NonTenderSelesaiParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getNonTenderSelesai(params: NonTenderSelesaiParams) {
  const res = await api.get<NonTenderSelesaiResponse>("/inaproc/tender/non-tender-selesai", { params });
  return res.data;
}

export interface SyncNonTenderSelesaiPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncNonTenderSelesai(payload: SyncNonTenderSelesaiPayload) {
  const res = await api.post("/inaproc/tender/non-tender-selesai/sync", payload);
  return res.data;
}

// ============ Inaproc - Tender: Pencatatan Non Tender ============

export interface PencatatanNonTenderItem {
  // Satker & KLPD
  kd_klpd: string | null;
  jenis_klpd: string | null;
  nama_klpd: string | null;
  kd_satker: string | null;
  kd_satker_str: string | null;
  nama_satker: string | null;

  // Paket
  kd_lpse: number | null;
  kd_nontender_pct: number;
  kd_pkt_dce: number | null;
  kd_rup: string | null;
  tahun_anggaran: number;
  nama_paket: string | null;
  kategori_pengadaan: string | null;
  mtd_pemilihan: string | null;
  sumber_dana: string | null;
  uraian_pekerjaan: string | null;

  // PPK
  nama_ppk: string | null;
  nip_ppk: string | null;

  // Status
  status_nontender_pct: string | null;
  status_nontender_pct_ket: string | null;
  alasan_pembatalan: string | null;

  // Lainnya
  bukti_pembayaran: string | null;
  informasi_lainnya: string | null;

  // Nilai
  pagu: number | null;
  nilai_pdn_pct: number | null;
  nilai_umk_pct: number | null;
  total_realisasi: number | null;

  // Tanggal
  tgl_buat_paket: string | null;
  tgl_mulai_paket: string | null;
  tgl_selesai_paket: string | null;
}

export interface PencatatanNonTenderResponse {
  success: boolean;
  data: PencatatanNonTenderItem[] | null;
  meta: InaprocMeta;
}

export interface PencatatanNonTenderParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getPencatatanNonTender(params: PencatatanNonTenderParams) {
  const res = await api.get<PencatatanNonTenderResponse>("/inaproc/tender/pencatatan-non-tender", { params });
  return res.data;
}

export interface SyncPencatatanNonTenderPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncPencatatanNonTender(payload: SyncPencatatanNonTenderPayload) {
  const res = await api.post("/inaproc/tender/pencatatan-non-tender/sync", payload);
  return res.data;
}

// ============ Inaproc - Tender: Pencatatan Non Tender Realisasi ============

export interface PencatatanNonTenderRealisasiItem {
  // Satker & KLPD
  kd_klpd: string | null;
  jenis_klpd: string | null;
  nama_klpd: string | null;
  kd_satker: string | null;
  kd_satker_str: string | null;
  nama_satker: string | null;

  // Paket
  kd_lpse: number | null;
  nama_lpse: string | null;
  kd_nontender_pct: number;
  kd_paket_dce: number | null;
  kd_rup_paket: string | null;
  tahun_anggaran: number;
  nama_paket: string | null;

  // PPK dan penyedia
  nama_ppk: string | null;
  nip_ppk: string | null;
  nama_penyedia: string | null;
  npwp_penyedia: string | null;

  // Realisasi
  no_realisasi: string | null;
  jenis_realisasi: string | null;
  ket_realisasi: string | null;
  dok_realisasi: unknown; // belum terdokumentasi (null di contoh); bisa teks, objek, atau larik

  // Nilai
  pagu: number | null;
  nilai_realisasi: number | null;

  // Tanggal
  tgl_realisasi: string | null;
}

export interface PencatatanNonTenderRealisasiResponse {
  success: boolean;
  data: PencatatanNonTenderRealisasiItem[] | null;
  meta: InaprocMeta;
}

export interface PencatatanNonTenderRealisasiParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getPencatatanNonTenderRealisasi(params: PencatatanNonTenderRealisasiParams) {
  const res = await api.get<PencatatanNonTenderRealisasiResponse>("/inaproc/tender/pencatatan-non-tender-realisasi", { params });
  return res.data;
}

export interface SyncPencatatanNonTenderRealisasiPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncPencatatanNonTenderRealisasi(payload: SyncPencatatanNonTenderRealisasiPayload) {
  const res = await api.post("/inaproc/tender/pencatatan-non-tender-realisasi/sync", payload);
  return res.data;
}

// ============ Inaproc - Tender: Pencatatan Swakelola ============

export interface PencatatanSwakelolaItem {
  // Satker & KLPD
  kd_klpd: string | null;
  jenis_klpd: string | null;
  nama_klpd: string | null;
  kd_satker: string | null;
  kd_satker_str: string | null;
  nama_satker: string | null;

  // Paket
  kd_lpse: number | null;
  kd_swakelola_pct: number;
  kd_pkt_dce: number | null;
  kd_rup: string | null;
  tahun_anggaran: number;
  nama_paket: string | null;
  sumber_dana: string | null;
  uraian_pekerjaan: string | null;
  tipe_swakelola: number | null;
  tipe_swakelola_nama: string | null;

  // PPK
  nama_ppk: string | null;
  nip_ppk: string | null;

  // Status
  status_swakelola_pct: string | null;
  status_swakelola_pct_ket: string | null;
  alasan_pembatalan: string | null;
  informasi_lainnya: string | null;

  // Nilai
  pagu: number | null;
  nilai_pdn_pct: number | null;
  nilai_umk_pct: number | null;
  total_realisasi: number | null;

  // Tanggal
  tgl_buat_paket: string | null;
  tgl_mulai_paket: string | null;
  tgl_selesai_paket: string | null;
}

export interface PencatatanSwakelolaResponse {
  success: boolean;
  data: PencatatanSwakelolaItem[] | null;
  meta: InaprocMeta;
}

export interface PencatatanSwakelolaParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getPencatatanSwakelola(params: PencatatanSwakelolaParams) {
  const res = await api.get<PencatatanSwakelolaResponse>("/inaproc/tender/pencatatan-swakelola", { params });
  return res.data;
}

export interface SyncPencatatanSwakelolaPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncPencatatanSwakelola(payload: SyncPencatatanSwakelolaPayload) {
  const res = await api.post("/inaproc/tender/pencatatan-swakelola/sync", payload);
  return res.data;
}

// ============ Inaproc - Tender: Pencatatan Swakelola Realisasi ============

// Respons ini hanya memuat kode: tidak ada nama paket, nama satker, maupun pagu (ada di Pencatatan Swakelola, lewat
// kd_swakelola_pct).
export interface PencatatanSwakelolaRealisasiItem {
  // KLPD, satker, dan paket
  kd_klpd: string | null;
  kd_satker: string | null;
  kd_lpse: number | null;
  kd_swakelola_pct: number;
  rsk_id: number | null;
  tahun_anggaran: number;

  // PPK dan pelaksana. nip_ppk dikirim API sebagai angka; getPencatatanSwakelolaRealisasi mengutip yang panjang (lib/angkaBesar.ts).
  nama_ppk: string | null;
  nip_ppk: string | number | null;
  nama_pelaksana: string | null;
  npwp_pelaksana: string | null;

  // Realisasi
  no_realisasi: string | null;
  jenis_realisasi: string | null;
  ket_realisasi: string | null;
  dok_realisasi: string | null;
  nilai_realisasi: number | null;
  tgl_realisasi: string | null;
}

export interface PencatatanSwakelolaRealisasiResponse {
  success: boolean;
  data: PencatatanSwakelolaRealisasiItem[] | null;
  meta: InaprocMeta;
}

export interface PencatatanSwakelolaRealisasiParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getPencatatanSwakelolaRealisasi(params: PencatatanSwakelolaRealisasiParams) {
  const res = await api.get<PencatatanSwakelolaRealisasiResponse>("/inaproc/tender/pencatatan-swakelola-realisasi", {
    params,
    transformResponse: [kutipNipPanjang],
  });
  return res.data;
}

export interface SyncPencatatanSwakelolaRealisasiPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncPencatatanSwakelolaRealisasi(payload: SyncPencatatanSwakelolaRealisasiPayload) {
  const res = await api.post("/inaproc/tender/pencatatan-swakelola-realisasi/sync", payload);
  return res.data;
}

// ============ Inaproc - Tender: Pengumuman ============

export interface TenderPengumumanItem {
  // Satker & KLPD
  kd_klpd: string | null;
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
  kd_pkt_dce: number | null;
  kd_rup: string | null;
  kd_tender: number;
  tahun_anggaran: number;
  list_tahun_anggaran: string | null;
  nama_paket: string | null;
  jenis_pengadaan: string | null;
  kualifikasi_paket: string | null;
  kontrak_pembayaran: string | null;
  lokasi_pekerjaan: string | null;
  sumber_dana: string | null;
  versi_tender: number | null;

  // Metode
  mtd_pemilihan: string | null;
  mtd_kualifikasi: string | null;
  mtd_evaluasi: string | null;

  // PPK dan Pokja
  nama_ppk: string | null;
  nip_ppk: string | null;
  nama_pokja: string | null;
  nip_pokja: string | null;

  // Status
  status_tender: string | null;
  ket_ditutup: string | null;
  ket_diulang: string | null;

  // Nilai
  pagu: number | null;
  hps: number | null;

  // Tanggal
  tanggal_status: string | null;
  tgl_buat_paket: string | null;
  tgl_kolektif_kolegial: string | null;
  tgl_pengumuman_tender: string | null;
}

export interface TenderPengumumanResponse {
  success: boolean;
  data: TenderPengumumanItem[] | null;
  meta: InaprocMeta;
}

// Dua skenario di Inaproc: (tahun + kode_klpd) atau kd_tender saja.
export interface TenderPengumumanParams {
  kode_klpd?: string;
  tahun?: number;
  kd_tender?: string;
  limit?: number;
  cursor?: string;
}

export async function getTenderPengumuman(params: TenderPengumumanParams) {
  // Bila kd_tender diisi, hanya itu (dan limit/cursor) yang dikirim; tahun dan kode_klpd diabaikan.
  const query = params.kd_tender ? { kd_tender: params.kd_tender, limit: params.limit, cursor: params.cursor } : { ...params, kd_tender: undefined };
  const res = await api.get<TenderPengumumanResponse>("/inaproc/tender/pengumuman", { params: query });
  return res.data;
}

export interface SyncTenderPengumumanPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncTenderPengumuman(payload: SyncTenderPengumumanPayload) {
  const res = await api.post("/inaproc/tender/pengumuman/sync", payload);
  return res.data;
}

// ============ Inaproc - Tender: Peserta Tender ============

export interface PesertaTenderItem {
  // Satker & KLPD
  kd_klpd: string | null;
  kd_satker: string | null;
  kd_satker_str: string | null;
  kd_lpse: number | null;

  // Tender
  kd_pkt_dce: number | null;
  kd_tender: number;
  kd_peserta: number;
  tahun_anggaran: number;

  // Penyedia
  kd_penyedia: number | null;
  nama_penyedia: string | null;
  npwp_penyedia: string | null;
  npwp_penyedia_16: string | null;

  // Hasil: penanda 0/1
  pemenang: number | null;
  pemenang_terverifikasi: number | null;
  alasan: string | null;

  // Nilai
  nilai_penawaran: number | null;
  nilai_terkoreksi: number | null;
}

export interface PesertaTenderResponse {
  success: boolean;
  data: PesertaTenderItem[] | null;
  meta: InaprocMeta;
}

export interface PesertaTenderParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getPesertaTender(params: PesertaTenderParams) {
  const res = await api.get<PesertaTenderResponse>("/inaproc/tender/peserta-tender", { params });
  return res.data;
}

export interface SyncPesertaTenderPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncPesertaTender(payload: SyncPesertaTenderPayload) {
  const res = await api.post("/inaproc/tender/peserta-tender/sync", payload);
  return res.data;
}

// ============ Inaproc - Tender: Tender E-Kontrak dan Kontrak ============

// Field kontrak, dipakai bersama oleh tender-ekontrak-kontrak dan tender-ekontrak.
export interface TenderEkontrakKontrakItem {
  // Satker & KLPD
  kd_klpd: string | null;
  jenis_klpd: string | null;
  nama_klpd: string | null;
  kd_lpse: number | null;
  kd_satker: string | null;
  kd_satker_str: string | null;
  nama_satker: string | null;
  alamat_satker: string | null;

  // Paket
  kd_tender: number;
  tahun_anggaran: number;
  nama_paket: string | null;
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
  kd_penyedia: number | null;
  nama_penyedia: string | null;
  bentuk_usaha_penyedia: string | null;
  tipe_penyedia: string | null;
  npwp_penyedia: string | null;
  npwp_16_penyedia: string | null;
  wakil_sah_penyedia: string | null;
  jabatan_wakil_penyedia: string | null;
  anggota_kso: string | null;
  nama_rek_bank: string | null;
  no_rek_bank: string | null;
  nama_pemilik_rek_bank: string | null;
}

// tender-ekontrak: kontrak ditambah tiga riwayat yang selalu array dari API ([] bila kosong). Baris yang belum punya kontrak
// bisa hanya memuat sebagian field.
export interface TenderEkontrakItem extends TenderEkontrakKontrakItem {
  bapbast_history_json: BapBastHistoryItem[];
  spmkspp_history_json: SpmkSppHistoryItem[];
  penilaian_kinerja_penyedia: PenilaianKinerjaPenyediaItem[];
}

export interface TenderEkontrakResponse {
  success: boolean;
  data: TenderEkontrakItem[] | null;
  meta: InaprocMeta;
}

export interface TenderEkontrakKontrakResponse {
  success: boolean;
  data: TenderEkontrakKontrakItem[] | null;
  meta: InaprocMeta;
}

export interface TenderEkontrakParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getTenderEkontrak(params: TenderEkontrakParams) {
  const res = await api.get<TenderEkontrakResponse>("/inaproc/tender/tender-ekontrak", { params });
  return res.data;
}

export async function getTenderEkontrakKontrak(params: TenderEkontrakParams) {
  const res = await api.get<TenderEkontrakKontrakResponse>("/inaproc/tender/tender-ekontrak-kontrak", { params });
  return res.data;
}

export interface SyncTenderEkontrakPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncTenderEkontrak(payload: SyncTenderEkontrakPayload) {
  const res = await api.post("/inaproc/tender/tender-ekontrak/sync", payload);
  return res.data;
}

export async function syncTenderEkontrakKontrak(payload: SyncTenderEkontrakPayload) {
  const res = await api.post("/inaproc/tender/tender-ekontrak-kontrak/sync", payload);
  return res.data;
}

// ============ Inaproc - Tender: Tender Selesai ============

export interface TenderSelesaiItem {
  // Satker & KLPD
  kd_klpd: string | null;
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
  kd_rup: string | null;
  kd_tender: number;
  tahun_anggaran: number;
  nama_paket: string | null;
  jenis_pengadaan: string | null;
  kualifikasi_paket: string | null;
  kontrak_pembayaran: string | null;
  sumber_dana: string | null;
  mak: string | null;
  mtd_pemilihan: string | null;
  mtd_kualifikasi: string | null;

  // Status
  status_tender: string | null;
  last_update_ref: string | null;

  // Nilai
  pagu: number | null;
  hps: number | null;

  // Tanggal
  tgl_pengumuman_tender: string | null;
  tgl_penetapan_pemenang: string | null;
}

export interface TenderSelesaiResponse {
  success: boolean;
  data: TenderSelesaiItem[] | null;
  meta: InaprocMeta;
}

export interface TenderSelesaiParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getTenderSelesai(params: TenderSelesaiParams) {
  const res = await api.get<TenderSelesaiResponse>("/inaproc/tender/tender-selesai", { params });
  return res.data;
}

export interface SyncTenderSelesaiPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncTenderSelesai(payload: SyncTenderSelesaiPayload) {
  const res = await api.post("/inaproc/tender/tender-selesai/sync", payload);
  return res.data;
}

// ============ Inaproc - Tender: Tender Selesai Nilai ============

// Respons ini tidak memuat nama paket (ada di Tender Selesai, lewat kd_tender); kd_satker di sini berbentuk kode bertitik.
export interface TenderSelesaiNilaiItem {
  // Satker & KLPD
  kd_klpd: string | null;
  jenis_klpd: string | null;
  nama_klpd: string | null;
  kd_satker: string | null;
  nama_satker: string | null;
  kd_lpse: number | null;

  // Tender
  kd_tender: number;
  kd_paket: number | null;
  kd_rup_paket: string | null;
  psr_id: number | null;
  tahun_anggaran: number;

  // Penyedia
  kd_penyedia: number | null;
  nama_penyedia: string | null;
  npwp_penyedia: string | null;
  npwp_16_penyedia: string | null;

  // Nilai
  pagu: number | null;
  hps: number | null;
  nilai_penawaran: number | null;
  nilai_terkoreksi: number | null;
  nilai_negosiasi: number | null;
  nilai_kontrak: number | null;
  nilai_pdn_kontrak: number | null;
  nilai_umk_kontrak: number | null;

  // Tanggal
  tgl_pengumuman_tender: string | null;
  tgl_penetapan_pemenang: string | null;
}

export interface TenderSelesaiNilaiResponse {
  success: boolean;
  data: TenderSelesaiNilaiItem[] | null;
  meta: InaprocMeta;
}

export interface TenderSelesaiNilaiParams {
  kode_klpd?: string;
  tahun: number;
  limit?: number;
  cursor?: string;
}

export async function getTenderSelesaiNilai(params: TenderSelesaiNilaiParams) {
  const res = await api.get<TenderSelesaiNilaiResponse>("/inaproc/tender/tender-selesai-nilai", { params });
  return res.data;
}

export interface SyncTenderSelesaiNilaiPayload {
  kode_klpd: string;
  tahun: string;
}

export async function syncTenderSelesaiNilai(payload: SyncTenderSelesaiNilaiPayload) {
  const res = await api.post("/inaproc/tender/tender-selesai-nilai/sync", payload);
  return res.data;
}

// ============ Inaproc - E-Katalog V5 (archive) ============
//
// Bentuk pencarian beda-beda: instansi-satker per kode_klpd (tanpa tahun), paket-e-purchasing per kode_klpd + tahun, dan
// komoditas/penyedia/distributor per satu kode (pencarian rujukan; tidak ada daftar "semua"). Sinkronisasi pencarian per kode
// menyimpan hasil untuk kode yang diberikan di badan sebagai `kode`.

export interface EkatalogInstansiSatkerItem {
  kd_klpd: string | null;
  jenis_klpd: string | null;
  nama_klpd: string | null;
  kd_satker: number | null;
  kd_satker_str: string | null;
  nama_satker: string | null;
}

export interface EkatalogKomoditasItem {
  Jenis_Katalog: string | null; // huruf besar mengikuti API
  kd_instansi_katalog: number | null;
  kd_komoditas: number;
  nama_instansi_katalog: string | null;
  nama_komoditas: string | null;
}

export interface EkatalogPaketItem {
  // Paket
  kd_klpd: string | null;
  kd_rup: number | null;
  tahun_anggaran: number;
  kd_paket: number;
  no_paket: string | null;
  nama_paket: string | null;
  deskripsi: string | null;
  catatan_produk: string | null;
  status_paket: string | null;
  paket_status_str: string | null;
  kode_anggaran: string | null;
  nama_sumber_dana: string | null;

  // Produk dan penyedia
  kd_komoditas: number | null;
  kd_produk: number | null;
  kd_paket_produk: number | null;
  kd_penyedia: number | null;
  kd_penyedia_distributor: number | null;

  // Satker
  satker_id: number | null;
  nama_satker: string | null;
  alamat_satker: string | null;
  npwp_satker: string | null;

  // PPK dan Pokja
  kd_user_ppk: number | null;
  ppk_nip: string | null;
  jabatan_ppk: string | null;
  kd_user_pokja: number | null;
  email_user_pokja: string | null;
  no_telp_user_pokja: string | null;

  // Wilayah harga
  kd_provinsi_wilayah_harga: number | null;
  kd_kabupaten_wilayah_harga: number | null;

  // Harga
  harga_satuan: number | null;
  kuantitas: number | null;
  ongkos_kirim: number | null;
  total_harga: number | null;
  jml_jenis_produk: number | null;

  // Tanggal (tanpa jam)
  tanggal_buat_paket: string | null;
  tanggal_edit_paket: string | null;
}

export interface EkatalogPenyediaItem {
  kd_penyedia: number;
  kode_penyedia_sikap: number | null;
  nama_penyedia: string | null;
  npwp_penyedia: string | null;
  npwp_16: string | null;
  penyedia_ukm: string | null;
  kbli2020_penyedia: string | null; // beberapa kode KBLI dipisah titik koma
  alamat_penyedia: string | null;
  email_penyedia: string | null;
  no_telp_penyedia: string | null;
}

export interface EkatalogDistributorItem {
  kd_penyedia_distributor: number;
  nama_distributor: string | null;
  npwp_distributor: string | null;
  alamat_distributor: string | null;
  email_distributor: string | null;
  no_telp_distributor: string | null;
}

interface EkatalogResponse<T> {
  success: boolean;
  data: T[] | null;
  meta: InaprocMeta;
}

export type EkatalogInstansiSatkerResponse = EkatalogResponse<EkatalogInstansiSatkerItem>;
export type EkatalogKomoditasResponse = EkatalogResponse<EkatalogKomoditasItem>;
export type EkatalogPaketResponse = EkatalogResponse<EkatalogPaketItem>;
export type EkatalogPenyediaResponse = EkatalogResponse<EkatalogPenyediaItem>;
export type EkatalogDistributorResponse = EkatalogResponse<EkatalogDistributorItem>;

export interface EkatalogParamsKlpd {
  kode_klpd?: string;
  limit?: number;
  cursor?: string;
}

export interface EkatalogParamsPaket extends EkatalogParamsKlpd {
  tahun: number;
}

export async function getEkatalogInstansiSatker(params: EkatalogParamsKlpd) {
  const res = await api.get<EkatalogInstansiSatkerResponse>("/inaproc/ekatalog-archive/instansi-satker", { params });
  return res.data;
}

export async function getEkatalogKomoditas(params: { kode_komoditas: string; limit?: number; cursor?: string }) {
  const res = await api.get<EkatalogKomoditasResponse>("/inaproc/ekatalog-archive/komoditas-detail", { params });
  return res.data;
}

export async function getEkatalogPaket(params: EkatalogParamsPaket) {
  const res = await api.get<EkatalogPaketResponse>("/inaproc/ekatalog-archive/paket-e-purchasing", { params });
  return res.data;
}

export async function getEkatalogPenyedia(params: { kode_penyedia: string; limit?: number; cursor?: string }) {
  const res = await api.get<EkatalogPenyediaResponse>("/inaproc/ekatalog-archive/penyedia-detail", { params });
  return res.data;
}

export async function getEkatalogDistributor(params: { kd_distributor: string; limit?: number; cursor?: string }) {
  const res = await api.get<EkatalogDistributorResponse>("/inaproc/ekatalog-archive/penyedia-distributor-detail", { params });
  return res.data;
}

export async function syncEkatalogInstansiSatker(payload: { kode_klpd: string }) {
  const res = await api.post("/inaproc/ekatalog-archive/instansi-satker/sync", payload);
  return res.data;
}

export async function syncEkatalogKomoditas(payload: { kode: string }) {
  const res = await api.post("/inaproc/ekatalog-archive/komoditas-detail/sync", payload);
  return res.data;
}

export async function syncEkatalogPaket(payload: { kode_klpd: string; tahun: string }) {
  const res = await api.post("/inaproc/ekatalog-archive/paket-e-purchasing/sync", payload);
  return res.data;
}

export async function syncEkatalogPenyedia(payload: { kode: string }) {
  const res = await api.post("/inaproc/ekatalog-archive/penyedia-detail/sync", payload);
  return res.data;
}

export async function syncEkatalogDistributor(payload: { kode: string }) {
  const res = await api.post("/inaproc/ekatalog-archive/penyedia-distributor-detail/sync", payload);
  return res.data;
}

// ============ Inaproc - E-Katalog V6 ============
//
// Penyedia dan produk penyedia dicari per kode_penyedia (kode teks); paket per kode_klpd + tahun (kode_klpd "swasta" = paket
// swasta); kategori produk berjenjang L1 -> L2 -> L3; transaksi per produk per tahun dengan filter status. Sinkronisasi pencarian
// per kode menyimpan hasil untuk kode di badan sebagai `kode`.

export interface Ekatalog6PenyediaItem {
  kode_penyedia: string;
  nama_penyedia: string | null;
  nib: string | null;
  npwp_penyedia: string | null;
  bentuk_usaha: string | null;
  jenis_perusahaan: string | null;
  status_aktif: string | null;
  status_umkk: number | null; // penanda 0/1
  rekan_id: number | null;
  alamat_penyedia: string | null;
  email: string | null;
  telepon: string | null;
  kbli_id: string | null; // beberapa kode dipisah koma
  kbli_name: string | null; // beberapa nama dipisah koma
}

export interface Ekatalog6ProdukPenyediaItem {
  kd_produk: string;
  nama_produk: string | null;
  status_produk: string | null;
  status_produk_tayang: boolean | null;
}

export interface Ekatalog6PaketItem {
  kode_klpd: string | null;
  fiscal_year: number;
  order_id: string;
  product_id: string | null;
  kode_penyedia: string | null;
  rekan_id: number | null;
  status: string | null;
  shipment_status: string | null;
  is_swasta: boolean | null;
  kode_satker: string | null;
  nama_satker: string | null;
  rup_code: string | null;
  rup_name: string | null;
  rup_desc: string | null;
  mak: string | null;
  funding_source: string | null;
  count_product: number | null;
  total_qty: number | null;
  shipping_fee: number | null;
  total: number | null;
  order_date: string | null;
  last_update_date: string | null;
}

// Tiap tingkat hanya memuat pasangan kode/nama untuk tingkatnya sendiri (+ datamart_id).
export interface Ekatalog6KategoriItem {
  datamart_id: number | null;
  kd_kategori_1?: string | null;
  nama_kategori_1?: string | null;
  kd_kategori_2?: string | null;
  nama_kategori_2?: string | null;
  kd_kategori_3?: string | null;
  nama_kategori_3?: string | null;
}

export interface Ekatalog6TransaksiItem {
  order_id: string;
  kd_kategori_1: string | null;
  kategori_1: string | null;
  kd_kategori_2: string | null;
  kategori_2: string | null;
  kd_kategori_3: string | null;
  kategori_3: string | null;
  product_id: string | null;
  nama_produk: string | null;
  kode_klpd: string | null;
  nama_group_klpd: string | null;
  nama_klpd: string | null;
  kode_satker: string | null;
  nama_satker: string | null;
  status: string | null;
  nilai_transaksi: number | null;
}

// Status transaksi yang didukung Inaproc (tidak peka huruf besar/kecil; bawaan COMPLETED).
export const STATUS_TRANSAKSI = [
  "COMPLETED",
  "CANCELLED",
  "CANCELLED_ON_NEGOTIATION",
  "CANCELLED_ON_REVIEW",
  "ESIGN_IN_PROGRESS",
  "ON_ADDENDUM",
  "ON_NEGOTIATION",
  "ON_PROCESS",
  "PAYMENT_OUTSIDE_SYSTEM",
  "REQUEST_CANCEL_BY_ADMIN",
  "WAITING_PPK_REVIEW",
  "WAITING_SELLER_CONFIRMATION",
];

interface Ekatalog6Response<T> {
  success: boolean;
  data: T[] | null;
  meta: InaprocMeta;
}

export interface Ekatalog6ParamsDasar {
  limit?: number;
  cursor?: string;
}

export async function getEkatalog6Penyedia(params: Ekatalog6ParamsDasar & { kode_penyedia: string }) {
  const res = await api.get<Ekatalog6Response<Ekatalog6PenyediaItem>>("/inaproc/ekatalog/penyedia-detail", { params });
  return res.data;
}

export async function getEkatalog6ProdukPenyedia(params: Ekatalog6ParamsDasar & { kode_penyedia: string }) {
  const res = await api.get<Ekatalog6Response<Ekatalog6ProdukPenyediaItem>>("/inaproc/ekatalog/list-produk-penyedia", { params });
  return res.data;
}

export async function getEkatalog6Paket(params: Ekatalog6ParamsDasar & { kode_klpd: string; tahun: number }) {
  const res = await api.get<Ekatalog6Response<Ekatalog6PaketItem>>("/inaproc/ekatalog/paket-e-purchasing", { params });
  return res.data;
}

// Tanpa kd_kategori_1: tingkat 1; dengan kd_kategori_1: tingkat 2; dengan keduanya: tingkat 3.
export async function getEkatalog6Kategori(params: Ekatalog6ParamsDasar & { kd_kategori_1?: string; kd_kategori_2?: string }) {
  const res = await api.get<Ekatalog6Response<Ekatalog6KategoriItem>>("/inaproc/ekatalog/list-kategori-produk", { params });
  return res.data;
}

export interface Ekatalog6ParamsTransaksi extends Ekatalog6ParamsDasar {
  tahun: number;
  kode_klpd?: string;
  kd_kategori_1?: string;
  kd_product?: string;
  status?: string;
}

export async function getEkatalog6Transaksi(params: Ekatalog6ParamsTransaksi) {
  const res = await api.get<Ekatalog6Response<Ekatalog6TransaksiItem>>("/inaproc/ekatalog/e-purchasing-by-produk", { params });
  return res.data;
}

export async function syncEkatalog6Penyedia(payload: { kode: string }) {
  const res = await api.post("/inaproc/ekatalog/penyedia-detail/sync", payload);
  return res.data;
}

export async function syncEkatalog6ProdukPenyedia(payload: { kode: string }) {
  const res = await api.post("/inaproc/ekatalog/list-produk-penyedia/sync", payload);
  return res.data;
}

export async function syncEkatalog6Paket(payload: { kode_klpd: string; tahun: string }) {
  const res = await api.post("/inaproc/ekatalog/paket-e-purchasing/sync", payload);
  return res.data;
}

// Menarik satu tingkat untuk satu induk: kosong = tingkat 1, kd_kategori_1 = tingkat 2, keduanya = tingkat 3.
export async function syncEkatalog6Kategori(payload: { kd_kategori_1?: string; kd_kategori_2?: string }) {
  const res = await api.post("/inaproc/ekatalog/list-kategori-produk/sync", payload);
  return res.data;
}

export async function syncEkatalog6Transaksi(payload: { tahun: string; kode_klpd?: string; kd_kategori_1?: string; kd_product?: string; status?: string }) {
  const res = await api.post("/inaproc/ekatalog/e-purchasing-by-produk/sync", payload);
  return res.data;
}

// ============ Penarikan Data Pengadaan terpadu (Pengadaan, Tender, E-Katalog V5 dan V6) ============

export type PenarikanMode = "klpd_tahun" | "klpd" | "kode" | "kategori" | "transaksi";
export type PenarikanStatusTugas = "antri" | "berjalan" | "sukses" | "gagal" | "dibatalkan" | "dilewati";

export interface PenarikanRiwayat {
  id: number;
  batch_id: string;
  dataset: string;
  parameter: string;
  pemicu: "manual" | "otomatis";
  status: PenarikanStatusTugas;
  jumlah_baris: number | null;
  baris_gagal: number | null;
  halaman: number | null;
  percobaan: number;
  pesan: string | null;
  dijalankan_oleh: string | null; // disembunyikan dari non-admin
  dibuat: string;
  mulai: string | null;
  selesai: string | null;
}

export interface PenarikanDataset {
  id: string;
  kelompok: string;
  subkelompok: string;
  nama: string;
  deskripsi: string;
  mode: PenarikanMode;
  otomatis: boolean;
  punya_klpd: boolean;
  punya_tahun: boolean;
  perlu_status: boolean;
  baris: number;
  disinkron_at: string | null;
  terakhir: PenarikanRiwayat | null;
  terakhir_sukses: PenarikanRiwayat | null;
}

export interface PenarikanTugasInfo {
  id: number;
  dataset: string;
  nama: string;
  parameter: string;
  status: PenarikanStatusTugas;
  pesan: string;
  jumlah_baris: number;
  baris_gagal: number;
  percobaan: number;
  mulai: string | null;
  selesai: string | null;
}

export interface PenarikanAktif {
  batch_id: string;
  pemicu: "manual" | "otomatis";
  oleh: string;
  mulai: string;
  dibatalkan: boolean;
  total: number;
  selesai: number;
  tugas: PenarikanTugasInfo[];
}

export interface PenarikanPengaturan {
  aktif: boolean;
  interval_hari: number;
  jam_mulai: number;
  jam_akhir: number;
  kode_klpd: string;
  jumlah_tahun: number;
  jeda_detik: number;
  dataset: string[];
  diubah: string | null;
  diubah_oleh: string;
  bawaan_server: boolean;
}

export interface PenarikanOtomatis {
  aktif: boolean;
  token_ada: boolean;
  berikutnya: string | null;
  jumlah_tugas: number;
  jatuh_tempo: number; // percobaan ulang dan tugas reguler yang jatuh tempo sekarang
  istirahat: number; // tugas yang istirahat setelah gagal berulang
  terakhir_otomatis: string | null;
  ditahan_sampai: string | null; // penarikan otomatis ditahan sementara (gangguan Inaproc/jaringan)
  maks_percobaan: number;
  istirahat_jam: number;
  zona: string;
}

// Batas permintaan ke Inaproc (1.000 per 60 detik dan 5.000 per jam; batas di server sedikit di bawahnya).
export interface PenarikanKuota {
  batas_per_menit: number;
  batas_per_jam: number;
  terpakai_menit: number;
  terpakai_jam: number;
  sisa_jam: number;
  tahan_sampai: string | null; // jeda bersama setelah Inaproc menjawab 429
  pulih_sekitar: string | null; // saat jatah mulai longgar lagi; null bila masih ada jatah
}

// Tugas yang gagal dan belum pulih: percobaan ulang otomatis, atau istirahat setelah gagal berulang.
export interface TugasBermasalah {
  dataset: string;
  nama: string;
  parameter: string;
  gagal: number;
  maks_percobaan: number;
  istirahat: boolean;
  berikutnya_sekitar: string | null;
  terakhir_gagal: string;
  pesan: string;
  dalam_rencana: boolean;
}

export interface PenarikanStatus {
  token_ada: boolean;
  aktif: PenarikanAktif | null;
  otomatis: PenarikanOtomatis;
  kuota: PenarikanKuota;
  bermasalah: TugasBermasalah[] | null; // null = tidak ada (backend lama mengirim null untuk daftar kosong); baca lewat daftarAman
  pengaturan: PenarikanPengaturan;
  kelompok: { id: string; nama: string }[];
  datasets: PenarikanDataset[];
  riwayat: PenarikanRiwayat[];
  kode_klpd: string;
  tahun_ini: number;
  tahun_bawaan: string[];
}

export interface PermintaanTugas {
  dataset: string;
  kode_klpd?: string;
  tahun?: string;
  kode?: string;
  status?: string;
  jenis_paket?: string;
  kd_kategori_1?: string;
  kd_kategori_2?: string;
  kd_product?: string;
}

export interface PermintaanPenarikan {
  tugas?: PermintaanTugas[];
  datasets?: string[];
  semua_otomatis?: boolean;
  tahun?: string[];
  kode_klpd?: string;
}

type Amplop<T> = { success: boolean; message: string; data: T };

export async function getPenarikanStatus() {
  const res = await api.get<Amplop<PenarikanStatus>>("/inaproc/penarikan");
  return res.data;
}

// Ringan: hanya antrean yang sedang berjalan, untuk polling kemajuan.
export async function getPenarikanAktif() {
  const res = await api.get<Amplop<{ aktif: PenarikanAktif | null; kuota: PenarikanKuota }>>("/inaproc/penarikan/aktif");
  return res.data;
}

export async function getPenarikanRiwayat(params: { dataset?: string; limit?: number }) {
  const res = await api.get<Amplop<PenarikanRiwayat[]>>("/inaproc/penarikan/riwayat", { params });
  return res.data;
}

// Khusus admin. Penarikan berjalan di server; respons langsung kembali (202) dan kemajuannya dibaca lewat getPenarikanAktif.
export async function startPenarikan(body: PermintaanPenarikan) {
  const res = await api.post<Amplop<{ aktif: PenarikanAktif }>>("/inaproc/penarikan", body);
  return res.data;
}

export async function cancelPenarikan() {
  const res = await api.post<Amplop<{ dibatalkan: boolean }>>("/inaproc/penarikan/batal");
  return res.data;
}

export async function savePenarikanPengaturan(body: Omit<PenarikanPengaturan, "diubah" | "diubah_oleh" | "bawaan_server">) {
  const res = await api.put<Amplop<{ pengaturan: PenarikanPengaturan; otomatis: PenarikanOtomatis }>>("/inaproc/penarikan/pengaturan", body);
  return res.data;
}

// ---- Data lokal dan ekspor ----

export interface DataKolom {
  nama: string;
  label: string;
  jenis: "teks" | "angka" | "tanggal";
}

export interface DataHalaman {
  dataset: string;
  kolom: DataKolom[];
  baris: (string | number | boolean | null)[][];
  total: number;
  halaman: number;
  per_halaman: number;
  tahun_tersedia?: string[];
}

export interface PenyaringData {
  kode_klpd?: string;
  tahun?: string;
  cari?: string;
}

export async function getInaprocData(dataset: string, params: PenyaringData & { halaman?: number; per_halaman?: number }) {
  const res = await api.get<Amplop<DataHalaman>>(`/inaproc/data/${dataset}`, { params });
  return res.data;
}

export type FormatEkspor = "xlsx" | "csv" | "pdf";
export type PemisahCsv = "titik-koma" | "koma" | "tab";

export async function eksporInaprocData(dataset: string, params: PenyaringData & { format: FormatEkspor; pemisah?: PemisahCsv }) {
  return ambilBerkas(`/inaproc/ekspor/${dataset}`, params);
}

// Unduhan data Digitalisasi Aset: penyaringnya sama dengan daftar (tanpa halaman). CSV memakai pemisah titik koma (bawaan server, untuk Excel Indonesia).
export async function eksporDigitalisasi(dataset: DGDatasetKey, params: Omit<DGListParams, "page" | "per_page"> & { format: FormatEkspor }) {
  return ambilBerkas(`/digitalisasi/ekspor/${dataset}`, params);
}

// Berkas diambil lewat axios (butuh header Authorization) lalu disimpan dari blob. Galat dari server datang sebagai blob JSON; pesannya dibaca di sini.
async function ambilBerkas(url: string, params: object) {
  try {
    const res = await api.get<Blob>(url, { params, responseType: "blob" });
    return { blob: res.data, disposition: String(res.headers["content-disposition"] ?? "") };
  } catch (err) {
    const data = (err as { response?: { data?: unknown } })?.response?.data;
    if (data instanceof Blob) {
      try {
        const msg = JSON.parse(await data.text())?.message;
        if (typeof msg === "string" && msg) throw new Error(msg);
      } catch (inner) {
        if (inner instanceof Error && inner.message && !(inner instanceof SyntaxError)) throw inner;
      }
    }
    throw err;
  }
}

// ---- Dasbor analitik ----

export interface AnPasangan {
  label: string;
  jumlah: number;
  nilai: number;
}

export interface AnBulan {
  bulan: number;
  jumlah: number;
  nilai: number;
}

export interface AnRUP {
  total_paket: number;
  total_pagu: number;
  paket_swakelola: number;
  pagu_swakelola: number;
  pagu_program: number;
  status_umumkan: AnPasangan[];
  per_metode: AnPasangan[];
  per_jenis: AnPasangan[];
  top_satker: AnPasangan[];
  per_bulan_pemilihan: AnBulan[];
  status_ukm: AnPasangan[];
  status_pdn: AnPasangan[];
}

export interface AnPemilihan {
  tender_jumlah: number;
  tender_pagu: number;
  tender_hps: number;
  non_tender_jumlah: number;
  non_tender_pagu: number;
  non_tender_hps: number;
  tender_selesai: number;
  nilai_kontrak: number;
  status_tender: AnPasangan[];
  metode_tender: AnPasangan[];
  metode_non_tender: AnPasangan[];
  jenis_tender: AnPasangan[];
  per_bulan_tender: AnBulan[];
  per_bulan_non_tender: AnBulan[];
  efisiensi: { sampel: number; total_hps: number; total_kontrak: number; persen: number; median: number; sebaran: AnPasangan[] };
  persaingan: { tender_berpeserta: number; satu_peserta: number; rata_peserta: number; sebaran: AnPasangan[] };
  waktu_proses: { sampel: number; median: number; rata: number };
  pasar: { jumlah_penyedia: number; total_nilai: number; hhi: number; top: AnPasangan[] };
}

export interface AnKontrakBerakhir {
  no_kontrak: string;
  nama_paket: string;
  penyedia: string;
  nilai: number;
  berakhir: string;
  sisa_hari: number;
  jenis: string;
}

export interface AnKontrak {
  tender_jumlah: number;
  tender_nilai: number;
  non_tender_jumlah: number;
  non_tender_nilai: number;
  status: AnPasangan[];
  addendum: number;
  per_bulan: AnBulan[];
  berakhir_dalam: number;
  nilai_berakhir: number;
  akan_berakhir: AnKontrakBerakhir[] | null;
  hari_peringatan: number;
}

export interface AnEkatalog {
  v5: { paket: number; nilai: number; per_bulan: AnBulan[]; top_komoditas: AnPasangan[]; top_penyedia: AnPasangan[]; status: AnPasangan[] };
  v6: {
    order: number;
    nilai: number;
    order_swasta: number;
    nilai_swasta: number;
    per_bulan: AnBulan[];
    top_penyedia: AnPasangan[];
    status: AnPasangan[];
    transaksi_nilai: number;
    transaksi_baris: number;
    top_kategori: AnPasangan[];
  };
}

export interface AnCorong {
  total_paket: number;
  total_pagu: number;
  tahap: AnPasangan[];
}

export interface AnPembanding {
  tahun: string;
  rup_paket: number;
  rup_pagu: number;
  tender_jumlah: number;
  nilai_kontrak: number;
}

export interface AnHasil {
  tahun: string;
  kode_klpd: string;
  rup: AnRUP | null;
  pemilihan: AnPemilihan | null;
  kontrak: AnKontrak | null;
  ekatalog: AnEkatalog | null;
  corong: AnCorong | null;
  pembanding: AnPembanding | null;
  dataset_kosong: string[] | null;
  terakhir_tarik: string | null;
  galat: Record<string, string> | null;
}

export type TingkatWawasan = "penting" | "perhatian" | "info" | "baik";

export interface AnWawasan {
  bagian: string;
  tingkat: TingkatWawasan;
  judul: string;
  isi: string;
}

export interface AnalitikResponse {
  hasil: AnHasil;
  wawasan: AnWawasan[];
  tahun_tersedia: string[];
  dibuat: string;
  dari_cache: boolean;
}

export async function getAnalitik(params: { tahun?: string; kode_klpd?: string; segarkan?: boolean }) {
  const res = await api.get<Amplop<AnalitikResponse>>("/inaproc/analitik", {
    params: { tahun: params.tahun || undefined, kode_klpd: params.kode_klpd || undefined, segarkan: params.segarkan ? 1 : undefined },
  });
  return res.data;
}
