"use client";

import { useMemo, useState } from "react";
import axios from "axios";
import { Info, Loader2 } from "lucide-react";
import { SLDKFlagKey, updateSLDKOverviewSettings } from "@/lib/api";
import { Alert } from "@/components/ui/Alert";
import { ModalShell } from "@/components/ui/ModalShell";
import { formatNumber } from "./asset";
import { flagKeyLabel, pct, share } from "./overview";

interface Props {
  flagKeys: SLDKFlagKey[];
  active: string[];
  onClose: () => void;
  onSaved: () => void;
}

// Admin memilih kombinasi penanda (status_data, sts_his, sts_ast, tgl_hapus) yang dihitung sebagai "aset aktif".
// Arti kode-kodenya belum dikonfirmasi di sistem ini, jadi keputusannya ada pada orang yang memahami datanya.
export function OverviewSettingsModal({ flagKeys, active, onClose, onSaved }: Props) {
  const [selected, setSelected] = useState<Set<string>>(() => new Set(active));
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState("");

  const total = useMemo(() => flagKeys.reduce((acc, k) => acc + k.jumlah, 0), [flagKeys]);
  const counted = useMemo(
    () => (selected.size === 0 ? total : flagKeys.filter((k) => selected.has(k.key)).reduce((acc, k) => acc + k.jumlah, 0)),
    [flagKeys, selected, total]
  );
  const dirty = selected.size !== active.length || active.some((k) => !selected.has(k));

  const toggle = (key: string) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const save = async () => {
    setIsSaving(true);
    setError("");
    try {
      await updateSLDKOverviewSettings(flagKeys.filter((k) => selected.has(k.key)).map((k) => k.key));
      onSaved();
      onClose();
    } catch (err) {
      setError(axios.isAxiosError(err) && err.response ? err.response.data?.message || "Gagal menyimpan pengaturan" : "Tidak dapat terhubung ke server");
      setIsSaving(false);
    }
  };

  return (
    <ModalShell title="Definisi aset aktif" subtitle="Menentukan baris mana di tabel aset SLDK yang dihitung pada Ringkasan dan Pemantauan" onClose={onClose}>
      <div className="space-y-4 px-4 py-4 sm:px-6">
        <div className="flex items-start gap-2 rounded-lg bg-slate-50 px-3.5 py-2.5 text-xs text-slate-600">
          <Info className="mt-0.5 h-4 w-4 shrink-0 text-slate-400" />
          <span>
            Tabel aset SLDK memuat baris riwayat dan baris yang sudah dihapus di samping aset yang masih tercatat. Pilih kombinasi penanda yang
            menurut Anda berarti &ldquo;aset aktif&rdquo;. Tanpa pilihan, semua baris dihitung. Perubahan berlaku segera, tanpa sinkronisasi ulang.
          </span>
        </div>

        <fieldset>
          <legend className="sr-only">Kombinasi penanda yang dihitung sebagai aset aktif</legend>
          <ul className="divide-y divide-slate-100 rounded-lg border border-slate-200">
            {flagKeys.map((k) => (
              <li key={k.key}>
                <label className="flex cursor-pointer items-start gap-3 px-3.5 py-2.5 hover:bg-slate-50">
                  <input
                    type="checkbox"
                    checked={selected.has(k.key)}
                    onChange={() => toggle(k.key)}
                    className="mt-0.5 h-4 w-4 shrink-0 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
                  />
                  <span className="min-w-0 flex-1 break-words text-sm text-slate-700">{flagKeyLabel(k.key)}</span>
                  <span className="shrink-0 text-right text-sm">
                    <span className="font-medium text-slate-900">{formatNumber(k.jumlah, 0)}</span>{" "}
                    <span className="ml-0.5 text-xs text-slate-400">{pct(share(k.jumlah, total))}</span>
                  </span>
                </label>
              </li>
            ))}
          </ul>
        </fieldset>

        <p className="text-sm text-slate-600">
          {selected.size === 0 ? "Tidak ada yang dipilih: semua baris dihitung. " : ""}
          Dihitung sebagai aset: <span className="font-semibold text-slate-900">{formatNumber(counted, 0)}</span> dari {formatNumber(total, 0)} baris
          ({pct(share(counted, total))}).
        </p>

        {error && <Alert message={error} />}

        <div className="flex justify-end gap-2 border-t border-slate-100 pt-4">
          <button type="button" onClick={onClose} className="rounded-lg px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100">
            Batal
          </button>
          <button
            type="button"
            onClick={save}
            disabled={!dirty || isSaving}
            className="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60"
          >
            {isSaving && <Loader2 className="h-4 w-4 animate-spin" />}
            Simpan
          </button>
        </div>
      </div>
    </ModalShell>
  );
}
