"use client";

import { useState, useCallback } from "react";
import { Search, Loader2, Users2, Eye } from "lucide-react";
import axios from "axios";
import { searchHRIS2Pegawai } from "@/lib/api";
import { PegawaiDetailModal } from "@/components/hris2/PegawaiDetailModal";

function extractRows(raw: unknown): Record<string, unknown>[] {
  if (Array.isArray(raw)) return raw as Record<string, unknown>[];
  if (raw && typeof raw === "object") {
    const obj = raw as Record<string, unknown>;
    // Respons HRIS2 dibungkus { statusCode, isError, data: [...] }
    if ("data" in obj) {
      const inner = obj.data;
      if (Array.isArray(inner)) return inner as Record<string, unknown>[];
      if (inner && typeof inner === "object") return [inner as Record<string, unknown>];
    }
    for (const key of ["items", "result", "results"]) {
      if (Array.isArray(obj[key])) return obj[key] as Record<string, unknown>[];
    }
    return [obj];
  }
  return [];
}

function getNIP(row: Record<string, unknown>): string {
  const candidates = ["nip18", "Nip18", "nip", "NIP"];
  for (const key of candidates) {
    const v = row[key];
    if (typeof v === "string" && v) return v;
  }
  return "";
}

export function PegawaiSearchTable() {
  const [query, setQuery] = useState("");
  const [rows, setRows] = useState<Record<string, unknown>[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [hasSearched, setHasSearched] = useState(false);
  const [selectedNIP, setSelectedNIP] = useState<string | null>(null);

  const handleSearch = useCallback(async () => {
    if (!query.trim()) return;
    setIsLoading(true);
    setError(null);
    try {
      const res = await searchHRIS2Pegawai(query);
      setRows(extractRows(res.data));
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
            placeholder="Cari pegawai berdasarkan nama atau NIP..."
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
        <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>
      )}

      {!hasSearched && !error && (
        <div className="flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-slate-300 py-16 text-slate-400">
          <Users2 className="h-8 w-8" />
          <p className="text-sm">Masukkan nama atau NIP untuk mencari data pegawai (HRIS2)</p>
        </div>
      )}

      {hasSearched && !error && rows.length === 0 && (
        <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-8 text-center text-sm text-slate-500">
          Tidak ada data pegawai yang cocok
        </div>
      )}

      {rows.length > 0 && (
        <div className="space-y-2">
          {rows.map((row, idx) => {
            const nip = getNIP(row);
            const nama = String(row.nama || row.Nama || "-");
            const satker = String(row.namaSatker || row.NamaSatker || "-");

            return (
              <div
                key={idx}
                className="flex items-center justify-between rounded-lg border border-slate-200 px-4 py-3 hover:bg-slate-50"
              >
                <div>
                  <p className="text-sm font-semibold text-slate-900">{nama}</p>
                  <p className="text-xs text-slate-500">
                    NIP: {nip || "-"} &middot; {satker}
                  </p>
                </div>
                <button
                  onClick={() => setSelectedNIP(nip)}
                  disabled={!nip}
                  className="flex items-center gap-1.5 rounded-lg border border-slate-300 px-3 py-1.5 text-xs font-medium text-slate-600 hover:bg-slate-100 disabled:opacity-40"
                >
                  <Eye className="h-3.5 w-3.5" />
                  Lihat Detail
                </button>
              </div>
            );
          })}
        </div>
      )}

      {selectedNIP && (
        <PegawaiDetailModal nip={selectedNIP} onClose={() => setSelectedNIP(null)} />
      )}
    </div>
  );
}