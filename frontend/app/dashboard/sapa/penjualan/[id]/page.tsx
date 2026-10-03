import Link from "next/link";
import { PenjualanDetail } from "@/components/sapa/PenjualanDetail";
import { SapaGate } from "@/components/sapa/SapaGate";
import { adalahUUID } from "@/components/sapa/sapa";

// Alamat usulan memakai UUID, mis. /dashboard/sapa/penjualan/6f9619ff-8b86-d011-b42d-00c04fc964ff.
export default async function SapaPenjualanDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;

  return (
    <div className="w-full">
      <div className="card p-4 sm:p-6">
        <SapaGate>
          {adalahUUID(id) ? (
            <PenjualanDetail id={id} />
          ) : (
            <div className="space-y-2">
              <p className="text-sm text-slate-700">Alamat usulan tidak valid.</p>
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
