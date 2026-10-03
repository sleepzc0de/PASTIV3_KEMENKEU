"use client";

import type { SapaTahapDetail } from "@/lib/sapa";
import { StepCircle } from "./badges";
import { STATUS_META, kelompokPeran, peranLabel } from "./sapa";

const PERAN_HEAD: Record<string, string> = {
  satker: "text-blue-700",
  kanwil: "text-violet-700",
  ue1: "text-teal-700",
};

// Ringkasan seluruh alur dalam satu baris: tiap pihak (Satker, Kanwil, UE1) dengan lingkaran status tahapnya. Konteks "posisi usulan
// sekarang" tetap terlihat walau halaman hanya menampilkan tahap milik satu peran. Mengetuk lingkaran membuka tahap itu.
export function AlurRingkas({
  tahap,
  aktifKunci,
  peranSaya,
  onPilih,
}: {
  tahap: SapaTahapDetail[];
  aktifKunci: string;
  peranSaya: string;
  onPilih: (t: SapaTahapDetail) => void;
}) {
  const kelompok = kelompokPeran(tahap);
  return (
    <div className="no-scrollbar -mx-1 overflow-x-auto px-1 pb-1">
      <ol aria-label="Ringkasan alur" className="flex min-w-max items-stretch gap-2">
        {kelompok.map((g) => {
          const milikSaya = g.peran === peranSaya;
          return (
            <li key={g.items[0].kunci} className={`rounded-xl border px-3 py-2.5 ${milikSaya ? "border-blue-200 bg-blue-50/60 ring-1 ring-blue-100" : "border-slate-200 bg-white"}`}>
              <p className={`mb-2 flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wide ${PERAN_HEAD[g.peran] ?? "text-slate-600"}`}>
                {peranLabel(g.peran)}
                {milikSaya && <span className="rounded-full bg-blue-600 px-1.5 py-px text-[10px] font-semibold normal-case tracking-normal text-white">Anda</span>}
              </p>
              <div className="flex items-center gap-1.5">
                {g.items.map((t, i) => (
                  <div key={t.kunci} className="flex items-center gap-1.5">
                    {i > 0 && <span aria-hidden="true" className="h-px w-2.5 bg-slate-300" />}
                    <button
                      type="button"
                      onClick={() => onPilih(t)}
                      title={`${t.urutan}. ${t.label} (${STATUS_META[t.status].label})`}
                      aria-label={`Tahap ${t.urutan}: ${t.label}, ${STATUS_META[t.status].label}${t.kunci === aktifKunci ? ", tahap berjalan" : ""}`}
                      className="rounded-full transition-transform hover:scale-110 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
                    >
                      <StepCircle no={t.urutan} status={t.status} aktif={t.kunci === aktifKunci} size="sm" />
                    </button>
                  </div>
                ))}
              </div>
            </li>
          );
        })}
      </ol>
    </div>
  );
}
