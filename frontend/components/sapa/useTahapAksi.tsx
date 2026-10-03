"use client";

import { useCallback, useState } from "react";
import { FileCheck2, Save } from "lucide-react";
import { generateSapaDokumen, saveSapaDraf } from "@/lib/sapa";
import { useToast } from "@/components/ui/Toast";
import { ErrorBox, NoticeBox, PrimaryButton, SecondaryButton } from "./fields";
import { ErrorInfo, errorInfo } from "./sapa";

export interface AksiState {
  busy: "draf" | "dokumen" | null;
  error: ErrorInfo | null;
  notice: string | null;
  peringatan: string[];
  simpan: (data: unknown) => Promise<void>;
  buat: (data: unknown) => Promise<void>;
}

// Aksi tahap berformulir: simpan draf dan buat dokumen. Keberhasilan memanggil onChanged (induk memuat ulang detail);
// kegagalan ditampilkan di bawah formulir bersama rincian validasinya.
export function useTahapAksi(usulanId: number, kunci: string, onChanged: () => void): AksiState {
  const [busy, setBusy] = useState<AksiState["busy"]>(null);
  const [error, setError] = useState<ErrorInfo | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [peringatan, setPeringatan] = useState<string[]>([]);
  const toast = useToast();

  const jalankan = useCallback(
    async (jenis: "draf" | "dokumen", fn: () => Promise<void>, galat: string) => {
      setBusy(jenis);
      setError(null);
      setNotice(null);
      setPeringatan([]);
      try {
        await fn();
      } catch (err) {
        setError(errorInfo(err, galat));
      } finally {
        setBusy(null);
      }
    },
    []
  );

  const simpan = useCallback(
    (data: unknown) =>
      jalankan(
        "draf",
        async () => {
          await saveSapaDraf(usulanId, kunci, data);
          setNotice("Draf tersimpan.");
          toast.success("Draf tersimpan.");
          onChanged();
        },
        "Gagal menyimpan draf"
      ),
    [jalankan, usulanId, kunci, onChanged, toast]
  );

  const buat = useCallback(
    (data: unknown) =>
      jalankan(
        "dokumen",
        async () => {
          const res = await generateSapaDokumen(usulanId, kunci, data);
          setNotice("Dokumen berhasil dibuat. Unduh dari daftar dokumen hasil di bawah.");
          // Formulir tertutup setelah tahap selesai, jadi pesan di dalam formulir tidak terlihat; toast tetap tampil.
          toast.success("Dokumen Word siap diunduh dari daftar dokumen hasil.", "Dokumen berhasil dibuat");
          setPeringatan(res.data.peringatan ?? []);
          onChanged();
        },
        "Gagal membuat dokumen"
      ),
    [jalankan, usulanId, kunci, onChanged, toast]
  );

  return { busy, error, notice, peringatan, simpan, buat };
}

// Tombol dan pesan di bawah setiap formulir tahap.
export function FormFooter({
  aksi,
  onSimpan,
  onBuat,
  templateTersedia,
  labelBuat = "Buat dokumen Word",
}: {
  aksi: AksiState;
  onSimpan: () => void;
  onBuat: () => void;
  templateTersedia: boolean;
  labelBuat?: string;
}) {
  return (
    <div className="space-y-3 border-t border-slate-100 pt-4">
      {!templateTersedia && (
        <NoticeBox tone="warn">
          Template dokumen untuk tahap ini belum tersedia. Isian tetap bisa disimpan sebagai draf; minta admin mengunggah template di Pengaturan SAPA
          sebelum membuat dokumen.
        </NoticeBox>
      )}
      <ErrorBox error={aksi.error} />
      {aksi.peringatan.length > 0 && (
        <NoticeBox tone="warn">
          <p className="font-medium">Dokumen dibuat dengan catatan:</p>
          <ul className="mt-1 list-disc space-y-0.5 pl-4 text-xs">
            {aksi.peringatan.map((p, i) => (
              <li key={i}>{p}</li>
            ))}
          </ul>
        </NoticeBox>
      )}
      <div className="flex flex-col gap-2 sm:flex-row sm:justify-end">
        <SecondaryButton onClick={onSimpan} busy={aksi.busy === "draf"} disabled={aksi.busy !== null}>
          <Save className="h-4 w-4" aria-hidden="true" />
          Simpan draf
        </SecondaryButton>
        <PrimaryButton onClick={onBuat} busy={aksi.busy === "dokumen"} disabled={aksi.busy !== null || !templateTersedia}>
          <FileCheck2 className="h-4 w-4" aria-hidden="true" />
          {labelBuat}
        </PrimaryButton>
      </div>
    </div>
  );
}
