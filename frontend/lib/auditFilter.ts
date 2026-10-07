// Fungsi murni untuk halaman Log Audit (tanpa React dan tanpa axios, jadi bisa diuji dengan Node): rentang waktu, penyusunan kueri, format waktu WIB, dan terjemahan
// rincian entri menjadi kalimat yang mudah dibaca.
import type { EntriAudit } from "./audit";

export type RentangKunci = "1j" | "24j" | "7h" | "30h" | "90h" | "semua" | "kustom";

export const RENTANG_AUDIT: { kunci: RentangKunci; label: string; jam?: number }[] = [
  { kunci: "1j", label: "1 jam terakhir", jam: 1 },
  { kunci: "24j", label: "24 jam terakhir", jam: 24 },
  { kunci: "7h", label: "7 hari terakhir", jam: 24 * 7 },
  { kunci: "30h", label: "30 hari terakhir", jam: 24 * 30 },
  { kunci: "90h", label: "90 hari terakhir", jam: 24 * 90 },
  { kunci: "semua", label: "Semua waktu" },
  { kunci: "kustom", label: "Pilih tanggal…" },
];

export interface FilterAudit {
  rentang: RentangKunci;
  dari: string; // YYYY-MM-DD (WIB), dipakai bila rentang kustom
  sampai: string; // YYYY-MM-DD (WIB), termasuk hari itu
  kategori: string;
  aksi: string;
  hasil: "semua" | "berhasil" | "gagal";
  username: string;
  user_id: string;
  ip: string;
  q: string;
  halaman: number;
}

export const FILTER_BAWAAN: FilterAudit = { rentang: "24j", dari: "", sampai: "", kategori: "", aksi: "", hasil: "semua", username: "", user_id: "", ip: "", q: "", halaman: 1 };

export const PER_HALAMAN_AUDIT = 50;

const reTanggal = /^\d{4}-\d{2}-\d{2}$/;

// batasWaktu menghitung batas "dari" dan "sampai" yang dikirim ke backend. Rentang berjalan memakai waktu sekarang (ISO UTC); rentang kustom memakai tanggal apa adanya
// (backend membacanya sebagai tanggal WIB dan menyertakan seluruh hari terakhir). Tanggal yang tidak sah diabaikan.
export function batasWaktu(f: Pick<FilterAudit, "rentang" | "dari" | "sampai">, sekarang: Date): { dari?: string; sampai?: string } {
  if (f.rentang === "semua") return {};
  if (f.rentang === "kustom") {
    const out: { dari?: string; sampai?: string } = {};
    if (reTanggal.test(f.dari)) out.dari = f.dari;
    if (reTanggal.test(f.sampai)) out.sampai = f.sampai;
    return out;
  }
  const r = RENTANG_AUDIT.find((x) => x.kunci === f.rentang);
  if (!r?.jam) return {};
  return { dari: new Date(sekarang.getTime() - r.jam * 3600_000).toISOString() };
}

// Pesan galat bila rentang kustom terbalik; kosong bila sah.
export function galatRentang(f: Pick<FilterAudit, "rentang" | "dari" | "sampai">): string {
  if (f.rentang !== "kustom") return "";
  if (f.dari && !reTanggal.test(f.dari)) return "Tanggal awal tidak valid";
  if (f.sampai && !reTanggal.test(f.sampai)) return "Tanggal akhir tidak valid";
  if (f.dari && f.sampai && f.sampai < f.dari) return "Tanggal akhir tidak boleh sebelum tanggal awal";
  return "";
}

