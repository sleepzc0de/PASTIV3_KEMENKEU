import { EkatalogDistributorTable } from "@/components/inaproc/EkatalogDistributorTable";
import { PageShell } from "@/components/ui/PageHeader";
import { Truck } from "lucide-react";

export default function EkatalogDistributorPage() {
  return (
    <PageShell
      title="Distributor E-Katalog (Inaproc)"
      icon={Truck}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <EkatalogDistributorTable />
    </PageShell>
  );
}
