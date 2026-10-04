"use client";

import { Banknote, Building2, Gavel, Trophy, Briefcase } from "lucide-react";
import { PesertaTenderItem } from "@/lib/api";
import { formatCurrency } from "@/lib/format";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: PesertaTenderItem;
  onClose: () => void;
}

// Nilai 0 tetap tampil; hanya null/undefined yang disembunyikan.
const uang = (v: number | null | undefined) => v != null && formatCurrency(v);
const teks = (v: string | number | null | undefined) => (v != null && v !== "" ? String(v) : null);
const penanda = (v: number | null | undefined) => (v === 1 ? "Ya" : v === 0 ? "Tidak" : null);

export function PesertaTenderDetailModal({ item, onClose }: Props) {
  const penyedia: DetailField[] = [
    ["Nama Penyedia", item.nama_penyedia],
    ["Kode Penyedia", teks(item.kd_penyedia)],
    ["NPWP", item.npwp_penyedia],
    ["NPWP 16 Digit", item.npwp_penyedia_16],
  ];

  const penawaran: DetailField[] = [
    ["Nilai Penawaran", uang(item.nilai_penawaran)],
    ["Nilai Terkoreksi", uang(item.nilai_terkoreksi)],
  ];

  const hasil: DetailField[] = [
    ["Pemenang", penanda(item.pemenang)],
    ["Pemenang Terverifikasi", penanda(item.pemenang_terverifikasi)],
    ["Alasan", item.alasan],
  ];

  const tender: DetailField[] = [
    ["Kode Tender", teks(item.kd_tender)],
    ["Kode Peserta", teks(item.kd_peserta)],
    ["Kode Paket DCE", teks(item.kd_pkt_dce)],
    ["Kode LPSE", teks(item.kd_lpse)],
    ["Kode Satker", item.kd_satker_str || item.kd_satker],
    ["Kode KLPD", item.kd_klpd],
  ];

  return (
    <TenderDetailShell
      judul="Detail Peserta Tender"
      subjudul={`Kode Tender ${item.kd_tender} · Peserta ${item.kd_peserta} · TA ${item.tahun_anggaran}`}
      nama={item.nama_penyedia}
      onClose={onClose}
    >
      <DetailGroup icon={Briefcase} title="Penyedia" fields={penyedia} />
      <DetailGroup icon={Banknote} title="Penawaran" fields={penawaran} />
      <DetailGroup icon={Trophy} title="Hasil" fields={hasil} />
      <DetailGroup icon={Gavel} title="Tender" fields={tender} />
    </TenderDetailShell>
  );
}
