// Fungsi murni untuk halaman Log Server (tanpa React dan tanpa axios, jadi bisa diuji dengan Node): penyusunan kueri, label level, deret grafik per jam, dan catatan
// kesehatan dari keadaan perekam dan tekanan beban. Format waktu WIB dan rentang waktu memakai lib/auditFilter.ts (dipanggil oleh komponen).
import type { RentangKunci } from "./auditFilter";
import type { KetahananServer, LevelLog, PengaturanLog, StatistikPerekamLog, TitikJamLog } from "./logServer";

export type LevelFilter = "semua" | LevelLog;

export interface FilterLog {
  rentang: RentangKunci;
  dari: string; // YYYY-MM-DD (WIB), dipakai bila rentang kustom
  sampai: string; // YYYY-MM-DD (WIB), termasuk hari itu
  level: LevelFilter;
  sumber: string;
  q: string;
  halaman: number;
}

export const FILTER_BAWAAN_LOG: FilterLog = { rentang: "24j", dari: "", sampai: "", level: "semua", sumber: "", q: "", halaman: 1 };

export const PER_HALAMAN_LOG = 50;

// kueriLog menyusun parameter permintaan; isian kosong tidak dikirim. batas = hasil batasWaktu (lib/auditFilter) untuk rentang yang dipilih.
export function kueriLog(f: FilterLog, batas: { dari?: string; sampai?: string }, perHalaman?: number): Record<string, string> {
  const k: Record<string, string> = {};
  if (batas.dari) k.dari = batas.dari;
  if (batas.sampai) k.sampai = batas.sampai;
  if (f.level !== "semua") k.level = f.level;
  const sumber = f.sumber.trim();
  if (sumber) k.sumber = sumber;
  const q = f.q.trim();
  if (q) k.q = q;
  if (perHalaman) {
    k.halaman = String(Math.max(1, Math.floor(f.halaman) || 1));
    k.per_halaman = String(perHalaman);
  }
  return k;
}

// Ada filter di luar bawaan (untuk menampilkan tombol Atur ulang)?
export function filterLogBerubah(f: FilterLog): boolean {
  const b = FILTER_BAWAAN_LOG;
  return f.rentang !== b.rentang || f.level !== b.level || f.sumber.trim() !== "" || f.q.trim() !== "";
}

export const LABEL_LEVEL: Record<LevelLog, string> = { error: "Error", warn: "Warning" };

// Kelas lencana per level: ikon dan teks selalu menyertai warna (warna saja tidak cukup).
export const KELAS_LEVEL: Record<LevelLog, string> = {
  error: "bg-red-50 text-red-700 ring-red-200",
  warn: "bg-amber-50 text-amber-800 ring-amber-200",
};

// Memadatkan pesan untuk sel tabel: spasi berulang dan baris baru jadi satu spasi, dipotong pada batas tanpa memenggal kata sebisa mungkin.
export function ringkasPesan(p: string, maks = 180): string {
  const t = p.replace(/\s+/g, " ").trim();
  if (t.length <= maks) return t;
  const potong = t.slice(0, maks);
  const spasi = potong.lastIndexOf(" ");
  return `${(spasi > maks * 0.6 ? potong.slice(0, spasi) : potong).trimEnd()}…`;
}

// "× 1.204" untuk kejadian yang digabung; kosong bila hanya sekali.
export function teksJumlah(n: number): string {
  return n > 1 ? `× ${n.toLocaleString("id-ID")}` : "";
}

export interface BatangJam {
  jam: string;
  error: number;
  warn: number;
  total: number;
  tinggiError: number; // persen dari tinggi grafik
  tinggiWarn: number;
}

// deretPerJam menghitung tinggi batang bertumpuk (error di bawah, warning di atas) relatif terhadap jam tersibuk. Jam tanpa kejadian tingginya nol.
export function deretPerJam(titik: TitikJamLog[]): { maks: number; batang: BatangJam[] } {
  const maks = Math.max(0, ...titik.map((t) => t.error + t.warn));
  const batang = titik.map((t) => ({
    jam: t.jam,
    error: t.error,
    warn: t.warn,
    total: t.error + t.warn,
    tinggiError: maks > 0 ? (t.error / maks) * 100 : 0,
    tinggiWarn: maks > 0 ? (t.warn / maks) * 100 : 0,
  }));
  return { maks, batang };
}

