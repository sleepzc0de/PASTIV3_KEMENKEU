"use client";

import { FormEvent, useEffect, useState } from "react";
import axios from "axios";
import { CheckCircle2, Send } from "lucide-react";
import { type FormPendaftaran, type InfoDataset, getInfoPendaftaran, kirimPendaftaran } from "@/lib/apiAkses";
import { BATAS_FORM, validasiPendaftaran } from "@/lib/apiAksesFormat";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { CaptchaField } from "@/components/auth/CaptchaField";

const KOSONG: FormPendaftaran = { nama_aplikasi: "", instansi: "", unit: "", pic_nama: "", pic_jabatan: "", pic_email: "", pic_telepon: "", tujuan: "", dataset_diminta: [] };

const inputCls =
  "w-full rounded-xl border bg-white px-3.5 py-2.5 text-sm text-slate-900 shadow-sm outline-none transition-all placeholder:text-slate-400 hover:border-slate-400 focus:border-blue-500 focus:shadow-glow";

function Medan({ id, label, wajib, galat, bantuan, children }: { id: string; label: string; wajib?: boolean; galat?: string; bantuan?: string; children: React.ReactNode }) {
  return (
    <div>
      <label htmlFor={id} className="mb-1.5 block text-sm font-medium text-slate-700">
        {label} {wajib && <span className="text-red-500" aria-hidden="true">*</span>}
      </label>
      {children}
      {galat ? <p className="mt-1.5 animate-fade-in text-xs font-medium text-red-600">{galat}</p> : bantuan ? <p className="mt-1.5 text-xs text-slate-500">{bantuan}</p> : null}
    </div>
  );
}

