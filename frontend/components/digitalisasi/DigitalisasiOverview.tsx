"use client";

import { ReactNode, useState } from "react";
import { DatabaseZap, Loader2, RefreshCw } from "lucide-react";
import { DGDatasetKey, DGOverview, DGOverviewData } from "@/lib/api";
import { formatDate } from "@/lib/format";
import { Alert } from "@/components/ui/Alert";
import { formatNumber, kondisiTone } from "@/lib/dasbor";
import { BarItem, BarList, ChartCard, ConditionBar, DataTable, StatTile } from "@/components/ui/charts";
import { KondisiPart, compactRupiah, pct } from "@/lib/dasbor";
import { Segmented } from "./controls";
import {
  ASSET_KEYS,
  BREAKDOWN_LABEL,
  BreakdownMetric,
  DATASET_LABEL,
  DATASET_SHORT,
  NILAI_KEYS,
  asuransiLabel,
  compactLuas,
  dataPer,
  formatBreakdown,
  mergeCounts,
  perJumlah,
  provinsiValue,
  ratio,
  shortStatusHukum,
  ue1Value,
} from "./digitalisasi";
import { AsyncState } from "./useDigitalisasi";

interface Props {
  overview: AsyncState<DGOverview>;
  isAdmin: boolean;
  onGoSync: () => void;
  onOpenData: (p: { dataset: DGDatasetKey; tanpaKoordinat?: boolean }) => void;
}

