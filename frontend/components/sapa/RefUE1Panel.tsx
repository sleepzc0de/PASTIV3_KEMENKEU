"use client";

import { useCallback, useEffect, useState } from "react";
import { Pencil, Trash2 } from "lucide-react";
import { SapaRefUE1, deleteSapaRefUE1, listSapaRefUE1, saveSapaRefUE1 } from "@/lib/sapa";
import { Alert } from "@/components/ui/Alert";
import { ErrorBox, NoticeBox, PrimaryButton, SecondaryButton, TextField } from "./fields";
import { ErrorInfo, errorInfo } from "./sapa";

const KOSONG: SapaRefUE1 = { kode: "", nama: "", sekretaris: "" };

// Referensi Unit Eselon I: kode 5 digit (lima digit pertama kode satker) -> nama dan sebutan Sekretaris. Dipakai mengisi
// tujuan Nota Dinas usulan Satker. Tidak ada data awal supaya tidak ada nama yang salah; admin mengisinya.
export function RefUE1Panel() {
  const [list, setList] = useState<SapaRefUE1[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<SapaRefUE1>(KOSONG);
  const [mengedit, setMengedit] = useState(false);
  const [busy, setBusy] = useState(false);
  const [galat, setGalat] = useState<ErrorInfo | null>(null);
  const [hapus, setHapus] = useState<string | null>(null);

  const muat = useCallback(async () => {
    try {
      const res = await listSapaRefUE1();
      setList(res.data);
      setError(null);
    } catch (err) {
      setError(errorInfo(err, "Gagal memuat referensi UE1").message);
    }
  }, []);

  useEffect(() => {
    muat();
  }, [muat]);

  const simpan = async () => {
    setBusy(true);
    setGalat(null);
    try {
      await saveSapaRefUE1({ kode: form.kode.trim(), nama: form.nama.trim(), sekretaris: form.sekretaris.trim() });
      setForm(KOSONG);
      setMengedit(false);
      await muat();
    } catch (err) {
      setGalat(errorInfo(err, "Gagal menyimpan"));
    } finally {
      setBusy(false);
    }
  };

  const konfirmasiHapus = async (kode: string) => {
    setBusy(true);
    setGalat(null);
    try {
      await deleteSapaRefUE1(kode);
      setHapus(null);
      await muat();
    } catch (err) {
      setGalat(errorInfo(err, "Gagal menghapus"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-5">
      <NoticeBox>
        Kode UE1 adalah <b>5 digit pertama</b> kode satker. Sebutan sekretaris ditulis lengkap seperti pada surat, mis. &ldquo;Sekretaris Direktorat Jenderal …&rdquo;.
      </NoticeBox>

      <section className="space-y-3 rounded-xl border border-slate-200 bg-white p-4">
        <h3 className="text-sm font-semibold text-slate-900">{mengedit ? `Ubah UE1 ${form.kode}` : "Tambah Unit Eselon I"}</h3>
        <div className="grid gap-3 sm:grid-cols-2">
          <TextField
            label="Kode UE1 (5 digit)"
            required
            value={form.kode}
            onChange={(v) => setForm((f) => ({ ...f, kode: v.replace(/\D/g, "").slice(0, 5) }))}
            inputMode="numeric"
            disabled={mengedit}
          />
          <TextField label="Nama Unit Eselon I" required value={form.nama} onChange={(v) => setForm((f) => ({ ...f, nama: v }))} maxLength={200} />
          <TextField
            label="Sebutan Sekretaris"
            required
            value={form.sekretaris}
            onChange={(v) => setForm((f) => ({ ...f, sekretaris: v }))}
            maxLength={300}
            className="sm:col-span-2"
          />
        </div>
        <ErrorBox error={galat} />
        <div className="flex justify-end gap-2">
          {mengedit && (
            <SecondaryButton
              onClick={() => {
                setForm(KOSONG);
                setMengedit(false);
                setGalat(null);
              }}
            >
              Batal
            </SecondaryButton>
          )}
          <PrimaryButton onClick={simpan} busy={busy} disabled={form.kode.length !== 5 || !form.nama.trim() || !form.sekretaris.trim()}>
            {mengedit ? "Simpan perubahan" : "Tambah"}
          </PrimaryButton>
        </div>
      </section>

      {error && <Alert message={error} />}
      {!list && !error && <div role="status" aria-label="Memuat referensi" className="h-32 animate-pulse rounded-xl bg-slate-100" />}
      {list && list.length === 0 && (
        <p className="rounded-xl border border-dashed border-slate-300 px-4 py-8 text-center text-sm text-slate-500">
          Belum ada referensi. Tanpa referensi, tujuan Nota Dinas harus diketik manual oleh pengguna.
        </p>
      )}
      {list && list.length > 0 && (
        <ul className="divide-y divide-slate-100 rounded-xl border border-slate-200 bg-white">
          {list.map((r) => (
            <li key={r.kode} className="flex flex-wrap items-center justify-between gap-3 p-3.5">
              <div className="min-w-0">
                <p className="text-sm font-semibold text-slate-900">
                  <span className="font-mono text-slate-500">{r.kode}</span> · {r.nama}
                </p>
                <p className="break-words text-xs text-slate-600">{r.sekretaris}</p>
              </div>
              {hapus === r.kode ? (
                <div className="flex items-center gap-2 text-sm">
                  <span className="text-slate-600">Hapus?</span>
                  <SecondaryButton onClick={() => setHapus(null)}>Batal</SecondaryButton>
                  <PrimaryButton onClick={() => konfirmasiHapus(r.kode)} busy={busy} className="!bg-red-600 hover:!bg-red-700">
                    Hapus
                  </PrimaryButton>
                </div>
              ) : (
                <div className="flex gap-1">
                  <button
                    type="button"
                    onClick={() => {
                      setForm(r);
                      setMengedit(true);
                      setGalat(null);
                    }}
                    aria-label={`Ubah UE1 ${r.kode}`}
                    className="rounded p-2 text-slate-400 hover:bg-slate-100 hover:text-slate-700"
                  >
                    <Pencil className="h-4 w-4" aria-hidden="true" />
                  </button>
                  <button
                    type="button"
                    onClick={() => setHapus(r.kode)}
                    aria-label={`Hapus UE1 ${r.kode}`}
                    className="rounded p-2 text-slate-400 hover:bg-red-50 hover:text-red-600"
                  >
                    <Trash2 className="h-4 w-4" aria-hidden="true" />
                  </button>
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
