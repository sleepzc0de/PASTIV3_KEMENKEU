import { ScrollText } from "lucide-react";
import { AuditView } from "@/components/audit/AuditView";
import { PageShell } from "@/components/ui/PageHeader";

export default function LogAuditPage() {
  return (
    <PageShell
      title="Log Audit"
      icon={ScrollText}
      description="Aktivitas pengguna: siapa mengubah data, mengekspor, mencari data pegawai, login, dan mencoba fitur di luar haknya. Khusus superadmin dan hanya dapat dibaca."
    >
      <AuditView />
    </PageShell>
  );
}
