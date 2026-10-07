"use client";

import { Hourglass, LogOut, RefreshCw } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { useDashboard } from "@/lib/dashboard-context";
import { Button } from "@/components/ui/Button";

// Layar untuk tamu yang sudah menyetujui pernyataan: belum punya peran, jadi belum ada fitur yang terbuka sampai Superadmin atau Pengguna Barang menetapkan peran.
export function MenungguPeran() {
  const { logout } = useAuth();
  const { profile, refetchProfile, isLoadingProfile } = useDashboard();
  return (
    <div className="flex min-h-dvh items-center justify-center bg-slate-50 p-4">
      <div className="w-full max-w-md rounded-2xl border border-slate-200 bg-white p-6 text-center shadow-sm sm:p-8">
        <span className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-amber-50 text-amber-600">
          <Hourglass className="h-6 w-6" aria-hidden="true" />
        </span>
        <h1 className="mt-4 text-lg font-semibold tracking-tight text-slate-900">Menunggu penetapan peran</h1>
        <p className="mt-2 text-sm text-slate-600">
          Terima kasih, pernyataan penggunaan aplikasi sudah Anda setujui. Akun Anda berstatus <b>tamu</b> sehingga belum dapat membuka fitur apa pun sampai peran ditetapkan oleh
          Superadmin atau Pengguna Barang.
        </p>
        {profile && (
          <dl className="mt-5 space-y-1 rounded-xl bg-slate-50 px-4 py-3 text-left text-sm">
            <div className="flex justify-between gap-3">
              <dt className="text-slate-500">Nama</dt>
              <dd className="break-words text-right font-medium text-slate-800">{profile.full_name}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-slate-500">Email</dt>
              <dd className="break-all text-right text-slate-800">{profile.email}</dd>
            </div>
            {profile.nip && (
              <div className="flex justify-between gap-3">
                <dt className="text-slate-500">NIP</dt>
                <dd className="text-right font-mono text-slate-800">{profile.nip}</dd>
              </div>
            )}
          </dl>
        )}
        <p className="mt-4 text-xs text-slate-500">Hubungi administrator aplikasi untuk meminta peran, lalu periksa kembali di sini.</p>
        <div className="mt-5 flex flex-col gap-2 sm:flex-row sm:justify-center">
          <Button type="button" variant="secondary" fullWidth={false} onClick={refetchProfile} isLoading={isLoadingProfile} icon={<RefreshCw className="h-4 w-4" aria-hidden="true" />}>
            Periksa lagi
          </Button>
          <Button type="button" variant="ghost" fullWidth={false} onClick={() => logout("manual")} icon={<LogOut className="h-4 w-4" aria-hidden="true" />}>
            Keluar
          </Button>
        </div>
      </div>
    </div>
  );
}
