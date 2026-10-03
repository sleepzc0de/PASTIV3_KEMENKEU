import { PenjualanList } from "@/components/sapa/PenjualanList";
import { SapaGate } from "@/components/sapa/SapaGate";

export default function SapaPenjualanPage() {
  return (
    <div className="w-full space-y-6">
      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <h1 className="text-xl font-bold text-slate-900">SAPA: Penjualan BMN</h1>
        <p className="mt-1 text-sm text-slate-500">
          Sistem Administrasi Pengelolaan Aset. Setiap usulan penjualan melewati 10 tahap dari Satuan Kerja, Kantor Wilayah, hingga Unit Eselon I. Dokumen Word (SK Tim, Berita Acara, Nota
          Dinas) disusun otomatis dari isian formulir dan dapat diunduh.
        </p>
      </div>
      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <SapaGate>
          <PenjualanList />
        </SapaGate>
      </div>
    </div>
  );
}
