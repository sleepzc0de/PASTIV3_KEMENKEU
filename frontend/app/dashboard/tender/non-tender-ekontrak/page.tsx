import { NonTenderEkontrakTable } from "@/components/inaproc/NonTenderEkontrakTable";
import { PageShell } from "@/components/ui/PageHeader";
import { FileSignature } from "lucide-react";

export default function NonTenderEkontrakPage() {
  return (
    <PageShell
      title="Non Tender E-Kontrak (Inaproc)"
      icon={FileSignature}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <NonTenderEkontrakTable />
    </PageShell>
  );
}