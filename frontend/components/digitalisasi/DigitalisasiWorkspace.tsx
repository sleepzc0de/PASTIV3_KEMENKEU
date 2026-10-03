"use client";

import { useCallback, useMemo, useState } from "react";
import { BarChart3, Map, RefreshCw, Table2 } from "lucide-react";
import { DGDatasetKey } from "@/lib/api";
import { useDashboard } from "@/lib/dashboard-context";
import { Tabs } from "@/components/ui/Tabs";
import { DigitalisasiData, DataPreset } from "./DigitalisasiData";
import { DigitalisasiOverview } from "./DigitalisasiOverview";
import { MapPanel } from "./MapPanel";
import type { MapFocus } from "./MapView";
import { RecordDetailModal } from "./RecordDetailModal";
import { SyncPanel } from "./SyncPanel";
import { useDGOverview, useDGSync } from "./useDigitalisasi";

type TabKey = "ringkasan" | "peta" | "data" | "sinkronisasi";

const TABS: { key: TabKey; label: string; icon: typeof Map }[] = [
  { key: "ringkasan", label: "Ringkasan", icon: BarChart3 },
  { key: "peta", label: "Peta", icon: Map },
  { key: "data", label: "Data", icon: Table2 },
  { key: "sinkronisasi", label: "Sinkronisasi", icon: RefreshCw },
];

export function DigitalisasiWorkspace() {
  const { profile } = useDashboard();
  const isAdmin = profile ? ["admin", "superadmin"].includes(profile.role) : false;

  const [tab, setTab] = useState<TabKey>("ringkasan");
  // Tab dimuat saat pertama dibuka lalu tetap terpasang (disembunyikan): peta dan hasil pencarian tidak hilang saat pindah tab.
  const [visited, setVisited] = useState<Record<TabKey, boolean>>({ ringkasan: true, peta: false, data: false, sinkronisasi: false });
  // Dinaikkan setiap sinkronisasi selesai supaya ringkasan, peta, dan daftar memuat data yang baru.
  const [version, setVersion] = useState(0);
  const [dataPreset, setDataPreset] = useState<DataPreset | undefined>();
  const [mapFocus, setMapFocus] = useState<MapFocus | null>(null);
  const [detail, setDetail] = useState<{ dataset: DGDatasetKey; id: number } | null>(null);

  const overview = useDGOverview(true, version);
  const onFinished = useCallback(() => setVersion((v) => v + 1), []);
  const sync = useDGSync(true, onFinished);

  const select = useCallback((key: TabKey) => {
    setTab(key);
    setVisited((v) => (v[key] ? v : { ...v, [key]: true }));
  }, []);

  const openData = useCallback(
    (p: { dataset: DGDatasetKey; tanpaKoordinat?: boolean }) => {
      setDataPreset({ ...p, nonce: Date.now() });
      select("data");
    },
    [select]
  );

  const ue1Options = useMemo(() => {
    const d = overview.data;
    return d && d.tersedia ? d.ue1.filter((u) => u.kode !== "(kosong)").map((u) => ({ value: u.kode, label: u.label })) : [];
  }, [overview.data]);


  return (
    <div className="space-y-5">
      <Tabs
        tabs={TABS.map((t) => (t.key === "sinkronisasi" && sync.data?.aktif ? { ...t, badge: <span className="h-2 w-2 animate-pulse rounded-full bg-blue-500" aria-label="sedang berjalan" /> } : t))}
        value={tab}
        onChange={select}
        label="Bagian Digitalisasi Aset"
        idPrefix="dg"
      />

      <div role="tabpanel" id="dg-panel-ringkasan" aria-labelledby="dg-tab-ringkasan" hidden={tab !== "ringkasan"}>
        <DigitalisasiOverview overview={overview} isAdmin={isAdmin} onGoSync={() => select("sinkronisasi")} onOpenData={openData} />
      </div>
      {visited.peta && (
        <div role="tabpanel" id="dg-panel-peta" aria-labelledby="dg-tab-peta" hidden={tab !== "peta"}>
          <MapPanel
            version={version}
            ue1Options={ue1Options}
            focus={mapFocus}
            onOpenDetail={(dataset, id) => setDetail({ dataset, id })}
            onOpenData={openData}
          />
        </div>
      )}
      {visited.data && (
        <div role="tabpanel" id="dg-panel-data" aria-labelledby="dg-tab-data" hidden={tab !== "data"}>
          <DigitalisasiData version={version} preset={dataPreset} onOpenDetail={(dataset, id) => setDetail({ dataset, id })} />
        </div>
      )}
      {visited.sinkronisasi && (
        <div role="tabpanel" id="dg-panel-sinkronisasi" aria-labelledby="dg-tab-sinkronisasi" hidden={tab !== "sinkronisasi"}>
          <SyncPanel sync={sync} isAdmin={isAdmin} />
        </div>
      )}

      {detail && (
        <RecordDetailModal
          dataset={detail.dataset}
          id={detail.id}
          onClose={() => setDetail(null)}
          onShowOnMap={(lat, lng) => {
            setMapFocus({ dataset: detail.dataset, id: detail.id, lat, lng, nonce: Date.now() });
            setDetail(null);
            select("peta");
          }}
        />
      )}
    </div>
  );
}
