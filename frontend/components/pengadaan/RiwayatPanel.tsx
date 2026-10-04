"use client";

import { useEffect, useState } from "react";
import { Loader2, RefreshCw } from "lucide-react";
import { PenarikanRiwayat, PenarikanStatus, getPenarikanRiwayat } from "@/lib/api";
import { durasi, formatAngka, formatDurasi, formatWaktu, labelParameter } from "@/lib/pengadaan";
import { Alert } from "@/components/ui/Alert";
import { EmptyState } from "@/components/ui/EmptyState";
import { errorMessage } from "../digitalisasi/useDigitalisasi";
import { KELAS_INPUT, LencanaStatus } from "./lencana";

// Riwayat penarikan (manual dan otomatis), terbaru dulu, dengan penyaring dataset.
export function RiwayatPanel({ status, isAdmin, versi }: { status: PenarikanStatus; isAdmin: boolean; versi: number }) {
  const [dataset, setDataset] = useState("");
  const [limit, setLimit] = useState(50);
  const [data, setData] = useState<PenarikanRiwayat[] | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let batal = false;
    setIsLoading(true);
    setError(null);
    getPenarikanRiwayat({ dataset: dataset || undefined, limit })
      .then((res) => {
        if (!batal) setData(res.data);
      })
      .catch((err) => {
        if (!batal) setError(errorMessage(err, "Gagal memuat riwayat penarikan"));
      })
      .finally(() => {
        if (!batal) setIsLoading(false);
      });
    return () => {
      batal = true;
    };
  }, [dataset, limit, versi, tick]);

  const nama = (id: string) => status.datasets.find((d) => d.id === id)?.nama ?? id;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="min-w-[14rem]">
          <label htmlFor="riwayat-dataset" className="mb-1 block text-xs font-medium text-slate-600">
            Dataset
          </label>
          <select id="riwayat-dataset" value={dataset} onChange={(e) => setDataset(e.target.value)} className={KELAS_INPUT}>
            <option value="">Semua dataset</option>
            {status.datasets.map((d) => (
              <option key={d.id} value={d.id}>
                {d.nama}
              </option>
            ))}
          </select>
        </div>
        <button type="button" onClick={() => setTick((t) => t + 1)} disabled={isLoading} className="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium text-slate-600 hover:bg-slate-100 disabled:opacity-60">
          {isLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
          Muat ulang
        </button>
      </div>

      {error && <Alert message={error} />}
      {!data && !error && <div role="status" aria-label="Memuat riwayat" className="h-40 animate-pulse rounded-xl bg-slate-100" />}
      {data && data.length === 0 && <EmptyState title="Belum ada riwayat penarikan" description="Riwayat muncul setelah penarikan manual atau otomatis dijalankan." />}
      {data && data.length > 0 && (
        <>
          <ul className={`divide-y divide-slate-100 overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm ${isLoading ? "opacity-60" : ""}`}>
            {data.map((r) => {
              const dur = durasi(r.mulai, r.selesai);
              return (
                <li key={r.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-4 py-2.5 text-xs">
                  <LencanaStatus status={r.status} />
                  <span className="font-medium text-slate-800">{nama(r.dataset)}</span>
                  {r.parameter && <span className="text-slate-500">{labelParameter(r.parameter)}</span>}
                  <span className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${r.pemicu === "otomatis" ? "bg-violet-50 text-violet-700" : "bg-slate-100 text-slate-600"}`}>
                    {r.pemicu === "otomatis" ? "Otomatis" : "Manual"}
                  </span>
                  <span className="text-slate-500">{formatWaktu(r.mulai ?? r.dibuat)}</span>
                  {dur !== null && <span className="text-slate-400">{formatDurasi(dur)}</span>}
                  {r.jumlah_baris !== null && <span className="text-slate-500">{formatAngka(r.jumlah_baris)} baris</span>}
                  {r.percobaan > 1 && <span className="text-amber-700">{r.percobaan} percobaan</span>}
                  {isAdmin && r.dijalankan_oleh && r.pemicu === "manual" && <span className="text-slate-400">oleh {r.dijalankan_oleh}</span>}
                  {r.pesan && r.status !== "berjalan" && r.status !== "antri" && <span className="w-full break-words text-slate-500">{r.pesan}</span>}
                </li>
              );
            })}
          </ul>
          {data.length >= limit && limit < 500 && (
            <div className="text-center">
              <button type="button" onClick={() => setLimit((l) => Math.min(500, l * 2))} className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50">
                Tampilkan lebih banyak
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
