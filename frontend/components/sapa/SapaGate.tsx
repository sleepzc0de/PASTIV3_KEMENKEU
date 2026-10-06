"use client";

import { ReactNode, createContext, useCallback, useContext, useEffect, useState } from "react";
import { LockKeyhole, ShieldAlert } from "lucide-react";
import { SapaSaya, getSapaSaya } from "@/lib/sapa";
import { Alert } from "@/components/ui/Alert";
import { errorInfo } from "./sapa";

const SapaContext = createContext<SapaSaya | null>(null);

// Peran SAPA pengguna yang sedang masuk (diturunkan dari peran data aplikasi yang sedang aktif). Hanya dipakai di dalam SapaGate.
export function useSapa(): SapaSaya {
  const v = useContext(SapaContext);
  if (!v) throw new Error("useSapa harus dipakai di dalam SapaGate");
  return v;
}

// Memuat peran SAPA pengguna dan hanya menampilkan isi halaman bila pengguna boleh memakai SAPA. Pengguna tanpa peran
// (dan bukan admin) mendapat penjelasan, bukan halaman kosong atau galat 403.
export function SapaGate({ children, adminOnly = false }: { children: ReactNode; adminOnly?: boolean }) {
  const [saya, setSaya] = useState<SapaSaya | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setError(null);
    getSapaSaya()
      .then((res) => {
        if (!cancelled) setSaya(res.data);
      })
      .catch((err) => {
        if (!cancelled) setError(errorInfo(err, "Gagal memuat data pengguna SAPA").message);
      });
    return () => {
      cancelled = true;
    };
  }, [tick]);

  const reload = useCallback(() => setTick((t) => t + 1), []);

  if (error) {
    return (
      <div className="space-y-3">
        <Alert message={error} />
        <button type="button" onClick={reload} className="text-sm font-medium text-blue-600 hover:text-blue-700">
          Coba lagi
        </button>
      </div>
    );
  }
  if (!saya) {
    return <div role="status" aria-label="Memuat" className="h-40 animate-pulse rounded-xl bg-slate-100" />;
  }
  if (!saya.punya_akses) {
    return (
      <div className="mx-auto max-w-xl rounded-xl border border-amber-200 bg-amber-50 p-6 text-center">
        <LockKeyhole className="mx-auto h-8 w-8 text-amber-600" aria-hidden="true" />
        <h2 className="mt-3 text-base font-semibold text-amber-900">Akun Anda belum dapat memakai SAPA</h2>
        <p className="mt-1.5 text-sm text-amber-800">{saya.alasan || "Akun Anda belum diberi peran data aplikasi."}</p>
        <p className="mt-3 text-xs text-amber-700">
          SAPA tidak menetapkan peran sendiri: peran Anda (Satuan Kerja, Kantor Wilayah, Unit Eselon I, atau Pengguna Barang) mengikuti peran data aplikasi yang
          diberikan admin di Manajemen Pengguna. Hubungi admin aplikasi untuk meminta peran, lalu pilih peran itu di menu pengguna bila Anda punya lebih dari satu.
        </p>
      </div>
    );
  }
  if (adminOnly && !saya.admin) {
    return (
      <div className="mx-auto max-w-xl rounded-xl border border-slate-200 bg-slate-50 p-6 text-center">
        <ShieldAlert className="mx-auto h-8 w-8 text-slate-500" aria-hidden="true" />
        <h2 className="mt-3 text-base font-semibold text-slate-800">Halaman ini khusus admin</h2>
        <p className="mt-1.5 text-sm text-slate-600">Pengaturan SAPA hanya dapat dibuka oleh admin atau superadmin.</p>
      </div>
    );
  }
  return <SapaContext.Provider value={saya}>{children}</SapaContext.Provider>;
}
