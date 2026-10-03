import { AssetWorkspace } from "@/components/sldk/AssetWorkspace";

export default function AssetsPage() {
  return (
    <div className="w-full space-y-6">
      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <h1 className="text-xl font-bold text-slate-900">Data Aset (SLDK)</h1>
        <p className="mt-1 text-sm text-slate-500">
          Pantau dan telusuri aset BMN dari SLDK (Interchange). Pencarian membaca langsung dari SLDK, jadi kode register atau filter satuan kerja
          paling cepat. Ringkasan dan Pemantauan dibaca dari agregat hasil sinkronisasi terjadwal.
        </p>
      </div>

      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <AssetWorkspace />
      </div>
    </div>
  );
}
