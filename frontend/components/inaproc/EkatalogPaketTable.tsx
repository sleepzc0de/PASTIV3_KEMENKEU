"use client";

import { ShoppingBag } from "lucide-react";
import { getEkatalogPaket, syncEkatalogPaket, EkatalogPaketItem } from "@/lib/api";
import { formatDate, formatCurrency, formatNumber } from "@/lib/format";
import { EkatalogPaketDetailModal } from "@/components/inaproc/EkatalogPaketDetailModal";
import { StatusBadge } from "@/components/inaproc/StatusBadge";
import { KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

const KOLOM: KolomTender<EkatalogPaketItem>[] = [
  { judul: "No. Paket", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.no_paket || r.kd_paket || "-" },
  { judul: "Nama Paket", kelas: "max-w-[280px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_paket, render: (r) => r.nama_paket || "-" },
  { judul: "Satker", kelas: "max-w-[200px] truncate text-slate-600", tooltip: (r) => r.nama_satker, render: (r) => r.nama_satker || "-" },
  { judul: "Kuantitas", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatNumber(r.kuantitas) },
  { judul: "Total Harga", rata: "kanan", kelas: "whitespace-nowrap font-medium text-slate-800", render: (r) => formatCurrency(r.total_harga) },
  { judul: "Status", render: (r) => <StatusBadge status={r.paket_status_str || r.status_paket} /> },
  { judul: "Tgl. Buat", kelas: "whitespace-nowrap text-xs text-slate-500", render: (r) => formatDate(r.tanggal_buat_paket) },
];

export function EkatalogPaketTable() {
  return (
    <TenderCursorTable<EkatalogPaketItem>
      ikon={ShoppingBag}
      petunjukAwal="Isi Tahun untuk melihat paket e-purchasing Kementerian Keuangan"
      ambil={(p) => getEkatalogPaket({ kode_klpd: p.kode_klpd, tahun: p.tahun, limit: p.limit, cursor: p.cursor })}
      sinkron={(p) => syncEkatalogPaket({ kode_klpd: p.kode_klpd, tahun: p.tahun })}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.kd_paket}-${r.kd_paket_produk ?? ""}-${i}`}
      detail={(row, tutup) => <EkatalogPaketDetailModal item={row} onClose={tutup} />}
    />
  );
}
