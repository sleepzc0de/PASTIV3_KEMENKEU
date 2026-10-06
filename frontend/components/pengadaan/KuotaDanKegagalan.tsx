"use client";

import { Gauge, PauseCircle, RotateCw, ShieldCheck, TriangleAlert } from "lucide-react";
import type { PenarikanKuota, PenarikanOtomatis, TugasBermasalah } from "@/lib/api";
import { NadaKuota, daftarAman, formatAngka, kalimatKebijakan, kalimatKuota, keadaanBermasalah, labelParameter, nadaKuota, persenPakai, waktuRelatif } from "@/lib/pengadaan";

const WARNA_BAR: Record<NadaKuota, string> = { normal: "#3358e0", waspada: "#fab219", melambat: "#fab219", habis: "#d03b3b", ditahan: "#d03b3b" };

const LABEL_NADA: Record<NadaKuota, { teks: string; kelas: string; Ikon: typeof ShieldCheck }> = {
  normal: { teks: "Normal", kelas: "bg-emerald-50 text-emerald-700", Ikon: ShieldCheck },
  waspada: { teks: "Mendekati batas", kelas: "bg-amber-50 text-amber-700", Ikon: TriangleAlert },
  melambat: { teks: "Melambat (batas per menit)", kelas: "bg-amber-50 text-amber-700", Ikon: TriangleAlert },
  habis: { teks: "Jatah habis, menunggu", kelas: "bg-red-50 text-red-700", Ikon: PauseCircle },
  ditahan: { teks: "Ditahan (429)", kelas: "bg-red-50 text-red-700", Ikon: PauseCircle },
};

function BilahPakai({ label, terpakai, batas, warna }: { label: string; terpakai: number; batas: number; warna: string }) {
  const persen = persenPakai(terpakai, batas);
  return (
    <div>
      <div className="flex items-baseline justify-between gap-2 text-xs">
        <span className="text-slate-600">{label}</span>
        <span className="font-medium text-slate-900 tabular-nums">
          {formatAngka(terpakai)} <span className="font-normal text-slate-400">dari {formatAngka(batas)}</span>
        </span>
      </div>
      <div className="mt-1 h-2 overflow-hidden rounded-full bg-slate-100" role="progressbar" aria-valuemin={0} aria-valuemax={batas} aria-valuenow={terpakai} aria-label={label}>
        <div className="h-full rounded-full transition-[width] duration-500" style={{ width: `${Math.max(persen, terpakai > 0 ? 1.5 : 0)}%`, backgroundColor: warna }} />
      </div>
    </div>
  );
}

// Pemakaian kuota permintaan ke Inaproc (jendela geser 60 detik dan 60 menit). Satu pembatas dipakai semua penarikan, jadi angka ini mencakup
// penarikan manual, otomatis, dan tampilan langsung.
export function KartuKuota({ kuota }: { kuota: PenarikanKuota }) {
  const nada = nadaKuota(kuota);
  const { teks, kelas, Ikon } = LABEL_NADA[nada];
  const warna = WARNA_BAR[nada];
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm sm:p-5" aria-label="Kuota permintaan Inaproc">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="flex items-center gap-2 text-sm font-semibold text-slate-900">
          <Gauge className="h-4 w-4 text-slate-500" aria-hidden="true" />
          Kuota permintaan Inaproc
        </h3>
        <span className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${kelas}`}>
          <Ikon className="h-3.5 w-3.5" aria-hidden="true" />
          {teks}
        </span>
      </div>
      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <BilahPakai label="Satu jam terakhir" terpakai={kuota.terpakai_jam} batas={kuota.batas_per_jam} warna={warna} />
        <BilahPakai label="60 detik terakhir" terpakai={kuota.terpakai_menit} batas={kuota.batas_per_menit} warna={warna} />
      </div>
      <p className="mt-2.5 text-xs text-slate-500">{kalimatKuota(kuota)}</p>
      <p className="mt-1 text-[11px] text-slate-400">
        Inaproc membatasi 1.000 permintaan per 60 detik dan 5.000 per jam (kuota direset tiap jam). Batas di sini sedikit di bawahnya supaya penarikan tidak pernah menabrak batas itu.
      </p>
    </section>
  );
}

// Tugas yang gagal dan belum pulih: percobaan ulang terjadwal atau istirahat. Kosong = tidak ditampilkan.
export function PanelBermasalah({ item: daftarMentah, otomatis, isAdmin }: { item: TugasBermasalah[] | null | undefined; otomatis: PenarikanOtomatis; isAdmin: boolean }) {
  const item = daftarAman(daftarMentah);
  if (item.length === 0) return null;
  return (
    <section className="rounded-2xl border border-amber-200 bg-amber-50/50 p-4 sm:p-5" aria-label="Tugas yang gagal ditarik">
      <h3 className="flex items-center gap-2 text-sm font-semibold text-slate-900">
        <RotateCw className="h-4 w-4 text-amber-600" aria-hidden="true" />
        Gagal ditarik, menunggu percobaan ulang ({item.length})
      </h3>
      <p className="mt-1 text-xs text-slate-600">{kalimatKebijakan(otomatis.maks_percobaan, otomatis.istirahat_jam)}</p>
      <ul className="mt-3 max-h-72 divide-y divide-amber-100 overflow-y-auto rounded-xl bg-white/80">
        {item.map((b) => {
          const k = keadaanBermasalah(b);
          return (
            <li key={`${b.dataset}|${b.parameter}`} className="px-3 py-2.5 text-xs">
              <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
                <span className="font-medium text-slate-900">{b.nama}</span>
                <span className="text-slate-500">{labelParameter(b.parameter)}</span>
                <span
                  className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${
                    k.nada === "istirahat" ? "bg-slate-100 text-slate-700" : k.nada === "ulang" ? "bg-blue-50 text-blue-700" : "bg-amber-50 text-amber-800"
                  }`}
                >
                  {k.nada === "istirahat" ? "Istirahat" : k.nada === "ulang" ? "Akan dicoba ulang" : "Manual"}
                </span>
                <span className="text-slate-400">gagal {waktuRelatif(b.terakhir_gagal)}</span>
              </div>
              <p className="mt-0.5 text-slate-600">{k.label}</p>
              {isAdmin && b.pesan && <p className="mt-0.5 break-words text-slate-400">{b.pesan}</p>}
            </li>
          );
        })}
      </ul>
      {otomatis.ditahan_sampai && (
        <p className="mt-2 text-xs text-amber-800">Penarikan otomatis sedang ditahan sementara karena banyak tugas gagal berturut-turut; dilanjutkan setelah jeda.</p>
      )}
    </section>
  );
}
