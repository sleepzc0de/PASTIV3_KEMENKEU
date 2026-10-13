"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import axios from "axios";
import { ChevronLeft, ChevronRight, CircleX, ListFilter, RefreshCw, Search, ServerCrash, TriangleAlert } from "lucide-react";
import { type BarisLog, type DaftarLog, type OpsiLog, type ResRingkasanLog, getServerLog, getServerLogOpsi, getServerLogRingkasan } from "@/lib/logServer";
import { RENTANG_AUDIT, type RentangKunci, batasWaktu, galatRentang, teksRelatif, waktuSingkatWIB, waktuWIB } from "@/lib/auditFilter";
import {
  FILTER_BAWAAN_LOG,
  type FilterLog,
  type LevelFilter,
  PER_HALAMAN_LOG,
  catatanKetahanan,
  catatanPerekam,
  filterLogBerubah,
  kueriLog,
  ringkasPesan,
  teksJumlah,
  teksPenyimpanan,
} from "@/lib/logServerFilter";
import { adalahSuperadmin } from "@/lib/peran";
import { useDashboard } from "@/lib/dashboard-context";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { ChartCard, DataTable, StatTile } from "@/components/ui/charts";
import { EmptyState } from "@/components/ui/EmptyState";
import { SkeletonCards, SkeletonTable } from "@/components/ui/Skeleton";
import { Tabs } from "@/components/ui/Tabs";
import { GrafikPerJam } from "./GrafikPerJam";
import { LogDetail } from "./LogDetail";
import { LencanaJumlah, LencanaLevel } from "./lencana";

const TAB_LEVEL = [
  { key: "semua" as const, label: "Semua", icon: ListFilter },
  { key: "error" as const, label: "Error", icon: CircleX },
  { key: "warn" as const, label: "Warning", icon: TriangleAlert },
];

const inputCls =
  "w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition-all placeholder:text-slate-400 hover:border-slate-400 focus:border-blue-500 focus:shadow-glow";

function pesanGalat(err: unknown, cadangan: string): string {
  return axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : cadangan;
}

