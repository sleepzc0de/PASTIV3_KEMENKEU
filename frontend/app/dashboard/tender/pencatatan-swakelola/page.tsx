import { PencatatanSwakelolaTable } from "@/components/inaproc/PencatatanSwakelolaTable";
import { PageShell } from "@/components/ui/PageHeader";
import { NotebookPen } from "lucide-react";

export default function PencatatanSwakelolaPage() {
  return (
    <PageShell
      title="Pencatatan Swakelola (Inaproc)"
      icon={NotebookPen}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <PencatatanSwakelolaTable />
    </PageShell>
  );
}
