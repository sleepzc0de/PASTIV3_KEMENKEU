"use client";

import { FileText, Banknote, Building2, UserRound, Info } from "lucide-react";
import { TenderEkontrakKontrakItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { StatusBadge } from "@/components/inaproc/StatusBadge";
import { KolomTender } from "@/components/inaproc/TenderCursorTable";

// Bagian yang sama antara Tender E-Kontrak (dengan riwayat) dan Kontrak Tender: kolom tabel dan kelompok field di modal.
// Dipakai juga untuk TenderEkontrakItem karena tipe itu memperluas TenderEkontrakKontrakItem.

export const KOLOM_KONTRAK_TENDER: KolomTender<TenderEkontrakKontrakItem>[] = [
  { judul: "Kode Tender", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.kd_tender || "-" },
  { judul: "No. Kontrak", kelas: "max-w-[180px] truncate text-slate-600", tooltip: (r) => r.no_kontrak, render: (r) => r.no_kontrak || "-" },
  { judul: "Nama Paket", kelas: "max-w-[280px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_paket, render: (r) => r.nama_paket || "-" },
  { judul: "Penyedia", kelas: "max-w-[220px] truncate text-slate-600", tooltip: (r) => r.nama_penyedia, render: (r) => r.nama_penyedia || "-" },
  { judul: "Nilai Kontrak", rata: "kanan", kelas: "whitespace-nowrap font-medium text-slate-800", render: (r) => formatCurrency(r.nilai_kontrak) },
  { judul: "Status", render: (r) => <StatusBadge status={r.status_kontrak} /> },
  { judul: "Tgl. Kontrak", kelas: "whitespace-nowrap text-xs text-slate-500", render: (r) => formatDate(r.tgl_kontrak) },
];

// Nilai 0 tetap tampil; hanya null/undefined yang disembunyikan.
const uang = (v: number | null | undefined) => v != null && formatCurrency(v);
const teks = (v: string | number | null | undefined) => (v != null && v !== "" ? String(v) : null);

export function KontrakTenderGroups({ item }: { item: TenderEkontrakKontrakItem }) {
  const addendum =
    item.apakah_addendum && item.versi_addendum != null && item.versi_addendum > 0
      ? `${item.apakah_addendum} (versi ${item.versi_addendum})`
      : item.apakah_addendum;

  const kontrak: DetailField[] = [
    ["No. Kontrak", item.no_kontrak],
    ["No. SPPBJ", item.no_sppbj],
    ["Jenis Kontrak", item.jenis_kontrak],
    ["Status Kontrak", item.status_kontrak],
    ["Tgl. Penetapan Status", item.tgl_penetapan_status_kontrak && formatDate(item.tgl_penetapan_status_kontrak)],
    ["Alasan Penetapan Status", item.alasan_penetapan_status_kontrak],
    ["Tgl. Kontrak", item.tgl_kontrak && formatDate(item.tgl_kontrak)],
    ["Tgl. Mulai Kontrak", item.tgl_kontrak_awal && formatDate(item.tgl_kontrak_awal)],
    ["Tgl. Akhir Kontrak", item.tgl_kontrak_akhir && formatDate(item.tgl_kontrak_akhir)],
    ["Kota Kontrak", item.kota_kontrak],
    ["Addendum", addendum],
    ["Alasan Addendum", item.alasan_addendum],
  ];

  const nilai: DetailField[] = [
    ["Nilai Kontrak", uang(item.nilai_kontrak)],
    ["Nilai PDN", uang(item.nilai_pdn_kontrak)],
    ["Nilai UMK", uang(item.nilai_umk_kontrak)],
    ["Alasan Ubah Nilai Kontrak", item.alasan_ubah_nilai_kontrak],
    ["Alasan Nilai Kontrak 10 Persen", item.alasan_nilai_kontrak_10_persen],
  ];

  const penyedia: DetailField[] = [
    ["Nama Penyedia", item.nama_penyedia],
    ["Kode Penyedia", teks(item.kd_penyedia)],
    ["Bentuk Usaha", item.bentuk_usaha_penyedia],
    ["Tipe Penyedia", item.tipe_penyedia],
    ["NPWP", item.npwp_penyedia],
    ["NPWP 16 Digit", item.npwp_16_penyedia],
    ["Wakil Sah Penyedia", item.wakil_sah_penyedia],
    ["Jabatan Wakil Penyedia", item.jabatan_wakil_penyedia],
    ["Anggota KSO", item.anggota_kso],
    ["Bank", item.nama_rek_bank],
    ["No. Rekening", item.no_rek_bank],
    ["Pemilik Rekening", item.nama_pemilik_rek_bank],
  ];

  const ppk: DetailField[] = [
    ["Nama PPK", item.nama_ppk],
    ["NIP PPK", item.nip_ppk],
    ["Jabatan PPK", item.jabatan_ppk],
    ["No. SK PPK", item.no_sk_ppk],
  ];

  const lainnya: DetailField[] = [
    ["Lingkup Pekerjaan", item.lingkup_pekerjaan],
    ["Informasi Lainnya", item.informasi_lainnya],
    ["Satuan Kerja", item.nama_satker],
    ["Kode Satker", item.kd_satker_str || item.kd_satker],
    ["KLPD", item.nama_klpd],
    ["Jenis KLPD", item.jenis_klpd],
    ["Kode Tender", teks(item.kd_tender)],
    ["Kode LPSE", teks(item.kd_lpse)],
  ];

  return (
    <>
      <DetailGroup icon={FileText} title="Kontrak" fields={kontrak} />
      <DetailGroup icon={Banknote} title="Nilai Kontrak" fields={nilai} />
      <DetailGroup icon={Building2} title="Penyedia" fields={penyedia} />
      <DetailGroup icon={UserRound} title="Pejabat Pembuat Komitmen (PPK)" fields={ppk} />
      <DetailGroup icon={Info} title="Informasi Lainnya" fields={lainnya} />
    </>
  );
}
