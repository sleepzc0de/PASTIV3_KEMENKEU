"use client";

import { ReactNode, useEffect, useRef, useState } from "react";
import { AlertTriangle, BarChart3, CheckCircle2, CircleHelp, Table2, XCircle } from "lucide-react";
import { KondisiPart, Metric, METRIC_LABEL, Point, axisLabel, formatMetric, niceMax, pct, share } from "./overview";

// Grafik ringan tanpa pustaka: batang horizontal, batang bertumpuk (satu batang), dan garis. Satu seri = satu warna
// (biru merek); status (baik/rusak) memakai warna status dengan ikon dan label.

const BAR = "#3358e0"; // blue-600 pada tailwind.config.ts (biru merek), selaras dengan tombol dan tautan aplikasi
const GRID = "#e2e8f0"; // slate-200, hairline
const AXIS = "#cbd5e1"; // slate-300
const MUTED = "#64748b"; // slate-500, teks sumbu
const SURFACE = "#ffffff";

const STATUS = { good: "#0ca30c", warning: "#fab219", critical: "#d03b3b", other: "#cbd5e1" };

export function StatTile({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className="min-w-0 rounded-2xl border border-slate-200/80 bg-gradient-to-br from-white to-slate-50 p-4 shadow-sm transition-shadow hover:shadow-md">
      <p className="text-xs font-medium uppercase tracking-wide text-slate-500">{label}</p>
      <p className="mt-1.5 text-2xl font-bold tracking-tight text-slate-900 tabular-nums">{value}</p>
      {sub && <p className="mt-0.5 break-words text-xs text-slate-500">{sub}</p>}
    </div>
  );
}

// Kartu grafik. Setiap grafik punya padanan tabel (tombol Tabel) supaya nilainya bisa dibaca tanpa melihat batang.
export function ChartCard({ title, subtitle, table, children }: { title: string; subtitle?: string; table?: ReactNode; children: ReactNode }) {
  const [asTable, setAsTable] = useState(false);
  return (
    <section className="min-w-0 rounded-2xl border border-slate-200/80 bg-white p-4 shadow-sm sm:p-5">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-slate-900">{title}</h3>
          {subtitle && <p className="mt-0.5 text-xs text-slate-500">{subtitle}</p>}
        </div>
        {table && (
          <button
            type="button"
            onClick={() => setAsTable((v) => !v)}
            aria-pressed={asTable}
            className="inline-flex shrink-0 items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-slate-500 hover:bg-slate-100 hover:text-slate-700"
          >
            {asTable ? <BarChart3 className="h-3.5 w-3.5" /> : <Table2 className="h-3.5 w-3.5" />}
            {asTable ? "Grafik" : "Tabel"}
          </button>
        )}
      </div>
      <div className="mt-4">{asTable && table ? table : children}</div>
    </section>
  );
}

