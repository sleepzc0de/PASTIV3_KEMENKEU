import { ChartNoAxesCombined } from "lucide-react";
import { DasborWorkspace } from "@/components/pengadaan/DasborWorkspace";
import { PageShell } from "@/components/ui/PageHeader";

export default function DasborPengadaanPage() {
  return (
    <PageShell
      title="Dasbor Pengadaan"
      icon={ChartNoAxesCombined}
      description="Perencanaan (RUP), pemilihan tender dan non-tender, kontrak, dan e-purchasing E-Katalog V5 dan V6 dalam satu tampilan yang saling terhubung, lengkap dengan wawasan analitik. Dibaca dari salinan data Inaproc di database PASTI."
    >
      <DasborWorkspace />
    </PageShell>
  );
}
