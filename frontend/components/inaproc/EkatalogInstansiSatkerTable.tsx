"use client";

import { Building2 } from "lucide-react";
import { getEkatalogInstansiSatker, syncEkatalogInstansiSatker, EkatalogInstansiSatkerItem } from "@/lib/api";
import { EkatalogInstansiSatkerDetailModal } from "@/components/inaproc/EkatalogInstansiSatkerDetailModal";
import { KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

const KOLOM: KolomTender<EkatalogInstansiSatkerItem>[] = [
  { judul: "Kode Satker", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.kd_satker_str || r.kd_satker || "-" },
  { judul: "Nama Satker", kelas: "max-w-[360px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_satker, render: (r) => r.nama_satker || "-" },
  { judul: "KLPD", kelas: "max-w-[240px] truncate text-slate-600", tooltip: (r) => r.nama_klpd, render: (r) => r.nama_klpd || "-" },
  { judul: "Jenis KLPD", render: (r) => r.jenis_klpd || "-" },
];

// Data rujukan per KLPD: tidak ada tahun.
export function EkatalogInstansiSatkerTable() {
  return (
    <TenderCursorTable<EkatalogInstansiSatkerItem>
      ikon={Building2}
      petunjukAwal="Klik Cari untuk melihat daftar instansi dan satker E-Katalog Kementerian Keuangan"
      tanpaTahun
      ambil={(p) => getEkatalogInstansiSatker({ kode_klpd: p.kode_klpd, limit: p.limit, cursor: p.cursor })}
      sinkron={(p) => syncEkatalogInstansiSatker({ kode_klpd: p.kode_klpd })}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.kd_satker}-${i}`}
      detail={(row, tutup) => <EkatalogInstansiSatkerDetailModal item={row} onClose={tutup} />}
    />
  );
}
