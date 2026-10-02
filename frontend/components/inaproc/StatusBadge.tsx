// Badge status paket/kontrak dari Inaproc. Nilai yang memuat "selesai" diberi warna hijau;
// nilai lain netral karena daftar status dari API tidak terdokumentasi lengkap.
export function StatusBadge({ status }: { status?: string | null }) {
  if (!status) return <span className="text-slate-400">-</span>;
  const done = status.toLowerCase().includes("selesai");
  return (
    <span
      className={`whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium ${
        done ? "bg-green-50 text-green-700" : "bg-slate-100 text-slate-600"
      }`}
    >
      {status}
    </span>
  );
}
