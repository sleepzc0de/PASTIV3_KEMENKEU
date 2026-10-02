import { PaketPenyediaTable } from "@/components/inaproc/PaketPenyediaTable";

export default function PaketPenyediaPage() {
  return (
    <div className="w-full space-y-6">
      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <h1 className="text-xl font-bold text-slate-900">Paket Penyedia (Inaproc)</h1>
        <p className="mt-1 text-sm text-slate-500">
          Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time
        </p>
      </div>
      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <PaketPenyediaTable />
      </div>
    </div>
  );
}