import { PaketSwakelolaTerumumkanTable } from "@/components/inaproc/PaketSwakelolaTerumumkanTable";
import { PageShell } from "@/components/ui/PageHeader";
import { ClipboardCheck } from "lucide-react";

export default function PaketSwakelolaTerumumkanPage() {
  return (
    <PageShell
      title="Paket Swakelola Terumumkan (Inaproc)"
      icon={ClipboardCheck}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <PaketSwakelolaTerumumkanTable />
    </PageShell>
  );
}