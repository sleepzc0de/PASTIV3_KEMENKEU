import { DatabaseBackup } from "lucide-react";
import { GerbangSemuaData } from "@/components/layout/GerbangSemuaData";
import { PenarikanWorkspace } from "@/components/pengadaan/PenarikanWorkspace";
import { PageShell } from "@/components/ui/PageHeader";

export default function PenarikanDataPage() {
  return (
    <PageShell
      title="Pengadaan Terpadu"
      icon={DatabaseBackup}
      description="Satu tempat untuk menarik data Pengadaan (RUP), Tender, E-Katalog V5, dan E-Katalog V6 dari Inaproc, secara manual maupun otomatis, lalu melihat dan mengekspornya ke Excel, CSV, atau PDF."
    >
      <GerbangSemuaData>
        <PenarikanWorkspace />
      </GerbangSemuaData>
    </PageShell>
  );
}
