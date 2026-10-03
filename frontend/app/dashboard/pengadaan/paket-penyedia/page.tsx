import { PaketPenyediaTable } from "@/components/inaproc/PaketPenyediaTable";
import { PageShell } from "@/components/ui/PageHeader";
import { Package } from "lucide-react";

export default function PaketPenyediaPage() {
  return (
    <PageShell
      title="Paket Penyedia (Inaproc)"
      icon={Package}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <PaketPenyediaTable />
    </PageShell>
  );
}