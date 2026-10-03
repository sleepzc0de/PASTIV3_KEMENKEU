"use client";

import { useRef, useState } from "react";
import { BadgeCheck, Plus, Trash2 } from "lucide-react";
import { SapaAnggota, SapaDataBA, SapaDataTim, SapaSaya, SapaTahapDetail, SapaUsulan } from "@/lib/sapa";
import { Section, SecondaryButton, SelectInput, TextField } from "./fields";
import { PegawaiPicker, PegawaiTerpilih } from "./PegawaiPicker";
import { kosongAnggota, muatanTim, nipGanda, nipTerpakai, normBA, normTim } from "./sapa";
import { FormFooter, useTahapAksi } from "./useTahapAksi";

interface Props {
  usulan: SapaUsulan;
  tahap: SapaTahapDetail;
  saya: SapaSaya;
  onChanged: () => void;
}

// Pembentukan Tim: SK Tim Pemindahtanganan.
export function TimForm({ usulan, tahap, saya, onChanged }: Props) {
  const [v, setV] = useState<SapaDataTim>(() => normTim(tahap.data ?? tahap.saran));
  const aksi = useTahapAksi(usulan.id, tahap.kunci, onChanged);
  const set = <K extends keyof SapaDataTim>(k: K, val: SapaDataTim[K]) => setV((p) => ({ ...p, [k]: val }));
  const setAnggota = (i: number, k: keyof SapaAnggota, val: string) =>
    setV((p) => ({ ...p, anggota: p.anggota.map((a, j) => (j === i ? { ...a, [k]: val } : a)) }));

  // Kunci tetap per baris anggota: pencarian HRIS2 di tiap baris menyimpan keadaannya sendiri, jadi baris yang dihapus di tengah
  // tidak boleh mewariskan keadaannya ke baris sesudahnya.
  const nextKey = useRef(v.anggota.length);
  const [kunci, setKunci] = useState<number[]>(() => v.anggota.map((_, i) => i));
  const [fokus, setFokus] = useState<number | null>(null);
  const tambah = () => {
    const k = nextKey.current++;
    setV((p) => ({ ...p, anggota: [...p.anggota, kosongAnggota()] }));
    setKunci((ks) => [...ks, k]);
    setFokus(k); // kotak pencarian baris baru langsung aktif
  };
  const hapus = (i: number) => {
    setV((p) => ({ ...p, anggota: p.anggota.filter((_, j) => j !== i) }));
    setKunci((ks) => ks.filter((_, j) => j !== i));
  };
  // Hasil pencarian HRIS2 mengisi nama (dengan gelar), jabatan, dan NIP; kedudukan tetap diisi pengguna. Semua bidang masih bisa diubah.
  const isiDariHRIS = (i: number, p: PegawaiTerpilih) =>
    setV((cur) => ({ ...cur, anggota: cur.anggota.map((a, j) => (j === i ? { ...a, nama: p.nama, jabatan: p.jabatan || a.jabatan, nip: p.nip } : a)) }));
  const ganda = nipGanda(v.anggota);
  const terpakai = nipTerpakai(v.anggota);

  return (
    <div className="space-y-5">
      <Section title="Data Surat Keputusan">
        <div className="grid gap-3 sm:grid-cols-2">
          <TextField label="Jabatan pimpinan" required value={v.jabatan_pimpinan} onChange={(x) => set("jabatan_pimpinan", x)} placeholder="mis. Kepala Kantor" maxLength={200} />
          <SelectInput label="Jenis tim" required value={v.jenis_tim} onChange={(x) => set("jenis_tim", x)} options={saya.jenis_tim} />
          <TextField label="Masa tugas: mulai" required type="date" value={v.masa_awal} onChange={(x) => set("masa_awal", x)} />
          <TextField label="Masa tugas: sampai" required type="date" value={v.masa_akhir} onChange={(x) => set("masa_akhir", x)} />
          <TextField label="Kota" required value={v.kota} onChange={(x) => set("kota", x)} maxLength={100} hint="Kota penetapan SK, mis. Jakarta" />
        </div>
      </Section>

      <Section
        title="Anggota tim"
        description="Cari pegawai di HRIS2 agar nama dan jabatannya terisi otomatis, atau isi manual. Lengkapi kedudukan dalam tim (mis. Ketua, Sekretaris, Anggota). Baris yang kosong diabaikan."
        action={
          <SecondaryButton onClick={tambah} disabled={v.anggota.length >= 60}>
            <Plus className="h-4 w-4" aria-hidden="true" />
            Tambah anggota
          </SecondaryButton>
        }
      >
        <ol className="space-y-3">
          {v.anggota.map((a, i) => {
            const nipRapi = a.nip.replace(/\s+/g, "");
            return (
              <li key={kunci[i] ?? i} className="rounded-xl border border-slate-200 bg-slate-50/60 p-3 sm:p-4">
                <div className="mb-3 flex items-center justify-between gap-2">
                  <span className="flex items-center gap-2 text-xs font-semibold text-slate-600">
                    Anggota {i + 1}
                    {nipRapi && (
                      <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700">
                        <BadgeCheck className="h-3 w-3" aria-hidden="true" />
                        NIP tercatat
                      </span>
                    )}
                  </span>
                  <button
                    type="button"
                    onClick={() => hapus(i)}
                    aria-label={`Hapus anggota ${i + 1}`}
                    className="rounded p-1 text-slate-400 hover:bg-red-50 hover:text-red-600"
                  >
                    <Trash2 className="h-4 w-4" aria-hidden="true" />
                  </button>
                </div>

                <PegawaiPicker onPilih={(p) => isiDariHRIS(i, p)} terpakai={terpakai} autoFocus={kunci[i] === fokus} />

                <div className="mt-3 grid gap-3 sm:grid-cols-2">
                  <TextField label="Nama" value={a.nama} onChange={(x) => setAnggota(i, "nama", x)} maxLength={150} />
                  <TextField label="Jabatan" value={a.jabatan} onChange={(x) => setAnggota(i, "jabatan", x)} maxLength={200} />
                  <TextField label="Kedudukan dalam tim" value={a.kedudukan} onChange={(x) => setAnggota(i, "kedudukan", x)} maxLength={100} list="sapa-kedudukan-tim" placeholder="mis. Ketua" />
                  <TextField label="NIP (opsional)" value={a.nip} onChange={(x) => setAnggota(i, "nip", x.replace(/[^\d\s]/g, ""))} inputMode="numeric" maxLength={24} hint="Terisi otomatis dari HRIS2; boleh diketik manual." />
                </div>
                {nipRapi && ganda.has(nipRapi) && <p className="mt-2 text-xs text-amber-700">NIP ini dipakai lebih dari satu anggota. Periksa kembali daftar anggota.</p>}
              </li>
            );
          })}
        </ol>
        <datalist id="sapa-kedudukan-tim">
          <option value="Ketua" />
          <option value="Sekretaris" />
          <option value="Anggota" />
        </datalist>
      </Section>

      <FormFooter
        aksi={aksi}
        templateTersedia={tahap.template_tersedia?.[tahap.jenis_dokumen ?? ""] !== false}
        onSimpan={() => aksi.simpan(muatanTim(v))}
        onBuat={() => aksi.buat(muatanTim(v))}
      />
    </div>
  );
}