// kueriAudit menyusun parameter permintaan; isian kosong tidak dikirim. perHalaman hanya dikirim bila diminta (ekspor tidak memakainya).
export function kueriAudit(f: FilterAudit, sekarang: Date, perHalaman?: number): Record<string, string> {
  const k: Record<string, string> = {};
  const w = batasWaktu(f, sekarang);
  if (w.dari) k.dari = w.dari;
  if (w.sampai) k.sampai = w.sampai;
  const isi = (nama: string, nilai: string) => {
    const v = nilai.trim();
    if (v) k[nama] = v;
  };
  isi("kategori", f.kategori);
  isi("aksi", f.aksi);
  isi("username", f.username);
  isi("user_id", f.user_id);
  isi("ip", f.ip);
  isi("q", f.q);
  if (f.hasil !== "semua") k.hasil = f.hasil;
  if (perHalaman) {
    k.halaman = String(Math.max(1, Math.floor(f.halaman) || 1));
    k.per_halaman = String(perHalaman);
  }
  return k;
}

// Ada filter di luar bawaan (untuk menampilkan tombol Atur ulang)?
export function filterBerubah(f: FilterAudit): boolean {
  const b = FILTER_BAWAAN;
  return f.rentang !== b.rentang || f.kategori !== b.kategori || f.aksi !== b.aksi || f.hasil !== b.hasil || f.username.trim() !== "" || f.user_id !== "" || f.ip.trim() !== "" || f.q.trim() !== "";
}

// ---------------------------------------------------------------- waktu

const BULAN = ["Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"];
const WIB_MS = 7 * 3600_000;

function bagianWIB(iso: string): { t: number; tahun: number; bulan: number; hari: number; jam: number; menit: number; detik: number } | null {
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return null;
  const d = new Date(t + WIB_MS); // geser +7 jam lalu baca sebagai UTC: tidak bergantung pada zona waktu browser
  return { t, tahun: d.getUTCFullYear(), bulan: d.getUTCMonth(), hari: d.getUTCDate(), jam: d.getUTCHours(), menit: d.getUTCMinutes(), detik: d.getUTCSeconds() };
}

const dua = (n: number) => String(n).padStart(2, "0");

// "7 Okt 2026, 15:04:05" dalam WIB; teks yang bukan waktu dikembalikan apa adanya.
export function waktuWIB(iso: string | undefined | null, denganDetik = true): string {
  if (!iso) return "-";
  const p = bagianWIB(iso);
  if (!p) return iso;
  return `${p.hari} ${BULAN[p.bulan]} ${p.tahun}, ${dua(p.jam)}:${dua(p.menit)}${denganDetik ? ":" + dua(p.detik) : ""}`;
}

// "15:04" pada hari yang sama atau "7 Okt 15:04" (untuk sumbu grafik).
export function waktuSingkatWIB(iso: string, denganTanggal: boolean): string {
  const p = bagianWIB(iso);
  if (!p) return iso;
  return denganTanggal ? `${p.hari} ${BULAN[p.bulan]} ${dua(p.jam)}:${dua(p.menit)}` : `${dua(p.jam)}:${dua(p.menit)}`;
}

export function tanggalSingkatWIB(iso: string): string {
  const p = bagianWIB(iso);
  return p ? `${p.hari} ${BULAN[p.bulan]}` : iso;
}

// "baru saja", "3 menit lalu", "2 jam lalu", "5 hari lalu"; lebih dari 30 hari memakai tanggal.
export function teksRelatif(iso: string | undefined | null, sekarang: Date): string {
  if (!iso) return "-";
  const p = bagianWIB(iso);
  if (!p) return iso;
  const dtk = Math.floor((sekarang.getTime() - p.t) / 1000);
  if (dtk < 0) return waktuWIB(iso, false);
  if (dtk < 60) return "baru saja";
  if (dtk < 3600) return `${Math.floor(dtk / 60)} menit lalu`;
  if (dtk < 86400) return `${Math.floor(dtk / 3600)} jam lalu`;
  if (dtk < 30 * 86400) return `${Math.floor(dtk / 86400)} hari lalu`;
  return waktuWIB(iso, false);
}

// ---------------------------------------------------------------- tampilan entri

export type KelasHasil = "berhasil" | "gagal" | "ditolak";

