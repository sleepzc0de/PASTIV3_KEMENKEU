"use client";

import { PackageSearch } from "lucide-react";
import { getEkatalog6ProdukPenyedia, syncEkatalog6ProdukPenyedia, Ekatalog6ProdukPenyediaItem } from "@/lib/api";
import { Ekatalog6ProdukPenyediaDetailModal } from "@/components/inaproc/Ekatalog6ProdukPenyediaDetailModal";
import { IsianKode, KolomTender, TenderCursorTable } from "@/components/inaproc/TenderCursorTable";

function Tayang({ ya }: { ya?: boolean | null }) {
  if (ya == null) return <span className="text-slate-400">-</span>;
  return (
    <span className={`whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium ${ya ? "bg-green-50 text-green-700" : "bg-slate-100 text-slate-600"}`}>
      {ya ? "Tayang" : "Tidak tayang"}
    </span>
  );
}

const KOLOM: KolomTender<Ekatalog6ProdukPenyediaItem>[] = [
  { judul: "Kode Produk", kelas: "whitespace-nowrap font-mono text-xs text-slate-600", render: (r) => r.kd_produk || "-" },
  { judul: "Nama Produk", kelas: "max-w-[420px] truncate font-medium text-slate-800", tooltip: (r) => r.nama_produk, render: (r) => r.nama_produk || "-" },
  { judul: "Status Produk", render: (r) => r.status_produk || "-" },
  { judul: "Penayangan", render: (r) => <Tayang ya={r.status_produk_tayang} /> },
];

const KODE: IsianKode = { label: "Kode Penyedia *", nama: "Kode Penyedia", placeholder: "mis. 01ABCXYZ123", wajib: true };

// Daftar produk satu penyedia (respons tidak memuat kode penyedia; kode yang dicari ikut disimpan saat sinkronisasi).
export function Ekatalog6ProdukPenyediaTable() {
  return (
    <TenderCursorTable<Ekatalog6ProdukPenyediaItem>
      ikon={PackageSearch}
      petunjukAwal="Isi Kode Penyedia untuk melihat daftar produknya di E-Katalog V6"
      keterangan="Daftar produk E-Katalog V6 berdasarkan kode penyedia."
      tanpaTahun
      kodeCari={KODE}
      ambil={(p) => getEkatalog6ProdukPenyedia({ kode_penyedia: p.kode ?? "", limit: p.limit, cursor: p.cursor })}
      sinkron={(p) => syncEkatalog6ProdukPenyedia({ kode: p.kode ?? "" })}
      kolom={KOLOM}
      kunciBaris={(r, i) => `${r.kd_produk}-${i}`}
      detail={(row, tutup) => <Ekatalog6ProdukPenyediaDetailModal item={row} onClose={tutup} />}
    />
  );
}
