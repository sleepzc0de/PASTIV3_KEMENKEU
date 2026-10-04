"use client";

import { Tags } from "lucide-react";
import { getEkatalogKomoditas, syncEkatalogKomoditas, EkatalogKomoditasItem } from "@/lib/api";
import { EkatalogKomoditasDetailModal } from "@/components/inaproc/EkatalogKomoditasDetailModal";
import { IsianKode, KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

const KOLOM: KolomTender<EkatalogKomoditasItem>[] = [
  { judul: "Kode Komoditas", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.kd_komoditas ?? "-" },
  { judul: "Nama Komoditas", kelas: "max-w-[360px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_komoditas, render: (r) => r.nama_komoditas || "-" },
  { judul: "Jenis Katalog", render: (r) => r.Jenis_Katalog || "-" },
  { judul: "Instansi Katalog", kelas: "max-w-[260px] truncate text-slate-600", tooltip: (r) => r.nama_instansi_katalog, render: (r) => r.nama_instansi_katalog || "-" },
];

const KODE: IsianKode = { label: "Kode Komoditas *", nama: "Kode Komoditas", placeholder: "mis. 999", wajib: true };

// Pencarian rujukan per satu kode komoditas.
export function EkatalogKomoditasTable() {
  return (
    <TenderCursorTable<EkatalogKomoditasItem>
      ikon={Tags}
      petunjukAwal="Isi Kode Komoditas untuk melihat detail komoditas E-Katalog"
      keterangan="Pencarian rujukan komoditas E-Katalog V5 (archive) berdasarkan kode komoditas."
      tanpaTahun
      kodeCari={KODE}
      ambil={(p) => getEkatalogKomoditas({ kode_komoditas: p.kode ?? "", limit: p.limit, cursor: p.cursor })}
      sinkron={(p) => syncEkatalogKomoditas({ kode: p.kode ?? "" })}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.kd_komoditas}-${r.kd_instansi_katalog ?? ""}-${i}`}
      detail={(row, tutup) => <EkatalogKomoditasDetailModal item={row} onClose={tutup} />}
    />
  );
}
