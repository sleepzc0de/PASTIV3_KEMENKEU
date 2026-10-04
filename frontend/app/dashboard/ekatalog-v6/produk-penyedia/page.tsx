import { Ekatalog6ProdukPenyediaTable } from "@/components/inaproc/Ekatalog6ProdukPenyediaTable";
import { PageShell } from "@/components/ui/PageHeader";
import { PackageSearch } from "lucide-react";

export default function Ekatalog6ProdukPenyediaPage() {
  return (
    <PageShell
      title="Produk Penyedia E-Katalog V6 (Inaproc)"
      icon={PackageSearch}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <Ekatalog6ProdukPenyediaTable />
    </PageShell>
  );
}