// Formulir pendaftaran aplikasi klien API Akses Data (publik, tanpa login). Pendaftaran tidak langsung memberi kredensial: superadmin PASTI menilai lebih dulu, dan
// kunci API diserahkan kepada PIC lewat jalur yang aman setelah disetujui.
export function PendaftaranForm() {
  const [dataset, setDataset] = useState<InfoDataset[]>([]);
  const [dibuka, setDibuka] = useState(true);
  const [muat, setMuat] = useState(true);
  const [galatMuat, setGalatMuat] = useState("");
  const [f, setF] = useState<FormPendaftaran>(KOSONG);
  const [galat, setGalat] = useState<Record<string, string>>({});
  const [galatUmum, setGalatUmum] = useState("");
  const [captchaId, setCaptchaId] = useState("");
  const [jawaban, setJawaban] = useState("");
  const [galatCaptcha, setGalatCaptcha] = useState("");
  const [putaranCaptcha, setPutaranCaptcha] = useState(0);
  const [mengirim, setMengirim] = useState(false);
  const [kode, setKode] = useState("");

  useEffect(() => {
    getInfoPendaftaran()
      .then((r) => {
        setDataset(r.dataset);
        setDibuka(r.pendaftaran_dibuka);
      })
      .catch(() => setGalatMuat("Daftar dataset tidak dapat dimuat. Muat ulang halaman ini."))
      .finally(() => setMuat(false));
  }, []);

  const set = (p: Partial<FormPendaftaran>) => setF((x) => ({ ...x, ...p }));
  const alih = (k: string) => set({ dataset_diminta: f.dataset_diminta.includes(k) ? f.dataset_diminta.filter((d) => d !== k) : [...f.dataset_diminta, k] });

  const kirim = async (e: FormEvent) => {
    e.preventDefault();
    setGalatUmum("");
    setGalatCaptcha("");
    const g = validasiPendaftaran(f);
    setGalat(g);
    if (!jawaban.trim()) setGalatCaptcha("Masukkan kode keamanan");
    if (Object.keys(g).length > 0 || !jawaban.trim()) return;
    if (!captchaId) {
      setGalatUmum("Kode keamanan belum siap. Tunggu sebentar lalu coba lagi.");
      return;
    }
    setMengirim(true);
    try {
      const r = await kirimPendaftaran(f, captchaId, jawaban.trim());
      setKode(r.data.kode);
    } catch (err) {
      if (axios.isAxiosError(err) && err.response) {
        const d = err.response.data ?? {};
        if (d.errors && typeof d.errors === "object") setGalat(d.errors as Record<string, string>);
        if (d.code === "captcha_salah") setGalatCaptcha(d.message);
        else setGalatUmum(d.message || "Pendaftaran gagal dikirim. Coba lagi.");
      } else {
        setGalatUmum("Tidak dapat terhubung ke server.");
      }
      setPutaranCaptcha((n) => n + 1); // kode keamanan hanya berlaku sekali: muat yang baru
    } finally {
      setMengirim(false);
    }
  };

  if (kode) {
    return (
      <div className="rounded-2xl border border-emerald-200 bg-emerald-50/60 p-6 text-center sm:p-8" role="status">
        <span className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-emerald-100 text-emerald-700">
          <CheckCircle2 className="h-6 w-6" aria-hidden="true" />
        </span>
        <h2 className="mt-4 text-lg font-bold text-slate-900">Pendaftaran diterima</h2>
        <p className="mx-auto mt-1.5 max-w-md text-sm text-slate-600">
          Pendaftaran Anda menunggu penilaian administrator PASTI. Bila disetujui, kunci API diserahkan kepada PIC lewat kontak yang Anda isikan. Belum ada kredensial yang diterbitkan.
        </p>
        <p className="mt-4 text-xs font-medium uppercase tracking-wide text-slate-500">Nomor pendaftaran</p>
        <p className="mt-1 select-all font-mono text-xl font-bold tracking-wider text-slate-900">{kode}</p>
        <p className="mt-1 text-xs text-slate-500">Simpan nomor ini untuk menanyakan status pendaftaran.</p>
      </div>
    );
  }

  return (
    <form onSubmit={kirim} noValidate className="space-y-6">
      {galatMuat && <Alert message={galatMuat} />}
      {!muat && !dibuka && <Alert tone="warning" message="Pendaftaran aplikasi baru sedang ditutup. Hubungi administrator PASTI." />}
      {galatUmum && <Alert message={galatUmum} />}

      <fieldset className="space-y-4" disabled={!dibuka}>
        <legend className="mb-1 text-sm font-semibold text-slate-900">Aplikasi dan instansi</legend>
        <Medan id="pd-nama" label="Nama aplikasi" wajib galat={galat.nama_aplikasi}>
          <input id="pd-nama" value={f.nama_aplikasi} maxLength={BATAS_FORM.nama} onChange={(e) => set({ nama_aplikasi: e.target.value })} aria-invalid={galat.nama_aplikasi ? true : undefined} className={`${inputCls} ${galat.nama_aplikasi ? "border-red-400" : "border-slate-300"}`} />
        </Medan>
        <div className="grid gap-4 sm:grid-cols-2">
          <Medan id="pd-instansi" label="Instansi pemilik" wajib galat={galat.instansi}>
            <input id="pd-instansi" value={f.instansi} maxLength={BATAS_FORM.instansi} onChange={(e) => set({ instansi: e.target.value })} className={`${inputCls} ${galat.instansi ? "border-red-400" : "border-slate-300"}`} />
          </Medan>
          <Medan id="pd-unit" label="Unit" galat={galat.unit}>
            <input id="pd-unit" value={f.unit} maxLength={BATAS_FORM.unit} onChange={(e) => set({ unit: e.target.value })} className={`${inputCls} ${galat.unit ? "border-red-400" : "border-slate-300"}`} />
          </Medan>
        </div>
      </fieldset>

      <fieldset className="space-y-4" disabled={!dibuka}>
        <legend className="mb-1 text-sm font-semibold text-slate-900">Penanggung jawab (PIC)</legend>
        <div className="grid gap-4 sm:grid-cols-2">
          <Medan id="pd-pic" label="Nama PIC" wajib galat={galat.pic_nama}>
            <input id="pd-pic" value={f.pic_nama} maxLength={BATAS_FORM.pic} onChange={(e) => set({ pic_nama: e.target.value })} autoComplete="name" className={`${inputCls} ${galat.pic_nama ? "border-red-400" : "border-slate-300"}`} />
          </Medan>
          <Medan id="pd-jabatan" label="Jabatan" galat={galat.pic_jabatan}>
            <input id="pd-jabatan" value={f.pic_jabatan} maxLength={BATAS_FORM.pic} onChange={(e) => set({ pic_jabatan: e.target.value })} className={`${inputCls} ${galat.pic_jabatan ? "border-red-400" : "border-slate-300"}`} />
          </Medan>
          <Medan id="pd-email" label="Email" wajib galat={galat.pic_email} bantuan="Kunci API dan kabar keputusan dikirim ke kontak ini.">
            <input id="pd-email" type="email" value={f.pic_email} maxLength={BATAS_FORM.email} onChange={(e) => set({ pic_email: e.target.value })} autoComplete="email" className={`${inputCls} ${galat.pic_email ? "border-red-400" : "border-slate-300"}`} />
          </Medan>
          <Medan id="pd-telepon" label="Telepon" galat={galat.pic_telepon}>
            <input id="pd-telepon" type="tel" value={f.pic_telepon} maxLength={BATAS_FORM.telepon} onChange={(e) => set({ pic_telepon: e.target.value })} autoComplete="tel" className={`${inputCls} ${galat.pic_telepon ? "border-red-400" : "border-slate-300"}`} />
          </Medan>
        </div>
      </fieldset>

      <fieldset className="space-y-4" disabled={!dibuka}>
        <legend className="mb-1 text-sm font-semibold text-slate-900">Penggunaan</legend>
        <Medan id="pd-tujuan" label="Tujuan penggunaan" wajib galat={galat.tujuan} bantuan={`Jelaskan data apa yang dibutuhkan dan untuk apa (${f.tujuan.trim().length.toLocaleString("id-ID")}/${BATAS_FORM.tujuanMaks.toLocaleString("id-ID")} karakter).`}>
          <textarea id="pd-tujuan" rows={4} value={f.tujuan} maxLength={BATAS_FORM.tujuanMaks} onChange={(e) => set({ tujuan: e.target.value })} className={`${inputCls} ${galat.tujuan ? "border-red-400" : "border-slate-300"}`} />
        </Medan>
        <div>
          <p className="mb-1.5 text-sm font-medium text-slate-700">
            Dataset yang diminta <span className="text-red-500" aria-hidden="true">*</span>
          </p>
          {muat ? (
            <p className="text-sm text-slate-400">Memuat daftar dataset…</p>
          ) : (
            <div className="grid gap-2 sm:grid-cols-2" role="group" aria-label="Dataset yang diminta">
              {dataset.map((d) => (
                <label key={d.kode} className="flex cursor-pointer items-start gap-2.5 rounded-xl border border-slate-200 bg-white p-3 text-sm hover:border-slate-300 has-[:checked]:border-blue-300 has-[:checked]:bg-blue-50/50">
                  <input type="checkbox" checked={f.dataset_diminta.includes(d.kode)} onChange={() => alih(d.kode)} className="mt-0.5 h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" />
                  <span className="min-w-0">
                    <span className="block font-medium text-slate-800">{d.label}</span>
                    <span className="block text-xs text-slate-500">{d.deskripsi}</span>
                  </span>
                </label>
              ))}
            </div>
          )}
          {galat.dataset_diminta ? (
            <p className="mt-1.5 text-xs font-medium text-red-600">{galat.dataset_diminta}</p>
          ) : (
            <p className="mt-1.5 text-xs text-slate-500">Administrator hanya memberi akses ke dataset yang benar-benar diperlukan. Kolom data pribadi tidak pernah diberikan.</p>
          )}
        </div>
      </fieldset>

      <CaptchaField key={putaranCaptcha} onCaptchaIdChange={setCaptchaId} value={jawaban} onChange={setJawaban} error={galatCaptcha} />

      <Button type="submit" isLoading={mengirim} disabled={!dibuka} icon={<Send className="h-4 w-4" aria-hidden="true" />}>
        Kirim pendaftaran
      </Button>
    </form>
  );
}
