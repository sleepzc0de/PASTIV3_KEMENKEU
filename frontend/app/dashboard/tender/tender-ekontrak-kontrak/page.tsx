import { TenderEkontrakKontrakTable } from "@/components/inaproc/TenderEkontrakKontrakTable";
import { PageShell } from "@/components/ui/PageHeader";
import { FileText } from "lucide-react";

export default function TenderEkontrakKontrakPage() {
  return (
    <PageShell
      title="Kontrak Tender (Inaproc)"
      icon={FileText}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <TenderEkontrakKontrakTable />
    </PageShell>
  );
}
