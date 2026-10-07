"use client";

import { KeyboardEvent, useCallback, useEffect, useId, useRef, useState } from "react";
import { Link2, Link2Off, Search, SearchX } from "lucide-react";
import { SapaPilihanKanwil, getSapaRefKanwil } from "@/lib/sapa";
import { CheckField, NoticeBox, TextField } from "./fields";
import { errorInfo, kode9DariSatker, labelPilihanKanwil, saringPilihanKanwil } from "./sapa";

const MAKS_HASIL = 8;

// Daftar Kanwil aktif dari Referensi Kanwil (dikelola superadmin) untuk pemilih tembusan; gagal memuat tidak menghalangi pengisian manual.
export function useRefKanwilSapa() {
  const [daftar, setDaftar] = useState<SapaPilihanKanwil[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const muat = useCallback(async () => {
    setError(null);
    try {
      const res = await getSapaRefKanwil();
      setDaftar(res.data.daftar ?? []);
    } catch (err) {
      setError(errorInfo(err, "Gagal memuat Referensi Kanwil").message);
    }
  }, []);

  useEffect(() => {
    void muat();
  }, [muat]);

  return { daftar, error, muat };
}

interface Props {
  kode: string; // kode Kanwil (9 digit) yang terhubung ke Referensi Kanwil; kosong bila diketik manual
  teks: string; // teks tembusan yang tercetak: "Kepala Kantor Wilayah ..."
  tanpa: boolean; // satker tidak punya Kanwil
  kodeSatker: string; // kode satker usulan: 9 karakter pertamanya = kode Kanwil yang disarankan
  onPilih: (p: SapaPilihanKanwil) => void;
  onLepas: () => void;
  onTeks: (t: string) => void;
  onTanpa: (v: boolean) => void;
  hint?: string;
}

// Tembusan Kepala Kantor Wilayah: cari Kanwil menurut kode atau uraiannya dari Referensi Kanwil, pilih, lalu kode dan teks tembusannya terisi dan terhubung ke referensi. Teks tetap bisa
// diubah (mis. penyesuaian penulisan), dan Kanwil yang tidak ada di referensi bisa diketik manual. Tidak semua satker punya Kanwil: kotak centang membuang butir ini dari dokumen.
export function KanwilField({ kode, teks, tanpa, kodeSatker, onPilih, onLepas, onTeks, onTanpa, hint }: Props) {
  const id = useId();
  const { daftar, error, muat } = useRefKanwilSapa();
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [aktif, setAktif] = useState(0);
  const wrap = useRef<HTMLDivElement>(null);

  const kodeSaran = kode9DariSatker(kodeSatker);
  const hasil = saringPilihanKanwil(daftar ?? [], query, MAKS_HASIL, kodeSaran);
  const terpilih = kode ? (daftar ?? []).find((p) => p.kode === kode) : undefined;
  const saran = !kode && kodeSaran ? (daftar ?? []).find((p) => p.kode === kodeSaran) : undefined;
  const tampilkan = open && !tanpa && daftar !== null;

  // Tutup bila mengklik di luar.
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (wrap.current && !wrap.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  const pilih = (p: SapaPilihanKanwil) => {
    onPilih(p);
    setQuery("");
    setOpen(false);
  };

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Escape") {
      setOpen(false);
      return;
    }
    if (hasil.length === 0) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setOpen(true);
      setAktif((a) => (a + 1) % hasil.length);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setAktif((a) => (a - 1 + hasil.length) % hasil.length);
    } else if (e.key === "Enter" && open) {
      e.preventDefault();
      pilih(hasil[Math.min(aktif, hasil.length - 1)]);
    }
  };

  return (
    <div className="space-y-3 sm:col-span-2">
      <div ref={wrap} className="relative">
        <label htmlFor={id} className="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-slate-600">
          Kanwil untuk tembusan
          {!tanpa && (
            <span aria-hidden="true" className="text-red-500">
              *
            </span>
          )}
          <span className="font-normal text-slate-400">· cari menurut kode atau uraian Referensi Kanwil</span>
        </label>
        <div className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
          <input
            id={id}
            type="text"
            role="combobox"
            aria-expanded={tampilkan}
            aria-controls={`${id}-list`}
            aria-autocomplete="list"
            aria-activedescendant={tampilkan && hasil[aktif] ? `${id}-opt-${aktif}` : undefined}
            autoComplete="off"
            disabled={tanpa}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setAktif(0);
              setOpen(true);
            }}
            onFocus={() => setOpen(true)}
            onKeyDown={onKeyDown}
            placeholder={daftar === null && !error ? "Memuat Referensi Kanwil…" : "Ketik kode (mis. 015040199) atau uraian Kanwil…"}
            className="w-full rounded-xl border border-slate-300 bg-white py-2.5 pl-9 pr-3 text-sm shadow-sm outline-none transition-all placeholder:text-slate-400 hover:border-slate-400 focus:border-blue-500 focus:shadow-glow disabled:cursor-not-allowed disabled:bg-slate-50 disabled:text-slate-500 disabled:shadow-none"
          />
        </div>

        {tampilkan && (
          <div className="absolute left-0 right-0 top-full z-30 mt-1.5 max-h-72 animate-scale-in overflow-y-auto rounded-xl bg-white p-1.5 shadow-lg ring-1 ring-slate-900/10">
            {hasil.length === 0 ? (
              <div className="flex flex-col items-center px-3 py-6 text-center">
                <SearchX className="h-6 w-6 text-slate-400" aria-hidden="true" />
                <p className="mt-2 text-sm font-medium text-slate-700">{(daftar ?? []).length === 0 ? "Referensi Kanwil belum berisi data" : "Kanwil tidak ditemukan"}</p>
                <p className="mt-0.5 text-xs text-slate-500">Anda tetap dapat mengetik teks tembusan secara manual di bawah.</p>
              </div>
            ) : (
              <ul id={`${id}-list`} role="listbox" aria-label="Hasil pencarian Kanwil">
                {hasil.map((p, i) => (
                  <li
                    key={p.kode}
                    id={`${id}-opt-${i}`}
                    role="option"
                    aria-selected={i === aktif}
                    onMouseMove={() => setAktif(i)}
                    onClick={() => pilih(p)}
                    className={`cursor-pointer rounded-lg px-3 py-2.5 ${i === aktif ? "bg-blue-50" : ""}`}
                  >
                    <span className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
                      <span className="font-mono text-xs text-slate-500">{p.kode}</span>
                      {p.kode === kodeSaran && <span className="rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700">Sesuai kode satker</span>}
                      {p.kode === kode && <span className="rounded-full bg-blue-50 px-2 py-0.5 text-[11px] font-medium text-blue-700">Dipilih</span>}
                    </span>
                    <span className="mt-0.5 block break-words text-sm font-medium text-slate-900">{p.nama}</span>
                    {p.singkatan && <span className="block text-xs text-slate-500">{p.singkatan}</span>}
                  </li>
                ))}
                {(daftar ?? []).length > MAKS_HASIL && hasil.length >= MAKS_HASIL && (
                  <li className="px-3 py-2 text-center text-[11px] text-slate-400">Menampilkan {MAKS_HASIL} teratas; ketik lebih banyak untuk mempersempit.</li>
                )}
              </ul>
            )}
          </div>
        )}
      </div>

      {error && (
        <NoticeBox tone="warn">
          Referensi Kanwil tidak dapat dimuat ({error}). Ketik teks tembusan secara manual di bawah, atau{" "}
          <button type="button" onClick={() => void muat()} className="font-semibold underline underline-offset-2">
            coba muat lagi
          </button>
          .
        </NoticeBox>
      )}

      {!tanpa && kode && (
        <p role="status" className="flex flex-wrap items-center gap-x-2 gap-y-1 rounded-lg bg-emerald-50 px-3 py-2 text-xs text-emerald-800">
          <Link2 className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          <span className="min-w-0 break-words">
            Terhubung ke Referensi Kanwil: <span className="font-semibold">{terpilih ? labelPilihanKanwil(terpilih) : `${kode} (tidak ada pada Referensi Kanwil aktif)`}</span>
          </span>
          <button type="button" onClick={onLepas} className="ml-auto rounded px-1.5 py-0.5 font-medium text-emerald-900 underline underline-offset-2 hover:bg-emerald-100">
            Lepas
          </button>
        </p>
      )}
      {!tanpa && !kode && saran && (
        <p className="flex flex-wrap items-center gap-x-2 gap-y-1 rounded-lg bg-blue-50 px-3 py-2 text-xs text-blue-900">
          <Link2Off className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          <span className="min-w-0 break-words">
            Belum terhubung ke referensi. Kanwil sesuai kode satker: <span className="font-semibold">{labelPilihanKanwil(saran)}</span>
          </span>
          <button type="button" onClick={() => pilih(saran)} className="ml-auto rounded px-1.5 py-0.5 font-medium underline underline-offset-2 hover:bg-blue-100">
            Pakai
          </button>
        </p>
      )}

      <TextField
        label="Teks tembusan pada dokumen"
        required={!tanpa}
        value={tanpa ? "" : teks}
        onChange={onTeks}
        maxLength={300}
        disabled={tanpa}
        placeholder="mis. Kepala Kantor Wilayah DJKN Jakarta"
        hint={hint ?? "Terisi dari Referensi Kanwil yang dipilih; ubah bila penulisannya perlu disesuaikan. Bila Kanwil tidak ada di referensi, ketik manual."}
      />
      <CheckField
        label="Satker ini tidak punya Kanwil"
        checked={tanpa}
        onChange={onTanpa}
        hint="Tidak semua satker punya Kanwil. Bila dicentang, butir tembusan Kanwil tidak dicetak."
      />
    </div>
  );
}
