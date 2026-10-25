// Fungsi murni untuk fitur API Akses Data (tanpa React dan tanpa axios, jadi bisa diuji dengan Node): label, format batas, validasi formulir klien baru, dan kueri
// pemantauan. Tipe ada di lib/apiAkses.ts.
import type { BatasEfektif, FormKlienBaru, InfoDataset, Klien, Metrik, StatusKlien } from "./apiAkses";

export const URUTAN_STATUS: StatusKlien[] = ["menunggu", "disetujui", "ditolak", "dicabut"];

export const LABEL_STATUS: Record<StatusKlien, string> = {
  menunggu: "Menunggu",
  disetujui: "Disetujui",
  ditolak: "Ditolak",
  dicabut: "Dicabut",
};

export const KETERANGAN_STATUS: Record<StatusKlien, string> = {
  menunggu: "Sisa pendaftaran dari formulir lama (sudah ditiadakan); belum ada kredensial, menunggu keputusan superadmin",
  disetujui: "Kunci API aktif sesuai scope dan batas",
  ditolak: "Pendaftaran ditolak (final); pemohon dapat mendaftar ulang",
  dicabut: "Semua kunci dihentikan; dapat diaktifkan kembali dengan kunci baru",
};

export const LABEL_HASIL: Record<string, string> = {
  ok: "Berhasil",
  galat_klien: "Galat permintaan",
  galat_server: "Galat server",
  batas_laju: "Batas laju (429)",
  kuota_habis: "Kuota habis (429)",
  tidak_sah: "Kunci tidak sah",
  ditolak_ip: "IP ditolak",
  ditolak_scope: "Di luar scope",
};

export type KelasHasil = "baik" | "peringatan" | "buruk";

// Warna lencana hasil: berhasil = baik; 429 dan ditolak = peringatan; galat server = buruk.
export function kelasHasilAkses(hasil: string): KelasHasil {
  switch (hasil) {
    case "ok":
      return "baik";
    case "galat_server":
      return "buruk";
    default:
      return "peringatan";
  }
}

export function labelHasil(hasil: string): string {
  return LABEL_HASIL[hasil] ?? hasil;
}

const formatId = (n: number) => new Intl.NumberFormat("id-ID").format(n);

export function formatJumlah(n: number | null | undefined): string {
  return n === null || n === undefined || Number.isNaN(n) ? "-" : formatId(n);
}

// 60 -> "60 detik", 3600 -> "1 jam", 5400 -> "90 menit", 86400 -> "1 hari": satuan terbesar yang pas tanpa pecahan.
export function teksPeriode(detik: number): string {
  if (!Number.isFinite(detik) || detik <= 0) return "-";
  if (detik % 86400 === 0) return `${detik / 86400} hari`;
  if (detik % 3600 === 0) return `${detik / 3600} jam`;
  if (detik % 60 === 0 && detik >= 120) return `${detik / 60} menit`;
  return `${detik} detik`;
}

// "1.000 permintaan / 60 detik · kuota 5.000 / 1 jam"
export function teksBatas(b: BatasEfektif | undefined): string {
  if (!b) return "-";
  return `${formatId(b.limit_permintaan)} permintaan / ${teksPeriode(b.limit_jendela_detik)} · kuota ${formatId(b.kuota_permintaan)} / ${teksPeriode(b.kuota_periode_detik)}`;
}

// Ringkasan override klien; kosong bila semua ikut bawaan global.
export function teksOverride(k: Pick<Klien, "limit_permintaan" | "limit_jendela_detik" | "kuota_permintaan" | "kuota_periode_detik">): string {
  const p: string[] = [];
  if (k.limit_permintaan !== null || k.limit_jendela_detik !== null) {
    p.push(`laju ${k.limit_permintaan !== null ? formatId(k.limit_permintaan) : "bawaan"} / ${k.limit_jendela_detik !== null ? teksPeriode(k.limit_jendela_detik) : "bawaan"}`);
  }
  if (k.kuota_permintaan !== null || k.kuota_periode_detik !== null) {
    p.push(`kuota ${k.kuota_permintaan !== null ? formatId(k.kuota_permintaan) : "bawaan"} / ${k.kuota_periode_detik !== null ? teksPeriode(k.kuota_periode_detik) : "bawaan"}`);
  }
  return p.join(" · ");
}

