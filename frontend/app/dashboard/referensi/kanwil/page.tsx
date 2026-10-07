import { Network } from "lucide-react";
import { RefKanwilManager } from "@/components/referensi/RefKanwilManager";
import { PageShell } from "@/components/ui/PageHeader";

export default function ReferensiKanwilPage() {
  return (
    <PageShell
      title="Referensi Kantor Wilayah"
      icon={Network}
      description="Daftar kode Kanwil (9 karakter pertama kode satker) beserta uraian dan singkatannya. Diambil dari nama satker pada data aset, ditarik dari SLDK, atau diisi manual oleh superadmin."
    >
      <RefKanwilManager />
    </PageShell>
  );
}
