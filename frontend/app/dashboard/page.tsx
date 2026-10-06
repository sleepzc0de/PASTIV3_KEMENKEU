"use client";

import { Suspense, useCallback, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Building2, LayoutDashboard, ShoppingCart } from "lucide-react";
import { DasborAset } from "@/components/dashboard/DasborAset";
import { DasborWorkspace } from "@/components/pengadaan/DasborWorkspace";
import { PageShell } from "@/components/ui/PageHeader";
import { Tabs } from "@/components/ui/Tabs";

type KunciTab = "aset" | "pengadaan";

const TABS: { key: KunciTab; label: string; icon: typeof Building2 }[] = [
  { key: "aset", label: "Aset", icon: Building2 },
  { key: "pengadaan", label: "Pengadaan", icon: ShoppingCart },
];

const dariQuery = (v: string | null): KunciTab => (v === "pengadaan" ? "pengadaan" : "aset");

// Dashboard: dua dasbor analitik yang saling terpisah. Aset dihitung dari data Digitalisasi Aset (disalin dari SLDK); Pengadaan dari data Pengadaan
// Terpadu (disalin dari Inaproc). Tab dipilih lewat ?tab= supaya bisa dibagikan dan tombol Kembali bekerja.
function IsiDashboard() {
  const router = useRouter();
  const params = useSearchParams();
  const [tab, setTab] = useState<KunciTab>(dariQuery(params.get("tab")));
  // Tab dimuat saat pertama dibuka lalu tetap terpasang (disembunyikan): pindah tab tidak menghitung ulang dan filter dasbor tidak hilang.
  const [dibuka, setDibuka] = useState<Record<KunciTab, boolean>>({ aset: true, pengadaan: dariQuery(params.get("tab")) === "pengadaan" });

  // Alamat berubah dari luar (Kembali/Maju, tautan dari halaman lain): tab mengikuti.
  useEffect(() => {
    const t = dariQuery(params.get("tab"));
    setTab(t);
    setDibuka((d) => (d[t] ? d : { ...d, [t]: true }));
  }, [params]);

  const pilih = useCallback(
    (t: KunciTab) => {
      setTab(t);
      setDibuka((d) => (d[t] ? d : { ...d, [t]: true }));
      router.replace(t === "aset" ? "/dashboard" : "/dashboard?tab=pengadaan", { scroll: false });
    },
    [router]
  );

  return (
    <div className="space-y-5">
      <Tabs tabs={TABS} value={tab} onChange={pilih} label="Dashboard" idPrefix="db" />
      <div role="tabpanel" id="db-panel-aset" aria-labelledby="db-tab-aset" hidden={tab !== "aset"}>
        <DasborAset />
      </div>
      {dibuka.pengadaan && (
        <div role="tabpanel" id="db-panel-pengadaan" aria-labelledby="db-tab-pengadaan" hidden={tab !== "pengadaan"}>
          <DasborWorkspace />
        </div>
      )}
    </div>
  );
}

export default function DashboardPage() {
  return (
    <PageShell
      title="Dashboard"
      icon={LayoutDashboard}
      description="Gambaran aset dan pengadaan dalam satu tempat, lengkap dengan wawasan analitik. Dashboard Aset dibaca dari data Digitalisasi Aset (disalin dari SLDK); Dashboard Pengadaan dari data Pengadaan Terpadu (disalin dari Inaproc)."
      bare
    >
      <Suspense fallback={<div role="status" aria-label="Memuat dashboard" className="h-64 animate-pulse rounded-2xl bg-slate-100" />}>
        <IsiDashboard />
      </Suspense>
    </PageShell>
  );
}