export function persen(bagian: number, total: number): number {
  return total > 0 ? (bagian / total) * 100 : 0;
}

export function teksPersen(bagian: number, total: number, desimal = 1): string {
  return `${new Intl.NumberFormat("id-ID", { minimumFractionDigits: desimal, maximumFractionDigits: desimal }).format(persen(bagian, total))}%`;
}

export type TingkatKuota = "aman" | "waspada" | "kritis";

// Pemakaian kuota: >= 80% waspada, >= 100% kritis.
export function tingkatKuota(persenKuota: number): TingkatKuota {
  if (persenKuota >= 100) return "kritis";
  if (persenKuota >= 80) return "waspada";
  return "aman";
}

export function formatMsAkses(ms: number | null | undefined): string {
  if (ms === null || ms === undefined || Number.isNaN(ms)) return "-";
  if (ms >= 1000) return `${new Intl.NumberFormat("id-ID", { maximumFractionDigits: 1 }).format(ms / 1000)} dtk`;
  return `${new Intl.NumberFormat("id-ID", { maximumFractionDigits: ms < 10 ? 1 : 0 }).format(ms)} ms`;
}

// Permintaan yang tidak berhasil dan bukan pembatasan, untuk satu tampilan "galat" ringkas.
export function jumlahGalat(m: Pick<Metrik, "galat_klien" | "galat_server" | "ditolak">): number {
  return m.galat_klien + m.galat_server + m.ditolak;
}

export function jumlahDibatasi(m: Pick<Metrik, "batas_laju" | "kuota_habis">): number {
  return m.batas_laju + m.kuota_habis;
}

// ---------------------------------------------------------------- daftar teks (IP, UE1)

// Teks isian (dipisah koma, spasi, atau baris baru) menjadi daftar tanpa kosong dan duplikat.
export function pecahDaftar(s: string): string[] {
  const out: string[] = [];
  for (const e of s.split(/[\s,;]+/)) if (e && !out.includes(e)) out.push(e);
  return out;
}

export const gabungDaftar = (d: string[] | undefined): string => (d ?? []).join(", ");

// Kode UE1 sah: tepat 5 digit.
export function galatUE1(s: string): string {
  for (const k of pecahDaftar(s)) if (!/^\d{5}$/.test(k)) return "Kode UE1 harus 5 digit angka, dipisah koma";
  return "";
}

// Pemeriksaan ringan daftar IP/CIDR di peramban (pemeriksaan sebenarnya di server): bentuk IPv4/IPv6 sederhana dengan awalan opsional.
export function galatIP(s: string): string {
  for (const e of pecahDaftar(s)) {
    const [alamat, bits] = e.split("/");
    const v4 = /^(\d{1,3})(\.\d{1,3}){3}$/.test(alamat) && alamat.split(".").every((o) => Number(o) <= 255);
    const v6 = alamat.includes(":") && /^[0-9a-fA-F:.]+$/.test(alamat);
    if (!v4 && !v6) return "Ada alamat IP yang tidak valid";
    if (bits !== undefined) {
      const n = Number(bits);
      if (!/^\d+$/.test(bits) || n < 1 || n > (v4 ? 32 : 128)) return "Ada awalan CIDR yang tidak valid (rentang /0 tidak diizinkan)";
    }
  }
  return "";
}

// ---------------------------------------------------------------- formulir klien baru (didaftarkan superadmin)

export const BATAS_FORM = { nama: 150, instansi: 200, unit: 200, pic: 150, email: 200, telepon: 40, tujuanMin: 20, tujuanMaks: 2000 } as const;

const reEmail = /^[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}$/;
const reTelepon = /^[0-9+().\- ]{6,40}$/;

