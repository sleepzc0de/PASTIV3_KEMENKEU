"use client";

import type { ReactNode } from "react";
import { Lock } from "lucide-react";
import { useDashboard } from "@/lib/dashboard-context";
import { EmptyState } from "@/components/ui/EmptyState";

// Membungkus halaman yang datanya belum bisa dibatasi per satker (mis. Pengadaan Terpadu). Peran UE1/Kanwil/Satker melihat penjelasan, bukan halaman yang
// pasti ditolak API-nya; yang sebenarnya menjaga data tetap backend (RequireCakupanSemua).
export function GerbangSemuaData({ children }: { children: ReactNode }) {
  const { semuaData, isLoadingProfile } = useDashboard();
  if (isLoadingProfile) return <div role="status" aria-label="Memuat" className="h-40 animate-pulse rounded-2xl bg-slate-100" />;
  if (!semuaData) {
    return (
      <EmptyState
        icon={Lock}
        title="Halaman ini tidak tersedia untuk peran Anda"
        description="Data Pengadaan lengkap hanya untuk peran yang melihat seluruh data. Ringkasan pengadaan satker Anda ada di tab Satker pada Dashboard."
      />
    );
  }
  return <>{children}</>;
}