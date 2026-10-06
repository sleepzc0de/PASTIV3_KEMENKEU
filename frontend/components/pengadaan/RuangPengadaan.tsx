"use client";

import { Lock } from "lucide-react";
import { useDashboard } from "@/lib/dashboard-context";
import { bolehLihatPengadaan } from "@/lib/peran";
import { EmptyState } from "@/components/ui/EmptyState";
import { DataTerbatas } from "./DataTerbatas";
import { PenarikanWorkspace } from "./PenarikanWorkspace";

// Isi halaman Pengadaan Terpadu menurut peran: yang melihat seluruh data memakai ruang kerja penuh (penarikan, riwayat, jadwal, data, ekspor); UE1, Kanwil, dan Satker
// hanya melihat dan mengekspor data pengadaan satkernya (backend membatasinya lewat kd_satker_str); pengguna yang belum diberi peran mendapat penjelasan.
export function RuangPengadaan() {
  const { profile, semuaData, isLoadingProfile } = useDashboard();
  if (isLoadingProfile) return <div role="status" aria-label="Memuat" className="h-40 animate-pulse rounded-2xl bg-slate-100" />;
  if (!bolehLihatPengadaan(profile?.peran)) {
    return (
      <EmptyState
        icon={Lock}
        title="Akun Anda belum diberi peran data"
        description="Data pengadaan ditampilkan menurut peran Anda. Hubungi administrator untuk diberi peran (Pengguna Barang, UE1, Kanwil, atau Satker)."
      />
    );
  }
  return semuaData ? <PenarikanWorkspace /> : <DataTerbatas />;
}
