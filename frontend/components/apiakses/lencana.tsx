import { CircleCheck, CircleX, Clock3, ShieldAlert, ShieldOff } from "lucide-react";
import type { StatusKlien } from "@/lib/apiAkses";
import { KETERANGAN_STATUS, LABEL_STATUS, type KelasHasil, type TingkatKuota, kelasHasilAkses, labelHasil, tingkatKuota } from "@/lib/apiAksesFormat";

const KELAS_STATUS: Record<StatusKlien, string> = {
  menunggu: "bg-amber-50 text-amber-800 ring-amber-200",
  disetujui: "bg-emerald-50 text-emerald-700 ring-emerald-200",
  ditolak: "bg-slate-100 text-slate-600 ring-slate-300",
  dicabut: "bg-red-50 text-red-700 ring-red-200",
};

const IKON_STATUS = { menunggu: Clock3, disetujui: CircleCheck, ditolak: CircleX, dicabut: ShieldOff } as const;

// Lencana status klien: ikon dan teks selalu tampil (warna saja tidak cukup).
export function LencanaStatus({ status }: { status: StatusKlien }) {
  const Ikon = IKON_STATUS[status];
  return (
    <span title={KETERANGAN_STATUS[status]} className={`inline-flex shrink-0 items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-semibold ring-1 ring-inset ${KELAS_STATUS[status]}`}>
      <Ikon className="h-3.5 w-3.5" aria-hidden="true" />
      {LABEL_STATUS[status]}
    </span>
  );
}

const KELAS_HASIL: Record<KelasHasil, string> = {
  baik: "bg-emerald-50 text-emerald-700 ring-emerald-200",
  peringatan: "bg-amber-50 text-amber-800 ring-amber-200",
  buruk: "bg-red-50 text-red-700 ring-red-200",
};

export function LencanaHasilAkses({ hasil }: { hasil: string }) {
  const kelas = kelasHasilAkses(hasil);
  const Ikon = kelas === "baik" ? CircleCheck : kelas === "buruk" ? CircleX : ShieldAlert;
  return (
    <span className={`inline-flex shrink-0 items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-semibold ring-1 ring-inset ${KELAS_HASIL[kelas]}`}>
      <Ikon className="h-3.5 w-3.5" aria-hidden="true" />
      {labelHasil(hasil)}
    </span>
  );
}

const WARNA_KUOTA: Record<TingkatKuota, string> = { aman: "bg-blue-600", waspada: "bg-amber-500", kritis: "bg-red-600" };
const TEKS_KUOTA: Record<TingkatKuota, string> = { aman: "Aman", waspada: "Hampir habis", kritis: "Habis" };

// Batang pemakaian kuota dengan label teks (tingkatnya juga ditulis, bukan hanya diwarnai).
export function BatangKuota({ terpakai, total, persen }: { terpakai: number; total: number; persen: number }) {
  const tingkat = tingkatKuota(persen);
  const lebar = Math.min(100, Math.max(persen > 0 ? 1 : 0, persen));
  return (
    <div className="min-w-[8rem]">
      <div
        className="h-2.5 overflow-hidden rounded-full bg-slate-100"
        role="meter"
        aria-valuemin={0}
        aria-valuemax={total}
        aria-valuenow={Math.min(terpakai, total)}
        aria-label={`Kuota terpakai ${terpakai.toLocaleString("id-ID")} dari ${total.toLocaleString("id-ID")}`}
      >
        <div className={`h-full rounded-full ${WARNA_KUOTA[tingkat]}`} style={{ width: `${lebar}%` }} />
      </div>
      <p className="mt-1 text-[11px] tabular-nums text-slate-500">
        {terpakai.toLocaleString("id-ID")} / {total.toLocaleString("id-ID")} · {persen.toFixed(persen < 10 ? 1 : 0).replace(".", ",")}%
        {tingkat !== "aman" && <span className="ml-1 font-semibold text-slate-700">· {TEKS_KUOTA[tingkat]}</span>}
      </p>
    </div>
  );
}
