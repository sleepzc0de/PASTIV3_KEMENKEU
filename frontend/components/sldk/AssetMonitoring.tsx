"use client";

import { ReactNode, useState } from "react";
import { Ban, CalendarX2, CheckCircle2, ChevronRight, Coins, Hourglass, SearchX, TriangleAlert, Search } from "lucide-react";
import { SLDKReferences } from "@/lib/api";
import { formatNumber } from "./asset";
import { OverviewGate, AvailableOverview } from "./OverviewGate";
import { SearchPreset, kodeRusakBerat, pct, share } from "./overview";
import { OverviewState } from "./useOverview";

interface Props {
  overview: OverviewState;
  refs: SLDKReferences | null;
  isAdmin: boolean;
  onSearch: (preset: Omit<SearchPreset, "nonce">) => void;
}

export function AssetMonitoring({ overview, refs, isAdmin, onSearch }: Props) {
  return (
    <OverviewGate overview={overview} isAdmin={isAdmin}>
      {(d) => <MonitoringBody d={d} refs={refs} onSearch={onSearch} />}
    </OverviewGate>
  );
}

const RULE_ICON: Record<string, ReactNode> = {
  idle: <Hourglass className="h-4 w-4" />,
  hilang: <SearchX className="h-4 w-4" />,
  dihentikan: <Ban className="h-4 w-4" />,
  nilai_nol: <Coins className="h-4 w-4" />,
  dq_tanggal: <CalendarX2 className="h-4 w-4" />,
};

interface SatkerRow {
  id: string;
  nama: string;
  title: string;
  jumlah: number;
  onOpen: () => void;
}

function MonitoringBody({ d, refs, onSearch }: { d: AvailableOverview; refs: SLDKReferences | null; onSearch: Props["onSearch"] }) {
  const satkerOf = (id: string) => {
    const s = d.satker[id];
    return s?.nama ? { nama: s.nama, title: `${s.kode} · ${s.nama}`, numeric: s.id } : { nama: `Satker ${id}`, title: `Satker ${id}`, numeric: Number(id) };
  };

  const rusakBerat = kodeRusakBerat(refs?.kondisi);
  const rusakBeratCount = rusakBerat ? d.kondisi.find((g) => g.k1 === rusakBerat)?.jumlah ?? 0 : null;

  return (
    <>
      <p className="text-sm text-slate-600">
        Temuan yang perlu ditindaklanjuti, dihitung dari data SLDK pada sinkronisasi terakhir. Pilih satuan kerja untuk melihat daftar asetnya di tab
        Pencarian.
      </p>
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {d.anomali.map((a) => {
          const rows: SatkerRow[] = a.satker_teratas.map((s) => {
            const info = satkerOf(s.id);
            return {
              id: s.id,
              nama: info.nama,
              title: info.title,
              jumlah: s.jumlah,
              onOpen: () => onSearch({ anomali: a.key, satker: { id: info.numeric, kode: d.satker[s.id]?.kode ?? "", nama: info.nama } }),
            };
          });
          return (
            <MonitorCard
              key={a.key}
              icon={RULE_ICON[a.key] ?? <TriangleAlert className="h-4 w-4" />}
              title={a.label}
              description={a.keterangan}
              count={a.jumlah}
              total={d.total.jumlah}
              rows={rows}
              onOpenAll={() => onSearch({ anomali: a.key })}
            />
          );
        })}
        {rusakBeratCount !== null && rusakBerat && (
          <MonitorCard
            icon={<TriangleAlert className="h-4 w-4" />}
            title="Rusak berat"
            description="Aset dengan kondisi Rusak Berat; kandidat penghapusan atau perbaikan."
            count={rusakBeratCount}
            total={d.total.jumlah}
            rows={[]}
            onOpenAll={() => onSearch({ kdKondisi: rusakBerat })}
          />
        )}
      </div>
    </>
  );
}

function MonitorCard({
  icon,
  title,
  description,
  count,
  total,
  rows,
  onOpenAll,
}: {
  icon: ReactNode;
  title: string;
  description: string;
  count: number;
  total: number;
  rows: SatkerRow[];
  onOpenAll: () => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const shown = expanded ? rows : rows.slice(0, 5);

  return (
    <article className="flex min-w-0 flex-col gap-3 rounded-xl border border-slate-200 p-4">
      <div className="flex items-center gap-2">
        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-slate-100 text-slate-500">{icon}</span>
        <h3 className="min-w-0 text-sm font-semibold text-slate-900">{title}</h3>
      </div>
      <p className="text-xs text-slate-500">{description}</p>

      {count === 0 ? (
        <p className="inline-flex items-center gap-1.5 text-sm font-medium text-emerald-700">
          <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
          Tidak ada temuan
        </p>
      ) : (
        <p>
          <span className="text-3xl font-semibold text-slate-900">{formatNumber(count, 0)}</span>{" "}
          <span className="text-sm text-slate-500">aset · {pct(share(count, total))} dari total</span>
        </p>
      )}

      {shown.length > 0 && (
        <div className="space-y-1">
          <p className="text-xs font-medium text-slate-600">Satuan kerja terbanyak</p>
          <ul className="divide-y divide-slate-100">
            {shown.map((r) => (
              <li key={r.id}>
                <button
                  type="button"
                  onClick={r.onOpen}
                  title={r.title}
                  className="group flex w-full items-center gap-2 rounded-md py-1.5 text-left text-xs hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500"
                >
                  <span className="min-w-0 flex-1 truncate text-slate-700">{r.nama}</span>
                  <span className="shrink-0 font-medium text-slate-900">{formatNumber(r.jumlah, 0)}</span>
                  <ChevronRight className="h-3.5 w-3.5 shrink-0 text-slate-300 group-hover:text-slate-500" aria-hidden="true" />
                </button>
              </li>
            ))}
          </ul>
          {rows.length > 5 && (
            <button type="button" onClick={() => setExpanded((v) => !v)} className="text-xs font-medium text-blue-600 hover:text-blue-700">
              {expanded ? "Tampilkan lebih sedikit" : `Tampilkan ${rows.length} satuan kerja`}
            </button>
          )}
        </div>
      )}

      {count > 0 && (
        <div className="mt-auto space-y-1 pt-1">
          <button
            type="button"
            onClick={onOpenAll}
            className="inline-flex items-center gap-1.5 rounded-lg border border-slate-300 px-3 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-50"
          >
            <Search className="h-3.5 w-3.5" />
            Cari contoh aset
          </button>
          <p className="text-xs text-slate-400">Tanpa satuan kerja, pencarian di seluruh tabel bisa lambat.</p>
        </div>
      )}
    </article>
  );
}
