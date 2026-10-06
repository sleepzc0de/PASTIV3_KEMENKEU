"use client";

import { ReactNode } from "react";
import Link from "next/link";
import { Info } from "lucide-react";
import type { AnPasangan, AnWawasan } from "@/lib/api";
import { bagi, formatAngka, formatPersen, rupiahRingkas } from "@/lib/pengadaan";
import { BarItem } from "@/components/ui/charts";
import { WARNA } from "./grafik";

// Daftar kelompok -> item batang untuk BarList. `ukuran` menentukan yang diukur (nilai rupiah atau jumlah paket); persentase terhadap total daftar.
export function keBarItems(daftar: AnPasangan[], ukuran: "nilai" | "jumlah" = "nilai", satuan = "paket"): BarItem[] {
  const total = daftar.reduce((a, x) => a + (ukuran === "nilai" ? x.nilai : x.jumlah), 0);
  return daftar.map((x, i) => {
    const v = ukuran === "nilai" ? x.nilai : x.jumlah;
    return {
      key: `${x.label}-${i}`,
      label: x.label,
      value: v,
      display: ukuran === "nilai" ? rupiahRingkas(v) : `${formatAngka(v)} ${satuan}`,
      note: formatPersen(bagi(v, total), 0),
      muted: x.label === "Lainnya" || x.label === "(tidak diisi)",
    };
  });
}

// Baris tabel (untuk padanan tabel grafik): label, jumlah, nilai, persen.
export function keBarisTabel(daftar: AnPasangan[], satuan = "paket"): string[][] {
  const total = daftar.reduce((a, x) => a + x.nilai, 0);
  return daftar.map((x) => [x.label, `${formatAngka(x.jumlah)} ${satuan}`, rupiahRingkas(x.nilai), formatPersen(bagi(x.nilai, total), 1)]);
}

export const KOLOM_TABEL = ["Kelompok", "Jumlah", "Nilai", "Porsi"];

// Palet kategorikal bertahap untuk komposisi: biru, jingga, aqua, kuning, magenta; "Lainnya" abu-abu netral.
const PALET = ["#2a78d6", "#eb6834", "#1baf7a", "#eda100", "#e87ba4", "#4a3aa7"];

export function keKomposisi(daftar: AnPasangan[], denganJumlah = true) {
  return daftar.map((x, i) => ({
    label: x.label,
    nilai: x.nilai,
    jumlah: denganJumlah ? x.jumlah : undefined,
    warna: x.label === "Lainnya" || x.label === "(tidak diisi)" ? WARNA.netral : PALET[i % PALET.length],
  }));
}

export function KartuKosong({ judul, isi, tautan }: { judul: string; isi: ReactNode; tautan?: { href: string; label: string } }) {
  return (
    <div className="rounded-2xl border border-dashed border-slate-300 bg-white px-5 py-8 text-center">
      <Info className="mx-auto h-5 w-5 text-slate-400" aria-hidden="true" />
      <p className="mt-2 text-sm font-semibold text-slate-800">{judul}</p>
      <p className="mx-auto mt-1 max-w-xl text-xs text-slate-500">{isi}</p>
      {tautan && (
        <Link href={tautan.href} className="mt-3 inline-block text-xs font-medium text-blue-600 hover:text-blue-700">
          {tautan.label}
        </Link>
      )}
    </div>
  );
}

export function Subjudul({ children, deskripsi }: { children: ReactNode; deskripsi?: string }) {
  return (
    <div className="mb-3">
      <h2 className="text-sm font-semibold text-slate-900">{children}</h2>
      {deskripsi && <p className="mt-0.5 text-xs text-slate-500">{deskripsi}</p>}
    </div>
  );
}

export function saring(w: AnWawasan[], ...bagian: string[]): AnWawasan[] {
  return w.filter((x) => bagian.includes(x.bagian));
}

export const URUTAN_TINGKAT = { penting: 0, perhatian: 1, info: 2, baik: 3 } as const;
