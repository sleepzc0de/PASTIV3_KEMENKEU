"use client";

import { useEffect, useState } from "react";
import { CheckCircle2, Loader2, RefreshCcw, XCircle } from "lucide-react";
import { getInaprocSyncLog } from "@/lib/api";

const MAX_ROWS = 5;

type SyncLogRow = Record<string, unknown>;

// "paket-anggaran-penyedia" -> "Paket Anggaran Penyedia"
function formatEndpoint(endpoint: unknown): string {
  return String(endpoint ?? "")
    .split("-")
    .filter(Boolean)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ");
}

function formatFinishedAt(value: unknown): string {
  const date = new Date(String(value ?? ""));
  if (Number.isNaN(date.getTime())) return "-";
  return date.toLocaleString("id-ID", {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function SyncActivityCard() {
  const [rows, setRows] = useState<SyncLogRow[] | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    getInaprocSyncLog()
      .then((res) => {
        if (!cancelled) setRows((res.data ?? []).slice(0, MAX_ROWS));
      })
      .catch(() => {
        if (!cancelled) setFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <section className="rounded-xl bg-white p-5 shadow-sm">
      <div className="mb-3 flex items-center gap-2">
        <RefreshCcw className="h-4 w-4 text-blue-600" />
        <h2 className="text-sm font-semibold text-slate-900">Sinkronisasi Inaproc Terakhir</h2>
      </div>

      {failed ? (
        <p className="rounded-lg bg-red-50 px-3 py-2 text-xs text-red-700">
          Gagal memuat riwayat sinkronisasi.
        </p>
      ) : rows === null ? (
        <div className="flex justify-center py-6">
          <Loader2 className="h-5 w-5 animate-spin text-blue-600" />
        </div>
      ) : rows.length === 0 ? (
        <p className="rounded-lg bg-slate-50 px-3 py-4 text-center text-xs text-slate-500">
          Belum ada sinkronisasi yang dijalankan.
        </p>
      ) : (
        <ul className="divide-y divide-slate-100">
          {rows.map((row, idx) => {
            const ok = row.status === "success";
            return (
              <li key={idx} className="flex items-start gap-2.5 py-2.5">
                {ok ? (
                  <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-green-600" />
                ) : (
                  <XCircle className="mt-0.5 h-4 w-4 shrink-0 text-red-500" />
                )}
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-slate-900">
                    {formatEndpoint(row.endpoint)}
                  </p>
                  <p className="text-xs text-slate-500">
                    {ok
                      ? `${Number(row.total_rows_synced ?? 0).toLocaleString("id-ID")} baris`
                      : "Gagal"}
                    {row.tahun ? ` · TA ${String(row.tahun)}` : ""}
                  </p>
                </div>
                <time className="shrink-0 text-xs text-slate-400">
                  {formatFinishedAt(row.finished_at)}
                </time>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
