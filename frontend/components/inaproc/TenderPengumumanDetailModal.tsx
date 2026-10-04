"use client";

import { FileText, Banknote, CalendarDays, Globe, Building2, UserRound, Scale } from "lucide-react";
import { TenderPengumumanItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: TenderPengumumanItem;
  onClose: () => void;
}

// Nilai 0 tetap tampil; hanya null/undefined yang disembunyikan.
const uang = (v: number | null | undefined) => v != null && formatCurrency(v);
const teks = (v: string | number | null | undefined) => (v != null && v !== "" ? String(v) : null);

export function TenderPengumumanDetailModal({ item, onClose }: Props) {
  const paket: DetailField[] = [
    ["Kode Tender", teks(item.kd_tender)],
    ["Versi Tender", teks(item.versi_tender)],
    ["Kode RUP", item.kd_rup],
    ["Jenis Pengadaan", item.jenis_pengadaan],
    ["Kualifikasi Paket", item.kualifikasi_paket],
    ["Kontrak Pembayaran", item.kontrak_pembayaran],
    ["Sumber Dana", item.sumber_dana],
    ["Lokasi Pekerjaan", item.lokasi_pekerjaan],
    ["Tahun Anggaran", teks(item.list_tahun_anggaran) ?? teks(item.tahun_anggaran)],
  ];

  const metode: DetailField[] = [
    ["Metode Pemilihan", item.mtd_pemilihan],
    ["Metode Kualifikasi", item.mtd_kualifikasi],
    ["Metode Evaluasi", item.mtd_evaluasi],
  ];

  const nilai: DetailField[] = [
    ["Pagu", uang(item.pagu)],
    ["HPS", uang(item.hps)],
  ];

  const pejabat: DetailField[] = [
    ["Nama PPK", item.nama_ppk],
    ["NIP PPK", item.nip_ppk],
    ["Nama Pokja", item.nama_pokja],
    ["NIP Pokja", item.nip_pokja],
  ];

  const status: DetailField[] = [
    ["Status", item.status_tender],
    ["Tgl. Status", item.tanggal_status && formatDate(item.tanggal_status)],
    ["Keterangan Ditutup", item.ket_ditutup],
    ["Keterangan Diulang", item.ket_diulang],
    ["Tgl. Buat Paket", item.tgl_buat_paket && formatDate(item.tgl_buat_paket)],
    ["Tgl. Kolektif Kolegial", item.tgl_kolektif_kolegial && formatDate(item.tgl_kolektif_kolegial)],
    ["Tgl. Pengumuman", item.tgl_pengumuman_tender && formatDate(item.tgl_pengumuman_tender)],
  ];

  const lpse: DetailField[] = [
    ["Nama LPSE", item.nama_lpse],
    ["Kode LPSE", teks(item.kd_lpse)],
    ["Kode Paket DCE", teks(item.kd_pkt_dce)],
    ["URL LPSE", item.url_lpse],
  ];

  const satker: DetailField[] = [
    ["Satuan Kerja", item.nama_satker],
    ["Kode Satker", item.kd_satker_str || item.kd_satker],
    ["KLPD", item.nama_klpd],
    ["Jenis KLPD", item.jenis_klpd],
  ];

  return (
    <TenderDetailShell
      judul="Detail Pengumuman Tender"
      subjudul={`Kode Tender ${item.kd_tender} · TA ${item.tahun_anggaran}`}
      nama={item.nama_paket}
      lokasi={item.nama_satker}
      onClose={onClose}
    >
      <DetailGroup icon={FileText} title="Paket" fields={paket} />
      <DetailGroup icon={Scale} title="Metode" fields={metode} />
      <DetailGroup icon={Banknote} title="Anggaran" fields={nilai} />
      <DetailGroup icon={UserRound} title="PPK & Pokja" fields={pejabat} />
      <DetailGroup icon={CalendarDays} title="Status & Jadwal" fields={status} />
      <DetailGroup icon={Globe} title="LPSE" fields={lpse} />
      <DetailGroup icon={Building2} title="Satker & KLPD" fields={satker} />
    </TenderDetailShell>
  );
}
