import { ProgramMasterTable } from "@/components/inaproc/ProgramMasterTable";
import { PageShell } from "@/components/ui/PageHeader";
import { LayoutList } from "lucide-react";

export default function ProgramMasterPage() {
  return (
    <PageShell
      title="Program Master (Inaproc)"
      icon={LayoutList}
      description="Data diambil langsung dari API Inaproc (data.inaproc.id) secara real-time."
    >
      <ProgramMasterTable />
    </PageShell>
  );
}