import { DatabaseZap } from "lucide-react";
import { AssetWorkspace } from "@/components/sldk/AssetWorkspace";
import { PageShell } from "@/components/ui/PageHeader";

export default function AssetsPage() {
  return (
    <PageShell
      title="Data Aset (SLDK)"
      icon={DatabaseZap}
      description="Pantau dan telusuri aset BMN dari SLDK (Interchange). Pencarian membaca langsung dari SLDK, jadi kode register atau filter satuan kerja paling cepat. Ringkasan dan Pemantauan dibaca dari agregat hasil sinkronisasi terjadwal."
    >
      <AssetWorkspace />
    </PageShell>
  );
}
