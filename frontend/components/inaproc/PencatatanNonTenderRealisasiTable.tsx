"use client";

import { Receipt } from "lucide-react";
import { getPencatatanNonTenderRealisasi, syncPencatatanNonTenderRealisasi, PencatatanNonTenderRealisasiItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { PencatatanNonTenderRealisasiDetailModal } from "@/components/inaproc/PencatatanNonTenderRealisasiDetailModal";
import { KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

const KOLOM: KolomTender<PencatatanNonTenderRealisasiItem>[] = [
  { judul: "No. Realisasi", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.no_realisasi || "-" },
  { judul: "Nama Paket", kelas: "max-w-[280px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_paket, render: (r) => r.nama_paket || "-" },
  { judul: "Satker", kelas: "max-w-[200px] truncate text-slate-600", tooltip: (r) => r.nama_satker, render: (r) => r.nama_satker || "-" },
  { judul: "Penyedia", kelas: "max-w-[200px] truncate text-slate-600", tooltip: (r) => r.nama_penyedia, render: (r) => r.nama_penyedia || "-" },
  { judul: "Jenis Realisasi", kelas: "max-w-[160px] truncate text-slate-600", tooltip: (r) => r.jenis_realisasi, render: (r) => r.jenis_realisasi || "-" },
  { judul: "Pagu", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.pagu) },
  { judul: "Nilai Realisasi", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.nilai_realisasi) },
  { judul: "Tgl. Realisasi", kelas: "whitespace-nowrap text-xs text-slate-500", render: (r) => formatDate(r.tgl_realisasi) },
];

export function PencatatanNonTenderRealisasiTable() {
  return (
    <TenderCursorTable<PencatatanNonTenderRealisasiItem>
      ikon={Receipt}
      petunjukAwal="Isi Tahun untuk melihat realisasi pencatatan non tender Kementerian Keuangan"
      ambil={getPencatatanNonTenderRealisasi}
      sinkron={syncPencatatanNonTenderRealisasi}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.kd_nontender_pct}-${r.no_realisasi ?? ""}-${i}`}
      detail={(row, tutup) => <PencatatanNonTenderRealisasiDetailModal item={row} onClose={tutup} />}
    />
  );
}
