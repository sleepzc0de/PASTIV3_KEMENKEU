import { KeyRound } from "lucide-react";
import { ApiAksesView } from "@/components/apiakses/ApiAksesView";
import { PageShell } from "@/components/ui/PageHeader";

export default function ApiAksesPage() {
  return (
    <PageShell
      title="API Akses Data"
      icon={KeyRound}
      description="Aplikasi lain mendaftar, disetujui superadmin, lalu menarik data PASTI lewat kunci API dengan scope, batas laju, dan kuota yang dapat diatur. Khusus superadmin."
    >
      <ApiAksesView />
    </PageShell>
  );
}
