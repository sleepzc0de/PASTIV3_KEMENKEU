import { CircleCheck, CircleX, ShieldAlert } from "lucide-react";
import { type KelasHasil, LABEL_HASIL } from "@/lib/auditFilter";

const KELAS: Record<KelasHasil, string> = {
  berhasil: "bg-emerald-50 text-emerald-700 ring-emerald-200",
  gagal: "bg-red-50 text-red-700 ring-red-200",
  ditolak: "bg-amber-50 text-amber-800 ring-amber-200",
};

// Lencana hasil aktivitas: ikon dan teks selalu tampil (warna saja tidak cukup).
export function LencanaHasil({ kelas }: { kelas: KelasHasil }) {
  const Ikon = kelas === "berhasil" ? CircleCheck : kelas === "gagal" ? CircleX : ShieldAlert;
  return (
    <span className={`inline-flex shrink-0 items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-semibold ring-1 ring-inset ${KELAS[kelas]}`}>
      <Ikon className="h-3.5 w-3.5" aria-hidden="true" />
      {LABEL_HASIL[kelas]}
    </span>
  );
}
