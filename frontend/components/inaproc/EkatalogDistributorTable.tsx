"use client";

import { Truck } from "lucide-react";
import { getEkatalogDistributor, syncEkatalogDistributor, EkatalogDistributorItem } from "@/lib/api";
import { EkatalogDistributorDetailModal } from "@/components/inaproc/EkatalogDistributorDetailModal";
import { IsianKode, KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

const KOLOM: KolomTender<EkatalogDistributorItem>[] = [
  { judul: "Kode Distributor", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.kd_penyedia_distributor ?? "-" },
  { judul: "Nama Distributor", kelas: "max-w-[300px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_distributor, render: (r) => r.nama_distributor || "-" },
  { judul: "NPWP", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.npwp_distributor || "-" },
  { judul: "Telepon", kelas: "whitespace-nowrap text-slate-600", render: (r) => r.no_telp_distributor || "-" },
  { judul: "Email", kelas: "max-w-[220px] truncate text-slate-600", tooltip: (r) => r.email_distributor, render: (r) => r.email_distributor || "-" },
  { judul: "Alamat", kelas: "max-w-[260px] truncate text-slate-600", tooltip: (r) => r.alamat_distributor, render: (r) => r.alamat_distributor || "-" },
];

const KODE: IsianKode = { label: "Kode Distributor *", nama: "Kode Distributor", placeholder: "mis. 8888", wajib: true };

// Pencarian rujukan per satu kode distributor.
export function EkatalogDistributorTable() {
  return (
    <TenderCursorTable<EkatalogDistributorItem>
      ikon={Truck}
      petunjukAwal="Isi Kode Distributor untuk melihat detail distributor E-Katalog"
      keterangan="Pencarian rujukan distributor E-Katalog V5 (archive) berdasarkan kode distributor."
      tanpaTahun
      kodeCari={KODE}
      ambil={(p) => getEkatalogDistributor({ kd_distributor: p.kode ?? "", limit: p.limit, cursor: p.cursor })}
      sinkron={(p) => syncEkatalogDistributor({ kode: p.kode ?? "" })}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.kd_penyedia_distributor}-${i}`}
      detail={(row, tutup) => <EkatalogDistributorDetailModal item={row} onClose={tutup} />}
    />
  );
}
