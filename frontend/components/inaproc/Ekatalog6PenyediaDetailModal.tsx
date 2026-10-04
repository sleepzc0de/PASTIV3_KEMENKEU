"use client";

import { Briefcase, FileBadge, Phone } from "lucide-react";
import { Ekatalog6PenyediaItem } from "@/lib/api";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: Ekatalog6PenyediaItem;
  onClose: () => void;
}

const teks = (v: string | number | null | undefined) => (v != null && v !== "" ? String(v) : null);
const ya = (v: number | null | undefined) => (v === 1 ? "Ya" : v === 0 ? "Tidak" : null);

export function Ekatalog6PenyediaDetailModal({ item, onClose }: Props) {
  const penyedia: DetailField[] = [
    ["Kode Penyedia", item.kode_penyedia],
    ["Nama Penyedia", item.nama_penyedia],
    ["Bentuk Usaha", item.bentuk_usaha?.replace(/_/g, " ")],
    ["Jenis Perusahaan", item.jenis_perusahaan],
    ["Status", item.status_aktif],
    ["UMKK", ya(item.status_umkk)],
    ["ID Rekan", teks(item.rekan_id)],
  ];

  // Beberapa kode/nama KBLI dipisah koma; ditampilkan apa adanya.
  const legal: DetailField[] = [
    ["NIB", item.nib],
    ["NPWP", item.npwp_penyedia],
    ["Kode KBLI", item.kbli_id],
    ["Nama KBLI", item.kbli_name],
  ];

  const kontak: DetailField[] = [
    ["Alamat", item.alamat_penyedia],
    ["Email", item.email],
    ["Telepon", item.telepon],
  ];

  return (
    <TenderDetailShell judul="Detail Penyedia E-Katalog V6" subjudul={`Kode Penyedia ${item.kode_penyedia}`} nama={item.nama_penyedia} onClose={onClose}>
      <DetailGroup icon={Briefcase} title="Penyedia" fields={penyedia} />
      <DetailGroup icon={FileBadge} title="Legalitas" fields={legal} />
      <DetailGroup icon={Phone} title="Kontak" fields={kontak} />
    </TenderDetailShell>
  );
}
