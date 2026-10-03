import Link from "next/link";
import { Compass } from "lucide-react";

export default function NotFound() {
  return (
    <div className="flex min-h-dvh flex-col items-center justify-center bg-slate-50 px-6 text-center">
      <div className="flex h-16 w-16 items-center justify-center rounded-3xl bg-gradient-to-br from-blue-500 to-blue-700 text-white shadow-lg shadow-blue-600/30">
        <Compass className="h-8 w-8" aria-hidden="true" />
      </div>
      <p className="mt-6 text-sm font-semibold text-blue-600">Kesalahan 404</p>
      <h1 className="mt-1 text-2xl font-bold tracking-tight text-slate-900">Halaman tidak ditemukan</h1>
      <p className="mt-2 max-w-sm text-sm text-slate-500">Alamat yang Anda buka tidak ada atau sudah dipindahkan. Kembali ke beranda untuk melanjutkan.</p>
      <Link
        href="/dashboard"
        className="mt-6 inline-flex items-center justify-center rounded-lg bg-blue-600 px-5 py-2.5 text-sm font-semibold text-white shadow-sm shadow-blue-600/20 transition hover:bg-blue-700 active:scale-[0.97]"
      >
        Kembali ke Beranda
      </Link>
    </div>
  );
}
