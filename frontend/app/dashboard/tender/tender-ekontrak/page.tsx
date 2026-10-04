import { TenderEkontrakTable } from "@/components/inaproc/TenderEkontrakTable";
import { PageShell } from "@/components/ui/PageHeader";
import { FileCheck2 } from "lucide-react";

export default function TenderEkontrakPage() {
  return (
    <PageShell
      title="Tender E-Kontrak (Inaproc)"
      icon={FileCheck2}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <TenderEkontrakTable />
    </PageShell>
  );
}
