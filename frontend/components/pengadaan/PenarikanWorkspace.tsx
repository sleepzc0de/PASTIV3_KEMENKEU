"use client";

import { useCallback, useState } from "react";
import { CalendarClock, DatabaseBackup, History, RefreshCw, Table2 } from "lucide-react";
import { PenarikanOtomatis, PenarikanPengaturan } from "@/lib/api";
import { useDashboard } from "@/lib/dashboard-context";
import { kalimatJadwal } from "@/lib/pengadaan";
import { Alert } from "@/components/ui/Alert";
import { Tabs } from "@/components/ui/Tabs";
import { DataEksporPanel } from "./DataEksporPanel";
import { JadwalPanel } from "./JadwalPanel";
import { PanelKemajuan } from "./PanelKemajuan";
import { RiwayatPanel } from "./RiwayatPanel";
import { DaftarStatusTransaksi, TarikPanel } from "./TarikPanel";
import { usePenarikan } from "./usePengadaan";

type KunciTab = "tarik" | "data" | "riwayat" | "jadwal";

const TABS: { key: KunciTab; label: string; icon: typeof Table2 }[] = [
  { key: "tarik", label: "Tarik Data", icon: DatabaseBackup },
  { key: "data", label: "Data & Ekspor", icon: Table2 },
  { key: "riwayat", label: "Riwayat", icon: History },
  { key: "jadwal", label: "Jadwal Otomatis", icon: CalendarClock },
];

// Satu halaman untuk semua penarikan data Pengadaan, Tender, E-Katalog V5, dan E-Katalog V6: penarikan manual, penarikan otomatis, tampilan
// dan ekspor data (Excel, CSV, PDF), dan riwayat.
export function PenarikanWorkspace() {
  const { profile } = useDashboard();
  const isAdmin = profile ? ["admin", "superadmin"].includes(profile.role) : false;
  const [tab, setTab] = useState<KunciTab>("tarik");
  const [dikunjungi, setDikunjungi] = useState<Record<KunciTab, boolean>>({ tarik: true, data: false, riwayat: false, jadwal: false });
  // Dinaikkan tiap penarikan selesai supaya tampilan data dan riwayat memuat yang baru.
  const [versi, setVersi] = useState(0);
  const onSelesai = useCallback(() => setVersi((v) => v + 1), []);
  const penarikan = usePenarikan(onSelesai);
  const { data: status, aktif, kuota } = penarikan;

  const pilih = useCallback((k: KunciTab) => {
    setTab(k);
    setDikunjungi((v) => (v[k] ? v : { ...v, [k]: true }));
  }, []);

  const disimpan = useCallback(
    (_p: PenarikanPengaturan, _o: PenarikanOtomatis) => {
      penarikan.reload();
    },
    [penarikan]
  );

  if (!status) {
    if (penarikan.error) {
      return (
        <div className="space-y-3">
          <Alert message={penarikan.error} />
          <button type="button" onClick={penarikan.reload} className="text-sm font-medium text-blue-600 hover:text-blue-700">
            Coba lagi
          </button>
        </div>
      );
    }
    return <div role="status" aria-label="Memuat status penarikan data" className="h-48 animate-pulse rounded-2xl bg-slate-100" />;
  }

  return (
    <div className="space-y-5">
      <DaftarStatusTransaksi />
      {penarikan.error && <Alert message={`${penarikan.error}. Menampilkan status yang dimuat sebelumnya.`} />}

      <p className="flex items-start gap-2 rounded-xl bg-slate-50 px-4 py-2.5 text-xs text-slate-600">
        <RefreshCw className="mt-0.5 h-3.5 w-3.5 shrink-0 text-slate-400" aria-hidden="true" />
        <span>{kalimatJadwal(status.otomatis, status.pengaturan)}</span>
      </p>

      {aktif && <PanelKemajuan aktif={aktif} isAdmin={isAdmin} onDibatalkan={penarikan.reload} />}

      <Tabs
        tabs={TABS.map((t) => (t.key === "riwayat" && aktif ? { ...t, badge: <span className="h-2 w-2 animate-pulse rounded-full bg-blue-500" aria-label="sedang berjalan" /> } : t))}
        value={tab}
        onChange={pilih}
        label="Bagian Penarikan Data"
        idPrefix="pt"
      />

      <div role="tabpanel" id="pt-panel-tarik" aria-labelledby="pt-tab-tarik" hidden={tab !== "tarik"}>
        <TarikPanel status={status} aktif={aktif} kuota={kuota ?? status.kuota} isAdmin={isAdmin} onMulai={penarikan.setAktif} />
      </div>
      {dikunjungi.data && (
        <div role="tabpanel" id="pt-panel-data" aria-labelledby="pt-tab-data" hidden={tab !== "data"}>
          <DataEksporPanel status={status} versi={versi} />
        </div>
      )}
      {dikunjungi.riwayat && (
        <div role="tabpanel" id="pt-panel-riwayat" aria-labelledby="pt-tab-riwayat" hidden={tab !== "riwayat"}>
          <RiwayatPanel status={status} isAdmin={isAdmin} versi={versi} />
        </div>
      )}
      {dikunjungi.jadwal && (
        <div role="tabpanel" id="pt-panel-jadwal" aria-labelledby="pt-tab-jadwal" hidden={tab !== "jadwal"}>
          <JadwalPanel status={status} isAdmin={isAdmin} onDisimpan={disimpan} />
        </div>
      )}
    </div>
  );
}
