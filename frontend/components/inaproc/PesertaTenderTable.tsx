"use client";

import { UsersRound } from "lucide-react";
import { getPesertaTender, syncPesertaTender, PesertaTenderItem } from "@/lib/api";
import { formatCurrency } from "@/lib/format";
import { PesertaTenderDetailModal } from "@/components/inaproc/PesertaTenderDetailModal";
import { KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

// pemenang dan pemenang_terverifikasi berupa penanda 0/1; peserta yang bukan pemenang cukup diberi tanda "-".
function Hasil({ item }: { item: PesertaTenderItem }) {
  if (item.pemenang !== 1) return <span className="text-slate-400">-</span>;
  return (
    <span className="flex items-center gap-1.5 whitespace-nowrap">
      <span className="rounded-full bg-green-50 px-2.5 py-0.5 text-xs font-medium text-green-700">Pemenang</span>
      {item.pemenang_terverifikasi === 1 && <span className="rounded-full bg-blue-50 px-2.5 py-0.5 text-xs font-medium text-blue-700">Terverifikasi</span>}
    </span>
  );
}

const KOLOM: KolomTender<PesertaTenderItem>[] = [
  { judul: "Kode Tender", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.kd_tender || "-" },
  { judul: "Penyedia", kelas: "max-w-[300px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_penyedia, render: (r) => r.nama_penyedia || "-" },
  { judul: "NPWP", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.npwp_penyedia || r.npwp_penyedia_16 || "-" },
  { judul: "Nilai Penawaran", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.nilai_penawaran) },
  { judul: "Nilai Terkoreksi", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatCurrency(r.nilai_terkoreksi) },
  { judul: "Hasil", render: (r) => <Hasil item={r} /> },
];

export function PesertaTenderTable() {
  return (
    <TenderCursorTable<PesertaTenderItem>
      ikon={UsersRound}
      petunjukAwal="Isi Tahun untuk melihat peserta tender Kementerian Keuangan"
      ambil={getPesertaTender}
      sinkron={syncPesertaTender}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.kd_tender}-${r.kd_peserta}-${i}`}
      detail={(row, tutup) => <PesertaTenderDetailModal item={row} onClose={tutup} />}
    />
  );
}
