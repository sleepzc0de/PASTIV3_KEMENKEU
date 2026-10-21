"use client";

import { useState } from "react";
import { Check, Copy, KeyRound, TriangleAlert } from "lucide-react";
import { contohCurl } from "@/lib/apiAksesFormat";
import { Button } from "@/components/ui/Button";
import { ModalShell } from "@/components/ui/ModalShell";
import { useToast } from "@/components/ui/Toast";

async function salin(teks: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(teks);
    return true;
  } catch {
    // Cadangan untuk peramban/konteks tanpa Clipboard API (mis. HTTP biasa).
    try {
      const ta = document.createElement("textarea");
      ta.value = teks;
      ta.style.position = "fixed";
      ta.style.opacity = "0";
      document.body.appendChild(ta);
      ta.select();
      const ok = document.execCommand("copy");
      document.body.removeChild(ta);
      return ok;
    } catch {
      return false;
    }
  }
}

function alamatDasar(): string {
  const akar = process.env.NEXT_PUBLIC_API_ROOT_URL || (typeof window !== "undefined" ? window.location.origin : "");
  return `${akar.replace(/\/$/, "")}/api/data/v1`;
}

// Menampilkan kunci API yang baru diterbitkan. Kunci hanya ada di layar ini: server hanya menyimpan hash-nya, jadi tidak dapat ditampilkan lagi. Modal tidak ditutup
// diam-diam (Escape atau klik di luar) sebelum pengguna menyatakan sudah menyimpannya.
export function KunciBaruModal({ kunci, judul, onTutup }: { kunci: string; judul: string; onTutup: () => void }) {
  const toast = useToast();
  const [disalin, setDisalin] = useState(false);
  const [sudah, setSudah] = useState(false);
  const [peringatan, setPeringatan] = useState(false);

  const tutup = () => {
    if (!sudah) {
      setPeringatan(true);
      return;
    }
    onTutup();
  };

  const salinKunci = async () => {
    if (await salin(kunci)) {
      setDisalin(true);
      toast.success("Kunci API disalin ke papan klip");
    } else {
      toast.error("Tidak dapat menyalin otomatis. Pilih teks kunci lalu salin manual.");
    }
  };

  return (
    <ModalShell title={judul} subtitle="Kunci ini hanya ditampilkan sekali" onClose={tutup} size="md">
      <div className="space-y-4 p-4 sm:p-6">
        <div className="flex items-start gap-2.5 rounded-xl border border-amber-200 bg-amber-50 px-3.5 py-3 text-sm text-amber-900">
          <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <p>
            PASTI hanya menyimpan <b>hash</b> kunci, jadi kunci ini <b>tidak dapat dilihat lagi</b> setelah jendela ini ditutup. Salin sekarang dan serahkan kepada PIC lewat jalur yang aman (bukan email biasa atau
            grup obrolan). Bila hilang, terbitkan kunci baru dan cabut yang lama.
          </p>
        </div>

        <div>
          <p className="mb-1 flex items-center gap-1.5 text-xs font-medium text-slate-600">
            <KeyRound className="h-3.5 w-3.5" aria-hidden="true" />
            Kunci API
          </p>
          <div className="flex items-stretch gap-2">
            <input
              readOnly
              value={kunci}
              onFocus={(e) => e.currentTarget.select()}
              aria-label="Kunci API"
              className="min-w-0 flex-1 rounded-xl border border-slate-300 bg-slate-50 px-3 py-2.5 font-mono text-xs text-slate-900 outline-none focus:border-blue-500 focus:shadow-glow"
            />
            <Button fullWidth={false} variant={disalin ? "soft" : "primary"} icon={disalin ? <Check className="h-4 w-4" aria-hidden="true" /> : <Copy className="h-4 w-4" aria-hidden="true" />} onClick={() => void salinKunci()}>
              {disalin ? "Tersalin" : "Salin"}
            </Button>
          </div>
        </div>

        <div>
          <p className="mb-1 text-xs font-medium text-slate-600">Contoh pemakaian</p>
          <pre className="overflow-x-auto rounded-xl bg-slate-900 px-3.5 py-3 text-xs leading-relaxed text-slate-100">{contohCurl(alamatDasar(), kunci)}</pre>
          <p className="mt-1.5 text-xs text-slate-500">
            Kunci dikirim lewat header <span className="font-mono">Authorization: Bearer</span> atau <span className="font-mono">X-API-Key</span>, tidak pernah lewat alamat (URL). Panduan lengkap ada di docs/api-akses-data-pemakaian.md.
          </p>
        </div>

        <label className="flex cursor-pointer items-start gap-2.5 rounded-xl border border-slate-200 bg-white p-3 text-sm text-slate-700">
          <input type="checkbox" checked={sudah} onChange={(e) => setSudah(e.target.checked)} className="mt-0.5 h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" />
          Saya sudah menyalin dan menyimpan kunci ini dengan aman.
        </label>
        {peringatan && !sudah && (
          <p role="alert" className="text-xs font-medium text-red-600">
            Salin kunci lalu centang pernyataan di atas sebelum menutup. Kunci tidak dapat ditampilkan lagi.
          </p>
        )}

        <div className="flex justify-end">
          <Button fullWidth={false} disabled={!sudah} onClick={onTutup}>
            Selesai
          </Button>
        </div>
      </div>
    </ModalShell>
  );
}
