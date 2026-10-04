"use client";

import { ReactNode, useEffect, useState } from "react";
import { AlertTriangle, Ban, CheckCircle2, Clock, Loader2, RefreshCw, XCircle } from "lucide-react";
import { DGAutoInfo, DGDatasetKey, DGSyncLog, DGSyncOverview, cancelDGSync, startDGSync } from "@/lib/api";
import { Alert } from "@/components/ui/Alert";
import { ModalShell } from "@/components/ui/ModalShell";
import { formatNumber } from "../sldk/asset";
import { formatDateTime } from "../sldk/overview";
import {
  DATASET_LABEL,
  STATUS_META,
  StatusTone,
  durationBetween,
  elapsedSince,
  formatDuration,
  queueState,
} from "./digitalisasi";
import { AsyncState, errorMessage } from "./useDigitalisasi";

interface Props {
  sync: AsyncState<DGSyncOverview>;
  isAdmin: boolean;
}

const TONE: Record<StatusTone, { cls: string; Icon: typeof CheckCircle2 }> = {
  ok: { cls: "bg-emerald-50 text-emerald-700", Icon: CheckCircle2 },
  fail: { cls: "bg-red-50 text-red-700", Icon: XCircle },
  run: { cls: "bg-blue-50 text-blue-700", Icon: Loader2 },
  wait: { cls: "bg-slate-100 text-slate-600", Icon: Clock },
  stop: { cls: "bg-amber-50 text-amber-700", Icon: Ban },
};

