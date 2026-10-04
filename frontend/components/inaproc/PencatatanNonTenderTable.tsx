"use client";

import { ClipboardList } from "lucide-react";
import { getPencatatanNonTender, syncPencatatanNonTender, PencatatanNonTenderItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { PencatatanNonTenderDetailModal } from "@/components/inaproc/PencatatanNonTenderDetailModal";
import { StatusBadge } from "@/components/inaproc/StatusBadge";
import { KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

const KOLOM: KolomTender<PencatatanNonTenderItem>[] = [
  { judul: "Kode Pencatatan", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.kd_nontender_pct || "-" },
  { judul: "Nama Paket", kelas: "max-w-[280px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_paket, render: (r) => r.nama_paket || "-" },
  { judul: "Satker", kelas: "max-w-[200px] truncate text-slate-600", tooltip: (r) => r.nama_satker, render: (r) => r.nama_satker || "-" },
  { judul: "Metode", render: (r) => r.mtd_pemilihan || "-" },
  { judul: "Pagu", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.pagu) },
  { judul: "Total Realisasi", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.total_realisasi) },
  { judul: "Status", render: (r) => <StatusBadge status={r.status_nontender_pct_ket || r.status_nontender_pct} /> },
  { judul: "Tgl. Selesai", kelas: "whitespace-nowrap text-xs text-slate-500", render: (r) => formatDate(r.tgl_selesai_paket) },
];

export function PencatatanNonTenderTable() {
  return (
    <TenderCursorTable<PencatatanNonTenderItem>
      ikon={ClipboardList}
      petunjukAwal="Isi Tahun untuk melihat pencatatan non tender Kementerian Keuangan"
      ambil={getPencatatanNonTender}
      sinkron={syncPencatatanNonTender}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.kd_nontender_pct}-${i}`}
      detail={(row, tutup) => <PencatatanNonTenderDetailModal item={row} onClose={tutup} />}
    />
  );
}
