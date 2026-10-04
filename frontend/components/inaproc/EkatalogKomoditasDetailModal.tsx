"use client";

import { Tags, Landmark } from "lucide-react";
import { EkatalogKomoditasItem } from "@/lib/api";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: EkatalogKomoditasItem;
  onClose: () => void;
}

const teks = (v: string | number | null | undefined) => (v != null && v !== "" ? String(v) : null);

export function EkatalogKomoditasDetailModal({ item, onClose }: Props) {
  const komoditas: DetailField[] = [
    ["Kode Komoditas", teks(item.kd_komoditas)],
    ["Nama Komoditas", item.nama_komoditas],
    ["Jenis Katalog", item.Jenis_Katalog],
  ];

  const instansi: DetailField[] = [
    ["Kode Instansi Katalog", teks(item.kd_instansi_katalog)],
    ["Nama Instansi Katalog", item.nama_instansi_katalog],
  ];

  return (
    <TenderDetailShell
      judul="Detail Komoditas E-Katalog"
      subjudul={`Kode Komoditas ${item.kd_komoditas}`}
      nama={item.nama_komoditas}
      onClose={onClose}
    >
      <DetailGroup icon={Tags} title="Komoditas" fields={komoditas} />
      <DetailGroup icon={Landmark} title="Instansi Katalog" fields={instansi} />
    </TenderDetailShell>
  );
}
