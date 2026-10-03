"use client";

import { useEffect, useRef, useState } from "react";
import { Building2, Loader2, X } from "lucide-react";
import { searchSLDKSatker, SLDKSatker } from "@/lib/api";

interface SatkerPickerProps {
  value: SLDKSatker | null;
  onChange: (satker: SLDKSatker | null) => void;
}

// Pemilih satker: mengetik minimal 3 huruf (nama atau awalan kode) mencari di tabel satker SLDK.
export function SatkerPicker({ value, onChange }: SatkerPickerProps) {
  const [text, setText] = useState("");
  const [items, setItems] = useState<SLDKSatker[]>([]);
  const [open, setOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [failed, setFailed] = useState(false);
  const requestId = useRef(0);
  const box = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const q = text.trim();
    if (q.length < 3) {
      requestId.current++;
      setItems([]);
      setIsLoading(false);
      setFailed(false);
      return;
    }
    const id = ++requestId.current;
    setIsLoading(true);
    setFailed(false);
    const timer = setTimeout(async () => {
      try {
        const res = await searchSLDKSatker(q);
        if (id === requestId.current) setItems(res.data.items ?? []);
      } catch {
        if (id === requestId.current) {
          setItems([]);
          setFailed(true);
        }
      } finally {
        if (id === requestId.current) setIsLoading(false);
      }
    }, 350);
    return () => clearTimeout(timer);
  }, [text]);

  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (box.current && !box.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, []);

  if (value) {
    return (
      <div className="flex items-center gap-2 rounded-lg border border-blue-200 bg-blue-50 px-3 py-2 text-sm text-blue-800">
        <Building2 className="h-4 w-4 shrink-0" />
        <span className="min-w-0 flex-1 truncate" title={value.nama}>
          {value.nama || `Satker ${value.id}`}
          {value.kode && <span className="text-blue-600"> ({value.kode})</span>}
        </span>
        <button
          type="button"
          onClick={() => {
            onChange(null);
            setText("");
          }}
          aria-label="Hapus satker terpilih"
          className="shrink-0 rounded p-0.5 hover:bg-blue-100"
        >
          <X className="h-4 w-4" />
        </button>
      </div>
    );
  }

  const q = text.trim();
  return (
    <div ref={box} className="relative">
      <Building2 className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
      <input
        type="text"
        autoComplete="off"
        aria-label="Cari satuan kerja"
        value={text}
        onChange={(e) => {
          setText(e.target.value);
          setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        onKeyDown={(e) => e.key === "Escape" && setOpen(false)}
        placeholder="Satuan kerja (min. 3 huruf)..."
        className="w-full rounded-lg border border-slate-300 bg-white py-2.5 pl-10 pr-9 text-sm outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10"
      />
      {isLoading && <Loader2 className="absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 animate-spin text-slate-400" />}

      {open && q.length >= 3 && !isLoading && (
        <ul
          role="listbox"
          aria-label="Hasil pencarian satuan kerja"
          className="absolute z-20 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-slate-200 bg-white py-1 text-sm shadow-lg"
        >
          {failed && <li className="px-3 py-2 text-xs text-red-600">Gagal mencari satuan kerja</li>}
          {!failed && items.length === 0 && <li className="px-3 py-2 text-xs text-slate-500">Satuan kerja tidak ditemukan</li>}
          {items.map((s) => (
            <li key={s.id} role="option" aria-selected="false">
              <button
                type="button"
                onClick={() => {
                  onChange(s);
                  setOpen(false);
                }}
                className="block w-full px-3 py-2 text-left hover:bg-slate-50 focus-visible:bg-slate-50 focus-visible:outline-none"
              >
                <span className="block truncate font-medium text-slate-800">{s.nama || `Satker ${s.id}`}</span>
                {s.kode && <span className="block text-xs text-slate-500">{s.kode}</span>}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
