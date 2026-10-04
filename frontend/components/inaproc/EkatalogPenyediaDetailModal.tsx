"use client";

import { Briefcase, Phone, FileBadge } from "lucide-react";
import { EkatalogPenyediaItem } from "@/lib/api";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: EkatalogPenyediaItem;
  onClose: () => void;
}

const teks = (v: string | number | null | undefined) => (v != null && v !== "" ? String(v) : null);

export function EkatalogPenyediaDetailModal({ item, onClose }: Props) {
  const penyedia: DetailField[] = [
    ["Kode Penyedia", teks(item.kd_penyedia)],
    ["Kode Penyedia SIKaP", teks(item.kode_penyedia_sikap)],
    ["Nama Penyedia", item.nama_penyedia],
    ["Jenis Usaha", item.penyedia_ukm],
  ];

  const legal: DetailField[] = [
    ["NPWP", item.npwp_penyedia],
    ["NPWP 16 Digit", item.npwp_16],
    // Bisa berisi banyak kode dipisah titik koma; ditampilkan apa adanya.
    ["KBLI 2020", item.kbli2020_penyedia],
  ];

  const kontak: DetailField[] = [
    ["Alamat", item.alamat_penyedia],
    ["Email", item.email_penyedia],
    ["Telepon", item.no_telp_penyedia],
  ];

  return (
    <TenderDetailShell judul="Detail Penyedia E-Katalog" subjudul={`Kode Penyedia ${item.kd_penyedia}`} nama={item.nama_penyedia} onClose={onClose}>
      <DetailGroup icon={Briefcase} title="Penyedia" fields={penyedia} />
      <DetailGroup icon={FileBadge} title="Legalitas" fields={legal} />
      <DetailGroup icon={Phone} title="Kontak" fields={kontak} />
    </TenderDetailShell>
  );
}
