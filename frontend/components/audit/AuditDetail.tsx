"use client";

import { Search } from "lucide-react";
import type { EntriAudit, OpsiAudit } from "@/lib/audit";
import { LABEL_HASIL, detailBaris, kelasHasil, labelPeran, ringkasUserAgent, waktuWIB } from "@/lib/auditFilter";
import { DetailEntry } from "@/components/ui/DetailEntry";
import { ModalShell } from "@/components/ui/ModalShell";
import { LencanaHasil } from "./lencana";

// Rincian lengkap satu entri log audit.
export function AuditDetail({ e, opsi, onTutup, onLihatPengguna }: { e: EntriAudit; opsi: OpsiAudit | null; onTutup: () => void; onLihatPengguna: (userId: string, username: string) => void }) {
  const kat = opsi?.kategori.find((k) => k.kode === e.kategori)?.label ?? e.kategori;
  const rincian = detailBaris(e.detail);
  const objek = [e.objek_tipe, e.objek_id].filter(Boolean).join(": ");
  return (
    <ModalShell title="Rincian aktivitas" subtitle={e.label} onClose={onTutup} size="lg">
      <div className="space-y-4">
        <div className="flex flex-wrap items-center gap-2">
          <LencanaHasil kelas={kelasHasil(e)} />
          <span className="rounded-full bg-slate-100 px-2.5 py-0.5 text-xs font-medium text-slate-600">{kat}</span>
          <span className="font-mono text-xs text-slate-500">{e.aksi}</span>
        </div>

        <DetailEntry
          fields={[
            ["Waktu (WIB)", waktuWIB(e.waktu)],
            ["Waktu (UTC)", e.waktu],
            ["Pengguna", e.username ? `${e.nama_lengkap ? e.nama_lengkap + " · " : ""}${e.username}` : "Tanpa pengguna (belum login atau akun tidak dikenal)"],
            ["ID pengguna", e.user_id],
            ["Peran saat itu", labelPeran(e.peran, e.kode_peran)],
            ["Aktivitas", e.label],
            ["Hasil", `${LABEL_HASIL[kelasHasil(e)]} (HTTP ${e.status_http})`],
            ["Permintaan", `${e.metode} ${e.rute}`],
            ["Objek", objek],
            ["Alamat IP", e.ip],
            ["Perangkat", e.user_agent ? `${ringkasUserAgent(e.user_agent)}` : undefined],
            ["User agent lengkap", e.user_agent],
            ["Lama proses", `${e.durasi_ms} ms`],
            ["ID permintaan", e.request_id],
          ]}
        />

        {rincian.length > 0 && (
          <section>
            <h4 className="text-xs font-semibold uppercase tracking-wide text-slate-500">Rincian tambahan</h4>
            <dl className="mt-2 divide-y divide-slate-100 rounded-xl border border-slate-200 text-sm">
              {rincian.map(([k, v]) => (
                <div key={k} className="grid grid-cols-[minmax(0,11rem)_minmax(0,1fr)] gap-3 px-3 py-2">
                  <dt className="text-slate-500">{k}</dt>
                  <dd className="break-words font-medium text-slate-900">{v}</dd>
                </div>
              ))}
            </dl>
          </section>
        )}

        <p className="rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-500">
          Isi permintaan, kata sandi, dan token tidak pernah disimpan di log audit. Entri ini tidak dapat diubah atau dihapus dari aplikasi; hanya dibersihkan otomatis setelah masa simpan berakhir.
        </p>

        {e.user_id && (
          <div className="flex justify-end">
            <button
              type="button"
              onClick={() => onLihatPengguna(e.user_id as string, e.username ?? "")}
              className="inline-flex items-center gap-1.5 rounded-lg px-3 py-2 text-sm font-medium text-blue-700 hover:bg-blue-50"
            >
              <Search className="h-4 w-4" aria-hidden="true" />
              Lihat semua aktivitas pengguna ini
            </button>
          </div>
        )}
      </div>
    </ModalShell>
  );
}
