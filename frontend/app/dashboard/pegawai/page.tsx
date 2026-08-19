"use client";

import { useDashboard } from "@/lib/dashboard-context";
import { PegawaiSearchTable } from "@/components/hris2/PegawaiSearchTable";
import { ShieldAlert, Loader2 } from "lucide-react";

export default function PegawaiPage() {
  const { profile, isLoadingProfile } = useDashboard();

  if (isLoadingProfile) {
    return (
      <div className="flex h-[60vh] items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-blue-600" />
      </div>
    );
  }

  const isAllowed = profile && ["admin", "superadmin"].includes(profile.role);

  if (!isAllowed) {
    return (
      <div className="flex h-[60vh] flex-col items-center justify-center gap-3 text-center">
        <ShieldAlert className="h-10 w-10 text-red-400" />
        <p className="text-sm font-medium text-slate-700">Akses Ditolak</p>
        <p className="max-w-sm text-sm text-slate-500">
          Fitur pencarian data pegawai (HRIS2) hanya tersedia untuk admin atau superadmin.
        </p>
      </div>
    );
  }

  return (
    <div className="w-full space-y-6">
      <div className="rounded-xl bg-white p-6 shadow-sm">
        <h1 className="text-xl font-bold text-slate-900">Pencarian Data Pegawai (HRIS2)</h1>
        <p className="mt-1 text-sm text-slate-500">
          Data diambil langsung dari API HRIS2 Kemenkeu. Klik salah satu hasil untuk melihat detail lengkap.
        </p>
      </div>
      <div className="rounded-xl bg-white p-6 shadow-sm">
        <PegawaiSearchTable />
      </div>
    </div>
  );
}