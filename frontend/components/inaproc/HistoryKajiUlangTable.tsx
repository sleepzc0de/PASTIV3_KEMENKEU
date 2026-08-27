"use client";

import { useState, useCallback } from "react";
import { Search, Loader2, FileClock, ChevronRight, RefreshCcw, CheckCircle2 } from "lucide-react";
import axios from "axios";
import { getHistoryKajiUlang, syncHistoryKajiUlang, KajiUlangItem } from "@/lib/api";
import { useDashboard } from "@/lib/dashboard-context";

const KODE_KLPD_KEMENKEU = "K10";

function formatDate(iso?: string): string {
  if (!iso) return "-";
  try {
    return new Date(iso).toLocaleDateString("id-ID", {
      day: "numeric",
      month: "short",
      year: "numeric",
    });
  } catch {
    return iso;
  }
}

export function HistoryKajiUlangTable() {
  const { profile } = useDashboard();
  const isAdmin = profile ? ["admin", "superadmin"].includes(profile.role) : false;

  const [tahun, setTahun] = useState(new Date().getFullYear().toString());
  const [jenisPaket, setJenisPaket] = useState("");

  const [rows, setRows] = useState<KajiUlangItem[]>([]);
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
        const res = await getHistoryKajiUlang({
          kode_klpd: KODE_KLPD_KEMENKEU,
          tahun: parseInt(tahun, 10),
          jenis_paket: jenisPaket || undefined,
          limit: 50,
          cursor: useCursor,
        });

        // "data" bisa null dari API kalau hasil kosong — selalu fallback ke [].
        const newRows = res.data ?? [];
        setRows((prev) => (useCursor ? [...prev, ...newRows] : newRows));
        setCursor(res.meta?.cursor || undefined);
        setHasMore(Boolean(res.meta?.has_more));
        setHasSearched(true);
      } catch (err) {
        if (axios.isAxiosError(err) && err.response) {
          const msg =
            err.response.data?.error?.message ||
            err.response.data?.message ||
            "Pencarian gagal";
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
    [tahun, jenisPaket]
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
      const res = await syncHistoryKajiUlang({
        kode_klpd: KODE_KLPD_KEMENKEU,
        tahun,
        jenis_paket: jenisPaket || undefined,
      });
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
  }, [tahun, jenisPaket]);

  return (
    <div className="w-full space-y-4">
      <div className="flex items-center gap-2 rounded-lg bg-blue-50 px-4 py-2.5 text-sm text-blue-700">
        <FileClock className="h-4 w-4 shrink-0" />
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
            className="w-full rounded-lg border border-slate-300 px-3.5 py-2.5 text-sm outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10"
          />
        </div>
        <div>
          <label className="mb-1.5 block text-xs font-medium text-slate-600">Jenis Paket</label>
          <select
            value={jenisPaket}
            onChange={(e) => setJenisPaket(e.target.value)}
            className="w-full rounded-lg border border-slate-300 px-3.5 py-2.5 text-sm outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10"
          >
            <option value="">Semua</option>
            <option value="PENYEDIA">Penyedia</option>
            <option value="SWAKELOLA">Swakelola</option>
          </select>
        </div>
        <div className="flex items-end">
          <button
            onClick={() => runSearch()}
            disabled={isLoading}
            className="flex w-full items-center justify-center gap-2 rounded-lg bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60"
          >
            {isLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4" />}
            Cari
          </button>
        </div>
      </div>

      {isAdmin && (
        <div className="flex flex-wrap items-center gap-3 rounded-lg border border-slate-200 bg-slate-50 px-4 py-3">
          <div className="flex-1">
            <p className="text-sm font-medium text-slate-700">Sinkronkan ke Database PASTI V3</p>
            <p className="text-xs text-slate-500">
              Menarik seluruh data (semua halaman) dari Inaproc untuk filter di atas dan menyimpannya secara lokal.
            </p>
          </div>
          <button
            onClick={handleSync}
            disabled={isSyncing}
            className="flex shrink-0 items-center gap-2 rounded-lg bg-slate-800 px-4 py-2.5 text-sm font-semibold text-white hover:bg-slate-900 disabled:opacity-60"
          >
            {isSyncing ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCcw className="h-4 w-4" />}
            {isSyncing ? "Menyinkronkan..." : "Tarik Data ke Database"}
          </button>
        </div>
      )}

      {syncMessage && (
        <div className="flex items-center gap-2 rounded-lg border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-700">
          <CheckCircle2 className="h-4 w-4 shrink-0" />
          {syncMessage}
        </div>
      )}

      {syncError && (
        <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{syncError}</div>
      )}

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>
      )}

      {!hasSearched && !error && (
        <div className="flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-slate-300 py-16 text-slate-400">
          <FileClock className="h-8 w-8" />
          <p className="text-sm">Isi Tahun untuk melihat riwayat kaji ulang RUP Kementerian Keuangan</p>
        </div>
      )}

      {hasSearched && !error && rows.length === 0 && (
        <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-8 text-center text-sm text-slate-500">
          Tidak ada data kaji ulang untuk filter tersebut
        </div>
      )}

      {rows.length > 0 && (
        <>
          <div className="overflow-x-auto rounded-lg border border-slate-200">
            <table className="w-full text-sm">
              <thead className="bg-slate-50">
                <tr>
                  <th className="whitespace-nowrap px-4 py-2.5 text-left font-semibold text-slate-600">Satker</th>
                  <th className="whitespace-nowrap px-4 py-2.5 text-left font-semibold text-slate-600">Kode RUP Lama</th>
                  <th className="whitespace-nowrap px-4 py-2.5 text-left font-semibold text-slate-600">Kode RUP Baru</th>
                  <th className="whitespace-nowrap px-4 py-2.5 text-left font-semibold text-slate-600">Jenis Paket</th>
                  <th className="whitespace-nowrap px-4 py-2.5 text-left font-semibold text-slate-600">Jenis Revisi</th>
                  <th className="whitespace-nowrap px-4 py-2.5 text-left font-semibold text-slate-600">Alasan</th>
                  <th className="whitespace-nowrap px-4 py-2.5 text-left font-semibold text-slate-600">Tgl Kaji Ulang</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {rows.map((row) => (
                  <tr key={row.datamart_id} className="hover:bg-slate-50">
                    <td className="whitespace-nowrap px-4 py-2.5 text-slate-700">{row.nama_satker || "-"}</td>
                    <td className="whitespace-nowrap px-4 py-2.5 font-mono text-xs text-slate-600">{row.kd_rup_lama || "-"}</td>
                    <td className="whitespace-nowrap px-4 py-2.5 font-mono text-xs text-slate-600">{row.kd_rup_baru || "-"}</td>
                    <td className="whitespace-nowrap px-4 py-2.5">
                      <span className="rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">
                        {row.jenis_paket || "-"}
                      </span>
                    </td>
                    <td className="whitespace-nowrap px-4 py-2.5 text-slate-700">{row.jenis_revisi || "-"}</td>
                    <td className="max-w-xs truncate px-4 py-2.5 text-slate-600" title={row.alasan_kajiulang}>
                      {row.alasan_kajiulang || "-"}
                    </td>
                    <td className="whitespace-nowrap px-4 py-2.5 text-slate-500">{formatDate(row.tgl_kaji_ulang)}</td>
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
                {isLoadingMore ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <ChevronRight className="h-4 w-4" />
                )}
                Muat Lebih Banyak
              </button>
            )}
          </div>
        </>
      )}
    </div>
  );
}