"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import axios from "axios";
import { ChevronRight, CheckCircle2, FolderTree, Loader2, RefreshCcw } from "lucide-react";
import { SkeletonTable } from "@/components/ui/Skeleton";
import { getEkatalog6Kategori, syncEkatalog6Kategori, Ekatalog6KategoriItem } from "@/lib/api";
import { useDashboard } from "@/lib/dashboard-context";

// Jalur telusur: kosong = level 1; kd1 = level 2 (anak kategori itu); kd1 + kd2 = level 3.
interface Jalur {
  kd1?: string;
  nama1?: string;
  kd2?: string;
  nama2?: string;
}

interface Baris {
  kode: string;
  nama: string;
  datamart: number | null;
}

const tingkatDari = (j: Jalur) => (j.kd2 ? 3 : j.kd1 ? 2 : 1);

// Tiap tingkat hanya memuat pasangan kode/nama untuk tingkatnya sendiri.
function keBaris(item: Ekatalog6KategoriItem, tingkat: number): Baris {
  const kode = tingkat === 1 ? item.kd_kategori_1 : tingkat === 2 ? item.kd_kategori_2 : item.kd_kategori_3;
  const nama = tingkat === 1 ? item.nama_kategori_1 : tingkat === 2 ? item.nama_kategori_2 : item.nama_kategori_3;
  return { kode: kode ?? "", nama: nama ?? "", datamart: item.datamart_id ?? null };
}

