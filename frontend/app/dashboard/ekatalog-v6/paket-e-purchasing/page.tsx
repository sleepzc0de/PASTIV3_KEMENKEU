import { Ekatalog6PaketTable } from "@/components/inaproc/Ekatalog6PaketTable";
import { PageShell } from "@/components/ui/PageHeader";
import { ShoppingBag } from "lucide-react";

export default function Ekatalog6PaketPage() {
  return (
    <PageShell
      title="Paket E-Purchasing V6 (Inaproc)"
      icon={ShoppingBag}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <Ekatalog6PaketTable />
    </PageShell>
  );
}
