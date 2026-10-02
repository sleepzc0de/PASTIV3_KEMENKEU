"use client";

import { useEffect } from "react";
import { X, MapPin, FileText, Banknote, Building2, UserRound, Info } from "lucide-react";
import { NonTenderEkontrakKontrakItem } from "@/lib/api";
import { formatDate, formatCurrency } from "@/lib/format";
import { DetailEntry, DetailField } from "@/components/inaproc/DetailEntry";

interface NonTenderEkontrakKontrakDetailModalProps {
  item: NonTenderEkontrakKontrakItem;
  onClose: () => void;
}

export function NonTenderEkontrakKontrakDetailModal({ item, onClose }: NonTenderEkontrakKontrakDetailModalProps) {
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  const addendum =
    item.apakah_addendum && item.versi_addendum != null && item.versi_addendum > 0
      ? `${item.apakah_addendum} (versi ${item.versi_addendum})`
      : item.apakah_addendum;

  const kontrak: DetailField[] = [
    ["No. Kontrak", item.no_kontrak],
    ["No. SPPBJ", item.no_sppbj],
    ["Jenis Kontrak", item.jenis_kontrak],
    ["Status Kontrak", item.status_kontrak],
    ["Tgl. Penetapan Status", item.tgl_penetapan_status_kontrak && formatDate(item.tgl_penetapan_status_kontrak)],
    ["Alasan Penetapan Status", item.alasan_penetapan_status_kontrak],
    ["Tgl. Kontrak", item.tgl_kontrak && formatDate(item.tgl_kontrak)],
    ["Tgl. Mulai Kontrak", item.tgl_kontrak_awal && formatDate(item.tgl_kontrak_awal)],
    ["Tgl. Akhir Kontrak", item.tgl_kontrak_akhir && formatDate(item.tgl_kontrak_akhir)],
    ["Kota Kontrak", item.kota_kontrak],
    ["Metode Pengadaan", item.mtd_pengadaan],
    ["Addendum", addendum],
    ["Alasan Addendum", item.alasan_addendum],
  ];

  const nilai: DetailField[] = [
    ["Nilai Kontrak", item.nilai_kontrak != null && formatCurrency(item.nilai_kontrak)],
    ["Nilai PDN", item.nilai_pdn_kontrak != null && formatCurrency(item.nilai_pdn_kontrak)],
    ["Nilai UMK", item.nilai_umk_kontrak != null && formatCurrency(item.nilai_umk_kontrak)],
    ["Alasan Ubah Nilai Kontrak", item.alasan_ubah_nilai_kontrak],
    ["Alasan Nilai Kontrak 10 Persen", item.alasan_nilai_kontrak_10_persen],
  ];

  const penyedia: DetailField[] = [
    ["Nama Penyedia", item.nama_penyedia],
    ["Bentuk Usaha", item.bentuk_usaha_penyedia],
    ["Tipe Penyedia", item.tipe_penyedia],
    ["NPWP", item.npwp_penyedia],
    ["NPWP 16 Digit", item.npwp16_penyedia],
    ["Wakil Sah Penyedia", item.wakil_sah_penyedia],
    ["Jabatan Wakil Penyedia", item.jabatan_wakil_penyedia],
    ["Anggota KSO", item.anggota_kso],
    ["Bank", item.nama_rek_bank],
    ["No. Rekening", item.no_rek_bank],
    ["Pemilik Rekening", item.nama_pemilik_rek_bank],
  ];

  const ppk: DetailField[] = [
    ["Nama PPK", item.nama_ppk],
    ["NIP PPK", item.nip_ppk],
    ["Jabatan PPK", item.jabatan_ppk],
    ["No. SK PPK", item.no_sk_ppk],
  ];

  const lainnya: DetailField[] = [
    ["Lingkup Pekerjaan", item.lingkup_pekerjaan],
    ["Informasi Lainnya", item.informasi_lainnya],
    ["Kode Satker", item.kd_satker_str || item.kd_satker],
    ["Jenis KLPD", item.jenis_klpd],
    ["Kode Non Tender", String(item.kd_nontender)],
    ["Kode LPSE", item.kd_lpse != null && String(item.kd_lpse)],
  ];

  return (
    // `!mt-0`: modal ini dirender di dalam wadah `space-y-*` yang memberi margin-top pada
    // setiap anaknya; tanpa ini overlay `fixed inset-0` bergeser dan menyisakan celah di atas.
    <div className="fixed inset-0 z-50 !mt-0 flex items-center justify-center bg-black/50 p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Detail Kontrak Non Tender E-Kontrak"
        className="max-h-[90dvh] w-full max-w-2xl overflow-y-auto rounded-xl bg-white shadow-xl"
      >
        <div className="sticky top-0 flex items-start justify-between gap-3 border-b border-slate-200 bg-white px-4 py-4 sm:px-6">
          <div className="min-w-0">
            <h2 className="text-base font-semibold text-slate-900">Detail Kontrak Non Tender</h2>
            <p className="mt-0.5 break-words text-xs text-slate-500">
              No. Kontrak {item.no_kontrak || "-"} · TA {item.tahun_anggaran}
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
            {(item.nama_satker || item.nama_klpd) && (
              <p className="text-xs text-slate-600">{[item.nama_satker, item.nama_klpd].filter(Boolean).join(" · ")}</p>
            )}
            {item.alamat_satker && (
              <p className="flex items-start gap-2 text-xs text-slate-500">
                <MapPin className="mt-0.5 h-3.5 w-3.5 shrink-0 text-slate-400" />
                {item.alamat_satker}
              </p>
            )}
          </div>

          <Group icon={FileText} title="Kontrak" fields={kontrak} />
          <Group icon={Banknote} title="Nilai Kontrak" fields={nilai} />
          <Group icon={Building2} title="Penyedia" fields={penyedia} />
          <Group icon={UserRound} title="Pejabat Pembuat Komitmen (PPK)" fields={ppk} />
          <Group icon={Info} title="Informasi Lainnya" fields={lainnya} />
        </div>
      </div>
    </div>
  );
}

// Satu kelompok field; seluruh kelompok disembunyikan kalau semua nilainya kosong.
function Group({ icon: Icon, title, fields }: { icon: React.ElementType; title: string; fields: DetailField[] }) {
  if (!fields.some(([, value]) => value)) return null;
  return (
    <section>
      <h3 className="mb-2 flex items-center gap-1.5 text-sm font-semibold text-slate-700">
        <Icon className="h-4 w-4" />
        {title}
      </h3>
      <DetailEntry fields={fields} />
    </section>
  );
}
