"use client";

import { useState, useCallback } from "react";
import { Search, Loader2, DatabaseZap } from "lucide-react";
import axios from "axios";
import { searchSLDKAssets, SLDKColumn } from "@/lib/api";

export function AssetSearchTable() {
  const [query, setQuery] = useState("");
  const [columns, setColumns] = useState<SLDKColumn[]>([]);
  const [results, setResults] = useState<Record<string, unknown>[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [hasSearched, setHasSearched] = useState(false);

  const handleSearch = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const res = await searchSLDKAssets(query);
      setColumns(res.data.columns);
      setResults(res.data.results);
      setHasSearched(true);
    } catch (err) {
      if (axios.isAxiosError(err) && err.response) {
        setError(err.response.data?.message || "Pencarian gagal");
      } else {
        setError("Tidak dapat terhubung ke server");
      }
    } finally {
      setIsLoading(false);
    }
  }, [query]);

  return (
    <div className="w-full space-y-4">
      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleSearch()}
            placeholder="Cari data aset dari SLDK (Interchange)..."
            className="w-full rounded-lg border border-slate-300 bg-white py-2.5 pl-10 pr-3.5 text-sm outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10"
          />
        </div>
        <button
          onClick={handleSearch}
          disabled={isLoading}
          className="flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60"
        >
          {isLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4" />}
          Cari
        </button>
      </div>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      )}

      {!hasSearched && !error && (
        <div className="flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-slate-300 py-16 text-slate-400">
          <DatabaseZap className="h-8 w-8" />
          <p className="text-sm">Masukkan kata kunci untuk mencari data aset dari SLDK</p>
        </div>
      )}

      {hasSearched && !error && results.length === 0 && (
        <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-8 text-center text-sm text-slate-500">
          Tidak ada data yang cocok dengan pencarian &quot;{query}&quot;
        </div>
      )}

      {results.length > 0 && (
        <div className="overflow-x-auto rounded-lg border border-slate-200">
          <table className="w-full text-sm">
            <thead className="bg-slate-50">
              <tr>
                {columns.map((col) => (
                  <th
                    key={col.name}
                    className="whitespace-nowrap px-4 py-2.5 text-left font-semibold text-slate-600"
                  >
                    {col.name}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {results.map((row, idx) => (
                <tr key={idx} className="hover:bg-slate-50">
                  {columns.map((col) => (
                    <td key={col.name} className="whitespace-nowrap px-4 py-2.5 text-slate-700">
                      {row[col.name] !== null && row[col.name] !== undefined
                        ? String(row[col.name])
                        : <span className="text-slate-300">—</span>}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {results.length > 0 && (
        <p className="text-xs text-slate-400">Menampilkan {results.length} hasil</p>
      )}
    </div>
  );
}