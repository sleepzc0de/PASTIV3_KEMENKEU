import { CircleX, TriangleAlert } from "lucide-react";
import type { LevelLog } from "@/lib/logServer";
import { KELAS_LEVEL, LABEL_LEVEL } from "@/lib/logServerFilter";

// Lencana level log: ikon dan teks selalu tampil (warna saja tidak cukup).
export function LencanaLevel({ level }: { level: LevelLog }) {
  const Ikon = level === "error" ? CircleX : TriangleAlert;
  return (
    <span className={`inline-flex shrink-0 items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-semibold ring-1 ring-inset ${KELAS_LEVEL[level]}`}>
      <Ikon className="h-3.5 w-3.5" aria-hidden="true" />
      {LABEL_LEVEL[level]}
    </span>
  );
}

// Penanda hitungan kejadian yang digabung ("× 1.204"); tidak tampil bila hanya sekali.
export function LencanaJumlah({ teks }: { teks: string }) {
  if (!teks) return null;
  return <span className="inline-flex shrink-0 rounded-md bg-slate-100 px-1.5 py-0.5 text-[11px] font-semibold tabular-nums text-slate-600">{teks}</span>;
}
