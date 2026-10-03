"use client";

import { useState } from "react";
import { SLDKReferences } from "@/lib/api";
import { formatCurrency } from "@/lib/format";
import { BarItem, BarList, ChartCard, ConditionBar, DataTable, StatTile, TrendChart } from "./charts";
import { formatNumber } from "./asset";
import { OverviewGate, AvailableOverview } from "./OverviewGate";
import {
  Group,
  METRIC_LABEL,
  Metric,
  compactRupiah,
  formatMetric,
  kondisiParts,
  pct,
  refLabel,
  share,
  sortByMetric,
  topWithOther,
  yearPoints,
} from "./overview";
import { OverviewState } from "./useOverview";

interface Props {
  overview: OverviewState;
  refs: SLDKReferences | null;
  isAdmin: boolean;
}

export function AssetOverview({ overview, refs, isAdmin }: Props) {
  return (
    <OverviewGate overview={overview} isAdmin={isAdmin}>
      {(d) => <OverviewBody d={d} refs={refs} />}
    </OverviewGate>
  );
}

const METRICS: Metric[] = ["nilai_buku", "nilai_perolehan", "jumlah"];

function toBars(groups: Group[], metric: Metric, base: number, label: (k1: string | null) => { label: string; title?: string }): BarItem[] {
  return groups.map((g, i) => {
    const l = g.other ? { label: "Lainnya" } : label(g.k1);
    return {
      key: `${g.k1 ?? "kosong"}-${i}`,
      label: l.label,
      title: l.title,
      value: g[metric],
      display: formatMetric(metric, g[metric]),
      note: pct(share(g[metric], base)),
      muted: g.other,
    };
  });
}

// Padanan tabel: nilai lengkap (bukan ringkas) untuk semua ukuran sekaligus.
function groupTable(first: string, groups: Group[], label: (k1: string | null) => string) {
  return (
    <DataTable
      columns={[first, "Jumlah", "Nilai perolehan", "Nilai buku", "Penyusutan"]}
      rows={groups.map((g) => [label(g.k1), formatNumber(g.jumlah, 0), formatCurrency(g.nilai_perolehan), formatCurrency(g.nilai_buku), formatCurrency(g.nilai_susut)])}
    />
  );
}

function OverviewBody({ d, refs }: { d: AvailableOverview; refs: SLDKReferences | null }) {
  const [metric, setMetric] = useState<Metric>("nilai_buku");
  const { total } = d;
  const base = total[metric];

  const satkerName = (id: string | null) => {
    if (id === null) return { label: "(kosong)" };
    const s = d.satker[id];
    return s?.nama ? { label: s.nama, title: `${s.kode} · ${s.nama}` } : { label: `Satker ${id}` };
  };

  const jenis = (k1: string | null) => ({ label: refLabel(refs?.jenis_bmn, k1) });
  const status = (k1: string | null) => ({ label: refLabel(refs?.status_penggunaan, k1) });
  const prov = (k1: string | null) => ({ label: k1 || "(kosong)" });

  const jenisBars = toBars(topWithOther(d.jenis, 8, metric), metric, base, jenis);
  const statusBars = toBars(topWithOther(d.status, 8, metric), metric, base, status);
  const provBars = toBars(sortByMetric(d.provinsi, metric).slice(0, 10), metric, base, prov);
  const satkerBars = toBars(sortByMetric(d.satker_teratas, metric), metric, base, satkerName);
  const trend = yearPoints(d.tahun, metric);

  return (
    <>
      <div className="grid grid-cols-1 gap-3 min-[480px]:grid-cols-2 lg:grid-cols-4">
        <StatTile label="Jumlah aset" value={formatNumber(total.jumlah, 0)} sub={d.definisi.terkonfirmasi ? "aset aktif" : "semua baris tabel aset"} />
        <StatTile label="Nilai perolehan" value={compactRupiah(total.nilai_perolehan)} sub={formatCurrency(total.nilai_perolehan)} />
        <StatTile label="Nilai buku" value={compactRupiah(total.nilai_buku)} sub={`${pct(share(total.nilai_buku, total.nilai_perolehan))} dari nilai perolehan`} />
        <StatTile label="Akumulasi penyusutan" value={compactRupiah(total.nilai_susut)} sub={`${pct(share(total.nilai_susut, total.nilai_perolehan))} dari nilai perolehan`} />
      </div>

      <div className="flex flex-wrap items-center gap-x-3 gap-y-2" role="group" aria-label="Ukuran yang ditampilkan">
        <span className="text-xs font-medium text-slate-600">Tampilkan menurut</span>
        <div className="inline-flex rounded-lg border border-slate-200 bg-slate-50 p-0.5 text-sm">
          {METRICS.map((m) => (
            <button
              key={m}
              type="button"
              onClick={() => setMetric(m)}
              aria-pressed={metric === m}
              className={`rounded-md px-3 py-1 text-xs font-medium sm:text-sm ${metric === m ? "bg-white text-blue-700 shadow-sm" : "text-slate-500 hover:text-slate-700"}`}
            >
              {METRIC_LABEL[m]}
            </button>
          ))}
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <ChartCard
          title="Jenis BMN"
          subtitle={`${d.jenis.length} jenis, menurut ${METRIC_LABEL[metric].toLowerCase()}`}
          table={groupTable("Jenis BMN", sortByMetric(d.jenis, metric), (k) => jenis(k).label)}
        >
          <BarList items={jenisBars} />
        </ChartCard>

        <ChartCard
          title="Kondisi aset"
          subtitle={`Komposisi menurut ${METRIC_LABEL[metric].toLowerCase()}`}
          table={groupTable("Kondisi", d.kondisi, (k) => refLabel(refs?.kondisi, k))}
        >
          <ConditionBar parts={kondisiParts(d.kondisi, refs?.kondisi, metric)} metric={metric} />
        </ChartCard>

        <ChartCard
          title="Status penggunaan"
          subtitle={`Menurut ${METRIC_LABEL[metric].toLowerCase()}`}
          table={groupTable("Status penggunaan", sortByMetric(d.status, metric), (k) => status(k).label)}
        >
          <BarList items={statusBars} />
        </ChartCard>

        <ChartCard
          title="Provinsi"
          subtitle="10 teratas dari 20 provinsi dengan nilai buku terbesar"
          table={groupTable("Provinsi", sortByMetric(d.provinsi, metric), (k) => prov(k).label)}
        >
          <BarList items={provBars} />
        </ChartCard>
      </div>

      <ChartCard
        title="Satuan kerja"
        subtitle="10 satuan kerja dengan nilai buku terbesar"
        table={groupTable("Satuan kerja", sortByMetric(d.satker_teratas, metric), (k) => satkerName(k).label)}
      >
        <BarList items={satkerBars} wide />
      </ChartCard>

      <ChartCard
        title="Tren per tahun perolehan"
        subtitle={`${METRIC_LABEL[metric]}${metric === "jumlah" ? "" : " (Rp)"} menurut tahun perolehan`}
        table={groupTable("Tahun", d.tahun.filter((g) => g.k1 !== null).sort((a, b) => (a.k1 ?? "").localeCompare(b.k1 ?? "")), (k) => k ?? "")}
      >
        <TrendChart points={trend} metric={metric} />
      </ChartCard>
    </>
  );
}
