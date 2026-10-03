"use client";

import { useEffect, useState } from "react";
import type { ElementType, ReactNode } from "react";
import axios from "axios";
import {
  Package,
  Banknote,
  CalendarDays,
  BadgeCheck,
  MapPin,
  Landmark,
  Scale,
  Car,
  Cpu,
  History,
  Building2,
  Users2,
  ShieldCheck,
  Camera,
  Mountain,
  ExternalLink,
  TriangleAlert,
  RotateCcw,
} from "lucide-react";
import { getSLDKAssetDetail, SLDKAssetDetailData, SLDKReferences } from "@/lib/api";
import { formatCurrency, formatDate } from "@/lib/format";
import { Alert } from "@/components/ui/Alert";
import { ModalShell } from "@/components/ui/ModalShell";
import { CopyButton } from "@/components/hris2/CopyButton";
import { DetailEntry, DetailField } from "@/components/inaproc/DetailEntry";
import { KondisiBadge, FlagBadge } from "@/components/sldk/AssetBadges";
import { AssetSummary, Row, str, num, yesNo, formatNumber, koordinat, namaDari } from "@/components/sldk/asset";

// ---------- pemformat nilai: mengembalikan false bila kosong supaya field disembunyikan ----------

const text = (v: unknown): string => (typeof v === "number" ? String(v) : typeof v === "boolean" ? yesNo(v) : str(v));
const money = (v: unknown) => {
  const n = num(v);
  return n !== null && formatCurrency(n);
};
const area = (v: unknown) => {
  const n = num(v);
  return n !== null && `${formatNumber(n)} m²`;
};
const count = (v: unknown) => {
  const n = num(v);
  return n !== null && formatNumber(n, 0);
};
const decimal = (v: unknown) => {
  const n = num(v);
  return n !== null && formatNumber(n);
};
const date = (v: unknown) => {
  const s = str(v);
  return s !== "" && formatDate(s);
};

function Group({ icon: Icon, title, fields, children }: { icon: ElementType; title: string; fields: DetailField[]; children?: ReactNode }) {
  if (!fields.some(([, value]) => value) && !children) return null;
  return (
    <section>
      <h3 className="mb-2 flex items-center gap-1.5 text-sm font-semibold text-slate-700">
        <Icon className="h-4 w-4" />
        {title}
      </h3>
      <DetailEntry fields={fields} />
      {children}
    </section>
  );
}

// ---------- rincian per jenis (dari tabel anak yang kecil) ----------

type Fmt = "text" | "date" | "num" | "area" | "money" | "yn";
type Col = [column: string, label: string, fmt?: Fmt];
interface SectionDef {
  key: string;
  title: string;
  icon: ElementType;
  cols: Col[];
}

