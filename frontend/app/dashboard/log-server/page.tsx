import { ServerCrash } from "lucide-react";
import { LogServerView } from "@/components/logserver/LogServerView";
import { PageShell } from "@/components/ui/PageHeader";

export default function LogServerPage() {
  return (
    <PageShell
      title="Log Server"
      icon={ServerCrash}
      description="Galat (error) dan peringatan (warning) dari server, digabung bila berulang. Info, debug, dan log akses tidak disimpan. Khusus superadmin dan hanya dapat dibaca."
    >
      <LogServerView />
    </PageShell>
  );
}
