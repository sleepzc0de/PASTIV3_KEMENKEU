import axios from "axios";
import Cookies from "js-cookie";
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
  label: string; // "01504 · DJP"; "UE1 01504" bila kodenya belum ada di referensi UE1
  nama?: string; // uraian lengkap dari referensi UE1
  singkatan?: string;
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
  kode_satker: string | null; // kode satker lengkap dari SSO Kemenkeu (kosong untuk akun non-SSO)
  satker_aset: string | null; // nama satker pada data Digitalisasi Aset yang kode 6 digitnya sama dengan kode satker SSO
  peran_data: PeranBaris[];
  tamu: boolean; // belum punya peran apa pun (dan bukan superadmin)
  setuju: boolean; // sudah menyetujui pernyataan penggunaan aplikasi versi terbaru
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
  is_active: boolean;
  password?: string;
}

export async function updateUser(userId: string, payload: UpdateUserPayload) {
  const res = await api.put(`/users/${userId}`, payload);
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

// Dataset yang boleh dibuka peran pengguna beserta jumlah barisnya dalam cakupan peran (untuk peran yang dibatasi per satker; tidak memuat keadaan penarikan).
export interface DaftarDatasetPengadaan {
  kelompok: { id: string; nama: string }[];
  datasets: PenarikanDataset[];
  kode_klpd: string;
  cakupan: CakupanData;
}

export async function getDaftarDatasetPengadaan() {
  const res = await api.get<Amplop<DaftarDatasetPengadaan>>("/inaproc/dataset");
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
  // Terisi bila dasbor hanya memuat pengadaan satker dalam cakupan peran (UE1, Kanwil, Satker); tidak_tersedia menyebut bagian yang tak dapat dibatasi per satker.
  batas: { tingkat: string; kode: string } | null;
  tidak_tersedia: string[] | null;
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


// ---- Referensi Unit Eselon I ----

export interface RefUE1 {
  kode: string;
  nama: string;
  singkatan: string;
  urutan: number;
  aktif: boolean;
  diubah_oleh?: string;
  diubah_pada?: string;
}

export interface RefUE1Daftar {
  daftar: RefUE1[];
  // Kode UE1 pada data satker Digitalisasi Aset yang belum punya referensi, beserta jumlah satkernya.
  belum_terdaftar: { kode: string; satker: number }[];
}

export async function getRefUE1() {
  const res = await api.get<Amplop<RefUE1Daftar>>("/referensi/ue1");
  return res.data;
}

// Membuat kode baru atau mengubah yang ada (admin/superadmin). urutan/aktif yang tidak dikirim tidak berubah.
export async function putRefUE1(kode: string, body: { nama: string; singkatan: string; urutan?: number; aktif?: boolean }) {
  const res = await api.put<Amplop<RefUE1>>(`/referensi/ue1/${encodeURIComponent(kode)}`, body);
  return res.data;
}

export async function deleteRefUE1(kode: string) {
  const res = await api.delete<Amplop<{ kode: string }>>(`/referensi/ue1/${encodeURIComponent(kode)}`);
  return res.data;
}

// ---- Referensi Kantor Wilayah (Kanwil) ----

// Kode Kanwil = 9 karakter pertama kode satker (KL 3 + UE1 2 + wilayah 4). sumber: "satker" (dari nama satker pada data Digitalisasi Aset), "sldk" (ditarik dari
// SLDK, DJKN.SIMAN2_R_KORWIL), atau "manual" (diisi superadmin; tidak pernah ditimpa penarikan SLDK).
export type SumberRefKanwil = "manual" | "satker" | "sldk";

export interface RefKanwil {
  kode: string;
  kode_ue1: string;
  nama: string;
  singkatan: string;
  urutan: number;
  aktif: boolean;
  sumber: SumberRefKanwil;
  status_sldk?: string; // status_korwil dari SLDK apa adanya (informasi; tidak mengubah aktif)
  diubah_oleh?: string;
  diubah_pada?: string;
}

// Kode Kanwil pada data satker Digitalisasi Aset yang belum punya referensi, beserta jumlah satkernya dan saran uraian dari nama satker (kosong bila tidak ada).
export interface KanwilBelumTerdaftar {
  kode: string;
  kode_ue1: string;
  satker: number;
  saran: string;
}

export interface RefKanwilDaftar {
  daftar: RefKanwil[];
  belum_terdaftar: KanwilBelumTerdaftar[];
  sldk_tersedia: boolean; // koneksi SLDK di server tersedia untuk penarikan
}

export interface HasilTarikKanwil {
  dibaca: number;
  ditambahkan: number;
  diperbarui: number;
  tanpa_perubahan: number;
  dilewati_manual: number;
  tidak_sah: number;
  dipotong: number;
}

export async function getRefKanwil() {
  const res = await api.get<Amplop<RefKanwilDaftar>>("/referensi/kanwil");
  return res.data;
}

// Membuat kode baru atau mengubah yang ada (superadmin); barisnya menjadi "manual". urutan/aktif yang tidak dikirim tidak berubah.
export async function putRefKanwil(kode: string, body: { nama: string; singkatan: string; urutan?: number; aktif?: boolean }) {
  const res = await api.put<Amplop<RefKanwil>>(`/referensi/kanwil/${encodeURIComponent(kode)}`, body);
  return res.data;
}

export async function deleteRefKanwil(kode: string) {
  const res = await api.delete<Amplop<{ kode: string }>>(`/referensi/kanwil/${encodeURIComponent(kode)}`);
  return res.data;
}

// Menambahkan semua kode Kanwil di data satker yang belum terdaftar dan punya saran uraian (superadmin).
export async function tambahKanwilDariSatker() {
  const res = await api.post<Amplop<{ ditambahkan: number; tanpa_saran: number }>>("/referensi/kanwil/dari-satker");
  return res.data;
}

// Menarik referensi dari SLDK (DJKN.SIMAN2_R_KORWIL, KL 015). Baris "manual" tidak ditimpa (superadmin).
export async function tarikKanwilDariSLDK() {
  const res = await api.post<Amplop<HasilTarikKanwil>>("/referensi/kanwil/tarik-sldk");
  return res.data;
}

// ---- Keterhubungan satker (aset dan pengadaan) ----

export type SatkerStatus = "terhubung" | "hanya_aset" | "hanya_pengadaan";

export interface SatkerPengadaan {
  rup_paket: number;
  rup_pagu: number;
  tender: number;
  tender_pagu: number;
  non_tender: number;
  non_tender_pagu: number;
}

export interface SatkerTerhubung {
  kode: string; // kode satker 6 digit = SUBSTRING(Kode_Satker, 10, 6) pada data aset = kd_satker_str pada Inaproc
  nama: string;
  nama_pengadaan?: string; // nama menurut Inaproc bila berbeda dari nama pada data aset
  kode_ue1: string;
  ue1: string;
  jenis: string;
  jumlah_kode_aset: number;
  aset: Partial<Record<DGDatasetKey, number>>;
  jumlah_aset: number;
  kdj: number;
  kdo: number;
  pengadaan: SatkerPengadaan;
  status: SatkerStatus;
}

export interface SatkerRingkasan {
  satker_aset: number;
  satker_pengadaan: number;
  terhubung: number;
  hanya_aset: number;
  hanya_pengadaan: number;
  aset_tanpa_kode: number;
}

export interface SatkerKeterhubungan {
  tahun: string;
  kode_klpd: string;
  tahun_tersedia: string[];
  ringkasan: SatkerRingkasan;
  satker: SatkerTerhubung[];
}

export async function getSatkerKeterhubungan(params: { tahun?: string; kode_klpd?: string }) {
  const res = await api.get<Amplop<SatkerKeterhubungan>>("/satker/keterhubungan", {
    params: { tahun: params.tahun || undefined, kode_klpd: params.kode_klpd || undefined },
  });
  return res.data;
}


// ---- Peran data pengguna (Super Admin, Pengguna Barang, UE1, Kanwil, Satker) ----

export type PeranData = "pengguna_barang" | "ue1" | "kanwil" | "satker";

export interface PeranBaris {
  id: number;
  role: PeranData;
  kode: string; // UE1 5 digit, Kanwil 9 digit, Satker 6 digit; kosong untuk Pengguna Barang
  aktif: boolean;
  label: string;
  dibuat_oleh?: string;
  dibuat_pada?: string;
}

export type TingkatCakupan = "semua" | "ue1" | "kanwil" | "satker" | "kosong";

export interface CakupanData {
  tingkat: TingkatCakupan;
  kode?: string;
}

// Peran yang berlaku bagi pengguna saat ini (dari /auth/me dan /auth/peran).
export interface PeranInfo {
  akun_role: string; // role akun: user | superadmin (superadmin hanya ditetapkan di .env)
  role: string; // hak superadmin saat ini; turun menjadi "user" selama bertindak sebagai peran data
  peran: string; // superadmin | pengguna_barang | ue1 | kanwil | satker | "" (belum punya peran = tamu)
  peran_label: string;
  peran_id: number; // 0 = peran bawaan akun
  kode: string;
  cakupan: CakupanData;
  tersedia: PeranBaris[];
  bawaan: boolean; // superadmin: boleh kembali ke peran akunnya
  tamu: boolean; // belum punya peran apa pun: belum boleh membuka fitur apa pun
}

export interface SaranPeran {
  role: PeranData;
  kode: string;
  label: string;
}

// Pernyataan penggunaan aplikasi yang harus disetujui setiap pengguna (kecuali superadmin) sebelum memakai aplikasi.
export interface Pernyataan {
  versi: string;
  judul: string;
  paragraf: string[];
  frasa: string; // kalimat yang harus diketik
  sudah: boolean;
  auth_provider: string;
  isi_profil: boolean; // akun non-SSO: nama lengkap, NIP, dan email diisi bersama persetujuan
  profil: { nama: string; nip: string; email: string };
}

export async function getPernyataan() {
  const res = await api.get<Amplop<Pernyataan>>("/auth/persetujuan");
  return res.data;
}

export interface MasukanPersetujuan {
  frasa: string;
  setuju: boolean;
  nama?: string;
  nip?: string;
  email?: string;
}

export async function setujuiPernyataan(body: MasukanPersetujuan) {
  const res = await api.post<Amplop<{ sudah: boolean }>>("/auth/persetujuan", body);
  return res.data;
}

export async function getPeranSaya() {
  const res = await api.get<Amplop<PeranInfo>>("/auth/peran");
  return res.data;
}

// id null = kembali ke peran bawaan akun.
export async function setPeranAktif(id: number | null) {
  const res = await api.post<Amplop<PeranInfo>>("/auth/peran/aktif", { id });
  return res.data;
}

export interface PeranPengguna {
  peran: PeranBaris[];
  saran: SaranPeran[];
  kode_satker_sso: string;
}

export async function getPeranPengguna(userId: string) {
  const res = await api.get<Amplop<PeranPengguna>>(`/users/${encodeURIComponent(userId)}/peran`);
  return res.data;
}

export async function tambahPeranPengguna(userId: string, body: { role: PeranData; kode: string }) {
  const res = await api.post<Amplop<PeranBaris>>(`/users/${encodeURIComponent(userId)}/peran`, body);
  return res.data;
}

export async function cabutPeranPengguna(userId: string, peranId: number) {
  const res = await api.delete<Amplop<{ id: number }>>(`/users/${encodeURIComponent(userId)}/peran/${peranId}`);
  return res.data;
}