const SECTIONS: SectionDef[] = [
  {
    key: "kendaraan", title: "Kendaraan", icon: Car,
    cols: [["pabrik", "Pabrik"], ["thn_rakit", "Tahun Rakit"], ["thn_buat", "Tahun Buat"], ["negara", "Negara"], ["no_mesin", "No. Mesin"], ["no_rangka", "No. Rangka"], ["no_polisi", "No. Polisi"], ["bhn_bakar", "Bahan Bakar"], ["daya", "Daya"], ["msn_gerak", "Mesin Penggerak"], ["jml_msn", "Jumlah Mesin", "num"], ["muat", "Muatan"], ["bobot", "Bobot"]],
  },
  {
    key: "tik", title: "Perangkat TIK", icon: Cpu,
    cols: [["jns_processor", "Jenis Processor"], ["processor", "Processor", "num"], ["ram", "RAM", "num"], ["hdd", "HDD", "num"], ["monitor", "Monitor", "num"], ["spek_lain", "Spesifikasi Lain"]],
  },
  {
    key: "riwayat_nopol", title: "Riwayat Nomor Polisi", icon: History,
    cols: [["no_polisi", "No. Polisi"], ["jenis_plat_nopol", "Jenis Plat"], ["tgl_keluar", "Tgl. Keluar", "date"], ["tgl_sd_berlaku", "Berlaku s.d.", "date"], ["ket", "Keterangan"], ["terakhir_yn", "Terakhir", "yn"]],
  },
  {
    key: "konstruksi_bangunan", title: "Konstruksi Bangunan", icon: Building2,
    cols: [["tgl_inv", "Tgl. Inventarisasi", "date"], ["str_atap", "Struktur Atap"], ["str_rangka", "Struktur Rangka"], ["material_atap", "Material Atap"], ["material_langit", "Material Langit-langit"], ["matrial_dinding", "Material Dinding"], ["lantai", "Lantai"], ["pelapis_dindin_dlm", "Pelapis Dinding Dalam"], ["pelapis_dindin_lr", "Pelapis Dinding Luar"], ["perkerasan", "Perkerasan"], ["pagar", "Pagar"], ["kd_kondisi", "Kode Kondisi"]],
  },
  {
    key: "tanah_bangunan", title: "Keterkaitan Tanah dan Bangunan", icon: Landmark,
    cols: [["id_aset_tanah", "ID Aset Tanah"], ["id_aset_bangunan", "ID Aset Bangunan"], ["nm_pemilik_bangunan", "Pemilik Bangunan"], ["ur_jenis_kepemilikan", "Jenis Kepemilikan"], ["jml_lantai", "Jumlah Lantai", "num"], ["luas_bangunan", "Luas Bangunan", "area"], ["luas_dasar_bangunan", "Luas Dasar Bangunan", "area"], ["keterangan", "Keterangan"]],
  },
  {
    key: "objek_tanah", title: "Profil Tanah", icon: Mountain,
    cols: [["luas", "Luas", "area"], ["ukuran", "Ukuran"], ["lebar", "Lebar", "num"], ["is_rawan_bencana", "Rawan Bencana", "yn"], ["is_permasalahan_hukum", "Ada Permasalahan Hukum", "yn"], ["tahun_pajak", "Tahun Pajak"], ["njop", "NJOP", "money"], ["njop_per_meter", "NJOP per m²", "money"], ["kode_pos", "Kode Pos"], ["nm_jalan_utama", "Jalan Utama"], ["lebar_jalan", "Lebar Jalan", "num"], ["jarak_jalan_utama", "Jarak ke Jalan Utama", "num"], ["nm_cbd_terdekat", "CBD Terdekat"], ["jarak_cbd_terdekat", "Jarak ke CBD", "num"], ["koordinat", "Koordinat"]],
  },
  {
    key: "riwayat_hukum", title: "Riwayat Permasalahan Hukum", icon: Scale,
    cols: [["tgl", "Tanggal", "date"], ["phk_sengketa", "Pihak Bersengketa"], ["ur_masalah", "Masalah"], ["no_reg_perkara", "No. Register Perkara"], ["kd_status_hukum", "Kode Status Hukum"], ["terakhir_yn", "Terakhir", "yn"]],
  },
  {
    key: "foto", title: "Foto", icon: Camera,
    cols: [["nm_photo", "Nama Berkas"], ["ket_photo", "Keterangan"], ["tanggal", "Tanggal", "date"], ["photo_utama_yn", "Foto Utama", "yn"]],
  },
];

function formatBy(fmt: Fmt | undefined, v: unknown): string | false {
  switch (fmt) {
    case "date": return date(v);
    case "num": return decimal(v);
    case "area": return area(v);
    case "money": return money(v);
    case "yn": return yesNo(v) || false;
    default: return text(v) || false;
  }
}

function SectionRows({ def, section }: { def: SectionDef; section: { rows: Row[]; error?: string } }) {
  const Icon = def.icon;
  if (section.rows.length === 0 && !section.error) return null;
  return (
    <section>
      <h3 className="mb-2 flex items-center gap-1.5 text-sm font-semibold text-slate-700">
        <Icon className="h-4 w-4" />
        {def.title}
        {section.rows.length > 1 && <span className="font-normal text-slate-400">({section.rows.length})</span>}
      </h3>
      {section.error && <p className="mb-2 text-xs text-amber-600">{section.error}</p>}
      <div className="space-y-2">
        {section.rows.map((row, i) => (
          <DetailEntry key={i} fields={def.cols.map(([col, label, fmt]): DetailField => [label, formatBy(fmt, row[col])])} />
        ))}
      </div>
      {def.key === "foto" && section.rows.length > 0 && (
        <p className="mt-2 text-xs text-slate-400">Hanya metadata foto yang tersedia; gambarnya disimpan di SIMAN.</p>
      )}
    </section>
  );
}

// ---------- komponen utama ----------

