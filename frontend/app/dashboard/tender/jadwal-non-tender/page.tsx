import { JadwalTahapanNonTenderTable } from "@/components/inaproc/JadwalTahapanNonTenderTable";
import { PageShell } from "@/components/ui/PageHeader";
import { CalendarClock } from "lucide-react";

export default function JadwalNonTenderPage() {
  return (
    <PageShell
      title="Jadwal Tahapan Non Tender (Inaproc)"
      icon={CalendarClock}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <JadwalTahapanNonTenderTable />
    </PageShell>
  );
}