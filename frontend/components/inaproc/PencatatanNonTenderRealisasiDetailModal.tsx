"use client";

import { FileText, Globe, Building2, UserRound, Briefcase, Receipt } from "lucide-react";
import { PencatatanNonTenderRealisasiItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: PencatatanNonTenderRealisasiItem;
  onClose: () => void;
}

// Nilai 0 tetap tampil; hanya null/undefined yang disembunyikan.
const uang = (v: number | null | undefined) => v != null && formatCurrency(v);
const teks = (v: string | number | null | undefined) => (v != null && v !== "" ? String(v) : null);

// dok_realisasi belum terdokumentasi: teks ditampilkan apa adanya, objek/larik sebagai JSON ringkas (tidak pernah "[object Object]").
function teksDokumen(v: unknown): string | null {
  if (v == null || v === "") return null;
  if (typeof v === "string" || typeof v === "number" || typeof v === "boolean") return String(v);
  try {
    return JSON.stringify(v);
  } catch {
    return null;
  }
}

export function PencatatanNonTenderRealisasiDetailModal({ item, onClose }: Props) {
  const realisasi: DetailField[] = [
    ["No. Realisasi", item.no_realisasi],
    ["Jenis Realisasi", item.jenis_realisasi],
    ["Nilai Realisasi", uang(item.nilai_realisasi)],
    ["Tgl. Realisasi", item.tgl_realisasi && formatDate(item.tgl_realisasi)],
    ["Keterangan", item.ket_realisasi],
    ["Dokumen Realisasi", teksDokumen(item.dok_realisasi)],
  ];

  const paket: DetailField[] = [
    ["Kode Pencatatan", teks(item.kd_nontender_pct)],
    ["Kode RUP Paket", item.kd_rup_paket],
    ["Kode Paket DCE", teks(item.kd_paket_dce)],
    ["Pagu", uang(item.pagu)],
  ];

  const penyedia: DetailField[] = [
    ["Nama Penyedia", item.nama_penyedia],
    ["NPWP", item.npwp_penyedia],
  ];

  const ppk: DetailField[] = [
    ["Nama PPK", item.nama_ppk],
    ["NIP PPK", item.nip_ppk],
  ];

  const lpse: DetailField[] = [
    ["Nama LPSE", item.nama_lpse],
    ["Kode LPSE", teks(item.kd_lpse)],
  ];

  const satker: DetailField[] = [
    ["Satuan Kerja", item.nama_satker],
    ["Kode Satker", item.kd_satker_str || item.kd_satker],
    ["KLPD", item.nama_klpd],
    ["Jenis KLPD", item.jenis_klpd],
  ];

  return (
    <TenderDetailShell
      judul="Detail Realisasi Non Tender"
      subjudul={`${item.no_realisasi ? `No. ${item.no_realisasi} · ` : ""}Kode Pencatatan ${item.kd_nontender_pct} · TA ${item.tahun_anggaran}`}
      nama={item.nama_paket}
      lokasi={item.nama_satker}
      onClose={onClose}
    >
      <DetailGroup icon={Receipt} title="Realisasi" fields={realisasi} />
      <DetailGroup icon={FileText} title="Paket" fields={paket} />
      <DetailGroup icon={Briefcase} title="Penyedia" fields={penyedia} />
      <DetailGroup icon={UserRound} title="PPK" fields={ppk} />
      <DetailGroup icon={Globe} title="LPSE" fields={lpse} />
      <DetailGroup icon={Building2} title="Satker & KLPD" fields={satker} />
    </TenderDetailShell>
  );
}
