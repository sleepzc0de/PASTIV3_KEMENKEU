"use client";

import { useState } from "react";
import axios from "axios";
import { BadgeCheck } from "lucide-react";
import { type FormKlienBaru, type InfoDataset, type KunciBaru, type Klien, type Pengaturan, postBuatKlien } from "@/lib/apiAkses";
import { BATAS_FORM, type NilaiFormAkses, susunBodyAkses, validasiKlienBaru } from "@/lib/apiAksesFormat";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { ModalShell } from "@/components/ui/ModalShell";
import { useToast } from "@/components/ui/Toast";
import { FormAkses } from "./FormAkses";

const KOSONG: FormKlienBaru = { nama_aplikasi: "", instansi: "", unit: "", pic_nama: "", pic_jabatan: "", pic_email: "", pic_telepon: "", tujuan: "" };
const AKSES_KOSONG: NilaiFormAkses = { dataset: [], ip: "", ue1: "", limit: "", jendela: "", kuota: "", periode: "", catatan: "" };

const inputCls =
  "w-full rounded-xl border bg-white px-3.5 py-2.5 text-sm text-slate-900 shadow-sm outline-none transition-all placeholder:text-slate-400 hover:border-slate-400 focus:border-blue-500 focus:shadow-glow";

function Medan({ id, label, wajib, galat, bantuan, children }: { id: string; label: string; wajib?: boolean; galat?: string; bantuan?: string; children: React.ReactNode }) {
  return (
    <div>
      <label htmlFor={id} className="mb-1.5 block text-sm font-medium text-slate-700">
        {label}{" "}
        {wajib && (
          <span className="text-red-500" aria-hidden="true">
            *
          </span>
        )}
      </label>
      {children}
      {galat ? <p className="mt-1.5 animate-fade-in text-xs font-medium text-red-600">{galat}</p> : bantuan ? <p className="mt-1.5 text-xs text-slate-500">{bantuan}</p> : null}
    </div>
  );
}

