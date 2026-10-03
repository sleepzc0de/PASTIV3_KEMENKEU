import { PaketSwakelolaTable } from "@/components/inaproc/PaketSwakelolaTable";
import { PageShell } from "@/components/ui/PageHeader";
import { Boxes } from "lucide-react";

export default function PaketSwakelolaPage() {
  return (
    <PageShell
      title="Paket Swakelola (Inaproc)"
      icon={Boxes}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <PaketSwakelolaTable />
    </PageShell>
  );
}