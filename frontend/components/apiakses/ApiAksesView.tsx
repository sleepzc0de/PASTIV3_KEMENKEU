"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Activity, BookOpen, Settings2, Users } from "lucide-react";
import { type Pengaturan, getPengaturan } from "@/lib/apiAkses";
import { adalahSuperadmin } from "@/lib/peran";
import { useDashboard } from "@/lib/dashboard-context";
import { Alert } from "@/components/ui/Alert";
import { buttonClasses } from "@/components/ui/Button";
import { SkeletonCards } from "@/components/ui/Skeleton";
import { Tabs } from "@/components/ui/Tabs";
import { DokumentasiApi } from "./DokumentasiApi";
import { KlienManager } from "./KlienManager";
import { PengaturanPanel } from "./PengaturanPanel";

type Tab = "klien" | "pengaturan" | "dokumentasi";

const TAB = [
  { key: "klien" as const, label: "Klien", icon: Users },
  { key: "pengaturan" as const, label: "Batas laju dan kuota", icon: Settings2 },
  { key: "dokumentasi" as const, label: "Dokumentasi", icon: BookOpen },
];

// Pengelolaan API Akses Data: daftar klien (persetujuan, scope, kunci), pengaturan global batas laju/kuota, dan dokumentasi pemakaian API. Khusus superadmin (backend menolak
// peran lain dengan 403; halaman ini juga menolak menampilkan isinya).
export function ApiAksesView() {
  const { profile, isLoadingProfile } = useDashboard();
  const bolehLihat = adalahSuperadmin(profile?.peran, profile?.role);
  const [tab, setTab] = useState<Tab>("klien");
  const [bawaan, setBawaan] = useState<Pengaturan | null>(null);

  useEffect(() => {
    if (!bolehLihat) return;
    getPengaturan().then((r) => setBawaan(r.pengaturan)).catch(() => setBawaan(null));
  }, [bolehLihat]);

  if (isLoadingProfile) return <SkeletonCards count={3} />;
  if (!bolehLihat) return <Alert tone="warning" message="Pengelolaan API Akses Data hanya dapat dibuka oleh superadmin." />;

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Tabs tabs={TAB} value={tab} onChange={setTab} label="Pengelolaan API Akses Data" idPrefix="apiakses" />
        <Link href="/dashboard/api-akses/monitoring" className={buttonClasses("soft", "md")}>
          <Activity className="h-4 w-4" aria-hidden="true" />
          Pemantauan penarikan data
        </Link>
      </div>
      {tab === "klien" && <KlienManager bawaan={bawaan} />}
      {tab === "pengaturan" && <PengaturanPanel onTersimpan={setBawaan} />}
      {tab === "dokumentasi" && <DokumentasiApi />}
    </div>
  );
}
