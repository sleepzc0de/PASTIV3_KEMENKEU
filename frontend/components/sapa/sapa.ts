// Fungsi murni untuk halaman SAPA (tanpa React): pemformatan, pembacaan nilai uang yang sama dengan backend,
// normalisasi isian formulir, dan pembacaan galat. Dipisah dari komponen supaya mudah diuji.

import type {
  SapaAnggota,
  SapaBarang,
  SapaDataBA,
  SapaDataNDSatker,
  SapaDataNDUE1,
  SapaDataTim,
  SapaDokPendukung,
  SapaItemDokumen,
  SapaPeran,
  SapaStatusTahap,
} from "@/lib/sapa";

// ---------------------------------------------------------------- uang

// Maksimum yang diterima backend (sen).
const MAX_SEN = BigInt("900000000000000000");

// Mencerminkan ParseUang di backend (backend/sapa/format.go): "1500000", "1500000.50", atau "1.500.000,50".
// Hasil dalam sen; null bila bukan angka yang sah. BigInt dipakai karena nilai bisa melewati batas ketepatan Number.
export function parseUang(input: string): bigint | null {
  let t = input.trim();
  if (t.startsWith("Rp") || t.startsWith("rp")) t = t.slice(2);
  t = t.trim().replace(/ /g, "");
  if (t === "") return null;
  if (t.includes(",")) {
    t = t.replace(/\./g, "");
    t = t.replace(",", ".");
  } else {
    const dots = t.split(".").length - 1;
    if (dots >= 2) {
      t = t.replace(/\./g, "");
    } else if (dots === 1) {
      const i = t.indexOf(".");
      // "1.500" (tepat tiga angka di belakang titik) dibaca sebagai pemisah ribuan.
      if (t.length - i - 1 === 3 && i >= 1 && i <= 3) t = t.replace(".", "");
    }
  }
  let intPart = t;
  let frac = "";
  const dot = t.indexOf(".");
  if (dot >= 0) {
    intPart = t.slice(0, dot);
    frac = t.slice(dot + 1);
  }
  if (intPart === "" && frac === "") return null;
  if (frac.length > 2) return null;
  if (!/^[0-9]*$/.test(intPart) || !/^[0-9]*$/.test(frac)) return null;
  if (intPart.length > 16) return null;
  const rp = intPart === "" ? BigInt(0) : BigInt(intPart);
  const total = rp * BigInt(100) + BigInt(frac.padEnd(2, "0"));
  if (total > MAX_SEN) return null;
  return total;
}

function ribuan(n: bigint): string {
  return n.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ".");
}

// "Rp1.500.000,50"; selalu dua desimal, sama dengan keluaran backend.
export function formatRupiah(sen: bigint): string {
  const rp = sen / BigInt(100);
  const s = sen % BigInt(100);
  return `Rp${ribuan(rp)},${s.toString().padStart(2, "0")}`;
}

export interface TotalBarang {
  jumlah: number;
  perolehan: bigint;
  limit: bigint;
  tidakSah: number; // baris yang nilainya belum bisa dibaca sehingga belum ikut dijumlahkan
}

// Pratinjau total di formulir; angka resmi dihitung backend dari daftar yang sama.
export function totalBarang(barang: SapaBarang[]): TotalBarang {
  const t: TotalBarang = { jumlah: barang.length, perolehan: BigInt(0), limit: BigInt(0), tidakSah: 0 };
  for (const b of barang) {
    const p = parseUang(b.nilai_perolehan);
    const l = parseUang(b.nilai_limit);
    if (p === null || l === null) t.tidakSah++;
    if (p !== null) t.perolehan += p;
    if (l !== null) t.limit += l;
  }
  return t;
}

// ---------------------------------------------------------------- tampilan menurut peran

// Sudut pandang halaman usulan: "saya" = hanya tahap milik peran pengguna; "semua" = seluruh tahap; atau satu peran tertentu
// (dipakai admin, yang boleh mengerjakan semua peran).
export type Lihat = "saya" | "semua" | SapaPeran;

// Pengguna biasa langsung fokus ke tahap perannya; admin melihat seluruh alur dan boleh menyaring per peran.
export function lihatAwal(admin: boolean): Lihat {
  return admin ? "semua" : "saya";
}

