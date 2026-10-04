"use client";

import { Building2, Landmark } from "lucide-react";
import { EkatalogInstansiSatkerItem } from "@/lib/api";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: EkatalogInstansiSatkerItem;
  onClose: () => void;
}

const teks = (v: string | number | null | undefined) => (v != null && v !== "" ? String(v) : null);

export function EkatalogInstansiSatkerDetailModal({ item, onClose }: Props) {
  const satker: DetailField[] = [
    ["Nama Satker", item.nama_satker],
    ["Kode Satker", teks(item.kd_satker)],
    ["Kode Satker (bertitik)", item.kd_satker_str],
  ];

  const klpd: DetailField[] = [
    ["Nama KLPD", item.nama_klpd],
    ["Kode KLPD", item.kd_klpd],
    ["Jenis KLPD", item.jenis_klpd],
  ];

  return (
    <TenderDetailShell
      judul="Detail Instansi & Satker"
      subjudul={`Kode Satker ${item.kd_satker_str || item.kd_satker || "-"}`}
      nama={item.nama_satker}
      lokasi={item.nama_klpd}
      onClose={onClose}
    >
      <DetailGroup icon={Building2} title="Satuan Kerja" fields={satker} />
      <DetailGroup icon={Landmark} title="KLPD" fields={klpd} />
    </TenderDetailShell>
  );
}
