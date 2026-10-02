"use client";

import { FormEvent, useCallback, useRef, useState } from "react";
import { Search, Loader2, Users2, SearchX, ChevronRight, Building2, X, Info } from "lucide-react";
import axios from "axios";
import { searchHRIS2Pegawai } from "@/lib/api";
import { Alert } from "@/components/ui/Alert";
import { PegawaiDetailModal } from "@/components/hris2/PegawaiDetailModal";
import { PegawaiAvatar } from "@/components/hris2/PegawaiAvatar";
import { extractRows, extractError, normalizePegawai, namaLengkap, PegawaiRingkas } from "@/components/hris2/pegawai";

// Jumlah hasil yang diminta backend ke HRIS2 (PageSize di handlers/hris2_handler.go: SearchPegawai).
// Bila hasilnya sebanyak ini, kemungkinan masih ada pegawai lain yang tidak ikut tampil.
const MAX_RESULTS = 25;

export function PegawaiSearchTable() {
  const [query, setQuery] = useState("");
  const [submittedQuery, setSubmittedQuery] = useState("");
  const [results, setResults] = useState<PegawaiRingkas[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [hasSearched, setHasSearched] = useState(false);
  const [selectedNIP, setSelectedNIP] = useState<string | null>(null);

  const inputRef = useRef<HTMLInputElement>(null);
  // Nomor pencarian terakhir: respons dari pencarian lama diabaikan supaya tidak menimpa yang baru.
  const requestId = useRef(0);

  const handleSearch = useCallback(
    async (e?: FormEvent) => {
      e?.preventDefault();
      const q = query.trim();
      if (!q) return;

      const id = ++requestId.current;
      setIsLoading(true);
      setError(null);
      try {
        const res = await searchHRIS2Pegawai(q);
        if (id !== requestId.current) return;
        const envelopeError = extractError(res.data);
        setResults(envelopeError ? [] : extractRows(res.data).map(normalizePegawai));
        setError(envelopeError);
      } catch (err) {
        if (id !== requestId.current) return;
        setResults([]);
        if (axios.isAxiosError(err) && err.response) {
          setError(err.response.data?.message || "Pencarian gagal");
        } else {
          setError("Tidak dapat terhubung ke server");
        }
      } finally {
        if (id === requestId.current) {
          setSubmittedQuery(q);
          setHasSearched(true);
          setIsLoading(false);
        }
      }
    },
    [query]
  );

  const clearQuery = () => {
    setQuery("");
    inputRef.current?.focus();
  };

  return (
    <div className="w-full space-y-4">
      <form role="search" onSubmit={handleSearch} className="space-y-1.5">
        <div className="flex items-center gap-2">
          <div className="relative min-w-0 flex-1">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              ref={inputRef}
              type="text"
              inputMode="search"
              enterKeyHint="search"
              autoComplete="off"
              aria-label="Cari pegawai berdasarkan nama atau NIP"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Cari berdasarkan nama atau NIP..."
              className="w-full rounded-lg border border-slate-300 bg-white py-2.5 pl-10 pr-9 text-sm outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10"
            />
            {query && (
              <button
                type="button"
                onClick={clearQuery}
                aria-label="Hapus pencarian"
                className="absolute right-1.5 top-1/2 -translate-y-1/2 rounded-md p-1.5 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
              >
                <X className="h-4 w-4" />
              </button>
            )}
          </div>
          <button
            type="submit"
            disabled={isLoading || !query.trim()}
            className="flex shrink-0 items-center gap-2 rounded-lg bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60"
          >
            {isLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4" />}
            Cari
          </button>
        </div>
        <p className="text-xs text-slate-400">Nama (boleh sebagian) atau NIP 18 digit. Tekan Enter untuk mencari.</p>
      </form>

      {error && <Alert message={error} />}

      {isLoading && <ResultSkeleton />}

      {!isLoading && !hasSearched && !error && (
        <div className="flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-slate-300 px-4 py-14 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-blue-50 text-blue-500">
            <Users2 className="h-6 w-6" />
          </div>
          <p className="text-sm font-medium text-slate-700">Cari data pegawai</p>
          <p className="max-w-sm text-sm text-slate-500">
            Ketik nama atau NIP pegawai, lalu tekan Enter. Contoh: &ldquo;Budi Santoso&rdquo; atau &ldquo;198501012010011001&rdquo;.
          </p>
        </div>
      )}

      {!isLoading && hasSearched && !error && results.length === 0 && (
        <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-10 text-center">
          <SearchX className="mx-auto h-8 w-8 text-slate-400" />
          <p className="mt-2 break-words text-sm font-medium text-slate-700">
            Tidak ada pegawai yang cocok dengan &ldquo;{submittedQuery}&rdquo;
          </p>
          <p className="mt-1 text-sm text-slate-500">Periksa ejaan, coba sebagian nama saja, atau cari memakai NIP.</p>
        </div>
      )}

      {!isLoading && results.length > 0 && (
        <div className="space-y-2">
          <p aria-live="polite" className="break-words text-sm text-slate-600">
            <span className="font-semibold text-slate-900">{results.length}</span> pegawai untuk &ldquo;{submittedQuery}&rdquo;
          </p>

          {results.length >= MAX_RESULTS && (
            <div className="flex items-start gap-2 rounded-lg bg-amber-50 px-3.5 py-2.5 text-xs text-amber-800">
              <Info className="mt-0.5 h-4 w-4 shrink-0" />
              <span>
                Hanya {MAX_RESULTS} hasil teratas yang ditampilkan. Persempit pencarian dengan nama lengkap atau NIP agar
                pegawai yang dicari tidak terlewat.
              </span>
            </div>
          )}

          <ul className="divide-y divide-slate-100 overflow-hidden rounded-lg border border-slate-200">
            {results.map((p, idx) => (
              <li key={`${p.nip}-${idx}`}>
                <button
                  type="button"
                  onClick={() => setSelectedNIP(p.nip)}
                  disabled={!p.nip}
                  title={p.nip ? undefined : "NIP tidak tersedia, detail tidak dapat dibuka"}
                  className="group flex w-full items-center gap-3 px-3 py-3 text-left hover:bg-slate-50 focus-visible:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-60 sm:px-4"
                >
                  <PegawaiAvatar nama={p.nama} src={p.foto} />
                  <div className="min-w-0 flex-1">
                    <p className="line-clamp-2 break-words text-sm font-semibold text-slate-900">
                      {namaLengkap(p) || "Tanpa nama"}
                    </p>
                    <p className="mt-0.5 truncate font-mono text-xs text-slate-500">{p.nip || "NIP tidak tersedia"}</p>
                    {p.satker && (
                      <p className="mt-0.5 flex items-center gap-1 text-xs text-slate-500">
                        <Building2 className="h-3 w-3 shrink-0 text-slate-400" />
                        <span className="truncate">{p.satker}</span>
                      </p>
                    )}
                    {p.jabatan && <p className="mt-0.5 truncate text-xs text-slate-400">{p.jabatan}</p>}
                  </div>
                  {p.golongan && (
                    <span className="hidden shrink-0 rounded-full bg-blue-50 px-2.5 py-0.5 text-xs font-medium text-blue-700 sm:inline-block">
                      {p.golongan}
                    </span>
                  )}
                  {p.nip && <span className="hidden shrink-0 text-xs font-medium text-blue-600 sm:inline">Lihat detail</span>}
                  <ChevronRight className="h-4 w-4 shrink-0 text-slate-300 group-hover:text-slate-500" />
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}

      {selectedNIP && <PegawaiDetailModal nip={selectedNIP} onClose={() => setSelectedNIP(null)} />}
    </div>
  );
}

function ResultSkeleton() {
  return (
    <div
      role="status"
      aria-label="Memuat hasil pencarian"
      className="divide-y divide-slate-100 overflow-hidden rounded-lg border border-slate-200"
    >
      {Array.from({ length: 5 }, (_, i) => (
        <div key={i} className="flex animate-pulse items-center gap-3 px-3 py-3 sm:px-4">
          <div className="h-10 w-10 shrink-0 rounded-full bg-slate-200" />
          <div className="flex-1 space-y-2">
            <div className="h-3.5 w-1/2 rounded bg-slate-200" />
            <div className="h-3 w-1/3 rounded bg-slate-100" />
          </div>
        </div>
      ))}
    </div>
  );
}