// Tahap yang ditampilkan untuk sudut pandang tertentu. Tanpa peran (tidak seharusnya terjadi) semua tahap ditampilkan,
// supaya halaman tidak pernah kosong.
export function tahapDilihat<T extends { peran: string }>(tahap: T[], lihat: Lihat, peranSaya: string): T[] {
  if (lihat === "semua") return tahap;
  const peran = lihat === "saya" ? peranSaya : lihat;
  if (!peran) return tahap;
  return tahap.filter((t) => t.peran === peran);
}

export interface Giliran {
  // "selesai": semua tahap beres; "saya": tahap yang sedang berjalan milik peran pengguna; "lain": milik peran lain.
  jenis: "selesai" | "saya" | "lain";
  kunci?: string;
  label?: string;
  peran?: string;
}

// Siapa yang sedang ditunggu. Admin dianggap boleh mengerjakan tahap berjalan apa pun ("saya").
export function giliranSaatIni<T extends { kunci: string; label: string; peran: string }>(tahap: T[], tahapSaatIni: string, peranSaya: string, admin: boolean): Giliran {
  const t = tahap.find((x) => x.kunci === tahapSaatIni);
  if (!t) return { jenis: "selesai" };
  return { jenis: admin || t.peran === peranSaya ? "saya" : "lain", kunci: t.kunci, label: t.label, peran: t.peran };
}

// Kelompok tahap berurutan yang diperankan pihak yang sama (mis. Satker 1-5, Kanwil 6, UE1 7-10), untuk ringkasan alur.
export function kelompokPeran<T extends { peran: string }>(tahap: T[]): { peran: string; items: T[] }[] {
  const out: { peran: string; items: T[] }[] = [];
  for (const t of tahap) {
    const last = out[out.length - 1];
    if (last && last.peran === t.peran) last.items.push(t);
    else out.push({ peran: t.peran, items: [t] });
  }
  return out;
}

// ---------------------------------------------------------------- tanggal dan ukuran

const BULAN = ["Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"];

// "2026-10-03" -> "3 Oktober 2026". Dibaca dari teksnya langsung (bukan Date) supaya tidak bergeser oleh zona waktu.
export function formatTanggal(iso: string | null | undefined): string {
  if (!iso) return "-";
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso);
  if (!m) return iso;
  const bulan = BULAN[Number(m[2]) - 1];
  if (!bulan) return iso;
  return `${Number(m[3])} ${bulan} ${m[1]}`;
}

