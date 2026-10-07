"use client";

import { ReactNode, useState } from "react";
import { ChevronDown, CircleAlert, CircleCheck, CircleHelp, TriangleAlert } from "lucide-react";
import type { Kapasitas, StatusResource } from "@/lib/monitor";
import { KELAS_STATUS, formatAngka, formatRingkas, labelStatus } from "@/lib/monitorFormat";

// Potongan tampilan kecil yang dipakai bersama oleh panel-panel Monitor Resource.

export function IkonStatus({ status, className = "h-4 w-4" }: { status: StatusResource; className?: string }) {
  if (status === "kritis") return <CircleAlert className={`${className} text-red-600`} aria-hidden="true" />;
  if (status === "perhatian") return <TriangleAlert className={`${className} text-amber-600`} aria-hidden="true" />;
  if (status === "cukup") return <CircleCheck className={`${className} text-emerald-600`} aria-hidden="true" />;
  return <CircleHelp className={`${className} text-slate-400`} aria-hidden="true" />;
}

export function LencanaStatus({ status, kelompok }: { status: StatusResource; kelompok?: Kapasitas["kelompok"] }) {
  return (
    <span className={`inline-flex shrink-0 items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-semibold ring-1 ring-inset ${KELAS_STATUS[status].lencana}`}>
      <span className={`h-1.5 w-1.5 rounded-full ${KELAS_STATUS[status].titik}`} aria-hidden="true" />
      {labelStatus(status, kelompok)}
    </span>
  );
}

// Bilah pemakaian 0-100%. Teks angka selalu ditampilkan di samping bilah (warna saja tidak cukup untuk membaca nilainya).
export function Pengukur({ label, nilai, teks, status, sub }: { label: string; nilai: number | null; teks: string; status: StatusResource; sub?: string }) {
  const lebar = nilai === null ? 0 : Math.min(100, Math.max(0, nilai));
  return (
    <div className="min-w-0">
      <div className="flex items-baseline justify-between gap-3">
        <span className="text-sm font-medium text-slate-700">{label}</span>
        <span className="text-sm font-semibold tabular-nums text-slate-900">{teks}</span>
      </div>
      <div className="mt-1.5 h-2.5 overflow-hidden rounded-full bg-slate-100" role="meter" aria-label={label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={nilai ?? undefined} aria-valuetext={teks}>
        <div className={`h-full rounded-full transition-[width] duration-500 ${KELAS_STATUS[status].garis}`} style={{ width: `${lebar}%` }} />
      </div>
      {sub && <p className="mt-1 text-xs text-slate-500">{sub}</p>}
    </div>
  );
}

// Daftar "nama: nilai" rapat.
export function BarisInfo({ baris }: { baris: [string, ReactNode][] }) {
  return (
    <dl className="divide-y divide-slate-100 text-sm">
      {baris.map(([k, v]) => (
        <div key={k} className="flex items-start justify-between gap-4 py-2">
          <dt className="text-slate-500">{k}</dt>
          <dd className="min-w-0 break-words text-right font-medium text-slate-900">{v}</dd>
        </div>
      ))}
    </dl>
  );
}

export function Panel({ judul, deskripsi, children, aksi }: { judul: string; deskripsi?: string; children: ReactNode; aksi?: ReactNode }) {
  return (
    <section className="min-w-0 rounded-2xl border border-slate-200/80 bg-white p-4 shadow-sm sm:p-5">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-slate-900">{judul}</h3>
          {deskripsi && <p className="mt-0.5 text-xs text-slate-500">{deskripsi}</p>}
        </div>
        {aksi}
      </div>
      <div className="mt-3">{children}</div>
    </section>
  );
}

function teksSatuan(k: Kapasitas, v: number | null): string {
  if (v === null) return "-";
  if (k.satuan === "%") return `${formatAngka(v, 1)}%`;
  if (k.satuan.startsWith("ms")) return `${formatRingkas(v)} ms`;
  if (k.satuan === "koneksi") return `${formatRingkas(v)} koneksi`;
  if (k.satuan === "goroutine") return `${formatRingkas(v)}`;
  if (k.satuan.startsWith("detik")) return `${formatRingkas(v)} dtk`;
  return formatRingkas(v);
}

// Kartu penilaian satu resource: status, tindakan yang disarankan, angka sekarang dan puncaknya, serta rincian alasan yang bisa dibuka.
export function KartuKapasitas({ k }: { k: Kapasitas }) {
  const [buka, setBuka] = useState(false);
  const persen = k.satuan === "%" && k.sekarang !== null ? k.sekarang : k.batas && k.sekarang !== null && k.satuan === "koneksi" ? (k.sekarang / k.batas) * 100 : null;
  const tebal = k.status === "kritis" ? "font-semibold text-red-700" : k.status === "perhatian" ? "font-semibold text-amber-800" : "text-slate-600";
  return (
    <article className={`flex min-w-0 flex-col rounded-2xl border bg-white p-4 shadow-sm ${KELAS_STATUS[k.status].kartu}`}>
      <header className="flex items-start justify-between gap-3">
        <h4 className="flex min-w-0 items-center gap-2 text-sm font-semibold text-slate-900">
          <IkonStatus status={k.status} />
          <span className="min-w-0 break-words">{k.nama}</span>
        </h4>
        <LencanaStatus status={k.status} kelompok={k.kelompok} />
      </header>
      <p className={`mt-2 text-sm ${tebal}`}>{k.tindakan}</p>

      {persen !== null && (
        <div className="mt-3">
          <Pengukur label={k.nama} nilai={persen} teks={teksSatuan(k, k.sekarang)} status={k.status} sub={k.satuan === "koneksi" && k.batas ? `dari batas ${formatRingkas(k.batas)} koneksi` : undefined} />
        </div>
      )}
      <dl className="mt-3 grid grid-cols-3 gap-2 text-xs">
        <div>
          <dt className="text-slate-500">Sekarang</dt>
          <dd className="mt-0.5 font-semibold tabular-nums text-slate-900">{teksSatuan(k, k.sekarang)}</dd>
        </div>
        <div>
          <dt className="text-slate-500">Puncak 24 jam</dt>
          <dd className="mt-0.5 font-semibold tabular-nums text-slate-900">{teksSatuan(k, k.puncak_24j)}</dd>
        </div>
        <div>
          <dt className="text-slate-500">Puncak 7 hari</dt>
          <dd className="mt-0.5 font-semibold tabular-nums text-slate-900">{teksSatuan(k, k.puncak_7h)}</dd>
        </div>
      </dl>

      {k.rincian.length > 0 && (
        <div className="mt-3">
          <button
            type="button"
            onClick={() => setBuka((v) => !v)}
            aria-expanded={buka}
            className="inline-flex items-center gap-1 rounded-md py-1 text-xs font-medium text-blue-700 hover:underline"
          >
            <ChevronDown className={`h-3.5 w-3.5 transition-transform ${buka ? "rotate-180" : ""}`} aria-hidden="true" />
            {buka ? "Sembunyikan alasan" : "Lihat alasan dan angka"}
          </button>
          {buka && (
            <ul className="mt-2 space-y-1.5 rounded-xl bg-slate-50 p-3 text-xs leading-relaxed text-slate-700">
              {k.rincian.map((r, i) => (
                <li key={i}>{r}</li>
              ))}
            </ul>
          )}
        </div>
      )}
    </article>
  );
}
