import { PaketAnggaranSwakelolaTable } from "@/components/inaproc/PaketAnggaranSwakelolaTable";
import { PageShell } from "@/components/ui/PageHeader";
import { WalletCards } from "lucide-react";

export default function PaketAnggaranSwakelolaPage() {
  return (
    <PageShell
      title="Paket Anggaran Swakelola (Inaproc)"
      icon={WalletCards}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <PaketAnggaranSwakelolaTable />
    </PageShell>
  );
}