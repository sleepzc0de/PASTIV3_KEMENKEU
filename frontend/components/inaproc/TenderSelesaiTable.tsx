"use client";

import { Trophy } from "lucide-react";
import { getTenderSelesai, syncTenderSelesai, TenderSelesaiItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { TenderSelesaiDetailModal } from "@/components/inaproc/TenderSelesaiDetailModal";
import { StatusBadge } from "@/components/inaproc/StatusBadge";
import { KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

const KOLOM: KolomTender<TenderSelesaiItem>[] = [
  { judul: "Kode Tender", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.kd_tender || "-" },
  { judul: "Nama Paket", kelas: "max-w-[280px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_paket, render: (r) => r.nama_paket || "-" },
  { judul: "Satker", kelas: "max-w-[200px] truncate text-slate-600", tooltip: (r) => r.nama_satker, render: (r) => r.nama_satker || "-" },
  { judul: "Metode", render: (r) => r.mtd_pemilihan || "-" },
  { judul: "Pagu", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.pagu) },
  { judul: "HPS", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.hps) },
  { judul: "Status", render: (r) => <StatusBadge status={r.status_tender} /> },
  { judul: "Tgl. Penetapan Pemenang", kelas: "whitespace-nowrap text-xs text-slate-500", render: (r) => formatDate(r.tgl_penetapan_pemenang) },
];

export function TenderSelesaiTable() {
  return (
    <TenderCursorTable<TenderSelesaiItem>
      ikon={Trophy}
      petunjukAwal="Isi Tahun untuk melihat tender selesai Kementerian Keuangan"
      ambil={getTenderSelesai}
      sinkron={syncTenderSelesai}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.kd_tender}-${i}`}
      detail={(row, tutup) => <TenderSelesaiDetailModal item={row} onClose={tutup} />}
    />
  );
}