export function DigitalisasiOverview({ overview, isAdmin, onGoSync, onOpenData }: Props) {
  const { data, isLoading, error, reload } = overview;

  if (!data) {
    if (error) {
      return (
        <div className="space-y-3">
          <Alert message={error} />
          <button type="button" onClick={reload} className="text-sm font-medium text-blue-600 hover:text-blue-700">
            Coba lagi
          </button>
        </div>
      );
    }
    return <Skeleton />;
  }

  if (!data.tersedia) {
    return (
      <div className="flex flex-col items-center gap-3 rounded-lg border border-dashed border-slate-300 px-4 py-12 text-center">
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-blue-50 text-blue-500">
          <DatabaseZap className="h-6 w-6" />
        </div>
        <p className="text-sm font-medium text-slate-700">Belum ada data digitalisasi aset</p>
        <p className="max-w-md text-sm text-slate-500">
          Data tanah, gedung, rusunara, rumah negara, mess, dan satuan kerja disalin dari SLDK lewat sinkronisasi. Setelah sinkronisasi pertama selesai,
          ringkasan, peta, dan daftar aset muncul di sini.
        </p>
        {isAdmin ? (
          <button type="button" onClick={onGoSync} className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700">
            Buka Sinkronisasi
          </button>
        ) : (
          <p className="text-xs text-slate-400">Hanya superadmin yang dapat menjalankan sinkronisasi.</p>
        )}
      </div>
    );
  }

  const last = dataPer(data.sinkron);
  const belum = data.sinkron.filter((s) => !s.terakhir_sukses).length;

  return (
    <div className="space-y-4" aria-busy={isLoading}>
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <p className="text-xs text-slate-500">
          Data disalin dari SLDK, terakhir <span className="font-medium text-slate-700">{formatDate(last)}</span>
          {belum > 0 && <span className="text-amber-700"> · {belum} dari {data.sinkron.length} dataset belum pernah disinkronkan</span>}
        </p>
        <button
          type="button"
          onClick={reload}
          disabled={isLoading}
          className="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium text-slate-600 hover:bg-slate-100 disabled:opacity-60"
        >
          {isLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
          Muat ulang
        </button>
      </div>
      {error && <Alert message={`${error}. Menampilkan data yang dimuat sebelumnya.`} />}
      <div className={`space-y-4 transition-opacity ${isLoading ? "opacity-60" : ""}`}>
        <RingkasanAset d={data} onOpenData={onOpenData} />
      </div>
    </div>
  );
}

// Isi ringkasan (angka pokok, grafik, kelengkapan data). Dipakai tab Ringkasan Digitalisasi Aset dan Dashboard Aset.
export function RingkasanAset({ d, onOpenData }: { d: DGOverviewData; onOpenData: Props["onOpenData"] }) {
  const stat = (k: DGDatasetKey) => d.aset.find((a) => a.key === k);
  const sum = (keys: DGDatasetKey[], f: (a: NonNullable<ReturnType<typeof stat>>) => number) => keys.reduce((acc, k) => acc + (stat(k) ? f(stat(k)!) : 0), 0);

  const gedung = sum(["gedung_kantor_utama", "gedung_lainnya"], (a) => a.jumlah);
  const hunian = sum(["rusunara", "rumah_negara", "mess_rumah_negara"], (a) => a.jumlah);
  const kamar =
    (d.hunian.rusun_kamar_e ?? 0) + (d.hunian.rusun_kamar_d ?? 0) + (d.hunian.rusun_kamar_c ?? 0) + (d.hunian.mess_kamar_e ?? 0) + (d.hunian.mess_kamar_d ?? 0);
  const nilai = sum(NILAI_KEYS, (a) => a.nilai);
  const tanah = stat("tanah");

  return (
    <>
      <div className="grid grid-cols-1 gap-3 min-[480px]:grid-cols-2 lg:grid-cols-3">
        <StatTile label="Satuan kerja" value={formatNumber(d.satker.total, 0)} sub={`${formatNumber(d.satker.induk, 0)} induk · ${formatNumber(d.satker.anak, 0)} anak`} />
        <StatTile label="Tanah" value={formatNumber(tanah?.jumlah ?? 0, 0)} sub={`Luas total ${compactLuas(tanah?.luas ?? 0)}`} />
        <StatTile
          label="Gedung"
          value={formatNumber(gedung, 0)}
          sub={`${formatNumber(stat("gedung_kantor_utama")?.jumlah ?? 0, 0)} kantor utama · ${formatNumber(stat("gedung_lainnya")?.jumlah ?? 0, 0)} lainnya`}
        />
        <StatTile label="Hunian" value={formatNumber(hunian, 0)} sub={`Rusunara, rumah negara, mess · ${formatNumber(kamar, 0)} kamar tidur`} />
        <StatTile label="Nilai aset" value={compactRupiah(nilai)} sub="Tanah, gedung, rusunara, mess" />
        <StatTile
          label="Kendaraan dinas"
          value={formatNumber(d.satker.kdj + d.satker.kdo, 0)}
          sub={`${formatNumber(d.satker.kdj, 0)} KDJ · ${formatNumber(d.satker.kdo, 0)} KDO`}
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <JenisChart d={d} />
        <KondisiChart d={d} />
      </div>

      <UE1Chart d={d} />
      <ProvinsiChart d={d} />

      <div className="grid gap-4 lg:grid-cols-2">
        <StatusHukumChart d={d} />
        <AsuransiChart d={d} />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <KamarChart d={d} />
        <PenghuniChart d={d} />
      </div>

      <Kelengkapan d={d} onOpenData={onOpenData} />
    </>
  );
}

// ------------------------------------------------------------------ grafik

function JenisChart({ d }: { d: DGOverviewData }) {
  const items: BarItem[] = [...d.aset]
    .sort((a, b) => b.jumlah - a.jumlah)
    .map((a) => ({ key: a.key, label: DATASET_LABEL[a.key], value: a.jumlah, display: formatNumber(a.jumlah, 0) }));
  return (
    <ChartCard
      title="Aset menurut jenis"
      subtitle="Jumlah aset pada tiap dataset"
      table={
        <DataTable
          columns={["Jenis", "Jumlah", "Luas (m²)", "Nilai"]}
          rows={d.aset.map((a) => [
            DATASET_LABEL[a.key],
            formatNumber(a.jumlah, 0),
            a.punya_luas ? formatNumber(a.luas, 2) : "-",
            a.punya_nilai ? `Rp ${formatNumber(a.nilai, 0)}` : "-",
          ])}
        />
      }
    >
      <BarList items={items} />
    </ChartCard>
  );
}

function KondisiChart({ d }: { d: DGOverviewData }) {
  const keys = ASSET_KEYS.filter((k) => (d.kondisi[k]?.length ?? 0) > 0);
  const [key, setKey] = useState<DGDatasetKey>(keys[0] ?? "tanah");
  const list = d.kondisi[key] ?? [];
  const rank = (k: string | null) => (k === null ? 99 : /baik/i.test(k) ? 0 : /ringan/i.test(k) ? 1 : /berat/i.test(k) ? 2 : 3);
  const parts: KondisiPart[] = [...list]
    .sort((a, b) => rank(a.k) - rank(b.k) || b.jumlah - a.jumlah)
    .map((c) => ({ key: c.k ?? "", label: c.k ?? "(kosong)", tone: kondisiTone(c.k ?? ""), value: c.jumlah }));
  return (
    <ChartCard
      title="Kondisi aset"
      subtitle="Komposisi menurut jumlah"
      table={<DataTable columns={["Kondisi", "Jumlah"]} rows={list.map((c) => [c.k ?? "(kosong)", formatNumber(c.jumlah, 0)])} />}
    >
      <div className="mb-4">
        <Segmented
          label="Jenis aset"
          value={key}
          options={keys.map((k) => ({ value: k, label: DATASET_SHORT[k] }))}
          onChange={setKey}
        />
      </div>
      <ConditionBar parts={parts} metric="jumlah" />
    </ChartCard>
  );
}

function topWithOther(items: { label: string; value: number }[], n: number): BarItem[] {
  const sorted = [...items].sort((a, b) => b.value - a.value);
  const head = sorted.slice(0, n);
  const rest = sorted.slice(n);
  const out: BarItem[] = head.map((i, idx) => ({ key: `${i.label}-${idx}`, label: i.label, value: i.value, display: "" }));
  if (rest.length > 0) out.push({ key: "lainnya", label: `Lainnya (${rest.length})`, value: rest.reduce((a, b) => a + b.value, 0), display: "", muted: true });
  return out;
}

function withDisplay(items: BarItem[], metric: BreakdownMetric, total: number): BarItem[] {
  return items.map((i) => ({ ...i, display: formatBreakdown(metric, i.value), note: pct(ratio(i.value, total)) }));
}

const UE1_METRICS: BreakdownMetric[] = ["jumlah", "luas_tanah", "nilai", "satker"];
type ProvMetric = Exclude<BreakdownMetric, "satker">;
const PROV_METRICS: ProvMetric[] = ["jumlah", "luas_tanah", "nilai"];

function UE1Chart({ d }: { d: DGOverviewData }) {
  const [metric, setMetric] = useState<BreakdownMetric>("jumlah");
  const rows = d.ue1.map((u) => ({ label: u.label, value: ue1Value(u, metric) })).filter((r) => r.value > 0);
  const total = rows.reduce((a, b) => a + b.value, 0);
  const items = withDisplay(topWithOther(rows, 10), metric, total);
  return (
    <ChartCard
      title="Per unit eselon I"
      subtitle={`${d.ue1.length} UE1, menurut ${BREAKDOWN_LABEL[metric].toLowerCase()}`}
      table={
        <DataTable
          columns={["UE1", "Satker", "Aset", "Luas tanah (m²)", "Nilai", "KDJ", "KDO"]}
          rows={d.ue1.map((u) => [
            u.label,
            formatNumber(u.satker, 0),
            formatNumber(perJumlah(u.per), 0),
            formatNumber(u.per.tanah?.luas ?? 0, 2),
            `Rp ${formatNumber(ue1Value(u, "nilai"), 0)}`,
            formatNumber(u.kdj, 0),
            formatNumber(u.kdo, 0),
          ])}
        />
      }
    >
      <div className="mb-4">
        <Segmented label="Ukuran per UE1" value={metric} options={UE1_METRICS.map((m) => ({ value: m, label: BREAKDOWN_LABEL[m] }))} onChange={setMetric} />
      </div>
      <BarList items={items} wide />
    </ChartCard>
  );
}

function ProvinsiChart({ d }: { d: DGOverviewData }) {
  const [metric, setMetric] = useState<ProvMetric>("jumlah");
  const rows = d.provinsi.map((p) => ({ label: p.nama, value: provinsiValue(p, metric) })).filter((r) => r.value > 0);
  const total = rows.reduce((a, b) => a + b.value, 0);
  const items = withDisplay(topWithOther(rows, 10), metric, total);
  return (
    <ChartCard
      title="Per provinsi"
      subtitle={`${d.provinsi.length} provinsi, menurut ${BREAKDOWN_LABEL[metric].toLowerCase()}`}
      table={
        <DataTable
          columns={["Provinsi", "Aset", "Luas tanah (m²)", "Nilai"]}
          rows={d.provinsi.map((p) => [p.nama, formatNumber(perJumlah(p.per), 0), formatNumber(p.per.tanah?.luas ?? 0, 2), `Rp ${formatNumber(provinsiValue(p, "nilai"), 0)}`])}
        />
      }
    >
      <div className="mb-4">
        <Segmented label="Ukuran per provinsi" value={metric} options={PROV_METRICS.map((m) => ({ value: m, label: BREAKDOWN_LABEL[m] }))} onChange={setMetric} />
      </div>
      <BarList items={items} />
    </ChartCard>
  );
}

const HUKUM_KEYS: DGDatasetKey[] = ["tanah", "gedung_lainnya", "rumah_negara", "mess_rumah_negara"];

function StatusHukumChart({ d }: { d: DGOverviewData }) {
  const keys = HUKUM_KEYS.filter((k) => (d.status_hukum[k]?.length ?? 0) > 0);
  const [key, setKey] = useState<DGDatasetKey>(keys[0] ?? "tanah");
  const list = d.status_hukum[key] ?? [];
  const total = list.reduce((a, b) => a + b.jumlah, 0);
  const items: BarItem[] = list.map((c, i) => ({
    key: `${c.k ?? "kosong"}-${i}`,
    label: shortStatusHukum(c.k, 60),
    title: c.k ?? "(tidak ada data)",
    value: c.jumlah,
    display: formatNumber(c.jumlah, 0),
    note: pct(ratio(c.jumlah, total)),
    muted: c.k === null,
  }));
  return (
    <ChartCard
      title="Status hukum"
      subtitle="10 status terbanyak"
      table={<DataTable columns={["Status hukum", "Jumlah"]} rows={list.map((c) => [c.k ?? "(tidak ada data)", formatNumber(c.jumlah, 0)])} />}
    >
      {keys.length > 1 && (
        <div className="mb-4">
          <Segmented label="Jenis aset" value={key} options={keys.map((k) => ({ value: k, label: DATASET_SHORT[k] }))} onChange={setKey} />
        </div>
      )}
      <BarList items={items} wide />
    </ChartCard>
  );
}

function AsuransiChart({ d }: { d: DGOverviewData }) {
  const merged = mergeCounts([d.asuransi.gedung_kantor_utama, d.asuransi.gedung_lainnya], asuransiLabel);
  const total = merged.reduce((a, b) => a + b.jumlah, 0);
  const items: BarItem[] = merged.map((m) => ({
    key: m.label,
    label: m.label,
    value: m.jumlah,
    display: formatNumber(m.jumlah, 0),
    note: pct(ratio(m.jumlah, total)),
    muted: m.label === "Tidak ada data",
  }));
  return (
    <ChartCard
      title="Asuransi gedung"
      subtitle="Gedung kantor utama dan gedung lainnya (kolom is_asuransi)"
      table={<DataTable columns={["Status", "Jumlah"]} rows={merged.map((m) => [m.label, formatNumber(m.jumlah, 0)])} />}
    >
      <BarList items={items} />
    </ChartCard>
  );
}

function KamarChart({ d }: { d: DGOverviewData }) {
  const items: BarItem[] = [
    ["Rusunara · tipe E", d.hunian.rusun_kamar_e],
    ["Rusunara · tipe D", d.hunian.rusun_kamar_d],
    ["Rusunara · tipe C", d.hunian.rusun_kamar_c],
    ["Mess · tipe E", d.hunian.mess_kamar_e],
    ["Mess · tipe D", d.hunian.mess_kamar_d],
  ].map(([label, v]) => ({ key: String(label), label: String(label), value: Number(v ?? 0), display: formatNumber(Number(v ?? 0), 0) }));
  return (
    <ChartCard
      title="Kamar tidur"
      subtitle="Jumlah ruang tidur pada rusunara dan mess menurut tipe"
      table={<DataTable columns={["Jenis", "Kamar"]} rows={items.map((i) => [i.label, i.display])} />}
    >
      <BarList items={items} />
    </ChartCard>
  );
}

function PenghuniChart({ d }: { d: DGOverviewData }) {
  const total = d.status_penghuni.reduce((a, b) => a + b.jumlah, 0);
  const items: BarItem[] = d.status_penghuni.map((c, i) => ({
    key: `${c.k ?? "kosong"}-${i}`,
    label: c.k ?? "(tidak ada data)",
    value: c.jumlah,
    display: formatNumber(c.jumlah, 0),
    note: pct(ratio(c.jumlah, total)),
    muted: c.k === null,
  }));
  return (
    <ChartCard
      title="Status penghuni rumah negara"
      subtitle="Jenis pemakai terbaru tiap rumah negara"
      table={<DataTable columns={["Status penghuni", "Jumlah"]} rows={d.status_penghuni.map((c) => [c.k ?? "(tidak ada data)", formatNumber(c.jumlah, 0)])} />}
    >
      <BarList items={items} />
    </ChartCard>
  );
}

// ------------------------------------------------------------------ kelengkapan data

function Kelengkapan({ d, onOpenData }: { d: DGOverviewData; onOpenData: Props["onOpenData"] }) {
  const cell = (n: number, total: number): ReactNode => (
    <>
      {formatNumber(n, 0)} <span className="text-slate-400">{pct(ratio(n, total))}</span>
    </>
  );
  return (
    <section className="min-w-0 rounded-xl border border-slate-200 p-4">
      <h3 className="text-sm font-semibold text-slate-900">Kelengkapan data</h3>
      <p className="mt-0.5 text-xs text-slate-500">Aset yang datanya perlu dilengkapi di SLDK. Koordinat dihitung valid bila ada dan berada di wilayah Indonesia.</p>
      <div className="mt-4 overflow-x-auto">
        <table className="w-full min-w-max text-xs">
          <thead>
            <tr className="border-b border-slate-200 text-slate-500">
              <th scope="col" className="py-1.5 pr-3 text-left font-medium">Jenis</th>
              <th scope="col" className="px-3 py-1.5 text-right font-medium">Jumlah</th>
              <th scope="col" className="px-3 py-1.5 text-right font-medium">Tanpa koordinat</th>
              <th scope="col" className="px-3 py-1.5 text-right font-medium">Koordinat di luar Indonesia</th>
              <th scope="col" className="px-3 py-1.5 text-right font-medium">Tanpa foto</th>
              <th scope="col" className="px-3 py-1.5 text-right font-medium">Tanpa kondisi</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100 text-slate-700">
            {d.aset.map((a) => (
              <tr key={a.key}>
                <th scope="row" className="py-1.5 pr-3 text-left font-normal">{DATASET_LABEL[a.key]}</th>
                <td className="px-3 py-1.5 text-right tabular-nums">{formatNumber(a.jumlah, 0)}</td>
                <td className="px-3 py-1.5 text-right tabular-nums">
                  {a.geo && a.tanpa_koordinat > 0 ? (
                    <button
                      type="button"
                      onClick={() => onOpenData({ dataset: a.key, tanpaKoordinat: true })}
                      className="font-medium text-blue-600 hover:text-blue-700"
                      title="Lihat daftar aset tanpa koordinat"
                    >
                      {cell(a.tanpa_koordinat, a.jumlah)}
                    </button>
                  ) : a.geo ? (
                    cell(0, a.jumlah)
                  ) : (
                    "-"
                  )}
                </td>
                <td className="px-3 py-1.5 text-right tabular-nums">{a.geo ? cell(a.di_luar_indonesia, a.jumlah) : "-"}</td>
                <td className="px-3 py-1.5 text-right tabular-nums">{cell(a.tanpa_foto, a.jumlah)}</td>
                <td className="px-3 py-1.5 text-right tabular-nums">{cell(a.tanpa_kondisi, a.jumlah)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <dl className="mt-4 grid gap-3 text-sm sm:grid-cols-2">
        <div className="rounded-lg bg-slate-50 px-3.5 py-2.5">
          <dt className="text-xs text-slate-500">Satker induk tanpa gedung kantor utama</dt>
          <dd className="mt-0.5 font-semibold text-slate-900">
            {cell(d.kelengkapan.induk_tanpa_kantor_utama, d.kelengkapan.satker_induk)}
            <span className="ml-1 text-xs font-normal text-slate-500">dari {formatNumber(d.kelengkapan.satker_induk, 0)} induk</span>
          </dd>
        </div>
        <div className="rounded-lg bg-slate-50 px-3.5 py-2.5">
          <dt className="text-xs text-slate-500">Satker induk tanpa data tanah</dt>
          <dd className="mt-0.5 font-semibold text-slate-900">
            {cell(d.kelengkapan.induk_tanpa_tanah, d.kelengkapan.satker_induk)}
            <span className="ml-1 text-xs font-normal text-slate-500">dari {formatNumber(d.kelengkapan.satker_induk, 0)} induk</span>
          </dd>
        </div>
      </dl>
    </section>
  );
}

function Skeleton() {
  return (
    <div role="status" aria-label="Memuat ringkasan" className="animate-pulse space-y-4">
      <div className="h-4 w-1/2 rounded bg-slate-100" />
      <div className="grid grid-cols-1 gap-3 min-[480px]:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 6 }, (_, i) => (
          <div key={i} className="h-24 rounded-xl bg-slate-100" />
        ))}
      </div>
      <div className="grid gap-4 lg:grid-cols-2">
        <div className="h-56 rounded-xl bg-slate-100" />
        <div className="h-56 rounded-xl bg-slate-100" />
      </div>
    </div>
  );
}
