"use client";

import { Coins } from "lucide-react";
import { getTenderSelesaiNilai, syncTenderSelesaiNilai, TenderSelesaiNilaiItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { TenderSelesaiNilaiDetailModal } from "@/components/inaproc/TenderSelesaiNilaiDetailModal";
import { KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

// Respons nilai tidak memuat nama paket, jadi kolomnya berpusat pada penyedia dan nilai.
const KOLOM: KolomTender<TenderSelesaiNilaiItem>[] = [
  { judul: "Kode Tender", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.kd_tender || "-" },
  { judul: "Penyedia", kelas: "max-w-[260px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_penyedia, render: (r) => r.nama_penyedia || "-" },
  { judul: "Satker", kelas: "max-w-[200px] truncate text-slate-600", tooltip: (r) => r.nama_satker, render: (r) => r.nama_satker || "-" },
  { judul: "HPS", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.hps) },
  { judul: "Nilai Penawaran", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.nilai_penawaran) },
  { judul: "Nilai Negosiasi", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.nilai_negosiasi) },
  { judul: "Nilai Kontrak", rata: "kanan", kelas: "whitespace-nowrap font-medium text-slate-800", render: (r) => formatCurrency(r.nilai_kontrak) },
  { judul: "Tgl. Penetapan", kelas: "whitespace-nowrap text-xs text-slate-500", render: (r) => formatDate(r.tgl_penetapan_pemenang) },
];

export function TenderSelesaiNilaiTable() {
  return (
    <TenderCursorTable<TenderSelesaiNilaiItem>
      ikon={Coins}
      petunjukAwal="Isi Tahun untuk melihat nilai tender selesai Kementerian Keuangan"
      ambil={getTenderSelesaiNilai}
      sinkron={syncTenderSelesaiNilai}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.kd_tender}-${r.psr_id ?? ""}-${i}`}
      detail={(row, tutup) => <TenderSelesaiNilaiDetailModal item={row} onClose={tutup} />}
    />
  );
}
