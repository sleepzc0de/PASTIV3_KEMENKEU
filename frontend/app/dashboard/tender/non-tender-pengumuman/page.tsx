import { NonTenderPengumumanTable } from "@/components/inaproc/NonTenderPengumumanTable";
import { PageShell } from "@/components/ui/PageHeader";
import { Megaphone } from "lucide-react";

export default function NonTenderPengumumanPage() {
  return (
    <PageShell
      title="Pengumuman Non Tender (Inaproc)"
      icon={Megaphone}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <NonTenderPengumumanTable />
    </PageShell>
  );
}