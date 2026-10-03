"use client";

import { useEffect } from "react";
import { X, MapPin, FileText, ClipboardList, Star } from "lucide-react";
import { NonTenderEkontrakItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { DetailEntry } from "@/components/inaproc/DetailEntry";

interface NonTenderEkontrakDetailModalProps {
  item: NonTenderEkontrakItem;
  onClose: () => void;
}

export function NonTenderEkontrakDetailModal({ item, onClose }: NonTenderEkontrakDetailModalProps) {
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  const bapBast = item.bapbast_history_json ?? [];
  const spmkSpp = item.spmkspp_history_json ?? [];
  const penilaian = item.penilaian_kinerja_penyedia ?? [];

  // `!mt-0`: modal dirender di dalam wadah `space-y-*`, yang memberi margin-top pada
  // setiap anaknya. Untuk elemen `fixed inset-0`, margin itu menggeser overlay ke bawah
  // sehingga bagian paling atas layar tidak tertutup (muncul "celah").
  return (
    <div className="fixed inset-0 z-50 !mt-0 flex animate-fade-in items-end justify-center bg-slate-950/50 p-0 backdrop-blur-sm sm:items-center sm:p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Detail Non Tender E-Kontrak"
        className="max-h-[90dvh] w-full max-w-2xl overflow-y-auto animate-scale-in rounded-t-3xl bg-white shadow-xl sm:rounded-2xl"
      >
        <div className="sticky top-0 flex items-start justify-between gap-3 border-b border-slate-100 bg-white/90 px-4 py-4 backdrop-blur sm:px-6">
          <div className="min-w-0">
            <h2 className="text-base font-semibold text-slate-900">Detail Non Tender E-Kontrak</h2>
            <p className="mt-0.5 text-xs text-slate-500">
              Kode Tender {item.kd_tender} · TA {item.tahun_anggaran}
            </p>
          </div>
          <button
            onClick={onClose}
            aria-label="Tutup"
            className="shrink-0 rounded-md p-1.5 text-slate-400 hover:bg-slate-100"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="space-y-6 p-4 sm:p-6">
          <div className="space-y-3 rounded-lg border border-slate-100 bg-slate-50 p-4">
            <p className="text-sm font-semibold text-slate-900">{item.nama_paket || "-"}</p>
            <p className="flex items-start gap-2 text-xs text-slate-500">
              <MapPin className="mt-0.5 h-3.5 w-3.5 shrink-0 text-slate-400" />
              {item.alamat_satker || "-"}
            </p>
          </div>

          <Section icon={FileText} title="Riwayat BAP / BAST" count={bapBast.length}>
            {bapBast.map((h, idx) => (
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

          <Section icon={ClipboardList} title="Riwayat SPMK / SPP" count={spmkSpp.length}>
            {spmkSpp.map((h, idx) => (
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

          <Section icon={Star} title="Penilaian Kinerja Penyedia" count={penilaian.length}>
            {penilaian.map((p, idx) => (
              <div
                key={idx}
                className="flex items-center justify-between gap-3 rounded-lg border border-slate-200 px-3 py-2.5 text-sm"
              >
                <span className="min-w-0 text-slate-700">{p.indikator_penilaian || "-"}</span>
                <span className="shrink-0 rounded-full bg-blue-50 px-2.5 py-0.5 text-xs font-semibold text-blue-700">
                  {p.nilai_indikator ?? "-"}
                </span>
              </div>
            ))}
          </Section>
        </div>
      </div>
    </div>
  );
}

function Section({
  icon: Icon,
  title,
  count,
  children,
}: {
  icon: React.ElementType;
  title: string;
  count: number;
  children: React.ReactNode;
}) {
  return (
    <section>
      <h3 className="mb-2 flex items-center gap-1.5 text-sm font-semibold text-slate-700">
        <Icon className="h-4 w-4" />
        {title}
        <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-500">{count}</span>
      </h3>
      {count === 0 ? (
        <p className="rounded-lg bg-slate-50 px-3 py-4 text-center text-xs text-slate-500">Belum ada data.</p>
      ) : (
        <div className="space-y-3">{children}</div>
      )}
    </section>
  );
}
