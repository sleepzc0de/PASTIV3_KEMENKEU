"use client";

import { ReactNode, useState } from "react";
import { AlertTriangle, BarChart3, CheckCircle2, CircleHelp, Table2, XCircle } from "lucide-react";
import { KondisiPart, Metric, METRIC_LABEL, formatMetric, pct, share } from "@/lib/dasbor";

// Grafik ringan tanpa pustaka: batang horizontal dan batang bertumpuk (satu batang). Satu seri = satu warna
// (biru merek); status (baik/rusak) memakai warna status dengan ikon dan label.

const BAR = "#3358e0"; // blue-600 pada tailwind.config.ts (biru merek), selaras dengan tombol dan tautan aplikasi

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
