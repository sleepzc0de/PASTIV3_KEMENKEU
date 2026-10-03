import { Ban, Check, CircleDashed, ExternalLink, Minus, PenLine } from "lucide-react";
import type { SapaStatusTahap } from "@/lib/sapa";
import { STATUS_META, Tone, peranLabel } from "./sapa";

const PERAN_CLS: Record<string, string> = {
  satker: "bg-blue-50 text-blue-700 ring-blue-200",
  kanwil: "bg-violet-50 text-violet-700 ring-violet-200",
  ue1: "bg-teal-50 text-teal-700 ring-teal-200",
};

export function PeranBadge({ peran }: { peran: string }) {
  return (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium ring-1 ring-inset ${PERAN_CLS[peran] ?? "bg-slate-50 text-slate-600 ring-slate-200"}`}>
      {peranLabel(peran)}
    </span>
  );
}

export function KanalBadge({ kanal }: { kanal: string }) {
  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-600">
      <ExternalLink className="h-3 w-3" aria-hidden="true" />
      {kanal}
    </span>
  );
}

const STATUS_CLS: Record<Tone, string> = {
  ok: "bg-emerald-50 text-emerald-700",
  draft: "bg-amber-50 text-amber-700",
  skip: "bg-slate-100 text-slate-600",
  wait: "bg-slate-100 text-slate-500",
  run: "bg-blue-50 text-blue-700",
};

const STATUS_ICON: Record<SapaStatusTahap, typeof Check> = {
  selesai: Check,
  dilewati: Ban,
  draft: PenLine,
  belum: CircleDashed,
};

// Status selalu memakai ikon + teks, bukan warna saja.
export function StatusBadge({ status }: { status: SapaStatusTahap }) {
  const meta = STATUS_META[status];
  const Icon = STATUS_ICON[status];
  return (
    <span className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_CLS[meta.tone]}`}>
      <Icon className="h-3.5 w-3.5" aria-hidden="true" />
      {meta.label}
    </span>
  );
}

// Lingkaran nomor pada linimasa tahap. `sm` untuk ringkasan alur.
export function StepCircle({ no, status, aktif, size = "md" }: { no: number; status: SapaStatusTahap; aktif: boolean; size?: "sm" | "md" }) {
  const base = size === "sm" ? "flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold" : "flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-sm font-semibold";
  const icon = size === "sm" ? "h-3.5 w-3.5" : "h-4 w-4";
  if (status === "selesai")
    return (
      <span className={`${base} bg-emerald-600 text-white`}>
        <Check className={icon} aria-hidden="true" />
      </span>
    );
  if (status === "dilewati")
    return (
      <span className={`${base} bg-slate-400 text-white`}>
        <Minus className={icon} aria-hidden="true" />
      </span>
    );
  if (status === "draft") return <span className={`${base} bg-amber-500 text-white`}>{no}</span>;
  return <span className={`${base} ${aktif ? "bg-blue-600 text-white ring-4 ring-blue-100" : "bg-slate-200 text-slate-500"}`}>{no}</span>;
}