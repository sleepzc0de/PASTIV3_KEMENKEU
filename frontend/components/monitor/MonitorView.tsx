"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import axios from "axios";
import { Activity, Database, Gauge, LineChart, RefreshCw, Server, Timer } from "lucide-react";
import { type RingkasanMonitor, getMonitorRingkasan } from "@/lib/monitor";
import { KELAS_STATUS, ringkasanSpanduk } from "@/lib/monitorFormat";
import { waktuWIB } from "@/lib/auditFilter";
import { adalahSuperadmin } from "@/lib/peran";
import { useDashboard } from "@/lib/dashboard-context";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { SkeletonCards } from "@/components/ui/Skeleton";
import { Tabs } from "@/components/ui/Tabs";
import { IkonStatus, KartuKapasitas } from "./bagian";
import { PanelAplikasi } from "./PanelAplikasi";
import { PanelDatabase } from "./PanelDatabase";
import { PanelRiwayat } from "./PanelRiwayat";
import { PanelServer } from "./PanelServer";

type Tab = "ringkasan" | "server" | "aplikasi" | "database" | "riwayat";

const TAB = [
  { key: "ringkasan" as const, label: "Ringkasan", icon: Gauge },
  { key: "server" as const, label: "Server", icon: Server },
  { key: "aplikasi" as const, label: "Aplikasi", icon: Activity },
  { key: "database" as const, label: "Database", icon: Database },
  { key: "riwayat" as const, label: "Riwayat", icon: LineChart },
];

const INTERVAL_SEGARKAN = 15_000;

