"use client";

import { AlertCircle, RefreshCcw } from "lucide-react";
import { useDashboard } from "@/lib/dashboard-context";
import { WelcomeBanner } from "@/components/dashboard/WelcomeBanner";
import { ProfileCard } from "@/components/dashboard/ProfileCard";
import { QuickAccess } from "@/components/dashboard/QuickAccess";
import { SyncActivityCard } from "@/components/dashboard/SyncActivityCard";

export default function DashboardPage() {
  const { profile, isLoadingProfile, refetchProfile } = useDashboard();

  if (isLoadingProfile) {
    return (
      <div role="status" aria-label="Memuat dashboard" className="w-full animate-pulse space-y-6">
        <div className="h-48 rounded-2xl bg-slate-200" />
        <div className="grid grid-cols-1 gap-6 xl:grid-cols-3">
          <div className="h-80 rounded-xl bg-slate-200" />
          <div className="h-80 rounded-xl bg-slate-200 xl:col-span-2" />
        </div>
      </div>
    );
  }

  if (!profile) {
    return (
      <div className="flex w-full flex-col items-center gap-3 rounded-xl bg-white px-6 py-16 text-center shadow-sm">
        <AlertCircle className="h-8 w-8 text-red-500" />
        <div>
          <h1 className="text-base font-semibold text-slate-900">Data profil tidak dapat dimuat</h1>
          <p className="mt-1 text-sm text-slate-500">Periksa koneksi Anda, lalu coba lagi.</p>
        </div>
        <button
          onClick={refetchProfile}
          className="flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700"
        >
          <RefreshCcw className="h-4 w-4" />
          Coba Lagi
        </button>
      </div>
    );
  }

  const isAdmin = ["admin", "superadmin"].includes(profile.role);

  return (
    <div className="w-full space-y-6">
      <WelcomeBanner profile={profile} />

      <div className="grid grid-cols-1 items-start gap-6 xl:grid-cols-3">
        <div className="order-last space-y-6 xl:order-none xl:col-span-1">
          <ProfileCard profile={profile} />
          {isAdmin && <SyncActivityCard />}
        </div>

        <div className="xl:col-span-2">
          <QuickAccess role={profile.role} />
        </div>
      </div>
    </div>
  );
}
