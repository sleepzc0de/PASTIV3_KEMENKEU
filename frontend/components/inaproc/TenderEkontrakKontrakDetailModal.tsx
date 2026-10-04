"use client";

import { TenderEkontrakKontrakItem } from "@/lib/api";
import { KontrakTenderGroups } from "@/components/inaproc/KontrakTenderBersama";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: TenderEkontrakKontrakItem;
  onClose: () => void;
}

export function TenderEkontrakKontrakDetailModal({ item, onClose }: Props) {
  return (
    <TenderDetailShell
      judul="Detail Kontrak Tender"
      subjudul={`No. Kontrak ${item.no_kontrak || "-"} · Kode Tender ${item.kd_tender} · TA ${item.tahun_anggaran}`}
      nama={item.nama_paket}
      lokasi={item.alamat_satker || item.nama_satker}
      onClose={onClose}
    >
      <KontrakTenderGroups item={item} />
    </TenderDetailShell>
  );
}