// Formulir "Daftarkan aplikasi": SUPERADMIN mendaftarkan aplikasi klien dari data yang diterima dari pemohon (tidak ada pendaftaran mandiri). Klien langsung disetujui dengan scope
// dan batas yang dipilih di sini, dan kunci API pertama diterbitkan sekaligus (ditampilkan sekali oleh pemanggil lewat onSukses).
export function KlienBaruModal({
  dataset,
  bawaan,
  onTutup,
  onSukses,
}: {
  dataset: InfoDataset[];
  bawaan: Pengaturan | null;
  onTutup: () => void;
  onSukses: (klien: Klien, kunci?: KunciBaru) => void;
}) {
  const toast = useToast();
  const [f, setF] = useState<FormKlienBaru>(KOSONG);
  const [akses, setAkses] = useState<NilaiFormAkses>(AKSES_KOSONG);
  const [labelKunci, setLabelKunci] = useState("");
  const [galat, setGalat] = useState<Record<string, string>>({});
  const [galatUmum, setGalatUmum] = useState("");
  const [sibuk, setSibuk] = useState(false);

  const set = (p: Partial<FormKlienBaru>) => setF((x) => ({ ...x, ...p }));
  const garis = (k: string) => (galat[k] ? "border-red-400" : "border-slate-300");

  const kirim = async () => {
    setGalatUmum("");
    const g1 = validasiKlienBaru(f);
    const { body, galat: g2 } = susunBodyAkses(akses);
    const gabungan = { ...g1, ...g2 };
    setGalat(gabungan);
    if (Object.keys(gabungan).length > 0 || !body) {
      setGalatUmum("Periksa kembali isian yang ditandai merah.");
      return;
    }
    setSibuk(true);
    try {
      const rapi = (s: string) => s.trim().replace(/\s+/g, " ");
      const res = await postBuatKlien({
        nama_aplikasi: rapi(f.nama_aplikasi),
        instansi: rapi(f.instansi),
        unit: rapi(f.unit),
        pic_nama: rapi(f.pic_nama),
        pic_jabatan: rapi(f.pic_jabatan),
        pic_email: f.pic_email.trim(),
        pic_telepon: f.pic_telepon.trim(),
        tujuan: f.tujuan.trim(),
        ...body,
        label_kunci: labelKunci.trim(),
      });
      if (!res.data.kunci_baru) toast.warning(res.message);
      onSukses(res.data.klien, res.data.kunci_baru);
    } catch (err) {
      if (axios.isAxiosError(err) && err.response) {
        const d = err.response.data ?? {};
        if (d.errors && typeof d.errors === "object") setGalat(d.errors as Record<string, string>);
        setGalatUmum(d.message || "Klien gagal didaftarkan. Coba lagi.");
      } else {
        setGalatUmum("Tidak dapat terhubung ke server.");
      }
    } finally {
      setSibuk(false);
    }
  };

  return (
    <ModalShell title="Daftarkan aplikasi" subtitle="Didaftarkan oleh superadmin; langsung disetujui dan kunci API pertama diterbitkan" onClose={sibuk ? () => undefined : onTutup} size="xl">
      <form
        noValidate
        onSubmit={(e) => {
          e.preventDefault();
          void kirim();
        }}
        className="space-y-6 p-4 sm:p-6"
      >
        <p className="rounded-xl border border-blue-100 bg-blue-50/60 px-4 py-3 text-sm text-slate-700">
          Tidak ada pendaftaran mandiri: pihak luar menghubungi administrator PASTI, lalu Anda mengisi data berikut dari permintaan mereka. Pilih hanya dataset yang benar-benar dibutuhkan (hak akses
          minimum). Kunci API ditampilkan <b>sekali</b> setelah klien dibuat.
        </p>
        {galatUmum && <Alert message={galatUmum} />}

        <fieldset className="space-y-4">
          <legend className="mb-1 text-sm font-semibold text-slate-900">Aplikasi dan instansi</legend>
          <Medan id="kb-nama" label="Nama aplikasi" wajib galat={galat.nama_aplikasi}>
            <input id="kb-nama" value={f.nama_aplikasi} maxLength={BATAS_FORM.nama} onChange={(e) => set({ nama_aplikasi: e.target.value })} aria-invalid={galat.nama_aplikasi ? true : undefined} className={`${inputCls} ${garis("nama_aplikasi")}`} />
          </Medan>
          <div className="grid gap-4 sm:grid-cols-2">
            <Medan id="kb-instansi" label="Instansi pemilik" wajib galat={galat.instansi}>
              <input id="kb-instansi" value={f.instansi} maxLength={BATAS_FORM.instansi} onChange={(e) => set({ instansi: e.target.value })} className={`${inputCls} ${garis("instansi")}`} />
            </Medan>
            <Medan id="kb-unit" label="Unit" galat={galat.unit}>
              <input id="kb-unit" value={f.unit} maxLength={BATAS_FORM.unit} onChange={(e) => set({ unit: e.target.value })} className={`${inputCls} ${garis("unit")}`} />
            </Medan>
          </div>
        </fieldset>

        <fieldset className="space-y-4">
          <legend className="mb-1 text-sm font-semibold text-slate-900">Penanggung jawab (PIC)</legend>
          <div className="grid gap-4 sm:grid-cols-2">
            <Medan id="kb-pic" label="Nama PIC" wajib galat={galat.pic_nama}>
              <input id="kb-pic" value={f.pic_nama} maxLength={BATAS_FORM.pic} onChange={(e) => set({ pic_nama: e.target.value })} className={`${inputCls} ${garis("pic_nama")}`} />
            </Medan>
            <Medan id="kb-jabatan" label="Jabatan" galat={galat.pic_jabatan}>
              <input id="kb-jabatan" value={f.pic_jabatan} maxLength={BATAS_FORM.pic} onChange={(e) => set({ pic_jabatan: e.target.value })} className={`${inputCls} ${garis("pic_jabatan")}`} />
            </Medan>
            <Medan id="kb-email" label="Email" wajib galat={galat.pic_email} bantuan="Kontak untuk menyerahkan kunci dan menghubungi pemilik aplikasi.">
              <input id="kb-email" type="email" value={f.pic_email} maxLength={BATAS_FORM.email} onChange={(e) => set({ pic_email: e.target.value })} className={`${inputCls} ${garis("pic_email")}`} />
            </Medan>
            <Medan id="kb-telepon" label="Telepon" galat={galat.pic_telepon}>
              <input id="kb-telepon" type="tel" value={f.pic_telepon} maxLength={BATAS_FORM.telepon} onChange={(e) => set({ pic_telepon: e.target.value })} className={`${inputCls} ${garis("pic_telepon")}`} />
            </Medan>
          </div>
        </fieldset>

        <fieldset className="space-y-4">
          <legend className="mb-1 text-sm font-semibold text-slate-900">Tujuan penggunaan</legend>
          <Medan id="kb-tujuan" label="Tujuan penggunaan" wajib galat={galat.tujuan} bantuan={`Data apa yang dibutuhkan dan untuk apa (${f.tujuan.trim().length.toLocaleString("id-ID")}/${BATAS_FORM.tujuanMaks.toLocaleString("id-ID")} karakter).`}>
            <textarea id="kb-tujuan" rows={3} value={f.tujuan} maxLength={BATAS_FORM.tujuanMaks} onChange={(e) => set({ tujuan: e.target.value })} className={`${inputCls} ${garis("tujuan")}`} />
          </Medan>
        </fieldset>

        <fieldset className="space-y-4">
          <legend className="mb-1 text-sm font-semibold text-slate-900">Akses dan batas</legend>
          <FormAkses nilai={akses} onUbah={(p) => setAkses((a) => ({ ...a, ...p }))} dataset={dataset} bawaan={bawaan} galat={galat} />
          <Medan id="kb-label" label="Label kunci pertama (opsional)" bantuan="mis. produksi atau uji; tampil di daftar kunci klien.">
            <input id="kb-label" value={labelKunci} maxLength={100} onChange={(e) => setLabelKunci(e.target.value)} className={`${inputCls} border-slate-300`} />
          </Medan>
        </fieldset>

        <div className="flex flex-wrap justify-end gap-2 border-t border-slate-100 pt-4">
          <Button type="button" fullWidth={false} variant="secondary" onClick={onTutup} disabled={sibuk}>
            Batal
          </Button>
          <Button type="submit" fullWidth={false} isLoading={sibuk} icon={<BadgeCheck className="h-4 w-4" aria-hidden="true" />}>
            Daftarkan dan terbitkan kunci
          </Button>
        </div>
      </form>
    </ModalShell>
  );
}
