"use client";

import { ChartColumn } from "lucide-react";
import { getEkatalog6Transaksi, syncEkatalog6Transaksi, Ekatalog6TransaksiItem, STATUS_TRANSAKSI } from "@/lib/api";
import { formatCurrency } from "@/lib/format";
import { Ekatalog6TransaksiDetailModal } from "@/components/inaproc/Ekatalog6TransaksiDetailModal";
import { StatusTransaksiBadge } from "@/components/inaproc/StatusTransaksiBadge";
import { FilterTambahan, KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

const KOLOM: KolomTender<Ekatalog6TransaksiItem>[] = [
  { judul: "Order", kelas: "max-w-[160px] truncate font-mono text-xs text-slate-600", tooltip: (r) => r.order_id, render: (r) => r.order_id || "-" },
  { judul: "Produk", kelas: "max-w-[280px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_produk, render: (r) => r.nama_produk || "-" },
  { judul: "Kategori", kelas: "max-w-[180px] truncate text-slate-600", tooltip: (r) => r.kategori_1, render: (r) => r.kategori_1 || "-" },
  { judul: "Satker", kelas: "max-w-[200px] truncate text-slate-600", tooltip: (r) => r.nama_satker, render: (r) => r.nama_satker || "-" },
  { judul: "Status", render: (r) => <StatusTransaksiBadge status={r.status} /> },
  { judul: "Nilai Transaksi", rata: "kanan", kelas: "whitespace-nowrap font-medium text-slate-800", render: (r) => formatCurrency(r.nilai_transaksi) },
];

// Tahun wajib. Status bawaan COMPLETED (yang juga bawaan API); kode produk dan kode kategori level 1 mempersempit hasil. Sinkronisasi
// memakai filter yang sama: satu kombinasi (tahun, status, ...) per tarikan, mengganti baris untuk kombinasi itu.
const FILTER: FilterTambahan[] = [
  { kunci: "status", label: "Status Transaksi", pilihan: STATUS_TRANSAKSI, bawaan: "COMPLETED" },
  { kunci: "kd_product", label: "Kode Produk (opsional)", placeholder: "mis. ac657ed7-..." },
  { kunci: "kd_kategori_1", label: "Kode Kategori Level 1 (opsional)", placeholder: "dari halaman Kategori Produk" },
];

export function Ekatalog6TransaksiTable() {
  return (
    <TenderCursorTable<Ekatalog6TransaksiItem>
      ikon={ChartColumn}
      petunjukAwal="Isi Tahun (dan, bila perlu, status/kode produk/kategori) untuk melihat nilai transaksi e-purchasing Kementerian Keuangan"
      filterTambahan={FILTER}
      ambil={(p) =>
        getEkatalog6Transaksi({
          tahun: p.tahun,
          kode_klpd: p.kode_klpd,
          status: p.tambahan?.status,
          kd_product: p.tambahan?.kd_product,
          kd_kategori_1: p.tambahan?.kd_kategori_1,
          limit: p.limit,
          cursor: p.cursor,
        })
      }
      sinkron={(p) =>
        syncEkatalog6Transaksi({
          tahun: p.tahun,
          kode_klpd: p.kode_klpd,
          status: p.tambahan?.status,
          kd_product: p.tambahan?.kd_product,
          kd_kategori_1: p.tambahan?.kd_kategori_1,
        })
      }
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.order_id}-${r.product_id ?? ""}-${i}`}
      detail={(row, tutup) => <Ekatalog6TransaksiDetailModal item={row} onClose={tutup} />}
    />
  );
}
