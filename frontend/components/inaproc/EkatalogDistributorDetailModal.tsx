"use client";

import { Truck, Phone } from "lucide-react";
import { EkatalogDistributorItem } from "@/lib/api";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: EkatalogDistributorItem;
  onClose: () => void;
}

const teks = (v: string | number | null | undefined) => (v != null && v !== "" ? String(v) : null);

export function EkatalogDistributorDetailModal({ item, onClose }: Props) {
  const distributor: DetailField[] = [
    ["Kode Distributor", teks(item.kd_penyedia_distributor)],
    ["Nama Distributor", item.nama_distributor],
    ["NPWP", item.npwp_distributor],
  ];

  const kontak: DetailField[] = [
    ["Alamat", item.alamat_distributor],
    ["Email", item.email_distributor],
    ["Telepon", item.no_telp_distributor],
  ];

  return (
    <TenderDetailShell
      judul="Detail Distributor E-Katalog"
      subjudul={`Kode Distributor ${item.kd_penyedia_distributor}`}
      nama={item.nama_distributor}
      onClose={onClose}
    >
      <DetailGroup icon={Truck} title="Distributor" fields={distributor} />
      <DetailGroup icon={Phone} title="Kontak" fields={kontak} />
    </TenderDetailShell>
  );
}
