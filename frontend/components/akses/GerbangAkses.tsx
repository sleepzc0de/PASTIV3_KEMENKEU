"use client";

import type { ReactNode } from "react";
import { useDashboard } from "@/lib/dashboard-context";
import { ModalPersetujuan } from "./ModalPersetujuan";
import { MenungguPeran } from "./MenungguPeran";

// Gerbang akses aplikasi menurut keadaan akun (backend menolak permintaan tamu dan yang belum menyetujui dengan 403; ini hanya menampilkan jalan keluarnya):
//   belum menyetujui pernyataan penggunaan aplikasi -> hanya modal persetujuan (tidak dapat ditutup);
//   sudah menyetujui tetapi belum punya peran (tamu)  -> layar "menunggu penetapan peran";
//   selain itu -> aplikasi seperti biasa. Superadmin tidak wajib menyetujui dan bukan tamu.
export function GerbangAkses({ children }: { children: ReactNode }) {
  const { profile, isLoadingProfile } = useDashboard();
  if (isLoadingProfile && !profile) return <div role="status" aria-label="Memuat" className="min-h-dvh animate-pulse bg-slate-50" />;
  if (!profile) return <>{children}</>; // profil gagal dimuat: penanganan sesi berakhir ada di interseptor API
  if (profile.persetujuan && !profile.persetujuan.sudah) {
    return (
      <div className="min-h-dvh bg-slate-50">
        <ModalPersetujuan />
      </div>
    );
  }
  if (profile.tamu) return <MenungguPeran />;
  return <>{children}</>;
}
