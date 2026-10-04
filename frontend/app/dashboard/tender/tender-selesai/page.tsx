import { TenderSelesaiTable } from "@/components/inaproc/TenderSelesaiTable";
import { PageShell } from "@/components/ui/PageHeader";
import { Trophy } from "lucide-react";

export default function TenderSelesaiPage() {
  return (
    <PageShell
      title="Tender Selesai (Inaproc)"
      icon={Trophy}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <TenderSelesaiTable />
    </PageShell>
  );
}
