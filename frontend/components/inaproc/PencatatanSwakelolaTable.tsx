"use client";

import { NotebookPen } from "lucide-react";
import { getPencatatanSwakelola, syncPencatatanSwakelola, PencatatanSwakelolaItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { PencatatanSwakelolaDetailModal } from "@/components/inaproc/PencatatanSwakelolaDetailModal";
import { StatusBadge } from "@/components/inaproc/StatusBadge";
import { KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

const KOLOM: KolomTender<PencatatanSwakelolaItem>[] = [
  { judul: "Kode Pencatatan", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.kd_swakelola_pct || "-" },
  { judul: "Nama Paket", kelas: "max-w-[280px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_paket, render: (r) => r.nama_paket || "-" },
  { judul: "Satker", kelas: "max-w-[200px] truncate text-slate-600", tooltip: (r) => r.nama_satker, render: (r) => r.nama_satker || "-" },
  { judul: "Tipe", kelas: "max-w-[140px] truncate text-slate-600", tooltip: (r) => r.tipe_swakelola_nama, render: (r) => r.tipe_swakelola_nama || "-" },
  { judul: "Pagu", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.pagu) },
  { judul: "Total Realisasi", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.total_realisasi) },
  { judul: "Status", render: (r) => <StatusBadge status={r.status_swakelola_pct_ket || r.status_swakelola_pct} /> },
  { judul: "Tgl. Selesai", kelas: "whitespace-nowrap text-xs text-slate-500", render: (r) => formatDate(r.tgl_selesai_paket) },
];

export function PencatatanSwakelolaTable() {
  return (
    <TenderCursorTable<PencatatanSwakelolaItem>
      ikon={NotebookPen}
      petunjukAwal="Isi Tahun untuk melihat pencatatan swakelola Kementerian Keuangan"
      ambil={getPencatatanSwakelola}
      sinkron={syncPencatatanSwakelola}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.kd_swakelola_pct}-${i}`}
      detail={(row, tutup) => <PencatatanSwakelolaDetailModal item={row} onClose={tutup} />}
    />
  );
}
