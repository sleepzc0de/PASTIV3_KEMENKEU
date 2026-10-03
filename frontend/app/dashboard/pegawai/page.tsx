"use client";

import { useDashboard } from "@/lib/dashboard-context";
import { PegawaiSearchTable } from "@/components/hris2/PegawaiSearchTable";
import { PageShell } from "@/components/ui/PageHeader";
import { EmptyState } from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import { ShieldAlert, KeyRound, Users2 } from "lucide-react";

export default function PegawaiPage() {
  const { profile, isLoadingProfile } = useDashboard();

  if (isLoadingProfile) {
    return (
      <div role="status" aria-label="Memuat" className="w-full space-y-5">
        <Skeleton className="h-12 w-80 max-w-full" />
        <Skeleton className="h-64 rounded-2xl" />
      </div>
    );
  }

  const isAllowed = profile && ["admin", "superadmin"].includes(profile.role);

  if (!isAllowed) {
    return (
      <div className="mx-auto mt-10 max-w-lg">
        <EmptyState icon={ShieldAlert} title="Akses ditolak" description="Fitur pencarian data pegawai (HRIS2) hanya tersedia untuk admin atau superadmin." />
      </div>
    );
  }

  // Pencarian memakai access token SSO milik pengguna yang sedang login, jadi akun lokal pasti
  // ditolak backend (403). Dijelaskan di awal daripada dibiarkan mencari lalu gagal. Bila jenis
  // akun tidak diketahui, biarkan backend yang memutuskan.
  const isLocalAccount = Boolean(profile.auth_provider) && profile.auth_provider !== "sso";

  if (isLocalAccount) {
    return (
      <div className="mx-auto mt-10 max-w-lg">
        <EmptyState
          icon={KeyRound}
          title="Perlu login lewat SSO Kemenkeu"
          description="Data pegawai diambil dari HRIS2 memakai sesi SSO Kemenkeu Anda. Akun lokal tidak dapat memakai fitur ini; logout lalu login kembali melalui SSO Kemenkeu."
        />
      </div>
    );
  }

  return (
    <PageShell
      title="Pencarian Data Pegawai (HRIS2)"
      icon={Users2}
      description="Data diambil langsung dari API HRIS2 Kemenkeu memakai sesi SSO Anda. Pilih salah satu hasil untuk melihat profil lengkapnya."
    >
      <PegawaiSearchTable />
    </PageShell>
  );
}
