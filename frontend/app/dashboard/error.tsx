"use client";

import { useEffect } from "react";
import { RefreshCcw, TriangleAlert } from "lucide-react";

// Batas galat untuk seluruh halaman dashboard: kesalahan tak terduga di satu halaman tidak mengosongkan layar,
// sidebar dan navbar tetap berfungsi dan pengguna bisa mencoba lagi.
export default function DashboardError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    console.error("[DASHBOARD ERROR]", error);
  }, [error]);

  return (
    <div className="card mx-auto mt-10 flex max-w-lg flex-col items-center px-6 py-14 text-center">
      <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-red-50 text-red-600">
        <TriangleAlert className="h-7 w-7" aria-hidden="true" />
      </div>
      <h1 className="mt-5 text-lg font-bold text-slate-900">Terjadi kesalahan pada halaman ini</h1>
      <p className="mt-2 text-sm text-slate-500">Halaman tidak dapat ditampilkan. Coba muat ulang; bila masih terjadi, hubungi administrator.</p>
      <button
        onClick={reset}
        className="mt-6 inline-flex items-center gap-2 rounded-lg bg-blue-600 px-5 py-2.5 text-sm font-semibold text-white shadow-sm shadow-blue-600/20 hover:bg-blue-700 active:scale-[0.97]"
      >
        <RefreshCcw className="h-4 w-4" aria-hidden="true" />
        Coba lagi
      </button>
    </div>
  );
}
