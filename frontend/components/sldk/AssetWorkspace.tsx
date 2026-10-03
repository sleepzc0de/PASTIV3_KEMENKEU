"use client";

import { useCallback, useEffect, useState } from "react";
import { BarChart3, Search, ShieldAlert } from "lucide-react";
import { getSLDKReferences, SLDKReferences } from "@/lib/api";
import { useDashboard } from "@/lib/dashboard-context";
import { Tabs } from "@/components/ui/Tabs";
import { AssetSearch } from "./AssetSearch";
import { AssetOverview } from "./AssetOverview";
import { AssetMonitoring } from "./AssetMonitoring";
import { SearchPreset } from "./overview";
import { useOverview } from "./useOverview";

type TabKey = "cari" | "ringkasan" | "pemantauan";

const TABS: { key: TabKey; label: string; icon: typeof Search }[] = [
  { key: "cari", label: "Pencarian", icon: Search },
  { key: "ringkasan", label: "Ringkasan", icon: BarChart3 },
  { key: "pemantauan", label: "Pemantauan", icon: ShieldAlert },
];

export function AssetWorkspace() {
  const { profile } = useDashboard();
  const isAdmin = profile ? ["admin", "superadmin"].includes(profile.role) : false;

  const [tab, setTab] = useState<TabKey>("cari");
  // Tab dimuat saat pertama dibuka, lalu tetap terpasang (disembunyikan) agar hasil pencarian dan pilihan tidak hilang.
  const [visited, setVisited] = useState<Record<TabKey, boolean>>({ cari: true, ringkasan: false, pemantauan: false });
  const [refs, setRefs] = useState<SLDKReferences | null>(null);
  const [preset, setPreset] = useState<SearchPreset | undefined>();

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


  return (
    <div className="space-y-5">
      <Tabs tabs={TABS} value={tab} onChange={select} label="Bagian Data Aset" idPrefix="aset" />

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
