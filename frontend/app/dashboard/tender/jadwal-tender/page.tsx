import { JadwalTahapanTenderTable } from "@/components/inaproc/JadwalTahapanTenderTable";
import { PageShell } from "@/components/ui/PageHeader";
import { CalendarRange } from "lucide-react";

export default function JadwalTenderPage() {
  return (
    <PageShell
      title="Jadwal Tahapan Tender (Inaproc)"
      icon={CalendarRange}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <JadwalTahapanTenderTable />
    </PageShell>
  );
}