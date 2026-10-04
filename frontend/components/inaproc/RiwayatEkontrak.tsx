"use client";

import { ReactNode } from "react";
import { FileText, ClipboardList, Star } from "lucide-react";
import { BapBastHistoryItem, SpmkSppHistoryItem, PenilaianKinerjaPenyediaItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { DetailEntry } from "@/components/inaproc/DetailEntry";

// Tiga riwayat e-kontrak: BAP/BAST, SPMK/SPP, dan penilaian kinerja penyedia. Larik yang kosong (atau tidak dikirim) tampil
// sebagai "Belum ada data".
interface Props {
  bapBast?: BapBastHistoryItem[] | null;
  spmkSpp?: SpmkSppHistoryItem[] | null;
  penilaian?: PenilaianKinerjaPenyediaItem[] | null;
}

export function RiwayatEkontrak({ bapBast, spmkSpp, penilaian }: Props) {
  const bap = bapBast ?? [];
  const spmk = spmkSpp ?? [];
  const nilai = penilaian ?? [];

  return (
    <>
      <Section icon={FileText} title="Riwayat BAP / BAST" count={bap.length}>
        {bap.map((h, idx) => (
          <DetailEntry
            key={idx}
            fields={[
              ["No. BAP", h.no_bap],
              ["Tgl. BAP", h.tgl_bap && formatDate(h.tgl_bap)],
              ["No. BAST", h.no_bast],
              ["Tgl. BAST", h.tgl_bast && formatDate(h.tgl_bast)],
              ["Besar Pembayaran", h.besar_pembayaran != null && formatCurrency(h.besar_pembayaran)],
              ["Progres Pekerjaan", h.progres_pekerjaan != null && `${h.progres_pekerjaan}%`],
              ["Wakil Sah Penyedia", h.wakil_sah_penyedia],
              ["Jabatan Wakil Penyedia", h.jabatan_wakil_penyedia],
              ["Jabatan Penandatangan SK", h.jabatan_penandatangan_sk],
            ]}
          />
        ))}
      </Section>

      <Section icon={ClipboardList} title="Riwayat SPMK / SPP" count={spmk.length}>
        {spmk.map((h, idx) => (
          <DetailEntry
            key={idx}
            fields={[
              ["No. SPMK/SPP", h.no_spmk_spp],
              ["Tgl. SPMK/SPP", h.tgl_spmk_spp && formatDate(h.tgl_spmk_spp)],
              ["Mulai Pekerjaan", h.tgl_mulai_pekerjaan && formatDate(h.tgl_mulai_pekerjaan)],
              ["Selesai Pekerjaan", h.tgl_selesai_pekerjaan && formatDate(h.tgl_selesai_pekerjaan)],
              ["Waktu Penyelesaian", h.waktu_penyelesaian],
              ["Kota", h.kota_spmk_spp],
              ["Alamat Pengiriman", h.alamat_pengiriman],
              ["Wakil Sah Penyedia", h.wakil_sah_penyedia],
              ["Jabatan Wakil Penyedia", h.jabatan_wakil_penyedia],
            ]}
          />
        ))}
      </Section>

      <Section icon={Star} title="Penilaian Kinerja Penyedia" count={nilai.length}>
        {nilai.map((p, idx) => (
          <div key={idx} className="flex items-center justify-between gap-3 rounded-lg border border-slate-200 px-3 py-2.5 text-sm">
            <span className="min-w-0 text-slate-700">{p.indikator_penilaian || "-"}</span>
            <span className="shrink-0 rounded-full bg-blue-50 px-2.5 py-0.5 text-xs font-semibold text-blue-700">{p.nilai_indikator ?? "-"}</span>
          </div>
        ))}
      </Section>
    </>
  );
}

function Section({ icon: Icon, title, count, children }: { icon: React.ElementType; title: string; count: number; children: ReactNode }) {
  return (
    <section>
      <h3 className="mb-2 flex items-center gap-1.5 text-sm font-semibold text-slate-700">
        <Icon className="h-4 w-4" />
        {title}
        <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-500">{count}</span>
      </h3>
      {count === 0 ? <p className="rounded-lg bg-slate-50 px-3 py-4 text-center text-xs text-slate-500">Belum ada data.</p> : <div className="space-y-3">{children}</div>}
    </section>
  );
}
