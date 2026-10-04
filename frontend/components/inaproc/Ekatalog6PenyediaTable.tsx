"use client";

import { Factory } from "lucide-react";
import { getEkatalog6Penyedia, syncEkatalog6Penyedia, Ekatalog6PenyediaItem } from "@/lib/api";
import { Ekatalog6PenyediaDetailModal } from "@/components/inaproc/Ekatalog6PenyediaDetailModal";
import { IsianKode, KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

function Aktif({ status }: { status?: string | null }) {
  if (!status) return <span className="text-slate-400">-</span>;
  const aktif = status.toLowerCase() === "active";
  return (
    <span className={`whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium ${aktif ? "bg-green-50 text-green-700" : "bg-slate-100 text-slate-600"}`}>
      {status}
    </span>
  );
}

const KOLOM: KolomTender<Ekatalog6PenyediaItem>[] = [
  { judul: "Kode Penyedia", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.kode_penyedia || "-" },
  { judul: "Nama Penyedia", kelas: "max-w-[280px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_penyedia, render: (r) => r.nama_penyedia || "-" },
  { judul: "NIB", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.nib || "-" },
  { judul: "NPWP", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.npwp_penyedia || "-" },
  { judul: "Bentuk Usaha", kelas: "max-w-[180px] truncate text-slate-600", tooltip: (r) => r.bentuk_usaha, render: (r) => (r.bentuk_usaha || "-").replace(/_/g, " ") },
  { judul: "Status", render: (r) => <Aktif status={r.status_aktif} /> },
  { judul: "Telepon", kelas: "whitespace-nowrap text-slate-600", render: (r) => r.telepon || "-" },
];

const KODE: IsianKode = { label: "Kode Penyedia *", nama: "Kode Penyedia", placeholder: "mis. 01ABCXYZ123", wajib: true };

// Pencarian rujukan per satu kode penyedia (kode teks).
export function Ekatalog6PenyediaTable() {
  return (
    <TenderCursorTable<Ekatalog6PenyediaItem>
      ikon={Factory}
      petunjukAwal="Isi Kode Penyedia untuk melihat detail penyedia E-Katalog V6"
      keterangan="Pencarian rujukan penyedia E-Katalog V6 berdasarkan kode penyedia."
      tanpaTahun
      kodeCari={KODE}
      ambil={(p) => getEkatalog6Penyedia({ kode_penyedia: p.kode ?? "", limit: p.limit, cursor: p.cursor })}
      sinkron={(p) => syncEkatalog6Penyedia({ kode: p.kode ?? "" })}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.kode_penyedia}-${i}`}
      detail={(row, tutup) => <Ekatalog6PenyediaDetailModal item={row} onClose={tutup} />}
    />
  );
}
