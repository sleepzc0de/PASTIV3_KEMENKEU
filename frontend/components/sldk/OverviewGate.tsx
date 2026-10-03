"use client";

import { ReactNode, useState } from "react";
import { AlertTriangle, DatabaseZap, Info, Loader2, RefreshCw, SlidersHorizontal } from "lucide-react";
import { SLDKOverviewData, SLDKSyncInfo } from "@/lib/api";
import { formatDate } from "@/lib/format";
import { Alert } from "@/components/ui/Alert";
import { OverviewSettingsModal } from "./OverviewSettingsModal";
import { formatDateTime } from "./overview";
import { OverviewState } from "./useOverview";

export type AvailableOverview = Extract<SLDKOverviewData, { tersedia: true }>;

const SYNC_COMMAND = "docker compose --env-file deploy.env run --rm --no-deps --entrypoint ./pasti-sldk-sync backend ringkasan";

interface GateProps {
  overview: OverviewState;
  isAdmin: boolean;
  children: (data: AvailableOverview) => ReactNode;
}

// Pembungkus bersama tab Ringkasan dan Pemantauan: menangani muat, galat, belum ada sinkronisasi, dan baris
// status "data per ..." beserta pengaturan definisi aset aktif untuk admin.
export function OverviewGate({ overview, isAdmin, children }: GateProps) {
  const { data, isLoading, error, reload } = overview;
  const [showSettings, setShowSettings] = useState(false);

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
    return <OverviewSkeleton />;
  }

  if (!data.tersedia) {
    return (
      <div className="space-y-3">
        {error && <Alert message={error} />}
        <NotSynced info={data.sinkron_terakhir} isAdmin={isAdmin} isLoading={isLoading} onReload={reload} />
      </div>
    );
  }

  const latest = data.sinkron_terakhir;
  const failedSince = latest && latest.status === "gagal" && latest.id !== data.sinkron_sukses.id ? latest : null;
  const running = latest && latest.status === "berjalan" ? latest : null;

  return (
    <div className="space-y-4" aria-busy={isLoading}>
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <p className="text-xs text-slate-500">
          Data SLDK per <span className="font-medium text-slate-700">{formatDate(data.sinkron_sukses.data_per)}</span>
          <span className="text-slate-400"> · disinkronkan {formatDateTime(data.sinkron_sukses.selesai)}</span>
          {data.sinkron_sukses.cakupan && <span className="text-slate-400"> · cakupan {data.sinkron_sukses.cakupan}</span>}
        </p>
        <div className="flex items-center gap-1">
          {isAdmin && (
            <button
              type="button"
              onClick={() => setShowSettings(true)}
              className="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium text-slate-600 hover:bg-slate-100"
            >
              <SlidersHorizontal className="h-3.5 w-3.5" />
              Definisi aset aktif
            </button>
          )}
          <button
            type="button"
            onClick={reload}
            disabled={isLoading}
            className="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium text-slate-600 hover:bg-slate-100 disabled:opacity-60"
          >
            {isLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
            Muat ulang
          </button>
        </div>
      </div>

      {error && <Alert message={`${error}. Menampilkan data yang dimuat sebelumnya.`} />}

      {running && (
        <Notice tone="info">Sinkronisasi sedang berjalan sejak {formatDateTime(running.mulai)}. Angka di bawah masih dari sinkronisasi sebelumnya.</Notice>
      )}
      {failedSince && (
        <Notice tone="warning">
          Sinkronisasi terakhir ({formatDateTime(failedSince.mulai)}) gagal; angka di bawah masih dari sinkronisasi sukses sebelumnya.
          {isAdmin && failedSince.pesan ? ` Pesan: ${failedSince.pesan}` : ""}
        </Notice>
      )}
      {!data.definisi.terkonfirmasi && (
        <Notice tone="warning">
          Definisi &ldquo;aset aktif&rdquo; belum ditentukan, jadi angka menghitung semua baris tabel aset SLDK, termasuk baris riwayat atau yang sudah
          dihapus.{" "}
          {isAdmin ? (
            <button type="button" onClick={() => setShowSettings(true)} className="font-medium underline underline-offset-2">
              Tentukan sekarang
            </button>
          ) : (
            "Administrator dapat menentukannya."
          )}
        </Notice>
      )}

      <div className={`space-y-4 transition-opacity ${isLoading ? "opacity-60" : ""}`}>{children(data)}</div>

      {showSettings && (
        <OverviewSettingsModal flagKeys={data.flag_keys} active={data.definisi.flag_keys_aktif} onClose={() => setShowSettings(false)} onSaved={reload} />
      )}
    </div>
  );
}

