"use client";

import { FileText, Handshake, UserRound, Receipt } from "lucide-react";
import { PencatatanSwakelolaRealisasiItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: PencatatanSwakelolaRealisasiItem;
  onClose: () => void;
}

// Nilai 0 tetap tampil; hanya null/undefined yang disembunyikan.
const uang = (v: number | null | undefined) => v != null && formatCurrency(v);
const teks = (v: string | number | null | undefined) => (v != null && v !== "" ? String(v) : null);

export function PencatatanSwakelolaRealisasiDetailModal({ item, onClose }: Props) {
  const realisasi: DetailField[] = [
    ["No. Realisasi", item.no_realisasi],
    ["Jenis Realisasi", item.jenis_realisasi],
    ["Nilai Realisasi", uang(item.nilai_realisasi)],
    ["Tgl. Realisasi", item.tgl_realisasi && formatDate(item.tgl_realisasi)],
    ["Keterangan", item.ket_realisasi],
    ["Dokumen Realisasi", item.dok_realisasi],
  ];

  const pelaksana: DetailField[] = [
    ["Nama Pelaksana", item.nama_pelaksana],
    ["NPWP", item.npwp_pelaksana],
  ];

  const ppk: DetailField[] = [
    ["Nama PPK", item.nama_ppk],
    ["NIP PPK", teks(item.nip_ppk)],
  ];

  // Respons tidak memuat nama paket/satker: yang tersedia hanya kode, untuk dicocokkan dengan halaman Pencatatan Swakelola.
  const paket: DetailField[] = [
    ["Kode Pencatatan", teks(item.kd_swakelola_pct)],
    ["ID Realisasi (rsk_id)", teks(item.rsk_id)],
    ["Kode Satker", item.kd_satker],
    ["Kode LPSE", teks(item.kd_lpse)],
    ["Kode KLPD", item.kd_klpd],
  ];

  return (
    <TenderDetailShell
      judul="Detail Realisasi Swakelola"
      subjudul={`${item.no_realisasi ? `No. ${item.no_realisasi} · ` : ""}Kode Pencatatan ${item.kd_swakelola_pct} · TA ${item.tahun_anggaran}`}
      nama={item.nama_pelaksana}
      onClose={onClose}
    >
      <DetailGroup icon={Receipt} title="Realisasi" fields={realisasi} />
      <DetailGroup icon={Handshake} title="Pelaksana" fields={pelaksana} />
      <DetailGroup icon={UserRound} title="PPK" fields={ppk} />
      <DetailGroup icon={FileText} title="Paket" fields={paket} />
    </TenderDetailShell>
  );
}
