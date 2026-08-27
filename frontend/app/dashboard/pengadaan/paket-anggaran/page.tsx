import { PaketAnggaranTable } from "@/components/inaproc/PaketAnggaranTable";

export default function PaketAnggaranPage() {
  return (
    <div className="w-full space-y-6">
      <div className="rounded-xl bg-white p-6 shadow-sm">
        <h1 className="text-xl font-bold text-slate-900">Paket Anggaran Penyedia (Inaproc)</h1>
        <p className="mt-1 text-sm text-slate-500">
          Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time
        </p>
      </div>
      <div className="rounded-xl bg-white p-6 shadow-sm">
        <PaketAnggaranTable />
      </div>
    </div>
  );
}