"use client";

import { useState } from "react";
import type { JumlahPer, OpsiAudit, RingkasanAudit } from "@/lib/audit";
import { tanggalSingkatWIB, waktuSingkatWIB, waktuWIB } from "@/lib/auditFilter";
import { ChartCard, DataTable } from "@/components/ui/charts";

const BIRU = "#3358e0";
const MERAH = "#d03b3b";

function Batang({ r }: { r: RingkasanAudit }) {
  const [sorot, setSorot] = useState<number | null>(null);
  const maks = Math.max(1, ...r.deret.map((t) => t.jumlah));
  const n = r.deret.length;
  const lebar = 640;
  const tinggi = 150;
  const lebarBatang = Math.max(2, Math.min(28, (lebar / Math.max(n, 1)) * 0.7));
  const x = (i: number) => (n <= 1 ? lebar / 2 : 12 + (i / (n - 1)) * (lebar - 24));
  const label = (iso: string) => (r.per_jam ? waktuSingkatWIB(iso, true) : tanggalSingkatWIB(iso));
  if (n === 0) return <p className="rounded-xl border border-dashed border-slate-300 bg-slate-50/60 px-4 py-8 text-center text-sm text-slate-500">Belum ada aktivitas pada rentang ini.</p>;
  return (
    <div className="relative">
      <svg viewBox={`0 0 ${lebar} ${tinggi + 20}`} role="img" aria-label="Aktivitas per waktu. Gunakan tombol Tabel untuk membaca nilainya." className="h-auto w-full select-none">
        <line x1={0} x2={lebar} y1={tinggi} y2={tinggi} stroke="#e1e7ef" />
        {r.deret.map((t, i) => {
          const h = (t.jumlah / maks) * (tinggi - 6);
          const hg = (t.gagal / maks) * (tinggi - 6);
          return (
            <g key={t.waktu} onMouseEnter={() => setSorot(i)} onMouseLeave={() => setSorot(null)}>
              <rect x={x(i) - lebarBatang / 2 - 2} y={0} width={lebarBatang + 4} height={tinggi} fill="transparent" />
              <rect x={x(i) - lebarBatang / 2} y={tinggi - h} width={lebarBatang} height={Math.max(h, 1)} rx="2" fill={BIRU} opacity={sorot === null || sorot === i ? 1 : 0.55} />
              {t.gagal > 0 && <rect x={x(i) - lebarBatang / 2} y={tinggi - hg} width={lebarBatang} height={Math.max(hg, 1)} rx="2" fill={MERAH} />}
            </g>
          );
        })}
        <text x={0} y={tinggi + 14} fontSize="10" fill="#64748b">{label(r.deret[0].waktu)}</text>
        <text x={lebar} y={tinggi + 14} fontSize="10" fill="#64748b" textAnchor="end">{label(r.deret[n - 1].waktu)}</text>
      </svg>
      {sorot !== null && (
        <div role="status" className="pointer-events-none absolute top-0 z-10 rounded-lg bg-slate-900/95 px-2.5 py-1.5 text-xs text-white shadow-lg" style={{ left: `${Math.min(Math.max((x(sorot) / lebar) * 100, 10), 90)}%`, transform: "translateX(-50%)" }}>
          <p className="font-medium text-slate-200">{r.per_jam ? waktuWIB(r.deret[sorot].waktu, false) : tanggalSingkatWIB(r.deret[sorot].waktu)}</p>
          <p>
            <span className="font-semibold tabular-nums">{r.deret[sorot].jumlah}</span> aktivitas
            {r.deret[sorot].gagal > 0 && <span className="text-red-300"> · {r.deret[sorot].gagal} gagal</span>}
          </p>
        </div>
      )}
      <ul className="mt-1 flex gap-4 text-xs text-slate-600">
        <li className="flex items-center gap-1.5">
          <span className="inline-block h-2 w-3 rounded-sm" style={{ background: BIRU }} aria-hidden="true" />
          Semua aktivitas
        </li>
        <li className="flex items-center gap-1.5">
          <span className="inline-block h-2 w-3 rounded-sm" style={{ background: MERAH }} aria-hidden="true" />
          Gagal atau ditolak
        </li>
      </ul>
    </div>
  );
}