export function formatUkuran(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "-";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1).replace(".", ",")} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1).replace(".", ",")} MB`;
}

// ---------------------------------------------------------------- kode satker

// Hanya angka, paling banyak 18 digit. Kode di data aset berakhiran "KP" ("015010199409294002KP"): akhiran itu
// dibuang karena kode usulan adalah 18 digit pertamanya.
export function bersihKodeSatker(s: string): string {
  return s.replace(/\D/g, "").slice(0, 18);
}

// ---------------------------------------------------------------- label

export const PERAN_LABEL: Record<SapaPeran, string> = {
  satker: "Satuan Kerja",
  kanwil: "Kantor Wilayah",
  ue1: "Unit Eselon I",
};

export function peranLabel(p: string): string {
  return PERAN_LABEL[p as SapaPeran] ?? (p || "Belum ada peran");
}

export type Tone = "ok" | "run" | "wait" | "skip" | "draft";

export const STATUS_META: Record<SapaStatusTahap, { label: string; tone: Tone }> = {
  belum: { label: "Belum dikerjakan", tone: "wait" },
  draft: { label: "Draf", tone: "draft" },
  selesai: { label: "Selesai", tone: "ok" },
  dilewati: { label: "Dilewati", tone: "skip" },
};

// ---------------------------------------------------------------- galat

export interface ErrorInfo {
  message: string;
  errors: string[]; // rincian validasi per bidang (kosong bila tidak ada)
}

type AxiosLike = {
  isAxiosError?: boolean;
  code?: string;
  response?: { status?: number; data?: { message?: unknown; errors?: unknown; code?: unknown } };
};

// Membaca galat axios tanpa mengimpor axios (supaya fungsi ini murni). `message` selalu teks yang aman ditampilkan.
export function errorInfo(err: unknown, fallback: string): ErrorInfo {
  const e = err as AxiosLike | null;
  if (e && typeof e === "object" && e.isAxiosError) {
    if (e.response) {
      const d = e.response.data;
      const errors = Array.isArray(d?.errors) ? (d.errors as unknown[]).filter((x): x is string => typeof x === "string") : [];
      const message = typeof d?.message === "string" && d.message ? d.message : fallback;
      return { message, errors: errors.length > 1 ? errors : [] };
    }
    if (e.code === "ECONNABORTED") return { message: "Permintaan terlalu lama dan dihentikan", errors: [] };
  }
  return { message: "Tidak dapat terhubung ke server", errors: [] };
}

export function errorStatus(err: unknown): number | null {
  const e = err as AxiosLike | null;
  return e && typeof e === "object" && e.isAxiosError && e.response?.status ? e.response.status : null;
}

// Nama berkas dari header Content-Disposition (mendukung filename*=UTF-8''... dan filename="...").
export function namaDariDisposition(header: string, fallback: string): string {
  const star = /filename\*\s*=\s*([^']*)'[^']*'([^;]+)/i.exec(header);
  if (star) {
    try {
      return decodeURIComponent(star[2].trim());
    } catch {
      // jatuh ke bentuk biasa
    }
  }
  const plain = /filename\s*=\s*"?([^";]+)"?/i.exec(header);
  if (plain && plain[1].trim()) return plain[1].trim();
  return fallback;
}

// ---------------------------------------------------------------- normalisasi isian

// Backend mengirim slice/map kosong sebagai null dan bisa menghilangkan bidang; formulir butuh nilai lengkap.
const s = (v: unknown): string => (typeof v === "string" ? v : "");
const rec = (v: unknown): Record<string, unknown> => (v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : {});
const arr = (v: unknown): unknown[] => (Array.isArray(v) ? v : []);

export const kosongAnggota = (): SapaAnggota => ({ nama: "", jabatan: "", kedudukan: "" });

export const kosongBarang = (): SapaBarang => ({
  nama: "",
  kode: "",
  nup: "",
  lokasi: "",
  kondisi: "",
  tahun_perolehan: "",
  nilai_perolehan: "",
  nilai_limit: "",
  keterangan: "",
});

function normBarang(v: unknown): SapaBarang {
  const o = rec(v);
  return {
    nama: s(o.nama),
    kode: s(o.kode),
    nup: s(o.nup),
    lokasi: s(o.lokasi),
    kondisi: s(o.kondisi),
    tahun_perolehan: s(o.tahun_perolehan),
    nilai_perolehan: s(o.nilai_perolehan),
    nilai_limit: s(o.nilai_limit),
    keterangan: s(o.keterangan),
  };
}

export function normTim(v: unknown): SapaDataTim {
  const o = rec(v);
  const anggota = arr(o.anggota).map((a) => {
    const x = rec(a);
    return { nama: s(x.nama), jabatan: s(x.jabatan), kedudukan: s(x.kedudukan) };
  });
  return {
    jabatan_pimpinan: s(o.jabatan_pimpinan),
    jenis_tim: s(o.jenis_tim),
    masa_awal: s(o.masa_awal),
    masa_akhir: s(o.masa_akhir),
    kota: s(o.kota),
    anggota: anggota.length > 0 ? anggota : [kosongAnggota()],
  };
}

export function normBA(v: unknown): SapaDataBA {
  const o = rec(v);
  return { bentuk: s(o.bentuk), tanggal_penelitian: s(o.tanggal_penelitian), nama_tim: s(o.nama_tim) };
}

function normPenandatangan(v: unknown) {
  const o = rec(v);
  return { nama: s(o.nama), nip: s(o.nip), jabatan: s(o.jabatan) };
}

// Bawaan satu dokumen pendukung: dokumen otomatis dianggap ada (sama dengan DataNDSatker.Dok di backend).
export function dokBawaan(item: SapaItemDokumen): SapaDokPendukung {
  return { ada: item.otomatis, nomor: "", tanggal: "" };
}

export function normNDSatker(v: unknown, items: SapaItemDokumen[]): SapaDataNDSatker {
  const o = rec(v);
  const dok = rec(o.dokumen);
  const dokumen: Record<string, SapaDokPendukung> = {};
  for (const it of items) {
    const d = rec(dok[it.kunci]);
    dokumen[it.kunci] = it.kunci in dok ? { ada: d.ada === true, nomor: s(d.nomor), tanggal: s(d.tanggal) } : dokBawaan(it);
  }
  return {
    sudah_rp4: o.sudah_rp4 === true,
    tujuan_surat: s(o.tujuan_surat),
    kota: s(o.kota),
    singkatan_satker: s(o.singkatan_satker),
    jenis_bmn: s(o.jenis_bmn),
    satuan: s(o.satuan),
    alasan: s(o.alasan),
    tiket_siman: s(o.tiket_siman),
    kepala_kanwil: s(o.kepala_kanwil),
    penandatangan: normPenandatangan(o.penandatangan),
    barang: arr(o.barang).map(normBarang),
    dokumen,
  };
}

export function normNDUE1(v: unknown): SapaDataNDUE1 {
  const o = rec(v);
  return {
    nomor_nd: s(o.nomor_nd),
    tanggal_nd: s(o.tanggal_nd),
    hal_nd: s(o.hal_nd),
    sekretaris_ue1: s(o.sekretaris_ue1),
    kepala_kanwil: s(o.kepala_kanwil),
    pejabat_pengelola: s(o.pejabat_pengelola),
    penandatangan: normPenandatangan(o.penandatangan),
  };
}

// Isian yang dikirim ke backend: dokumen otomatis tidak dikirim (backend sudah menganggapnya ada); nomor dan tanggal
// dokumen yang dinyatakan tidak ada dikosongkan supaya tidak ikut tercetak.
// Baris yang sama sekali kosong (mis. baris contoh yang tidak diisi) dibuang, supaya tidak dihitung sebagai barang/anggota.
export function barangKosong(b: SapaBarang): boolean {
  return KOLOM_BARANG.every((k) => b[k].trim() === "");
}

export function muatanNDSatker(d: SapaDataNDSatker, items: SapaItemDokumen[]): SapaDataNDSatker {
  const dokumen: Record<string, SapaDokPendukung> = {};
  for (const it of items) {
    if (it.otomatis) continue;
    const x = d.dokumen[it.kunci] ?? dokBawaan(it);
    dokumen[it.kunci] = x.ada ? { ada: true, nomor: x.nomor.trim(), tanggal: x.tanggal } : { ada: false, nomor: "", tanggal: "" };
  }
  return { ...d, barang: d.barang.filter((b) => !barangKosong(b)), dokumen };
}

export function muatanTim(d: SapaDataTim): SapaDataTim {
  return { ...d, anggota: d.anggota.filter((a) => a.nama.trim() !== "" || a.jabatan.trim() !== "" || a.kedudukan.trim() !== "") };
}

// ---------------------------------------------------------------- tempel dari spreadsheet

const KOLOM_BARANG: (keyof SapaBarang)[] = [
  "nama",
  "kode",
  "nup",
  "lokasi",
  "kondisi",
  "tahun_perolehan",
  "nilai_perolehan",
  "nilai_limit",
  "keterangan",
];

export const URUTAN_KOLOM_TEMPEL = "Nama, Kode, NUP, Lokasi/Merk/Tipe, Kondisi, Tahun Perolehan, Nilai Perolehan, Nilai Limit, Keterangan";

export const MAKS_BARANG = 500;

export interface HasilTempel {
  barang: SapaBarang[];
  dilewati: number; // baris di luar batas MAKS_BARANG
}

// Mengubah teks yang ditempel dari Excel/Sheets (kolom dipisah tab) menjadi daftar barang menurut URUTAN_KOLOM_TEMPEL.
// Baris judul (nilai perolehan/limit bukan angka dan kolom pertama memuat "nama") dibuang; baris kosong dilewati.
export function parseBarangTempel(text: string): HasilTempel {
  const rows: SapaBarang[] = [];
  let dilewati = 0;
  const lines = text.replace(/\r/g, "").split("\n");
  lines.forEach((line, i) => {
    if (line.trim() === "") return;
    const cells = line.split("\t").map((c) => c.trim());
    const b = kosongBarang();
    KOLOM_BARANG.forEach((k, idx) => {
      b[k] = cells[idx] ?? "";
    });
    const judul = i === 0 && /nama/i.test(b.nama) && parseUang(b.nilai_perolehan) === null && parseUang(b.nilai_limit) === null;
    if (judul) return;
    if (rows.length >= MAKS_BARANG) {
      dilewati++;
      return;
    }
    rows.push(b);
  });
  return { barang: rows, dilewati };
}