function Notice({ tone, children }: { tone: "info" | "warning"; children: ReactNode }) {
  const Icon = tone === "warning" ? AlertTriangle : Info;
  const style = tone === "warning" ? "bg-amber-50 text-amber-800" : "bg-blue-50 text-blue-800";
  return (
    <div className={`flex items-start gap-2 rounded-lg px-3.5 py-2.5 text-xs ${style}`}>
      <Icon className="mt-0.5 h-4 w-4 shrink-0" />
      <span>{children}</span>
    </div>
  );
}

function NotSynced({ info, isAdmin, isLoading, onReload }: { info: SLDKSyncInfo | null; isAdmin: boolean; isLoading: boolean; onReload: () => void }) {
  let detail = "Sinkronisasi pertama belum dijalankan.";
  if (info?.status === "berjalan") detail = `Sinkronisasi sedang berjalan sejak ${formatDateTime(info.mulai)}. Muat ulang setelah selesai.`;
  if (info?.status === "gagal") detail = `Sinkronisasi terakhir (${formatDateTime(info.mulai)}) gagal.${isAdmin && info.pesan ? ` Pesan: ${info.pesan}` : ""}`;

  return (
    <div className="flex flex-col items-center gap-3 rounded-lg border border-dashed border-slate-300 px-4 py-12 text-center">
      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-blue-50 text-blue-500">
        <DatabaseZap className="h-6 w-6" />
      </div>
      <p className="text-sm font-medium text-slate-700">Ringkasan aset belum tersedia</p>
      <p className="max-w-md text-sm text-slate-500">
        Ringkasan dan Pemantauan dihitung dari salinan agregat data SLDK yang diisi lewat sinkronisasi terjadwal, bukan langsung dari tabel aset yang
        berukuran ratusan GB. {detail}
      </p>
      {isAdmin ? (
        <div className="w-full max-w-xl space-y-1 text-left">
          <p className="text-xs text-slate-500">Jalankan di server aplikasi (petunjuk lengkap di docs/sldk-sync.md):</p>
          <code className="block overflow-x-auto whitespace-pre rounded-lg bg-slate-900 px-3.5 py-2.5 text-xs text-slate-100">{SYNC_COMMAND}</code>
        </div>
      ) : (
        <p className="text-xs text-slate-400">Hubungi administrator untuk menjalankan sinkronisasi.</p>
      )}
      <button
        type="button"
        onClick={onReload}
        disabled={isLoading}
        className="inline-flex items-center gap-1.5 rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-60"
      >
        {isLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
        Muat ulang
      </button>
    </div>
  );
}

function OverviewSkeleton() {
  return (
    <div role="status" aria-label="Memuat ringkasan" className="animate-pulse space-y-4">
      <div className="h-4 w-1/2 rounded bg-slate-100" />
      <div className="grid grid-cols-1 gap-3 min-[480px]:grid-cols-2 lg:grid-cols-4">
        {Array.from({ length: 4 }, (_, i) => (
          <div key={i} className="h-24 rounded-xl bg-slate-100" />
        ))}
      </div>
      <div className="grid gap-4 lg:grid-cols-2">
        <div className="h-56 rounded-xl bg-slate-100" />
        <div className="h-56 rounded-xl bg-slate-100" />
      </div>
    </div>
  );
}
