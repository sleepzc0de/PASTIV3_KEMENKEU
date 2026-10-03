"use client";

import { KeyboardEvent, useEffect, useId, useRef, useState } from "react";
import { BadgeCheck, Building2, Loader2, Search, SearchX, UserRoundSearch } from "lucide-react";
import { SapaPegawai, cariSapaPegawai, getSapaPegawai } from "@/lib/sapa";
import { ErrorInfo, MIN_CARI_PEGAWAI, errorInfo, errorStatus, kataKunciPegawai } from "./sapa";

export interface PegawaiTerpilih {
  nip: string;
  nama: string; // dengan gelar
  jabatan: string; // kosong bila HRIS2 tidak memuatnya (diisi manual)
}

const TUNDA_MS = 400;

// Kotak pencarian pegawai HRIS2: ketik nama atau NIP, pilih hasilnya, lalu nama dan jabatan terisi otomatis lewat onPilih.
// Pencarian memakai sesi SSO pengguna, jadi akun lokal atau sesi yang habis mendapat penjelasan dan tetap bisa mengisi manual.
export function PegawaiPicker({
  onPilih,
  terpakai,
  autoFocus,
  label = "Cari pegawai di HRIS2",
  kataSukses = "ditambahkan",
}: {
  onPilih: (p: PegawaiTerpilih) => void;
  terpakai: Set<string>; // NIP yang sudah ada di daftar: ditandai dan tidak bisa dipilih lagi
  autoFocus?: boolean;
  label?: string;
  kataSukses?: string; // akhiran pesan setelah memilih: "<nama> <kataSukses>."
}) {
  const id = useId();
  const [query, setQuery] = useState("");
  const [hasil, setHasil] = useState<SapaPegawai[] | null>(null);
  const [memuat, setMemuat] = useState(false);
  const [mengisi, setMengisi] = useState<string | null>(null); // NIP yang sedang diambil detailnya
  const [error, setError] = useState<ErrorInfo | null>(null);
  const [open, setOpen] = useState(false);
  const [aktif, setAktif] = useState(0);
  const [catatan, setCatatan] = useState<string | null>(null);
  const wrap = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const nomor = useRef(0);

  // Pencarian ditunda sedikit setelah mengetik; permintaan lama dibatalkan supaya tidak menimpa yang baru.
  useEffect(() => {
    const kunci = kataKunciPegawai(query);
    setError(null);
    if (!kunci) {
      setHasil(null);
      setMemuat(false);
      return;
    }
    const ac = new AbortController();
    const mine = ++nomor.current;
    setMemuat(true);
    const t = setTimeout(() => {
      cariSapaPegawai(kunci, ac.signal)
        .then((res) => {
          if (mine !== nomor.current) return;
          setHasil(res.data.pegawai);
          setAktif(0);
          setOpen(true);
        })
        .catch((err) => {
          if (ac.signal.aborted || mine !== nomor.current) return;
          setHasil(null);
          setError(errorInfo(err, "Pencarian pegawai gagal"));
          setOpen(true);
        })
        .finally(() => {
          if (mine === nomor.current) setMemuat(false);
        });
    }, TUNDA_MS);
    return () => {
      clearTimeout(t);
      ac.abort();
    };
  }, [query]);

  // Tutup bila mengklik di luar.
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (wrap.current && !wrap.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  const pilih = async (p: SapaPegawai) => {
    if (terpakai.has(p.nip) || mengisi) return;
    setMengisi(p.nip);
    setCatatan(null);
    let nama = p.nama_lengkap || p.nama;
    let jabatan = p.jabatan;
    try {
      // Jabatan aktif hanya pasti ada pada detail pegawai, bukan pada daftar hasil pencarian.
      const d = (await getSapaPegawai(p.nip)).data;
      nama = d.nama_lengkap || d.nama || nama;
      jabatan = d.jabatan || jabatan;
    } catch (err) {
      // Detail gagal: tetap pakai data dari daftar; jabatan bisa kosong sehingga pengguna mengisinya.
      if (errorStatus(err) === 401) {
        setError(errorInfo(err, "Sesi SSO berakhir"));
        setMengisi(null);
        return;
      }
    }
    onPilih({ nip: p.nip, nama, jabatan });
    setCatatan(jabatan ? `${nama} ${kataSukses}.` : `${nama} ${kataSukses}; jabatan tidak ditemukan di HRIS2, isi manual.`);
    setQuery("");
    setHasil(null);
    setOpen(false);
    setMengisi(null);
  };

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Escape") {
      setOpen(false);
      return;
    }
    if (!hasil || hasil.length === 0) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setOpen(true);
      setAktif((a) => (a + 1) % hasil.length);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setAktif((a) => (a - 1 + hasil.length) % hasil.length);
    } else if (e.key === "Enter" && open) {
      e.preventDefault();
      void pilih(hasil[aktif]);
    }
  };

  const tampilkan = open && (error !== null || hasil !== null);

  return (
    <div ref={wrap} className="relative">
      <label htmlFor={id} className="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-slate-600">
        <UserRoundSearch className="h-3.5 w-3.5 text-blue-600" aria-hidden="true" />
        {label}
        <span className="font-normal text-slate-400">· nama atau NIP</span>
      </label>
      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
        <input
          id={id}
          ref={inputRef}
          type="text"
          role="combobox"
          aria-expanded={tampilkan}
          aria-controls={`${id}-list`}
          aria-autocomplete="list"
          aria-activedescendant={tampilkan && hasil?.[aktif] ? `${id}-opt-${aktif}` : undefined}
          autoComplete="off"
          autoFocus={autoFocus}
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setCatatan(null);
            setOpen(true);
          }}
          onFocus={() => hasil && setOpen(true)}
          onKeyDown={onKeyDown}
          placeholder={`Ketik minimal ${MIN_CARI_PEGAWAI} huruf nama atau NIP…`}
          className="w-full rounded-xl border border-slate-300 bg-white py-2.5 pl-9 pr-9 text-sm shadow-sm outline-none transition-all placeholder:text-slate-400 hover:border-slate-400 focus:border-blue-500 focus:shadow-glow"
        />
        {(memuat || mengisi) && <Loader2 className="absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 animate-spin text-blue-600" aria-label="Memuat" />}
      </div>

      {catatan && !tampilkan && (
        <p role="status" className="mt-1.5 flex items-center gap-1.5 text-xs text-emerald-700">
          <BadgeCheck className="h-3.5 w-3.5" aria-hidden="true" />
          {catatan}
        </p>
      )}

      {tampilkan && (
        <div className="absolute left-0 right-0 top-full z-30 mt-1.5 max-h-72 animate-scale-in overflow-y-auto rounded-xl bg-white p-1.5 shadow-lg ring-1 ring-slate-900/10">
          {error ? (
            <div role="alert" className="px-3 py-3 text-sm text-slate-700">
              <p className="font-medium text-red-700">{error.message}</p>
              <p className="mt-1 text-xs text-slate-500">Anda tetap dapat mengisi nama dan jabatan secara manual.</p>
            </div>
          ) : hasil && hasil.length === 0 ? (
            <div className="flex flex-col items-center px-3 py-6 text-center">
              <SearchX className="h-6 w-6 text-slate-400" aria-hidden="true" />
              <p className="mt-2 text-sm font-medium text-slate-700">Pegawai tidak ditemukan</p>
              <p className="mt-0.5 text-xs text-slate-500">Periksa ejaan, atau isi nama dan jabatan secara manual.</p>
            </div>
          ) : (
            <ul id={`${id}-list`} role="listbox" aria-label="Hasil pencarian pegawai">
              {hasil?.map((p, i) => {
                const sudah = terpakai.has(p.nip);
                const sel = i === aktif;
                return (
                  <li
                    key={p.nip || `${p.nama}-${i}`}
                    id={`${id}-opt-${i}`}
                    role="option"
                    aria-selected={sel}
                    aria-disabled={sudah}
                    onMouseMove={() => setAktif(i)}
                    onClick={() => void pilih(p)}
                    className={`flex items-start gap-3 rounded-lg px-3 py-2.5 ${sudah ? "cursor-not-allowed opacity-50" : "cursor-pointer"} ${sel && !sudah ? "bg-blue-50" : ""}`}
                  >
                    <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-slate-100 text-xs font-bold text-slate-600">
                      {Array.from(p.nama)[0]?.toUpperCase() ?? "?"}
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block break-words text-sm font-medium text-slate-900">{p.nama_lengkap || p.nama}</span>
                      <span className="mt-0.5 block font-mono text-[11px] text-slate-500">{p.nip || "NIP tidak tersedia"}</span>
                      {(p.jabatan || p.satker) && (
                        <span className="mt-0.5 flex items-start gap-1 text-xs text-slate-500">
                          <Building2 className="mt-0.5 h-3 w-3 shrink-0" aria-hidden="true" />
                          <span className="break-words">{[p.jabatan, p.satker].filter(Boolean).join(" · ")}</span>
                        </span>
                      )}
                    </span>
                    {sudah && <span className="shrink-0 rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-500">Sudah ditambahkan</span>}
                  </li>
                );
              })}
              {hasil && hasil.length >= 15 && <li className="px-3 py-2 text-center text-[11px] text-slate-400">Menampilkan 15 teratas; persempit kata kunci untuk hasil lebih tepat.</li>}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
