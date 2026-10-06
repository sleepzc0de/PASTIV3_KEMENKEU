import { Suspense } from "react";
import { MapPinned } from "lucide-react";
import { DigitalisasiWorkspace } from "@/components/digitalisasi/DigitalisasiWorkspace";
import { PageShell } from "@/components/ui/PageHeader";

export default function DigitalisasiPage() {
  return (
    <PageShell
      title="Digitalisasi Aset"
      icon={MapPinned}
      description="Tanah, gedung, rusunara, rumah negara, mess, dan satuan kerja KL 015 yang disalin dari SLDK: ringkasan analitik, peta sebaran aset, daftar, dan sinkronisasi. Data dibaca dari salinan di database PASTI, bukan langsung dari SLDK."
    >
      <Suspense fallback={<div role="status" aria-label="Memuat Digitalisasi Aset" className="h-64 animate-pulse rounded-2xl bg-slate-100" />}>
        <DigitalisasiWorkspace />
      </Suspense>
    </PageShell>
  );
}
