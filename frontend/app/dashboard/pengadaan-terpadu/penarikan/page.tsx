import { DatabaseBackup } from "lucide-react";
import { RuangPengadaan } from "@/components/pengadaan/RuangPengadaan";
import { PageShell } from "@/components/ui/PageHeader";

export default function PenarikanDataPage() {
  return (
    <PageShell
      title="Pengadaan Terpadu"
      icon={DatabaseBackup}
      description="Data Pengadaan (RUP), Tender, E-Katalog V5, dan E-Katalog V6 dari Inaproc: lihat dan ekspor ke Excel, CSV, atau PDF. Admin juga menarik datanya, secara manual maupun otomatis."
    >
      <RuangPengadaan />
    </PageShell>
  );
}
