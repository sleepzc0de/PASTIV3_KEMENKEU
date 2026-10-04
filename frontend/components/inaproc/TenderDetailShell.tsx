"use client";

import { ReactNode, useEffect } from "react";
import { X, MapPin } from "lucide-react";

interface Props {
  judul: string; // judul dialog, mis. "Detail Non Tender Selesai"
  subjudul: string;
  nama: string | null; // judul ringkasan di atas: nama paket (atau nama pelaksana bila respons tidak memuat nama paket)
  lokasi?: string | null; // nama satker
  onClose: () => void;
  children: ReactNode; // kelompok-kelompok field (DetailGroup)
}

// Kerangka dialog detail untuk halaman Tender: judul, ringkasan paket, tutup lewat tombol atau Escape.
export function TenderDetailShell({ judul, subjudul, nama, lokasi, onClose, children }: Props) {
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  return (
    // `!mt-0`: modal ini dirender di dalam wadah `space-y-*` yang memberi margin-top pada
    // setiap anaknya; tanpa ini overlay `fixed inset-0` bergeser dan menyisakan celah di atas.
    <div className="fixed inset-0 z-50 !mt-0 flex animate-fade-in items-end justify-center bg-slate-950/50 p-0 backdrop-blur-sm sm:items-center sm:p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-label={judul}
        className="max-h-[90dvh] w-full max-w-2xl overflow-y-auto animate-scale-in rounded-t-3xl bg-white shadow-xl sm:rounded-2xl"
      >
        <div className="sticky top-0 flex items-start justify-between gap-3 border-b border-slate-100 bg-white/90 px-4 py-4 backdrop-blur sm:px-6">
          <div className="min-w-0">
            <h2 className="text-base font-semibold text-slate-900">{judul}</h2>
            <p className="mt-0.5 text-xs text-slate-500">{subjudul}</p>
          </div>
          <button onClick={onClose} aria-label="Tutup" className="shrink-0 rounded-md p-1.5 text-slate-400 hover:bg-slate-100">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="space-y-6 p-4 sm:p-6">
          <div className="space-y-2 rounded-lg border border-slate-100 bg-slate-50 p-4">
            <p className="text-sm font-semibold text-slate-900">{nama || "-"}</p>
            {lokasi && (
              <p className="flex items-start gap-2 text-xs text-slate-500">
                <MapPin className="mt-0.5 h-3.5 w-3.5 shrink-0 text-slate-400" />
                {lokasi}
              </p>
            )}
          </div>
          {children}
        </div>
      </div>
    </div>
  );
}
