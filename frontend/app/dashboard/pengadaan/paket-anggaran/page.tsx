import { PaketAnggaranTable } from "@/components/inaproc/PaketAnggaranTable";
import { PageShell } from "@/components/ui/PageHeader";
import { Wallet } from "lucide-react";

export default function PaketAnggaranPage() {
  return (
    <PageShell
      title="Paket Anggaran Penyedia (Inaproc)"
      icon={Wallet}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <PaketAnggaranTable />
    </PageShell>
  );
}