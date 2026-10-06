"use client";

import { useEffect, useRef, useState } from "react";
import { X, RefreshCcw, Briefcase, Building2, Award, UserRound, CalendarDays } from "lucide-react";
import axios from "axios";
import { searchPegawaiByNIP } from "@/lib/api";
import { formatDate } from "@/lib/format";
import { Alert } from "@/components/ui/Alert";
import { DetailGroup, DetailField } from "@/components/ui/DetailEntry";
import { PegawaiAvatar } from "@/components/hris2/PegawaiAvatar";
import { CopyButton } from "@/components/hris2/CopyButton";
import { normalizeDetail, namaLengkap, PegawaiDetail } from "@/components/hris2/pegawai";

interface PegawaiDetailModalProps {
  nip: string;
  onClose: () => void;
}

export function PegawaiDetailModal({ nip, onClose }: PegawaiDetailModalProps) {
  const [detail, setDetail] = useState<PegawaiDetail | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);
    setError(null);
    setDetail(null);

    searchPegawaiByNIP(nip)
      .then((res) => {
        if (!cancelled) setDetail(normalizeDetail(res.data ?? {}, nip));
      })
      .catch((err) => {
        if (cancelled) return;
        if (axios.isAxiosError(err) && err.response) {
          setError(err.response.data?.message || "Gagal mengambil detail pegawai");
        } else {
          setError("Gagal terhubung ke server");
        }
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [nip, reloadKey]);

  // Elemen yang membuka modal dicatat saat render pertama, sebelum autoFocus memindahkan fokus,
  // supaya fokus bisa dikembalikan ke sana dan pengguna keyboard tidak kehilangan posisi.
  const [opener] = useState<HTMLElement | null>(() =>
    typeof document !== "undefined" && document.activeElement instanceof HTMLElement ? document.activeElement : null
  );
  // onClose dari induk berupa fungsi inline yang berganti tiap render; disimpan di ref agar efek
  // di bawah cukup berjalan sekali.
  const onCloseRef = useRef(onClose);
  useEffect(() => {
    onCloseRef.current = onClose;
  });

  const closeButtonRef = useRef<HTMLButtonElement>(null);

  // Fokus dipindahkan di efek yang sama dengan pemulihannya (bukan lewat atribut autoFocus) agar
  // urutannya pasti: ke tombol Tutup saat dibuka, kembali ke pembuka saat ditutup.
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCloseRef.current();
    };
    document.addEventListener("keydown", onKeyDown);
    closeButtonRef.current?.focus();
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      opener?.focus();
    };
  }, [opener]);

  return (
    // `!mt-0`: modal ini dirender di dalam wadah `space-y-*` yang memberi margin-top pada
    // setiap anaknya; tanpa ini overlay `fixed inset-0` bergeser dan menyisakan celah di atas.
    <div
      className="fixed inset-0 z-50 !mt-0 flex animate-fade-in items-end justify-center bg-slate-950/50 p-0 backdrop-blur-sm sm:items-center sm:p-4"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Detail Profil Pegawai"
        className="max-h-[90dvh] w-full max-w-2xl overflow-y-auto overscroll-contain animate-scale-in rounded-t-3xl bg-white shadow-xl sm:rounded-2xl"
      >
        <div className="sticky top-0 z-10 flex items-center justify-between gap-3 border-b border-slate-100 bg-white/90 px-4 py-4 backdrop-blur sm:px-6">
          <h2 className="text-base font-semibold text-slate-900">Detail Profil Pegawai</h2>
          <button
            type="button"
            ref={closeButtonRef}
            onClick={onClose}
            aria-label="Tutup"
            className="shrink-0 rounded-md p-1.5 text-slate-400 hover:bg-slate-100"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="p-4 sm:p-6">
          {isLoading && <DetailSkeleton />}

          {error && (
            <div className="space-y-3">
              <Alert message={error} />
              <button
                type="button"
                onClick={() => setReloadKey((k) => k + 1)}
                className="flex items-center gap-1.5 rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
              >
                <RefreshCcw className="h-3.5 w-3.5" />
                Coba lagi
              </button>
            </div>
          )}

          {!isLoading && !error && detail && <DetailContent d={detail} />}
        </div>
      </div>
    </div>
  );
}

