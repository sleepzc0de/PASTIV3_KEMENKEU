"use client";

import type { TitikJamLog } from "@/lib/logServer";
import { waktuSingkatWIB, waktuWIB } from "@/lib/auditFilter";
import { deretPerJam } from "@/lib/logServerFilter";

const WARNA = { error: "#d03b3b", warn: "#fab219" };
const TINGGI = 120; // px

// Batang bertumpuk per jam (error di bawah, warning di atas), tanpa pustaka. Tinggi relatif terhadap jam tersibuk; nilai persisnya ada di tooltip dan di tampilan Tabel.
export function GrafikPerJam({ titik }: { titik: TitikJamLog[] }) {
  const { maks, batang } = deretPerJam(titik);
  const totalError = titik.reduce((a, t) => a + t.error, 0);
  const totalWarn = titik.reduce((a, t) => a + t.warn, 0);
  if (maks === 0) {
    return <p className="py-8 text-center text-sm text-slate-400">Tidak ada error atau warning dalam 24 jam terakhir.</p>;
  }
  return (
    <div>
      <div className="flex items-end gap-[3px]" style={{ height: TINGGI }} role="img" aria-label={`Kejadian per jam, 24 jam terakhir: ${totalError} error dan ${totalWarn} warning. Jam tersibuk ${maks} kejadian.`}>
        {batang.map((b) => (
          <div key={b.jam} className="flex h-full min-w-0 flex-1 flex-col justify-end" title={`${waktuWIB(b.jam, false)} WIB: ${b.error} error, ${b.warn} warning`}>
            <div style={{ height: `${b.tinggiWarn}%`, backgroundColor: WARNA.warn, minHeight: b.warn > 0 ? 2 : 0 }} />
            <div className="rounded-b-[2px]" style={{ height: `${b.tinggiError}%`, backgroundColor: WARNA.error, minHeight: b.error > 0 ? 2 : 0 }} />
          </div>
        ))}
      </div>
      <div className="mt-1 flex gap-[3px] text-[10px] text-slate-400" aria-hidden="true">
        {batang.map((b, i) => (
          <span key={b.jam} className="min-w-0 flex-1 truncate text-center tabular-nums">
            {i % 4 === 0 ? waktuSingkatWIB(b.jam, false).slice(0, 2) : ""}
          </span>
        ))}
      </div>
      <p className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-slate-500">
        <span className="inline-flex items-center gap-1.5">
          <span className="h-2.5 w-2.5 rounded-sm" style={{ backgroundColor: WARNA.error }} aria-hidden="true" />
          Error
        </span>
        <span className="inline-flex items-center gap-1.5">
          <span className="h-2.5 w-2.5 rounded-sm" style={{ backgroundColor: WARNA.warn }} aria-hidden="true" />
          Warning
        </span>
        <span className="text-slate-400">jam menurut WIB; tertinggi {maks.toLocaleString("id-ID")} kejadian per jam</span>
      </p>
    </div>
  );
}
