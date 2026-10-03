"use client";

import { KeyboardEvent, useCallback, useMemo, useRef, useState } from "react";
import { BarChart3, Map, RefreshCw, Table2 } from "lucide-react";
import { DGDatasetKey } from "@/lib/api";
import { useDashboard } from "@/lib/dashboard-context";
import { DigitalisasiData, DataPreset } from "./DigitalisasiData";
import { DigitalisasiOverview } from "./DigitalisasiOverview";
import { MapPanel } from "./MapPanel";
import type { MapFocus } from "./MapView";
import { RecordDetailModal } from "./RecordDetailModal";
import { SyncPanel } from "./SyncPanel";
import { useDGOverview, useDGSync } from "./useDigitalisasi";

type TabKey = "ringkasan" | "peta" | "data" | "sinkronisasi";

const TABS: { key: TabKey; label: string; Icon: typeof Map }[] = [
  { key: "ringkasan", label: "Ringkasan", Icon: BarChart3 },
  { key: "peta", label: "Peta", Icon: Map },
  { key: "data", label: "Data", Icon: Table2 },
  { key: "sinkronisasi", label: "Sinkronisasi", Icon: RefreshCw },
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
  const tabRefs = useRef<Record<TabKey, HTMLButtonElement | null>>({ ringkasan: null, peta: null, data: null, sinkronisasi: null });

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

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const i = TABS.findIndex((t) => t.key === tab);
    let next = -1;
    if (e.key === "ArrowRight") next = (i + 1) % TABS.length;
    else if (e.key === "ArrowLeft") next = (i - 1 + TABS.length) % TABS.length;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = TABS.length - 1;
    if (next < 0) return;
    e.preventDefault();
    select(TABS[next].key);
    tabRefs.current[TABS[next].key]?.focus();
  };

  return (
    <div className="space-y-5">
      <div role="tablist" aria-label="Bagian Digitalisasi Aset" onKeyDown={onKeyDown} className="flex gap-0.5 overflow-x-auto border-b border-slate-200 sm:gap-1">
        {TABS.map(({ key, label, Icon }) => {
          const selected = tab === key;
          return (
            <button
              key={key}
              ref={(el) => {
                tabRefs.current[key] = el;
              }}
              type="button"
              role="tab"
              id={`dg-tab-${key}`}
              aria-selected={selected}
              aria-controls={`dg-panel-${key}`}
              tabIndex={selected ? 0 : -1}
              onClick={() => select(key)}
              className={`inline-flex shrink-0 items-center gap-1.5 border-b-2 px-2.5 py-2 text-sm font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 sm:px-3 ${
                selected ? "border-blue-600 text-blue-700" : "border-transparent text-slate-500 hover:text-slate-700"
              }`}
            >
              {/* Ikon disembunyikan di layar sempit supaya keempat tab muat tanpa menggulir. */}
              <Icon className="hidden h-4 w-4 sm:block" aria-hidden="true" />
              {label}
              {key === "sinkronisasi" && sync.data?.aktif && <span className="h-2 w-2 animate-pulse rounded-full bg-blue-500" aria-label="sedang berjalan" />}
            </button>
          );
        })}
      </div>

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