// Status selalu memakai ikon + teks, bukan warna saja.
export function StatusBadge({ status }: { status: DGSyncLog["status"] }) {
  const meta = STATUS_META[status];
  const { cls, Icon } = TONE[meta.tone];
  return (
    <span className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${cls}`}>
      <Icon className={`h-3.5 w-3.5 ${meta.tone === "run" ? "animate-spin" : ""}`} aria-hidden="true" />
      {meta.label}
    </span>
  );
}

function useNow(enabled: boolean): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!enabled) return;
    setNow(Date.now());
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [enabled]);
  return now;
}

type Pending = { keys: DGDatasetKey[]; all: boolean } | "cancel" | null;

export function SyncPanel({ sync, isAdmin }: Props) {
  const { data, isLoading, error, reload } = sync;
  const [selected, setSelected] = useState<Set<DGDatasetKey>>(new Set());
  const [pending, setPending] = useState<Pending>(null);
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState("");
  const active = data?.aktif ?? null;
  const now = useNow(active !== null);

  if (!data) {
    if (error) {
      return (
        <div className="space-y-3">
          <Alert message={error} />
          <button type="button" onClick={reload} className="text-sm font-medium text-blue-600 hover:text-blue-700">
            Coba lagi
          </button>
        </div>
      );
    }
    return <div role="status" aria-label="Memuat status sinkronisasi" className="h-40 animate-pulse rounded-xl bg-slate-100" />;
  }

  const canStart = isAdmin && data.sldk_tersedia && !active;
  const toggle = (key: DGDatasetKey) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const confirm = async () => {
    if (!pending) return;
    setBusy(true);
    setActionError("");
    try {
      if (pending === "cancel") await cancelDGSync();
      else await startDGSync(pending.all ? { semua: true } : { datasets: pending.keys });
      if (pending !== "cancel") setSelected(new Set());
      setPending(null);
      reload();
    } catch (err) {
      setActionError(errorMessage(err, "Permintaan gagal"));
      setPending(null);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-5">
      {!data.sldk_tersedia && (
        <Notice tone="warning">
          Koneksi ke SLDK belum tersedia di server ini, jadi sinkronisasi tidak bisa dijalankan. Periksa pengaturan <code>SLDK_DB_*</code> dan izin akses IP server di SLDK.
        </Notice>
      )}
      {actionError && <Alert message={actionError} />}
      {error && <Alert message={`${error}. Menampilkan status yang dimuat sebelumnya.`} />}

      {active && (
        <section className="rounded-xl border border-blue-200 bg-blue-50/50 p-4" aria-live="polite">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="flex items-center gap-2 text-sm font-semibold text-slate-900">
              <Loader2 className="h-4 w-4 animate-spin text-blue-600" aria-hidden="true" />
              Sinkronisasi sedang berjalan
            </p>
            {isAdmin && (
              <button
                type="button"
                onClick={() => setPending("cancel")}
                className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-50"
              >
                Batalkan
              </button>
            )}
          </div>
          <p className="mt-1 text-xs text-slate-600">
            Dimulai {formatDateTime(active.mulai)} · berjalan {formatDuration(elapsedSince(active.mulai, now))}
            {isAdmin && active.oleh ? ` · oleh ${active.oleh}` : ""}
          </p>
          <ol className="mt-3 space-y-1.5">
            {active.datasets.map((key) => {
              const st = queueState(active.datasets, active.saat_ini, key);
              const log = data.datasets.find((d) => d.key === key)?.terakhir;
              // Baris riwayat milik antrean ini: yang dibuat sejak antrean dimulai.
              const mine = log && new Date(log.dibuat).getTime() >= new Date(active.mulai).getTime() - 5000 ? log : undefined;
              return (
                <li key={key} className="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-sm">
                  {mine ? <StatusBadge status={mine.status} /> : <StatusBadge status={st === "jalan" ? "berjalan" : "antri"} />}
                  <span className="font-medium text-slate-800">{DATASET_LABEL[key]}</span>
                  {mine?.pesan && (st === "jalan" || mine.status === "sukses") && <span className="min-w-0 break-words text-xs text-slate-500">{mine.pesan}</span>}
                </li>
              );
            })}
          </ol>
          <p className="mt-3 text-xs text-slate-500">Anda boleh meninggalkan halaman ini; sinkronisasi berjalan di server.</p>
        </section>
      )}

      <section>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h3 className="text-sm font-semibold text-slate-900">Dataset</h3>
            <p className="text-xs text-slate-500">Tiap dataset adalah satu query ke SLDK; isi tabelnya diganti penuh setelah seluruh data terbaca.</p>
            <p className="mt-0.5 text-xs text-slate-500">
              <Clock className="mr-1 inline h-3 w-3 align-[-1px]" aria-hidden="true" />
              {jadwalOtomatis(data.otomatis)}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={reload}
              disabled={isLoading}
              className="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium text-slate-600 hover:bg-slate-100 disabled:opacity-60"
            >
              {isLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
              Muat ulang
            </button>
            {isAdmin && (
              <>
                <button
                  type="button"
                  onClick={() => setPending({ keys: [...selected], all: false })}
                  disabled={!canStart || selected.size === 0}
                  className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50"
                >
                  Sinkronkan terpilih{selected.size > 0 ? ` (${selected.size})` : ""}
                </button>
                <button
                  type="button"
                  onClick={() => setPending({ keys: data.datasets.map((d) => d.key), all: true })}
                  disabled={!canStart}
                  className="rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50"
                >
                  Sinkronkan semua
                </button>
              </>
            )}
          </div>
        </div>
        {!isAdmin && <p className="mt-2 text-xs text-slate-500">Hanya admin yang dapat menjalankan sinkronisasi.</p>}

        <ul className="mt-3 divide-y divide-slate-100 overflow-hidden rounded-lg border border-slate-200">
          {data.datasets.map((d) => {
            const ok = d.terakhir_sukses;
            const failedNewer = d.terakhir && d.terakhir.status !== "sukses" && (!ok || d.terakhir.id > ok.id) ? d.terakhir : null;
            const dur = ok ? durationBetween(ok.mulai, ok.selesai) : null;
            return (
              <li key={d.key} className="flex items-start gap-3 px-3 py-3 sm:px-4">
                {isAdmin && (
                  <input
                    type="checkbox"
                    aria-label={`Pilih ${d.label}`}
                    checked={selected.has(d.key)}
                    onChange={() => toggle(d.key)}
                    disabled={!!active}
                    className="mt-1 h-4 w-4 shrink-0 rounded border-slate-300 text-blue-600 focus:ring-blue-500 disabled:opacity-50"
                  />
                )}
                <div className="min-w-0 flex-1 space-y-1">
                  <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                    <p className="text-sm font-semibold text-slate-900">{d.label}</p>
                    {failedNewer && <StatusBadge status={failedNewer.status} />}
                  </div>
                  <p className="text-xs text-slate-500">{d.deskripsi}</p>
                  <p className="text-xs text-slate-600">
                    {formatNumber(d.jumlah_baris, 0)} baris tersimpan
                    {ok ? (
                      <span className="text-slate-400">
                        {" "}
                        · terakhir {formatDateTime(ok.selesai)}
                        {dur !== null ? ` (${formatDuration(dur)})` : ""}
                        {d.peta && ok.jumlah_koordinat !== null ? ` · ${formatNumber(ok.jumlah_koordinat, 0)} berkoordinat` : ""}
                      </span>
                    ) : (
                      <span className="text-amber-700"> · belum pernah disinkronkan</span>
                    )}
                  </p>
                  {failedNewer?.pesan && failedNewer.status !== "berjalan" && failedNewer.status !== "antri" && (
                    <p className="break-words text-xs text-slate-500">{failedNewer.pesan}</p>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      </section>

      <section>
        <h3 className="text-sm font-semibold text-slate-900">Riwayat</h3>
        {data.riwayat.length === 0 ? (
          <p className="mt-2 text-sm text-slate-500">Belum ada riwayat sinkronisasi.</p>
        ) : (
          <ul className="mt-3 divide-y divide-slate-100 overflow-hidden rounded-lg border border-slate-200">
            {data.riwayat.map((r) => {
              const dur = durationBetween(r.mulai, r.selesai);
              return (
                <li key={r.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2.5 text-xs sm:px-4">
                  <StatusBadge status={r.status} />
                  <span className="font-medium text-slate-800">{DATASET_LABEL[r.dataset] ?? r.dataset}</span>
                  <span className="text-slate-500">{formatDateTime(r.mulai ?? r.dibuat)}</span>
                  {dur !== null && <span className="text-slate-400">{formatDuration(dur)}</span>}
                  {r.jumlah_baris !== null && <span className="text-slate-500">{formatNumber(r.jumlah_baris, 0)} baris</span>}
                  {isAdmin && r.dijalankan_oleh && <span className="text-slate-400">oleh {r.dijalankan_oleh}</span>}
                  {r.pesan && r.status !== "sukses" && <span className="w-full break-words text-slate-500">{r.pesan}</span>}
                </li>
              );
            })}
          </ul>
        )}
      </section>

      {pending && (
        <ConfirmModal
          pending={pending}
          datasets={data.datasets.map((d) => ({ key: d.key, label: d.label }))}
          busy={busy}
          onCancel={() => setPending(null)}
          onConfirm={confirm}
        />
      )}
    </div>
  );
}

// Kalimat jadwal sinkronisasi otomatis. Sinkronisasi manual maupun otomatis berjalan tanpa batas waktu sampai selesai.
function jadwalOtomatis(o: DGAutoInfo | undefined): string {
  if (!o || !o.aktif) return "Sinkronisasi otomatis tidak aktif di server ini; sinkronkan secara manual.";
  const jam = (n: number) => String(n).padStart(2, "0") + ".00";
  const periode = o.interval_hari === 7 ? "setiap minggu" : `setiap ${o.interval_hari} hari`;
  const jendela = o.jam_mulai === o.jam_akhir ? "" : `, dimulai antara pukul ${jam(o.jam_mulai)} dan ${jam(o.jam_akhir)} ${o.zona}`;
  const next = o.berikutnya ? ` Berikutnya sekitar ${formatDateTime(o.berikutnya)}.` : "";
  return `Otomatis ${periode}${jendela}, dihitung dari sinkronisasi sukses terakhir tiap dataset.${next}`;
}

function Notice({ tone, children }: { tone: "warning"; children: ReactNode }) {
  return (
    <div className={`flex items-start gap-2 rounded-lg px-3.5 py-2.5 text-xs ${tone === "warning" ? "bg-amber-50 text-amber-800" : ""}`}>
      <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
      <span>{children}</span>
    </div>
  );
}

function ConfirmModal({
  pending,
  datasets,
  busy,
  onCancel,
  onConfirm,
}: {
  pending: NonNullable<Pending>;
  datasets: { key: DGDatasetKey; label: string }[];
  busy: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const isCancel = pending === "cancel";
  const names = isCancel ? [] : pending.keys.map((k) => datasets.find((d) => d.key === k)?.label ?? k);
  return (
    <ModalShell title={isCancel ? "Batalkan sinkronisasi?" : "Mulai sinkronisasi dari SLDK?"} onClose={onCancel}>
      <div className="space-y-4 px-4 py-4 sm:px-6">
        {isCancel ? (
          <p className="text-sm text-slate-600">
            Query yang sedang berjalan di SLDK dihentikan. Data yang sudah tersimpan tidak berubah; dataset yang belum selesai tetap berisi data sebelumnya.
          </p>
        ) : (
          <>
            <p className="text-sm text-slate-600">
              Dataset: <span className="font-medium text-slate-900">{names.join(", ")}</span>
            </p>
            <Notice tone="warning">
              Tiap dataset menjalankan query berat ke tabel aset SLDK (ratusan GB) dan dapat memakan waktu lama. Tidak ada batas waktu: sinkronisasi berjalan
              sampai selesai dan bisa dibatalkan kapan saja. Sebaiknya dijalankan di luar jam kerja. Isi tabel dataset diganti penuh setelah seluruh data
              terbaca; bila gagal, data lama tetap utuh.
            </Notice>
          </>
        )}
        <div className="flex justify-end gap-2 border-t border-slate-100 pt-4">
          <button type="button" onClick={onCancel} className="rounded-lg px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100">
            Kembali
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={busy}
            className="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60"
          >
            {busy && <Loader2 className="h-4 w-4 animate-spin" />}
            {isCancel ? "Batalkan sinkronisasi" : "Mulai sinkronisasi"}
          </button>
        </div>
      </div>
    </ModalShell>
  );
}
