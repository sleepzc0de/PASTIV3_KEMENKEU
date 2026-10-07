import { Gauge } from "lucide-react";
import { MonitorView } from "@/components/monitor/MonitorView";
import { PageShell } from "@/components/ui/PageHeader";

export default function MonitorResourcePage() {
  return (
    <PageShell
      title="Monitor Resource"
      icon={Gauge}
      description="Apakah CPU, memori, disk, dan koneksi database masih cukup, resource mana yang perlu ditambah, dan riwayat pemakaiannya. Khusus superadmin."
    >
      <MonitorView />
    </PageShell>
  );
}
