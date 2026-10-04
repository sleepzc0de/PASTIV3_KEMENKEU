"use client";

import { FileText } from "lucide-react";
import { getTenderEkontrakKontrak, syncTenderEkontrakKontrak, TenderEkontrakKontrakItem } from "@/lib/api";
import { TenderEkontrakKontrakDetailModal } from "@/components/inaproc/TenderEkontrakKontrakDetailModal";
import { KOLOM_KONTRAK_TENDER } from "@/components/inaproc/KontrakTenderBersama";
import { TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

export function TenderEkontrakKontrakTable() {
  return (
    <TenderCursorTable<TenderEkontrakKontrakItem>
      ikon={FileText}
      petunjukAwal="Isi Tahun untuk melihat data kontrak tender Kementerian Keuangan"
      ambil={getTenderEkontrakKontrak}
      sinkron={syncTenderEkontrakKontrak}
      kolom={KOLOM_KONTRAK_TENDER}
      kunciBaris={(r, i) => `${r.kd_tender}-${r.no_kontrak ?? ""}-${i}`}
      detail={(row, tutup) => <TenderEkontrakKontrakDetailModal item={row} onClose={tutup} />}
    />
  );
}
