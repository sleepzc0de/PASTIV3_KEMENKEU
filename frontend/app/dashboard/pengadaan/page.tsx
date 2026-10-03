import { HistoryKajiUlangTable } from "@/components/inaproc/HistoryKajiUlangTable";
import { PageShell } from "@/components/ui/PageHeader";
import { FileClock } from "lucide-react";

export default function PengadaanPage() {
  return (
    <PageShell
      title="Riwayat Kaji Ulang RUP (Inaproc)"
      icon={FileClock}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <HistoryKajiUlangTable />
    </PageShell>
  );
}