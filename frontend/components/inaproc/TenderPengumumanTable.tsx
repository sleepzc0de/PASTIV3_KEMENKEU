"use client";

import { Newspaper } from "lucide-react";
import { getTenderPengumuman, syncTenderPengumuman, TenderPengumumanItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { TenderPengumumanDetailModal } from "@/components/inaproc/TenderPengumumanDetailModal";
import { StatusBadge } from "@/components/inaproc/StatusBadge";
import { IsianKode, KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

const KOLOM: KolomTender<TenderPengumumanItem>[] = [
  { judul: "Kode Tender", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.kd_tender || "-" },
  { judul: "Nama Paket", kelas: "max-w-[280px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_paket, render: (r) => r.nama_paket || "-" },
  { judul: "Satker", kelas: "max-w-[200px] truncate text-slate-600", tooltip: (r) => r.nama_satker, render: (r) => r.nama_satker || "-" },
  { judul: "Metode", render: (r) => r.mtd_pemilihan || "-" },
  { judul: "Pagu", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.pagu) },
  { judul: "HPS", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.hps) },
  { judul: "Status", render: (r) => <StatusBadge status={r.status_tender} /> },
  { judul: "Tgl. Pengumuman", kelas: "whitespace-nowrap text-xs text-slate-500", render: (r) => formatDate(r.tgl_pengumuman_tender) },
];

// Dua skenario di Inaproc: (tahun + kode_klpd) atau kd_tender saja; bila Kode Tender diisi, tahun diabaikan.
const KODE_TENDER: IsianKode = { label: "Kode Tender (opsional)", nama: "Kode Tender", placeholder: "Bila diisi, tahun diabaikan", numerik: true };

export function TenderPengumumanTable() {
  return (
    <TenderCursorTable<TenderPengumumanItem>
      ikon={Newspaper}
      petunjukAwal="Isi Tahun (atau Kode Tender) untuk melihat pengumuman tender Kementerian Keuangan"
      kodeCari={KODE_TENDER}
      ambil={(p) => getTenderPengumuman({ kode_klpd: p.kode_klpd, tahun: p.tahun, kd_tender: p.kode, limit: p.limit, cursor: p.cursor })}
      sinkron={syncTenderPengumuman}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.kd_tender}-${r.versi_tender ?? ""}-${i}`}
      detail={(row, tutup) => <TenderPengumumanDetailModal item={row} onClose={tutup} />}
    />
  );
}
