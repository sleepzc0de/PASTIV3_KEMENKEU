import { EkatalogPaketTable } from "@/components/inaproc/EkatalogPaketTable";
import { PageShell } from "@/components/ui/PageHeader";
import { ShoppingBag } from "lucide-react";

export default function EkatalogPaketPage() {
  return (
    <PageShell
      title="Paket E-Purchasing (Inaproc)"
      icon={ShoppingBag}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <EkatalogPaketTable />
    </PageShell>
  );
}
