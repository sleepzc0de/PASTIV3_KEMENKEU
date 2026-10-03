import { AssetSearch } from "@/components/sldk/AssetSearch";

export default function AssetsPage() {
  return (
    <div className="w-full space-y-6">
      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <h1 className="text-xl font-bold text-slate-900">Data Aset (SLDK)</h1>
        <p className="mt-1 text-sm text-slate-500">
          Telusuri aset BMN dari SLDK (Interchange). Data dibaca langsung dari SLDK, bukan salinan, sehingga pencarian dengan kode atau
          filter satuan kerja paling cepat.
        </p>
      </div>

      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <AssetSearch />
      </div>
    </div>
  );
}
