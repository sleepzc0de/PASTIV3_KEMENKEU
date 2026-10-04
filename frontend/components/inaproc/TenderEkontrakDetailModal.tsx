"use client";

import { TenderEkontrakItem } from "@/lib/api";
import { KontrakTenderGroups } from "@/components/inaproc/KontrakTenderBersama";
import { RiwayatEkontrak } from "@/components/inaproc/RiwayatEkontrak";
import { TenderDetailShell } from "@/components/inaproc/TenderDetailShell";

interface Props {
  item: TenderEkontrakItem;
  onClose: () => void;
}

// Kontrak (kelompok yang sama dengan Kontrak Tender) ditambah riwayat BAP/BAST, SPMK/SPP, dan penilaian kinerja penyedia.
export function TenderEkontrakDetailModal({ item, onClose }: Props) {
  return (
    <TenderDetailShell
      judul="Detail Tender E-Kontrak"
      subjudul={`Kode Tender ${item.kd_tender}${item.no_kontrak ? ` · No. Kontrak ${item.no_kontrak}` : ""} · TA ${item.tahun_anggaran}`}
      nama={item.nama_paket}
      lokasi={item.alamat_satker || item.nama_satker}
      onClose={onClose}
    >
      <KontrakTenderGroups item={item} />
      <RiwayatEkontrak bapBast={item.bapbast_history_json} spmkSpp={item.spmkspp_history_json} penilaian={item.penilaian_kinerja_penyedia} />
    </TenderDetailShell>
  );
}
