"use client";

import { ReactNode, useState, useCallback } from "react";
import type { LucideIcon } from "lucide-react";
import { Search, Loader2, ChevronRight, RefreshCcw, CheckCircle2 } from "lucide-react";
import axios from "axios";
import { SkeletonTable } from "@/components/ui/Skeleton";
import { InaprocMeta } from "@/lib/api";
import { useDashboard } from "@/lib/dashboard-context";

const KODE_KLPD_KEMENKEU = "K10";

export interface KolomTender<T> {
  judul: string;
  rata?: "kiri" | "kanan";
  render: (row: T) => ReactNode;
  // Kelas tambahan untuk sel (mis. batas lebar + truncate untuk teks panjang).
  kelas?: string;
  // Teks tooltip untuk sel yang dipotong.
  tooltip?: (row: T) => string | null | undefined;
}

interface Props<T> {
  ikon: LucideIcon;
  petunjukAwal: string; // teks sebelum pencarian pertama
  // Tampilkan kolom "Kode Tender" (opsional). Bila diisi, pencarian memakai kd_tender saja dan tahun diabaikan; hanya untuk
  // endpoint yang punya skenario itu (tender/pengumuman). `ambil` menerima kd_tender dan harus meneruskannya sendirian.
  cariKodeTender?: boolean;
  ambil: (p: { kode_klpd: string; tahun: number; kd_tender?: string; limit: number; cursor?: string }) => Promise<{ data: T[] | null; meta?: InaprocMeta }>;
  sinkron: (p: { kode_klpd: string; tahun: string }) => Promise<{ data: { total_synced: number; total_failed?: number } }>;
  kolom: KolomTender<T>[];
  kunciBaris: (row: T, idx: number) => string;
  detail: (row: T, tutup: () => void) => ReactNode;
}

