// Lencana status pesanan/transaksi E-Katalog (kode seperti COMPLETED, ON_PROCESS, CANCELLED_ON_REVIEW): selesai hijau, dibatalkan
// merah, sedang berjalan/menunggu kuning, selainnya netral. Teks ditampilkan dengan garis bawah diganti spasi.
export function StatusTransaksiBadge({ status }: { status?: string | null }) {
  if (!status) return <span className="text-slate-400">-</span>;
  const s = status.toUpperCase();
  const warna =
    s === "COMPLETED"
      ? "bg-green-50 text-green-700"
      : s.includes("CANCEL")
        ? "bg-red-50 text-red-700"
        : s.startsWith("ON_") || s.startsWith("WAITING") || s.startsWith("ESIGN")
          ? "bg-amber-50 text-amber-700"
          : "bg-slate-100 text-slate-600";
  return <span className={`whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium ${warna}`}>{status.replace(/_/g, " ")}</span>;
}
