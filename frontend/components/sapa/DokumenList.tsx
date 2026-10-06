"use client";

import { useState } from "react";
import { AlertTriangle, FileText, FileDown, Loader2 } from "lucide-react";
import { SapaDokumen, unduhSapaDokumen } from "@/lib/sapa";
import { formatDateTime } from "@/lib/dasbor";
import { simpanBlob } from "./download";
import { ErrorBox } from "./fields";
import { ErrorInfo, errorInfo, formatUkuran } from "./sapa";

// Dokumen Word hasil pengisian template. Yang terbaru ditandai; versi lama tetap bisa diunduh sebagai riwayat.
export function DokumenList({ dokumen }: { dokumen: SapaDokumen[] }) {
  const [busyId, setBusyId] = useState<number | null>(null);
  const [error, setError] = useState<ErrorInfo | null>(null);

  if (dokumen.length === 0) return null;

  const unduh = async (d: SapaDokumen) => {
    setBusyId(d.id);
    setError(null);
    try {
      const { blob, disposition } = await unduhSapaDokumen(d.id);
      simpanBlob(blob, disposition, d.nama_file);
    } catch (err) {
      setError(errorInfo(err, "Gagal mengunduh dokumen"));
    } finally {
      setBusyId(null);
    }
  };

  return (
    <div className="space-y-2">
      <h4 className="text-xs font-semibold uppercase tracking-wide text-slate-500">Dokumen hasil</h4>
      <ul className="space-y-2">
        {dokumen.map((d, i) => (
          <li key={d.id} className="rounded-lg border border-slate-200 bg-white p-3">
            <div className="flex flex-wrap items-center gap-3">
              <FileText className="h-5 w-5 shrink-0 text-blue-600" aria-hidden="true" />
              <div className="min-w-0 flex-1">
                <p className="break-words text-sm font-medium text-slate-900">
                  {d.nama_file}
                  {i === 0 && dokumen.length > 1 && (
                    <span className="ml-2 rounded-full bg-blue-50 px-2 py-0.5 text-[11px] font-medium text-blue-700">Terbaru</span>
                  )}
                </p>
                <p className="mt-0.5 text-xs text-slate-500">
                  {d.jenis_label} · {formatUkuran(d.ukuran)} · {d.dibuat_oleh || "-"} · {formatDateTime(d.dibuat_pada)}
                </p>
              </div>
              <button
                type="button"
                onClick={() => unduh(d)}
                disabled={busyId !== null}
                className="inline-flex items-center gap-1.5 rounded-lg bg-blue-50 px-3 py-2 text-sm font-medium text-blue-700 transition hover:bg-blue-100 disabled:opacity-60"
              >
                {busyId === d.id ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" /> : <FileDown className="h-4 w-4" aria-hidden="true" />}
                Unduh
              </button>
            </div>
            {d.peringatan && d.peringatan.length > 0 && (
              <div className="mt-2 flex items-start gap-2 rounded-md bg-amber-50 px-2.5 py-2 text-xs text-amber-800">
                <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                <ul className="space-y-0.5">
                  {d.peringatan.map((p, k) => (
                    <li key={k}>{p}</li>
                  ))}
                </ul>
              </div>
            )}
          </li>
        ))}
      </ul>
      <ErrorBox error={error} />
    </div>
  );
}