// Tabel data Inaproc berhalaman dengan cursor: cari per tahun, "Muat Lebih Banyak", detail per baris, dan (admin) sinkronisasi ke
// database. Dipakai halaman Tender yang bentuknya sama; tiap halaman hanya menyediakan kolom dan modal detailnya.
export function TenderCursorTable<T>({ ikon: Ikon, petunjukAwal, cariKodeTender = false, ambil, sinkron, kolom, kunciBaris, detail }: Props<T>) {
  const { profile } = useDashboard();
  const isAdmin = profile ? ["admin", "superadmin"].includes(profile.role) : false;

  const [tahun, setTahun] = useState(new Date().getFullYear().toString());
  const [kodeTender, setKodeTender] = useState("");

  const [rows, setRows] = useState<T[]>([]);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [hasMore, setHasMore] = useState(false);

  const [isLoading, setIsLoading] = useState(false);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [hasSearched, setHasSearched] = useState(false);

  const [isSyncing, setIsSyncing] = useState(false);
  const [syncMessage, setSyncMessage] = useState<string | null>(null);
  const [syncError, setSyncError] = useState<string | null>(null);

  const [selected, setSelected] = useState<T | null>(null);

  const runSearch = useCallback(
    async (useCursor?: string) => {
      const kd = cariKodeTender ? kodeTender.trim() : "";
      if (kd && !/^\d+$/.test(kd)) {
        setError("Kode Tender harus berupa angka");
        return;
      }
      if (!kd && !tahun.trim()) {
        setError(cariKodeTender ? "Isi Tahun atau Kode Tender" : "Tahun wajib diisi");
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
        const res = await ambil({
          kode_klpd: KODE_KLPD_KEMENKEU,
          tahun: parseInt(tahun, 10),
          kd_tender: kd || undefined,
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
    [tahun, kodeTender, cariKodeTender, ambil]
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
      const res = await sinkron({ kode_klpd: KODE_KLPD_KEMENKEU, tahun });
      const gagal = res.data.total_failed ?? 0;
      setSyncMessage(
        `Berhasil menyinkronkan ${res.data.total_synced} baris data ke database.` + (gagal > 0 ? ` ${gagal} baris gagal disimpan (lihat log server).` : "")
      );
    } catch (err) {
      if (axios.isAxiosError(err) && err.response) {
        setSyncError(err.response.data?.message || "Sinkronisasi gagal");
      } else {
        setSyncError("Tidak dapat terhubung ke server");
      }
    } finally {
      setIsSyncing(false);
    }
  }, [tahun, sinkron]);

  return (
    <div className="w-full space-y-4">
      <div className="flex items-center gap-2.5 rounded-xl border border-blue-100 bg-blue-50/70 px-4 py-3 text-sm text-blue-800">
        <Ikon className="h-4 w-4 shrink-0" />
        <span>
          Menampilkan data untuk <span className="font-semibold">Kementerian Keuangan (Kode KLPD: {KODE_KLPD_KEMENKEU})</span>
        </span>
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div>
          <label className="mb-1.5 block text-xs font-medium text-slate-600">{cariKodeTender ? "Tahun" : "Tahun *"}</label>
          <input
            type="number"
            value={tahun}
            onChange={(e) => setTahun(e.target.value)}
            placeholder="2025"
            className="w-full rounded-xl border border-slate-300 bg-white px-3.5 py-2.5 text-sm shadow-sm outline-none hover:border-slate-400 focus:border-blue-500 focus:shadow-glow"
          />
        </div>
        {cariKodeTender && (
          <div>
            <label className="mb-1.5 block text-xs font-medium text-slate-600">Kode Tender (opsional)</label>
            <input
              type="text"
              inputMode="numeric"
              value={kodeTender}
              onChange={(e) => setKodeTender(e.target.value)}
              placeholder="Bila diisi, tahun diabaikan"
              className="w-full rounded-xl border border-slate-300 bg-white px-3.5 py-2.5 text-sm shadow-sm outline-none hover:border-slate-400 focus:border-blue-500 focus:shadow-glow"
            />
          </div>
        )}
        <div className={`flex items-end ${cariKodeTender ? "" : "sm:col-span-2"}`}>
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
        <div className="flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-slate-300 py-16 text-center text-slate-400">
          <Ikon className="h-8 w-8" />
          <p className="px-4 text-sm">{petunjukAwal}</p>
        </div>
      )}

      {hasSearched && !error && !isLoading && rows.length === 0 && (
        <div className="rounded-2xl border border-dashed border-slate-300 bg-slate-50/60 px-4 py-10 text-center text-sm text-slate-500">
          Tidak ada data untuk filter tersebut
        </div>
      )}

      {rows.length > 0 && (
        <>
          <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm">
            <table className="w-full text-sm">
              <thead className="bg-slate-50">
                <tr>
                  {kolom.map((k) => (
                    <th
                      key={k.judul}
                      className={`whitespace-nowrap px-4 py-2.5 font-semibold text-slate-600 ${k.rata === "kanan" ? "text-right" : "text-left"}`}
                    >
                      {k.judul}
                    </th>
                  ))}
                  <th className="whitespace-nowrap px-4 py-2.5 text-right font-semibold text-slate-600">Aksi</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {rows.map((row, idx) => (
                  <tr key={kunciBaris(row, idx)} onClick={() => setSelected(row)} className="cursor-pointer hover:bg-slate-50">
                    {kolom.map((k) => (
                      <td
                        key={k.judul}
                        title={k.tooltip?.(row) ?? undefined}
                        className={`px-4 py-2.5 ${k.rata === "kanan" ? "text-right" : ""} ${k.kelas ?? "whitespace-nowrap text-slate-600"}`}
                      >
                        {k.render(row)}
                      </td>
                    ))}
                    <td className="whitespace-nowrap px-4 py-2.5 text-right">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          setSelected(row);
                        }}
                        className="rounded-md px-2.5 py-1 text-xs font-medium text-blue-600 hover:bg-blue-50"
                      >
                        Detail
                      </button>
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

      {selected && detail(selected, () => setSelected(null))}
    </div>
  );
}
