"use client";

import { FileText, Banknote, Building2, Package, CalendarDays } from "lucide-react";
import { Ekatalog6PaketItem } from "@/lib/api";
import { formatDate, formatCurrency, formatNumber } from "@/lib/format";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: Ekatalog6PaketItem;
  onClose: () => void;
}

// Nilai 0 tetap tampil; hanya null/undefined yang disembunyikan.
const uang = (v: number | null | undefined) => v != null && formatCurrency(v);
const teks = (v: string | number | null | undefined) => (v != null && v !== "" ? String(v) : null);

export function Ekatalog6PaketDetailModal({ item, onClose }: Props) {
  const order: DetailField[] = [
    ["Order ID", item.order_id],
    ["Status", item.status?.replace(/_/g, " ")],
    ["Status Pengiriman", item.shipment_status?.replace(/_/g, " ")],
    ["Paket Swasta", item.is_swasta == null ? null : item.is_swasta ? "Ya" : "Tidak"],
    ["Kode KLPD", item.kode_klpd],
    ["Tahun Fiskal", teks(item.fiscal_year)],
  ];

  const rup: DetailField[] = [
    ["Kode RUP", item.rup_code],
    ["Nama Paket", item.rup_name],
    ["Deskripsi", item.rup_desc],
    ["MAK", item.mak],
    ["Sumber Dana", item.funding_source],
  ];

  const produk: DetailField[] = [
    ["Kode Produk", item.product_id],
    ["Jumlah Produk", item.count_product != null && formatNumber(item.count_product)],
    ["Kuantitas", item.total_qty != null && formatNumber(item.total_qty)],
    ["Kode Penyedia", item.kode_penyedia],
    ["ID Rekan", teks(item.rekan_id)],
  ];

  const harga: DetailField[] = [
    ["Ongkos Kirim", uang(item.shipping_fee)],
    ["Total", uang(item.total)],
  ];

  const satker: DetailField[] = [
    ["Satuan Kerja", item.nama_satker],
    ["Kode Satker", item.kode_satker],
  ];

  const tanggal: DetailField[] = [
    ["Tgl. Order", item.order_date && formatDate(item.order_date)],
    ["Terakhir Diperbarui", item.last_update_date && formatDate(item.last_update_date)],
  ];

  return (
    <TenderDetailShell
      judul="Detail Paket E-Purchasing V6"
      subjudul={`Order ${item.order_id} · TA ${item.fiscal_year}`}
      nama={item.rup_name}
      lokasi={item.nama_satker}
      onClose={onClose}
    >
      <DetailGroup icon={FileText} title="Order" fields={order} />
      <DetailGroup icon={FileText} title="Paket (RUP)" fields={rup} />
      <DetailGroup icon={Package} title="Produk & Penyedia" fields={produk} />
      <DetailGroup icon={Banknote} title="Nilai" fields={harga} />
      <DetailGroup icon={Building2} title="Satker" fields={satker} />
      <DetailGroup icon={CalendarDays} title="Tanggal" fields={tanggal} />
    </TenderDetailShell>
  );
}