function DetailContent({ d }: { d: PegawaiDetail }) {
  const ttl = [d.tempatLahir, d.tanggalLahir && formatDate(d.tanggalLahir)].filter(Boolean).join(", ");

  const pribadi: DetailField[] = [
    ["Tempat, Tanggal Lahir", ttl],
    ["Jenis Kelamin", d.jenisKelamin],
    ["No. HP", d.noHp],
    ["Email", d.email],
  ];
  const satker: DetailField[] = [
    ["Satuan Kerja", d.satker],
    ["Kode Satker", d.kdSatker],
  ];
  const pangkat: DetailField[] = [
    ["Pangkat", d.namaPangkat],
    ["Golongan", d.kodeGolongan],
    ["TMT Pangkat", d.tmtPangkat && formatDate(d.tmtPangkat)],
  ];

  const nama = namaLengkap(d);
  const aktif = /aktif/i.test(d.status);
  const hasGroups = [pribadi, satker, pangkat].some((fields) => fields.some(([, value]) => value));

  return (
    <div className="space-y-6">
      <div className="flex items-start gap-4">
        <PegawaiAvatar nama={d.nama} src={d.foto} size="lg" />
        <div className="min-w-0 flex-1">
          <p className="break-words text-lg font-bold leading-snug text-slate-900">{nama || "Tanpa nama"}</p>
          <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-0.5">
            <span className="font-mono text-sm text-slate-600">{d.nip}</span>
            <CopyButton text={d.nip} label="Salin NIP" />
          </div>
          {(d.status || d.kodeGolongan) && (
            <div className="mt-2 flex flex-wrap gap-1.5">
              {d.status && (
                <span
                  className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
                    aktif ? "bg-green-50 text-green-700" : "bg-slate-100 text-slate-600"
                  }`}
                >
                  {d.status}
                </span>
              )}
              {d.kodeGolongan && (
                <span className="rounded-full bg-blue-50 px-2.5 py-0.5 text-xs font-medium text-blue-700">
                  Gol. {d.kodeGolongan}
                </span>
              )}
            </div>
          )}
        </div>
      </div>

      <DetailGroup icon={UserRound} title="Data Pribadi" fields={pribadi} />
      <DetailGroup icon={Building2} title="Penempatan" fields={satker} />
      <DetailGroup icon={Award} title="Pangkat / Golongan" fields={pangkat} />

      {d.jabatanList.length > 0 && (
        <section>
          <h3 className="mb-2 flex items-center gap-1.5 text-sm font-semibold text-slate-700">
            <Briefcase className="h-4 w-4" />
            Jabatan
            <span className="font-normal text-slate-400">({d.jabatanList.length})</span>
          </h3>
          <ul className="space-y-3">
            {d.jabatanList.map((j, idx) => (
              <li key={idx} className="rounded-lg border border-slate-200 p-3 text-sm">
                <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-1">
                  <p className="min-w-0 break-words font-medium text-slate-900">{j.namaJabatan || "-"}</p>
                  {j.statusJabatan && (
                    <span className="rounded-full bg-blue-50 px-2 py-0.5 text-[11px] font-medium text-blue-700">
                      {j.statusJabatan}
                    </span>
                  )}
                </div>
                {j.jenisJabatan && <p className="mt-0.5 text-xs text-slate-500">{j.jenisJabatan}</p>}
                {j.unit.length > 0 && (
                  <ol className="mt-2 space-y-0.5 border-l-2 border-slate-100 pl-3 text-xs text-slate-500">
                    {j.unit.map((u) => (
                      <li key={u} className="break-words">
                        {u}
                      </li>
                    ))}
                  </ol>
                )}
                {j.tanggalMulai && (
                  <p className="mt-2 flex items-center gap-1 text-xs text-slate-400">
                    <CalendarDays className="h-3 w-3" />
                    TMT {formatDate(j.tanggalMulai)}
                  </p>
                )}
              </li>
            ))}
          </ul>
        </section>
      )}

      {!hasGroups && d.jabatanList.length === 0 && (
        <p className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-6 text-center text-sm text-slate-500">
          HRIS2 tidak mengirim rincian profil untuk pegawai ini.
        </p>
      )}
    </div>
  );
}

function DetailSkeleton() {
  return (
    <div role="status" aria-label="Memuat detail pegawai" className="animate-pulse space-y-6">
      <div className="flex items-center gap-4">
        <div className="h-16 w-16 shrink-0 rounded-full bg-slate-200 sm:h-20 sm:w-20" />
        <div className="flex-1 space-y-2.5">
          <div className="h-5 w-3/4 rounded bg-slate-200" />
          <div className="h-3.5 w-1/2 rounded bg-slate-100" />
          <div className="h-5 w-24 rounded-full bg-slate-100" />
        </div>
      </div>
      {[0, 1, 2].map((i) => (
        <div key={i} className="space-y-2">
          <div className="h-4 w-32 rounded bg-slate-200" />
          <div className="grid grid-cols-1 gap-3 rounded-lg border border-slate-100 p-3 sm:grid-cols-2">
            <div className="h-9 rounded bg-slate-100" />
            <div className="h-9 rounded bg-slate-100" />
          </div>
        </div>
      ))}
    </div>
  );
}
