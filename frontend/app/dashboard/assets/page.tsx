import { AssetSearchTable } from "@/components/sldk/AssetSearchTable";

export default function AssetsPage() {
  return (
    <div className="w-full space-y-6">
      <div className="rounded-xl bg-white p-6 shadow-sm">
        <h1 className="text-xl font-bold text-slate-900">Pencarian Data Aset (SLDK)</h1>
        <p className="mt-1 text-sm text-slate-500">
          Data diambil langsung dari sistem SLDK (Interchange) secara real-time
        </p>
      </div>

      <div className="rounded-xl bg-white p-6 shadow-sm">
        <AssetSearchTable />
      </div>
    </div>
  );
}