// Monitor resource: apakah CPU, memori, disk, dan koneksi database masih cukup, resource mana yang perlu ditambah, dan riwayat pemakaiannya. Khusus superadmin (backend menolak
// peran lain dengan 403; halaman ini juga menolak menampilkan isinya).
export function MonitorView() {
  const { profile, isLoadingProfile } = useDashboard();
  const bolehLihat = adalahSuperadmin(profile?.peran, profile?.role);
  const [tab, setTab] = useState<Tab>("ringkasan");
  const [data, setData] = useState<RingkasanMonitor | null>(null);
  const [galat, setGalat] = useState("");
  const [otomatis, setOtomatis] = useState(true);
  const [memuat, setMemuat] = useState(false);
  const [diperbarui, setDiperbarui] = useState<string>("");
  const ac = useRef<AbortController | null>(null);

  const muat = useCallback(async () => {
    ac.current?.abort();
    const c = new AbortController();
    ac.current = c;
    setMemuat(true);
    try {
      const r = await getMonitorRingkasan(c.signal);
      setData(r.data);
      setDiperbarui(r.data.waktu);
      setGalat("");
    } catch (err) {
      if (axios.isCancel(err)) return;
      setGalat(axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : "Gagal memuat keadaan resource");
    } finally {
      if (ac.current === c) setMemuat(false);
    }
  }, []);

  useEffect(() => {
    if (!bolehLihat) return;
    void muat();
    return () => ac.current?.abort();
  }, [bolehLihat, muat]);

  useEffect(() => {
    if (!bolehLihat || !otomatis) return;
    const t = setInterval(() => {
      if (!document.hidden) void muat();
    }, INTERVAL_SEGARKAN);
    return () => clearInterval(t);
  }, [bolehLihat, otomatis, muat]);

  const spanduk = useMemo(() => (data ? ringkasanSpanduk(data.status, data.perlu_ditambah, data.kapasitas) : null), [data]);

  if (isLoadingProfile) return <SkeletonCards count={3} />;
  if (!bolehLihat) return <Alert tone="warning" message="Monitor resource hanya dapat dibuka oleh superadmin." />;

  const resource = data?.kapasitas.filter((k) => k.kelompok === "resource") ?? [];
  const kinerja = data?.kapasitas.filter((k) => k.kelompok === "kinerja") ?? [];
  const berjalan = data?.tugas.filter((t) => t.berjalan) ?? [];

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Tabs tabs={TAB} value={tab} onChange={setTab} label="Bagian monitor resource" idPrefix="monitor" />
        <div className="flex flex-wrap items-center gap-3">
          <label className="flex cursor-pointer items-center gap-2 text-xs text-slate-600">
            <input type="checkbox" checked={otomatis} onChange={(e) => setOtomatis(e.target.checked)} className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" />
            Segarkan otomatis (15 detik)
          </label>
          <Button size="sm" variant="secondary" fullWidth={false} isLoading={memuat} icon={<RefreshCw className="h-4 w-4" aria-hidden="true" />} onClick={() => void muat()}>
            Segarkan
          </Button>
        </div>
      </div>

      {galat && <Alert message={galat} />}
      {!data && !galat && <SkeletonCards count={4} />}

      {data && spanduk && (
        <>
          <section aria-live="polite" className={`flex items-start gap-3 rounded-2xl border px-4 py-3.5 ${KELAS_STATUS[data.status].kartu} ${data.status === "kritis" ? "bg-red-50/60" : data.status === "perhatian" ? "bg-amber-50/60" : "bg-white"}`}>
            <IkonStatus status={data.status} className="mt-0.5 h-5 w-5 shrink-0" />
            <div className="min-w-0">
              <p className="text-sm font-semibold text-slate-900">{spanduk.judul}</p>
              <p className="mt-0.5 text-sm text-slate-600">{spanduk.uraian}</p>
              <p className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-slate-500">
                <span className="inline-flex items-center gap-1">
                  <Timer className="h-3.5 w-3.5" aria-hidden="true" />
                  Diperbarui {waktuWIB(diperbarui)} WIB
                </span>
                {!data.pengukuran.aktif ? <span>Pengukur berkala mati (MONITOR_AKTIF=false): riwayat tidak disimpan.</span> : <span>Riwayat disimpan {data.pengukuran.retensi_hari} hari.</span>}
              </p>
            </div>
          </section>

          {berjalan.length > 0 && (
            <div className="rounded-xl border border-sky-200 bg-sky-50 px-4 py-2.5 text-sm text-sky-900">
              <span className="font-semibold">Tugas berat sedang berjalan</span>, yang wajar membuat CPU, memori, atau koneksi database melonjak:
              <ul className="mt-1 list-disc pl-5 text-xs">
                {berjalan.map((t) => (
                  <li key={t.nama}>
                    {t.nama}
                    {t.info ? ` (${t.info})` : ""}
                  </li>
                ))}
              </ul>
            </div>
          )}

          {tab === "ringkasan" && (
            <div className="space-y-6">
              <section aria-labelledby="mon-resource">
                <h2 id="mon-resource" className="text-sm font-semibold text-slate-900">
                  Resource yang bisa ditambah
                </h2>
                <p className="mt-0.5 text-xs text-slate-500">Penilaian dari pengukuran sekarang dan puncak 24 jam dan 7 hari terakhir. Buka &ldquo;alasan dan angka&rdquo; untuk melihat dasarnya.</p>
                <div className="mt-3 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                  {resource.map((k) => (
                    <KartuKapasitas key={k.kunci} k={k} />
                  ))}
                </div>
              </section>
              <section aria-labelledby="mon-kinerja">
                <h2 id="mon-kinerja" className="text-sm font-semibold text-slate-900">
                  Kinerja dan kesehatan aplikasi
                </h2>
                <p className="mt-0.5 text-xs text-slate-500">Masalah di sini belum tentu teratasi dengan menambah resource; cocokkan dengan kartu di atas.</p>
                <div className="mt-3 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                  {kinerja.map((k) => (
                    <KartuKapasitas key={k.kunci} k={k} />
                  ))}
                </div>
              </section>
            </div>
          )}
          {tab === "server" && <PanelServer data={data} />}
          {tab === "aplikasi" && <PanelAplikasi data={data} />}
          {tab === "database" && <PanelDatabase data={data} />}
        </>
      )}
      {tab === "riwayat" && <PanelRiwayat />}
    </div>
  );
}
