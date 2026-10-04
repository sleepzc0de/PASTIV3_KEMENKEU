"use client";

import { FileText, Banknote, CalendarDays, Globe, Building2 } from "lucide-react";
import { TenderSelesaiItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: TenderSelesaiItem;
  onClose: () => void;
}

// Nilai 0 tetap tampil; hanya null/undefined yang disembunyikan.
const uang = (v: number | null | undefined) => v != null && formatCurrency(v);
const teks = (v: string | number | null | undefined) => (v != null && v !== "" ? String(v) : null);

export function TenderSelesaiDetailModal({ item, onClose }: Props) {
  const paket: DetailField[] = [
    ["Kode Tender", teks(item.kd_tender)],
    ["Kode RUP", item.kd_rup],
    ["Jenis Pengadaan", item.jenis_pengadaan],
    ["Kualifikasi Paket", item.kualifikasi_paket],
    ["Kontrak Pembayaran", item.kontrak_pembayaran],
    ["Metode Pemilihan", item.mtd_pemilihan],
    ["Metode Kualifikasi", item.mtd_kualifikasi],
    ["Sumber Dana", item.sumber_dana],
    ["MAK", item.mak],
  ];

  const nilai: DetailField[] = [
    ["Pagu", uang(item.pagu)],
    ["HPS", uang(item.hps)],
  ];

  const status: DetailField[] = [
    ["Status", item.status_tender],
    ["Tgl. Pengumuman", item.tgl_pengumuman_tender && formatDate(item.tgl_pengumuman_tender)],
    ["Tgl. Penetapan Pemenang", item.tgl_penetapan_pemenang && formatDate(item.tgl_penetapan_pemenang)],
  ];

  const lpse: DetailField[] = [
    ["Nama LPSE", item.nama_lpse],
    ["Kode LPSE", teks(item.kd_lpse)],
    ["URL LPSE", item.url_lpse],
    ["Referensi Pembaruan", item.last_update_ref],
  ];

  const satker: DetailField[] = [
    ["Satuan Kerja", item.nama_satker],
    ["Kode Satker", item.kd_satker_str || item.kd_satker],
    ["KLPD", item.nama_klpd],
    ["Jenis KLPD", item.jenis_klpd],
  ];

  return (
    <TenderDetailShell
      judul="Detail Tender Selesai"
      subjudul={`Kode Tender ${item.kd_tender} · TA ${item.tahun_anggaran}`}
      nama={item.nama_paket}
      lokasi={item.nama_satker}
      onClose={onClose}
    >
      <DetailGroup icon={FileText} title="Paket" fields={paket} />
      <DetailGroup icon={Banknote} title="Anggaran" fields={nilai} />
      <DetailGroup icon={CalendarDays} title="Status & Jadwal" fields={status} />
      <DetailGroup icon={Globe} title="LPSE" fields={lpse} />
      <DetailGroup icon={Building2} title="Satker & KLPD" fields={satker} />
    </TenderDetailShell>
  );
}
