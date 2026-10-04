import { PencatatanSwakelolaRealisasiTable } from "@/components/inaproc/PencatatanSwakelolaRealisasiTable";
import { PageShell } from "@/components/ui/PageHeader";
import { ReceiptText } from "lucide-react";

export default function PencatatanSwakelolaRealisasiPage() {
  return (
    <PageShell
      title="Realisasi Pencatatan Swakelola (Inaproc)"
      icon={ReceiptText}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <PencatatanSwakelolaRealisasiTable />
    </PageShell>
  );
}
