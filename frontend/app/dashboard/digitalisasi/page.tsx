import { DigitalisasiWorkspace } from "@/components/digitalisasi/DigitalisasiWorkspace";

export default function DigitalisasiPage() {
  return (
    <div className="w-full space-y-6">
      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <h1 className="text-xl font-bold text-slate-900">Digitalisasi Aset</h1>
        <p className="mt-1 text-sm text-slate-500">
          Tanah, gedung, rusunara, rumah negara, mess, dan satuan kerja KL 015 yang disalin dari SLDK: ringkasan analitik, peta sebaran aset, daftar,
          dan sinkronisasi. Data dibaca dari salinan di database PASTI, bukan langsung dari SLDK.
        </p>
      </div>

      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <DigitalisasiWorkspace />
      </div>
    </div>
  );
}
