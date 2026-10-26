"use client";

import type { ReactNode } from "react";
import { CopyButton } from "@/components/hris2/CopyButton";

// Blok kode dengan tombol salin. Dipakai di tab Dokumentasi untuk alamat, header, JSON, dan contoh kode.
export function KotakKode({ kode, judul, labelSalin }: { kode: string; judul?: string; labelSalin: string }) {
  return (
    <figure className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
      <figcaption className="flex items-center justify-between gap-2 border-b border-slate-200 bg-slate-50 px-3 py-1">
        <span className="min-w-0 truncate text-xs font-medium text-slate-500">{judul ?? "Kode"}</span>
        <CopyButton text={kode} label={labelSalin} />
      </figcaption>
      <pre className="overflow-x-auto bg-slate-900 px-3.5 py-3 text-xs leading-relaxed text-slate-100">
        <code>{kode}</code>
      </pre>
    </figure>
  );
}

// Potongan kode di dalam kalimat.
export function Kode({ children }: { children: ReactNode }) {
  return <code className="rounded bg-slate-100 px-1 py-0.5 font-mono text-[0.8em] text-slate-800">{children}</code>;
}
