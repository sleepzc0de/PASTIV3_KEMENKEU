"use client";

import { useDashboard } from "@/lib/dashboard-context";
import { Loader2, Building2, IdCard, BadgeCheck } from "lucide-react";

export default function DashboardPage() {
  const { profile, isLoadingProfile } = useDashboard();

  if (isLoadingProfile) {
    return (
      <div className="flex h-[60vh] items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-blue-600" />
      </div>
    );
  }

  return (
    <div className="w-full space-y-6">
      <div className="rounded-xl bg-white p-6 shadow-sm">
        <h1 className="text-xl font-bold text-slate-900">Dashboard PASTI V3</h1>
        <p className="mt-1 text-sm text-slate-500">
          Selamat datang kembali,{" "}
          <span className="font-semibold text-slate-900">{profile?.full_name}</span>
        </p>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <InfoCard
          icon={BadgeCheck}
          label="Role"
          value={profile?.role || "-"}
          color="blue"
        />
        {profile?.jabatan && (
          <InfoCard icon={IdCard} label="Jabatan" value={profile.jabatan} color="purple" />
        )}
        {profile?.satker && (
          <InfoCard icon={Building2} label="Satuan Kerja" value={profile.satker} color="green" />
        )}
      </div>
    </div>
  );
}

function InfoCard({
  icon: Icon,
  label,
  value,
  color,
}: {
  icon: React.ElementType;
  label: string;
  value: string;
  color: "blue" | "purple" | "green";
}) {
  const colorMap = {
    blue: "bg-blue-50 text-blue-600",
    purple: "bg-purple-50 text-purple-600",
    green: "bg-green-50 text-green-600",
  };

  return (
    <div className="flex items-center gap-4 rounded-xl bg-white p-5 shadow-sm">
      <div className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-lg ${colorMap[color]}`}>
        <Icon className="h-5 w-5" />
      </div>
      <div className="min-w-0">
        <p className="text-xs text-slate-500">{label}</p>
        <p className="truncate text-sm font-semibold text-slate-900">{value}</p>
      </div>
    </div>
  );
}