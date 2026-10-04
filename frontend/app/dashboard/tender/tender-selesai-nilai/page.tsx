import { TenderSelesaiNilaiTable } from "@/components/inaproc/TenderSelesaiNilaiTable";
import { PageShell } from "@/components/ui/PageHeader";
import { Coins } from "lucide-react";

export default function TenderSelesaiNilaiPage() {
  return (
    <PageShell
      title="Nilai Tender Selesai (Inaproc)"
      icon={Coins}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <TenderSelesaiNilaiTable />
    </PageShell>
  );
}