function Daftar({ judul, deskripsi, item, onPilih, waspada }: { judul: string; deskripsi: string; item: JumlahPer[]; onPilih?: (kode: string) => void; waspada?: number }) {
  return (
    <section className="min-w-0 rounded-2xl border border-slate-200/80 bg-white p-4 shadow-sm">
      <h3 className="text-sm font-semibold text-slate-900">{judul}</h3>
      <p className="mt-0.5 text-xs text-slate-500">{deskripsi}</p>
      {item.length === 0 ? (
        <p className="mt-3 text-sm text-slate-400">Tidak ada.</p>
      ) : (
        <ul className="mt-3 space-y-1">
          {item.map((j) => {
            const isi = (
              <>
                <span className="min-w-0 flex-1 truncate text-left" title={j.label || j.kode}>
                  {j.label || j.kode}
                </span>
                <span className={`shrink-0 rounded-full px-2 py-0.5 text-xs font-semibold tabular-nums ${waspada && j.jumlah >= waspada ? "bg-red-50 text-red-700" : "bg-slate-100 text-slate-700"}`}>{j.jumlah.toLocaleString("id-ID")}</span>
              </>
            );
            return (
              <li key={j.kode}>
                {onPilih ? (
                  <button type="button" onClick={() => onPilih(j.kode)} className="flex w-full items-center gap-3 rounded-lg px-2 py-1.5 text-sm text-slate-800 hover:bg-slate-50" title="Saring log menurut ini">
                    {isi}
                  </button>
                ) : (
                  <div className="flex items-center gap-3 px-2 py-1.5 text-sm text-slate-800">{isi}</div>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

// Gambaran aktivitas pada rentang terpilih: grafik per jam/hari dan tiga daftar yang bisa diklik untuk menyaring log.
export function RingkasanAktivitas({ r, opsi, onKategori, onAksi, onIP }: { r: RingkasanAudit; opsi: OpsiAudit | null; onKategori: (kode: string) => void; onAksi: (kode: string) => void; onIP: (ip: string) => void }) {
  // Uraian per kelompok aksi dari backend diambil dari satu entri sembarang (memuat ID spesifik); nama umum dari katalog lebih jelas.
  const aksi = r.aksi_teratas.map((j) => ({ ...j, label: opsi?.aksi.find((a) => a.kode === j.kode)?.label ?? j.label }));
  return (
    <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
      <ChartCard
        title={r.per_jam ? "Aktivitas per jam" : "Aktivitas per hari"}
        subtitle="Biru: semua aktivitas. Merah: yang gagal atau ditolak."
        table={<DataTable columns={[r.per_jam ? "Jam (WIB)" : "Tanggal", "Aktivitas", "Gagal"]} rows={r.deret.map((t) => [r.per_jam ? waktuWIB(t.waktu, false) : tanggalSingkatWIB(t.waktu), String(t.jumlah), String(t.gagal)])} />}
      >
        <Batang r={r} />
      </ChartCard>
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-1">
        <Daftar judul="Menurut kategori" deskripsi="Klik untuk menyaring log." item={r.per_kategori} onPilih={onKategori} />
        <Daftar judul="Aksi teratas" deskripsi="Klik untuk menyaring log." item={aksi} onPilih={onAksi} />
      </div>
      {r.login_gagal_ip.length > 0 && (
        <div className="xl:col-span-2">
          <Daftar judul="Sumber login gagal terbanyak (alamat IP)" deskripsi="Banyak login gagal dari satu alamat dapat berarti percobaan menebak kata sandi. Klik untuk menyaring log menurut alamat itu." item={r.login_gagal_ip} onPilih={onIP} waspada={5} />
        </div>
      )}
    </div>
  );
}
