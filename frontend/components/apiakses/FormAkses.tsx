"use client";

import type { InfoDataset, Pengaturan } from "@/lib/apiAkses";
import { type NilaiFormAkses, alihKelompokDataset, jumlahTerpilih, kelompokkanDataset, teksPeriode } from "@/lib/apiAksesFormat";

const inputCls =
  "w-full rounded-xl border bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition-all placeholder:text-slate-400 hover:border-slate-400 focus:border-blue-500 focus:shadow-glow";

function Medan({ id, label, bantuan, galat, children }: { id: string; label: string; bantuan?: string; galat?: string; children: React.ReactNode }) {
  return (
    <div>
      <label htmlFor={id} className="mb-1 block text-xs font-medium text-slate-700">
        {label}
      </label>
      {children}
      {bantuan && !galat && <p className="mt-1 text-xs text-slate-500">{bantuan}</p>}
      {galat && <p className="mt-1 text-xs font-medium text-red-600">{galat}</p>}
    </div>
  );
}

// Formulir scope dan batas satu klien: dataset yang diizinkan, allowlist IP, pembatasan UE1, dan override batas laju/kuota. Dipakai saat menyetujui pendaftaran dan saat
// mengubah klien yang sudah disetujui. Isian batas yang kosong berarti ikut bawaan global (ditampilkan sebagai petunjuk).
export function FormAkses({
  nilai,
  onUbah,
  dataset,
  bawaan,
  galat,
  diminta,
}: {
  nilai: NilaiFormAkses;
  onUbah: (p: Partial<NilaiFormAkses>) => void;
  dataset: InfoDataset[];
  bawaan: Pengaturan | null;
  galat: Record<string, string>;
  diminta?: string[];
}) {
  const alih = (kode: string) => onUbah({ dataset: nilai.dataset.includes(kode) ? nilai.dataset.filter((d) => d !== kode) : [...nilai.dataset, kode] });
  const kelompok = kelompokkanDataset(dataset);
  return (
    <div className="space-y-4">
      <fieldset>
        <legend className="mb-1.5 text-xs font-medium text-slate-700">Dataset yang diizinkan (hak akses minimum)</legend>
        <div className="space-y-4">
          {kelompok.map((g) => (
            <div key={g.kelompok}>
              <div className="mb-1.5 flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
                <p className="text-xs font-semibold text-slate-600">
                  {g.judul} <span className="font-normal text-slate-400">({jumlahTerpilih(nilai.dataset, g.kode)}/{g.kode.length} dipilih)</span>
                </p>
                <div className="flex gap-3 text-xs">
                  <button type="button" onClick={() => onUbah({ dataset: alihKelompokDataset(nilai.dataset, g.kode, true) })} className="font-medium text-blue-600 hover:underline">
                    Pilih semua
                  </button>
                  <button type="button" onClick={() => onUbah({ dataset: alihKelompokDataset(nilai.dataset, g.kode, false) })} className="font-medium text-slate-500 hover:underline">
                    Kosongkan
                  </button>
                </div>
              </div>
              <div className="space-y-2.5">
                {g.bagian.map((b) => (
                  <div key={b.judul || g.kelompok}>
                    {b.judul && <p className="mb-1 text-[11px] font-medium uppercase tracking-wide text-slate-400">{b.judul}</p>}
                    <div className="grid gap-2 sm:grid-cols-2">
                      {b.dataset.map((d) => (
                        <label key={d.kode} className="flex cursor-pointer items-start gap-2.5 rounded-xl border border-slate-200 bg-white p-2.5 text-sm hover:border-slate-300 has-[:checked]:border-blue-300 has-[:checked]:bg-blue-50/50">
                          <input type="checkbox" checked={nilai.dataset.includes(d.kode)} onChange={() => alih(d.kode)} className="mt-0.5 h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" />
                          <span className="min-w-0">
                            <span className="block font-medium text-slate-800">
                              {d.label} <span className="break-all font-mono text-[11px] font-normal text-slate-400">{d.kode}</span>
                              {diminta?.includes(d.kode) && <span className="ml-1.5 rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium text-slate-600">diminta</span>}
                            </span>
                            <span className="block text-xs text-slate-500">{d.deskripsi}</span>
                          </span>
                        </label>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
        {galat.dataset && <p className="mt-1 text-xs font-medium text-red-600">{galat.dataset}</p>}
        {nilai.dataset.some((k) => k.startsWith("pengadaan.")) && (
          <p className="mt-2 rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-600">
            Data pengadaan memuat nama paket, satker, nilai, dan penyedia. Nama dan NIP PPK/Pokja, NPWP, rekening, serta isian bebas petugas tidak pernah diberikan. Bila klien dibatasi ke UE1, hanya pengadaan satker milik UE1 itu
            (menurut data aset) yang terlihat.
          </p>
        )}
      </fieldset>

      <div className="grid gap-4 sm:grid-cols-2">
        <Medan id="fa-ip" label="Allowlist IP (opsional)" bantuan="Alamat IP atau CIDR, dipisah koma atau baris baru. Kosong = semua alamat boleh." galat={galat.ip}>
          <textarea id="fa-ip" rows={2} value={nilai.ip} onChange={(e) => onUbah({ ip: e.target.value })} placeholder="mis. 10.20.30.0/24, 203.0.113.9" className={`${inputCls} font-mono ${galat.ip ? "border-red-400" : "border-slate-300"}`} />
        </Medan>
        <Medan id="fa-ue1" label="Batasi ke Unit Eselon I (opsional)" bantuan="Kode UE1 5 digit, dipisah koma. Kosong = semua UE1." galat={galat.ue1}>
          <textarea id="fa-ue1" rows={2} value={nilai.ue1} onChange={(e) => onUbah({ ue1: e.target.value })} placeholder="mis. 01504, 01505" className={`${inputCls} font-mono ${galat.ue1 ? "border-red-400" : "border-slate-300"}`} />
        </Medan>
      </div>

      <fieldset>
        <legend className="mb-1.5 text-xs font-medium text-slate-700">Batas khusus klien (kosongkan untuk mengikuti bawaan global)</legend>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <Medan id="fa-limit" label="Permintaan per jendela" bantuan={bawaan ? `Bawaan: ${bawaan.limit_permintaan.toLocaleString("id-ID")}` : undefined} galat={galat.limit}>
            <input id="fa-limit" inputMode="numeric" value={nilai.limit} onChange={(e) => onUbah({ limit: e.target.value })} placeholder={bawaan ? String(bawaan.limit_permintaan) : "bawaan"} className={`${inputCls} ${galat.limit ? "border-red-400" : "border-slate-300"}`} />
          </Medan>
          <Medan id="fa-jendela" label="Jendela laju (detik)" bantuan={bawaan ? `Bawaan: ${teksPeriode(bawaan.limit_jendela_detik)}` : undefined} galat={galat.jendela}>
            <input id="fa-jendela" inputMode="numeric" value={nilai.jendela} onChange={(e) => onUbah({ jendela: e.target.value })} placeholder={bawaan ? String(bawaan.limit_jendela_detik) : "bawaan"} className={`${inputCls} ${galat.jendela ? "border-red-400" : "border-slate-300"}`} />
          </Medan>
          <Medan id="fa-kuota" label="Kuota per periode" bantuan={bawaan ? `Bawaan: ${bawaan.kuota_permintaan.toLocaleString("id-ID")}` : undefined} galat={galat.kuota}>
            <input id="fa-kuota" inputMode="numeric" value={nilai.kuota} onChange={(e) => onUbah({ kuota: e.target.value })} placeholder={bawaan ? String(bawaan.kuota_permintaan) : "bawaan"} className={`${inputCls} ${galat.kuota ? "border-red-400" : "border-slate-300"}`} />
          </Medan>
          <Medan id="fa-periode" label="Periode kuota (detik)" bantuan={bawaan ? `Bawaan: ${teksPeriode(bawaan.kuota_periode_detik)}` : undefined} galat={galat.periode}>
            <input id="fa-periode" inputMode="numeric" value={nilai.periode} onChange={(e) => onUbah({ periode: e.target.value })} placeholder={bawaan ? String(bawaan.kuota_periode_detik) : "bawaan"} className={`${inputCls} ${galat.periode ? "border-red-400" : "border-slate-300"}`} />
          </Medan>
        </div>
      </fieldset>

      <Medan id="fa-catatan" label="Catatan (opsional)" galat={galat.catatan}>
        <input id="fa-catatan" value={nilai.catatan} maxLength={1000} onChange={(e) => onUbah({ catatan: e.target.value })} placeholder="mis. nomor surat permintaan" className={`${inputCls} ${galat.catatan ? "border-red-400" : "border-slate-300"}`} />
      </Medan>
    </div>
  );
}
