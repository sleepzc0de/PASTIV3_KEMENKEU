"use client";

import { ShoppingBag } from "lucide-react";
import { getEkatalog6Paket, syncEkatalog6Paket, Ekatalog6PaketItem } from "@/lib/api";
import { formatDate, formatCurrency, formatNumber } from "@/lib/format";
import { Ekatalog6PaketDetailModal } from "@/components/inaproc/Ekatalog6PaketDetailModal";
import { StatusTransaksiBadge } from "@/components/inaproc/StatusTransaksiBadge";
import { KlpdAlternatif, KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

const KOLOM: KolomTender<Ekatalog6PaketItem>[] = [
  { judul: "Order", kelas: "max-w-[160px] truncate font-mono text-xs text-slate-600", tooltip: (r) => r.order_id, render: (r) => r.order_id || "-" },
  { judul: "Paket (RUP)", kelas: "max-w-[260px] truncate font-medium text-slate-800", tooltip: (r) => r.rup_name, render: (r) => r.rup_name || "-" },
  { judul: "Satker", kelas: "max-w-[200px] truncate text-slate-600", tooltip: (r) => r.nama_satker, render: (r) => r.nama_satker || "-" },
  { judul: "Penyedia", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.kode_penyedia || "-" },
  { judul: "Jumlah", rata: "kanan", kelas: "whitespace-nowrap text-slate-800", render: (r) => formatNumber(r.total_qty) },
  { judul: "Total", rata: "kanan", kelas: "whitespace-nowrap font-medium text-slate-800", render: (r) => formatCurrency(r.total) },
  { judul: "Status", render: (r) => <StatusTransaksiBadge status={r.status} /> },
  { judul: "Tgl. Order", kelas: "whitespace-nowrap text-xs text-slate-500", render: (r) => formatDate(r.order_date) },
];

// kode_klpd "swasta" (nilai literal di API) menampilkan paket swasta.
const SWASTA: KlpdAlternatif = { label: "Paket swasta", kode: "swasta", keterangan: "Menampilkan paket e-purchasing swasta (kode KLPD: swasta)" };

export function Ekatalog6PaketTable() {
  return (
    <TenderCursorTable<Ekatalog6PaketItem>
      ikon={ShoppingBag}
      petunjukAwal="Isi Tahun untuk melihat paket e-purchasing E-Katalog V6 Kementerian Keuangan"
      klpdAlternatif={SWASTA}
      ambil={(p) => getEkatalog6Paket({ kode_klpd: p.kode_klpd, tahun: p.tahun, limit: p.limit, cursor: p.cursor })}
      sinkron={(p) => syncEkatalog6Paket({ kode_klpd: p.kode_klpd, tahun: p.tahun })}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.order_id}-${r.product_id ?? ""}-${i}`}
      detail={(row, tutup) => <Ekatalog6PaketDetailModal item={row} onClose={tutup} />}
    />
  );
}
