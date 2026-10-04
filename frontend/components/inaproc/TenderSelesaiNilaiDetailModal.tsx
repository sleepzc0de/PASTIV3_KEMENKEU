"use client";

import { Banknote, Building2, Briefcase, Gavel } from "lucide-react";
import { TenderSelesaiNilaiItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: TenderSelesaiNilaiItem;
  onClose: () => void;
}

// Nilai 0 tetap tampil; hanya null/undefined yang disembunyikan.
const uang = (v: number | null | undefined) => v != null && formatCurrency(v);
const teks = (v: string | number | null | undefined) => (v != null && v !== "" ? String(v) : null);

export function TenderSelesaiNilaiDetailModal({ item, onClose }: Props) {
  const penyedia: DetailField[] = [
    ["Nama Penyedia", item.nama_penyedia],
    ["Kode Penyedia", teks(item.kd_penyedia)],
    ["NPWP", item.npwp_penyedia],
    ["NPWP 16 Digit", item.npwp_16_penyedia],
  ];

  const nilai: DetailField[] = [
    ["Pagu", uang(item.pagu)],
    ["HPS", uang(item.hps)],
    ["Nilai Penawaran", uang(item.nilai_penawaran)],
    ["Nilai Terkoreksi", uang(item.nilai_terkoreksi)],
    ["Nilai Negosiasi", uang(item.nilai_negosiasi)],
    ["Nilai Kontrak", uang(item.nilai_kontrak)],
    ["Nilai PDN Kontrak", uang(item.nilai_pdn_kontrak)],
    ["Nilai UMK Kontrak", uang(item.nilai_umk_kontrak)],
  ];

  // Respons tidak memuat nama paket: yang tersedia hanya kode, untuk dicocokkan dengan halaman Tender Selesai.
  const tender: DetailField[] = [
    ["Kode Tender", teks(item.kd_tender)],
    ["Kode Paket", teks(item.kd_paket)],
    ["Kode RUP Paket", item.kd_rup_paket],
    ["ID Peserta (psr_id)", teks(item.psr_id)],
    ["Tgl. Pengumuman", item.tgl_pengumuman_tender && formatDate(item.tgl_pengumuman_tender)],
    ["Tgl. Penetapan Pemenang", item.tgl_penetapan_pemenang && formatDate(item.tgl_penetapan_pemenang)],
    ["Kode LPSE", teks(item.kd_lpse)],
  ];

  const satker: DetailField[] = [
    ["Satuan Kerja", item.nama_satker],
    ["Kode Satker", item.kd_satker],
    ["KLPD", item.nama_klpd],
    ["Jenis KLPD", item.jenis_klpd],
  ];

  return (
    <TenderDetailShell
      judul="Detail Nilai Tender Selesai"
      subjudul={`Kode Tender ${item.kd_tender} · TA ${item.tahun_anggaran}`}
      nama={item.nama_penyedia}
      lokasi={item.nama_satker}
      onClose={onClose}
    >
      <DetailGroup icon={Briefcase} title="Penyedia Pemenang" fields={penyedia} />
      <DetailGroup icon={Banknote} title="Nilai" fields={nilai} />
      <DetailGroup icon={Gavel} title="Tender" fields={tender} />
      <DetailGroup icon={Building2} title="Satker & KLPD" fields={satker} />
    </TenderDetailShell>
  );
}