interface AssetDetailModalProps {
  asset: AssetSummary;
  refs: SLDKReferences | null;
  satkerNama: string;
  onClose: () => void;
}

export function AssetDetailModal({ asset, refs, satkerNama, onClose }: AssetDetailModalProps) {
  const [detail, setDetail] = useState<SLDKAssetDetailData | null>(null);
  const [isLoading, setIsLoading] = useState(asset.idAset !== null);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    if (asset.idAset === null) return;
    let cancelled = false;
    setIsLoading(true);
    setError(null);
    getSLDKAssetDetail(asset.idAset)
      .then((res) => {
        if (!cancelled) setDetail(res.data);
      })
      .catch((err) => {
        if (cancelled) return;
        setDetail(null);
        if (axios.isAxiosError(err) && err.response) setError(err.response.data?.message || "Gagal memuat rincian aset");
        else setError("Gagal terhubung ke server");
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [asset.idAset, reloadKey]);

  const r = asset.raw;
  const kondisiNama = namaDari(refs?.kondisi, asset.kdKondisi);
  const statusNama = namaDari(refs?.status_penggunaan, asset.kdStatus);
  const jenisNama = namaDari(refs?.jenis_bmn, asset.kdJnsBmn);
  const hukumNama = namaDari(refs?.status_hukum, str(r.kd_status_hukum));
  const kdSatker = text(r.kd_satker);
  const coord = koordinat(r.gps_latitude, r.gps_longitude);
  const dqInvalid = num(r.dq_tgl_invalid_cnt);

  const identitas: DetailField[] = [
    ["Kode Register", asset.kodeRegister],
    ["No. KIB", text(r.no_kib)],
    ["NUP", text(r.no_aset)],
    ["Kode Barang", text(r.kd_brg)],
    ["Jenis BMN", jenisNama || asset.kdJnsBmn],
    ["Jenis Aset", text(r.jns_aset)],
    ["Merk", text(r.merk)],
    ["Tipe", text(r.tipe)],
    ["Serial Number", text(r.serial_number)],
    ["No. Polisi", text(r.no_polisi)],
    ["Kuantitas", count(r.kuantitas)],
    ["Satuan Kerja", satkerNama ? `${satkerNama}${kdSatker ? ` (${kdSatker})` : ""}` : kdSatker],
    ["Intra/Ekstrakomptabel", text(r.intra_extra)],
    ["Status BMN", yesNo(r.status_bmn_yn)],
    ["Tercatat", text(r.tercatat)],
  ];
  const nilai: DetailField[] = [
    ["Nilai Perolehan", money(r.rph_aset)],
    ["Mutasi", money(r.rph_mutasi)],
    ["Akumulasi Penyusutan", money(r.rph_susut)],
    ["Nilai Buku", money(r.rph_buku)],
    ["Umur Sisa", decimal(r.umur_sisa)],
    ["Cara Perolehan", text(r.cara_perlh)],
    ["Asal Perolehan", text(r.asl_perlh)],
    ["Sumber Dana", text(r.ur_sumber_dana)],
    ["No. Dokumen Perolehan", text(r.no_dok_perolehan)],
  ];
  const tanggal: DetailField[] = [
    ["Tgl. Perolehan", date(r.tgl_perlh)],
    ["Tgl. Buku Pertama", date(r.tgl_buku_pertama)],
    ["Tgl. Digunakan", date(r.tgl_guna)],
    ["Tgl. Renovasi", date(r.tgl_renov)],
    ["Tgl. Rekam", date(r.tgl_rekam)],
    ["Tgl. Rekam Pertama", date(r.tgl_rekam_pertama)],
    ["Tgl. Hapus", date(r.tgl_hapus)],
  ];
  const status: DetailField[] = [
    ["Kondisi", kondisiNama || (asset.kdKondisi && `Kode ${asset.kdKondisi}`)],
    ["Status Penggunaan", statusNama || (asset.kdStatus && `Kode ${asset.kdStatus}`)],
    ["Status Pengelolaan", text(r.status_pengelolaan)],
    ["BMN Idle", yesNo(r.status_bmn_idle)],
    ["Jenis Idle", text(r.kd_jns_idle)],
    ["Barang Hilang", yesNo(r.brg_hilang_yn)],
    ["Barang Rusak", yesNo(r.brg_rusak_yn)],
    ["Dihentikan", yesNo(r.dihentikan_yn)],
    ["Hapus Lainnya", yesNo(r.hapus_lainnya_yn)],
    ["Rencana Hibah", yesNo(r.rencana_hibah_yn)],
    ["Kemitraan", yesNo(r.kemitraan_yn)],
    ["Properti Investasi", yesNo(r.properti_investasi_yn)],
    ["Status SBSN", text(r.status_sbsn)],
    ["KMK SBSN", text(r.kmk_sbsn)],
    ["Tgl. Akhir SBSN", date(r.tgl_akhir_sbsn)],
    ["Status Sanksi", text(r.status_sanksi)],
  ];
  const lokasi: DetailField[] = [
    ["Alamat", text(r.vc_alamat_lengkap) || text(r.alamat)],
    ["Alamat Lain", text(r.alamat_lain)],
    ["Komplek", text(r.komplek)],
    ["RT/RW", text(r.kd_rtrw)],
    ["Kelurahan", text(r.ur_kel)],
    ["Kecamatan", text(r.ur_kec)],
    ["Kabupaten/Kota", text(r.ur_kab)],
    ["Provinsi", text(r.ur_prov)],
    ["Kode Pos", text(r.kd_pos)],
    ["Negara", text(r.negara)],
    ["Lokasi Ruang", text(r.lokasi_ruang)],
    ["Koordinat", coord && `${coord.lat}, ${coord.lng}`],
    ["Batas Utara", text(r.bts_utara)],
    ["Batas Selatan", text(r.bts_selatan)],
    ["Batas Barat", text(r.bts_barat)],
    ["Batas Timur", text(r.bts_timur)],
  ];
  const tanahBangunan: DetailField[] = [
    ["Luas", area(r.luas)],
    ["Luas Tapak", area(r.luas_tapak)],
    ["Luas Pemanfaatan", area(r.luas_pemanfaatan)],
    ["Jumlah Lantai", count(r.jml_lantai)],
    ["Jumlah Bangunan", count(r.jml_bdg)],
    ["Jumlah Bidang", count(r.jml_bidang)],
    ["Bentuk", text(r.bentuk)],
    ["Peruntukan", text(r.peruntukan_tnh) || text(r.peruntukan)],
    ["Topografi (Kontur)", text(r.topografi_kontur)],
    ["Topografi (Elevasi)", text(r.topografi_elevasi)],
    ["Aksesibilitas", text(r.aksesibilitas)],
    ["Panjang", decimal(r.panjang)],
    ["Lebar", decimal(r.lebar)],
    ["Optimalisasi", decimal(r.optimalisasi)],
    ["Kapasitas", decimal(r.kapasitas)],
    ["SBSK", decimal(r.sbsk)],
  ];
  const hukum: DetailField[] = [
    ["Status Hukum", hukumNama || text(r.kd_status_hukum)],
    ["No. Perkara Hukum", text(r.no_perkara_hukum)],
    ["Jenis Dokumen Kepemilikan", text(r.jns_dok_bukti_kepemilikan)],
    ["No. Dokumen Kepemilikan", text(r.no_dok_bukti_kepemilikan)],
    ["Status Dokumen", text(r.stat_dok_bukti_kepemilikan)],
    ["Tgl. Dokumen", text(r.tgl_dok_bukti_kepemilikan)],
    ["Jenis Sertifikat", text(r.jns_sertifikat)],
    ["No. PSP", text(r.no_psp)],
    ["Tgl. PSP", date(r.tgl_psp)],
  ];
  const pengguna: DetailField[] = [
    ["Jenis Pengguna", text(r.jns_pengguna)],
    ["Kode Unit Pengguna", text(r.kd_unit_pengguna)],
    ["Unit Pengguna", text(r.nm_unit_pengguna)],
    ["Keterangan", text(r.ket_pengguna)],
  ];
  const kualitas: DetailField[] = [
    ["Status Data", text(r.status_data)],
    ["Riwayat (sts_his)", text(r.sts_his)],
    ["Status Aset (sts_ast)", text(r.sts_ast)],
    ["Tanggal Tidak Valid", dqInvalid !== null && dqInvalid > 0 && formatNumber(dqInvalid, 0)],
    ["Data per", date(r._ingestion_date)],
    ["Diperbarui", date(r.updated_at)],
  ];

  const hasDetail = detail && SECTIONS.some((s) => (detail.sections[s.key]?.rows.length ?? 0) > 0 || detail.sections[s.key]?.error);

  return (
    <ModalShell title="Detail Aset" subtitle={asset.kodeRegister ? `Kode register ${asset.kodeRegister}` : undefined} onClose={onClose}>
      <div className="space-y-6 p-4 sm:p-6">
        <div className="space-y-2 rounded-lg border border-slate-100 bg-slate-50 p-4">
          <p className="break-words text-base font-semibold text-slate-900">{asset.nama || "Tanpa nama"}</p>
          {asset.merkTipe && <p className="text-sm text-slate-500">{asset.merkTipe}</p>}
          <div className="flex flex-wrap items-center gap-1.5">
            <KondisiBadge kode={asset.kdKondisi} nama={kondisiNama} />
            {jenisNama && <span className="rounded-full bg-blue-50 px-2.5 py-0.5 text-xs font-medium text-blue-700">{jenisNama}</span>}
            {asset.flags.map((f) => (
              <FlagBadge key={f} label={f} />
            ))}
          </div>
          {asset.kodeRegister && (
            <div className="flex flex-wrap items-center gap-x-2">
              <span className="break-all font-mono text-xs text-slate-600">{asset.kodeRegister}</span>
              <CopyButton text={asset.kodeRegister} label="Salin kode register" />
            </div>
          )}
        </div>

        <Group icon={Package} title="Identitas" fields={identitas} />
        <Group icon={Banknote} title="Nilai" fields={nilai} />
        <Group icon={CalendarDays} title="Tanggal" fields={tanggal} />
        <Group icon={BadgeCheck} title="Status dan Kondisi" fields={status} />
        <Group icon={MapPin} title="Lokasi" fields={lokasi}>
          {coord && (
            <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
              {coord.status !== "tidak valid" && (
                <a
                  href={`https://www.google.com/maps?q=${coord.lat},${coord.lng}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 font-medium text-blue-600 hover:underline"
                >
                  <ExternalLink className="h-3.5 w-3.5" />
                  Buka di peta
                </a>
              )}
              {coord.status !== "ok" && (
                <span className="inline-flex items-center gap-1 text-amber-600">
                  <TriangleAlert className="h-3.5 w-3.5" />
                  {coord.status === "luar" ? "Koordinat di luar wilayah Indonesia, periksa datanya" : "Koordinat tidak valid"}
                </span>
              )}
            </div>
          )}
        </Group>
        <Group icon={Landmark} title="Tanah dan Bangunan" fields={tanahBangunan} />
        <Group icon={Scale} title="Dokumen dan Hukum" fields={hukum} />
        <Group icon={Users2} title="Pengguna" fields={pengguna} />
        {text(r.catatan) && (
          <section>
            <h3 className="mb-2 text-sm font-semibold text-slate-700">Catatan</h3>
            <p className="whitespace-pre-line break-words rounded-lg border border-slate-200 p-3 text-sm text-slate-700">{text(r.catatan)}</p>
          </section>
        )}

        <div className="space-y-6 border-t border-slate-100 pt-6">
          <p className="text-xs font-medium uppercase tracking-wide text-slate-400">Rincian per jenis aset</p>
          {isLoading && (
            <div role="status" aria-label="Memuat rincian aset" className="animate-pulse space-y-3">
              <div className="h-4 w-32 rounded bg-slate-200" />
              <div className="h-16 rounded-lg bg-slate-100" />
            </div>
          )}
          {error && (
            <div className="space-y-3">
              <Alert message={error} />
              <button
                type="button"
                onClick={() => setReloadKey((k) => k + 1)}
                className="flex items-center gap-1.5 rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
              >
                <RotateCcw className="h-3.5 w-3.5" />
                Coba lagi
              </button>
            </div>
          )}
          {asset.idAset === null && <p className="text-sm text-slate-500">Aset ini tidak punya ID, rincian per jenis tidak tersedia.</p>}
          {detail && SECTIONS.map((def) => detail.sections[def.key] && <SectionRows key={def.key} def={def} section={detail.sections[def.key]} />)}
          {detail && !hasDetail && <p className="text-sm text-slate-500">Tidak ada rincian tambahan yang tercatat untuk aset ini.</p>}
        </div>

        <Group icon={ShieldCheck} title="Kualitas dan Sumber Data" fields={kualitas} />
      </div>
    </ModalShell>
  );
}
