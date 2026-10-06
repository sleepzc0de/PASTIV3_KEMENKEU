// Fungsi murni untuk peran data (tanpa React, jadi bisa diuji dengan Node; lihat lib/peran.test.mjs). Aturan kode sama dengan backend/peran/peran.go.
import type { CakupanData, PeranBaris, PeranData, PeranInfo, SaranPeran } from "./api";

export const PERAN_LABEL: Record<string, string> = {
  superadmin: "Super Admin",
  admin: "Admin",
  pengguna_barang: "Pengguna Barang",
  ue1: "UE1",
  kanwil: "Kanwil",
  satker: "Satker",
  user: "Pengguna",
};

export const labelPeran = (p: string | null | undefined): string => (p ? (PERAN_LABEL[p] ?? p) : "");

// Peran yang bisa diberikan admin, dengan panjang kode (0 = tanpa kode).
export const PERAN_DATA: { peran: PeranData; label: string; panjang: number; contoh: string; keterangan: string }[] = [
  { peran: "pengguna_barang", label: "Pengguna Barang", panjang: 0, contoh: "", keterangan: "Melihat seluruh data (tanpa kode)" },
  { peran: "ue1", label: "UE1", panjang: 5, contoh: "01504", keterangan: "Data satker di bawah satu Unit Eselon I (5 digit pertama kode satker)" },
  { peran: "kanwil", label: "Kanwil", panjang: 9, contoh: "015040199", keterangan: "Data satker di bawah satu Kanwil (9 digit pertama kode satker)" },
  { peran: "satker", label: "Satker", panjang: 6, contoh: "119091", keterangan: "Data satu satker (kode satker 6 digit)" },
];

export function panjangKode(peran: string): number {
  return PERAN_DATA.find((p) => p.peran === peran)?.panjang ?? -1;
}

// Kode sah bila tanpa kode (Pengguna Barang) atau angka sepanjang tingkatnya.
export function kodePeranSah(peran: string, kode: string): boolean {
  const n = panjangKode(peran);
  if (n < 0) return false;
  if (n === 0) return kode.trim() === "";
  return kode.length === n && /^[0-9]+$/.test(kode);
}

// "UE1 01504", "Kanwil 015040199", "Satker 119091", "Pengguna Barang".
export function namaPeranBerkode(peran: string, kode: string): string {
  const l = labelPeran(peran);
  return kode ? `${l} ${kode}` : l;
}

export function teksCakupan(c: CakupanData | undefined): string {
  switch (c?.tingkat) {
    case "ue1":
      return `UE1 ${c.kode}`;
    case "kanwil":
      return `Kanwil ${c.kode}`;
    case "satker":
      return `Satker ${c.kode}`;
    case "kosong":
      return "Tidak ada data (belum diberi peran)";
    default:
      return "Seluruh data";
  }
}

// Tanpa info peran (belum dimuat atau galat) dianggap boleh: yang menentukan hanya backend, ini hanya untuk menyembunyikan menu yang pasti ditolak.
export function bolehSemuaData(info: PeranInfo | undefined | null): boolean {
  return !info || info.cakupan?.tingkat === "semua";
}

// Pengadaan terbuka bagi semua peran yang punya data: yang melihat seluruh data melihat semuanya, UE1/Kanwil/Satker melihat pengadaan satkernya. Hanya pengguna yang
// belum diberi peran (cakupan kosong) sementara pembatasan diwajibkan yang ditolak backend. Tanpa info peran dianggap boleh (yang menentukan hanya backend).
export function bolehLihatPengadaan(info: PeranInfo | undefined | null): boolean {
  return !info || info.cakupan?.tingkat !== "kosong";
}

export interface OpsiPeran {
  id: number | null; // null = peran bawaan akun
  label: string;
  aktif: boolean;
}

// Pilihan di pemilih peran: peran bawaan akun (admin/superadmin) lalu peran data yang dipegang. Peran yang berlaku ditandai aktif.
export function opsiPeran(info: PeranInfo | undefined | null): OpsiPeran[] {
  if (!info) return [];
  const out: OpsiPeran[] = [];
  if (info.bawaan) out.push({ id: null, label: labelPeran(info.akun_role), aktif: info.peran_id === 0 });
  for (const b of info.tersedia) out.push({ id: b.id, label: namaPeranBerkode(b.role, b.kode), aktif: b.id === info.peran_id });
  return out;
}

// Pemilih hanya berguna bila ada lebih dari satu peran untuk dipilih.
export const perluPemilih = (info: PeranInfo | undefined | null): boolean => opsiPeran(info).length >= 2;

// Peran yang tampil di samping nama pengguna: "Super Admin", "UE1 01504", atau "Pengguna" bila belum punya peran.
export function peranTampil(info: PeranInfo | undefined | null, roleAkun: string): string {
  if (!info) return PERAN_LABEL[roleAkun] ?? roleAkun;
  if (!info.peran) return PERAN_LABEL[info.role] ?? info.role;
  return namaPeranBerkode(info.peran, info.kode);
}

// Saran yang belum dimiliki pengguna (peran + kode sama persis tidak disarankan lagi).
export function saranBaru(saran: SaranPeran[], dimiliki: PeranBaris[]): SaranPeran[] {
  return saran.filter((s) => !dimiliki.some((d) => d.role === s.role && d.kode === s.kode));
}
