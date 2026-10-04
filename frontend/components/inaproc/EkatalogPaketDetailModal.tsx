"use client";

import { FileText, Banknote, Building2, UserRound, Users, Package, MapPin, CalendarDays } from "lucide-react";
import { EkatalogPaketItem } from "@/lib/api";
import { formatDate, formatCurrency, formatNumber } from "@/lib/format";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: EkatalogPaketItem;
  onClose: () => void;
}

// Nilai 0 tetap tampil; hanya null/undefined yang disembunyikan.
const uang = (v: number | null | undefined) => v != null && formatCurrency(v);
const teks = (v: string | number | null | undefined) => (v != null && v !== "" ? String(v) : null);

export function EkatalogPaketDetailModal({ item, onClose }: Props) {
  const paket: DetailField[] = [
    ["No. Paket", item.no_paket],
    ["Kode Paket", teks(item.kd_paket)],
    ["Status", item.paket_status_str],
    ["Kode Status", item.status_paket],
    ["Deskripsi", item.deskripsi],
    ["Catatan Produk", item.catatan_produk],
    ["Kode RUP", teks(item.kd_rup)],
    ["Kode Anggaran", item.kode_anggaran],
    ["Sumber Dana", item.nama_sumber_dana],
  ];

  const produk: DetailField[] = [
    ["Kode Komoditas", teks(item.kd_komoditas)],
    ["Kode Produk", teks(item.kd_produk)],
    ["Kode Paket Produk", teks(item.kd_paket_produk)],
    ["Jumlah Jenis Produk", item.jml_jenis_produk != null && formatNumber(item.jml_jenis_produk)],
    ["Kode Penyedia", teks(item.kd_penyedia)],
    ["Kode Distributor", teks(item.kd_penyedia_distributor)],
  ];

  const harga: DetailField[] = [
    ["Harga Satuan", uang(item.harga_satuan)],
    ["Kuantitas", item.kuantitas != null && formatNumber(item.kuantitas)],
    ["Ongkos Kirim", uang(item.ongkos_kirim)],
    ["Total Harga", uang(item.total_harga)],
  ];

  const satker: DetailField[] = [
    ["Satuan Kerja", item.nama_satker],
    ["Alamat", item.alamat_satker],
    ["NPWP Satker", item.npwp_satker],
    ["ID Satker", teks(item.satker_id)],
    ["Kode KLPD", item.kd_klpd],
  ];

  const ppk: DetailField[] = [
    ["NIP PPK", item.ppk_nip],
    ["Jabatan PPK", item.jabatan_ppk],
    ["Kode User PPK", teks(item.kd_user_ppk)],
  ];

  const pokja: DetailField[] = [
    ["Kode User Pokja", teks(item.kd_user_pokja)],
    ["Email", item.email_user_pokja],
    ["Telepon", item.no_telp_user_pokja],
  ];

  const wilayah: DetailField[] = [
    ["Kode Provinsi", teks(item.kd_provinsi_wilayah_harga)],
    ["Kode Kabupaten/Kota", teks(item.kd_kabupaten_wilayah_harga)],
  ];

  const tanggal: DetailField[] = [
    ["Tgl. Buat Paket", item.tanggal_buat_paket && formatDate(item.tanggal_buat_paket)],
    ["Tgl. Edit Paket", item.tanggal_edit_paket && formatDate(item.tanggal_edit_paket)],
  ];

  return (
    <TenderDetailShell
      judul="Detail Paket E-Purchasing"
      subjudul={`${item.no_paket ? `No. ${item.no_paket} · ` : ""}Kode Paket ${item.kd_paket} · TA ${item.tahun_anggaran}`}
      nama={item.nama_paket}
      lokasi={item.nama_satker}
      onClose={onClose}
    >
      <DetailGroup icon={FileText} title="Paket" fields={paket} />
      <DetailGroup icon={Package} title="Produk & Penyedia" fields={produk} />
      <DetailGroup icon={Banknote} title="Harga" fields={harga} />
      <DetailGroup icon={Building2} title="Satker" fields={satker} />
      <DetailGroup icon={UserRound} title="PPK" fields={ppk} />
      <DetailGroup icon={Users} title="Pokja" fields={pokja} />
      <DetailGroup icon={MapPin} title="Wilayah Harga" fields={wilayah} />
      <DetailGroup icon={CalendarDays} title="Tanggal" fields={tanggal} />
    </TenderDetailShell>
  );
}
