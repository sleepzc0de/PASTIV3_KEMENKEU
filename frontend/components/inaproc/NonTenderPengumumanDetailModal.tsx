"use client";

import { useEffect } from "react";
import { X, MapPin, FileText, Banknote, CalendarDays, Users2, Globe, Building2 } from "lucide-react";
import { NonTenderPengumumanItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { DetailGroup, DetailField } from "@/components/inaproc/DetailEntry";

interface NonTenderPengumumanDetailModalProps {
  item: NonTenderPengumumanItem;
  onClose: () => void;
}

export function NonTenderPengumumanDetailModal({ item, onClose }: NonTenderPengumumanDetailModalProps) {
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  const paket: DetailField[] = [
    ["Kode Non Tender", String(item.kd_nontender)],
    ["Kode RUP", item.kd_rup],
    ["Jenis Pengadaan", item.jenis_pengadaan],
    ["Metode Pemilihan", item.mtd_pemilihan],
    ["Kualifikasi Paket", item.kualifikasi_paket],
    ["Kontrak Pembayaran", item.kontrak_pembayaran],
    ["Sumber Dana", item.sumber_dana],
    ["MAK", item.mak],
    ["Repeat Order", item.repeat_order],
    ["Versi Non Tender", item.versi_nontender != null && String(item.versi_nontender)],
  ];

  const anggaran: DetailField[] = [
    ["Pagu", item.pagu != null && formatCurrency(item.pagu)],
    ["HPS", item.hps != null && formatCurrency(item.hps)],
  ];

  const status: DetailField[] = [
    ["Status", item.status_nontender],
    ["Tgl. Buat Paket", item.tgl_buat_paket && formatDate(item.tgl_buat_paket)],
    ["Tgl. Kolektif Kolegial", item.tgl_kolektif_kolegial && formatDate(item.tgl_kolektif_kolegial)],
    ["Tgl. Pengumuman", item.tgl_pengumuman_nontender && formatDate(item.tgl_pengumuman_nontender)],
    ["Keterangan Ditutup", item.ket_ditutup],
    ["Keterangan Diulang", item.ket_diulang],
  ];

  const pelaksana: DetailField[] = [
    ["PPK", item.nip_nama_ppk],
    ["Pejabat Pengadaan (PP)", item.nip_nama_pp],
    ["Pokja", item.nip_nama_pokja],
  ];

  const lpse: DetailField[] = [
    ["Nama LPSE", item.nama_lpse],
    ["URL LPSE", item.url_lpse],
    ["Kode LPSE", item.kd_lpse != null && String(item.kd_lpse)],
    ["ID Lelang", item.lls_id != null && String(item.lls_id)],
    ["Kode Paket DCE", item.kd_pkt_dce != null && String(item.kd_pkt_dce)],
  ];

  const satker: DetailField[] = [
    ["Satuan Kerja", item.nama_satker],
    ["Kode Satker", item.kd_satker_str || item.kd_satker],
    ["KLPD", item.nama_klpd],
    ["Jenis KLPD", item.jenis_klpd],
  ];

  return (
    // `!mt-0`: modal ini dirender di dalam wadah `space-y-*` yang memberi margin-top pada
    // setiap anaknya; tanpa ini overlay `fixed inset-0` bergeser dan menyisakan celah di atas.
    <div className="fixed inset-0 z-50 !mt-0 flex animate-fade-in items-end justify-center bg-slate-950/50 p-0 backdrop-blur-sm sm:items-center sm:p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Detail Pengumuman Non Tender"
        className="max-h-[90dvh] w-full max-w-2xl overflow-y-auto animate-scale-in rounded-t-3xl bg-white shadow-xl sm:rounded-2xl"
      >
        <div className="sticky top-0 flex items-start justify-between gap-3 border-b border-slate-100 bg-white/90 px-4 py-4 backdrop-blur sm:px-6">
          <div className="min-w-0">
            <h2 className="text-base font-semibold text-slate-900">Detail Pengumuman Non Tender</h2>
            <p className="mt-0.5 text-xs text-slate-500">
              Kode Non Tender {item.kd_nontender} · TA {item.tahun_anggaran}
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
          <div className="space-y-2 rounded-lg border border-slate-100 bg-slate-50 p-4">
            <p className="text-sm font-semibold text-slate-900">{item.nama_paket || "-"}</p>
            {item.nama_satker && (
              <p className="flex items-start gap-2 text-xs text-slate-500">
                <MapPin className="mt-0.5 h-3.5 w-3.5 shrink-0 text-slate-400" />
                {item.nama_satker}
              </p>
            )}
          </div>

          <DetailGroup icon={FileText} title="Paket" fields={paket} />
          <DetailGroup icon={Banknote} title="Anggaran" fields={anggaran} />
          <DetailGroup icon={CalendarDays} title="Status & Jadwal" fields={status} />
          <DetailGroup icon={Users2} title="Pelaksana" fields={pelaksana} />
          <DetailGroup icon={Globe} title="LPSE" fields={lpse} />
          <DetailGroup icon={Building2} title="Satker & KLPD" fields={satker} />
        </div>
      </div>
    </div>
  );
}
