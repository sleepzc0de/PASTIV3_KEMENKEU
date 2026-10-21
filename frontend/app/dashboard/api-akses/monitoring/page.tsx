import { Suspense } from "react";
import { Activity } from "lucide-react";
import { MonitoringView } from "@/components/apiakses/MonitoringView";
import { PageShell } from "@/components/ui/PageHeader";
import { SkeletonCards } from "@/components/ui/Skeleton";

export default function PemantauanApiAksesPage() {
  return (
    <PageShell
      title="Pemantauan Penarikan Data API"
      icon={Activity}
      description="Jumlah permintaan menurut klien, endpoint, dan waktu; pemakaian kuota saat ini; galat dan permintaan yang dibatasi (429); waktu respons; dan riwayat permintaan terakhir. Khusus superadmin."
    >
      <Suspense fallback={<SkeletonCards count={4} />}>
        <MonitoringView />
      </Suspense>
    </PageShell>
  );
}