export function DataTable({ columns, rows }: { columns: string[]; rows: string[][] }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-max text-xs">
        <thead>
          <tr className="border-b border-slate-200 text-slate-500">
            {columns.map((c, i) => (
              <th key={c} scope="col" className={`py-1.5 font-medium ${i === 0 ? "pr-3 text-left" : "px-3 text-right"}`}>
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100">
          {rows.map((r, ri) => (
            <tr key={ri}>
              {r.map((cell, i) =>
                i === 0 ? (
                  <th key={i} scope="row" className="max-w-[16rem] py-1.5 pr-3 text-left font-normal text-slate-700">
                    {cell}
                  </th>
                ) : (
                  <td key={i} className="px-3 py-1.5 text-right tabular-nums text-slate-700">
                    {cell}
                  </td>
                )
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export interface BarItem {
  key: string;
  label: string;
  title?: string; // teks lengkap untuk tooltip bila label terpotong
  value: number;
  display: string;
  note?: string; // mis. persentase
  muted?: boolean; // baris gabungan ("Lainnya")
}

// Batang horizontal: label di kiri (di atas batang pada layar sempit), nilai di ujung batang. Label dan nilai
// tidak pernah di dalam batang, jadi tidak ada yang terpotong. `wide` memberi kolom label lebih lebar dan boleh dua
// baris, untuk nama panjang seperti satuan kerja.
export function BarList({ items, wide = false }: { items: BarItem[]; wide?: boolean }) {
  const max = Math.max(0, ...items.map((i) => i.value));
  if (items.length === 0 || max <= 0) {
    return <p className="py-6 text-center text-sm text-slate-400">Tidak ada data</p>;
  }
  return (
    <ul className="space-y-2.5">
      {items.map((it) => {
        const width = it.value > 0 ? Math.max((it.value / max) * 100, 0.6) : 0;
        return (
          <li
            key={it.key}
            className={`grid grid-cols-1 gap-y-1 sm:items-center sm:gap-x-3 ${wide ? "sm:grid-cols-[minmax(0,17rem)_minmax(0,1fr)]" : "sm:grid-cols-[minmax(0,11rem)_minmax(0,1fr)]"}`}
          >
            <span className={`text-xs text-slate-700 ${wide ? "line-clamp-2 break-words" : "truncate"}`} title={it.title ?? it.label}>
              {it.label}
            </span>
            <div className="flex items-center gap-3">
              <div className="h-3 min-w-0 flex-1 border-l border-slate-300" aria-hidden="true">
                <div
                  className="h-full origin-left animate-grow-x rounded-r-[4px]"
                  style={{ width: `${width}%`, backgroundColor: it.muted ? STATUS.other : BAR }}
                />
              </div>
              <span className="w-28 shrink-0 text-right text-xs font-medium text-slate-900">
                {it.display}
                {it.note && (
                  <>
                    {" "}
                    <span className="ml-0.5 font-normal text-slate-400">{it.note}</span>
                  </>
                )}
              </span>
            </div>
          </li>
        );
      })}
    </ul>
  );
}

const toneStyle = {
  baik: { color: STATUS.good, Icon: CheckCircle2 },
  ringan: { color: STATUS.warning, Icon: AlertTriangle },
  berat: { color: STATUS.critical, Icon: XCircle },
  lain: { color: STATUS.other, Icon: CircleHelp },
} as const;

// Satu batang bertumpuk untuk bagian-dari-keseluruhan (kondisi aset), dengan daftar ikon + label + nilai di bawahnya.
export function ConditionBar({ parts, metric }: { parts: KondisiPart[]; metric: Metric }) {
  const visible = parts.filter((p) => p.value > 0);
  const total = visible.reduce((acc, p) => acc + p.value, 0);
  if (total <= 0) return <p className="py-6 text-center text-sm text-slate-400">Tidak ada data</p>;
  return (
    <div className="space-y-4">
      <div className="flex h-3 gap-[2px]" role="img" aria-label={`Komposisi kondisi aset menurut ${METRIC_LABEL[metric].toLowerCase()}`}>
        {visible.map((p, i) => (
          <div
            key={p.key || "kosong"}
            className={`min-w-[2px] ${i === 0 ? "rounded-l-[4px]" : ""} ${i === visible.length - 1 ? "rounded-r-[4px]" : ""}`}
            style={{ flexGrow: p.value, flexBasis: 0, backgroundColor: toneStyle[p.tone].color }}
          />
        ))}
      </div>
      <ul className="space-y-2">
        {visible.map((p) => {
          const { color, Icon } = toneStyle[p.tone];
          return (
            <li key={p.key || "kosong"} className="flex items-center gap-2 text-xs">
              <Icon className="h-4 w-4 shrink-0" style={{ color }} aria-hidden="true" />
              <span className="min-w-0 flex-1 truncate text-slate-700">{p.label}</span>
              <span className="shrink-0 font-medium text-slate-900">{formatMetric(metric, p.value)}</span>
              <span className="w-12 shrink-0 text-right text-slate-400">{pct(share(p.value, total))}</span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function useElementWidth<T extends HTMLElement>() {
  const ref = useRef<T>(null);
  const [width, setWidth] = useState(0);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    setWidth(Math.floor(el.getBoundingClientRect().width));
    if (typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver((entries) => setWidth(Math.floor(entries[0].contentRect.width)));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  return [ref, width] as const;
}

const HEIGHT = 220;
const MARGIN = { top: 20, right: 16, bottom: 26, left: 44 };

// Garis tren per tahun: garis 2px, area 10%, titik akhir 8px dengan cincin 2px; hanya nilai terakhir dan
// puncak yang diberi label. Gerakkan penunjuk (atau sentuh) untuk membaca nilai tiap tahun.
export function TrendChart({ points, metric }: { points: Point[]; metric: Metric }) {
  const [ref, width] = useElementWidth<HTMLDivElement>();
  const [hover, setHover] = useState<number | null>(null);

  const content = (() => {
    if (points.length === 0) return <p className="py-10 text-center text-sm text-slate-400">Tidak ada data tahun perolehan</p>;
    if (width === 0) return null;

    const plotW = Math.max(width - MARGIN.left - MARGIN.right, 10);
    const plotH = HEIGHT - MARGIN.top - MARGIN.bottom;
    const minX = points[0].x;
    const maxX = points[points.length - 1].x;
    const yMax = niceMax(Math.max(...points.map((p) => p.y)));
    const sx = (x: number) => MARGIN.left + (maxX === minX ? plotW / 2 : ((x - minX) / (maxX - minX)) * plotW);
    const sy = (y: number) => MARGIN.top + plotH - (Math.max(y, 0) / yMax) * plotH;

    const line = points.map((p, i) => `${i === 0 ? "M" : "L"}${sx(p.x).toFixed(1)} ${sy(p.y).toFixed(1)}`).join(" ");
    const base = MARGIN.top + plotH;
    const area = `${line} L${sx(maxX).toFixed(1)} ${base} L${sx(minX).toFixed(1)} ${base} Z`;

    const yTicks = [0, 1, 2, 3, 4].map((i) => (yMax * i) / 4);
    const xCount = Math.max(2, Math.min(7, Math.floor(plotW / 64)));
    const xTicks = Array.from(new Set(Array.from({ length: xCount }, (_, i) => Math.round(minX + ((maxX - minX) * i) / (xCount - 1)))));

    const last = points[points.length - 1];
    const peak = points.reduce((a, b) => (b.y > a.y ? b : a));
    const showPeak = peak !== last && Math.abs(sx(peak.x) - sx(last.x)) > 90;
    const active = hover !== null ? points[hover] : null;

    const onMove = (clientX: number, rect: DOMRect) => {
      const x = clientX - rect.left;
      let best = 0;
      let bestD = Infinity;
      points.forEach((p, i) => {
        const d = Math.abs(sx(p.x) - x);
        if (d < bestD) {
          bestD = d;
          best = i;
        }
      });
      setHover(best);
    };

    const tipLeft = active ? Math.min(Math.max(sx(active.x), 70), width - 70) : 0;

    return (
      <div className="relative">
        <svg
          width={width}
          height={HEIGHT}
          role="img"
          aria-label={`Grafik garis ${METRIC_LABEL[metric].toLowerCase()} per tahun perolehan, ${minX} sampai ${maxX}`}
          style={{ touchAction: "pan-y" }}
          onPointerMove={(e) => onMove(e.clientX, e.currentTarget.getBoundingClientRect())}
          onPointerLeave={() => setHover(null)}
        >
          {yTicks.map((v) => (
            <g key={v}>
              <line x1={MARGIN.left} x2={width - MARGIN.right} y1={sy(v)} y2={sy(v)} stroke={v === 0 ? AXIS : GRID} strokeWidth={1} />
              <text x={MARGIN.left - 8} y={sy(v)} textAnchor="end" dominantBaseline="middle" fontSize={11} fill={MUTED}>
                {axisLabel(v)}
              </text>
            </g>
          ))}
          {xTicks.map((x) => (
            <text key={x} x={sx(x)} y={HEIGHT - 6} textAnchor={x === minX && xTicks.length > 1 ? "start" : x === maxX ? "end" : "middle"} fontSize={11} fill={MUTED}>
              {x}
            </text>
          ))}
          <path d={area} fill={BAR} fillOpacity={0.1} />
          <path d={line} fill="none" stroke={BAR} strokeWidth={2} strokeLinejoin="round" strokeLinecap="round" />

          {showPeak && (
            <>
              <circle cx={sx(peak.x)} cy={sy(peak.y)} r={4} fill={BAR} stroke={SURFACE} strokeWidth={2} />
              <text x={sx(peak.x)} y={sy(peak.y) - 10} textAnchor="middle" fontSize={11} fill="#334155">
                {peak.x} · {axisLabel(peak.y)}
              </text>
            </>
          )}
          <circle cx={sx(last.x)} cy={sy(last.y)} r={4} fill={BAR} stroke={SURFACE} strokeWidth={2} />
          <text x={Math.min(sx(last.x), width - MARGIN.right)} y={sy(last.y) - 10} textAnchor="end" fontSize={11} fill="#334155">
            {last.x} · {axisLabel(last.y)}
          </text>

          {active && (
            <>
              <line x1={sx(active.x)} x2={sx(active.x)} y1={MARGIN.top} y2={base} stroke={AXIS} strokeWidth={1} />
              <circle cx={sx(active.x)} cy={sy(active.y)} r={5} fill={BAR} stroke={SURFACE} strokeWidth={2} />
            </>
          )}
        </svg>
        {active && (
          <div
            className="pointer-events-none absolute top-0 -translate-x-1/2 rounded-md border border-slate-200 bg-white px-2 py-1 text-xs shadow-sm"
            style={{ left: tipLeft }}
          >
            <span className="font-semibold text-slate-900">{active.x}</span>
            <span className="ml-2 text-slate-600">{formatMetric(metric, active.y)}</span>
          </div>
        )}
      </div>
    );
  })();

  return (
    <div ref={ref} className="w-full">
      {content}
    </div>
  );
}
