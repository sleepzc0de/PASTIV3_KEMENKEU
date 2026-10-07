"use client";

import { useMemo } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { DatabaseZap, Lightbulb, Loader2, MapPinned, RefreshCw } from "lucide-react";
import type { DGDatasetKey } from "@/lib/api";
import { useDashboard } from "@/lib/dashboard-context";
import { formatDate } from "@/lib/format";
import { wawasanAset } from "@/lib/wawasanAset";
import { Alert } from "@/components/ui/Alert";
import { dataPer } from "../digitalisasi/digitalisasi";
import { RingkasanAset } from "../digitalisasi/DigitalisasiOverview";
import { useDGOverview } from "../digitalisasi/useDigitalisasi";
import { DaftarWawasan } from "../pengadaan/grafik";

// Dashboard Aset: angka pokok, grafik, dan wawasan analitik dari data Digitalisasi Aset yang sudah disalin dari SLDK ke database PASTI. Data dibaca
// dari salinan (bukan langsung dari SLDK); kapan terakhir disalin tampil di bagian atas.
export function DasborAset() {
  const router = useRouter();
  const { profile } = useDashboard();
  const isAdmin = profile?.role === "superadmin";
  const { data, isLoading, error, reload } = useDGOverview(true, 0);

  const wawasan = useMemo(() => (data && data.tersedia ? wawasanAset(data) : []), [data]);

  const bukaData = (p: { dataset: DGDatasetKey; tanpaKoordinat?: boolean }) =>
    router.push(`/dashboard/digitalisasi?tab=data&dataset=${p.dataset}${p.tanpaKoordinat ? "&tanpa_koordinat=1" : ""}`);

  if (!data) {
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
    return <div role="status" aria-label="Memuat dashboard aset" className="h-64 animate-pulse rounded-2xl bg-slate-100" />;
  }

  if (!data.tersedia) {
    return (
      <div className="flex flex-col items-center gap-3 rounded-2xl border border-dashed border-slate-300 bg-white px-4 py-14 text-center">
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-blue-50 text-blue-500">
          <DatabaseZap className="h-6 w-6" aria-hidden="true" />
        </div>
        <p className="text-sm font-semibold text-slate-800">Belum ada data aset untuk dianalisis</p>
        <p className="max-w-md text-sm text-slate-500">
          Dashboard Aset dihitung dari data Digitalisasi Aset yang disalin dari SLDK. Setelah sinkronisasi pertama selesai, angka, grafik, dan wawasan muncul di sini.
        </p>
        {isAdmin ? (
          <Link href="/dashboard/digitalisasi?tab=sinkronisasi" className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700">
            Buka Sinkronisasi
          </Link>
        ) : (
          <p className="text-xs text-slate-400">Hanya superadmin yang dapat menjalankan sinkronisasi.</p>
        )}
      </div>
    );
  }

  const terakhir = dataPer(data.sinkron);
  const belum = data.sinkron.filter((s) => !s.terakhir_sukses).length;

  return (
    <div className="space-y-5" aria-busy={isLoading}>
      <section className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 rounded-2xl border border-slate-200 bg-white px-4 py-3 shadow-sm">
        <p className="text-xs text-slate-500">
          Data disalin dari SLDK, terakhir <span className="font-medium text-slate-700">{formatDate(terakhir)}</span>
          {belum > 0 && (
            <span className="text-amber-700">
              {" "}
              · {belum} dari {data.sinkron.length} dataset belum pernah disinkronkan
            </span>
          )}
        </p>
        <div className="flex flex-wrap items-center gap-2 text-xs">
          <button
            type="button"
            onClick={reload}
            disabled={isLoading}
            className="inline-flex items-center gap-1.5 rounded-lg border border-slate-300 bg-white px-3 py-1.5 font-medium text-slate-700 shadow-sm hover:bg-slate-50 disabled:opacity-60"
          >
            {isLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden="true" /> : <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />}
            Muat ulang
          </button>
          <Link href="/dashboard/digitalisasi" className="inline-flex items-center gap-1.5 rounded-lg bg-blue-50 px-3 py-1.5 font-medium text-blue-700 hover:bg-blue-100">
            <MapPinned className="h-3.5 w-3.5" aria-hidden="true" />
            Digitalisasi Aset
          </Link>
        </div>
      </section>

      {error && <Alert message={`${error}. Menampilkan data yang dimuat sebelumnya.`} />}

      <div className={`space-y-5 transition-opacity ${isLoading ? "opacity-60" : ""}`}>
        <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm sm:p-5" aria-labelledby="dg-wawasan">
          <div className="mb-3 flex items-start gap-2.5">
            <Lightbulb className="mt-0.5 h-4 w-4 shrink-0 text-amber-500" aria-hidden="true" />
            <div>
              <h2 id="dg-wawasan" className="text-sm font-semibold text-slate-900">
                Wawasan analitik
              </h2>
              <p className="mt-0.5 text-xs text-slate-500">Penjelasan berbasis angka dari data yang tersinkron: yang terpenting lebih dulu. Ambangnya ada di kode, bukan penilaian manual.</p>
            </div>
          </div>
          <DaftarWawasan wawasan={wawasan} kosong="Belum ada wawasan: data aset belum cukup untuk dianalisis." />
        </section>

        <RingkasanAset d={data} onOpenData={bukaData} />
      </div>
    </div>
  );
}
