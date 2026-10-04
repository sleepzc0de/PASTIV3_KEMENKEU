import { Ekatalog6KategoriBrowser } from "@/components/inaproc/Ekatalog6KategoriBrowser";
import { PageShell } from "@/components/ui/PageHeader";
import { FolderTree } from "lucide-react";

export default function Ekatalog6KategoriPage() {
  return (
    <PageShell
      title="Kategori Produk E-Katalog V6 (Inaproc)"
      icon={FolderTree}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <Ekatalog6KategoriBrowser />
    </PageShell>
  );
}