// Memeriksa data aplikasi klien seperti server: hasilnya peta kolom -> pesan (kosong bila sah). Dataset, IP, UE1, dan batas diperiksa susunBodyAkses. Server tetap yang menentukan.
export function validasiKlienBaru(f: FormKlienBaru): Record<string, string> {
  const g: Record<string, string> = {};
  const rapi = (s: string) => s.trim().replace(/\s+/g, " ");
  const panjang = (s: string) => Array.from(s).length;
  const teks = (kolom: keyof FormKlienBaru, nama: string, min: number, maks: number, wajib: boolean) => {
    const v = rapi(String(f[kolom] ?? ""));
    if (!v && wajib) g[kolom] = `${nama} wajib diisi`;
    else if (v && panjang(v) < min) g[kolom] = `${nama} terlalu pendek`;
    else if (panjang(v) > maks) g[kolom] = `${nama} terlalu panjang (maksimal ${maks} karakter)`;
  };
  teks("nama_aplikasi", "Nama aplikasi", 3, BATAS_FORM.nama, true);
  teks("instansi", "Instansi", 2, BATAS_FORM.instansi, true);
  teks("unit", "Unit", 2, BATAS_FORM.unit, false);
  teks("pic_nama", "Nama PIC", 3, BATAS_FORM.pic, true);
  teks("pic_jabatan", "Jabatan PIC", 2, BATAS_FORM.pic, false);
  const email = f.pic_email.trim();
  if (!email) g.pic_email = "Email PIC wajib diisi";
  else if (email.length > BATAS_FORM.email || !reEmail.test(email)) g.pic_email = "Email PIC tidak valid";
  const tel = f.pic_telepon.trim();
  if (tel && !reTelepon.test(tel)) g.pic_telepon = "Nomor telepon tidak valid";
  const tujuan = f.tujuan.trim();
  if (!tujuan) g.tujuan = "Tujuan penggunaan wajib diisi";
  else if (panjang(tujuan) < BATAS_FORM.tujuanMin) g.tujuan = `Jelaskan tujuan penggunaan dengan lebih rinci (minimal ${BATAS_FORM.tujuanMin} karakter)`;
  else if (panjang(tujuan) > BATAS_FORM.tujuanMaks) g.tujuan = `Tujuan penggunaan terlalu panjang (maksimal ${BATAS_FORM.tujuanMaks} karakter)`;
  return g;
}

// ---------------------------------------------------------------- rentang pemantauan

export type RentangPantau = "1j" | "6j" | "24j" | "7h" | "30h" | "90h" | "1t" | "kustom";

export const RENTANG_PANTAU: { kunci: RentangPantau; label: string }[] = [
  { kunci: "1j", label: "1 jam terakhir" },
  { kunci: "6j", label: "6 jam terakhir" },
  { kunci: "24j", label: "24 jam terakhir" },
  { kunci: "7h", label: "7 hari terakhir" },
  { kunci: "30h", label: "30 hari terakhir" },
  { kunci: "90h", label: "90 hari terakhir" },
  { kunci: "1t", label: "1 tahun terakhir" },
  { kunci: "kustom", label: "Pilih tanggal…" },
];

export interface FilterPantau {
  rentang: RentangPantau;
  dari: string; // YYYY-MM-DD (WIB), untuk rentang kustom
  sampai: string;
  klien_id: string;
  endpoint: string;
}

export const FILTER_PANTAU_BAWAAN: FilterPantau = { rentang: "24j", dari: "", sampai: "", klien_id: "", endpoint: "" };

const reTanggal = /^\d{4}-\d{2}-\d{2}$/;

// Pesan galat bila rentang kustom tidak sah; kosong bila sah.
export function galatRentangPantau(f: Pick<FilterPantau, "rentang" | "dari" | "sampai">): string {
  if (f.rentang !== "kustom") return "";
  if (!f.dari || !reTanggal.test(f.dari)) return "Pilih tanggal awal";
  if (f.sampai && !reTanggal.test(f.sampai)) return "Tanggal akhir tidak valid";
  if (f.sampai && f.sampai < f.dari) return "Tanggal akhir tidak boleh sebelum tanggal awal";
  return "";
}

