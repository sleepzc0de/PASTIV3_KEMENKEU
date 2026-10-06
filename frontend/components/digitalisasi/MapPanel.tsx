"use client";

import { useEffect, useMemo, useState } from "react";
import dynamic from "next/dynamic";
import { Loader2, MapPinOff, X } from "lucide-react";
import { DGDatasetKey, DGDetailData, DGMapSet, getDGDetail, getDGMap } from "@/lib/api";
import { Alert } from "@/components/ui/Alert";
import { formatNumber } from "@/lib/dasbor";
import { SelectField } from "./controls";
import {
  DATASET_COLOR,
  DATASET_LABEL,
  DATASET_SHORT,
  TABLE_COLUMNS,
  TITLE_COLUMN,
  formatCell,
} from "./digitalisasi";
import type { MapFocus } from "./MapView";
import { errorMessage } from "./useDigitalisasi";

// Leaflet membaca `window` saat diimpor, jadi hanya dimuat di browser.
const MapView = dynamic(() => import("./MapView"), {
  ssr: false,
  loading: () => <div className="h-full w-full animate-pulse bg-slate-100" aria-label="Memuat peta" />,
});

interface Props {
  version: number;
  ue1Options: { value: string; label: string }[];
  focus: MapFocus | null;
  onOpenDetail: (dataset: DGDatasetKey, id: number) => void;
  onOpenData: (p: { dataset: DGDatasetKey; tanpaKoordinat?: boolean }) => void;
}

