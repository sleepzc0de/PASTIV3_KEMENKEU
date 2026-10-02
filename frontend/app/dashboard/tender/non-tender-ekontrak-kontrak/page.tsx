import { NonTenderEkontrakKontrakTable } from "@/components/inaproc/NonTenderEkontrakKontrakTable";

export default function NonTenderEkontrakKontrakPage() {
  return (
    <div className="w-full space-y-6">
      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <h1 className="text-xl font-bold text-slate-900">Kontrak Non Tender E-Kontrak (Inaproc)</h1>
        <p className="mt-1 text-sm text-slate-500">
          Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time
        </p>
      </div>
      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <NonTenderEkontrakKontrakTable />
      </div>
    </div>
  );
}
