"use client";

import Link from "next/link";
import { Settings2 } from "lucide-react";
import { useDashboard } from "@/lib/dashboard-context";

// Pengaturan SAPA tidak punya baris sendiri di sidebar (menu Aset hanya memuat "SAPA"), jadi admin mencapainya dari tombol ini di halaman SAPA.
// Hanya tampil bagi admin dan superadmin, sama dengan hak akses halaman pengaturannya.
export function TautanPengaturan() {
  const { profile } = useDashboard();
  if (!profile || !["admin", "superadmin"].includes(profile.role)) return null;
  return (
    <Link
      href="/dashboard/sapa/pengaturan"
      className="inline-flex items-center gap-2 rounded-lg border border-slate-300 bg-white px-3.5 py-2 text-sm font-medium text-slate-700 shadow-sm hover:border-slate-400 hover:bg-slate-50"
    >
      <Settings2 className="h-4 w-4" aria-hidden="true" />
      Pengaturan SAPA
    </Link>
  );
}
