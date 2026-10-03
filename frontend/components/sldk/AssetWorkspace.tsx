"use client";

import { KeyboardEvent, useCallback, useEffect, useRef, useState } from "react";
import { BarChart3, Search, ShieldAlert } from "lucide-react";
import { getSLDKReferences, SLDKReferences } from "@/lib/api";
import { useDashboard } from "@/lib/dashboard-context";
import { AssetSearch } from "./AssetSearch";
import { AssetOverview } from "./AssetOverview";
import { AssetMonitoring } from "./AssetMonitoring";
import { SearchPreset } from "./overview";
import { useOverview } from "./useOverview";

type TabKey = "cari" | "ringkasan" | "pemantauan";

const TABS: { key: TabKey; label: string; Icon: typeof Search }[] = [
  { key: "cari", label: "Pencarian", Icon: Search },
  { key: "ringkasan", label: "Ringkasan", Icon: BarChart3 },
  { key: "pemantauan", label: "Pemantauan", Icon: ShieldAlert },
];

export function AssetWorkspace() {
  const { profile } = useDashboard();
  const isAdmin = profile ? ["admin", "superadmin"].includes(profile.role) : false;

  const [tab, setTab] = useState<TabKey>("cari");
  // Tab dimuat saat pertama dibuka, lalu tetap terpasang (disembunyikan) agar hasil pencarian dan pilihan tidak hilang.
  const [visited, setVisited] = useState<Record<TabKey, boolean>>({ cari: true, ringkasan: false, pemantauan: false });
  const [refs, setRefs] = useState<SLDKReferences | null>(null);
  const [preset, setPreset] = useState<SearchPreset | undefined>();
  const tabRefs = useRef<Record<TabKey, HTMLButtonElement | null>>({ cari: null, ringkasan: null, pemantauan: null });

  const overview = useOverview(visited.ringkasan || visited.pemantauan);

  // Tabel referensi (kode -> nama) kecil; bila gagal, filter referensi disembunyikan dan kode ditampilkan apa adanya.
  useEffect(() => {
    let cancelled = false;
    getSLDKReferences()
      .then((res) => {
        if (!cancelled) setRefs(res.data);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);

  const select = useCallback((key: TabKey) => {
    setTab(key);
    setVisited((v) => (v[key] ? v : { ...v, [key]: true }));
  }, []);

  const openSearch = useCallback(
    (p: Omit<SearchPreset, "nonce">) => {
      setPreset({ ...p, nonce: Date.now() });
      select("cari");
    },
    [select]
  );

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
      <div role="tablist" aria-label="Bagian Data Aset" onKeyDown={onKeyDown} className="flex gap-0.5 overflow-x-auto border-b border-slate-200 sm:gap-1">
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
              id={`aset-tab-${key}`}
              aria-selected={selected}
              aria-controls={`aset-panel-${key}`}
              tabIndex={selected ? 0 : -1}
              onClick={() => select(key)}
              className={`inline-flex shrink-0 items-center gap-1.5 border-b-2 px-2.5 py-2 text-sm font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 sm:px-3 ${
                selected ? "border-blue-600 text-blue-700" : "border-transparent text-slate-500 hover:text-slate-700"
              }`}
            >
              {/* Ikon disembunyikan di layar sempit supaya ketiga tab muat tanpa menggulir. */}
              <Icon className="hidden h-4 w-4 sm:block" aria-hidden="true" />
              {label}
            </button>
          );
        })}
      </div>

      <div role="tabpanel" id="aset-panel-cari" aria-labelledby="aset-tab-cari" hidden={tab !== "cari"}>
        <AssetSearch refs={refs} preset={preset} />
      </div>
      {visited.ringkasan && (
        <div role="tabpanel" id="aset-panel-ringkasan" aria-labelledby="aset-tab-ringkasan" hidden={tab !== "ringkasan"}>
          <AssetOverview overview={overview} refs={refs} isAdmin={isAdmin} />
        </div>
      )}
      {visited.pemantauan && (
        <div role="tabpanel" id="aset-panel-pemantauan" aria-labelledby="aset-tab-pemantauan" hidden={tab !== "pemantauan"}>
          <AssetMonitoring overview={overview} refs={refs} isAdmin={isAdmin} onSearch={openSearch} />
        </div>
      )}
    </div>
  );
}
