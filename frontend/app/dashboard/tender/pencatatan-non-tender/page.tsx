import { PencatatanNonTenderTable } from "@/components/inaproc/PencatatanNonTenderTable";
import { PageShell } from "@/components/ui/PageHeader";
import { ClipboardList } from "lucide-react";

export default function PencatatanNonTenderPage() {
  return (
    <PageShell
      title="Pencatatan Non Tender (Inaproc)"
      icon={ClipboardList}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <PencatatanNonTenderTable />
    </PageShell>
  );
}