export function MapPanel({ version, ue1Options, focus, onOpenDetail, onOpenData }: Props) {
  const [ue1, setUe1] = useState("");
  const [sets, setSets] = useState<DGMapSet[] | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState("");
  const [visible, setVisible] = useState<Set<DGDatasetKey>>(new Set());
  const [selected, setSelected] = useState<{ dataset: DGDatasetKey; id: number } | null>(null);
  const [tileOk, setTileOk] = useState<boolean | null>(null);

  // Data titik dimuat ulang saat filter UE1 berganti atau sinkronisasi baru selesai. Data lama tetap tampil selama memuat.
  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);
    setError("");
    getDGMap({ ue1: ue1 || undefined })
      .then((res) => {
        if (cancelled) return;
        setSets(res.data.datasets);
        // Kategori yang punya titik ditampilkan semua pada pemuatan pertama; pilihan pengguna dipertahankan setelahnya.
        setVisible((prev) => (prev.size > 0 ? prev : new Set(res.data.datasets.filter((s) => s.bertitik > 0).map((s) => s.key))));
      })
      .catch((err) => {
        if (!cancelled) setError(errorMessage(err, "Gagal memuat peta"));
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [ue1, version]);

  // "Lihat di peta" dari daftar: titik langsung dipilih.
  useEffect(() => {
    if (focus) setSelected({ dataset: focus.dataset, id: focus.id });
  }, [focus]);

  const toggle = (key: DGDatasetKey) =>
    setVisible((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const totals = useMemo(() => {
    const list = sets ?? [];
    return {
      shown: list.filter((s) => visible.has(s.key)).reduce((a, s) => a + s.titik.length, 0),
      none: list.reduce((a, s) => a + s.tanpa_koordinat, 0),
      outside: list.reduce((a, s) => a + s.di_luar_indonesia, 0),
      cut: list.some((s) => s.terpotong),
    };
  }, [sets, visible]);

  const worst = useMemo(() => [...(sets ?? [])].sort((a, b) => b.tanpa_koordinat - a.tanpa_koordinat)[0], [sets]);

  if (error && !sets) {
    return <Alert message={error} />;
  }

  return (
    <div className="space-y-3" aria-busy={isLoading}>
      <div className="flex flex-wrap items-end gap-3">
        <div className="min-w-0 flex-1">
          <p className="mb-1.5 text-xs font-medium text-slate-600">Jenis aset di peta</p>
          <div className="flex flex-wrap gap-2" role="group" aria-label="Jenis aset di peta">
            {(sets ?? []).map((s) => {
              const on = visible.has(s.key);
              return (
                <button
                  key={s.key}
                  type="button"
                  onClick={() => toggle(s.key)}
                  aria-pressed={on}
                  disabled={s.bertitik === 0}
                  className={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-medium disabled:opacity-50 ${
                    on ? "border-slate-300 bg-white text-slate-900 shadow-sm" : "border-slate-200 bg-slate-50 text-slate-500 hover:text-slate-700"
                  }`}
                >
                  <span
                    className="h-2.5 w-2.5 shrink-0 rounded-full ring-2 ring-white"
                    style={{ backgroundColor: on ? DATASET_COLOR[s.key] : "#cbd5e1" }}
                    aria-hidden="true"
                  />
                  {DATASET_SHORT[s.key]}
                  <span className="font-normal text-slate-400">{formatNumber(s.bertitik, 0)}</span>
                </button>
              );
            })}
          </div>
        </div>
        <div className="w-full sm:w-64">
          <SelectField label="Unit eselon I" value={ue1} onChange={setUe1} options={ue1Options} allLabel="Semua UE1" />
        </div>
      </div>

      {error && <Alert message={`${error}. Menampilkan data yang dimuat sebelumnya.`} />}

      <div className={`relative transition-opacity ${isLoading ? "opacity-70" : ""}`}>
        <div className="h-[62vh] min-h-[420px] overflow-hidden rounded-xl border border-slate-200">
          {sets && (
            <MapView
              sets={sets}
              visible={visible}
              selected={selected}
              focus={focus}
              onSelect={(dataset, id) => setSelected({ dataset, id })}
              onTileStatus={setTileOk}
            />
          )}
        </div>
        {isLoading && (
          <div className="pointer-events-none absolute right-3 top-3 z-[1000] inline-flex items-center gap-1.5 rounded-full bg-white px-3 py-1 text-xs text-slate-600 shadow">
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
            Memuat titik...
          </div>
        )}
        {selected && <SelectedCard selected={selected} onClose={() => setSelected(null)} onOpenDetail={onOpenDetail} />}
      </div>

      <div className="space-y-1.5 text-xs text-slate-500">
        <p>
          <span className="font-medium text-slate-700">{formatNumber(totals.shown, 0)}</span> titik ditampilkan
          {totals.none > 0 && <> · {formatNumber(totals.none, 0)} aset belum punya koordinat</>}
          {totals.outside > 0 && <> · {formatNumber(totals.outside, 0)} koordinat di luar Indonesia tidak digambar</>}
          {worst && worst.tanpa_koordinat > 0 && (
            <>
              {" "}
              <button type="button" onClick={() => onOpenData({ dataset: worst.key, tanpaKoordinat: true })} className="font-medium text-blue-600 hover:text-blue-700">
                Lihat {DATASET_SHORT[worst.key].toLowerCase()} tanpa koordinat
              </button>
            </>
          )}
        </p>
        {totals.cut && <p className="text-amber-700">Sebagian titik tidak ikut digambar karena jumlahnya terlalu banyak. Persempit dengan filter UE1.</p>}
        {tileOk === false && (
          <p className="inline-flex items-start gap-1.5 text-amber-700">
            <MapPinOff className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            Gambar peta dasar tidak bisa dimuat (jaringan memblokir server peta?). Titik aset tetap ditampilkan; administrator dapat mengganti server peta lewat
            NEXT_PUBLIC_MAP_TILE_URL.
          </p>
        )}
      </div>
    </div>
  );
}

function SelectedCard({
  selected,
  onClose,
  onOpenDetail,
}: {
  selected: { dataset: DGDatasetKey; id: number };
  onClose: () => void;
  onOpenDetail: (dataset: DGDatasetKey, id: number) => void;
}) {
  const [data, setData] = useState<DGDetailData | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setData(null);
    setError("");
    getDGDetail(selected.dataset, selected.id)
      .then((res) => {
        if (!cancelled) setData(res.data);
      })
      .catch((err) => {
        if (!cancelled) setError(errorMessage(err, "Gagal memuat detail"));
      });
    return () => {
      cancelled = true;
    };
  }, [selected.dataset, selected.id]);

  const cols = TABLE_COLUMNS[selected.dataset].filter((c) => c.name !== TITLE_COLUMN[selected.dataset]);
  return (
    <div className="mt-3 rounded-xl border border-slate-200 bg-white p-4 shadow-sm sm:absolute sm:bottom-3 sm:left-3 sm:z-[1000] sm:mt-0 sm:w-80" role="region" aria-label="Aset terpilih">
      <div className="flex items-start justify-between gap-2">
        <p className="flex items-center gap-1.5 text-xs font-medium text-slate-500">
          <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: DATASET_COLOR[selected.dataset] }} aria-hidden="true" />
          {DATASET_LABEL[selected.dataset]}
        </p>
        <button type="button" onClick={onClose} aria-label="Tutup" className="-m-1 rounded p-1 text-slate-400 hover:bg-slate-100">
          <X className="h-4 w-4" />
        </button>
      </div>
      {error && <p className="mt-2 text-sm text-red-600">{error}</p>}
      {!data && !error && <Loader2 className="mt-3 h-4 w-4 animate-spin text-blue-600" />}
      {data && (
        <>
          <p className="mt-1 line-clamp-2 break-words text-sm font-semibold text-slate-900">{String(data.row[TITLE_COLUMN[selected.dataset]] ?? "Tanpa nama")}</p>
          <dl className="mt-2 space-y-1 text-xs">
            {cols.map((c) => {
              const col = data.kolom.find((k) => k.nama === c.name);
              return (
                <div key={c.name} className="flex gap-2">
                  <dt className="w-20 shrink-0 text-slate-500">{c.label}</dt>
                  <dd className="min-w-0 break-words text-slate-800">{formatCell(col, c.name, data.row[c.name])}</dd>
                </div>
              );
            })}
          </dl>
          <button
            type="button"
            onClick={() => onOpenDetail(selected.dataset, selected.id)}
            className="mt-3 text-xs font-medium text-blue-600 hover:text-blue-700"
          >
            Detail lengkap
          </button>
        </>
      )}
    </div>
  );
}
