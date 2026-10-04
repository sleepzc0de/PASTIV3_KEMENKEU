import { Ekatalog6PenyediaTable } from "@/components/inaproc/Ekatalog6PenyediaTable";
import { PageShell } from "@/components/ui/PageHeader";
import { Factory } from "lucide-react";

export default function Ekatalog6PenyediaPage() {
  return (
    <PageShell
      title="Penyedia E-Katalog V6 (Inaproc)"
      icon={Factory}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <Ekatalog6PenyediaTable />
    </PageShell>
  );
}
