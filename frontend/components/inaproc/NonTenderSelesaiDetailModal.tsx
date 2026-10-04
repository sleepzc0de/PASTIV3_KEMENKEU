"use client";

import { FileText, Banknote, CalendarDays, Globe, Building2, Briefcase } from "lucide-react";
import { NonTenderSelesaiItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface NonTenderSelesaiDetailModalProps {
  item: NonTenderSelesaiItem;
  onClose: () => void;
}

// Nilai 0 tetap tampil (paket bernilai nol berbeda dari data yang kosong); hanya null/undefined yang disembunyikan.
const uang = (v: number | null | undefined) => v != null && formatCurrency(v);
const teks = (v: string | number | null | undefined) => (v != null && v !== "" ? String(v) : null);

export function NonTenderSelesaiDetailModal({ item, onClose }: NonTenderSelesaiDetailModalProps) {
  const paket: DetailField[] = [
    ["Kode Non Tender", teks(item.kd_nontender)],
    ["Kode RUP", item.kd_rup],
    ["Jenis Pengadaan", item.jenis_pengadaan],
    ["Metode Pemilihan", item.mtd_pemilihan],
    ["Kualifikasi Paket", item.kualifikasi_paket],
    ["Kontrak Pembayaran", item.kontrak_pembayaran],
    ["Sumber Dana", item.sumber_dana],
    ["MAK", item.mak],
  ];

  const nilai: DetailField[] = [
    ["Pagu", uang(item.pagu)],
    ["HPS", uang(item.hps)],
    ["Nilai Penawaran", uang(item.nilai_penawaran)],
    ["Nilai Negosiasi", uang(item.nilai_negosiasi)],
    ["Nilai Terkoreksi", uang(item.nilai_terkoreksi)],
    ["Nilai Kontrak", uang(item.nilai_kontrak)],
    ["Nilai PDN Kontrak", uang(item.nilai_pdn_kontrak)],
    ["Nilai UMK Kontrak", uang(item.nilai_umk_kontrak)],
  ];

  const penyedia: DetailField[] = [
    ["Nama Penyedia", item.nama_penyedia],
    ["Kode Penyedia", teks(item.kd_penyedia)],
    ["NPWP", item.npwp_penyedia],
    ["NPWP 16 Digit", item.npwp16_penyedia],
  ];

  const status: DetailField[] = [
    ["Status", item.status_nontender],
    ["Tgl. Pengumuman", item.tgl_pengumuman_nontender && formatDate(item.tgl_pengumuman_nontender)],
    ["Tgl. Selesai", item.tgl_selesai_nontender && formatDate(item.tgl_selesai_nontender)],
    ["Tgl. Penarikan Data", item.tgl_penarikan && formatDate(item.tgl_penarikan)],
  ];

  const lpse: DetailField[] = [
    ["Nama LPSE", item.nama_lpse],
    ["URL LPSE", item.url_lpse],
    ["Kode LPSE", teks(item.kd_lpse)],
    ["ID LPSE", teks(item.lpse_id)],
    ["Kode Paket DCE", teks(item.kd_pkt_dce)],
  ];

  const satker: DetailField[] = [
    ["Satuan Kerja", item.nama_satker],
    ["Kode Satker", item.kd_satker_str || item.kd_satker],
    ["KLPD", item.nama_klpd],
    ["Jenis KLPD", item.jenis_klpd],
  ];

  return (
    <TenderDetailShell
      judul="Detail Non Tender Selesai"
      subjudul={`Kode Non Tender ${item.kd_nontender} · TA ${item.tahun_anggaran}`}
      nama={item.nama_paket}
      lokasi={item.nama_satker}
      onClose={onClose}
    >
      <DetailGroup icon={FileText} title="Paket" fields={paket} />
      <DetailGroup icon={Banknote} title="Nilai" fields={nilai} />
      <DetailGroup icon={Briefcase} title="Penyedia" fields={penyedia} />
      <DetailGroup icon={CalendarDays} title="Status & Jadwal" fields={status} />
      <DetailGroup icon={Globe} title="LPSE" fields={lpse} />
      <DetailGroup icon={Building2} title="Satker & KLPD" fields={satker} />
    </TenderDetailShell>
  );
}