export type TingkatCatatan = "info" | "warning";
export interface Catatan {
  tingkat: TingkatCatatan;
  teks: string;
}

const angka = (n: number) => n.toLocaleString("id-ID");

// catatanPerekam: hal yang perlu diketahui superadmin tentang penyimpanan log itu sendiri.
export function catatanPerekam(p: StatistikPerekamLog, s: PengaturanLog): Catatan[] {
  const out: Catatan[] = [];
  if (s.simpan === "off") {
    out.push({ tingkat: "warning", teks: "Penyimpanan log dimatikan (LOG_SIMPAN=off): halaman ini tidak menerima catatan baru. Galat tetap tercetak di log konsol container (docker logs)." });
  } else if (s.simpan === "error") {
    out.push({ tingkat: "info", teks: "Hanya galat (error) yang disimpan (LOG_SIMPAN=error); peringatan (warning) tidak disimpan." });
  }
  if (p.dijatuhkan > 0) {
    out.push({
      tingkat: "warning",
      teks: `${angka(p.dijatuhkan)} catatan tidak sempat disimpan sejak server menyala karena antrean penuh (badai galat atau database macet). Galat tetap tercetak di log konsol; cari akar masalahnya pada catatan dengan hitungan terbesar.`,
    });
  }
  if (p.gagal > 0) {
    out.push({ tingkat: "warning", teks: `Penulisan ke database gagal ${angka(p.gagal)} kali sejak server menyala (dicoba lagi otomatis). Periksa kesehatan database.` });
  }
  if (p.tertunda > 0) {
    out.push({ tingkat: "info", teks: `${angka(p.tertunda)} jenis kejadian sedang menunggu ditulis ke database.` });
  }
  return out;
}

// catatanKetahanan: tekanan beban sejak server menyala, dengan petunjuk apa yang perlu disesuaikan.
export function catatanKetahanan(k: KetahananServer): Catatan[] {
  const out: Catatan[] = [];
  if (k.ditolak_beban > 0) {
    out.push({
      tingkat: "warning",
      teks: `${angka(k.ditolak_beban)} permintaan ditolak (503) karena permintaan serentak melewati batas ${angka(k.maks_serentak)}. Bila sering terjadi, periksa Monitor Resource (CPU, memori, kolam database); menambah resource lalu menaikkan MAKS_PERMINTAAN_SERENTAK.`,
    });
  }
  if (k.ditolak_berat > 0) {
    out.push({
      tingkat: "warning",
      teks: `${angka(k.ditolak_berat)} permintaan ekspor/dokumen/analitik ditolak (503) karena antrean pekerjaan berat penuh (batas ${angka(k.maks_berat)} sekaligus). Minta pengguna mengekspor bergantian atau tambah CPU lalu naikkan MAKS_BERAT_SERENTAK.`,
    });
  }
  if (k.ditolak_laju > 0) {
    out.push({
      tingkat: "info",
      teks: `${angka(k.ditolak_laju)} permintaan ditolak (429) karena satu alamat IP terlalu sering mengakses. Bila pengguna yang sah terkena (banyak pengguna di balik satu IP kantor), naikkan RATE_LIMIT_UMUM_PER_DETIK.`,
    });
  }
  if (k.maks_serentak > 0 && k.serentak_puncak / k.maks_serentak >= 0.8 && k.ditolak_beban === 0) {
    out.push({ tingkat: "info", teks: `Puncak permintaan serentak ${angka(k.serentak_puncak)} sudah mendekati batas ${angka(k.maks_serentak)}.` });
  }
  if (!k.laju_aktif) {
    out.push({ tingkat: "info", teks: "Pembatas laju per IP dimatikan (RATE_LIMIT_AKTIF=false)." });
  }
  return out;
}

// Uraian apa yang disimpan, untuk keterangan di atas halaman.
export function teksPenyimpanan(s: PengaturanLog): string {
  if (s.simpan === "off") return "Penyimpanan log dimatikan.";
  const apa = s.simpan === "error" ? "Hanya galat (error)" : "Galat (error) dan peringatan (warning)";
  const hari = s.simpan === "error" ? `error disimpan ${s.retensi_error_hari} hari` : `error disimpan ${s.retensi_error_hari} hari, warning ${s.retensi_warn_hari} hari`;
  return `${apa} yang disimpan; ${hari}, paling banyak ${angka(s.maks_baris)} baris.`;
}
