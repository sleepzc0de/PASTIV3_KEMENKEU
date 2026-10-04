import { EkatalogPenyediaTable } from "@/components/inaproc/EkatalogPenyediaTable";
import { PageShell } from "@/components/ui/PageHeader";
import { Factory } from "lucide-react";

export default function EkatalogPenyediaPage() {
  return (
    <PageShell
      title="Penyedia E-Katalog (Inaproc)"
      icon={Factory}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <EkatalogPenyediaTable />
    </PageShell>
  );
}
