"use client";

import { ReceiptText } from "lucide-react";
import { getPencatatanSwakelolaRealisasi, syncPencatatanSwakelolaRealisasi, PencatatanSwakelolaRealisasiItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { PencatatanSwakelolaRealisasiDetailModal } from "@/components/inaproc/PencatatanSwakelolaRealisasiDetailModal";
import { KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

// Respons realisasi swakelola tidak memuat nama paket/satker, jadi kolomnya berpusat pada pelaksana dan nilai realisasi.
const KOLOM: KolomTender<PencatatanSwakelolaRealisasiItem>[] = [
  { judul: "No. Realisasi", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.no_realisasi || "-" },
  { judul: "Kode Pencatatan", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.kd_swakelola_pct || "-" },
  { judul: "Pelaksana", kelas: "max-w-[240px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_pelaksana, render: (r) => r.nama_pelaksana || "-" },
  { judul: "Jenis Realisasi", kelas: "max-w-[180px] truncate text-slate-600", tooltip: (r) => r.jenis_realisasi, render: (r) => r.jenis_realisasi || "-" },
  { judul: "PPK", kelas: "max-w-[200px] truncate text-slate-600", tooltip: (r) => r.nama_ppk, render: (r) => r.nama_ppk || "-" },
  { judul: "Nilai Realisasi", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.nilai_realisasi) },
  { judul: "Tgl. Realisasi", kelas: "whitespace-nowrap text-xs text-slate-500", render: (r) => formatDate(r.tgl_realisasi) },
];

export function PencatatanSwakelolaRealisasiTable() {
  return (
    <TenderCursorTable<PencatatanSwakelolaRealisasiItem>
      ikon={ReceiptText}
      petunjukAwal="Isi Tahun untuk melihat realisasi pencatatan swakelola Kementerian Keuangan"
      ambil={getPencatatanSwakelolaRealisasi}
      sinkron={syncPencatatanSwakelolaRealisasi}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.kd_swakelola_pct}-${r.rsk_id ?? ""}-${i}`}
      detail={(row, tutup) => <PencatatanSwakelolaRealisasiDetailModal item={row} onClose={tutup} />}
    />
  );
}
