"use client";

import { FileText, Banknote, Building2, Package, FolderTree } from "lucide-react";
import { Ekatalog6TransaksiItem } from "@/lib/api";
import { formatCurrency } from "@/lib/format";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: Ekatalog6TransaksiItem;
  onClose: () => void;
}

export function Ekatalog6TransaksiDetailModal({ item, onClose }: Props) {
  const order: DetailField[] = [
    ["Order ID", item.order_id],
    ["Status", item.status?.replace(/_/g, " ")],
  ];

  const nilai: DetailField[] = [["Nilai Transaksi", item.nilai_transaksi != null && formatCurrency(item.nilai_transaksi)]];

  const produk: DetailField[] = [
    ["Nama Produk", item.nama_produk],
    ["Kode Produk", item.product_id],
  ];

  const kategori: DetailField[] = [
    ["Level 1", item.kategori_1],
    ["Kode Level 1", item.kd_kategori_1],
    ["Level 2", item.kategori_2],
    ["Kode Level 2", item.kd_kategori_2],
    ["Level 3", item.kategori_3],
    ["Kode Level 3", item.kd_kategori_3],
  ];

  const satker: DetailField[] = [
    ["Satuan Kerja", item.nama_satker],
    ["Kode Satker", item.kode_satker],
    ["KLPD", item.nama_klpd],
    ["Kode KLPD", item.kode_klpd],
    ["Kelompok KLPD", item.nama_group_klpd],
  ];

  return (
    <TenderDetailShell
      judul="Detail Transaksi E-Purchasing per Produk"
      subjudul={`Order ${item.order_id}`}
      nama={item.nama_produk}
      lokasi={item.nama_satker}
      onClose={onClose}
    >
      <DetailGroup icon={FileText} title="Order" fields={order} />
      <DetailGroup icon={Banknote} title="Nilai" fields={nilai} />
      <DetailGroup icon={Package} title="Produk" fields={produk} />
      <DetailGroup icon={FolderTree} title="Kategori Produk" fields={kategori} />
      <DetailGroup icon={Building2} title="Satker & KLPD" fields={satker} />
    </TenderDetailShell>
  );
}
