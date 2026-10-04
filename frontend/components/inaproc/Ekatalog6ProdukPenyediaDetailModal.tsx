"use client";

import { Package } from "lucide-react";
import { Ekatalog6ProdukPenyediaItem } from "@/lib/api";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: Ekatalog6ProdukPenyediaItem;
  onClose: () => void;
}

export function Ekatalog6ProdukPenyediaDetailModal({ item, onClose }: Props) {
  const produk: DetailField[] = [
    ["Kode Produk", item.kd_produk],
    ["Nama Produk", item.nama_produk],
    ["Status Produk", item.status_produk],
    ["Penayangan", item.status_produk_tayang == null ? null : item.status_produk_tayang ? "Tayang" : "Tidak tayang"],
  ];

  return (
    <TenderDetailShell judul="Detail Produk Penyedia" subjudul={`Kode Produk ${item.kd_produk}`} nama={item.nama_produk} onClose={onClose}>
      <DetailGroup icon={Package} title="Produk" fields={produk} />
    </TenderDetailShell>
  );
}
