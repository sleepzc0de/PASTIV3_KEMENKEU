"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { ChevronLeft, ChevronRight, Info, Link2, Loader2, RefreshCw, Search, SearchX, TriangleAlert } from "lucide-react";
import axios from "axios";
import { DGDatasetKey, SatkerKeterhubungan, SatkerStatus, SatkerTerhubung, getSatkerKeterhubungan } from "@/lib/api";
import { compactRupiah, formatNumber } from "@/lib/dasbor";
import { URUT_LABEL, UrutSatker, hitungPerStatus, kodeUE1Daftar, persenTerhubung, petunjukKeterhubungan, saringSatker, urutkanSatker } from "@/lib/satker";
import { useRefUE1 } from "@/lib/useRefUE1";
import { Alert } from "@/components/ui/Alert";
import { StatTile } from "@/components/ui/charts";
import { Segmented, SelectField } from "../digitalisasi/controls";
import { DATASET_SHORT } from "../digitalisasi/digitalisasi";

const PER_HALAMAN = 25;
const URUTAN_DATASET: DGDatasetKey[] = ["tanah", "gedung_kantor_utama", "gedung_lainnya", "rusunara", "rumah_negara", "mess_rumah_negara"];

const STATUS_LABEL: Record<SatkerStatus, string> = { terhubung: "Terhubung", hanya_aset: "Hanya aset", hanya_pengadaan: "Hanya pengadaan" };
const STATUS_CLS: Record<SatkerStatus, string> = {
  terhubung: "bg-emerald-50 text-emerald-700 ring-emerald-200",
  hanya_aset: "bg-sky-50 text-sky-700 ring-sky-200",
  hanya_pengadaan: "bg-amber-50 text-amber-700 ring-amber-200",
};

function pesanGalat(err: unknown): string {
  return axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : "Gagal memuat keterhubungan satker";
}

// Rincian aset satker, mis. "Tanah 3 · Gedung Lain 1".
function rincianAset(s: SatkerTerhubung): string {
  return URUTAN_DATASET.filter((k) => (s.aset[k] ?? 0) > 0)
    .map((k) => `${DATASET_SHORT[k]} ${formatNumber(s.aset[k] ?? 0, 0)}`)
    .join(" · ");
}

// Tautan ke data aset satker itu: dataset aset dengan jumlah terbanyak, atau daftar satker bila belum ada aset.
function tautanAset(s: SatkerTerhubung): string {
  const ds = URUTAN_DATASET.find((k) => (s.aset[k] ?? 0) > 0) ?? "satker";
  return `/dashboard/digitalisasi?tab=data&dataset=${ds}&q=${s.kode}`;
}

