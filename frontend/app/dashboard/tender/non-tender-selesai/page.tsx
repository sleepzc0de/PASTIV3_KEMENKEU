import { NonTenderSelesaiTable } from "@/components/inaproc/NonTenderSelesaiTable";
import { PageShell } from "@/components/ui/PageHeader";
import { BadgeCheck } from "lucide-react";

export default function NonTenderSelesaiPage() {
  return (
    <PageShell
      title="Non Tender Selesai (Inaproc)"
      icon={BadgeCheck}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <NonTenderSelesaiTable />
    </PageShell>
  );
}
