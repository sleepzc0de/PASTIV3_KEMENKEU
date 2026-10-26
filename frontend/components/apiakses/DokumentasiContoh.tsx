"use client";

import { useMemo, useState } from "react";
import type { Dokumentasi } from "@/lib/apiAkses";
import { type Bahasa, type Skenario, BAHASA, SKENARIO, contohKode } from "@/lib/apiAksesDokumentasi";
import { JUDUL_KELOMPOK_DATA, kelompokkanDataset } from "@/lib/apiAksesFormat";
import { Tabs } from "@/components/ui/Tabs";
import { Kode, KotakKode } from "./KotakKode";

const selectCls =
  "w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition-all hover:border-slate-400 focus:border-blue-500 focus:shadow-glow";

const CATATAN: Record<Bahasa, string> = {
  curl: "Untuk shell bash (Linux, macOS, Git Bash). Di Windows PowerShell pakai curl.exe dan $env:PASTI_API_KEY, dengan perintah ditulis dalam satu baris.",
  python: "Memerlukan paket requests (pip install requests).",
  node: "Node.js 18 ke atas (fetch bawaan). Simpan sebagai berkas .mjs agar await tingkat atas berlaku.",
};

// Contoh kode siap salin untuk dataset pilihan. Alamat, penyaring, dan ukuran halaman mengikuti dataset dan pengaturan yang berlaku sekarang. Kunci selalu dibaca dari variabel
// lingkungan PASTI_API_KEY, tidak pernah ditulis di kode atau alamat.
export function DokumentasiContoh({ dok, akar, kodeDataset, onKodeDataset }: { dok: Dokumentasi; akar: string; kodeDataset: string; onKodeDataset: (kode: string) => void }) {
  const [bahasa, setBahasa] = useState<Bahasa>("curl");
  const [skenario, setSkenario] = useState<Skenario>("satu");
  const kelompok = useMemo(() => kelompokkanDataset(dok.dataset), [dok.dataset]);
  const ds = dok.dataset.find((d) => d.kode === kodeDataset) ?? dok.dataset[0];
  if (!ds) return null;

  const kode = contohKode(bahasa, skenario, {
    akar,
    ds,
    tahun: new Date().getFullYear(),
    halamanBawaan: dok.bawaan.halaman_bawaan,
    halamanMaks: dok.bawaan.halaman_maks,
  });
  const uraianSkenario = SKENARIO.find((s) => s.key === skenario)?.uraian;

  return (
    <div className="space-y-3">
      <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
        <div>
          <label htmlFor="dok-contoh-dataset" className="mb-1 block text-xs font-medium text-slate-700">
            Dataset contoh
          </label>
          <select id="dok-contoh-dataset" value={ds.kode} onChange={(e) => onKodeDataset(e.target.value)} className={selectCls}>
            {kelompok.map((k) => (
              <optgroup key={k.kelompok} label={JUDUL_KELOMPOK_DATA[k.kelompok] ?? k.kelompok}>
                {k.bagian.flatMap((b) =>
                  b.dataset.map((d) => (
                    <option key={d.kode} value={d.kode}>
                      {b.judul ? `${b.judul}: ` : ""}
                      {d.label} ({d.kode})
                    </option>
                  )),
                )}
              </optgroup>
            ))}
          </select>
        </div>
        <Tabs tabs={SKENARIO} value={skenario} onChange={setSkenario} label="Jenis contoh" idPrefix="dok-skenario" />
      </div>

      <Tabs tabs={BAHASA} value={bahasa} onChange={setBahasa} label="Bahasa pemrograman" idPrefix="dok-bahasa" />
      <div role="tabpanel" id={`dok-bahasa-panel-${bahasa}`} aria-labelledby={`dok-bahasa-tab-${bahasa}`} className="space-y-2">
        <KotakKode kode={kode} judul={`${BAHASA.find((b) => b.key === bahasa)?.label} · ${ds.alamat.replace(dok.alamat_dasar, "")}`} labelSalin="Salin contoh kode" />
        <p className="text-xs text-slate-500">
          {uraianSkenario} {CATATAN[bahasa]} Contoh membaca kunci dari variabel lingkungan <Kode>PASTI_API_KEY</Kode>; jangan menaruh kunci di kode sumber atau alamat.
        </p>
      </div>
    </div>
  );
}
