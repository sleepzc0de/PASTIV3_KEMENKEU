import { EkatalogInstansiSatkerTable } from "@/components/inaproc/EkatalogInstansiSatkerTable";
import { PageShell } from "@/components/ui/PageHeader";
import { Building2 } from "lucide-react";

export default function EkatalogInstansiSatkerPage() {
  return (
    <PageShell
      title="Instansi & Satker E-Katalog (Inaproc)"
      icon={Building2}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <EkatalogInstansiSatkerTable />
    </PageShell>
  );
}
