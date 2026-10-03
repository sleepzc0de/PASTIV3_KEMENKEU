"use client";

import { useId } from "react";
import { Bot } from "lucide-react";
import { SapaDokPendukung, SapaItemDokumen } from "@/lib/sapa";
import { inputCls } from "./fields";
import { dokBawaan } from "./sapa";

interface Props {
  items: SapaItemDokumen[];
  value: Record<string, SapaDokPendukung>;
  onChange: (v: Record<string, SapaDokPendukung>) => void;
}

// Checklist kelengkapan dokumen pendukung. Dokumen yang disusun aplikasi dalam berkas Nota Dinas (daftar barang, SKPP,
// SPTJ, surat pernyataan) otomatis "ada" dan tidak perlu diisi.
export function ChecklistEditor({ items, value, onChange }: Props) {
  const ubah = (kunci: string, patch: Partial<SapaDokPendukung>, item: SapaItemDokumen) =>
    onChange({ ...value, [kunci]: { ...(value[kunci] ?? dokBawaan(item)), ...patch } });

  return (
    <ol className="divide-y divide-slate-100 rounded-lg border border-slate-200">
      {items.map((it, i) => (
        <Baris key={it.kunci} no={i + 1} item={it} nilai={value[it.kunci] ?? dokBawaan(it)} onChange={(p) => ubah(it.kunci, p, it)} />
      ))}
    </ol>
  );
}

function Baris({ no, item, nilai, onChange }: { no: number; item: SapaItemDokumen; nilai: SapaDokPendukung; onChange: (p: Partial<SapaDokPendukung>) => void }) {
  const id = useId();
  const wajibBA = item.kunci === "ba" && nilai.ada;
  return (
    <li className="p-3">
      <div className="flex items-start gap-3">
        <span className="mt-0.5 w-5 shrink-0 text-right text-xs text-slate-400">{no}.</span>
        <div className="min-w-0 flex-1">
          <p className="text-sm text-slate-800">{item.label}</p>
          {item.otomatis ? (
            <p className="mt-1 inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-2 py-0.5 text-xs text-emerald-700">
              <Bot className="h-3.5 w-3.5" aria-hidden="true" />
              Disusun otomatis dalam berkas Nota Dinas
            </p>
          ) : (
            <>
              <label htmlFor={id} className="mt-1.5 inline-flex cursor-pointer items-center gap-2 text-sm text-slate-700">
                <input
                  id={id}
                  type="checkbox"
                  checked={nilai.ada}
                  onChange={(e) => onChange({ ada: e.target.checked })}
                  className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
                />
                Dokumen ini ada
              </label>
              {nilai.ada && (
                <div className="mt-2 grid gap-2 sm:grid-cols-2">
                  <div>
                    <label htmlFor={id + "-nomor"} className="mb-1 block text-[11px] font-medium text-slate-500">
                      Nomor{wajibBA ? " *" : " (opsional)"}
                    </label>
                    <input
                      id={id + "-nomor"}
                      value={nilai.nomor}
                      onChange={(e) => onChange({ nomor: e.target.value })}
                      maxLength={150}
                      autoComplete="off"
                      className={inputCls}
                    />
                  </div>
                  <div>
                    <label htmlFor={id + "-tanggal"} className="mb-1 block text-[11px] font-medium text-slate-500">
                      Tanggal{wajibBA ? " *" : " (opsional)"}
                    </label>
                    <input id={id + "-tanggal"} type="date" value={nilai.tanggal} onChange={(e) => onChange({ tanggal: e.target.value })} className={inputCls} />
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </li>
  );
}
