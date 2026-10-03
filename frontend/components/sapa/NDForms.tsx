"use client";

import { useState } from "react";
import { SapaDataNDSatker, SapaDataNDUE1, SapaSaya, SapaTahapDetail, SapaUsulan } from "@/lib/sapa";
import { BarangEditor } from "./BarangEditor";
import { ChecklistEditor } from "./ChecklistEditor";
import { CheckField, NoticeBox, Section, TextAreaField, TextField } from "./fields";
import { muatanNDSatker, normNDSatker, normNDUE1 } from "./sapa";
import { FormFooter, useTahapAksi } from "./useTahapAksi";

interface Props {
  usulan: SapaUsulan;
  tahap: SapaTahapDetail;
  saya: SapaSaya;
  onChanged: () => void;
}

// Nota Dinas Usulan Penjualan Satker: beserta daftar barang, checklist dokumen, dan surat-surat pernyataan.
export function NDSatkerForm({ usulan, tahap, saya, onChanged }: Props) {
  const [v, setV] = useState<SapaDataNDSatker>(() => normNDSatker(tahap.data ?? tahap.saran, saya.item_dokumen));
  const aksi = useTahapAksi(usulan.id, tahap.kunci, onChanged);
  const set = <K extends keyof SapaDataNDSatker>(k: K, val: SapaDataNDSatker[K]) => setV((p) => ({ ...p, [k]: val }));
  const setTtd = (k: "nama" | "nip" | "jabatan", val: string) => setV((p) => ({ ...p, penandatangan: { ...p.penandatangan, [k]: val } }));
  const muatan = () => muatanNDSatker(v, saya.item_dokumen);

  return (
    <div className="space-y-6">
      <NoticeBox>
        Nomor registrasi (Noreg) usulan ini: <span className="font-semibold">{usulan.noreg}</span>. Nama satker <span className="font-semibold">{usulan.nama_satker}</span>{" "}
        dan kode satker diambil dari usulan.
      </NoticeBox>

      <Section title="Data surat">
        <div className="space-y-3">
          <CheckField
            label="BMN yang diusulkan sudah ditetapkan dalam RP4 (Rencana Pemindahtanganan BMN)"
            checked={v.sudah_rp4}
            onChange={(x) => set("sudah_rp4", x)}
            hint="Nota Dinas hanya dapat dibuat untuk BMN yang sudah masuk RP4."
          />
          <div className="grid gap-3 sm:grid-cols-2">
            <TextField
              label="Tujuan surat (Sekretaris UE1)"
              required
              value={v.tujuan_surat}
              onChange={(x) => set("tujuan_surat", x)}
              maxLength={300}
              className="sm:col-span-2"
              hint="Terisi otomatis dari referensi kode satker; ubah bila tidak sesuai."
            />
            <TextField label="Kota / kabupaten lokasi satker" required value={v.kota} onChange={(x) => set("kota", x)} maxLength={100} />
            <TextField
              label="Singkatan satker"
              value={v.singkatan_satker}
              onChange={(x) => set("singkatan_satker", x)}
              maxLength={100}
              hint="Opsional, mis. KPKNL Jakarta I"
            />
            <TextField label="Jenis BMN" required value={v.jenis_bmn} onChange={(x) => set("jenis_bmn", x)} maxLength={200} placeholder="mis. Tanah dan Bangunan" />
            <TextField
              label="Satuan jumlah BMN"
              value={v.satuan}
              onChange={(x) => set("satuan", x)}
              maxLength={30}
              placeholder="mis. unit, bidang"
              hint="Opsional; dipakai pada terbilang jumlah BMN"
            />
            <TextField label="Nomor tiket SIMAN" required value={v.tiket_siman} onChange={(x) => set("tiket_siman", x)} maxLength={100} />
            <TextField
              label="Tembusan: Kepala Kantor Wilayah"
              required
              value={v.kepala_kanwil}
              onChange={(x) => set("kepala_kanwil", x)}
              maxLength={300}
              placeholder="mis. Kepala Kantor Wilayah DJKN Jakarta"
            />
            <TextAreaField
              label="Alasan / pertimbangan penjualan"
              required
              value={v.alasan}
              onChange={(x) => set("alasan", x)}
              maxLength={2000}
              rows={4}
              className="sm:col-span-2"
            />
          </div>
        </div>
      </Section>

      <Section title="Pejabat penandatangan" description="Pejabat satker yang menandatangani Nota Dinas dan surat pernyataan.">
        <div className="grid gap-3 sm:grid-cols-3">
          <TextField label="Nama" required value={v.penandatangan.nama} onChange={(x) => setTtd("nama", x)} maxLength={150} />
          <TextField label="NIP (18 digit)" required value={v.penandatangan.nip} onChange={(x) => setTtd("nip", x.replace(/[^\d\s]/g, ""))} inputMode="numeric" maxLength={24} />
          <TextField label="Jabatan" required value={v.penandatangan.jabatan} onChange={(x) => setTtd("jabatan", x)} maxLength={300} />
        </div>
      </Section>

      <Section title="Daftar barang" description="Barang yang diusulkan untuk dijual. Jumlah dan total nilai pada dokumen dihitung dari daftar ini.">
        <BarangEditor barang={v.barang} onChange={(b) => set("barang", b)} />
      </Section>

      <Section
        title="Checklist dokumen pendukung"
        description="Centang dokumen yang tersedia dan isi nomor serta tanggalnya. Hasilnya tercetak pada tabel checklist di Nota Dinas."
      >
        <ChecklistEditor items={saya.item_dokumen} value={v.dokumen} onChange={(d) => set("dokumen", d)} />
      </Section>

      <FormFooter
        aksi={aksi}
        templateTersedia={tahap.template_tersedia?.[tahap.jenis_dokumen ?? ""] !== false}
        onSimpan={() => aksi.simpan(muatan())}
        onBuat={() => aksi.buat(muatan())}
        labelBuat="Buat Nota Dinas (Word)"
      />
    </div>
  );
}

