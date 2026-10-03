import { HandCoins } from "lucide-react";
import { PenjualanList } from "@/components/sapa/PenjualanList";
import { SapaGate } from "@/components/sapa/SapaGate";
import { PageShell } from "@/components/ui/PageHeader";

export default function SapaPenjualanPage() {
  return (
    <PageShell
      title="SAPA: Penjualan BMN"
      icon={HandCoins}
      description="Sistem Administrasi Pengelolaan Aset. Setiap usulan penjualan melewati 10 tahap dari Satuan Kerja, Kantor Wilayah, hingga Unit Eselon I. Dokumen Word (SK Tim, Berita Acara, Nota Dinas) disusun otomatis dari isian formulir dan dapat diunduh."
    >
      <SapaGate>
        <PenjualanList />
      </SapaGate>
    </PageShell>
  );
}
