"use client";

import { useEffect, useState } from "react";
import axios from "axios";
import { Copy } from "lucide-react";
import { type BarisLog, getServerLogDetail } from "@/lib/logServer";
import { waktuWIB } from "@/lib/auditFilter";
import { LABEL_LEVEL, teksJumlah } from "@/lib/logServerFilter";
import { Alert } from "@/components/ui/Alert";
import { DetailEntry } from "@/components/ui/DetailEntry";
import { ModalShell } from "@/components/ui/ModalShell";
import { Skeleton } from "@/components/ui/Skeleton";
import { useToast } from "@/components/ui/Toast";
import { LencanaJumlah, LencanaLevel } from "./lencana";

// Rincian lengkap satu baris log, termasuk jejak tumpukan (hanya dibaca saat dibuka, supaya daftar tetap ringan).
export function LogDetail({ baris, onTutup }: { baris: BarisLog; onTutup: () => void }) {
  const toast = useToast();
  const [penuh, setPenuh] = useState<BarisLog | null>(null);
  const [galat, setGalat] = useState("");

  useEffect(() => {
    const ac = new AbortController();
    setPenuh(null);
    setGalat("");
    getServerLogDetail(baris.id, ac.signal)
      .then((r) => setPenuh(r.data))
      .catch((err) => {
        if (axios.isCancel(err)) return;
        setGalat(axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : "Gagal memuat rincian log");
      });
    return () => ac.abort();
  }, [baris.id]);

  const b = penuh ?? baris;
  const salin = async () => {
    const teks = [
      `[${LABEL_LEVEL[b.level]}] ${b.sumber}`,
      b.pesan,
      `Terakhir: ${b.waktu_akhir} (UTC)`,
      b.rute ? `Permintaan: ${b.metode ?? ""} ${b.rute} ${b.status ?? ""}` : "",
      b.request_id ? `ID permintaan: ${b.request_id}` : "",
      b.detail ?? "",
    ]
      .filter(Boolean)
      .join("\n");
    try {
      await navigator.clipboard.writeText(teks);
      toast.success("Rincian log disalin");
    } catch {
      toast.error("Tidak bisa menyalin otomatis; blok teksnya lalu salin manual");
    }
  };

  return (
    <ModalShell title="Rincian log server" subtitle={b.sumber} onClose={onTutup} size="lg">
      <div className="space-y-4">
        <div className="flex flex-wrap items-center gap-2">
          <LencanaLevel level={b.level} />
          <LencanaJumlah teks={teksJumlah(b.jumlah)} />
          {b.jumlah > 1 && <span className="text-xs text-slate-500">kejadian yang sama digabung menjadi satu baris</span>}
        </div>

        <p className="break-words rounded-xl bg-slate-50 px-3.5 py-3 text-sm font-medium text-slate-900">{b.pesan}</p>

        <DetailEntry
          fields={[
            ["Sumber", b.sumber],
            ["Kejadian pertama (WIB)", waktuWIB(b.waktu)],
            ["Kejadian terakhir (WIB)", waktuWIB(b.waktu_akhir)],
            ["Jumlah kejadian", b.jumlah > 1 ? b.jumlah.toLocaleString("id-ID") : "1"],
            ["Permintaan", b.rute ? `${b.metode ?? ""} ${b.rute}`.trim() : undefined],
            ["Status HTTP", b.status ? String(b.status) : undefined],
            ["Lama proses", b.durasi_ms ? `${b.durasi_ms.toLocaleString("id-ID")} ms` : undefined],
            ["ID permintaan (contoh terbaru)", b.request_id],
            ["ID pengguna (contoh terbaru)", b.user_id],
          ]}
        />

        {galat && <Alert message={galat} />}
        {!penuh && !galat && <Skeleton className="h-16 w-full" />}
        {penuh?.detail && (
          <section>
            <div className="flex items-center justify-between gap-3">
              <h4 className="text-xs font-semibold uppercase tracking-wide text-slate-500">Rincian teknis</h4>
              <button type="button" onClick={() => void salin()} className="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium text-blue-700 hover:bg-blue-50">
                <Copy className="h-3.5 w-3.5" aria-hidden="true" />
                Salin
              </button>
            </div>
            <pre className="mt-2 max-h-72 overflow-auto rounded-xl bg-slate-900 p-3 text-[11px] leading-relaxed text-slate-100">{penuh.detail}</pre>
          </section>
        )}
        {penuh && !penuh.detail && (
          <div className="flex justify-end">
            <button type="button" onClick={() => void salin()} className="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium text-blue-700 hover:bg-blue-50">
              <Copy className="h-3.5 w-3.5" aria-hidden="true" />
              Salin ringkasan
            </button>
          </div>
        )}

        <p className="rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-500">
          Kata sandi, token, dan rahasia lain disamarkan sebelum log disimpan; isi permintaan tidak disimpan. Baris ini tidak dapat diubah atau dihapus dari aplikasi; hanya dibersihkan otomatis setelah masa simpan berakhir.
        </p>
      </div>
    </ModalShell>
  );
}