// Dashboard Satker: setiap satker dengan jumlah aset (Digitalisasi Aset) dan pengadaan (Inaproc) pada tahun terpilih. Kedua data dihubungkan lewat kode
// satker 6 digit: SUBSTRING(Kode_Satker, 10, 6) pada data aset = kd_satker_str pada data Inaproc.
export function DasborSatker() {
  const [tahun, setTahun] = useState("");
  const [data, setData] = useState<SatkerKeterhubungan | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState("");
  const idPermintaan = useRef(0);
  const [muatUlang, setMuatUlang] = useState(0);

  const [q, setQ] = useState("");
  const [status, setStatus] = useState<SatkerStatus | "">("");
  const [ue1, setUe1] = useState("");
  const [urut, setUrut] = useState<UrutSatker>("aset");
  const [halaman, setHalaman] = useState(1);
  const refUE1 = useRefUE1();

  useEffect(() => {
    const id = ++idPermintaan.current;
    setIsLoading(true);
    setError("");
    getSatkerKeterhubungan({ tahun: tahun || undefined })
      .then((res) => {
        if (id === idPermintaan.current) setData(res.data);
      })
      .catch((err) => {
        if (id === idPermintaan.current) setError(pesanGalat(err));
      })
      .finally(() => {
        if (id === idPermintaan.current) setIsLoading(false);
      });
  }, [tahun, muatUlang]);

  const gantiFilter = useCallback(<T,>(set: (v: T) => void) => (v: T) => {
    set(v);
    setHalaman(1);
  }, []);

  const tampil = useMemo(() => {
    if (!data) return [];
    return urutkanSatker(saringSatker(data.satker, { q, status, ue1 }), urut);
  }, [data, q, status, ue1, urut]);
  const perStatus = useMemo(() => hitungPerStatus(data ? saringSatker(data.satker, { q, status: "", ue1 }) : []), [data, q, ue1]);
  const opsiUE1 = useMemo(() => (data ? refUE1.opsi(kodeUE1Daftar(data.satker)) : []), [data, refUE1]);

  if (!data) {
    if (error) {
      return (
        <div className="space-y-3">
          <Alert message={error} />
          <button type="button" onClick={() => setMuatUlang((n) => n + 1)} className="text-sm font-medium text-blue-600 hover:text-blue-700">
            Coba lagi
          </button>
        </div>
      );
    }
    return <div role="status" aria-label="Memuat keterhubungan satker" className="h-64 animate-pulse rounded-2xl bg-slate-100" />;
  }

  const r = data.ringkasan;
  const petunjuk = petunjukKeterhubungan(r, data.tahun);
  const halamanMaks = Math.max(1, Math.ceil(tampil.length / PER_HALAMAN));
  const hal = Math.min(halaman, halamanMaks);
  const irisan = tampil.slice((hal - 1) * PER_HALAMAN, hal * PER_HALAMAN);
  const adaFilter = Boolean(q.trim() || status || ue1);

  return (
    <div className="space-y-5" aria-busy={isLoading}>
      <section className="flex flex-wrap items-end justify-between gap-x-4 gap-y-3 rounded-2xl border border-slate-200 bg-white px-4 py-3 shadow-sm">
        <div className="min-w-0 text-xs text-slate-500">
          <p className="flex items-center gap-1.5 font-medium text-slate-700">
            <Link2 className="h-3.5 w-3.5 text-blue-600" aria-hidden="true" />
            Aset dan pengadaan dihubungkan lewat kode satker 6 digit
          </p>
          <p className="mt-0.5">
            <span className="font-mono">SUBSTRING(Kode_Satker, 10, 6)</span> pada data aset = <span className="font-mono">kd_satker_str</span> pada Inaproc · KLPD {data.kode_klpd}
          </p>
        </div>
        <div className="flex items-end gap-2">
          <div className="w-32">
            <label htmlFor="satker-tahun" className="mb-1.5 block text-xs font-medium text-slate-600">
              Tahun anggaran
            </label>
            <select
              id="satker-tahun"
              value={data.tahun}
              onChange={(e) => {
                setTahun(e.target.value);
                setHalaman(1);
              }}
              className="w-full rounded-xl border border-slate-300 bg-white px-3 py-2.5 text-sm shadow-sm outline-none hover:border-slate-400 focus:border-blue-500 focus:shadow-glow"
            >
              {(data.tahun_tersedia.includes(data.tahun) ? data.tahun_tersedia : [data.tahun, ...data.tahun_tersedia]).map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
          </div>
          <button
            type="button"
            onClick={() => setMuatUlang((n) => n + 1)}
            disabled={isLoading}
            className="inline-flex items-center gap-1.5 rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-xs font-medium text-slate-700 shadow-sm hover:bg-slate-50 disabled:opacity-60"
          >
            {isLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden="true" /> : <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />}
            Muat ulang
          </button>
        </div>
      </section>

      {error && <Alert message={`${error}. Menampilkan data yang dimuat sebelumnya.`} />}

      <div className={`space-y-5 transition-opacity ${isLoading ? "opacity-60" : ""}`}>
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
          <StatTile label="Satker pada data aset" value={formatNumber(r.satker_aset, 0)} sub="Digitalisasi Aset (SLDK)" />
          <StatTile label={`Satker pengadaan ${data.tahun}`} value={formatNumber(r.satker_pengadaan, 0)} sub="punya paket RUP, tender, atau non-tender" />
          <StatTile label="Terhubung" value={formatNumber(r.terhubung, 0)} sub={`${formatNumber(persenTerhubung(r), 1)}% dari satker pada data aset`} />
          <StatTile label="Hanya aset" value={formatNumber(r.hanya_aset, 0)} sub={`tanpa pengadaan pada ${data.tahun}`} />
          <StatTile label="Hanya pengadaan" value={formatNumber(r.hanya_pengadaan, 0)} sub="tidak ditemukan pada data aset" />
        </div>

        {petunjuk.length > 0 && (
          <ul className="space-y-2" aria-label="Petunjuk">
            {petunjuk.map((p, i) => (
              <li
                key={i}
                className={`flex items-start gap-2.5 rounded-xl border px-3.5 py-2.5 text-sm ${
                  p.tingkat === "perhatian" ? "border-amber-200 bg-amber-50 text-amber-900" : "border-slate-200 bg-slate-50 text-slate-700"
                }`}
              >
                {p.tingkat === "perhatian" ? <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" aria-hidden="true" /> : <Info className="mt-0.5 h-4 w-4 shrink-0 text-slate-400" aria-hidden="true" />}
                <span>{p.isi}</span>
              </li>
            ))}
          </ul>
        )}

        <section className="space-y-3 rounded-2xl border border-slate-200 bg-white p-4 shadow-sm sm:p-5" aria-label="Daftar satker">
          <div className="relative">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
            <input
              type="text"
              inputMode="search"
              autoComplete="off"
              aria-label="Cari satker"
              value={q}
              onChange={(e) => gantiFilter(setQ)(e.target.value)}
              placeholder="Cari kode satker, nama, atau UE1..."
              className="w-full rounded-lg border border-slate-300 bg-white py-2.5 pl-10 pr-3.5 text-sm outline-none focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10"
            />
          </div>
          <div className="flex flex-wrap items-end gap-3">
            <div className="max-w-full overflow-x-auto">
              <Segmented
                label="Status keterhubungan"
                value={status || "semua"}
                options={[
                  { value: "semua", label: `Semua (${formatNumber(perStatus.semua, 0)})` },
                  { value: "terhubung", label: `Terhubung (${formatNumber(perStatus.terhubung, 0)})` },
                  { value: "hanya_aset", label: `Hanya aset (${formatNumber(perStatus.hanya_aset, 0)})` },
                  { value: "hanya_pengadaan", label: `Hanya pengadaan (${formatNumber(perStatus.hanya_pengadaan, 0)})` },
                ]}
                onChange={(v) => gantiFilter(setStatus)(v === "semua" ? "" : (v as SatkerStatus))}
              />
            </div>
            <div className="w-full sm:w-64">
              <SelectField label="Unit eselon I" value={ue1} onChange={gantiFilter(setUe1)} options={opsiUE1} allLabel="Semua UE1" />
            </div>
            <div className="w-full sm:w-64">
              <SelectField
                label="Urutkan"
                value={urut === "aset" ? "" : urut}
                onChange={(v) => gantiFilter(setUrut)((v || "aset") as UrutSatker)}
                options={(Object.keys(URUT_LABEL) as UrutSatker[]).filter((k) => k !== "aset").map((k) => ({ value: k, label: URUT_LABEL[k] }))}
                allLabel={URUT_LABEL.aset}
              />
            </div>
            {adaFilter && (
              <button
                type="button"
                onClick={() => {
                  setQ("");
                  setStatus("");
                  setUe1("");
                  setHalaman(1);
                }}
                className="pb-2.5 text-xs font-medium text-slate-500 hover:text-slate-700"
              >
                Reset
              </button>
            )}
          </div>

          <p aria-live="polite" className="text-sm text-slate-600">
            <span className="font-semibold text-slate-900">{formatNumber(tampil.length, 0)}</span> satker
            {tampil.length > 0 && (
              <span className="text-slate-400">
                {" "}
                · menampilkan {formatNumber((hal - 1) * PER_HALAMAN + 1, 0)}–{formatNumber(Math.min(tampil.length, hal * PER_HALAMAN), 0)}
              </span>
            )}
          </p>

          {tampil.length === 0 ? (
            <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-10 text-center">
              <SearchX className="mx-auto h-8 w-8 text-slate-400" aria-hidden="true" />
              <p className="mt-2 text-sm font-medium text-slate-700">{adaFilter ? "Tidak ada satker yang cocok" : "Belum ada satker"}</p>
              <p className="mt-1 text-sm text-slate-500">{adaFilter ? "Kurangi kata kunci atau filter." : "Data muncul setelah sinkronisasi aset dan penarikan pengadaan dijalankan."}</p>
            </div>
          ) : (
            <>
              <div className="hidden overflow-x-auto rounded-lg border border-slate-200 md:block">
                <table className="w-full min-w-max text-sm">
                  <thead className="bg-slate-50 text-xs text-slate-500">
                    <tr>
                      <th scope="col" className="px-3 py-2 text-left font-medium">Satker</th>
                      <th scope="col" className="px-3 py-2 text-right font-medium">Aset</th>
                      <th scope="col" className="px-3 py-2 text-right font-medium">Paket RUP</th>
                      <th scope="col" className="px-3 py-2 text-right font-medium">Pagu RUP</th>
                      <th scope="col" className="px-3 py-2 text-right font-medium">Tender</th>
                      <th scope="col" className="px-3 py-2 text-right font-medium">Non-tender</th>
                      <th scope="col" className="px-3 py-2 text-left font-medium">Status</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {irisan.map((s) => (
                      <tr key={s.kode} className="align-top hover:bg-slate-50">
                        <td className="max-w-[24rem] px-3 py-2">
                          <p className="break-words font-medium text-slate-900">{s.nama || s.nama_pengadaan || "(tanpa nama)"}</p>
                          <p className="text-xs text-slate-500">
                            <span className="font-mono">{s.kode}</span>
                            {s.ue1 && <span> · {s.ue1}</span>}
                            {s.jumlah_kode_aset > 1 && <span> · {s.jumlah_kode_aset} kode (induk + anak)</span>}
                          </p>
                          {s.nama_pengadaan && s.nama && <p className="text-xs text-amber-700">Di Inaproc: {s.nama_pengadaan}</p>}
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums">
                          {s.jumlah_aset > 0 ? (
                            <Link href={tautanAset(s)} className="font-medium text-blue-700 hover:underline" title={rincianAset(s)}>
                              {formatNumber(s.jumlah_aset, 0)}
                            </Link>
                          ) : (
                            <span className="text-slate-300">0</span>
                          )}
                          {s.jumlah_aset > 0 && <p className="max-w-[12rem] text-[11px] text-slate-400">{rincianAset(s)}</p>}
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums text-slate-700">{formatNumber(s.pengadaan.rup_paket, 0)}</td>
                        <td className="px-3 py-2 text-right tabular-nums text-slate-700">{s.pengadaan.rup_pagu > 0 ? compactRupiah(s.pengadaan.rup_pagu) : <span className="text-slate-300">-</span>}</td>
                        <td className="px-3 py-2 text-right tabular-nums text-slate-700">{formatNumber(s.pengadaan.tender, 0)}</td>
                        <td className="px-3 py-2 text-right tabular-nums text-slate-700">{formatNumber(s.pengadaan.non_tender, 0)}</td>
                        <td className="px-3 py-2">
                          <span className={`whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset ${STATUS_CLS[s.status]}`}>{STATUS_LABEL[s.status]}</span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <ul className="divide-y divide-slate-100 overflow-hidden rounded-lg border border-slate-200 md:hidden">
                {irisan.map((s) => (
                  <li key={s.kode} className="space-y-1.5 px-3 py-3">
                    <div className="flex items-start justify-between gap-2">
                      <p className="min-w-0 break-words text-sm font-semibold text-slate-900">{s.nama || s.nama_pengadaan || "(tanpa nama)"}</p>
                      <span className={`shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium ring-1 ring-inset ${STATUS_CLS[s.status]}`}>{STATUS_LABEL[s.status]}</span>
                    </div>
                    <p className="text-xs text-slate-500">
                      <span className="font-mono">{s.kode}</span>
                      {s.ue1 && <span> · {s.ue1}</span>}
                    </p>
                    <dl className="grid grid-cols-2 gap-x-3 gap-y-0.5 text-xs">
                      <dt className="text-slate-400">Aset</dt>
                      <dd className="text-right tabular-nums text-slate-700">
                        {s.jumlah_aset > 0 ? (
                          <Link href={tautanAset(s)} className="font-medium text-blue-700 hover:underline">
                            {formatNumber(s.jumlah_aset, 0)}
                          </Link>
                        ) : (
                          0
                        )}
                      </dd>
                      <dt className="text-slate-400">Paket RUP</dt>
                      <dd className="text-right tabular-nums text-slate-700">
                        {formatNumber(s.pengadaan.rup_paket, 0)}
                        {s.pengadaan.rup_pagu > 0 && <span className="text-slate-400"> · {compactRupiah(s.pengadaan.rup_pagu)}</span>}
                      </dd>
                      <dt className="text-slate-400">Tender / non-tender</dt>
                      <dd className="text-right tabular-nums text-slate-700">
                        {formatNumber(s.pengadaan.tender, 0)} / {formatNumber(s.pengadaan.non_tender, 0)}
                      </dd>
                    </dl>
                  </li>
                ))}
              </ul>
            </>
          )}

          {halamanMaks > 1 && (
            <div className="flex items-center justify-between gap-3">
              <button
                type="button"
                onClick={() => setHalaman(Math.max(1, hal - 1))}
                disabled={hal <= 1}
                className="inline-flex items-center gap-1 rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50"
              >
                <ChevronLeft className="h-4 w-4" aria-hidden="true" />
                Sebelumnya
              </button>
              <span className="text-xs text-slate-500">
                Halaman {formatNumber(hal, 0)} dari {formatNumber(halamanMaks, 0)}
              </span>
              <button
                type="button"
                onClick={() => setHalaman(Math.min(halamanMaks, hal + 1))}
                disabled={hal >= halamanMaks}
                className="inline-flex items-center gap-1 rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50"
              >
                Berikutnya
                <ChevronRight className="h-4 w-4" aria-hidden="true" />
              </button>
            </div>
          )}
          <p className="text-xs text-slate-400">
            Paket RUP, tender, dan non-tender dihitung seperti pada Dashboard Pengadaan (RUP yang masih berlaku; tender dan non-tender versi pengumuman terbaru). Satu paket bisa muncul di ketiganya,
            jadi angka-angka ini ukuran keaktifan satker, bukan total paket.
          </p>
        </section>
      </div>
    </div>
  );
}
