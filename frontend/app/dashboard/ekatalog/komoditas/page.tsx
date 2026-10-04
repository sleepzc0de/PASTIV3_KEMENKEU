import { EkatalogKomoditasTable } from "@/components/inaproc/EkatalogKomoditasTable";
import { PageShell } from "@/components/ui/PageHeader";
import { Tags } from "lucide-react";

export default function EkatalogKomoditasPage() {
  return (
    <PageShell
      title="Komoditas E-Katalog (Inaproc)"
      icon={Tags}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <EkatalogKomoditasTable />
    </PageShell>
  );
}
