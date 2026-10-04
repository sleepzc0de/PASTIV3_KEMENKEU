"use client";

import { FormEvent, useMemo, useState } from "react";
import Link from "next/link";
import { BarChart3, Boxes, DatabaseBackup, FileSignature, Gavel, Loader2, RefreshCw, ShoppingCart } from "lucide-react";
import { daftarTahun, formatWaktu, waktuRelatif } from "@/lib/pengadaan";
import { Alert } from "@/components/ui/Alert";
import { Tabs } from "@/components/ui/Tabs";
import { BagianEkatalog, BagianKontrak, BagianPemilihan, BagianRUP, Ikhtisar } from "./DasborBagian";
import { KELAS_INPUT } from "./lencana";
import { KartuKosong } from "./dasborUtil";
import { useAnalitik } from "./usePengadaan";

type KunciTab = "ikhtisar" | "rup" | "pemilihan" | "kontrak" | "ekatalog";

const TABS: { key: KunciTab; label: string; icon: typeof BarChart3 }[] = [
  { key: "ikhtisar", label: "Ikhtisar", icon: BarChart3 },
  { key: "rup", label: "Perencanaan", icon: ShoppingCart },
  { key: "pemilihan", label: "Pemilihan", icon: Gavel },
  { key: "kontrak", label: "Kontrak", icon: FileSignature },
  { key: "ekatalog", label: "E-Katalog", icon: Boxes },
];