// Telusur kategori produk E-Katalog V6 (L1 -> L2 -> L3): klik baris untuk turun satu level, klik remah roti untuk naik. Dimuat
// otomatis karena level 1 tidak butuh isian. Admin dapat menarik tingkat yang sedang dilihat ke database.
export function Ekatalog6KategoriBrowser() {
  const { profile } = useDashboard();
  const isAdmin = profile ? ["admin", "superadmin"].includes(profile.role) : false;

  const [jalur, setJalur] = useState<Jalur>({});
  const [rows, setRows] = useState<Baris[]>([]);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [hasMore, setHasMore] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [isSyncing, setIsSyncing] = useState(false);
  const [syncMessage, setSyncMessage] = useState<string | null>(null);
  const [syncError, setSyncError] = useState<string | null>(null);

  // Jawaban untuk jalur yang sudah ditinggalkan diabaikan.
  const permintaanTerakhir = useRef(0);
  const tingkat = tingkatDari(jalur);

  const muat = useCallback(async (j: Jalur, useCursor?: string) => {
    const nomor = ++permintaanTerakhir.current;
    const t = tingkatDari(j);
    if (useCursor) {
      setIsLoadingMore(true);
    } else {
      setIsLoading(true);
      setRows([]);
    }
    setError(null);
    try {
      const res = await getEkatalog6Kategori({ kd_kategori_1: j.kd1, kd_kategori_2: j.kd2, limit: 100, cursor: useCursor });
      if (nomor !== permintaanTerakhir.current) return;
      const baru = (res.data ?? []).map((d) => keBaris(d, t));
      setRows((prev) => (useCursor ? [...prev, ...baru] : baru));
      setCursor(res.meta?.cursor || undefined);
      setHasMore(Boolean(res.meta?.has_more));
    } catch (err) {
      if (nomor !== permintaanTerakhir.current) return;
      if (axios.isAxiosError(err) && err.response) {
        const msg = err.response.data?.error?.message || err.response.data?.message || "Gagal memuat kategori";
        const details = err.response.data?.error?.details;
        setError(details ? `${msg}: ${details}` : msg);
      } else {
        setError("Tidak dapat terhubung ke server");
      }
    } finally {
      if (nomor === permintaanTerakhir.current) {
        setIsLoading(false);
        setIsLoadingMore(false);
      }
    }
  }, []);

  useEffect(() => {
    muat(jalur);
  }, [jalur, muat]);

  const pindah = (j: Jalur) => {
    setSyncMessage(null);
    setSyncError(null);
    setJalur(j);
  };

  const turun = (b: Baris) => {
    if (!b.kode || tingkat >= 3) return;
    pindah(tingkat === 1 ? { kd1: b.kode, nama1: b.nama } : { ...jalur, kd2: b.kode, nama2: b.nama });
  };

  const tarik = async () => {
    setIsSyncing(true);
    setSyncError(null);
    setSyncMessage(null);
    try {
      const res = await syncEkatalog6Kategori({ kd_kategori_1: jalur.kd1, kd_kategori_2: jalur.kd2 });
      const gagal = res.data.total_failed ?? 0;
      setSyncMessage(`Berhasil menyinkronkan ${res.data.total_synced} baris kategori level ${tingkat} ke database.` + (gagal > 0 ? ` ${gagal} baris gagal disimpan (lihat log server).` : ""));
    } catch (err) {
      setSyncError(axios.isAxiosError(err) && err.response ? err.response.data?.message || "Sinkronisasi gagal" : "Tidak dapat terhubung ke server");
    } finally {
      setIsSyncing(false);
    }
  };

  const induk = tingkat === 3 ? jalur.nama2 || jalur.kd2 : tingkat === 2 ? jalur.nama1 || jalur.kd1 : null;

  return (
    <div className="w-full space-y-4">
      <div className="flex items-center gap-2.5 rounded-xl border border-blue-100 bg-blue-50/70 px-4 py-3 text-sm text-blue-800">
        <FolderTree className="h-4 w-4 shrink-0" />
        <span>Kategori produk E-Katalog V6. Klik kategori untuk melihat turunannya (level 1 → 2 → 3).</span>
      </div>

      <nav aria-label="Jalur kategori" className="flex flex-wrap items-center gap-1 text-sm">
        <button onClick={() => pindah({})} disabled={tingkat === 1} className={tingkat === 1 ? "font-semibold text-slate-800" : "text-blue-600 hover:underline"}>
          Semua kategori
        </button>
        {jalur.kd1 && (
          <>
            <ChevronRight className="h-3.5 w-3.5 text-slate-400" />
            <button
              onClick={() => pindah({ kd1: jalur.kd1, nama1: jalur.nama1 })}
              disabled={tingkat === 2}
              className={tingkat === 2 ? "font-semibold text-slate-800" : "text-blue-600 hover:underline"}
            >
              {jalur.nama1 || jalur.kd1}
            </button>
          </>
        )}
        {jalur.kd2 && (
          <>
            <ChevronRight className="h-3.5 w-3.5 text-slate-400" />
            <span className="font-semibold text-slate-800">{jalur.nama2 || jalur.kd2}</span>
          </>
        )}
      </nav>

      {isAdmin && (
        <div className="flex flex-wrap items-center gap-3 rounded-xl border border-slate-200 bg-slate-50/80 px-4 py-3">
          <div className="flex-1">
            <p className="text-sm font-medium text-slate-700">Sinkronkan ke Database PASTI V3</p>
            <p className="text-xs text-slate-500">
              Menarik kategori level {tingkat}
              {induk ? ` di bawah "${induk}"` : ""} dan menyimpannya secara lokal; data lama untuk level dan induk yang sama diganti.
            </p>
          </div>
          <button
            onClick={tarik}
            disabled={isSyncing}
            className="flex shrink-0 items-center gap-2 rounded-lg bg-slate-800 px-4 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-slate-900 active:scale-[0.98] disabled:opacity-60"
          >
            {isSyncing ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCcw className="h-4 w-4" />}
            {isSyncing ? "Menyinkronkan..." : "Tarik Level Ini ke Database"}
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

      {isLoading && <SkeletonTable rows={6} cols={3} />}

      {!isLoading && !error && rows.length === 0 && (
        <div className="rounded-2xl border border-dashed border-slate-300 bg-slate-50/60 px-4 py-10 text-center text-sm text-slate-500">Tidak ada kategori pada level ini</div>
      )}

      {rows.length > 0 && (
        <>
          <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm">
            <table className="w-full text-sm">
              <thead className="bg-slate-50">
                <tr>
                  <th className="whitespace-nowrap px-4 py-2.5 text-left font-semibold text-slate-600">Kode Level {tingkat}</th>
                  <th className="whitespace-nowrap px-4 py-2.5 text-left font-semibold text-slate-600">Nama Kategori</th>
                  <th className="px-4 py-2.5" />
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {rows.map((b, i) => (
                  <tr key={`${b.kode}-${i}`} onClick={() => turun(b)} className={tingkat < 3 ? "cursor-pointer hover:bg-slate-50" : ""}>
                    <td className="whitespace-nowrap px-4 py-2.5 font-mono text-xs text-slate-600">{b.kode || "-"}</td>
                    <td className="px-4 py-2.5 font-medium text-slate-800">{b.nama || "-"}</td>
                    <td className="whitespace-nowrap px-4 py-2.5 text-right text-slate-400">{tingkat < 3 && <ChevronRight className="inline h-4 w-4" />}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="flex items-center justify-between">
            <p className="text-xs text-slate-400">Menampilkan {rows.length} kategori</p>
            {hasMore && (
              <button
                onClick={() => muat(jalur, cursor)}
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