// Kueri untuk backend: rentang bernama, atau dari/sampai (tanggal WIB) untuk rentang kustom. Penyaring kosong tidak dikirim.
export function kueriPantau(f: FilterPantau): Record<string, string> {
  const k: Record<string, string> = {};
  if (f.rentang === "kustom") {
    if (reTanggal.test(f.dari)) k.dari = f.dari;
    if (reTanggal.test(f.sampai)) k.sampai = f.sampai;
  } else {
    k.rentang = f.rentang;
  }
  if (/^\d+$/.test(f.klien_id)) k.klien_id = f.klien_id;
  if (f.endpoint) k.endpoint = f.endpoint;
  return k;
}

// Isian angka opsional pada formulir batas: "" -> null (ikut bawaan), bilangan bulat positif -> angka, selain itu undefined (tidak sah).
export function angkaOpsional(s: string): number | null | undefined {
  const t = s.trim();
  if (t === "") return null;
  if (!/^\d+$/.test(t)) return undefined;
  const n = Number(t);
  return Number.isSafeInteger(n) && n > 0 ? n : undefined;
}

export const teksDariAngka = (n: number | null | undefined): string => (n === null || n === undefined ? "" : String(n));

// Alamat dataset pada API klien dari kodenya: "aset.tanah" -> "/aset/tanah", "pengadaan.tender_pengumuman" -> "/pengadaan/tender_pengumuman". Kode yang bentuknya tidak dikenal
// (atau kosong) jatuh ke contoh aset.tanah, jadi contoh selalu berupa alamat yang ada.
export function alamatDataset(kode: string | undefined): string {
  const m = /^(aset|pengadaan)\.([a-z0-9_]+)$/.exec(kode ?? "");
  return m ? `/${m[1]}/${m[2]}` : "/aset/tanah";
}

// Contoh perintah curl untuk pemegang kunci (kunci hanya muncul di layar penerbitan). kodeDataset: dataset pertama dalam scope klien, supaya contoh tidak berujung 403.
export function contohCurl(alamatDasar: string, kunci: string, kodeDataset?: string): string {
  return `curl -H "Authorization: Bearer ${kunci}" \\\n  "${alamatDasar}${alamatDataset(kodeDataset)}?per_halaman=100"`;
}

// ---------------------------------------------------------------- pilihan dataset per kelompok

export interface BagianDataset {
  judul: string; // subkelompok; kosong bila kelompok tidak punya subkelompok
  dataset: InfoDataset[];
}

export interface KelompokDataset {
  kelompok: string;
  judul: string;
  kode: string[]; // semua kode dataset di kelompok ini
  bagian: BagianDataset[];
}

export const JUDUL_KELOMPOK_DATA: Record<string, string> = {
  aset: "Aset (Digitalisasi Aset)",
  pengadaan: "Pengadaan (Inaproc)",
};

const URUTAN_KELOMPOK_DATA = ["aset", "pengadaan"];

// Mengelompokkan dataset untuk pilihan scope: kelompok menurut urutan tetap (aset, lalu pengadaan, lalu yang tidak dikenal), subkelompok menurut urutan kemunculan, dan
// dataset dalam subkelompok menurut urutan dari backend. Dataset tanpa kelompok (backend lama) masuk kelompok aset.
export function kelompokkanDataset(dataset: InfoDataset[]): KelompokDataset[] {
  const peta = new Map<string, KelompokDataset>();
  for (const d of dataset) {
    const kel = d.kelompok ?? "aset";
    let g = peta.get(kel);
    if (!g) {
      g = { kelompok: kel, judul: JUDUL_KELOMPOK_DATA[kel] ?? kel, kode: [], bagian: [] };
      peta.set(kel, g);
    }
    g.kode.push(d.kode);
    const judul = d.subkelompok ?? "";
    let b = g.bagian.find((x) => x.judul === judul);
    if (!b) {
      b = { judul, dataset: [] };
      g.bagian.push(b);
    }
    b.dataset.push(d);
  }
  const rank = (k: string) => {
    const i = URUTAN_KELOMPOK_DATA.indexOf(k);
    return i < 0 ? URUTAN_KELOMPOK_DATA.length : i;
  };
  return [...peta.values()].sort((a, b) => rank(a.kelompok) - rank(b.kelompok));
}

