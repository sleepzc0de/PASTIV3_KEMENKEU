"use client";

import { ReactNode, useEffect, useRef, useState } from "react";
import { AlertTriangle, CheckCircle2, Info, Lightbulb, XCircle } from "lucide-react";
import type { AnBulan, AnPasangan, AnWawasan, TingkatWawasan } from "@/lib/api";
import { BULAN_SINGKAT, NAMA_BULAN, bagi, formatAngka, formatPersen, rupiahRingkas, skalaSumbu, sumbuRingkas } from "@/lib/pengadaan";

// Grafik ringan tanpa pustaka untuk Dasbor Pengadaan. Mengikuti aturan pembuatan grafik yang sama dengan components/sldk/charts.tsx:
// satu sumbu (tanpa sumbu ganda), batang tipis dengan ujung membulat, grid samar, nilai ditulis di teks (bukan di dalam batang), legenda
// selalu ada untuk dua seri atau lebih, dan setiap grafik punya padanan tabel lewat ChartCard. Warna seri: slot 1 biru, slot 2 jingga dari
// palet kategorikal terverifikasi; "belum/lainnya" memakai abu-abu netral; status memakai warna status dengan ikon dan label.

export const WARNA = {
  seri1: "#2a78d6",
  seri2: "#eb6834",
  seri3: "#1baf7a",
  netral: "#cbd5e1",
  merek: "#3358e0",
  baik: "#0ca30c",
  perhatian: "#fab219",
  penting: "#d03b3b",
};

const GRID = "#e2e8f0";
const AXIS = "#cbd5e1";
const MUTED = "#64748b";
const PERMUKAAN = "#ffffff";

