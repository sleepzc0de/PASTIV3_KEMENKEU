"use client";

import { useDashboard } from "@/lib/dashboard-context";
import { PegawaiSearchTable } from "@/components/hris2/PegawaiSearchTable";
import { ShieldAlert, KeyRound, Loader2 } from "lucide-react";

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
      <div className="flex h-[60vh] flex-col items-center justify-center gap-3 px-4 text-center">
        <ShieldAlert className="h-10 w-10 text-red-400" />
        <p className="text-sm font-medium text-slate-700">Akses Ditolak</p>
        <p className="max-w-sm text-sm text-slate-500">
          Fitur pencarian data pegawai (HRIS2) hanya tersedia untuk admin atau superadmin.
        </p>
      </div>
    );
  }

  // Pencarian memakai access token SSO milik pengguna yang sedang login, jadi akun lokal pasti
  // ditolak backend (403). Dijelaskan di awal daripada dibiarkan mencari lalu gagal. Bila jenis
  // akun tidak diketahui, biarkan backend yang memutuskan.
  const isLocalAccount = Boolean(profile.auth_provider) && profile.auth_provider !== "sso";

  if (isLocalAccount) {
    return (
      <div className="flex h-[60vh] flex-col items-center justify-center gap-3 px-4 text-center">
        <KeyRound className="h-10 w-10 text-amber-400" />
        <p className="text-sm font-medium text-slate-700">Perlu login lewat SSO Kemenkeu</p>
        <p className="max-w-sm text-sm text-slate-500">
          Data pegawai diambil dari HRIS2 memakai sesi SSO Kemenkeu Anda. Akun lokal tidak dapat memakai fitur ini; logout
          lalu login kembali melalui SSO Kemenkeu.
        </p>
      </div>
    );
  }

  return (
    <div className="w-full space-y-6">
      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <h1 className="text-xl font-bold text-slate-900">Pencarian Data Pegawai (HRIS2)</h1>
        <p className="mt-1 text-sm text-slate-500">
          Data diambil langsung dari API HRIS2 Kemenkeu memakai sesi SSO Anda. Pilih salah satu hasil untuk melihat profil
          lengkapnya.
        </p>
      </div>
      <div className="rounded-xl bg-white p-4 shadow-sm sm:p-6">
        <PegawaiSearchTable />
      </div>
    </div>
  );
}
