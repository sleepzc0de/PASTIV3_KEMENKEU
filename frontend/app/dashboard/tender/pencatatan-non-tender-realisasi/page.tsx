import { PencatatanNonTenderRealisasiTable } from "@/components/inaproc/PencatatanNonTenderRealisasiTable";
import { PageShell } from "@/components/ui/PageHeader";
import { Receipt } from "lucide-react";

export default function PencatatanNonTenderRealisasiPage() {
  return (
    <PageShell
      title="Realisasi Pencatatan Non Tender (Inaproc)"
      icon={Receipt}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <PencatatanNonTenderRealisasiTable />
    </PageShell>
  );
}
