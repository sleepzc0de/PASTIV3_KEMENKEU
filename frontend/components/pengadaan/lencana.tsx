"use client";

import { useEffect, useState } from "react";
import { AlertTriangle, Ban, CheckCircle2, Circle, Clock, Loader2, MinusCircle, XCircle } from "lucide-react";
import type { PenarikanStatusTugas } from "@/lib/api";
import { KesegaranData, NadaStatus, STATUS_TUGAS } from "@/lib/pengadaan";

// Kelas bersama untuk isian formulir halaman Penarikan Data.
export const KELAS_INPUT =
  "w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none placeholder:text-slate-400 hover:border-slate-400 focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 disabled:cursor-not-allowed disabled:bg-slate-50 disabled:text-slate-500";

const NADA: Record<NadaStatus, { kelas: string; Ikon: typeof CheckCircle2; putar?: boolean }> = {
  ok: { kelas: "bg-emerald-50 text-emerald-700", Ikon: CheckCircle2 },
  gagal: { kelas: "bg-red-50 text-red-700", Ikon: XCircle },
  jalan: { kelas: "bg-blue-50 text-blue-700", Ikon: Loader2, putar: true },
  tunggu: { kelas: "bg-slate-100 text-slate-600", Ikon: Clock },
  henti: { kelas: "bg-amber-50 text-amber-700", Ikon: Ban },
};

// Status selalu memakai ikon dan teks, bukan warna saja.
export function LencanaStatus({ status }: { status: PenarikanStatusTugas }) {
  const meta = STATUS_TUGAS[status];
  const { kelas, Ikon, putar } = NADA[meta.nada];
  return (
    <span className={`inline-flex shrink-0 items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${kelas}`}>
      <Ikon className={`h-3.5 w-3.5 ${putar ? "animate-spin" : ""}`} aria-hidden="true" />
      {meta.label}
    </span>
  );
}

const SEGAR: Record<KesegaranData, { label: string; kelas: string; Ikon: typeof CheckCircle2 }> = {
  segar: { label: "Segar", kelas: "bg-emerald-50 text-emerald-700", Ikon: CheckCircle2 },
  lama: { label: "Perlu diperbarui", kelas: "bg-amber-50 text-amber-700", Ikon: AlertTriangle },
  belum: { label: "Belum pernah ditarik", kelas: "bg-slate-100 text-slate-600", Ikon: Circle },
  kosong: { label: "Hasil kosong", kelas: "bg-slate-100 text-slate-600", Ikon: MinusCircle },
  gagal: { label: "Penarikan terakhir gagal", kelas: "bg-red-50 text-red-700", Ikon: XCircle },
};

export function LencanaKesegaran({ keadaan }: { keadaan: KesegaranData }) {
  const { label, kelas, Ikon } = SEGAR[keadaan];
  return (
    <span className={`inline-flex shrink-0 items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium ${kelas}`}>
      <Ikon className="h-3 w-3" aria-hidden="true" />
      {label}
    </span>
  );
}

// Jam yang berdetak tiap detik selama `aktif` (untuk lama berjalan).
export function useSekarang(aktif: boolean): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!aktif) return;
    setNow(Date.now());
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [aktif]);
  return now;
}
