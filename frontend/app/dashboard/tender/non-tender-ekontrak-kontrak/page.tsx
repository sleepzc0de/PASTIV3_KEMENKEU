import { NonTenderEkontrakKontrakTable } from "@/components/inaproc/NonTenderEkontrakKontrakTable";
import { PageShell } from "@/components/ui/PageHeader";
import { ScrollText } from "lucide-react";

export default function NonTenderEkontrakKontrakPage() {
  return (
    <PageShell
      title="Kontrak Non Tender E-Kontrak (Inaproc)"
      icon={ScrollText}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <NonTenderEkontrakKontrakTable />
    </PageShell>
  );
}