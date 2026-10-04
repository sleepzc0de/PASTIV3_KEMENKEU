import { TenderPengumumanTable } from "@/components/inaproc/TenderPengumumanTable";
import { PageShell } from "@/components/ui/PageHeader";
import { Newspaper } from "lucide-react";

export default function TenderPengumumanPage() {
  return (
    <PageShell
      title="Pengumuman Tender (Inaproc)"
      icon={Newspaper}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <TenderPengumumanTable />
    </PageShell>
  );
}
