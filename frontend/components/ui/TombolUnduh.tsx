"use client";

import { FileText, Loader2 } from "lucide-react";
import type { FormatEkspor } from "@/lib/api";

// Tombol unduh satu format (Excel, CSV, PDF). `sibuk` = format yang sedang disiapkan (null bila tidak ada); selama ada yang disiapkan, semua tombol
// dikunci supaya tidak ada dua unduhan besar sekaligus. Dipakai halaman Penarikan Data dan Digitalisasi Aset.
export function TombolUnduh({
  format,
  label,
  ikon: Ikon,
  sibuk,
  nonaktif,
  onKlik,
  judul,
}: {
  format: FormatEkspor;
  label: string;
  ikon: typeof FileText;
  sibuk: FormatEkspor | null;
  nonaktif: boolean;
  onKlik: (f: FormatEkspor) => void;
  judul?: string;
}) {
  const aktif = sibuk === format;
  return (
    <button
      type="button"
      onClick={() => onKlik(format)}
      disabled={nonaktif || sibuk !== null}
      title={judul}
      className="inline-flex items-center gap-2 rounded-lg border border-slate-300 bg-white px-3.5 py-2 text-sm font-medium text-slate-700 shadow-sm hover:border-slate-400 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-55"
    >
      {aktif ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" /> : <Ikon className="h-4 w-4" aria-hidden="true" />}
      {aktif ? "Menyiapkan..." : label}
    </button>
  );
}