// Berita Acara Penelitian.
export function BAForm({ usulan, tahap, saya, onChanged }: Props) {
  const [v, setV] = useState<SapaDataBA>(() => normBA(tahap.data ?? tahap.saran));
  const aksi = useTahapAksi(usulan.id, tahap.kunci, onChanged);
  const set = <K extends keyof SapaDataBA>(k: K, val: SapaDataBA[K]) => setV((p) => ({ ...p, [k]: val }));

  return (
    <div className="space-y-5">
      <Section title="Data Berita Acara Penelitian" description="Hari dan bulan penelitian dibentuk otomatis dari tanggal yang dipilih.">
        <div className="grid gap-3 sm:grid-cols-2">
          <SelectInput label="Bentuk pemindahtanganan" required value={v.bentuk} onChange={(x) => set("bentuk", x)} options={saya.bentuk} />
          <TextField label="Tanggal penelitian" required type="date" value={v.tanggal_penelitian} onChange={(x) => set("tanggal_penelitian", x)} />
          <TextField
            label="Nama tim"
            required
            value={v.nama_tim}
            onChange={(x) => set("nama_tim", x)}
            list="sapa-nama-tim"
            maxLength={200}
            className="sm:col-span-2"
            hint="Pilih dari daftar atau ketik nama tim sesuai SK"
          />
          <datalist id="sapa-nama-tim">
            {saya.jenis_tim.map((j) => (
              <option key={j} value={j} />
            ))}
          </datalist>
        </div>
      </Section>
      <FormFooter
        aksi={aksi}
        templateTersedia={tahap.template_tersedia?.[tahap.jenis_dokumen ?? ""] !== false}
        onSimpan={() => aksi.simpan(v)}
        onBuat={() => aksi.buat(v)}
      />
    </div>
  );
}
