// Fungsi murni untuk referensi Kantor Wilayah (tanpa React, jadi bisa diuji dengan Node). Aturannya mengikuti referensi UE1 (lib/refUE1.ts).
// Kode Kanwil = 9 karakter pertama kode satker: KL 3 + UE1 2 + wilayah 4 (mis. 015040199).
import type { HasilTarikKanwil, RefKanwil, SumberRefKanwil } from "./api";

export type PetaKanwil = Record<string, RefKanwil>;

export function petaKanwil(daftar: RefKanwil[]): PetaKanwil {
  const peta: PetaKanwil = {};
  for (const r of daftar) peta[r.kode] = r;
  return peta;
}

const kosong = (kode: string | null | undefined) => !kode || kode === "(kosong)";

// Kode Kanwil dari kode satker lengkap ("015040199119091000KP" -> "015040199"); kosong bila kodenya terlalu pendek atau memuat huruf pada 9 karakter pertama.
export function kodeKanwilDariSatker(kodeSatker: string | null | undefined): string {
  const k = (kodeSatker ?? "").trim().slice(0, 9);
  return /^\d{9}$/.test(k) ? k : "";
}

// "015040199 · KW DJP JKT"; tanpa singkatan "015040199 · KANTOR WILAYAH DJP JAKARTA PUSAT"; kode yang belum terdaftar "Kanwil 015040199"; kode kosong "(kosong)".
export function labelKanwil(kode: string | null | undefined, peta: PetaKanwil): string {
  if (kosong(kode)) return "(kosong)";
  const r = peta[kode as string];
  if (r?.singkatan) return `${kode} · ${r.singkatan}`;
  if (r?.nama) return `${kode} · ${r.nama}`;
  return `Kanwil ${kode}`;
}

// Uraian lengkap, atau kode itu sendiri bila belum terdaftar.
export function namaKanwil(kode: string | null | undefined, peta: PetaKanwil): string {
  if (kosong(kode)) return "(kosong)";
  return peta[kode as string]?.nama || (kode as string);
}

// Kode bersama uraian dan singkatannya untuk rincian: "015040199 · KANTOR WILAYAH DJP JAKARTA PUSAT (KW DJP JKT)".
export function kodeKanwilDenganUraian(kode: string | null | undefined, peta: PetaKanwil): string {
  if (kosong(kode)) return "(kosong)";
  const r = peta[kode as string];
  if (!r?.nama) return kode as string;
  return r.singkatan ? `${kode} · ${r.nama} (${r.singkatan})` : `${kode} · ${r.nama}`;
}

export const LABEL_SUMBER_KANWIL: Record<SumberRefKanwil, string> = { manual: "Manual", satker: "Data satker", sldk: "SLDK" };

// Pencarian pada daftar: kode (awalan atau bagian), uraian, atau singkatan; huruf besar/kecil diabaikan, kata-kata dipisah spasi harus cocok semuanya.
export function saringKanwil(daftar: RefKanwil[], q: string): RefKanwil[] {
  const kata = q.trim().toLowerCase().split(/\s+/).filter(Boolean);
  if (kata.length === 0) return daftar;
  return daftar.filter((r) => {
    const jerami = `${r.kode} ${r.nama} ${r.singkatan}`.toLowerCase();
    return kata.every((k) => jerami.includes(k));
  });
}

// Ringkasan satu kalimat hasil penarikan dari SLDK untuk ditampilkan kepada superadmin.
export function ringkasHasilTarik(h: HasilTarikKanwil): string {
  const bagian = [`${h.dibaca} baris dibaca dari SLDK`, `${h.ditambahkan} ditambahkan`, `${h.diperbarui} diperbarui`, `${h.tanpa_perubahan} tanpa perubahan`];
  if (h.dilewati_manual > 0) bagian.push(`${h.dilewati_manual} diisi manual (tidak ditimpa)`);
  if (h.tidak_sah > 0) bagian.push(`${h.tidak_sah} dilewati karena tidak sah`);
  if (h.dipotong > 0) bagian.push(`${h.dipotong} uraian dipotong`);
  return bagian.join(", ") + ".";
}
