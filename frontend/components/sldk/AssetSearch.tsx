"use client";

import { FormEvent, ReactNode, useEffect, useRef, useState } from "react";
import axios from "axios";
import { Search, Loader2, DatabaseZap, ChevronRight, Info, SearchX, Package, MapPin, RotateCcw, Building2 } from "lucide-react";
import { getSLDKReferences, searchSLDKAssets, SLDKReferences, SLDKSatker, SLDKAssetSearchData } from "@/lib/api";
import { formatCurrency, formatDate } from "@/lib/format";
import { Alert } from "@/components/ui/Alert";
import { SatkerPicker } from "@/components/sldk/SatkerPicker";
import { AssetDetailModal } from "@/components/sldk/AssetDetailModal";
import { KondisiBadge, FlagBadge } from "@/components/sldk/AssetBadges";
import { AssetSummary, normalizeAsset, namaDari, str } from "@/components/sldk/asset";

const MIN_TEXT = 3;

interface SearchResult {
  assets: AssetSummary[];
  satker: SLDKAssetSearchData["satker"];
  limit: number;
  elapsedMs: number;
}

interface SearchError {
  message: string;
  status?: number;
}

export function AssetSearch() {
  const [refs, setRefs] = useState<SLDKReferences | null>(null);
  const [by, setBy] = useState<"id" | "teks">("id");
  const [q, setQ] = useState("");
  const [jns, setJns] = useState("");
  const [kondisi, setKondisi] = useState("");
  const [status, setStatus] = useState("");
  const [tahun, setTahun] = useState("");
  const [satker, setSatker] = useState<SLDKSatker | null>(null);

  const [result, setResult] = useState<SearchResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<SearchError | null>(null);
  const [selected, setSelected] = useState<AssetSummary | null>(null);
  const requestId = useRef(0);

  // Tabel referensi (kode -> nama) kecil; bila gagal, daftar filter disembunyikan dan kode ditampilkan apa adanya.
  useEffect(() => {
    let cancelled = false;
    getSLDKReferences()
      .then((res) => {
        if (!cancelled) setRefs(res.data);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);

  const qTrim = q.trim();
  const tahunTrim = tahun.trim();
  const tahunInvalid = tahunTrim !== "" && !/^\d{4}$/.test(tahunTrim);
  const qTooShort = qTrim.length > 0 && qTrim.length < MIN_TEXT;
  const hasFilter = Boolean(jns || kondisi || status || tahunTrim || satker);
  const canSearch = !isLoading && !qTooShort && !tahunInvalid && (qTrim.length > 0 || hasFilter);

  const handleSearch = async (e?: FormEvent) => {
    e?.preventDefault();
    if (!canSearch) return;
    const id = ++requestId.current;
    setIsLoading(true);
    setError(null);
    try {
      const res = await searchSLDKAssets({
        q: qTrim || undefined,
        by,
        id_satker: satker?.id,
        kd_jns_bmn: jns || undefined,
        kd_kondisi: kondisi || undefined,
        kd_status: status || undefined,
        tahun: tahunTrim || undefined,
      });
      if (id !== requestId.current) return;
      setResult({
        assets: res.data.results.map(normalizeAsset),
        satker: res.data.satker ?? {},
        limit: res.data.limit,
        elapsedMs: res.data.elapsed_ms,
      });
    } catch (err) {
      if (id !== requestId.current) return;
      setResult(null);
      if (axios.isAxiosError(err) && err.response) {
        setError({ message: err.response.data?.message || "Pencarian gagal", status: err.response.status });
      } else if (axios.isAxiosError(err) && err.code === "ECONNABORTED") {
        setError({ message: "Pencarian terlalu lama dan dihentikan. Persempit dengan satker, jenis BMN, atau cari memakai kode register.", status: 504 });
      } else {
        setError({ message: "Tidak dapat terhubung ke server" });
      }
    } finally {
      if (id === requestId.current) setIsLoading(false);
    }
  };

  const reset = () => {
    requestId.current++;
    setQ("");
    setJns("");
    setKondisi("");
    setStatus("");
    setTahun("");
    setSatker(null);
    setResult(null);
    setError(null);
    setIsLoading(false);
  };

  const dataPer = result?.assets[0] ? str(result.assets[0].raw._ingestion_date) : "";

  return (
    <div className="w-full space-y-4">
      <form onSubmit={handleSearch} role="search" className="space-y-3">
        <div className="inline-flex rounded-lg border border-slate-200 bg-slate-50 p-0.5 text-sm" role="group" aria-label="Jenis pencarian">
          {(
            [
              ["id", "Kode / nomor"],
              ["teks", "Teks bebas"],
            ] as const
          ).map(([value, label]) => (
            <button
              key={value}
              type="button"
              onClick={() => setBy(value)}
              aria-pressed={by === value}
              className={`rounded-md px-3 py-1.5 font-medium ${by === value ? "bg-white text-blue-700 shadow-sm" : "text-slate-500 hover:text-slate-700"}`}
            >
              {label}
            </button>
          ))}
        </div>

        <div className="flex items-center gap-2">
          <div className="relative min-w-0 flex-1">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              type="text"
              inputMode="search"
              enterKeyHint="search"
              autoComplete="off"
              aria-label="Kata kunci pencarian aset"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder={by === "id" ? "Kode register, No. KIB, No. polisi, atau serial number..." : "Nama barang, merk, tipe, alamat, atau unit pengguna..."}
              className="w-full rounded-lg border border-slate-300 bg-white py-2.5 pl-10 pr-3.5 text-sm outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10"
            />
          </div>
          <button
            type="submit"
            disabled={!canSearch}
            className="flex shrink-0 items-center gap-2 rounded-lg bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60"
          >
            {isLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4" />}
            Cari
          </button>
        </div>
        <p className="text-xs text-slate-400">
          {by === "id"
            ? "Pencocokan persis, jadi paling cepat. Bisa digabung dengan filter di bawah."
            : `Mencari kata di nama barang, merk, tipe, alamat, dan unit pengguna (min. ${MIN_TEXT} huruf). Lebih lambat; sebaiknya digabung dengan filter satker.`}
        </p>
        {qTooShort && <p className="text-xs text-amber-600">Kata kunci minimal {MIN_TEXT} karakter.</p>}

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-6">
          <div className="sm:col-span-2 lg:col-span-2">
            <FieldLabel>Satuan kerja</FieldLabel>
            <SatkerPicker value={satker} onChange={setSatker} />
          </div>
          {refs && refs.jenis_bmn.length > 0 && (
            <FilterSelect label="Jenis BMN" value={jns} onChange={setJns} items={refs.jenis_bmn} />
          )}
          {refs && refs.kondisi.length > 0 && (
            <FilterSelect label="Kondisi" value={kondisi} onChange={setKondisi} items={refs.kondisi} />
          )}
          {refs && refs.status_penggunaan.length > 0 && (
            <FilterSelect label="Status penggunaan" value={status} onChange={setStatus} items={refs.status_penggunaan} />
          )}
          <div>
            <FieldLabel>Tahun perolehan</FieldLabel>
            <input
              type="text"
              inputMode="numeric"
              maxLength={4}
              value={tahun}
              onChange={(e) => setTahun(e.target.value.replace(/\D/g, ""))}
              placeholder="mis. 2021"
              aria-invalid={tahunInvalid}
              className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10"
            />
          </div>
        </div>

        {(hasFilter || qTrim) && (
          <button type="button" onClick={reset} className="inline-flex items-center gap-1 text-xs font-medium text-slate-500 hover:text-slate-700">
            <RotateCcw className="h-3.5 w-3.5" />
            Reset pencarian dan filter
          </button>
        )}
      </form>

      {error && (
        <div className="space-y-2">
          <Alert message={error.message} />
          {(error.status === 504 || error.status === 429) && (
            <p className="text-xs text-slate-500">
              Tabel aset SLDK berisi ratusan juta baris. Pencarian paling cepat memakai kode register atau No. KIB, dan filter satuan kerja.
            </p>
          )}
        </div>
      )}

      {isLoading && (
        <div role="status" aria-label="Mencari aset" className="divide-y divide-slate-100 overflow-hidden rounded-lg border border-slate-200">
          {Array.from({ length: 4 }, (_, i) => (
            <div key={i} className="flex animate-pulse items-center gap-3 px-3 py-3 sm:px-4">
              <div className="h-10 w-10 shrink-0 rounded-lg bg-slate-200" />
              <div className="flex-1 space-y-2">
                <div className="h-3.5 w-1/2 rounded bg-slate-200" />
                <div className="h-3 w-1/3 rounded bg-slate-100" />
              </div>
            </div>
          ))}
          <p className="px-4 py-2 text-xs text-slate-400">Mencari di SLDK; bisa memakan waktu hingga 25 detik...</p>
        </div>
      )}

      {!isLoading && !result && !error && (
        <div className="flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-slate-300 px-4 py-14 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-blue-50 text-blue-500">
            <DatabaseZap className="h-6 w-6" />
          </div>
          <p className="text-sm font-medium text-slate-700">Telusuri aset BMN dari SLDK</p>
          <p className="max-w-md text-sm text-slate-500">
            Isi kode register, No. KIB, No. polisi, atau serial number untuk pencarian tercepat. Atau pilih satuan kerja, jenis BMN,
            kondisi, status, dan tahun perolehan untuk menyaring.
          </p>
        </div>
      )}

      {!isLoading && result && result.assets.length === 0 && (
        <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-10 text-center">
          <SearchX className="mx-auto h-8 w-8 text-slate-400" />
          <p className="mt-2 text-sm font-medium text-slate-700">Tidak ada aset yang cocok</p>
          <p className="mt-1 text-sm text-slate-500">
            Periksa ejaan kode, kurangi filter, atau coba mode &ldquo;Teks bebas&rdquo;. Pencarian kode harus persis sama.
          </p>
        </div>
      )}

      {!isLoading && result && result.assets.length > 0 && (
        <div className="space-y-2">
          <p aria-live="polite" className="text-sm text-slate-600">
            <span className="font-semibold text-slate-900">{result.assets.length}</span> aset
            {result.assets.length >= result.limit ? " pertama" : ""} ditemukan
            <span className="text-slate-400"> · {(result.elapsedMs / 1000).toFixed(1)} detik</span>
            {dataPer && <span className="text-slate-400"> · data per {formatDate(dataPer)}</span>}
          </p>

          {result.assets.length >= result.limit && (
            <div className="flex items-start gap-2 rounded-lg bg-amber-50 px-3.5 py-2.5 text-xs text-amber-800">
              <Info className="mt-0.5 h-4 w-4 shrink-0" />
              <span>Hanya {result.limit} aset pertama yang ditampilkan, tanpa urutan tertentu. Persempit dengan filter agar aset yang dicari tidak terlewat.</span>
            </div>
          )}

          <ul className="divide-y divide-slate-100 overflow-hidden rounded-lg border border-slate-200">
            {result.assets.map((a, idx) => {
              const satkerNama = a.idSatker !== null ? result.satker[String(a.idSatker)]?.nama : "";
              const kondisiNama = namaDari(refs?.kondisi, a.kdKondisi);
              return (
                <li key={`${a.idAset ?? "x"}-${idx}`}>
                  <button
                    type="button"
                    onClick={() => setSelected(a)}
                    className="group flex w-full items-start gap-3 px-3 py-3 text-left hover:bg-slate-50 focus-visible:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 sm:px-4"
                  >
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-500">
                      <Package className="h-5 w-5" />
                    </div>
                    <div className="min-w-0 flex-1 space-y-0.5">
                      <p className="line-clamp-2 break-words text-sm font-semibold text-slate-900">{a.nama || "Tanpa nama"}</p>
                      {a.merkTipe && <p className="truncate text-xs text-slate-500">{a.merkTipe}</p>}
                      <p className="truncate font-mono text-xs text-slate-500">{a.kodeRegister || "Tanpa kode register"}</p>
                      {(satkerNama || a.lokasi) && (
                        <p className="flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-slate-500">
                          {satkerNama && (
                            <span className="inline-flex min-w-0 items-center gap-1">
                              <Building2 className="h-3 w-3 shrink-0 text-slate-400" />
                              <span className="truncate">{satkerNama}</span>
                            </span>
                          )}
                          {a.lokasi && (
                            <span className="inline-flex min-w-0 items-center gap-1">
                              <MapPin className="h-3 w-3 shrink-0 text-slate-400" />
                              <span className="truncate">{a.lokasi}</span>
                            </span>
                          )}
                        </p>
                      )}
                      <div className="flex flex-wrap items-center gap-1.5 pt-1">
                        <KondisiBadge kode={a.kdKondisi} nama={kondisiNama} />
                        {a.flags.map((f) => (
                          <FlagBadge key={f} label={f} />
                        ))}
                        {a.tahunPerolehan && <span className="text-xs text-slate-400">Perolehan {a.tahunPerolehan}</span>}
                        {/* Di layar kecil kolom nilai di kanan disembunyikan; nilai buku dipindah ke sini. */}
                        {a.nilaiBuku !== null && (
                          <span className="text-xs text-slate-500 sm:hidden">Nilai buku {formatCurrency(a.nilaiBuku)}</span>
                        )}
                      </div>
                    </div>
                    <div className="hidden shrink-0 text-right sm:block">
                      {a.nilaiBuku !== null && (
                        <>
                          <p className="text-sm font-semibold text-slate-800">{formatCurrency(a.nilaiBuku)}</p>
                          <p className="text-xs text-slate-400">Nilai buku</p>
                        </>
                      )}
                    </div>
                    <ChevronRight className="mt-2 h-4 w-4 shrink-0 text-slate-300 group-hover:text-slate-500" />
                  </button>
                </li>
              );
            })}
          </ul>
        </div>
      )}

      {selected && (
        <AssetDetailModal
          asset={selected}
          refs={refs}
          satkerNama={selected.idSatker !== null ? result?.satker[String(selected.idSatker)]?.nama ?? "" : ""}
          onClose={() => setSelected(null)}
        />
      )}
    </div>
  );
}

function FieldLabel({ children }: { children: ReactNode }) {
  return <label className="mb-1.5 block text-xs font-medium text-slate-600">{children}</label>;
}

function FilterSelect({
  label,
  value,
  onChange,
  items,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  items: { kode: string; nama: string }[];
}) {
  return (
    <div>
      <FieldLabel>{label}</FieldLabel>
      <select
        aria-label={label}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10"
      >
        <option value="">Semua</option>
        {items.map((i) => (
          <option key={i.kode} value={i.kode}>
            {i.nama || i.kode}
          </option>
        ))}
      </select>
    </div>
  );
}
