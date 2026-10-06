"use client";

import { useCallback, useEffect, useState } from "react";
import { Info } from "lucide-react";
import { DaftarDatasetPengadaan, getDaftarDatasetPengadaan } from "@/lib/api";
import { teksCakupan } from "@/lib/peran";
import { Alert } from "@/components/ui/Alert";
import { errorMessage } from "../digitalisasi/useDigitalisasi";
import { DataEksporPanel } from "./DataEksporPanel";

// Pengadaan Terpadu bagi peran yang dibatasi per satker (UE1, Kanwil, Satker): hanya lihat dan ekspor data satkernya. Penarikan, riwayat, dan jadwal adalah urusan admin
// dan memuat keadaan seluruh data, jadi tidak ditampilkan di sini.
export function DataTerbatas() {
  const [data, setData] = useState<DaftarDatasetPengadaan | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let batal = false;
    setError(null);
    getDaftarDatasetPengadaan()
      .then((res) => {
        if (!batal) setData(res.data);
      })
      .catch((err) => {
        if (!batal) setError(errorMessage(err, "Gagal memuat daftar dataset pengadaan"));
      });
    return () => {
      batal = true;
    };
  }, [tick]);
  const muatUlang = useCallback(() => setTick((t) => t + 1), []);

  if (!data) {
    if (error) {
      return (
        <div className="space-y-3">
          <Alert message={error} />
          <button type="button" onClick={muatUlang} className="text-sm font-medium text-blue-600 hover:text-blue-700">
            Coba lagi
          </button>
        </div>
      );
    }
    return <div role="status" aria-label="Memuat daftar dataset" className="h-48 animate-pulse rounded-2xl bg-slate-100" />;
  }

  return (
    <div className="space-y-5">
      <p role="note" className="flex items-start gap-2 rounded-xl border border-blue-200 bg-blue-50 px-4 py-3 text-xs text-blue-900">
        <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />
        <span>
          Menampilkan pengadaan satker dalam cakupan Anda ({teksCakupan(data.cakupan)}), dihubungkan lewat kode satker pada data Inaproc dan data aset. Dataset yang tidak memuat kode satker
          (mis. program master dan rujukan katalog) tidak ditampilkan. Satker yang punya pengadaan tetapi belum ada di data aset tidak terlihat oleh UE1 dan Kanwil. Penarikan data dari Inaproc
          dilakukan admin.
        </span>
      </p>
      <DataEksporPanel status={data} versi={0} terbatas />
    </div>
  );
}