export function kelasHasil(e: Pick<EntriAudit, "sukses" | "aksi">): KelasHasil {
  if (e.aksi === "akses.ditolak") return "ditolak";
  return e.sukses ? "berhasil" : "gagal";
}

export const LABEL_HASIL: Record<KelasHasil, string> = { berhasil: "Berhasil", gagal: "Gagal", ditolak: "Ditolak" };

export const LABEL_ALASAN: Record<string, string> = {
  permintaan_tidak_valid: "Permintaan tidak lengkap",
  captcha_salah: "Captcha salah atau kedaluwarsa",
  pengguna_tidak_ditemukan: "Pengguna tidak ditemukan",
  akun_tidak_aktif: "Akun tidak aktif",
  akun_terkunci: "Akun terkunci sementara",
  akun_dikunci: "Akun dikunci setelah percobaan gagal berulang",
  akun_sso: "Akun SSO tidak punya kata sandi lokal",
  kata_sandi_salah: "Kata sandi salah",
  sso_ditolak: "SSO gagal",
};

const LABEL_KUNCI: Record<string, string> = {
  alasan: "Alasan",
  percobaan: "Percobaan gagal ke-",
  metode: "Metode login",
  sebab: "Sebab",
  parameter: "Parameter rute",
  kueri: "Kata kunci / filter",
  remote_addr: "Alamat tersambung langsung (proxy)",
  aksi_asal: "Aksi yang ditolak",
  username: "Username",
  role: "Role",
  aktif: "Akun aktif",
  kata_sandi_diganti: "Kata sandi diganti",
  peran: "Peran",
  kode: "Kode peran",
  baru: "Peran baru",
  peran_id: "ID peran",
};

function teksNilai(v: unknown): string {
  if (v === null || v === undefined) return "-";
  if (typeof v === "boolean") return v ? "Ya" : "Tidak";
  if (typeof v === "object") {
    return Object.entries(v as Record<string, unknown>)
      .map(([k, x]) => `${k} = ${String(x)}`)
      .join(", ");
  }
  return String(v);
}

// Rincian entri sebagai pasangan [label, nilai] siap tampil; kode alasan diterjemahkan.
export function detailBaris(detail: Record<string, unknown> | undefined): [string, string][] {
  if (!detail) return [];
  return Object.entries(detail).map(([k, v]) => {
    if (k === "alasan" && typeof v === "string") return [LABEL_KUNCI.alasan, LABEL_ALASAN[v] ?? v] as [string, string];
    return [LABEL_KUNCI[k] ?? k, teksNilai(v)] as [string, string];
  });
}

// Ringkasan user agent: "Chrome · Windows"; bila tidak dikenali, dipotong.
export function ringkasUserAgent(ua: string | undefined): string {
  if (!ua) return "-";
  const peramban = /Edg\//.test(ua) ? "Edge" : /OPR\//.test(ua) ? "Opera" : /Firefox\//.test(ua) ? "Firefox" : /Chrome\//.test(ua) ? "Chrome" : /Safari\//.test(ua) ? "Safari" : /curl\//i.test(ua) ? "curl" : "";
  const sistem = /Windows/.test(ua) ? "Windows" : /Android/.test(ua) ? "Android" : /iPhone|iPad/.test(ua) ? "iOS" : /Mac OS X/.test(ua) ? "macOS" : /Linux/.test(ua) ? "Linux" : "";
  const gabung = [peramban, sistem].filter(Boolean).join(" · ");
  return gabung || (ua.length > 40 ? ua.slice(0, 40) + "…" : ua);
}

export const LABEL_PERAN: Record<string, string> = { superadmin: "Superadmin", pengguna_barang: "Pengguna Barang", ue1: "UE1", kanwil: "Kanwil", satker: "Satker", user: "Pengguna" };

export function labelPeran(p: string | undefined, kode?: string): string {
  if (!p) return "";
  const l = LABEL_PERAN[p] ?? p;
  return kode ? `${l} ${kode}` : l;
}
