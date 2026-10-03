"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Search } from "lucide-react";
import { SapaPeran, SapaPeranRow, listSapaPeran, setSapaPeran } from "@/lib/sapa";
import { Alert } from "@/components/ui/Alert";
import { PeranBadge } from "./badges";
import { ErrorBox, NoticeBox, PrimaryButton, inputCls } from "./fields";
import { ErrorInfo, PERAN_LABEL, bersihKodeSatker, errorInfo } from "./sapa";

// Penetapan peran SAPA per pengguna. Peran SAPA terpisah dari peran aplikasi (user/admin/superadmin): admin aplikasi
// selalu dapat memakai SAPA, sedangkan pengguna lain butuh peran di sini.
export function PeranPanel() {
  const [q, setQ] = useState("");
  const [cari, setCari] = useState("");
  const [list, setList] = useState<SapaPeranRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const idPermintaan = useRef(0);

  useEffect(() => {
    const t = setTimeout(() => setCari(q.trim()), 350);
    return () => clearTimeout(t);
  }, [q]);

  const muat = useCallback(async () => {
    const id = ++idPermintaan.current;
    try {
      const res = await listSapaPeran(cari);
      if (id === idPermintaan.current) {
        setList(res.data);
        setError(null);
      }
    } catch (err) {
      if (id === idPermintaan.current) setError(errorInfo(err, "Gagal memuat pengguna").message);
    }
  }, [cari]);

  useEffect(() => {
    muat();
  }, [muat]);

  return (
    <div className="space-y-4">
      <NoticeBox>
        <b>Satuan Kerja</b> mengerjakan tahap usulan untuk satkernya (butuh kode satker 18 digit). <b>Unit Eselon I</b> mengerjakan tahap UE1 untuk satker di bawahnya (butuh kode UE1 5
        digit). <b>Kantor Wilayah</b> meneliti tiket SIMAN dan melihat semua usulan. Pengguna tanpa peran tidak dapat membuka SAPA.
      </NoticeBox>
      <div className="relative max-w-md">
        <label htmlFor="sapa-cari-pengguna" className="sr-only">
          Cari pengguna
        </label>
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
        <input
          id="sapa-cari-pengguna"
          type="search"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          maxLength={100}
          placeholder="Cari nama, username, atau email"
          className={inputCls + " pl-9"}
        />
      </div>
      {error && <Alert message={error} />}
      {!list && !error && <div role="status" aria-label="Memuat pengguna" className="h-40 animate-pulse rounded-xl bg-slate-100" />}
      {list && list.length === 0 && <p className="rounded-xl border border-dashed border-slate-300 px-4 py-8 text-center text-sm text-slate-500">Tidak ada pengguna yang cocok.</p>}
      {list && list.length > 0 && (
        <ul className="space-y-2.5">
          {list.map((r) => (
            <BarisPeran key={r.user_id} r={r} onSaved={muat} />
          ))}
          {list.length >= 50 && <li className="text-center text-xs text-slate-500">Menampilkan 50 pengguna pertama; persempit dengan pencarian.</li>}
        </ul>
      )}
    </div>
  );
}

function BarisPeran({ r, onSaved }: { r: SapaPeranRow; onSaved: () => void }) {
  const [peran, setPeran] = useState<SapaPeran | "">(r.peran);
  const [satker, setSatker] = useState(r.kode_satker);
  const [ue1, setUe1] = useState(r.kode_ue1);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ErrorInfo | null>(null);
  const [ok, setOk] = useState(false);

  const berubah = peran !== r.peran || (peran === "satker" && satker !== r.kode_satker) || (peran === "ue1" && ue1 !== r.kode_ue1);

  const simpan = async () => {
    setBusy(true);
    setError(null);
    setOk(false);
    try {
      await setSapaPeran(r.user_id, { peran, kode_satker: peran === "satker" ? satker : "", kode_ue1: peran === "ue1" ? ue1 : "" });
      setOk(true);
      onSaved();
    } catch (err) {
      setError(errorInfo(err, "Gagal menyimpan peran"));
    } finally {
      setBusy(false);
    }
  };

  const idPeran = `peran-${r.user_id}`;
  return (
    <li className="rounded-xl border border-slate-200 bg-white p-3.5">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="break-words text-sm font-semibold text-slate-900">{r.nama || r.username}</p>
          <p className="break-all text-xs text-slate-500">
            {r.username} · {r.email}
          </p>
        </div>
        <div className="flex items-center gap-1.5">
          <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-600">{r.peran_app}</span>
          {r.peran ? (
            <PeranBadge peran={r.peran} />
          ) : r.peran_app === "admin" || r.peran_app === "superadmin" ? (
            <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-600">Akses penuh sebagai admin</span>
          ) : (
            <span className="rounded-full bg-amber-50 px-2 py-0.5 text-[11px] font-medium text-amber-700">Belum ada peran SAPA</span>
          )}
        </div>
      </div>
      <div className="mt-3 grid gap-3 sm:grid-cols-[12rem_1fr_auto] sm:items-end">
        <div>
          <label htmlFor={idPeran} className="mb-1.5 block text-xs font-medium text-slate-600">
            Peran SAPA
          </label>
          <select
            id={idPeran}
            value={peran}
            onChange={(e) => {
              setPeran(e.target.value as SapaPeran | "");
              setOk(false);
            }}
            className={inputCls}
          >
            <option value="">Tanpa peran</option>
            {(Object.keys(PERAN_LABEL) as SapaPeran[]).map((p) => (
              <option key={p} value={p}>
                {PERAN_LABEL[p]}
              </option>
            ))}
          </select>
        </div>
        <div>
          {peran === "satker" && (
            <>
              <label htmlFor={idPeran + "-satker"} className="mb-1.5 block text-xs font-medium text-slate-600">
                Kode satker (18 digit)
              </label>
              <input
                id={idPeran + "-satker"}
                value={satker}
                onChange={(e) => setSatker(bersihKodeSatker(e.target.value))}
                inputMode="numeric"
                autoComplete="off"
                className={inputCls + " font-mono"}
              />
            </>
          )}
          {peran === "ue1" && (
            <>
              <label htmlFor={idPeran + "-ue1"} className="mb-1.5 block text-xs font-medium text-slate-600">
                Kode UE1 (5 digit)
              </label>
              <input
                id={idPeran + "-ue1"}
                value={ue1}
                onChange={(e) => setUe1(e.target.value.replace(/\D/g, "").slice(0, 5))}
                inputMode="numeric"
                autoComplete="off"
                className={inputCls + " font-mono"}
              />
            </>
          )}
        </div>
        <PrimaryButton onClick={simpan} busy={busy} disabled={!berubah}>
          Simpan
        </PrimaryButton>
      </div>
      {ok && !berubah && (
        <p role="status" className="mt-2 text-xs font-medium text-emerald-700">
          Peran tersimpan.
        </p>
      )}
      <div className="mt-2">
        <ErrorBox error={error} />
      </div>
    </li>
  );
}