// Memilih atau mengosongkan semua dataset satu kelompok tanpa mengubah pilihan di kelompok lain.
export function alihKelompokDataset(terpilih: string[], kodeKelompok: string[], pilih: boolean): string[] {
  const kelompok = new Set(kodeKelompok);
  const lain = terpilih.filter((k) => !kelompok.has(k));
  return pilih ? [...lain, ...kodeKelompok] : lain;
}

export function jumlahTerpilih(terpilih: string[], kodeKelompok: string[]): number {
  const kelompok = new Set(kodeKelompok);
  return terpilih.filter((k) => kelompok.has(k)).length;
}

// ---------------------------------------------------------------- formulir akses dan batas klien (persetujuan dan ubah)

export interface NilaiFormAkses {
  dataset: string[];
  ip: string;
  ue1: string;
  limit: string;
  jendela: string;
  kuota: string;
  periode: string;
  catatan: string;
}

// Nilai awal: untuk klien yang menunggu, dataset diminta menjadi usulan; untuk klien yang sudah disetujui, yang diberikan sekarang.
export function nilaiAwalAkses(k: Klien, pakaiDiminta: boolean): NilaiFormAkses {
  return {
    dataset: [...(pakaiDiminta ? k.dataset_diminta : k.dataset_diberikan)],
    ip: gabungDaftar(k.ip_diizinkan),
    ue1: gabungDaftar(k.ue1_diizinkan),
    limit: teksDariAngka(k.limit_permintaan),
    jendela: teksDariAngka(k.limit_jendela_detik),
    kuota: teksDariAngka(k.kuota_permintaan),
    periode: teksDariAngka(k.kuota_periode_detik),
    catatan: pakaiDiminta ? "" : (k.catatan ?? ""),
  };
}

export interface HasilSusunAkses {
  body?: {
    dataset_diberikan: string[];
    ip_diizinkan: string;
    ue1_diizinkan: string;
    limit_permintaan: number | null;
    limit_jendela_detik: number | null;
    kuota_permintaan: number | null;
    kuota_periode_detik: number | null;
    catatan: string;
  };
  galat: Record<string, string>;
}

// Memeriksa isian lalu menyusun badan permintaan; galat per kolom bila ada yang tidak sah. Server tetap memeriksa ulang.
export function susunBodyAkses(n: NilaiFormAkses): HasilSusunAkses {
  const galat: Record<string, string> = {};
  if (n.dataset.length === 0) galat.dataset = "Pilih sedikitnya satu dataset";
  const ipG = galatIP(n.ip);
  if (ipG) galat.ip = ipG;
  const ueG = galatUE1(n.ue1);
  if (ueG) galat.ue1 = ueG;
  const angka = (kolom: string, s: string, nama: string) => {
    const v = angkaOpsional(s);
    if (v === undefined) galat[kolom] = `${nama} harus bilangan bulat positif atau dikosongkan`;
    return v ?? null;
  };
  const limit = angka("limit", n.limit, "Batas permintaan");
  const jendela = angka("jendela", n.jendela, "Jendela batas laju");
  const kuota = angka("kuota", n.kuota, "Kuota");
  const periode = angka("periode", n.periode, "Periode kuota");
  if (n.catatan.trim().length > 1000) galat.catatan = "Catatan terlalu panjang (maksimal 1000 karakter)";
  if (Object.keys(galat).length > 0) return { galat };
  return {
    galat,
    body: {
      dataset_diberikan: n.dataset,
      ip_diizinkan: pecahDaftar(n.ip).join(", "),
      ue1_diizinkan: pecahDaftar(n.ue1).join(", "),
      limit_permintaan: limit,
      limit_jendela_detik: jendela,
      kuota_permintaan: kuota,
      kuota_periode_detik: periode,
      catatan: n.catatan.trim(),
    },
  };
}
