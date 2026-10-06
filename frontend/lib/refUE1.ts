// Fungsi murni untuk referensi Unit Eselon I (tanpa React, jadi bisa diuji dengan Node). Aturannya sama dengan RefUE1Baris.Label di backend
// (backend/handlers/ref_ue1.go) supaya label di layar sama dengan label di grafik dan berkas ekspor.
import type { RefUE1 } from "./api";

export type PetaUE1 = Record<string, RefUE1>;

export function petaUE1(daftar: RefUE1[]): PetaUE1 {
  const peta: PetaUE1 = {};
  for (const r of daftar) peta[r.kode] = r;
  return peta;
}

const kosong = (kode: string | null | undefined) => !kode || kode === "(kosong)";

// "01504 · DJP"; tanpa singkatan "01504 · DIREKTORAT JENDERAL PAJAK"; kode yang belum terdaftar "UE1 01504"; kode kosong "(kosong)".
export function labelUE1(kode: string | null | undefined, peta: PetaUE1): string {
  if (kosong(kode)) return "(kosong)";
  const r = peta[kode as string];
  if (r?.singkatan) return `${kode} · ${r.singkatan}`;
  if (r?.nama) return `${kode} · ${r.nama}`;
  return `UE1 ${kode}`;
}

// Uraian lengkap, atau kode itu sendiri bila belum terdaftar.
export function namaUE1(kode: string | null | undefined, peta: PetaUE1): string {
  if (kosong(kode)) return "(kosong)";
  return peta[kode as string]?.nama || (kode as string);
}

// Kode tampil bersama uraiannya untuk kolom "Kode UE1" pada rincian: "01504 · DIREKTORAT JENDERAL PAJAK (DJP)".
export function kodeDenganUraian(kode: string | null | undefined, peta: PetaUE1): string {
  if (kosong(kode)) return "(kosong)";
  const r = peta[kode as string];
  if (!r?.nama) return kode as string;
  return r.singkatan ? `${kode} · ${r.nama} (${r.singkatan})` : `${kode} · ${r.nama}`;
}

// Pilihan untuk daftar bawah (select): kode yang dikirim tetap kode aslinya, labelnya dari referensi. Urutan mengikuti `kode`.
export function opsiUE1(kode: string[], peta: PetaUE1): { value: string; label: string }[] {
  return kode.map((k) => ({ value: k, label: labelUE1(k, peta) }));
}
