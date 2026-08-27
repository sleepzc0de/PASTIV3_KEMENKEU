import { HistoryKajiUlangTable } from "@/components/inaproc/HistoryKajiUlangTable";

export default function PengadaanPage() {
  return (
    <div className="w-full space-y-6">
      <div className="rounded-xl bg-white p-6 shadow-sm">
        <h1 className="text-xl font-bold text-slate-900">Riwayat Kaji Ulang RUP (Inaproc)</h1>
        <p className="mt-1 text-sm text-slate-500">
          Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time
        </p>
      </div>
      <div className="rounded-xl bg-white p-6 shadow-sm">
        <HistoryKajiUlangTable />
      </div>
    </div>
  );
}