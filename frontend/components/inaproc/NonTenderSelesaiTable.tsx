"use client";

import { BadgeCheck } from "lucide-react";
import { getNonTenderSelesai, syncNonTenderSelesai, NonTenderSelesaiItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { NonTenderSelesaiDetailModal } from "@/components/inaproc/NonTenderSelesaiDetailModal";
import { StatusBadge } from "@/components/inaproc/StatusBadge";
import { KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

const KOLOM: KolomTender<NonTenderSelesaiItem>[] = [
  { judul: "Kode Non Tender", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.kd_nontender || "-" },
  { judul: "Nama Paket", kelas: "max-w-[280px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_paket, render: (r) => r.nama_paket || "-" },
  { judul: "Satker", kelas: "max-w-[200px] truncate text-slate-600", tooltip: (r) => r.nama_satker, render: (r) => r.nama_satker || "-" },
  { judul: "Penyedia", kelas: "max-w-[200px] truncate text-slate-600", tooltip: (r) => r.nama_penyedia, render: (r) => r.nama_penyedia || "-" },
  { judul: "Pagu", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.pagu) },
  { judul: "HPS", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.hps) },
  { judul: "Nilai Kontrak", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.nilai_kontrak) },
  { judul: "Status", render: (r) => <StatusBadge status={r.status_nontender} /> },
  { judul: "Tgl. Selesai", kelas: "whitespace-nowrap text-xs text-slate-500", render: (r) => formatDate(r.tgl_selesai_nontender) },
];

export function NonTenderSelesaiTable() {
  return (
    <TenderCursorTable<NonTenderSelesaiItem>
      ikon={BadgeCheck}
      petunjukAwal="Isi Tahun untuk melihat paket non tender yang sudah selesai di Kementerian Keuangan"
      ambil={getNonTenderSelesai}
      sinkron={syncNonTenderSelesai}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.kd_nontender}-${i}`}
      detail={(row, tutup) => <NonTenderSelesaiDetailModal item={row} onClose={tutup} />}
    />
  );
}
