"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, FileSpreadsheet, FileText, FileType, Loader2, Search, SearchX } from "lucide-react";
import { DGDatasetKey, DGListData, DGRow, FormatEkspor, eksporDigitalisasi, listDGData } from "@/lib/api";
import { Alert } from "@/components/ui/Alert";
import { TombolUnduh } from "@/components/ui/TombolUnduh";
import { useToast } from "@/components/ui/Toast";
import { formatNumber } from "@/lib/dasbor";
import { useRefUE1 } from "@/lib/useRefUE1";
import { simpanBlob } from "../sapa/download";
import { Segmented, SelectField } from "./controls";
import { DATASET_LABEL, DATASET_SHORT, TABLE_COLUMNS, TITLE_COLUMN, formatCell } from "./digitalisasi";
import { errorMessage } from "./useDigitalisasi";

const DATASETS: DGDatasetKey[] = ["tanah", "gedung_kantor_utama", "gedung_lainnya", "rusunara", "rumah_negara", "mess_rumah_negara", "satker"];
const PER_PAGE = 25;
const BATAS_PDF = 5000; // sama dengan laporan.MaksBarisPDF di backend

export interface DataPreset {
  nonce: number;
  dataset: DGDatasetKey;
  tanpaKoordinat?: boolean;
  q?: string; // kata kunci awal (mis. kode satker dari Dashboard)
}

interface Props {
  version: number;
  preset?: DataPreset;
  onOpenDetail: (dataset: DGDatasetKey, id: number) => void;
}

