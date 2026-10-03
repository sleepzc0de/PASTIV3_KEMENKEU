import Link from "next/link";
import { PenjualanDetail } from "@/components/sapa/PenjualanDetail";
import { SapaGate } from "@/components/sapa/SapaGate";

export default async function SapaPenjualanDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const nomor = /^\d{1,15}$/.test(id) ? Number(id) : 0;

  return (
    <div className="w-full space-y-6">
      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <SapaGate>
          {nomor > 0 ? (
            <PenjualanDetail id={nomor} />
          ) : (
            <div className="space-y-2">
              <p className="text-sm text-slate-700">Nomor usulan tidak valid.</p>
              <Link href="/dashboard/sapa/penjualan" className="text-sm font-medium text-blue-600 hover:text-blue-700">
                Kembali ke daftar usulan
              </Link>
            </div>
          )}
        </SapaGate>
      </div>
    </div>
  );
}
