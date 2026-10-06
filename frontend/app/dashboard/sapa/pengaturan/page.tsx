import { Settings2 } from "lucide-react";
import { PengaturanSapa } from "@/components/sapa/PengaturanSapa";
import { SapaGate } from "@/components/sapa/SapaGate";
import { PageShell } from "@/components/ui/PageHeader";

export default function SapaPengaturanPage() {
  return (
    <PageShell
      title="Pengaturan SAPA"
      icon={Settings2}
      description="Template dokumen Word, jenis dan satuan BMN, dan referensi Unit Eselon I. Khusus admin. Peran pengguna diatur di Manajemen Pengguna."
    >
      <SapaGate adminOnly>
        <PengaturanSapa />
      </SapaGate>
    </PageShell>
  );
}