function useLebar<T extends HTMLElement>() {
  const ref = useRef<T>(null);
  const [lebar, setLebar] = useState(0);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    setLebar(Math.floor(el.getBoundingClientRect().width));
    if (typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver((e) => setLebar(Math.floor(e[0].contentRect.width)));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  return [ref, lebar] as const;
}

export interface SeriBulan {
  nama: string;
  warna: string;
  data: AnBulan[];
}

type Metrik = "nilai" | "jumlah";

// Kolom per bulan (Januari-Desember). Satu atau dua seri berdampingan; satu sumbu y. Arahkan penunjuk atau sentuh untuk membaca angka bulan itu.
// `metrik` awal bisa diganti pengguna bila `bolehGanti`; "nilai" memakai singkatan rupiah di sumbu.
export function KolomBulanan({ seri, metrik: awal = "nilai", bolehGanti = true, satuanJumlah = "paket" }: { seri: SeriBulan[]; metrik?: Metrik; bolehGanti?: boolean; satuanJumlah?: string }) {
  const [ref, lebar] = useLebar<HTMLDivElement>();
  const [metrik, setMetrik] = useState<Metrik>(awal);
  const [aktif, setAktif] = useState<number | null>(null);

  const nilaiDari = (b: AnBulan | undefined) => (b ? (metrik === "nilai" ? b.nilai : b.jumlah) : 0);
  const semuaNol = seri.every((s) => s.data.every((b) => nilaiDari(b) === 0));

  const isi = (() => {
    if (semuaNol) return <p className="py-10 text-center text-sm text-slate-400">Tidak ada data pada tahun ini</p>;
    if (lebar === 0) return null;
    const tinggi = 220;
    const m = { atas: 14, kanan: 8, bawah: 24, kiri: metrik === "nilai" ? 52 : 40 };
    const plotW = Math.max(lebar - m.kiri - m.kanan, 12 * 8);
    const plotH = tinggi - m.atas - m.bawah;
    const { batas: maks, langkah } = skalaSumbu(Math.max(...seri.flatMap((s) => s.data.map(nilaiDari))), metrik === "jumlah");
    const sy = (v: number) => m.atas + plotH - (Math.max(v, 0) / maks) * plotH;
    const lebarBulan = plotW / 12;
    const lebarBatang = Math.max(Math.min((lebarBulan * 0.7) / seri.length, 22), 3);
    const sumbu = (v: number) => (metrik === "nilai" ? sumbuRingkas(v) : formatAngka(v));
    const ticks = Array.from({ length: Math.round(maks / langkah) + 1 }, (_, i) => i * langkah);

    const dasar = m.atas + plotH;
    return (
      <div className="relative">
        <svg
          width={lebar}
          height={tinggi}
          role="img"
          aria-label={`Grafik kolom per bulan: ${seri.map((s) => s.nama).join(", ")}`}
          style={{ touchAction: "pan-y" }}
          onPointerLeave={() => setAktif(null)}
        >
          {ticks.map((v) => (
            <g key={v}>
              <line x1={m.kiri} x2={lebar - m.kanan} y1={sy(v)} y2={sy(v)} stroke={v === 0 ? AXIS : GRID} strokeWidth={1} />
              <text x={m.kiri - 6} y={sy(v)} textAnchor="end" dominantBaseline="middle" fontSize={10.5} fill={MUTED}>
                {sumbu(v)}
              </text>
            </g>
          ))}
          {Array.from({ length: 12 }, (_, i) => {
            const x0 = m.kiri + i * lebarBulan + (lebarBulan - lebarBatang * seri.length - (seri.length - 1) * 2) / 2;
            return (
              <g key={i} onPointerEnter={() => setAktif(i)} onPointerMove={() => setAktif(i)}>
                {/* Area sentuh sepenuh kolom bulan, lebih besar dari batangnya. */}
                <rect x={m.kiri + i * lebarBulan} y={m.atas} width={lebarBulan} height={plotH + m.bawah} fill={aktif === i ? "#f1f5f9" : "transparent"} />
                {seri.map((s, k) => {
                  const v = nilaiDari(s.data[i]);
                  const tinggiBatang = dasar - sy(v);
                  if (v <= 0) return null;
                  return (
                    <rect
                      key={s.nama}
                      x={x0 + k * (lebarBatang + 2)}
                      y={sy(v)}
                      width={lebarBatang}
                      height={Math.max(tinggiBatang, 1.5)}
                      rx={Math.min(3, lebarBatang / 2)}
                      fill={s.warna}
                      opacity={aktif === null || aktif === i ? 1 : 0.55}
                    />
                  );
                })}
                <text x={m.kiri + i * lebarBulan + lebarBulan / 2} y={tinggi - 7} textAnchor="middle" fontSize={10.5} fill={aktif === i ? "#0f172a" : MUTED} fontWeight={aktif === i ? 600 : 400}>
                  {BULAN_SINGKAT[i]}
                </text>
              </g>
            );
          })}
        </svg>
        {aktif !== null && (
          <div
            className="pointer-events-none absolute top-0 z-10 min-w-[8rem] -translate-x-1/2 rounded-md border border-slate-200 bg-white px-2.5 py-1.5 text-xs shadow-md"
            style={{ left: Math.min(Math.max(m.kiri + aktif * lebarBulan + lebarBulan / 2, 70), lebar - 70) }}
          >
            <p className="font-semibold text-slate-900">{NAMA_BULAN[aktif]}</p>
            {seri.map((s) => (
              <p key={s.nama} className="mt-0.5 flex items-center gap-1.5 text-slate-600">
                <span className="h-2 w-2 shrink-0 rounded-sm" style={{ backgroundColor: s.warna }} aria-hidden="true" />
                <span className="min-w-0 flex-1 truncate">{s.nama}</span>
                <span className="font-medium text-slate-900">
                  {metrik === "nilai" ? rupiahRingkas(s.data[aktif]?.nilai ?? 0) : `${formatAngka(s.data[aktif]?.jumlah ?? 0)} ${satuanJumlah}`}
                </span>
              </p>
            ))}
          </div>
        )}
      </div>
    );
  })();

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        {seri.length > 1 ? <Legenda item={seri.map((s) => ({ label: s.nama, warna: s.warna }))} /> : <span />}
        {bolehGanti && (
          <div role="group" aria-label="Ukuran yang ditampilkan" className="inline-flex rounded-lg bg-slate-100 p-0.5 text-xs">
            {(["nilai", "jumlah"] as const).map((k) => (
              <button
                key={k}
                type="button"
                aria-pressed={metrik === k}
                onClick={() => setMetrik(k)}
                className={`rounded-md px-2.5 py-1 font-medium ${metrik === k ? "bg-white text-blue-700 shadow-sm" : "text-slate-500 hover:text-slate-800"}`}
              >
                {k === "nilai" ? "Nilai" : "Jumlah"}
              </button>
            ))}
          </div>
        )}
      </div>
      <div ref={ref} className="w-full">
        {isi}
      </div>
    </div>
  );
}

export function Legenda({ item }: { item: { label: string; warna: string }[] }) {
  return (
    <ul className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-600">
      {item.map((i) => (
        <li key={i.label} className="inline-flex items-center gap-1.5">
          <span className="h-2.5 w-2.5 shrink-0 rounded-sm" style={{ backgroundColor: i.warna }} aria-hidden="true" />
          {i.label}
        </li>
      ))}
    </ul>
  );
}

export interface BagianKomposisi {
  label: string;
  nilai: number;
  jumlah?: number;
  warna: string;
}

