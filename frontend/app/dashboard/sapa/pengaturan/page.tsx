import { PengaturanSapa } from "@/components/sapa/PengaturanSapa";
import { SapaGate } from "@/components/sapa/SapaGate";

export default function SapaPengaturanPage() {
  return (
    <div className="w-full space-y-6">
      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <h1 className="text-xl font-bold text-slate-900">Pengaturan SAPA</h1>
        <p className="mt-1 text-sm text-slate-500">Template dokumen Word, peran pengguna di SAPA, dan referensi Unit Eselon I. Khusus admin.</p>
      </div>
      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <SapaGate adminOnly>
          <PengaturanSapa />
        </SapaGate>
      </div>
    </div>
  );
}