export function DigitalisasiData({ version, preset, onOpenDetail }: Props) {
  const toast = useToast();
  const refUE1 = useRefUE1();
  const [mengunduh, setMengunduh] = useState<FormatEkspor | null>(null);
  const [dataset, setDataset] = useState<DGDatasetKey>("tanah");
  const [q, setQ] = useState("");
  const [qApplied, setQApplied] = useState("");
  const [ue1, setUe1] = useState("");
  const [provinsi, setProvinsi] = useState("");
  const [kondisi, setKondisi] = useState("");
  const [jenisSatker, setJenisSatker] = useState("");
  const [tanpaKoordinat, setTanpaKoordinat] = useState(false);
  const [page, setPage] = useState(1);

  const [data, setData] = useState<DGListData | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState("");
  const requestId = useRef(0);

  // Kata kunci dikirim setelah pengguna berhenti mengetik.
  useEffect(() => {
    const t = setTimeout(() => {
      setQApplied(q.trim());
      setPage(1);
    }, 400);
    return () => clearTimeout(t);
  }, [q]);

  const resetFilters = () => {
    setQ("");
    setQApplied("");
    setUe1("");
    setProvinsi("");
    setKondisi("");
    setJenisSatker("");
    setTanpaKoordinat(false);
    setPage(1);
  };

  const appliedNonce = useRef<number | null>(null);
  useEffect(() => {
    if (!preset || appliedNonce.current === preset.nonce) return;
    appliedNonce.current = preset.nonce;
    resetFilters();
    setDataset(preset.dataset);
    setTanpaKoordinat(Boolean(preset.tanpaKoordinat));
    if (preset.q) {
      setQ(preset.q);
      setQApplied(preset.q);
    }
  }, [preset]);

  useEffect(() => {
    const id = ++requestId.current;
    setIsLoading(true);
    setError("");
    listDGData(dataset, {
      q: qApplied || undefined,
      ue1: ue1 || undefined,
      provinsi: provinsi || undefined,
      kondisi: kondisi || undefined,
      jenis_satker: jenisSatker || undefined,
      tanpa_koordinat: tanpaKoordinat ? "1" : undefined,
      page,
      per_page: PER_PAGE,
    })
      .then((res) => {
        if (id === requestId.current) setData(res.data);
      })
      .catch((err) => {
        if (id === requestId.current) setError(errorMessage(err, "Gagal memuat data"));
      })
      .finally(() => {
        if (id === requestId.current) setIsLoading(false);
      });
  }, [dataset, qApplied, ue1, provinsi, kondisi, jenisSatker, tanpaKoordinat, page, version]);

  const switchDataset = (key: DGDatasetKey) => {
    if (key === dataset) return;
    resetFilters();
    setDataset(key);
    setData(null);
  };

  const filter = (setter: (v: string) => void) => (v: string) => {
    setter(v);
    setPage(1);
  };

  const hasFilter = Boolean(qApplied || ue1 || provinsi || kondisi || jenisSatker || tanpaKoordinat);

  // Unduh mengikuti pencarian dan filter yang sedang berlaku (yang dipakai daftar di layar), tanpa halaman.
  const unduh = async (format: FormatEkspor) => {
    setMengunduh(format);
    try {
      const { blob, disposition } = await eksporDigitalisasi(dataset, {
        q: qApplied || undefined,
        ue1: ue1 || undefined,
        provinsi: provinsi || undefined,
        kondisi: kondisi || undefined,
        jenis_satker: jenisSatker || undefined,
        tanpa_koordinat: tanpaKoordinat ? "1" : undefined,
        format,
      });
      simpanBlob(blob, disposition, `digitalisasi-${dataset.replace(/_/g, "-")}.${format}`);
      toast.success(`Berkas ${format.toUpperCase()} diunduh.`);
    } catch (err) {
      toast.error(err instanceof Error && !("response" in err) ? err.message : errorMessage(err, "Gagal membuat berkas unduhan"));
    } finally {
      setMengunduh(null);
    }
  };
  // Hasil dari dataset lain (saat berganti) tidak boleh dipakai untuk menggambar tabel dataset ini.
  const current = data && data.dataset === dataset ? data : null;
  const cols = TABLE_COLUMNS[dataset];
  const pages = current ? Math.max(1, Math.ceil(current.total / current.per_page)) : 1;
  const from = current && current.total > 0 ? (current.page - 1) * current.per_page + 1 : 0;
  const to = current ? Math.min(current.total, current.page * current.per_page) : 0;
  const toOptions = (list?: string[]) => (list ?? []).map((v) => ({ value: v, label: v }));

  return (
    <div className="space-y-4">
      <div className="overflow-x-auto pb-1">
        <Segmented label="Jenis data" value={dataset} options={DATASETS.map((k) => ({ value: k, label: DATASET_SHORT[k] }))} onChange={switchDataset} />
      </div>

      <div className="space-y-3">
        <div className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            inputMode="search"
            autoComplete="off"
            aria-label={`Cari ${DATASET_LABEL[dataset].toLowerCase()}`}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Cari nama satker, uraian, alamat, atau kode..."
            className="w-full rounded-lg border border-slate-300 bg-white py-2.5 pl-10 pr-3.5 text-sm outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10"
          />
        </div>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {current?.filter.ue1 && <SelectField label="Unit eselon I" value={ue1} onChange={filter(setUe1)} options={refUE1.opsi(current.filter.ue1)} />}
          {current?.filter.provinsi && <SelectField label="Provinsi" value={provinsi} onChange={filter(setProvinsi)} options={toOptions(current.filter.provinsi)} />}
          {current?.filter.kondisi && <SelectField label="Kondisi" value={kondisi} onChange={filter(setKondisi)} options={toOptions(current.filter.kondisi)} />}
          {dataset === "satker" && (
            <SelectField
              label="Jenis satker"
              value={jenisSatker}
              onChange={filter(setJenisSatker)}
              options={[
                { value: "INDUK SATKER", label: "Induk satker" },
                { value: "ANAK SATKER", label: "Anak satker" },
              ]}
            />
          )}
        </div>
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          {current?.geo && (
            <label className="inline-flex cursor-pointer items-center gap-2 text-sm text-slate-700">
              <input
                type="checkbox"
                checked={tanpaKoordinat}
                onChange={(e) => {
                  setTanpaKoordinat(e.target.checked);
                  setPage(1);
                }}
                className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
              />
              Hanya yang belum punya koordinat
            </label>
          )}
          {hasFilter && (
            <button type="button" onClick={resetFilters} className="text-xs font-medium text-slate-500 hover:text-slate-700">
              Reset pencarian dan filter
            </button>
          )}
        </div>
      </div>

      {error && <Alert message={error} />}

      {!current && !error && (
        <div role="status" aria-label="Memuat data" className="animate-pulse space-y-2">
          {Array.from({ length: 5 }, (_, i) => (
            <div key={i} className="h-10 rounded-lg bg-slate-100" />
          ))}
        </div>
      )}

      {current && (
        <div className={`space-y-3 transition-opacity ${isLoading ? "opacity-60" : ""}`} aria-busy={isLoading}>
          <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
            <p aria-live="polite" className="text-sm text-slate-600">
              <span className="font-semibold text-slate-900">{formatNumber(current.total, 0)}</span> {DATASET_LABEL[dataset].toLowerCase()}
              {current.total > 0 && <span className="text-slate-400"> · menampilkan {formatNumber(from, 0)}–{formatNumber(to, 0)}</span>}
            </p>
            <div role="group" aria-label={`Unduh ${DATASET_LABEL[dataset].toLowerCase()}`} className="flex flex-wrap items-center gap-2">
              <TombolUnduh
                format="xlsx"
                label="Excel"
                ikon={FileSpreadsheet}
                sibuk={mengunduh}
                nonaktif={current.total === 0 || isLoading}
                onKlik={unduh}
                judul="Unduh semua kolom sebagai Excel (.xlsx)"
              />
              <TombolUnduh
                format="csv"
                label="CSV"
                ikon={FileText}
                sibuk={mengunduh}
                nonaktif={current.total === 0 || isLoading}
                onKlik={unduh}
                judul="Unduh semua kolom sebagai CSV (pemisah titik koma, untuk Excel Indonesia)"
              />
              <TombolUnduh
                format="pdf"
                label="PDF"
                ikon={FileType}
                sibuk={mengunduh}
                nonaktif={current.total === 0 || isLoading}
                onKlik={unduh}
                judul={`Unduh ringkasan sebagai PDF (maksimum ${formatNumber(BATAS_PDF, 0)} baris)`}
              />
            </div>
          </div>
          {current.total > 0 && (
            <p className="-mt-1 text-xs text-slate-400">
              Unduhan mengikuti pencarian dan filter di atas ({formatNumber(current.total, 0)} baris). Excel dan CSV memuat semua kolom; PDF memuat kolom ringkasan dan
              dibatasi {formatNumber(BATAS_PDF, 0)} baris.
            </p>
          )}

          {current.rows.length === 0 ? (
            <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-10 text-center">
              <SearchX className="mx-auto h-8 w-8 text-slate-400" />
              <p className="mt-2 text-sm font-medium text-slate-700">{hasFilter ? "Tidak ada data yang cocok" : "Belum ada data"}</p>
              <p className="mt-1 text-sm text-slate-500">
                {hasFilter ? "Kurangi kata kunci atau filter." : "Data muncul setelah dataset ini disinkronkan dari SLDK."}
              </p>
            </div>
          ) : (
            <>
              <div className="hidden overflow-x-auto rounded-lg border border-slate-200 md:block">
                <table className="w-full min-w-max text-sm">
                  <thead className="bg-slate-50 text-xs text-slate-500">
                    <tr>
                      {cols.map((c) => (
                        <th key={c.name} scope="col" className="px-3 py-2 text-left font-medium">
                          {c.label}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {current.rows.map((row) => (
                      <tr key={row.id} className="hover:bg-slate-50">
                        {cols.map((c, i) => {
                          const text = formatCell(current.kolom.find((k) => k.nama === c.name), c.name, row[c.name]);
                          const numeric = /^(Luas_|Nilai_|Jumlah_)/.test(c.name);
                          return (
                            <td key={c.name} className={`max-w-[18rem] px-3 py-2 ${numeric ? "text-right tabular-nums" : "truncate"} text-slate-700`} title={text.length > 40 ? text : undefined}>
                              {i === 0 ? (
                                <button type="button" onClick={() => onOpenDetail(dataset, row.id)} className="max-w-full truncate text-left font-medium text-blue-700 hover:underline">
                                  {text}
                                </button>
                              ) : (
                                text
                              )}
                            </td>
                          );
                        })}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <ul className="divide-y divide-slate-100 overflow-hidden rounded-lg border border-slate-200 md:hidden">
                {current.rows.map((row) => (
                  <li key={row.id}>
                    <MobileRow dataset={dataset} row={row} kolom={current.kolom} onOpen={() => onOpenDetail(dataset, row.id)} />
                  </li>
                ))}
              </ul>
            </>
          )}

          {pages > 1 && (
            <div className="flex items-center justify-between gap-3">
              <button
                type="button"
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page <= 1 || isLoading}
                className="inline-flex items-center gap-1 rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50"
              >
                <ChevronLeft className="h-4 w-4" />
                Sebelumnya
              </button>
              <span className="text-xs text-slate-500">
                Halaman {formatNumber(current.page, 0)} dari {formatNumber(pages, 0)}
              </span>
              <button
                type="button"
                onClick={() => setPage((p) => Math.min(pages, p + 1))}
                disabled={page >= pages || isLoading}
                className="inline-flex items-center gap-1 rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50"
              >
                Berikutnya
                <ChevronRight className="h-4 w-4" />
              </button>
            </div>
          )}
        </div>
      )}
      {isLoading && current && (
        <p className="flex items-center gap-1.5 text-xs text-slate-400">
          <Loader2 className="h-3.5 w-3.5 animate-spin" /> Memuat...
        </p>
      )}
    </div>
  );
}

function MobileRow({ dataset, row, kolom, onOpen }: { dataset: DGDatasetKey; row: DGRow; kolom: DGListData["kolom"]; onOpen: () => void }) {
  const get = (name: string) => {
    const v = row[name];
    return v === null || v === undefined || v === "" ? "" : formatCell(kolom.find((k) => k.nama === name), name, v);
  };
  const cols = TABLE_COLUMNS[dataset];
  const title = get(TITLE_COLUMN[dataset]) || "Tanpa nama";
  const rest = cols.filter((c) => c.name !== TITLE_COLUMN[dataset]);
  return (
    <button type="button" onClick={onOpen} className="flex w-full items-start gap-3 px-3 py-3 text-left hover:bg-slate-50 focus-visible:bg-slate-50 focus-visible:outline-none">
      <div className="min-w-0 flex-1 space-y-1">
        <p className="line-clamp-2 break-words text-sm font-semibold text-slate-900">{title}</p>
        <dl className="space-y-0.5 text-xs">
          {rest.map((c) => {
            const v = get(c.name);
            if (!v) return null;
            return (
              <div key={c.name} className="flex gap-2">
                <dt className="w-16 shrink-0 text-slate-400">{c.label.replace(" (m²)", "")}</dt>
                <dd className="min-w-0 break-words text-slate-700">{/^Luas_/.test(c.name) ? `${v} m²` : v}</dd>
              </div>
            );
          })}
        </dl>
      </div>
      <ChevronRight className="mt-1 h-4 w-4 shrink-0 text-slate-300" />
    </button>
  );
}
