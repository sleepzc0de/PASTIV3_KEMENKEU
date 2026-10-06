"use client";

import { useMemo, useState } from "react";
import { ChevronLeft, ChevronRight, FileSpreadsheet, FileText, FileType, Loader2, RefreshCw, Search } from "lucide-react";
import { FormatEkspor, PemisahCsv, PenarikanStatus, eksporInaprocData } from "@/lib/api";
import { formatAngka, formatPersen, formatWaktu, kelompokkanDataset } from "@/lib/pengadaan";
import { Alert } from "@/components/ui/Alert";
import { EmptyState } from "@/components/ui/EmptyState";
import { TombolUnduh } from "@/components/ui/TombolUnduh";
import { useToast } from "@/components/ui/Toast";
import { simpanBlob } from "../sapa/download";
import { errorMessage } from "../digitalisasi/useDigitalisasi";
import { KELAS_INPUT } from "./lencana";
import { useDataHalaman } from "./usePengadaan";

const BATAS_PDF = 5000;

// Tampilan data lokal satu dataset dengan penyaring, plus ekspor ke Excel, CSV, dan PDF dengan penyaring yang sama.
export function DataEksporPanel({ status, versi, awal }: { status: PenarikanStatus; versi: number; awal?: string }) {
  const toast = useToast();
  const kelompok = useMemo(() => kelompokkanDataset(status.datasets, status.kelompok), [status.datasets, status.kelompok]);
  const [datasetId, setDatasetId] = useState(awal ?? "tender/pengumuman");
  const dataset = status.datasets.find((d) => d.id === datasetId) ?? status.datasets[0];

  const [klpd, setKlpd] = useState(status.kode_klpd);
  const [tahun, setTahun] = useState("");
  const [cariInput, setCariInput] = useState("");
  const [cari, setCari] = useState("");
  const [halaman, setHalaman] = useState(1);
  const [perHalaman, setPerHalaman] = useState(25);
  const [pemisah, setPemisah] = useState<PemisahCsv>("titik-koma");
  const [mengunduh, setMengunduh] = useState<FormatEkspor | null>(null);
  const [galat, setGalat] = useState("");

  const filter = useMemo(
    () => ({ kode_klpd: dataset?.punya_klpd ? klpd.trim() || undefined : undefined, tahun: dataset?.punya_tahun ? tahun || undefined : undefined, cari: cari || undefined }),
    [dataset, klpd, tahun, cari]
  );
  const data = useDataHalaman(dataset ? { dataset: dataset.id, filter, halaman, perHalaman } : null, versi);
  const hasil = data.data;
  const jumlahHalaman = hasil ? Math.max(1, Math.ceil(hasil.total / hasil.per_halaman)) : 1;

  const ubah = (f: () => void) => {
    f();
    setHalaman(1);
  };

  const unduh = async (format: FormatEkspor) => {
    if (!dataset) return;
    setMengunduh(format);
    setGalat("");
    try {
      const { blob, disposition } = await eksporInaprocData(dataset.id, { ...filter, format, pemisah: format === "csv" ? pemisah : undefined });
      simpanBlob(blob, disposition, `${dataset.id.replace("/", "-")}.${format}`);
      toast.success(`Berkas ${format.toUpperCase()} diunduh.`);
    } catch (err) {
      const pesan = err instanceof Error && !("response" in err) ? err.message : errorMessage(err, "Gagal membuat berkas ekspor");
      setGalat(pesan);
      toast.error(pesan);
    } finally {
      setMengunduh(null);
    }
  };

  if (!dataset) return <EmptyState title="Tidak ada dataset" />;
  const tahunTersedia = hasil?.tahun_tersedia ?? [];
  const total = hasil?.total ?? 0;

  return (
    <div className="space-y-4">
      <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm sm:p-5">
        <div className="grid gap-3 md:grid-cols-[minmax(0,1.4fr)_9rem_9rem_minmax(0,1fr)]">
          <div>
            <label htmlFor="data-dataset" className="mb-1 block text-xs font-medium text-slate-600">
              Dataset
            </label>
            <select
              id="data-dataset"
              value={dataset.id}
              onChange={(e) =>
                ubah(() => {
                  setDatasetId(e.target.value);
                  setTahun("");
                })
              }
              className={KELAS_INPUT}
            >
              {kelompok.map((k) => (
                <optgroup key={k.id} label={k.nama}>
                  {k.subkelompok.flatMap((s) => s.datasets).map((d) => (
                    <option key={d.id} value={d.id}>
                      {d.nama}
                    </option>
                  ))}
                </optgroup>
              ))}
            </select>
          </div>
          <div>
            <label htmlFor="data-klpd" className="mb-1 block text-xs font-medium text-slate-600">
              Kode KLPD
            </label>
            <input id="data-klpd" value={klpd} onChange={(e) => ubah(() => setKlpd(e.target.value))} disabled={!dataset.punya_klpd} placeholder={dataset.punya_klpd ? "K10" : "tidak berlaku"} className={KELAS_INPUT} />
          </div>
          <div>
            <label htmlFor="data-tahun" className="mb-1 block text-xs font-medium text-slate-600">
              Tahun
            </label>
            <select id="data-tahun" value={tahun} onChange={(e) => ubah(() => setTahun(e.target.value))} disabled={!dataset.punya_tahun} className={KELAS_INPUT}>
              <option value="">{dataset.punya_tahun ? "Semua tahun" : "tidak berlaku"}</option>
              {tahunTersedia.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
          </div>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              ubah(() => setCari(cariInput.trim()));
            }}
          >
            <label htmlFor="data-cari" className="mb-1 block text-xs font-medium text-slate-600">
              Cari
            </label>
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
              <input id="data-cari" value={cariInput} onChange={(e) => setCariInput(e.target.value)} placeholder="Nama paket, kode, satker..." className={`${KELAS_INPUT} pl-9`} />
            </div>
          </form>
        </div>
        <p className="mt-3 text-xs text-slate-500">{dataset.deskripsi}</p>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm sm:p-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h3 className="text-sm font-semibold text-slate-900">Ekspor data</h3>
            <p className="text-xs text-slate-500">
              {formatAngka(total)} baris cocok dengan penyaring di atas. Excel dan CSV memuat semua kolom; PDF memuat kolom ringkasan dan dibatasi {formatAngka(BATAS_PDF)} baris.
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <TombolUnduh format="xlsx" label="Excel" ikon={FileSpreadsheet} sibuk={mengunduh} nonaktif={total === 0} onKlik={unduh} />
            <TombolUnduh format="csv" label="CSV" ikon={FileText} sibuk={mengunduh} nonaktif={total === 0} onKlik={unduh} />
            <TombolUnduh format="pdf" label="PDF" ikon={FileType} sibuk={mengunduh} nonaktif={total === 0} onKlik={unduh} />
          </div>
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-slate-600">
          <label className="inline-flex items-center gap-2">
            Pemisah CSV
            <select value={pemisah} onChange={(e) => setPemisah(e.target.value as PemisahCsv)} className="rounded-md border border-slate-300 bg-white px-2 py-1 text-xs">
              <option value="titik-koma">Titik koma (;) untuk Excel Indonesia</option>
              <option value="koma">Koma (,)</option>
              <option value="tab">Tab</option>
            </select>
          </label>
          <span className="text-slate-400">Angka di CSV memakai titik desimal. Untuk angka yang langsung bisa dihitung, pilih Excel.</span>
        </div>
        {total > BATAS_PDF && <p className="mt-2 text-xs text-amber-700">PDF hanya memuat {formatAngka(BATAS_PDF)} baris pertama dari {formatAngka(total)} baris ({formatPersen((BATAS_PDF / total) * 100, 0)}). Gunakan Excel atau CSV untuk data lengkap.</p>}
        {galat && (
          <div className="mt-3">
            <Alert message={galat} />
          </div>
        )}
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white shadow-sm">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-slate-100 px-4 py-3">
          <p className="text-sm font-semibold text-slate-900">
            {dataset.nama}
            <span className="ml-2 text-xs font-normal text-slate-500">
              {dataset.disinkron_at ? `data diperbarui ${formatWaktu(dataset.disinkron_at)}` : "belum ada data"}
            </span>
          </p>
          <div className="flex items-center gap-2 text-xs text-slate-600">
            <label className="inline-flex items-center gap-1.5">
              Baris
              <select value={perHalaman} onChange={(e) => ubah(() => setPerHalaman(Number(e.target.value)))} className="rounded-md border border-slate-300 bg-white px-1.5 py-1 text-xs">
                {[25, 50, 100].map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
            </label>
            <button type="button" onClick={data.reload} disabled={data.isLoading} className="inline-flex items-center gap-1 rounded-md px-2 py-1 font-medium text-slate-600 hover:bg-slate-100 disabled:opacity-60">
              {data.isLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
              Muat ulang
            </button>
          </div>
        </div>

        {data.error && (
          <div className="p-4">
            <Alert message={data.error} />
          </div>
        )}
        {!hasil && !data.error && <div role="status" aria-label="Memuat data" className="m-4 h-40 animate-pulse rounded-xl bg-slate-100" />}
        {hasil && hasil.baris.length === 0 && (
          <div className="p-4">
            <EmptyState
              title="Belum ada data"
              description={total === 0 && !cari && !tahun ? "Dataset ini belum pernah ditarik. Gunakan tab Tarik Data untuk mengambilnya dari Inaproc." : "Tidak ada baris yang cocok dengan penyaring ini."}
            />
          </div>
        )}
        {hasil && hasil.baris.length > 0 && (
          <div className={`overflow-x-auto ${data.isLoading ? "opacity-60" : ""}`}>
            <table className="w-full min-w-max text-xs">
              <thead>
                <tr className="border-b border-slate-200 bg-slate-50 text-slate-500">
                  {hasil.kolom.map((k) => (
                    <th key={k.nama} scope="col" className={`whitespace-nowrap px-3 py-2 font-medium ${k.jenis === "angka" ? "text-right" : "text-left"}`}>
                      {k.label}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {hasil.baris.map((b, i) => (
                  <tr key={i} className="hover:bg-slate-50/70">
                    {b.map((v, j) => (
                      <td key={j} className={`max-w-[22rem] px-3 py-1.5 align-top text-slate-700 ${hasil.kolom[j].jenis === "angka" ? "whitespace-nowrap text-right tabular-nums" : "truncate"}`} title={typeof v === "string" ? v : undefined}>
                        {sel(v, hasil.kolom[j].jenis)}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {hasil && hasil.total > 0 && (
          <div className="flex flex-wrap items-center justify-between gap-2 border-t border-slate-100 px-4 py-2.5 text-xs text-slate-600">
            <span>
              Halaman {formatAngka(halaman)} dari {formatAngka(jumlahHalaman)} · {formatAngka(hasil.total)} baris
            </span>
            <span className="inline-flex gap-1">
              <button type="button" onClick={() => setHalaman((h) => Math.max(1, h - 1))} disabled={halaman <= 1 || data.isLoading} aria-label="Halaman sebelumnya" className="rounded-md border border-slate-300 p-1.5 hover:bg-slate-50 disabled:opacity-40">
                <ChevronLeft className="h-4 w-4" />
              </button>
              <button type="button" onClick={() => setHalaman((h) => Math.min(jumlahHalaman, h + 1))} disabled={halaman >= jumlahHalaman || data.isLoading} aria-label="Halaman berikutnya" className="rounded-md border border-slate-300 p-1.5 hover:bg-slate-50 disabled:opacity-40">
                <ChevronRight className="h-4 w-4" />
              </button>
            </span>
          </div>
        )}
      </section>
    </div>
  );
}

function sel(v: string | number | boolean | null, jenis: "teks" | "angka" | "tanggal"): string {
  if (v === null || v === undefined || v === "") return "-";
  if (typeof v === "number") return formatAngka(v, 2);
  if (typeof v === "boolean") return v ? "Ya" : "Tidak";
  // Tanggal disimpan tanpa zona; tanggal tanpa jam ditulis tanggalnya saja.
  if (jenis === "tanggal") return v.endsWith(" 00:00:00") ? v.slice(0, 10) : v;
  return v;
}
