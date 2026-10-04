"use client";

import { FileText, Banknote, CalendarDays, Globe, Building2, UserRound, StickyNote } from "lucide-react";
import { PencatatanNonTenderItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: PencatatanNonTenderItem;
  onClose: () => void;
}

// Nilai 0 tetap tampil; hanya null/undefined yang disembunyikan.
const uang = (v: number | null | undefined) => v != null && formatCurrency(v);
const teks = (v: string | number | null | undefined) => (v != null && v !== "" ? String(v) : null);

export function PencatatanNonTenderDetailModal({ item, onClose }: Props) {
  const paket: DetailField[] = [
    ["Kode Pencatatan", teks(item.kd_nontender_pct)],
    ["Kode RUP", item.kd_rup],
    ["Kategori Pengadaan", item.kategori_pengadaan],
    ["Metode Pemilihan", item.mtd_pemilihan],
    ["Sumber Dana", item.sumber_dana],
    ["Uraian Pekerjaan", item.uraian_pekerjaan],
  ];

  const nilai: DetailField[] = [
    ["Pagu", uang(item.pagu)],
    ["Total Realisasi", uang(item.total_realisasi)],
    ["Nilai PDN", uang(item.nilai_pdn_pct)],
    ["Nilai UMK", uang(item.nilai_umk_pct)],
  ];

  const ppk: DetailField[] = [
    ["Nama PPK", item.nama_ppk],
    ["NIP PPK", item.nip_ppk],
  ];

  const status: DetailField[] = [
    ["Status", item.status_nontender_pct],
    ["Keterangan Status", item.status_nontender_pct_ket],
    ["Alasan Pembatalan", item.alasan_pembatalan],
    ["Tgl. Buat Paket", item.tgl_buat_paket && formatDate(item.tgl_buat_paket)],
    ["Tgl. Mulai", item.tgl_mulai_paket && formatDate(item.tgl_mulai_paket)],
    ["Tgl. Selesai", item.tgl_selesai_paket && formatDate(item.tgl_selesai_paket)],
  ];

  const lainnya: DetailField[] = [
    ["Bukti Pembayaran", item.bukti_pembayaran],
    ["Informasi Lainnya", item.informasi_lainnya],
  ];

  const lpse: DetailField[] = [
    ["Kode LPSE", teks(item.kd_lpse)],
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
      judul="Detail Pencatatan Non Tender"
      subjudul={`Kode Pencatatan ${item.kd_nontender_pct} · TA ${item.tahun_anggaran}`}
      nama={item.nama_paket}
      lokasi={item.nama_satker}
      onClose={onClose}
    >
      <DetailGroup icon={FileText} title="Paket" fields={paket} />
      <DetailGroup icon={Banknote} title="Anggaran & Realisasi" fields={nilai} />
      <DetailGroup icon={UserRound} title="PPK" fields={ppk} />
      <DetailGroup icon={CalendarDays} title="Status & Jadwal" fields={status} />
      <DetailGroup icon={StickyNote} title="Lainnya" fields={lainnya} />
      <DetailGroup icon={Globe} title="LPSE" fields={lpse} />
      <DetailGroup icon={Building2} title="Satker & KLPD" fields={satker} />
    </TenderDetailShell>
  );
}
