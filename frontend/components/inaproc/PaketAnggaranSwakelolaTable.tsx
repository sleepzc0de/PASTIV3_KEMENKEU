"use client";

import { useState, useCallback } from "react";
import { Search, Loader2, WalletCards, ChevronRight, RefreshCcw, CheckCircle2 } from "lucide-react";
import { SkeletonTable } from "@/components/ui/Skeleton";
import axios from "axios";
import { getPaketAnggaranSwakelola, syncPaketAnggaranSwakelola, PaketAnggaranSwakelolaItem } from "@/lib/api";
import { useDashboard } from "@/lib/dashboard-context";

const KODE_KLPD_KEMENKEU = "K10";

function formatCurrency(n?: number): string {
  if (n === undefined || n === null) return "-";
  return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(n);
}

export function PaketAnggaranSwakelolaTable() {
  const { profile } = useDashboard();
  const isAdmin = profile ? ["admin", "superadmin"].includes(profile.role) : false;

  const [tahun, setTahun] = useState(new Date().getFullYear().toString());

  const [rows, setRows] = useState<PaketAnggaranSwakelolaItem[]>([]);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [hasMore, setHasMore] = useState(false);

  const [isLoading, setIsLoading] = useState(false);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [hasSearched, setHasSearched] = useState(false);

  const [isSyncing, setIsSyncing] = useState(false);
  const [syncMessage, setSyncMessage] = useState<string | null>(null);
  const [syncError, setSyncError] = useState<string | null>(null);

  const runSearch = useCallback(
    async (useCursor?: string) => {
      if (!tahun.trim()) {
        setError("Tahun wajib diisi");
        return;
      }
      if (useCursor) {
        setIsLoadingMore(true);
      } else {
        setIsLoading(true);
        setRows([]);
      }
      setError(null);

      try {
        const res = await getPaketAnggaranSwakelola({
          kode_klpd: KODE_KLPD_KEMENKEU,
          tahun: parseInt(tahun, 10),
          limit: 50,
          cursor: useCursor,
        });

        const newRows = res.data ?? [];
        setRows((prev) => (useCursor ? [...prev, ...newRows] : newRows));
        setCursor(res.meta?.cursor || undefined);
        setHasMore(Boolean(res.meta?.has_more));
        setHasSearched(true);
      } catch (err) {
        if (axios.isAxiosError(err) && err.response) {
          const msg = err.response.data?.error?.message || err.response.data?.message || "Pencarian gagal";
          const details = err.response.data?.error?.details;
          setError(details ? `${msg}: ${details}` : msg);
        } else {
          setError("Tidak dapat terhubung ke server");
        }
      } finally {
        setIsLoading(false);
        setIsLoadingMore(false);
      }
    },
    [tahun]
  );

  const handleSync = useCallback(async () => {
    if (!tahun.trim()) {
      setSyncError("Isi Tahun terlebih dahulu sebelum sinkronisasi");
      return;
    }
    setIsSyncing(true);
    setSyncError(null);
    setSyncMessage(null);
    try {
      const res = await syncPaketAnggaranSwakelola({ kode_klpd: KODE_KLPD_KEMENKEU, tahun });
      setSyncMessage(`Berhasil menyinkronkan ${res.data.total_synced} baris data ke database.`);
    } catch (err) {
      if (axios.isAxiosError(err) && err.response) {
        setSyncError(err.response.data?.message || "Sinkronisasi gagal");
      } else {
        setSyncError("Tidak dapat terhubung ke server");
      }
    } finally {
      setIsSyncing(false);
    }
  }, [tahun]);

  return (
    <div className="w-full space-y-4">
      <div className="flex items-center gap-2.5 rounded-xl border border-blue-100 bg-blue-50/70 px-4 py-3 text-sm text-blue-800">
        <WalletCards className="h-4 w-4 shrink-0" />
        Menampilkan data untuk <span className="font-semibold">Kementerian Keuangan (Kode KLPD: {KODE_KLPD_KEMENKEU})</span>
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div>
          <label className="mb-1.5 block text-xs font-medium text-slate-600">Tahun *</label>
          <input
            type="number"
            value={tahun}
            onChange={(e) => setTahun(e.target.value)}
            placeholder="2025"
            className="w-full rounded-xl border border-slate-300 bg-white px-3.5 py-2.5 text-sm shadow-sm outline-none hover:border-slate-400 focus:border-blue-500 focus:shadow-glow"
          />
        </div>
        <div className="flex items-end sm:col-span-2">
          <button
            onClick={() => runSearch()}
            disabled={isLoading}
            className="flex w-full items-center justify-center gap-2 rounded-lg bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm shadow-blue-600/20 hover:bg-blue-700 hover:shadow-md active:scale-[0.98] disabled:opacity-60"
          >
            {isLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4" />}
            Cari
          </button>
        </div>
      </div>

      {isAdmin && (
        <div className="flex flex-wrap items-center gap-3 rounded-xl border border-slate-200 bg-slate-50/80 px-4 py-3">
          <div className="flex-1">
            <p className="text-sm font-medium text-slate-700">Sinkronkan ke Database PASTI V3</p>
            <p className="text-xs text-slate-500">Menarik seluruh data (semua halaman) dari Inaproc dan menyimpannya secara lokal.</p>
          </div>
          <button
            onClick={handleSync}
            disabled={isSyncing}
            className="flex shrink-0 items-center gap-2 rounded-lg bg-slate-800 px-4 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-slate-900 active:scale-[0.98] disabled:opacity-60"
          >
            {isSyncing ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCcw className="h-4 w-4" />}
            {isSyncing ? "Menyinkronkan..." : "Tarik Data ke Database"}
          </button>
        </div>
      )}

      {syncMessage && (
        <div className="flex items-center gap-2 animate-fade-in rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-green-700">
          <CheckCircle2 className="h-4 w-4 shrink-0" />
          {syncMessage}
        </div>
      )}
      {syncError && <div className="animate-fade-in rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{syncError}</div>}
      {error && <div className="animate-fade-in rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>}

      {isLoading && <SkeletonTable rows={6} cols={6} />}

      {!hasSearched && !error && !isLoading && (
        <div className="flex flex-col items-center justify-center gap-2 rounded-2xl border border-dashed border-slate-300 bg-slate-50/60 py-16 text-slate-400">
          <WalletCards className="h-8 w-8" />
          <p className="text-sm">Isi Tahun untuk melihat paket anggaran swakelola Kementerian Keuangan</p>
        </div>
      )}

      {hasSearched && !error && !isLoading && rows.length === 0 && (
        <div className="rounded-2xl border border-dashed border-slate-300 bg-slate-50/60 px-4 py-10 text-center text-sm text-slate-500">
          Tidak ada data paket anggaran swakelola untuk filter tersebut
        </div>
      )}

      {rows.length > 0 && (
        <>
          <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm">
            <table className="w-full text-sm">
              <thead className="bg-slate-50">
                <tr>
                  <th className="whitespace-nowrap px-4 py-2.5 text-left font-semibold text-slate-600">Satker</th>
                  <th className="whitespace-nowrap px-4 py-2.5 text-left font-semibold text-slate-600">Kode RUP</th>
                  <th className="whitespace-nowrap px-4 py-2.5 text-left font-semibold text-slate-600">MAK</th>
                  <th className="whitespace-nowrap px-4 py-2.5 text-left font-semibold text-slate-600">Sumber Dana</th>
                  <th className="whitespace-nowrap px-4 py-2.5 text-right font-semibold text-slate-600">Pagu</th>
                  <th className="whitespace-nowrap px-4 py-2.5 text-left font-semibold text-slate-600">Status Umumkan</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {rows.map((row, idx) => (
                  <tr key={idx} className="hover:bg-blue-50/40">
                    <td className="whitespace-nowrap px-4 py-2.5 text-slate-700">{row.nama_satker || "-"}</td>
                    <td className="whitespace-nowrap px-4 py-2.5 font-mono text-xs text-slate-600">{row.kd_rup || "-"}</td>
                    <td className="max-w-[200px] truncate px-4 py-2.5 text-slate-600" title={row.mak}>
                      {row.mak || "-"}
                    </td>
                    <td className="whitespace-nowrap px-4 py-2.5 text-slate-600">{row.sumber_dana || "-"}</td>
                    <td className="whitespace-nowrap px-4 py-2.5 text-right font-medium text-slate-800">{formatCurrency(row.pagu)}</td>
                    <td className="whitespace-nowrap px-4 py-2.5">
                      <span
                        className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                          row.status_umumkan_rup === "Terumumkan" ? "bg-green-50 text-green-700" : "bg-slate-100 text-slate-600"
                        }`}
                      >
                        {row.status_umumkan_rup || "-"}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="flex items-center justify-between">
            <p className="text-xs text-slate-400">Menampilkan {rows.length} baris</p>
            {hasMore && (
              <button
                onClick={() => runSearch(cursor)}
                disabled={isLoadingMore}
                className="flex items-center gap-1.5 rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50 disabled:opacity-60"
              >
                {isLoadingMore ? <Loader2 className="h-4 w-4 animate-spin" /> : <ChevronRight className="h-4 w-4" />}
                Muat Lebih Banyak
              </button>
            )}
          </div>
        </>
      )}
    </div>
  );
}