"use client";

import { useMemo, useState } from "react";
import { ChevronDown, Code2, MapPin, Search } from "lucide-react";
import type { DatasetDok, Dokumentasi } from "@/lib/apiAkses";
import { cariDataset, penyaringDataset, tipeJson } from "@/lib/apiAksesDokumentasi";
import { kelompokkanDataset } from "@/lib/apiAksesFormat";
import { CopyButton } from "@/components/hris2/CopyButton";
import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { Kode } from "./KotakKode";

const inputCls =
  "w-full rounded-xl border border-slate-300 bg-white py-2 pl-9 pr-3 text-sm text-slate-900 shadow-sm outline-none transition-all placeholder:text-slate-400 hover:border-slate-400 focus:border-blue-500 focus:shadow-glow";

function Lencana({ children, redup }: { children: string; redup?: boolean }) {
  return (
    <span className={`rounded-md px-1.5 py-0.5 font-mono text-xs ${redup ? "bg-slate-100 text-slate-400 line-through" : "bg-blue-50 text-blue-700"}`}>{children}</span>
  );
}

function RincianDataset({ ds, dok, alamatLengkap, onPakaiContoh }: { ds: DatasetDok; dok: Dokumentasi; alamatLengkap: string; onPakaiContoh: (kode: string) => void }) {
  const penyaring = penyaringDataset(ds, dok.parameter);
  return (
    <div className="space-y-4 border-t border-slate-100 px-4 py-4">
      <p className="text-sm text-slate-600">{ds.deskripsi}</p>

      <div className="flex flex-wrap items-center gap-2">
        <span className="text-xs font-medium text-slate-500">Alamat</span>
        <span className="break-all rounded-md bg-slate-100 px-2 py-1 font-mono text-xs text-slate-800">GET {alamatLengkap}</span>
        <CopyButton text={alamatLengkap} label={`Salin alamat ${ds.label}`} />
        <Button fullWidth={false} variant="soft" size="sm" icon={<Code2 className="h-3.5 w-3.5" aria-hidden="true" />} onClick={() => onPakaiContoh(ds.kode)}>
          Pakai di contoh kode
        </Button>
      </div>

      <div className="space-y-1.5">
        <p className="text-xs font-medium text-slate-500">Penyaring yang didukung</p>
        <div className="flex flex-wrap gap-1.5">
          {penyaring.didukung.map((n) => (
            <Lencana key={n}>{n}</Lencana>
          ))}
        </div>
        {penyaring.ditolak.length > 0 && (
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="text-xs text-slate-500">Tidak didukung (ditolak 400):</span>
            {penyaring.ditolak.map((n) => (
              <Lencana key={n} redup>
                {n}
              </Lencana>
            ))}
          </div>
        )}
      </div>

      <div className="space-y-1.5">
        <p className="text-xs font-medium text-slate-500">Kolom pada setiap baris ({ds.kolom.length})</p>
        <div className="max-h-80 overflow-auto rounded-xl border border-slate-200">
          <table className="w-full min-w-[20rem] text-sm">
            <thead className="sticky top-0 bg-slate-50 text-xs text-slate-500">
              <tr>
                <th scope="col" className="px-3 py-2 text-left font-medium">
                  Kolom
                </th>
                <th scope="col" className="px-3 py-2 text-left font-medium">
                  Tipe JSON
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {ds.kolom.map((k) => (
                <tr key={k.nama}>
                  <td className="px-3 py-1.5 font-mono text-xs text-slate-800">{k.nama}</td>
                  <td className="px-3 py-1.5 text-xs text-slate-600">{tipeJson(k.tipe)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {ds.tertutup && ds.tertutup.length > 0 && (
        <div className="space-y-1.5">
          <p className="text-xs font-medium text-slate-500">Kolom tabel sumber yang sengaja tidak diberikan ({ds.tertutup.length})</p>
          <p className="break-words font-mono text-xs leading-relaxed text-slate-500">{ds.tertutup.join(", ")}</p>
        </div>
      )}
    </div>
  );
}

// Referensi dataset: semua dataset (bukan hanya yang dalam scope satu kunci) dengan alamat, penyaring, dan kolomnya, bisa dicari. Dibaca dari katalog yang sama dengan yang dipakai
// API untuk klien, jadi selalu sama dengan perilaku sebenarnya.
export function DokumentasiDataset({ dok, akar, onPakaiContoh }: { dok: Dokumentasi; akar: string; onPakaiContoh: (kode: string) => void }) {
  const [kata, setKata] = useState("");
  const [terbuka, setTerbuka] = useState<Record<string, boolean>>({});
  const hasil = useMemo(() => cariDataset(dok.dataset, kata), [dok.dataset, kata]);
  const kelompok = useMemo(() => kelompokkanDataset(hasil), [hasil]);

  return (
    <div className="space-y-4">
      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
        <input
          type="search"
          value={kata}
          onChange={(e) => setKata(e.target.value)}
          placeholder="Cari dataset, uraian, atau nama kolom (mis. tender, npwp, luas)"
          aria-label="Cari dataset"
          className={inputCls}
        />
      </div>
      <p className="text-xs text-slate-500" aria-live="polite">
        {hasil.length === dok.dataset.length ? `${dok.dataset.length} dataset` : `${hasil.length} dari ${dok.dataset.length} dataset`}
      </p>

      {hasil.length === 0 ? (
        <EmptyState title="Tidak ada dataset yang cocok" description="Coba kata lain, atau hapus pencarian." />
      ) : (
        kelompok.map((k) => (
          <div key={k.kelompok} className="space-y-3">
            <h3 className="text-sm font-semibold text-slate-900">
              {k.judul} <span className="font-normal text-slate-400">· {k.kode.length}</span>
            </h3>
            {k.bagian.map((b) => (
              <div key={b.judul} className="space-y-2">
                {b.judul && <p className="text-xs font-medium uppercase tracking-wide text-slate-400">{b.judul}</p>}
                <ul className="space-y-2">
                  {b.dataset.map((d) => {
                    const buka = !!terbuka[d.kode];
                    const idRincian = `dok-ds-${d.kode.replace(/\W/g, "-")}`;
                    return (
                      <li key={d.kode} className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
                        <button
                          type="button"
                          onClick={() => setTerbuka({ ...terbuka, [d.kode]: !buka })}
                          aria-expanded={buka}
                          aria-controls={idRincian}
                          className="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-slate-50"
                        >
                          <span className="min-w-0 flex-1">
                            <span className="block text-sm font-medium text-slate-900">{d.label}</span>
                            <span className="block break-all font-mono text-xs text-slate-500">{d.kode}</span>
                          </span>
                          {d.geo && (
                            <span className="hidden items-center gap-1 rounded-md bg-emerald-50 px-1.5 py-0.5 text-xs text-emerald-700 sm:inline-flex" title="Memuat koordinat">
                              <MapPin className="h-3 w-3" aria-hidden="true" />
                              Koordinat
                            </span>
                          )}
                          <span className="shrink-0 text-xs text-slate-500">{d.kolom.length} kolom</span>
                          <ChevronDown className={`h-4 w-4 shrink-0 text-slate-400 transition-transform ${buka ? "rotate-180" : ""}`} aria-hidden="true" />
                        </button>
                        {buka && (
                          <div id={idRincian}>
                            <RincianDataset ds={d} dok={dok} alamatLengkap={`${akar}${d.alamat}`} onPakaiContoh={onPakaiContoh} />
                          </div>
                        )}
                      </li>
                    );
                  })}
                </ul>
              </div>
            ))}
          </div>
        ))
      )}

      <p className="text-xs text-slate-500">
        Nama kolom selalu huruf kecil, nilai desimal berupa angka JSON, dan setiap baris memuat <Kode>id</Kode> dan <Kode>disinkronkan_pada</Kode>.
      </p>
    </div>
  );
}
