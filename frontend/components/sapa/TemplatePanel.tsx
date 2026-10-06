"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ChevronDown, FileDown, History, Upload } from "lucide-react";
import { SapaHasilUnggah, SapaStatusTemplate, listSapaTemplate, unduhSapaTemplate, uploadSapaTemplate } from "@/lib/sapa";
import { Alert } from "@/components/ui/Alert";
import { formatDateTime } from "@/lib/dasbor";
import { simpanBlob } from "./download";
import { ErrorBox, NoticeBox, PrimaryButton, SecondaryButton, TextField } from "./fields";
import { ErrorInfo, errorInfo, formatUkuran } from "./sapa";

const SUMBER: Record<SapaStatusTemplate["sumber"], { label: string; cls: string }> = {
  unggahan: { label: "Unggahan admin", cls: "bg-emerald-50 text-emerald-700" },
  bawaan: { label: "Template bawaan aplikasi", cls: "bg-blue-50 text-blue-700" },
  belum: { label: "Belum ada template", cls: "bg-amber-50 text-amber-700" },
};

// Template Word per jenis dokumen. Admin mengunggah template versi baru; aplikasi menguji pengisiannya dengan data contoh
// sebelum menyimpan, dan melaporkan penanda <<...>> yang tidak dikenal atau tidak terpakai.
export function TemplatePanel() {
  const [list, setList] = useState<SapaStatusTemplate[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const muat = useCallback(async () => {
    try {
      const res = await listSapaTemplate();
      setList(res.data);
      setError(null);
    } catch (err) {
      setError(errorInfo(err, "Gagal memuat daftar template").message);
    }
  }, []);

  useEffect(() => {
    muat();
  }, [muat]);

  if (error && !list) {
    return (
      <div className="space-y-2">
        <Alert message={error} />
        <button type="button" onClick={muat} className="text-sm font-medium text-blue-600 hover:text-blue-700">
          Coba lagi
        </button>
      </div>
    );
  }
  if (!list) return <div role="status" aria-label="Memuat template" className="h-40 animate-pulse rounded-xl bg-slate-100" />;

  return (
    <div className="space-y-4">
      <NoticeBox>
        Tulis penanda pada dokumen Word dengan tanda kurung sudut ganda, mis. <code className="rounded bg-white/70 px-1">&lt;&lt;nama satker&gt;&gt;</code>. Aplikasi mengganti penanda
        itu dengan isian pengguna dan mempertahankan format dokumen. Daftar penanda yang dikenali ada di setiap kartu di bawah.
      </NoticeBox>
      {list.map((s) => (
        <KartuTemplate key={s.jenis.kunci} s={s} onChanged={muat} />
      ))}
    </div>
  );
}

function KartuTemplate({ s, onChanged }: { s: SapaStatusTemplate; onChanged: () => void }) {
  const [penanda, setPenanda] = useState(false);
  const [riwayat, setRiwayat] = useState(false);
  const [unggah, setUnggah] = useState(false);
  const [hasil, setHasil] = useState<SapaHasilUnggah | null>(null);
  const [galatUnduh, setGalatUnduh] = useState<ErrorInfo | null>(null);
  const [unduhBusy, setUnduhBusy] = useState(false);
  const sumber = SUMBER[s.sumber];

  const unduh = async () => {
    setUnduhBusy(true);
    setGalatUnduh(null);
    try {
      const { blob, disposition } = await unduhSapaTemplate(s.jenis.kunci);
      simpanBlob(blob, disposition, `Template ${s.jenis.label}.docx`);
    } catch (err) {
      setGalatUnduh(errorInfo(err, "Gagal mengunduh template"));
    } finally {
      setUnduhBusy(false);
    }
  };

  return (
    <section className="rounded-xl border border-slate-200 bg-white">
      <div className="flex flex-wrap items-start justify-between gap-3 p-4">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-slate-900">{s.jenis.label}</h3>
          <div className="mt-1.5 flex flex-wrap items-center gap-2 text-xs">
            <span className={`rounded-full px-2 py-0.5 font-medium ${sumber.cls}`}>{sumber.label}</span>
            {s.aktif && (
              <span className="text-slate-500">
                Versi {s.aktif.versi} · {s.aktif.nama_file} · {formatUkuran(s.aktif.ukuran)} · {s.aktif.diunggah_oleh || "-"}, {formatDateTime(s.aktif.diunggah_pada)}
              </span>
            )}
          </div>
          {s.aktif?.catatan && <p className="mt-1 text-xs text-slate-500">Catatan: {s.aktif.catatan}</p>}
        </div>
        <div className="flex flex-wrap gap-2">
          {s.sumber !== "belum" && (
            <SecondaryButton onClick={unduh} busy={unduhBusy}>
              <FileDown className="h-4 w-4" aria-hidden="true" />
              Unduh
            </SecondaryButton>
          )}
          <PrimaryButton onClick={() => setUnggah((v) => !v)}>
            <Upload className="h-4 w-4" aria-hidden="true" />
            {s.sumber === "belum" ? "Unggah template" : "Unggah versi baru"}
          </PrimaryButton>
        </div>
      </div>

      <div className="space-y-3 px-4 pb-4">
        <ErrorBox error={galatUnduh} />
        {s.jenis.catatan && <p className="text-xs text-slate-500">{s.jenis.catatan}</p>}
        {unggah && (
          <FormUnggah
            kunci={s.jenis.kunci}
            onTutup={() => setUnggah(false)}
            onBerhasil={(h) => {
              setHasil(h);
              setUnggah(false);
              onChanged();
            }}
          />
        )}
        {hasil && <HasilUnggahan hasil={hasil} onTutup={() => setHasil(null)} />}

        <div className="flex flex-wrap gap-4 border-t border-slate-100 pt-3 text-sm">
          <button type="button" onClick={() => setPenanda((v) => !v)} aria-expanded={penanda} className="inline-flex items-center gap-1 font-medium text-blue-600 hover:text-blue-700">
            <ChevronDown className={`h-4 w-4 transition-transform ${penanda ? "rotate-180" : ""}`} aria-hidden="true" />
            Daftar penanda ({s.jenis.penanda.length})
          </button>
          {s.riwayat.length > 0 && (
            <button type="button" onClick={() => setRiwayat((v) => !v)} aria-expanded={riwayat} className="inline-flex items-center gap-1 font-medium text-slate-600 hover:text-slate-800">
              <History className="h-4 w-4" aria-hidden="true" />
              Riwayat versi ({s.riwayat.length})
            </button>
          )}
        </div>

        {penanda && (
          <div className="overflow-x-auto rounded-lg border border-slate-200">
            <table className="w-full min-w-[32rem] text-left text-xs">
              <thead className="bg-slate-50 text-slate-500">
                <tr>
                  <th scope="col" className="px-3 py-2 font-medium">
                    Penanda
                  </th>
                  <th scope="col" className="px-3 py-2 font-medium">
                    Keterangan
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {s.jenis.penanda.map((p) => (
                  <tr key={p.nama}>
                    <td className="whitespace-nowrap px-3 py-2 font-mono text-slate-800">
                      &lt;&lt;{p.nama}&gt;&gt;
                      {p.ulang && <span className="ml-1.5 rounded bg-slate-100 px-1 font-sans text-[10px] text-slate-500">baris berulang</span>}
                    </td>
                    <td className="px-3 py-2 text-slate-600">{p.keterangan}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {riwayat && (
          <ul className="divide-y divide-slate-100 rounded-lg border border-slate-200 text-xs">
            {s.riwayat.map((r) => (
              <li key={r.id} className="flex flex-wrap items-center justify-between gap-2 px-3 py-2">
                <span className="text-slate-700">
                  Versi {r.versi} · {r.nama_file} · {formatUkuran(r.ukuran)}
                </span>
                <span className="text-slate-500">
                  {r.diunggah_oleh || "-"}, {formatDateTime(r.diunggah_pada)} {r.aktif && <span className="ml-1 rounded-full bg-emerald-50 px-1.5 py-0.5 font-medium text-emerald-700">aktif</span>}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}

const MAKS_BYTE = 5 * 1024 * 1024;

function FormUnggah({ kunci, onTutup, onBerhasil }: { kunci: string; onTutup: () => void; onBerhasil: (h: SapaHasilUnggah) => void }) {
  const [berkas, setBerkas] = useState<File | null>(null);
  const [catatan, setCatatan] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ErrorInfo | null>(null);
  const input = useRef<HTMLInputElement>(null);

  const pilih = (f: File | null) => {
    setError(null);
    if (f && !f.name.toLowerCase().endsWith(".docx")) {
      setError({ message: "Berkas harus berformat .docx (Word)", errors: [] });
      setBerkas(null);
      if (input.current) input.current.value = "";
      return;
    }
    if (f && f.size > MAKS_BYTE) {
      setError({ message: `Berkas terlalu besar (maksimal ${MAKS_BYTE / 1024 / 1024} MB)`, errors: [] });
      setBerkas(null);
      if (input.current) input.current.value = "";
      return;
    }
    setBerkas(f);
  };

  const kirim = async () => {
    if (!berkas) return;
    setBusy(true);
    setError(null);
    try {
      const res = await uploadSapaTemplate(kunci, berkas, catatan);
      onBerhasil(res.data);
    } catch (err) {
      setError(errorInfo(err, "Gagal mengunggah template"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-3 rounded-lg border border-blue-200 bg-blue-50/40 p-3">
      <div>
        <label htmlFor={`berkas-${kunci}`} className="mb-1.5 block text-xs font-medium text-slate-600">
          Berkas template (.docx, maksimal 5 MB)
        </label>
        <input
          id={`berkas-${kunci}`}
          ref={input}
          type="file"
          accept=".docx,application/vnd.openxmlformats-officedocument.wordprocessingml.document"
          onChange={(e) => pilih(e.target.files?.[0] ?? null)}
          className="block w-full text-sm text-slate-600 file:mr-3 file:rounded-lg file:border-0 file:bg-blue-600 file:px-3 file:py-2 file:text-sm file:font-medium file:text-white hover:file:bg-blue-700"
        />
      </div>
      <TextField label="Catatan versi (opsional)" value={catatan} onChange={setCatatan} maxLength={500} placeholder="mis. perbaikan redaksi paragraf 2" />
      <ErrorBox error={error} />
      <div className="flex justify-end gap-2">
        <SecondaryButton onClick={onTutup}>Batal</SecondaryButton>
        <PrimaryButton onClick={kirim} busy={busy} disabled={!berkas}>
          Periksa dan simpan
        </PrimaryButton>
      </div>
    </div>
  );
}

function HasilUnggahan({ hasil, onTutup }: { hasil: SapaHasilUnggah; onTutup: () => void }) {
  const ada = hasil.tidak_dikenal.length > 0 || hasil.tidak_dipakai.length > 0;
  return (
    <div className="space-y-2">
      <NoticeBox tone={ada ? "warn" : "ok"}>
        <p className="font-medium">
          Template tersimpan sebagai versi {hasil.template.versi} dan langsung dipakai. Uji pengisian dengan data contoh berhasil; {hasil.penanda.length} penanda ditemukan.
        </p>
        {hasil.tidak_dikenal.length > 0 && (
          <p className="mt-1.5 text-xs">
            <span className="font-medium">Tidak dikenal (dibiarkan apa adanya pada dokumen hasil):</span> {hasil.tidak_dikenal.map((p) => `<<${p}>>`).join(", ")}
          </p>
        )}
        {hasil.tidak_dipakai.length > 0 && (
          <p className="mt-1.5 text-xs">
            <span className="font-medium">Dikenal tetapi tidak ada di template (isiannya tidak tercetak):</span> {hasil.tidak_dipakai.map((p) => `<<${p}>>`).join(", ")}
          </p>
        )}
      </NoticeBox>
      <button type="button" onClick={onTutup} className="text-xs font-medium text-slate-500 hover:text-slate-700">
        Tutup laporan
      </button>
    </div>
  );
}
