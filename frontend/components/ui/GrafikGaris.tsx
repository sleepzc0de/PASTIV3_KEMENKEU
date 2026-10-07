"use client";

import { useMemo, useRef, useState } from "react";
import { ChartCard, DataTable } from "@/components/ui/charts";
import { waktuSingkatWIB, waktuWIB } from "@/lib/auditFilter";
import { type DeretGrafik, batasAtas, jalurGaris, jumlahTitik } from "@/lib/monitorFormat";

// Grafik garis ringan tanpa pustaka: beberapa deret berbagi satu sumbu Y, garis putus pada celah data, sorot dengan mouse atau sentuh menampilkan nilai pada waktu itu.
// Setiap grafik punya padanan tabel (tombol Tabel) supaya nilainya bisa dibaca tanpa melihat garis.

const LEBAR = 640;
const TINGGI = 200;
const KIRI = 46;
const BAWAH = 22;
const ATAS = 8;

interface Props {
  judul: string;
  subjudul?: string;
  waktu: string[]; // ISO UTC per titik
  deret: DeretGrafik[];
  format: (v: number) => string; // nilai pada sumbu dan tooltip
  maksY?: number; // sumbu tetap (mis. 100 untuk persen); tanpa ini dibulatkan dari data
  denganTanggal?: boolean; // label sumbu X memuat tanggal (rentang lebih dari sehari)
}

export function GrafikGaris({ judul, subjudul, waktu, deret, format, maksY, denganTanggal = false }: Props) {
  const [sorot, setSorot] = useState<number | null>(null);
  const kotak = useRef<HTMLDivElement>(null);
  const atas = useMemo(() => batasAtas(deret.map((d) => d.nilai), maksY), [deret, maksY]);
  const ada = deret.some((d) => jumlahTitik(d.nilai) >= 2);
  const n = waktu.length;

  const tinggiPlot = TINGGI - BAWAH - ATAS;
  const lebarPlot = LEBAR - KIRI;
  const x = (i: number) => KIRI + (n <= 1 ? lebarPlot / 2 : (i / (n - 1)) * lebarPlot);

  const tabel = useMemo(() => {
    // paling banyak 48 baris (diambil merata) supaya tabel tetap ringkas
    const langkah = Math.max(1, Math.ceil(n / 48));
    const baris: string[][] = [];
    for (let i = n - 1; i >= 0; i -= langkah) baris.push([waktuWIB(waktu[i], false), ...deret.map((d) => (d.nilai[i] === null || d.nilai[i] === undefined ? "-" : format(d.nilai[i] as number)))]);
    return baris;
  }, [waktu, deret, n, format]);

  const gerak = (clientX: number) => {
    const el = kotak.current;
    if (!el || n === 0) return;
    const r = el.getBoundingClientRect();
    const px = ((clientX - r.left) / r.width) * LEBAR; // koordinat dalam viewBox
    const i = Math.round(((px - KIRI) / lebarPlot) * (n - 1));
    setSorot(Math.min(n - 1, Math.max(0, n === 1 ? 0 : i)));
  };

  const tick = [0, 0.25, 0.5, 0.75, 1];
  const labelX = n <= 1 ? [0] : [0, Math.floor((n - 1) / 3), Math.floor(((n - 1) * 2) / 3), n - 1];

  return (
    <ChartCard title={judul} subtitle={subjudul} table={ada ? <DataTable columns={["Waktu (WIB)", ...deret.map((d) => d.nama)]} rows={tabel} /> : undefined}>
      {!ada ? (
        <p className="rounded-xl border border-dashed border-slate-300 bg-slate-50/60 px-4 py-10 text-center text-sm text-slate-500">
          Belum ada cukup data pada rentang ini. Pengukuran disimpan setiap 5 menit, jadi grafik terisi setelah beberapa saat aplikasi berjalan.
        </p>
      ) : (
        <div>
          <div ref={kotak} className="relative" onMouseMove={(e) => gerak(e.clientX)} onMouseLeave={() => setSorot(null)} onTouchMove={(e) => e.touches[0] && gerak(e.touches[0].clientX)} onTouchEnd={() => setSorot(null)}>
            <svg viewBox={`0 0 ${LEBAR} ${TINGGI}`} role="img" aria-label={`${judul}. Gunakan tombol Tabel untuk membaca nilainya.`} className="h-auto w-full touch-pan-y select-none">
              {tick.map((t) => {
                const y = ATAS + tinggiPlot - t * tinggiPlot;
                return (
                  <g key={t}>
                    <line x1={KIRI} x2={LEBAR} y1={y} y2={y} stroke="#e1e7ef" strokeDasharray={t === 0 ? undefined : "3 4"} />
                    <text x={KIRI - 6} y={y + 3.5} textAnchor="end" fontSize="10" fill="#64748b">
                      {format(atas * t)}
                    </text>
                  </g>
                );
              })}
              {labelX.map((i, k) => (
                <text key={`${i}-${k}`} x={x(i)} y={TINGGI - 6} textAnchor={k === 0 ? "start" : k === labelX.length - 1 ? "end" : "middle"} fontSize="10" fill="#64748b">
                  {waktuSingkatWIB(waktu[i], denganTanggal)}
                </text>
              ))}
              <g transform={`translate(0 ${ATAS})`}>
                {deret.map((d) => (
                  <path key={d.nama} d={jalurGaris(d.nilai, LEBAR, tinggiPlot, atas, KIRI)} fill="none" stroke={d.warna} strokeWidth="1.8" strokeLinejoin="round" strokeLinecap="round" />
                ))}
                {sorot !== null && (
                  <g>
                    <line x1={x(sorot)} x2={x(sorot)} y1={0} y2={tinggiPlot} stroke="#94a3b8" strokeDasharray="3 3" />
                    {deret.map((d) => {
                      const v = d.nilai[sorot];
                      if (v === null || v === undefined) return null;
                      return <circle key={d.nama} cx={x(sorot)} cy={tinggiPlot - (Math.min(Math.max(v, 0), atas) / atas) * tinggiPlot} r="3.5" fill="#fff" stroke={d.warna} strokeWidth="2" />;
                    })}
                  </g>
                )}
              </g>
            </svg>
            {sorot !== null && (
              <div
                role="status"
                className="pointer-events-none absolute top-1 z-10 min-w-[9rem] rounded-lg bg-slate-900/95 px-2.5 py-2 text-xs text-white shadow-lg"
                style={{ left: `${Math.min(Math.max((x(sorot) / LEBAR) * 100, 12), 88)}%`, transform: "translateX(-50%)" }}
              >
                <p className="font-medium text-slate-200">{waktuWIB(waktu[sorot], false)} WIB</p>
                {deret.map((d) => (
                  <p key={d.nama} className="mt-0.5 flex items-center justify-between gap-3">
                    <span className="flex items-center gap-1.5">
                      <span className="inline-block h-2 w-2 rounded-full" style={{ background: d.warna }} aria-hidden="true" />
                      {d.nama}
                    </span>
                    <span className="font-semibold tabular-nums">{d.nilai[sorot] === null || d.nilai[sorot] === undefined ? "-" : format(d.nilai[sorot] as number)}</span>
                  </p>
                ))}
              </div>
            )}
          </div>
          <ul className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-600">
            {deret.map((d) => (
              <li key={d.nama} className="flex items-center gap-1.5">
                <span className="inline-block h-0.5 w-4 rounded" style={{ background: d.warna }} aria-hidden="true" />
                {d.nama}
              </li>
            ))}
          </ul>
        </div>
      )}
    </ChartCard>
  );
}