// Satu batang 100% (bagian dari keseluruhan) dengan daftar label, nilai, dan persentase di bawahnya.
export function Komposisi({ bagian, tampil = "rupiah", ringkasan }: { bagian: BagianKomposisi[]; tampil?: "rupiah" | "angka"; ringkasan: string }) {
  const terlihat = bagian.filter((b) => b.nilai > 0);
  const total = terlihat.reduce((a, b) => a + b.nilai, 0);
  if (total <= 0) return <p className="py-6 text-center text-sm text-slate-400">Tidak ada data</p>;
  const tulis = (v: number) => (tampil === "rupiah" ? rupiahRingkas(v) : formatAngka(v));
  return (
    <div className="space-y-4">
      <div className="flex h-3.5 gap-[2px]" role="img" aria-label={ringkasan}>
        {terlihat.map((b, i) => (
          <div
            key={b.label}
            className={`min-w-[3px] ${i === 0 ? "rounded-l-[4px]" : ""} ${i === terlihat.length - 1 ? "rounded-r-[4px]" : ""}`}
            style={{ flexGrow: b.nilai, flexBasis: 0, backgroundColor: b.warna }}
            title={`${b.label}: ${tulis(b.nilai)} (${formatPersen(bagi(b.nilai, total))})`}
          />
        ))}
      </div>
      <ul className="space-y-1.5">
        {terlihat.map((b) => (
          <li key={b.label} className="flex items-center gap-2 text-xs">
            <span className="h-2.5 w-2.5 shrink-0 rounded-sm" style={{ backgroundColor: b.warna }} aria-hidden="true" />
            <span className="min-w-0 flex-1 truncate text-slate-700" title={b.label}>
              {b.label}
            </span>
            {b.jumlah !== undefined && <span className="shrink-0 text-slate-400">{formatAngka(b.jumlah)} paket</span>}
            <span className="shrink-0 font-medium text-slate-900">{tulis(b.nilai)}</span>
            <span className="w-12 shrink-0 text-right text-slate-400">{formatPersen(bagi(b.nilai, total))}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

export interface ItemKolom {
  label: string;
  jumlah: number;
  warna?: string;
  tebal?: boolean; // diberi tanda (mis. rentang yang bermasalah)
  catatan?: string;
}

// Histogram sederhana (sebaran): kolom vertikal per kategori dengan jumlah di atasnya. Untuk beberapa kategori saja (<= 8).
export function Histogram({ item, satuan = "paket", ringkasan }: { item: ItemKolom[]; satuan?: string; ringkasan: string }) {
  const maks = Math.max(0, ...item.map((i) => i.jumlah));
  if (item.length === 0 || maks <= 0) return <p className="py-6 text-center text-sm text-slate-400">Tidak ada data</p>;
  const total = item.reduce((a, b) => a + b.jumlah, 0);
  return (
    <div role="img" aria-label={ringkasan}>
      <div className="flex h-40 items-end gap-2 border-b border-slate-300 px-1">
        {item.map((i) => (
          <div key={i.label} className="flex h-full min-w-0 flex-1 flex-col items-center justify-end" title={`${i.label}: ${formatAngka(i.jumlah)} ${satuan} (${formatPersen(bagi(i.jumlah, total))})`}>
            <span className="mb-1 text-xs font-medium text-slate-900 tabular-nums">{i.jumlah > 0 ? formatAngka(i.jumlah) : ""}</span>
            <div
              className="w-full max-w-[44px] origin-bottom animate-grow-y rounded-t-[4px]"
              style={{ height: `${Math.max((i.jumlah / maks) * 100, i.jumlah > 0 ? 2 : 0)}%`, backgroundColor: i.warna ?? WARNA.merek }}
            />
          </div>
        ))}
      </div>
      <div className="mt-1.5 flex gap-2 px-1">
        {item.map((i) => (
          <div key={i.label} className="min-w-0 flex-1 text-center text-[11px] leading-tight text-slate-500">
            <span className={i.tebal ? "font-semibold text-red-700" : ""}>{i.label}</span>
            {i.catatan && <span className="block text-slate-400">{i.catatan}</span>}
          </div>
        ))}
      </div>
    </div>
  );
}

// Batang horizontal berwarna per tahap (corong): label, jumlah paket, nilai, dan persentase terhadap total.
export function Corong({ tahap, total, warna }: { tahap: AnPasangan[]; total: number; warna: Record<string, string> }) {
  const maks = Math.max(0, ...tahap.map((t) => t.nilai));
  if (maks <= 0) return <p className="py-6 text-center text-sm text-slate-400">Tidak ada data RUP yang bisa dicocokkan</p>;
  return (
    <ul className="space-y-3">
      {tahap.map((t) => (
        <li key={t.label} className="space-y-1">
          <div className="flex flex-wrap items-baseline justify-between gap-x-3 text-xs">
            <span className="inline-flex items-center gap-1.5 font-medium text-slate-800">
              <span className="h-2.5 w-2.5 shrink-0 rounded-sm" style={{ backgroundColor: warna[t.label] ?? WARNA.netral }} aria-hidden="true" />
              {t.label}
            </span>
            <span className="text-slate-600">
              <span className="font-semibold text-slate-900">{rupiahRingkas(t.nilai)}</span> · {formatAngka(t.jumlah)} paket ·{" "}
              <span className="text-slate-500">{formatPersen(bagi(t.nilai, total))}</span>
            </span>
          </div>
          <div className="h-3 border-l border-slate-300" aria-hidden="true">
            <div className="h-full origin-left animate-grow-x rounded-r-[4px]" style={{ width: `${t.nilai > 0 ? Math.max((t.nilai / maks) * 100, 0.8) : 0}%`, backgroundColor: warna[t.label] ?? WARNA.netral }} />
          </div>
        </li>
      ))}
    </ul>
  );
}

// ---- wawasan ----

const GAYA_WAWASAN: Record<TingkatWawasan, { label: string; kelas: string; ikon: typeof Info; warnaIkon: string }> = {
  penting: { label: "Penting", kelas: "border-red-200 bg-red-50/60", ikon: XCircle, warnaIkon: "text-red-600" },
  perhatian: { label: "Perhatian", kelas: "border-amber-200 bg-amber-50/60", ikon: AlertTriangle, warnaIkon: "text-amber-600" },
  info: { label: "Info", kelas: "border-slate-200 bg-slate-50/70", ikon: Info, warnaIkon: "text-slate-500" },
  baik: { label: "Baik", kelas: "border-emerald-200 bg-emerald-50/60", ikon: CheckCircle2, warnaIkon: "text-emerald-600" },
};

// Daftar penjelasan analitik. Tiap kartu memakai ikon dan label tingkat (bukan warna saja).
export function DaftarWawasan({ wawasan, kosong = "Belum ada wawasan: data belum cukup untuk dianalisis." }: { wawasan: AnWawasan[]; kosong?: string }) {
  if (wawasan.length === 0) {
    return (
      <p className="flex items-center gap-2 rounded-xl border border-dashed border-slate-300 px-4 py-5 text-sm text-slate-500">
        <Lightbulb className="h-4 w-4 shrink-0 text-slate-400" aria-hidden="true" />
        {kosong}
      </p>
    );
  }
  return (
    <ul className="space-y-2.5">
      {wawasan.map((w, i) => {
        const g = GAYA_WAWASAN[w.tingkat];
        const Ikon = g.ikon;
        return (
          <li key={`${w.judul}-${i}`} className={`rounded-xl border px-4 py-3 ${g.kelas}`}>
            <div className="flex items-start gap-2.5">
              <Ikon className={`mt-0.5 h-4 w-4 shrink-0 ${g.warnaIkon}`} aria-hidden="true" />
              <div className="min-w-0">
                <p className="text-sm font-semibold text-slate-900">
                  {w.judul} <span className="ml-1 align-middle text-[10px] font-semibold uppercase tracking-wide text-slate-500">{g.label}</span>
                </p>
                <p className="mt-0.5 text-[13px] leading-relaxed text-slate-700">{w.isi}</p>
              </div>
            </div>
          </li>
        );
      })}
    </ul>
  );
}

// Angka pokok besar dengan keterangan; opsional bilah kecil menunjukkan porsi (0-100).
export function AngkaPokok({ label, nilai, keterangan, porsi, nada }: { label: string; nilai: string; keterangan?: ReactNode; porsi?: number; nada?: "baik" | "perhatian" | "penting" }) {
  return (
    <div className="min-w-0 rounded-2xl border border-slate-200/80 bg-gradient-to-br from-white to-slate-50 p-4 shadow-sm">
      <p className="text-xs font-medium uppercase tracking-wide text-slate-500">{label}</p>
      <p className="mt-1.5 text-2xl font-bold tracking-tight text-slate-900 tabular-nums">{nilai}</p>
      {porsi !== undefined && (
        <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-slate-200" aria-hidden="true">
          <div className="h-full rounded-full" style={{ width: `${Math.min(Math.max(porsi, 0), 100)}%`, backgroundColor: nada ? WARNA[nada] : WARNA.merek }} />
        </div>
      )}
      {keterangan && <p className="mt-1.5 break-words text-xs text-slate-500">{keterangan}</p>}
    </div>
  );
}

export { PERMUKAAN };
