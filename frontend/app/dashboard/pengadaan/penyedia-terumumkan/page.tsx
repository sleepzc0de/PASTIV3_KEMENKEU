import { PaketPenyediaTerumumkanTable } from "@/components/inaproc/PaketPenyediaTerumumkanTable";
import { PageShell } from "@/components/ui/PageHeader";
import { PackageCheck } from "lucide-react";

export default function PaketPenyediaTerumumkanPage() {
  return (
    <PageShell
      title="Paket Penyedia Terumumkan (Inaproc)"
      icon={PackageCheck}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <PaketPenyediaTerumumkanTable />
    </PageShell>
  );
}