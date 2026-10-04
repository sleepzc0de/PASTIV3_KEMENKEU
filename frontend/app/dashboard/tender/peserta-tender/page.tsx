import { PesertaTenderTable } from "@/components/inaproc/PesertaTenderTable";
import { PageShell } from "@/components/ui/PageHeader";
import { UsersRound } from "lucide-react";

export default function PesertaTenderPage() {
  return (
    <PageShell
      title="Peserta Tender (Inaproc)"
      icon={UsersRound}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <PesertaTenderTable />
    </PageShell>
  );
}
