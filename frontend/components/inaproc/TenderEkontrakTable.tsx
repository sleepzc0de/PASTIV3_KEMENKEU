"use client";

import { FileCheck2 } from "lucide-react";
import { getTenderEkontrak, syncTenderEkontrak, TenderEkontrakItem } from "@/lib/api";
import { TenderEkontrakDetailModal } from "@/components/inaproc/TenderEkontrakDetailModal";
import { KOLOM_KONTRAK_TENDER } from "@/components/inaproc/KontrakTenderBersama";
import { TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

export function TenderEkontrakTable() {
  return (
    <TenderCursorTable<TenderEkontrakItem>
      ikon={FileCheck2}
      petunjukAwal="Isi Tahun untuk melihat e-kontrak tender Kementerian Keuangan"
      ambil={getTenderEkontrak}
      sinkron={syncTenderEkontrak}
      kolom={KOLOM_KONTRAK_TENDER}
      kunciBaris={(r, i) => `${r.kd_tender}-${r.no_kontrak ?? ""}-${i}`}
      detail={(row, tutup) => <TenderEkontrakDetailModal item={row} onClose={tutup} />}
    />
  );
}
