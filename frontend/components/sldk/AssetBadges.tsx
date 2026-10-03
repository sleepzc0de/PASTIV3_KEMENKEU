import { kondisiTone, KondisiTone } from "@/components/sldk/asset";

const KONDISI_CLASS: Record<KondisiTone, string> = {
  baik: "bg-green-50 text-green-700",
  ringan: "bg-amber-50 text-amber-700",
  berat: "bg-red-50 text-red-700",
  lain: "bg-slate-100 text-slate-600",
};

// Badge kondisi aset; warnanya mengikuti NAMA kondisi dari tabel referensi, kode dipakai bila nama belum termuat.
export function KondisiBadge({ kode, nama }: { kode: string; nama: string }) {
  const label = nama || (kode ? `Kondisi ${kode}` : "");
  if (!label) return null;
  return (
    <span className={`whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium ${KONDISI_CLASS[kondisiTone(nama)]}`}>
      {label}
    </span>
  );
}

const FLAG_ALERT = new Set(["Hilang", "Ditandai rusak"]);

// Penanda yang perlu perhatian: merah untuk hilang/rusak, kuning untuk yang lain.
export function FlagBadge({ label }: { label: string }) {
  return (
    <span
      className={`whitespace-nowrap rounded-full px-2 py-0.5 text-[11px] font-medium ${
        FLAG_ALERT.has(label) ? "bg-red-50 text-red-700" : "bg-amber-50 text-amber-700"
      }`}
    >
      {label}
    </span>
  );
}