// Dasbor Pengadaan terpadu: perencanaan (RUP), pemilihan (tender dan non-tender), kontrak, dan e-purchasing yang saling terhubung lewat
// kode RUP, lengkap dengan wawasan analitik berbasis angka. Data dibaca dari salinan lokal hasil penarikan.
export function DasborWorkspace() {
  const [tab, setTab] = useState<KunciTab>("ikhtisar");
  const [tahunPilih, setTahunPilih] = useState("");
  const [klpdInput, setKlpdInput] = useState("K10");
  const [klpd, setKlpd] = useState("K10");
  const analitik = useAnalitik(tahunPilih, klpd, 0);
  const { data, isLoading, error } = analitik;

  const hasil = data?.hasil ?? null;
  const tahunAktif = hasil?.tahun ?? tahunPilih;
  const pilihanTahun = useMemo(() => {
    const dasar = data?.tahun_tersedia ?? [];
    const sekarang = new Date().getFullYear();
    return Array.from(new Set([...dasar, ...(dasar.length === 0 ? daftarTahun(sekarang, 3) : [])])).sort((a, b) => Number(b) - Number(a));
  }, [data?.tahun_tersedia]);

  const terapkanKlpd = (e: FormEvent) => {
    e.preventDefault();
    const k = klpdInput.trim();
    if (k && k !== klpd) setKlpd(k);
  };

  const kosongSemua = hasil !== null && !hasil.rup?.total_paket && !hasil.pemilihan?.tender_jumlah && !hasil.pemilihan?.non_tender_jumlah && !hasil.kontrak?.tender_jumlah && !hasil.kontrak?.non_tender_jumlah && !hasil.ekatalog?.v5.paket && !hasil.ekatalog?.v6.order && !hasil.ekatalog?.v6.transaksi_baris;

  return (
    <div className="space-y-5">
      <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm sm:p-5">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div className="flex flex-wrap items-end gap-3">
            <div className="w-32">
              <label htmlFor="ds-tahun" className="mb-1 block text-xs font-medium text-slate-600">
                Tahun anggaran
              </label>
              <select id="ds-tahun" value={tahunAktif} onChange={(e) => setTahunPilih(e.target.value)} className={KELAS_INPUT} disabled={!data && isLoading}>
                {tahunAktif && !pilihanTahun.includes(tahunAktif) && <option value={tahunAktif}>{tahunAktif}</option>}
                {pilihanTahun.map((t) => (
                  <option key={t} value={t}>
                    {t}
                  </option>
                ))}
              </select>
            </div>
            <form onSubmit={terapkanKlpd} className="flex items-end gap-2">
              <div className="w-32">
                <label htmlFor="ds-klpd" className="mb-1 block text-xs font-medium text-slate-600">
                  Kode KLPD
                </label>
                <input id="ds-klpd" value={klpdInput} onChange={(e) => setKlpdInput(e.target.value)} maxLength={20} className={KELAS_INPUT} />
              </div>
              <button type="submit" disabled={!klpdInput.trim() || klpdInput.trim() === klpd} className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 shadow-sm hover:bg-slate-50 disabled:opacity-50">
                Terapkan
              </button>
            </form>
          </div>
          <div className="flex flex-wrap items-center gap-3 text-xs text-slate-500">
            {data && (
              <span title={formatWaktu(data.dibuat)}>
                Dihitung {waktuRelatif(data.dibuat)}
                {data.dari_cache ? " (dari cache)" : ""}
              </span>
            )}
            <button type="button" onClick={analitik.segarkan} disabled={isLoading} className="inline-flex items-center gap-1.5 rounded-lg border border-slate-300 bg-white px-3 py-1.5 font-medium text-slate-700 shadow-sm hover:bg-slate-50 disabled:opacity-60">
              {isLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden="true" /> : <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />}
              Hitung ulang
            </button>
            <Link href="/dashboard/pengadaan-terpadu/penarikan" className="inline-flex items-center gap-1.5 rounded-lg bg-blue-50 px-3 py-1.5 font-medium text-blue-700 hover:bg-blue-100">
              <DatabaseBackup className="h-3.5 w-3.5" aria-hidden="true" />
              Penarikan Data
            </Link>
          </div>
        </div>
      </section>

      {error && <Alert message={error} />}
      {!hasil && !error && <div role="status" aria-label="Memuat dasbor pengadaan" className="h-64 animate-pulse rounded-2xl bg-slate-100" />}

      {hasil && (
        <div className={isLoading ? "opacity-60 transition-opacity" : "transition-opacity"} aria-busy={isLoading}>
          {kosongSemua ? (
            <KartuKosong
              judul={`Belum ada data pengadaan untuk ${hasil.kode_klpd} tahun ${hasil.tahun}`}
              isi="Dasbor dihitung dari salinan data Inaproc di PASTI. Tarik data Pengadaan, Tender, dan E-Katalog lebih dulu (manual atau lewat penarikan otomatis), lalu kembali ke sini."
              tautan={{ href: "/dashboard/pengadaan-terpadu/penarikan", label: "Buka halaman Penarikan Data" }}
            />
          ) : (
            <div className="space-y-5">
              <Tabs tabs={TABS} value={tab} onChange={setTab} label="Bagian dasbor pengadaan" idPrefix="ds" />
              <div role="tabpanel" id={`ds-panel-${tab}`} aria-labelledby={`ds-tab-${tab}`}>
                {tab === "ikhtisar" && <Ikhtisar hasil={hasil} wawasan={data?.wawasan ?? []} />}
                {tab === "rup" && <BagianRUP hasil={hasil} wawasan={data?.wawasan ?? []} />}
                {tab === "pemilihan" && <BagianPemilihan hasil={hasil} wawasan={data?.wawasan ?? []} />}
                {tab === "kontrak" && <BagianKontrak hasil={hasil} wawasan={data?.wawasan ?? []} />}
                {tab === "ekatalog" && <BagianEkatalog hasil={hasil} wawasan={data?.wawasan ?? []} />}
              </div>
              <p className="text-xs text-slate-400">
                Sumber: data.inaproc.id yang disalin ke PASTI. Angka mengikuti penarikan terakhir{hasil.terakhir_tarik ? ` (${formatWaktu(hasil.terakhir_tarik)})` : ""}; paket RUP yang dihapus atau tidak aktif tidak dihitung.
              </p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
