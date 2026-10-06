import { ListTree } from "lucide-react";
import { RefUE1Manager } from "@/components/referensi/RefUE1Manager";
import { PageShell } from "@/components/ui/PageHeader";

export default function ReferensiUE1Page() {
  return (
    <PageShell
      title="Referensi Unit Eselon I"
      icon={ListTree}
      description="Daftar kode UE1 beserta uraian dan singkatannya. Dikelola admin; dipakai Digitalisasi Aset, Dashboard, berkas unduhan, dan SAPA."
    >
      <RefUE1Manager />
    </PageShell>
  );
}