// Log server: hanya galat (error) dan peringatan (warning) dari backend. Khusus superadmin (backend menolak peran lain dengan 403; halaman ini juga menolak menampilkan
// isinya). Hanya membaca: baris tidak bisa diubah atau dihapus dari aplikasi.
export function LogServerView() {
  const { profile, isLoadingProfile } = useDashboard();
  const bolehLihat = adalahSuperadmin(profile?.peran, profile?.role);

  const [filter, setFilter] = useState<FilterLog>(FILTER_BAWAAN_LOG);
  const [teks, setTeks] = useState("");
  const [opsi, setOpsi] = useState<OpsiLog | null>(null);
  const [daftar, setDaftar] = useState<DaftarLog | null>(null);
  const [ringkasan, setRingkasan] = useState<ResRingkasanLog | null>(null);
  const [galat, setGalat] = useState("");
  const [galatRingkasan, setGalatRingkasan] = useState("");
  const [memuat, setMemuat] = useState(false);
  const [terpilih, setTerpilih] = useState<BarisLog | null>(null);
  const [otomatis, setOtomatis] = useState(false);
  const [versi, setVersi] = useState(0);
  const [sekarang, setSekarang] = useState(() => new Date());
  const pertama = useRef(true);

  const galatTanggal = galatRentang(filter);

  // Kata kunci diketik bebas, lalu diterapkan ke filter setelah jeda supaya tidak meminta data pada setiap ketukan.
  useEffect(() => {
    if (pertama.current) {
      pertama.current = false;
      return;
    }
    const t = setTimeout(() => setFilter((f) => (f.q === teks ? f : { ...f, q: teks, halaman: 1 })), 400);
    return () => clearTimeout(t);
  }, [teks]);

  // Ringkasan tidak bergantung pada filter; dimuat ulang tiap versi (Segarkan / otomatis).
  useEffect(() => {
    if (!bolehLihat) return;
    const ac = new AbortController();
    getServerLogRingkasan(ac.signal)
      .then((r) => {
        setRingkasan(r.data);
        setGalatRingkasan("");
      })
      .catch((err) => {
        if (axios.isCancel(err)) return;
        setGalatRingkasan(pesanGalat(err, "Gagal memuat ringkasan log server"));
      });
    return () => ac.abort();
  }, [bolehLihat, versi]);

  // Pilihan sumber untuk filter; dimuat ulang saat versi berubah karena sumber baru bisa muncul.
  useEffect(() => {
    if (!bolehLihat) return;
    getServerLogOpsi().then((r) => setOpsi(r.data)).catch(() => setOpsi(null));
  }, [bolehLihat, versi]);

  // Daftar: dimuat ulang saat filter berubah. Permintaan lama dibatalkan supaya jawaban yang telat tidak menimpa yang baru.
  useEffect(() => {
    if (!bolehLihat || galatTanggal) return;
    const ac = new AbortController();
    setMemuat(true);
    const now = new Date();
    setSekarang(now);
    getServerLog(kueriLog(filter, batasWaktu(filter, now), PER_HALAMAN_LOG), ac.signal)
      .then((r) => {
        setDaftar(r.data);
        setGalat("");
      })
      .catch((err) => {
        if (axios.isCancel(err)) return;
        setGalat(pesanGalat(err, "Gagal memuat log server"));
      })
      .finally(() => {
        if (!ac.signal.aborted) setMemuat(false);
      });
    return () => ac.abort();
  }, [bolehLihat, filter, galatTanggal, versi]);

  useEffect(() => {
    if (!bolehLihat || !otomatis) return;
    const t = setInterval(() => {
      if (!document.hidden) setVersi((v) => v + 1);
    }, 30_000);
    return () => clearInterval(t);
  }, [bolehLihat, otomatis]);

  const ubah = useCallback((p: Partial<FilterLog>) => setFilter((f) => ({ ...f, ...p, halaman: p.halaman ?? 1 })), []);
  const aturUlang = () => {
    setFilter(FILTER_BAWAAN_LOG);
    setTeks("");
  };

  if (isLoadingProfile) return <SkeletonCards count={3} />;
  if (!bolehLihat) return <Alert tone="warning" message="Log server hanya dapat dibuka oleh superadmin." />;

  const jumlahHalaman = daftar ? Math.max(1, Math.ceil(daftar.total / daftar.per_halaman)) : 1;
  const berubah = filterLogBerubah(filter);
  const r = ringkasan?.ringkasan;
  const catatan = ringkasan ? [...catatanPerekam(ringkasan.perekam, ringkasan.pengaturan), ...catatanKetahanan(ringkasan.ketahanan)] : [];

  return (
    <div className="space-y-5">
      <p className="rounded-xl border border-blue-100 bg-blue-50/60 px-4 py-3 text-sm text-slate-700">
        Hanya <b>galat (error)</b> dan <b>peringatan (warning)</b> dari server yang disimpan di sini. Catatan info dan debug serta log akses setiap permintaan <b>tidak</b> disimpan, supaya
        log tidak memberatkan server. Kejadian yang sama yang berulang digabung menjadi satu baris dengan hitungan. Waktu ditampilkan dalam WIB.
        {ringkasan ? ` ${teksPenyimpanan(ringkasan.pengaturan)}` : ""}
      </p>

      {catatan.map((c) => (
        <Alert key={c.teks} tone={c.tingkat === "warning" ? "warning" : "info"} message={c.teks} />
      ))}

      <div className="flex flex-wrap items-end gap-3">
        <div className="min-w-[11rem]">
          <label htmlFor="log-rentang" className="mb-1 block text-xs font-medium text-slate-600">
            Rentang waktu
          </label>
          <select id="log-rentang" value={filter.rentang} onChange={(e) => ubah({ rentang: e.target.value as RentangKunci })} className={inputCls}>
            {RENTANG_AUDIT.map((x) => (
              <option key={x.kunci} value={x.kunci}>
                {x.label}
              </option>
            ))}
          </select>
        </div>
        {filter.rentang === "kustom" && (
          <>
            <div>
              <label htmlFor="log-dari" className="mb-1 block text-xs font-medium text-slate-600">
                Dari tanggal
              </label>
              <input id="log-dari" type="date" value={filter.dari} max={filter.sampai || undefined} onChange={(e) => ubah({ dari: e.target.value })} className={inputCls} />
            </div>
            <div>
              <label htmlFor="log-sampai" className="mb-1 block text-xs font-medium text-slate-600">
                Sampai tanggal
              </label>
              <input id="log-sampai" type="date" value={filter.sampai} min={filter.dari || undefined} onChange={(e) => ubah({ sampai: e.target.value })} className={inputCls} />
            </div>
          </>
        )}
        <div className="ml-auto flex flex-wrap items-center gap-3">
          <label className="flex cursor-pointer items-center gap-2 text-xs text-slate-600">
            <input type="checkbox" checked={otomatis} onChange={(e) => setOtomatis(e.target.checked)} className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" />
            Segarkan otomatis (30 detik)
          </label>
          <Button size="sm" variant="secondary" fullWidth={false} icon={<RefreshCw className="h-4 w-4" aria-hidden="true" />} onClick={() => setVersi((v) => v + 1)}>
            Segarkan
          </Button>
        </div>
      </div>
      {galatTanggal && <Alert tone="warning" message={galatTanggal} />}

      {galatRingkasan && <Alert message={galatRingkasan} />}
      {!r && !galatRingkasan && <SkeletonCards count={3} />}
      {r && ringkasan && (
        <>
          <div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6">
            <StatTile label="Error (24 jam)" value={r.error_24_jam.toLocaleString("id-ID")} sub={`${r.error_7_hari.toLocaleString("id-ID")} dalam 7 hari`} />
            <StatTile label="Warning (24 jam)" value={r.warn_24_jam.toLocaleString("id-ID")} sub={`${r.warn_7_hari.toLocaleString("id-ID")} dalam 7 hari`} />
            <StatTile label="Jenis kejadian (24 jam)" value={r.kelompok_24_jam.toLocaleString("id-ID")} sub="kejadian serupa digabung" />
            <StatTile label="Kejadian terakhir" value={r.terakhir ? teksRelatif(r.terakhir, sekarang) : "-"} sub={r.terakhir ? `${waktuWIB(r.terakhir)} WIB` : "belum ada catatan"} />
            <StatTile label="Baris tersimpan" value={r.total_baris.toLocaleString("id-ID")} sub={`maks ${ringkasan.pengaturan.maks_baris.toLocaleString("id-ID")}`} />
            <StatTile
              label="Puncak serentak"
              value={`${ringkasan.ketahanan.serentak_puncak.toLocaleString("id-ID")}`}
              sub={`batas ${ringkasan.ketahanan.maks_serentak.toLocaleString("id-ID")} permintaan sekaligus`}
            />
          </div>

          <div className="grid gap-4 lg:grid-cols-3">
            <div className="lg:col-span-2">
              <ChartCard
                title="Kejadian per jam"
                subtitle="24 jam terakhir, error dan warning"
                table={
                  <DataTable
                    columns={["Jam (WIB)", "Error", "Warning"]}
                    rows={r.per_jam.map((t) => [waktuSingkatWIB(t.jam, true), t.error.toLocaleString("id-ID"), t.warn.toLocaleString("id-ID")])}
                  />
                }
              >
                <GrafikPerJam titik={r.per_jam} />
              </ChartCard>
            </div>
            <ChartCard title="Sumber teratas" subtitle="24 jam terakhir; klik untuk menyaring">
              {r.sumber.length === 0 ? (
                <p className="py-6 text-center text-sm text-slate-400">Tidak ada data</p>
              ) : (
                <ul className="space-y-1.5">
                  {r.sumber.map((s) => (
                    <li key={`${s.sumber}-${s.level}`}>
                      <button
                        type="button"
                        onClick={() => ubah({ sumber: s.sumber, level: s.level })}
                        className="flex w-full items-center justify-between gap-2 rounded-lg px-2 py-1.5 text-left text-sm hover:bg-slate-50"
                      >
                        <span className="flex min-w-0 items-center gap-2">
                          <LencanaLevel level={s.level} />
                          <span className="truncate text-slate-800">{s.sumber}</span>
                        </span>
                        <span className="shrink-0 text-xs font-semibold tabular-nums text-slate-600">{s.jumlah.toLocaleString("id-ID")}</span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </ChartCard>
          </div>

          {r.paling_berulang.length > 0 && (
            <ChartCard title="Paling sering berulang" subtitle="Kejadian dengan hitungan terbesar dalam 24 jam; biasanya akar masalah yang paling layak diperbaiki lebih dulu">
              <ul className="divide-y divide-slate-100">
                {r.paling_berulang.map((b) => (
                  <li key={b.id}>
                    <button type="button" onClick={() => setTerpilih(b)} className="flex w-full items-start gap-3 px-1 py-2.5 text-left hover:bg-slate-50">
                      <LencanaLevel level={b.level} />
                      <span className="min-w-0 flex-1">
                        <span className="block break-words text-sm text-slate-900">{ringkasPesan(b.pesan, 140)}</span>
                        <span className="block text-xs text-slate-500">
                          {b.sumber} · terakhir {teksRelatif(b.waktu_akhir, sekarang)}
                        </span>
                      </span>
                      <LencanaJumlah teks={teksJumlah(b.jumlah)} />
                    </button>
                  </li>
                ))}
              </ul>
            </ChartCard>
          )}
        </>
      )}

      <div className="flex flex-wrap items-center justify-between gap-3">
        <Tabs tabs={TAB_LEVEL} value={filter.level} onChange={(k) => ubah({ level: k as LevelFilter })} label="Tingkat log" idPrefix="log-level" />
      </div>

      <div className="flex flex-wrap items-end gap-3">
        <div className="min-w-[12rem]">
          <label htmlFor="log-sumber" className="mb-1 block text-xs font-medium text-slate-600">
            Sumber
          </label>
          <select id="log-sumber" value={filter.sumber} onChange={(e) => ubah({ sumber: e.target.value })} className={inputCls}>
            <option value="">Semua sumber</option>
            {filter.sumber && !opsi?.sumber.includes(filter.sumber) && <option value={filter.sumber}>{filter.sumber}</option>}
            {opsi?.sumber.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </div>
        <div className="relative min-w-[14rem] flex-1 sm:max-w-md">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
          <input
            type="search"
            value={teks}
            onChange={(e) => setTeks(e.target.value)}
            maxLength={100}
            placeholder="Cari pesan, rute, atau ID permintaan"
            aria-label="Cari di log server"
            className="w-full rounded-xl border border-slate-300 bg-white py-2.5 pl-9 pr-3 text-sm outline-none focus:border-blue-500 focus:shadow-glow"
          />
        </div>
        {berubah && (
          <button type="button" onClick={aturUlang} className="pb-2 text-sm font-medium text-blue-700 hover:underline">
            Atur ulang filter
          </button>
        )}
      </div>

      {galat && <Alert message={galat} />}
      {!daftar && !galat && !galatTanggal && <SkeletonTable rows={8} cols={5} />}
      {daftar && (
        <>
          <p className="text-sm text-slate-600" aria-live="polite">
            <span className="font-semibold text-slate-900">{daftar.total.toLocaleString("id-ID")}</span> baris log
            {memuat && <span className="ml-2 text-xs text-slate-400">memuat…</span>}
            {daftar.total > 0 && (
              <span className="text-slate-400">
                {" "}
                · halaman {daftar.halaman} dari {jumlahHalaman.toLocaleString("id-ID")}
              </span>
            )}
          </p>
          {daftar.baris.length === 0 ? (
            <EmptyState
              icon={ServerCrash}
              title={berubah ? "Tidak ada log yang cocok" : "Tidak ada error atau warning"}
              description={berubah ? "Ubah atau atur ulang filter." : "Tidak ada galat atau peringatan tercatat pada rentang waktu ini. Itu kabar baik."}
            />
          ) : (
            <div className={`overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm ${memuat ? "opacity-70" : ""}`}>
              <table className="w-full min-w-[52rem] text-sm">
                <thead className="bg-slate-50 text-xs text-slate-500">
                  <tr>
                    <th scope="col" className="px-4 py-2.5 text-left font-medium">Terakhir (WIB)</th>
                    <th scope="col" className="px-4 py-2.5 text-left font-medium">Level</th>
                    <th scope="col" className="px-4 py-2.5 text-left font-medium">Sumber</th>
                    <th scope="col" className="px-4 py-2.5 text-left font-medium">Pesan</th>
                    <th scope="col" className="px-4 py-2.5 text-right font-medium">Kejadian</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {daftar.baris.map((b) => (
                    <tr
                      key={b.id}
                      tabIndex={0}
                      role="button"
                      aria-label={`Lihat rincian: ${ringkasPesan(b.pesan, 80)}`}
                      onClick={() => setTerpilih(b)}
                      onKeyDown={(ev) => {
                        if (ev.key === "Enter" || ev.key === " ") {
                          ev.preventDefault();
                          setTerpilih(b);
                        }
                      }}
                      className="cursor-pointer outline-none hover:bg-slate-50 focus-visible:bg-blue-50/60"
                    >
                      <td className="whitespace-nowrap px-4 py-2.5 text-xs tabular-nums text-slate-600">
                        <p>{waktuWIB(b.waktu_akhir)}</p>
                        <p className="text-slate-400">{teksRelatif(b.waktu_akhir, sekarang)}</p>
                      </td>
                      <td className="px-4 py-2.5">
                        <LencanaLevel level={b.level} />
                      </td>
                      <td className="max-w-[12rem] px-4 py-2.5">
                        <p className="truncate font-medium text-slate-800" title={b.sumber}>
                          {b.sumber}
                        </p>
                        {b.rute && (
                          <p className="truncate font-mono text-[11px] text-slate-400" title={`${b.metode ?? ""} ${b.rute}`}>
                            {b.metode} {b.rute}
                            {b.status ? ` · ${b.status}` : ""}
                          </p>
                        )}
                      </td>
                      <td className="max-w-[30rem] px-4 py-2.5">
                        <p className="line-clamp-2 break-words text-slate-900">{ringkasPesan(b.pesan)}</p>
                      </td>
                      <td className="whitespace-nowrap px-4 py-2.5 text-right">
                        <LencanaJumlah teks={teksJumlah(b.jumlah)} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {daftar.total > daftar.per_halaman && (
            <nav aria-label="Halaman log server" className="flex items-center justify-between gap-3">
              <Button size="sm" variant="secondary" fullWidth={false} disabled={filter.halaman <= 1 || memuat} icon={<ChevronLeft className="h-4 w-4" aria-hidden="true" />} onClick={() => setFilter((f) => ({ ...f, halaman: f.halaman - 1 }))}>
                Lebih baru
              </Button>
              <span className="text-xs text-slate-500">
                Halaman {daftar.halaman} dari {jumlahHalaman.toLocaleString("id-ID")}
              </span>
              <Button size="sm" variant="secondary" fullWidth={false} disabled={filter.halaman >= jumlahHalaman || memuat} onClick={() => setFilter((f) => ({ ...f, halaman: f.halaman + 1 }))}>
                Lebih lama
                <ChevronRight className="h-4 w-4" aria-hidden="true" />
              </Button>
            </nav>
          )}
        </>
      )}
      <p className="text-[11px] text-slate-400">
        Terakhir dimuat {waktuWIB(sekarang.toISOString())} WIB.
        {ringkasan && ringkasan.perekam.dibatalkan_klien > 0
          ? ` ${ringkasan.perekam.dibatalkan_klien.toLocaleString("id-ID")} galat "context canceled" (pengguna menutup tab atau membatalkan permintaan) sengaja tidak dicatat karena bukan masalah server.`
          : ""}
      </p>

      {terpilih && <LogDetail baris={terpilih} onTutup={() => setTerpilih(null)} />}
    </div>
  );
}
