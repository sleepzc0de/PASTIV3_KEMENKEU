"use client";

import { useState } from "react";
import { CheckCircle2 } from "lucide-react";
import { SapaTahapDetail, completeSapaTahap } from "@/lib/sapa";
import { ErrorBox, NoticeBox, PrimaryButton, TextAreaField, TextField } from "./fields";
import { ErrorInfo, errorInfo } from "./sapa";

// Tahap yang dikerjakan di aplikasi lain (Nadine, SIMAN): aplikasi hanya mencatat bahwa tahap itu sudah selesai,
// beserta nomor dan tanggal dokumen bila ada.
export function EksternalPanel({ usulanId, tahap, sudahSelesai, onChanged }: { usulanId: string; tahap: SapaTahapDetail; sudahSelesai: boolean; onChanged: () => void }) {
  const [nomor, setNomor] = useState(tahap.nomor ?? "");
  const [tanggal, setTanggal] = useState(tahap.tanggal ?? "");
  const [catatan, setCatatan] = useState(tahap.catatan ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ErrorInfo | null>(null);
  const wajib = tahap.wajib_nomor_tanggal === true;

  const kirim = async () => {
    setBusy(true);
    setError(null);
    try {
      await completeSapaTahap(usulanId, tahap.kunci, { nomor, tanggal, catatan });
      onChanged();
    } catch (err) {
      setError(errorInfo(err, "Gagal menyimpan"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-3">
      <NoticeBox>
        Tahap ini dikerjakan di <span className="font-semibold">{tahap.kanal}</span>. Setelah selesai di sana, catat hasilnya di sini agar tahap berikutnya dapat dilanjutkan.
      </NoticeBox>
      <div className="grid gap-3 sm:grid-cols-2">
        <TextField
          label={wajib ? "Nomor dokumen" : "Nomor dokumen (opsional)"}
          required={wajib}
          value={nomor}
          onChange={setNomor}
          maxLength={150}
          hint={wajib ? "Nomor Nota Dinas dari Nadine; dipakai pada tahap berikutnya." : undefined}
        />
        <TextField label={wajib ? "Tanggal dokumen" : "Tanggal (opsional)"} required={wajib} type="date" value={tanggal} onChange={setTanggal} />
        <TextAreaField label="Catatan (opsional)" value={catatan} onChange={setCatatan} maxLength={1000} rows={2} className="sm:col-span-2" />
      </div>
      <ErrorBox error={error} />
      <div className="flex justify-end">
        <PrimaryButton onClick={kirim} busy={busy}>
          <CheckCircle2 className="h-4 w-4" aria-hidden="true" />
          {sudahSelesai ? "Perbarui catatan" : "Tandai selesai"}
        </PrimaryButton>
      </div>
    </div>
  );
}
