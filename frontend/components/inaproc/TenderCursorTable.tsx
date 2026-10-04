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

// Satu isian kode di form pencarian, di samping (atau pengganti) Tahun.
//  - opsional (bawaan): bila diisi, kode menggantikan Tahun sebagai penyaring (mis. kd_tender di Pengumuman Tender).
//  - wajib: pencarian rujukan per satu kode (komoditas, penyedia, distributor E-Katalog); kode itu satu-satunya penyaring dan juga
//    dikirim saat sinkronisasi.
export interface IsianKode {
  label: string; // teks label di atas isian
  nama: string; // nama dalam pesan galat, mis. "Kode Penyedia"
  placeholder?: string;
  wajib?: boolean;
  numerik?: boolean; // hanya angka
}

interface Props<T> {
  ikon: LucideIcon;
  petunjukAwal: string; // teks sebelum pencarian pertama
  kodeCari?: IsianKode;
  tanpaTahun?: boolean; // sembunyikan isian Tahun (endpoint tanpa tahun)
  // Pengganti teks "Menampilkan data untuk Kementerian Keuangan (Kode KLPD: K10)", mis. untuk pencarian per kode yang tidak
  // berkaitan dengan KLPD.
  keterangan?: string;
  ambil: (p: { kode_klpd: string; tahun: number; kode?: string; limit: number; cursor?: string }) => Promise<{ data: T[] | null; meta?: InaprocMeta }>;
  sinkron: (p: { kode_klpd: string; tahun: string; kode?: string }) => Promise<{ data: { total_synced: number; total_failed?: number } }>;
  kolom: KolomTender<T>[];
  kunciBaris: (row: T, idx: number) => string;
  detail: (row: T, tutup: () => void) => ReactNode;
}

const KELAS_ISIAN =
  "w-full rounded-xl border border-slate-300 bg-white px-3.5 py-2.5 text-sm shadow-sm outline-none hover:border-slate-400 focus:border-blue-500 focus:shadow-glow";

// Tabel data Inaproc berhalaman dengan cursor: cari per tahun (atau per kode), "Muat Lebih Banyak", detail per baris, dan (admin)
// sinkronisasi ke database. Dipakai halaman Tender dan E-Katalog yang bentuknya sama; tiap halaman hanya menyediakan kolom dan
// modal detailnya.
export function TenderCursorTable<T>({
  ikon: Ikon,
  petunjukAwal,
  kodeCari,
  tanpaTahun = false,
  keterangan,
  ambil,
  sinkron,
  kolom,
  kunciBaris,
  detail,
}: Props<T>) {
  const { profile } = useDashboard();
  const isAdmin = profile ? ["admin", "superadmin"].includes(profile.role) : false;

  const [tahun, setTahun] = useState(new Date().getFullYear().toString());
  const [kode, setKode] = useState("");

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
      const kd = kodeCari ? kode.trim() : "";
      if (kodeCari?.wajib && !kd) {
        setError(`${kodeCari.nama} wajib diisi`);
        return;
      }
      if (kodeCari && kd && kodeCari.numerik && !/^\d+$/.test(kd)) {
        setError(`${kodeCari.nama} harus berupa angka`);
        return;
      }
      // Tahun dibutuhkan kecuali endpoint-nya tanpa tahun, atau kode opsional yang terisi menggantikannya.
      if (!tanpaTahun && !(kodeCari && !kodeCari.wajib && kd) && !tahun.trim()) {
        setError(kodeCari && !kodeCari.wajib ? `Isi Tahun atau ${kodeCari.nama}` : "Tahun wajib diisi");
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
          kode: kd || undefined,
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
    [tahun, kode, kodeCari, tanpaTahun, ambil]
  );

  const handleSync = useCallback(async () => {
    // Kode ikut disinkronkan hanya bila wajib (pencarian rujukan per kode); kode opsional hanya untuk pencarian.
    const kd = kodeCari?.wajib ? kode.trim() : "";
    if (kodeCari?.wajib && !kd) {
      setSyncError(`Isi ${kodeCari.nama} terlebih dahulu sebelum sinkronisasi`);
      return;
    }
    if (kodeCari?.wajib && kodeCari.numerik && !/^\d+$/.test(kd)) {
      setSyncError(`${kodeCari.nama} harus berupa angka`);
      return;
    }
    if (!tanpaTahun && !tahun.trim()) {
      setSyncError("Isi Tahun terlebih dahulu sebelum sinkronisasi");
      return;
    }
    setIsSyncing(true);
    setSyncError(null);
    setSyncMessage(null);
    try {
      const res = await sinkron({ kode_klpd: KODE_KLPD_KEMENKEU, tahun, ...(kd ? { kode: kd } : {}) });
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
  }, [tahun, kode, kodeCari, tanpaTahun, sinkron]);

  // Tombol Cari mengisi sisa baris dari grid 3 kolom.
  const jumlahIsian = (tanpaTahun ? 0 : 1) + (kodeCari ? 1 : 0);
  const lebarCari = jumlahIsian === 0 ? "sm:col-span-3" : jumlahIsian === 1 ? "sm:col-span-2" : "";
  const cariBilaEnter = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !isLoading) runSearch();
  };

  return (
    <div className="w-full space-y-4">
      <div className="flex items-center gap-2.5 rounded-xl border border-blue-100 bg-blue-50/70 px-4 py-3 text-sm text-blue-800">
        <Ikon className="h-4 w-4 shrink-0" />
        {keterangan ? (
          <span>{keterangan}</span>
        ) : (
          <span>
            Menampilkan data untuk <span className="font-semibold">Kementerian Keuangan (Kode KLPD: {KODE_KLPD_KEMENKEU})</span>
          </span>
        )}
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        {!tanpaTahun && (
          <div>
            <label className="mb-1.5 block text-xs font-medium text-slate-600">{kodeCari && !kodeCari.wajib ? "Tahun" : "Tahun *"}</label>
            <input
              type="number"
              value={tahun}
              onChange={(e) => setTahun(e.target.value)}
              onKeyDown={cariBilaEnter}
              placeholder="2025"
              className={KELAS_ISIAN}
            />
          </div>
        )}
        {kodeCari && (
          <div>
            <label className="mb-1.5 block text-xs font-medium text-slate-600">{kodeCari.label}</label>
            <input
              type="text"
              inputMode={kodeCari.numerik ? "numeric" : "text"}
              maxLength={100}
              value={kode}
              onChange={(e) => setKode(e.target.value)}
              onKeyDown={cariBilaEnter}
              placeholder={kodeCari.placeholder}
              className={KELAS_ISIAN}
            />
          </div>
        )}
        <div className={`flex items-end ${lebarCari}`}>
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
            <p className="text-xs text-slate-500">
              {kodeCari?.wajib
                ? `Menarik data untuk ${kodeCari.nama} yang diisi dari Inaproc dan menyimpannya secara lokal; data lama untuk kode itu diganti.`
                : "Menarik seluruh data (semua halaman) dari Inaproc dan menyimpannya secara lokal."}
            </p>
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
