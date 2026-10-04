"use client";

import { Factory } from "lucide-react";
import { getEkatalogPenyedia, syncEkatalogPenyedia, EkatalogPenyediaItem } from "@/lib/api";
import { EkatalogPenyediaDetailModal } from "@/components/inaproc/EkatalogPenyediaDetailModal";
import { IsianKode, KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

const KOLOM: KolomTender<EkatalogPenyediaItem>[] = [
  { judul: "Kode Penyedia", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.kd_penyedia ?? "-" },
  { judul: "Nama Penyedia", kelas: "max-w-[300px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_penyedia, render: (r) => r.nama_penyedia || "-" },
  { judul: "NPWP", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.npwp_penyedia || r.npwp_16 || "-" },
  { judul: "Usaha", render: (r) => r.penyedia_ukm || "-" },
  { judul: "Telepon", kelas: "whitespace-nowrap text-slate-600", render: (r) => r.no_telp_penyedia || "-" },
  { judul: "Email", kelas: "max-w-[220px] truncate text-slate-600", tooltip: (r) => r.email_penyedia, render: (r) => r.email_penyedia || "-" },
];

const KODE: IsianKode = { label: "Kode Penyedia *", nama: "Kode Penyedia", placeholder: "mis. 42", wajib: true };

// Pencarian rujukan per satu kode penyedia.
export function EkatalogPenyediaTable() {
  return (
    <TenderCursorTable<EkatalogPenyediaItem>
      ikon={Factory}
      petunjukAwal="Isi Kode Penyedia untuk melihat detail penyedia E-Katalog"
      keterangan="Pencarian rujukan penyedia E-Katalog V5 (archive) berdasarkan kode penyedia."
      tanpaTahun
      kodeCari={KODE}
      ambil={(p) => getEkatalogPenyedia({ kode_penyedia: p.kode ?? "", limit: p.limit, cursor: p.cursor })}
      sinkron={(p) => syncEkatalogPenyedia({ kode: p.kode ?? "" })}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.kd_penyedia}-${i}`}
      detail={(row, tutup) => <EkatalogPenyediaDetailModal item={row} onClose={tutup} />}
    />
  );
}