// Nota Dinas Usulan Penjualan UE1: disusun dari data usulan Satker dan Nota Dinas Satker yang sudah ditetapkan di Nadine.
export function NDUE1Form({ usulan, tahap, onChanged }: Props) {
  const [v, setV] = useState<SapaDataNDUE1>(() => normNDUE1(tahap.data ?? tahap.saran));
  const aksi = useTahapAksi(usulan.id, tahap.kunci, onChanged);
  const set = <K extends keyof SapaDataNDUE1>(k: K, val: SapaDataNDUE1[K]) => setV((p) => ({ ...p, [k]: val }));
  const setTtd = (k: "nama" | "jabatan", val: string) => setV((p) => ({ ...p, penandatangan: { ...p.penandatangan, [k]: val } }));

  return (
    <div className="space-y-6">
      <NoticeBox>
        Noreg aplikasi: <span className="font-semibold">{usulan.noreg}</span>. Daftar barang, nilai, dan data lain diambil dari Nota Dinas usulan Satker yang sudah dibuat.
      </NoticeBox>

      <Section title="Nota Dinas usulan Satker yang dirujuk" description="Nomor dan tanggal terisi dari catatan penetapan di Nadine; periksa dan ubah bila perlu.">
        <div className="grid gap-3 sm:grid-cols-2">
          <TextField label="Nomor Nota Dinas" required value={v.nomor_nd} onChange={(x) => set("nomor_nd", x)} maxLength={150} />
          <TextField label="Tanggal Nota Dinas" required type="date" value={v.tanggal_nd} onChange={(x) => set("tanggal_nd", x)} />
          <TextAreaField label="Hal" required value={v.hal_nd} onChange={(x) => set("hal_nd", x)} maxLength={500} rows={2} className="sm:col-span-2" />
        </div>
      </Section>

      <Section title="Tujuan, tembusan, dan penandatangan">
        <div className="grid gap-3 sm:grid-cols-2">
          <TextField label="Sekretaris UE1" required value={v.sekretaris_ue1} onChange={(x) => set("sekretaris_ue1", x)} maxLength={300} />
          <TextField label="Tembusan: Kepala Kantor Wilayah" required value={v.kepala_kanwil} onChange={(x) => set("kepala_kanwil", x)} maxLength={300} />
          <TextField
            label="Tembusan: pejabat pengelola"
            required
            value={v.pejabat_pengelola}
            onChange={(x) => set("pejabat_pengelola", x)}
            maxLength={300}
            className="sm:col-span-2"
            placeholder="mis. Direktur Barang Milik Negara"
          />
          <TextField label="Nama pejabat UE1 penandatangan" required value={v.penandatangan.nama} onChange={(x) => setTtd("nama", x)} maxLength={150} />
          <TextField label="Jabatan penandatangan" required value={v.penandatangan.jabatan} onChange={(x) => setTtd("jabatan", x)} maxLength={300} />
        </div>
      </Section>

      <FormFooter
        aksi={aksi}
        templateTersedia={tahap.template_tersedia?.[tahap.jenis_dokumen ?? ""] !== false}
        onSimpan={() => aksi.simpan(v)}
        onBuat={() => aksi.buat(v)}
        labelBuat="Buat Nota Dinas (Word)"
      />
    </div>
  );
}
