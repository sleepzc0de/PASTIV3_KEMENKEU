"use client";

import { AlertCircle, RefreshCcw } from "lucide-react";
import { useDashboard } from "@/lib/dashboard-context";
import { WelcomeBanner } from "@/components/dashboard/WelcomeBanner";
import { ProfileCard } from "@/components/dashboard/ProfileCard";
import { QuickAccess } from "@/components/dashboard/QuickAccess";
import { RecentPages } from "@/components/dashboard/RecentPages";
import { SyncActivityCard } from "@/components/dashboard/SyncActivityCard";
import { Button } from "@/components/ui/Button";
import { Skeleton } from "@/components/ui/Skeleton";

export default function DashboardPage() {
  const { profile, isLoadingProfile, refetchProfile } = useDashboard();

  if (isLoadingProfile) {
    return (
      <div role="status" aria-label="Memuat dashboard" className="w-full space-y-6">
        <Skeleton className="h-52 rounded-3xl" />
        <div className="grid grid-cols-1 gap-6 xl:grid-cols-3">
          <div className="space-y-4 xl:col-span-2">
            <Skeleton className="h-5 w-40" />
            <div className="grid gap-3 sm:grid-cols-2">
              {Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className="h-[84px] rounded-2xl" />
              ))}
            </div>
          </div>
          <Skeleton className="h-80 rounded-2xl" />
        </div>
      </div>
    );
  }

  if (!profile) {
    return (
      <div className="card flex w-full flex-col items-center gap-3 px-6 py-16 text-center">
        <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-red-50 text-red-500">
          <AlertCircle className="h-7 w-7" />
        </div>
        <div>
          <h1 className="text-base font-semibold text-slate-900">Data profil tidak dapat dimuat</h1>
          <p className="mt-1 text-sm text-slate-500">Periksa koneksi Anda, lalu coba lagi.</p>
        </div>
        <Button onClick={refetchProfile} fullWidth={false} icon={<RefreshCcw className="h-4 w-4" />}>
          Coba Lagi
        </Button>
      </div>
    );
  }

  const isAdmin = ["admin", "superadmin"].includes(profile.role);

  return (
    <div className="w-full space-y-6">
      <WelcomeBanner profile={profile} />

      <RecentPages role={profile.role} />

      <div className="grid grid-cols-1 items-start gap-6 xl:grid-cols-3">
        <div className="xl:col-span-2">
          <QuickAccess role={profile.role} />
        </div>

        <div className="order-last space-y-6 xl:order-none xl:col-span-1">
          <ProfileCard profile={profile} />
          {isAdmin && <SyncActivityCard />}
        </div>
      </div>
    </div>
  );
}
