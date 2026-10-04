"use client";

import { useState } from "react";
import { Loader2, Square } from "lucide-react";
import { PenarikanAktif, cancelPenarikan } from "@/lib/api";
import { formatAngka, formatDurasi, formatWaktu, labelParameter } from "@/lib/pengadaan";
import { Alert } from "@/components/ui/Alert";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { useToast } from "@/components/ui/Toast";
import { errorMessage } from "../digitalisasi/useDigitalisasi";
import { LencanaStatus, useSekarang } from "./lencana";

// Kemajuan antrean penarikan yang sedang berjalan; tampil di atas semua tab supaya terlihat dari mana pun.
export function PanelKemajuan({ aktif, isAdmin, onDibatalkan }: { aktif: PenarikanAktif; isAdmin: boolean; onDibatalkan: () => void }) {
  const toast = useToast();
  const sekarang = useSekarang(true);
  const [konfirmasi, setKonfirmasi] = useState(false);
  const [sibuk, setSibuk] = useState(false);
  const [galat, setGalat] = useState("");

  const jalan = aktif.tugas.filter((t) => t.status === "berjalan");
  const persen = aktif.total > 0 ? Math.round((aktif.selesai / aktif.total) * 100) : 0;
  const berhasil = aktif.tugas.filter((t) => t.status === "sukses").length;
  const gagal = aktif.tugas.filter((t) => t.status === "gagal").length;
  const baris = aktif.tugas.reduce((a, t) => a + (t.status === "sukses" ? t.jumlah_baris : 0), 0);
  const lama = Math.max(0, Math.floor((sekarang - new Date(aktif.mulai).getTime()) / 1000));

  const batalkan = async () => {
    setSibuk(true);
    setGalat("");
    try {
      await cancelPenarikan();
      toast.info("Permintaan pembatalan dikirim. Tugas yang sedang berjalan dihentikan.");
      onDibatalkan();
    } catch (err) {
      setGalat(errorMessage(err, "Gagal membatalkan penarikan"));
    } finally {
      setSibuk(false);
      setKonfirmasi(false);
    }
  };

  return (
    <section className="rounded-2xl border border-blue-200 bg-blue-50/50 p-4 sm:p-5" aria-live="polite" aria-label="Kemajuan penarikan data">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="flex items-center gap-2 text-sm font-semibold text-slate-900">
            <Loader2 className="h-4 w-4 animate-spin text-blue-600" aria-hidden="true" />
            {aktif.dibatalkan ? "Membatalkan penarikan..." : aktif.pemicu === "otomatis" ? "Penarikan otomatis sedang berjalan" : "Penarikan data sedang berjalan"}
          </p>
          <p className="mt-1 text-xs text-slate-600">
            Dimulai {formatWaktu(aktif.mulai)} · berjalan {formatDurasi(lama)}
            {isAdmin && aktif.oleh ? ` · oleh ${aktif.oleh}` : ""}
          </p>
        </div>
        {isAdmin && !aktif.dibatalkan && (
          <button
            type="button"
            onClick={() => setKonfirmasi(true)}
            className="inline-flex items-center gap-1.5 rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-50"
          >
            <Square className="h-3.5 w-3.5" aria-hidden="true" />
            Batalkan
          </button>
        )}
      </div>

      <div className="mt-3">
        <div className="flex items-center justify-between text-xs text-slate-600">
          <span>
            {aktif.selesai} dari {aktif.total} tugas
          </span>
          <span className="font-medium text-slate-900">{persen}%</span>
        </div>
        <div className="mt-1 h-2 overflow-hidden rounded-full bg-blue-100" role="progressbar" aria-valuemin={0} aria-valuemax={aktif.total} aria-valuenow={aktif.selesai} aria-label="Kemajuan penarikan">
          <div className="h-full rounded-full bg-blue-600 transition-[width] duration-500" style={{ width: `${persen}%` }} />
        </div>
        <p className="mt-1.5 text-xs text-slate-500">
          {berhasil} berhasil · {gagal} gagal · {formatAngka(baris)} baris tersimpan
        </p>
      </div>

      {jalan.map((t) => (
        <p key={t.id} className="mt-3 rounded-lg bg-white/70 px-3 py-2 text-xs text-slate-700">
          <span className="font-semibold text-slate-900">{t.nama}</span> <span className="text-slate-500">({labelParameter(t.parameter)})</span>
          {t.percobaan > 1 && <span className="text-amber-700"> · percobaan ke-{t.percobaan}</span>}
          {t.pesan && <span className="mt-0.5 block break-words text-slate-500">{t.pesan}</span>}
        </p>
      ))}

      {galat && (
        <div className="mt-3">
          <Alert message={galat} />
        </div>
      )}

      <details className="mt-3 group">
        <summary className="cursor-pointer select-none text-xs font-medium text-blue-700 hover:text-blue-800">Lihat semua tugas ({aktif.tugas.length})</summary>
        <ul className="mt-2 max-h-72 divide-y divide-blue-100 overflow-y-auto rounded-lg bg-white/70">
          {aktif.tugas.map((t) => (
            <li key={t.id} className="flex flex-wrap items-center gap-x-2 gap-y-0.5 px-3 py-2 text-xs">
              <LencanaStatus status={t.status} />
              <span className="font-medium text-slate-800">{t.nama}</span>
              <span className="text-slate-500">{labelParameter(t.parameter)}</span>
              {t.status === "sukses" && <span className="text-slate-500">{formatAngka(t.jumlah_baris)} baris</span>}
              {t.status !== "sukses" && t.status !== "berjalan" && t.status !== "antri" && t.pesan && <span className="w-full break-words text-slate-500">{t.pesan}</span>}
            </li>
          ))}
        </ul>
      </details>
      <p className="mt-3 text-xs text-slate-500">Anda boleh meninggalkan halaman ini; penarikan berjalan di server.</p>

      {konfirmasi && (
        <ConfirmDialog
          title="Batalkan penarikan?"
          tone="warning"
          message="Tugas yang sedang berjalan dihentikan dan sisanya tidak dimulai. Data yang sudah ditarik tetap tersimpan; dataset yang belum selesai tetap berisi data sebelumnya."
          confirmLabel="Batalkan penarikan"
          cancelLabel="Kembali"
          busy={sibuk}
          onConfirm={batalkan}
          onCancel={() => setKonfirmasi(false)}
        />
      )}
    </section>
  );
}
