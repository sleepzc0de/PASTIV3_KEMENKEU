"use client";

import { Suspense, useCallback, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Building2, LayoutDashboard, Link2, ShoppingCart } from "lucide-react";
import { DasborAset } from "@/components/dashboard/DasborAset";
import { DasborSatker } from "@/components/dashboard/DasborSatker";
import { useDashboard } from "@/lib/dashboard-context";
import { bolehLihatPengadaan } from "@/lib/peran";
import { DasborWorkspace } from "@/components/pengadaan/DasborWorkspace";
import { PageShell } from "@/components/ui/PageHeader";
import { Tabs } from "@/components/ui/Tabs";

type KunciTab = "aset" | "pengadaan" | "satker";

const TABS: { key: KunciTab; label: string; icon: typeof Building2 }[] = [
  { key: "aset", label: "Aset", icon: Building2 },
  { key: "pengadaan", label: "Pengadaan", icon: ShoppingCart },
  { key: "satker", label: "Satker", icon: Link2 },
];

const dariQuery = (v: string | null): KunciTab => (v === "pengadaan" || v === "satker" ? v : "aset");

// Dashboard: dua dasbor analitik yang saling terpisah dan satu tab yang menghubungkannya. Aset dihitung dari data Digitalisasi Aset (disalin dari SLDK);
// Pengadaan dari data Pengadaan Terpadu (disalin dari Inaproc); Satker menampilkan aset dan pengadaan per satker (dihubungkan lewat kode satker 6 digit).
// Tab dipilih lewat ?tab= supaya bisa dibagikan dan tombol Kembali bekerja.
function IsiDashboard() {
  const router = useRouter();
  const params = useSearchParams();
  // Pengadaan dibatasi per satker di backend (kd_satker_str): peran UE1/Kanwil/Satker melihat pengadaan satkernya. Yang disembunyikan hanya bagi pengguna yang belum diberi peran
  // (API-nya pasti menolak).
  const { profile, isLoadingProfile } = useDashboard();
  const bolehPengadaan = !isLoadingProfile && bolehLihatPengadaan(profile?.peran);
  const tanpaPeran = Boolean(profile?.peran?.wajib && !profile.peran.peran && profile.role === "user");
  const [tab, setTab] = useState<KunciTab>(dariQuery(params.get("tab")));
  // Tab dimuat saat pertama dibuka lalu tetap terpasang (disembunyikan): pindah tab tidak menghitung ulang dan filter dasbor tidak hilang.
  const [dibuka, setDibuka] = useState<Record<KunciTab, boolean>>({
    aset: true,
    pengadaan: dariQuery(params.get("tab")) === "pengadaan",
    satker: dariQuery(params.get("tab")) === "satker",
  });

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
      router.replace(t === "aset" ? "/dashboard" : `/dashboard?tab=${t}`, { scroll: false });
    },
    [router]
  );

  const aktif: KunciTab = tab === "pengadaan" && !bolehPengadaan ? "aset" : tab;

  return (
    <div className="space-y-5">
      {tanpaPeran && (
        <p role="status" className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
          Akun Anda belum diberi peran data, sehingga belum ada data yang dapat ditampilkan. Hubungi administrator untuk diberi peran (Pengguna Barang, UE1, Kanwil, atau Satker).
        </p>
      )}
      <Tabs tabs={TABS.filter((t) => t.key !== "pengadaan" || bolehPengadaan)} value={aktif} onChange={pilih} label="Dashboard" idPrefix="db" />
      <div role="tabpanel" id="db-panel-aset" aria-labelledby="db-tab-aset" hidden={aktif !== "aset"}>
        <DasborAset />
      </div>
      {dibuka.pengadaan && bolehPengadaan && (
        <div role="tabpanel" id="db-panel-pengadaan" aria-labelledby="db-tab-pengadaan" hidden={aktif !== "pengadaan"}>
          <DasborWorkspace />
        </div>
      )}
      {dibuka.satker && (
        <div role="tabpanel" id="db-panel-satker" aria-labelledby="db-tab-satker" hidden={aktif !== "satker"}>
          <DasborSatker />
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
      description="Gambaran aset dan pengadaan dalam satu tempat, lengkap dengan wawasan analitik. Aset dibaca dari data Digitalisasi Aset (disalin dari SLDK); Pengadaan dari data Pengadaan Terpadu (disalin dari Inaproc); Satker menghubungkan keduanya per satuan kerja."
      bare
    >
      <Suspense fallback={<div role="status" aria-label="Memuat dashboard" className="h-64 animate-pulse rounded-2xl bg-slate-100" />}>
        <IsiDashboard />
      </Suspense>
    </PageShell>
  );
}
