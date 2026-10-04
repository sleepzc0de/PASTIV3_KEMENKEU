import { Ekatalog6TransaksiTable } from "@/components/inaproc/Ekatalog6TransaksiTable";
import { PageShell } from "@/components/ui/PageHeader";
import { ChartColumn } from "lucide-react";

export default function Ekatalog6TransaksiPage() {
  return (
    <PageShell
      title="Transaksi E-Purchasing per Produk (Inaproc)"
      icon={ChartColumn}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <Ekatalog6TransaksiTable />
    </PageShell>
  );
}
