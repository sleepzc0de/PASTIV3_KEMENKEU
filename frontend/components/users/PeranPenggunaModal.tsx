"use client";

import { useCallback, useEffect, useState } from "react";
import { Plus, Trash2, Wand2 } from "lucide-react";
import axios from "axios";
import { PeranData, PeranPengguna, cabutPeranPengguna, getPeranPengguna, tambahPeranPengguna } from "@/lib/api";
import { PERAN_DATA, kodePeranSah, namaPeranBerkode, panjangKode, saranBaru } from "@/lib/peran";
import { useRefKanwil } from "@/lib/useRefKanwil";
import { useRefUE1 } from "@/lib/useRefUE1";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { ModalShell } from "@/components/ui/ModalShell";
import { useToast } from "@/components/ui/Toast";

function pesanGalat(err: unknown, cadangan: string): string {
  return axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : cadangan;
}

interface Props {
  userId: string;
  nama: string;
  onClose: () => void;
  bacaSaja?: boolean; // UE1, Kanwil, dan Satker hanya melihat peran pengguna dalam cakupannya
}

// Peran data seorang pengguna: Pengguna Barang (seluruh data), UE1, Kanwil, atau Satker (data di bawah kode itu). Boleh lebih dari satu; pengguna
// berpindah lewat menu pengguna. Saran diturunkan dari kode satker di data SSO-nya (5 digit pertama = UE1, 9 digit pertama = Kanwil, digit 10-15 = satker).
export function PeranPenggunaModal({ userId, nama, onClose, bacaSaja = false }: Props) {
  const toast = useToast();
  const ue1 = useRefUE1();
  const kanwil = useRefKanwil();
  const [data, setData] = useState<PeranPengguna | null>(null);
  const [error, setError] = useState("");
  const [peran, setPeran] = useState<PeranData>("ue1");
  const [kode, setKode] = useState("");
  const [sibuk, setSibuk] = useState(false);
  const [mencabut, setMencabut] = useState<number | null>(null);

  const muat = useCallback(async () => {
    try {
      const res = await getPeranPengguna(userId);
      setData(res.data);
      setError("");
    } catch (err) {
      setError(pesanGalat(err, "Gagal memuat peran pengguna"));
    }
  }, [userId]);

  useEffect(() => {
    void muat();
  }, [muat]);

  const tambah = async (p: PeranData, k: string) => {
    setSibuk(true);
    setError("");
    try {
      const res = await tambahPeranPengguna(userId, { role: p, kode: k });
      toast.success(res.message || "Peran diberikan.");
      setKode("");
      await muat();
    } catch (err) {
      setError(pesanGalat(err, "Gagal memberi peran"));
    } finally {
      setSibuk(false);
    }
  };

  const cabut = async (id: number) => {
    setMencabut(id);
    setError("");
    try {
      await cabutPeranPengguna(userId, id);
      toast.success("Peran dicabut.");
      await muat();
    } catch (err) {
      setError(pesanGalat(err, "Gagal mencabut peran"));
    } finally {
      setMencabut(null);
    }
  };

  const spec = PERAN_DATA.find((x) => x.peran === peran)!;
  const bisaTambah = kodePeranSah(peran, kode) && !sibuk;
  const usulan = data ? saranBaru(data.saran, data.peran) : [];

  return (
    <ModalShell title={`Peran data: ${nama}`} subtitle={bacaSaja ? "Peran yang dimiliki pengguna ini (hanya melihat)." : "Pengguna boleh memegang banyak peran dan berpindah lewat menu pengguna."} onClose={onClose} size="lg">
      <div className="space-y-5 px-4 py-4 sm:px-6">
        {data && data.peran.length === 0 && (
          <p className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900">
            Pengguna ini <b>tamu</b>: belum punya peran, jadi belum dapat membuka fitur apa pun{bacaSaja ? "." : " sampai diberi peran."}
          </p>
        )}
        {error && <Alert message={error} />}

        <section aria-labelledby="peran-dimiliki">
          <h3 id="peran-dimiliki" className="text-sm font-semibold text-slate-900">
            Peran yang dimiliki
          </h3>
          {!data && !error && <div role="status" aria-label="Memuat peran" className="mt-2 h-16 animate-pulse rounded-lg bg-slate-100" />}
          {data && data.peran.length === 0 && <p className="mt-2 text-sm text-slate-500">Belum ada peran.</p>}
          {data && data.peran.length > 0 && (
            <ul className="mt-2 divide-y divide-slate-100 rounded-xl border border-slate-200">
              {data.peran.map((b) => (
                <li key={b.id} className="flex items-center justify-between gap-3 px-3.5 py-2.5">
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-slate-900">
                      {namaPeranBerkode(b.role, b.kode)}
                      {b.role === "ue1" && ue1.label(b.kode).includes("·") && <span className="font-normal text-slate-500"> ({ue1.label(b.kode).split(" · ")[1]})</span>}
                      {b.role === "kanwil" && kanwil.label(b.kode).includes("·") && <span className="font-normal text-slate-500"> ({kanwil.label(b.kode).split(" · ")[1]})</span>}
                      {b.aktif && <span className="ml-2 rounded-full bg-blue-50 px-2 py-0.5 text-[11px] font-medium text-blue-700">sedang aktif</span>}
                    </p>
                    {b.dibuat_oleh && <p className="text-xs text-slate-400">Diberikan oleh {b.dibuat_oleh}</p>}
                  </div>
                  {!bacaSaja && (
                    <button
                      type="button"
                      onClick={() => cabut(b.id)}
                      disabled={mencabut !== null}
                      aria-label={`Cabut peran ${namaPeranBerkode(b.role, b.kode)}`}
                      className="rounded p-2 text-slate-400 hover:bg-red-50 hover:text-red-600 disabled:opacity-50"
                    >
                      <Trash2 className="h-4 w-4" aria-hidden="true" />
                    </button>
                  )}
                </li>
              ))}
            </ul>
          )}
        </section>

        {data && !bacaSaja && (usulan.length > 0 || data.kode_satker_sso) && (
          <section aria-labelledby="peran-saran" className="rounded-xl border border-blue-100 bg-blue-50/50 p-3.5">
            <h3 id="peran-saran" className="flex items-center gap-1.5 text-sm font-semibold text-slate-900">
              <Wand2 className="h-4 w-4 text-blue-600" aria-hidden="true" />
              Saran dari data SSO
            </h3>
            {data.kode_satker_sso ? (
              <p className="mt-1 text-xs text-slate-600">
                Kode satker pegawai di SSO: <span className="font-mono">{data.kode_satker_sso}</span>
              </p>
            ) : (
              <p className="mt-1 text-xs text-slate-600">Pengguna ini belum punya kode satker dari SSO.</p>
            )}
            {usulan.length > 0 ? (
              <div className="mt-2 flex flex-wrap gap-2">
                {usulan.map((s) => (
                  <button
                    key={`${s.role}-${s.kode}`}
                    type="button"
                    disabled={sibuk}
                    onClick={() => tambah(s.role, s.kode)}
                    className="inline-flex items-center gap-1.5 rounded-lg bg-white px-3 py-1.5 text-sm font-medium text-blue-700 shadow-sm ring-1 ring-blue-200 hover:bg-blue-50 disabled:opacity-50"
                  >
                    <Plus className="h-3.5 w-3.5" aria-hidden="true" />
                    {namaPeranBerkode(s.role, s.kode)}
                    {s.role === "kanwil" && kanwil.label(s.kode).includes("·") && <span className="font-normal text-slate-500"> ({kanwil.label(s.kode).split(" · ")[1]})</span>}
                  </button>
                ))}
              </div>
            ) : (
              data.kode_satker_sso && <p className="mt-1 text-xs text-slate-500">Tidak ada saran baru (atau kodenya belum dikenali: kode satker lengkap minimal 15 karakter).</p>
            )}
          </section>
        )}

        {bacaSaja && data?.kode_satker_sso && (
          <p className="text-xs text-slate-500">
            Kode satker pegawai di SSO: <span className="font-mono">{data.kode_satker_sso}</span>
          </p>
        )}

        {!bacaSaja && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (bisaTambah) void tambah(peran, kode);
          }}
          className="space-y-3 rounded-xl border border-slate-200 p-3.5"
          aria-label="Tambah peran"
        >
          <h3 className="text-sm font-semibold text-slate-900">Tambah peran</h3>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="block text-sm">
              <span className="mb-1.5 block font-medium text-slate-700">Peran</span>
              <select
                value={peran}
                onChange={(e) => {
                  setPeran(e.target.value as PeranData);
                  setKode("");
                }}
                className="w-full rounded-xl border border-slate-300 bg-white px-3 py-2.5 text-sm outline-none focus:border-blue-500 focus:shadow-glow"
              >
                {PERAN_DATA.map((p) => (
                  <option key={p.peran} value={p.peran}>
                    {p.label}
                  </option>
                ))}
              </select>
            </label>
            {panjangKode(peran) > 0 && (
              <label className="block text-sm">
                <span className="mb-1.5 block font-medium text-slate-700">Kode {spec.label} ({spec.panjang} digit)</span>
                {peran === "ue1" && ue1.daftar.length > 0 ? (
                  <select
                    value={kode}
                    onChange={(e) => setKode(e.target.value)}
                    className="w-full rounded-xl border border-slate-300 bg-white px-3 py-2.5 text-sm outline-none focus:border-blue-500 focus:shadow-glow"
                  >
                    <option value="">Pilih UE1…</option>
                    {ue1.daftar
                      .filter((r) => r.aktif)
                      .map((r) => (
                        <option key={r.kode} value={r.kode}>
                          {ue1.label(r.kode)}
                        </option>
                      ))}
                  </select>
                ) : (
                  <input
                    value={kode}
                    onChange={(e) => setKode(e.target.value.replace(/\D/g, "").slice(0, spec.panjang))}
                    inputMode="numeric"
                    autoComplete="off"
                    placeholder={`mis. ${spec.contoh}`}
                    list={peran === "kanwil" && kanwil.daftar.length > 0 ? "daftar-kanwil" : undefined}
                    className="w-full rounded-xl border border-slate-300 bg-white px-3 py-2.5 font-mono text-sm outline-none focus:border-blue-500 focus:shadow-glow"
                  />
                )}
                {peran === "kanwil" && kanwil.daftar.length > 0 && (
                  <datalist id="daftar-kanwil">
                    {kanwil.daftar
                      .filter((r) => r.aktif)
                      .map((r) => (
                        <option key={r.kode} value={r.kode}>
                          {r.nama}
                        </option>
                      ))}
                  </datalist>
                )}
                {peran === "kanwil" && kode.length === 9 && (
                  <span className="mt-1 block text-xs text-slate-500">{kanwil.label(kode).startsWith("Kanwil ") ? "Kode ini belum ada di Referensi Kanwil." : kanwil.nama(kode)}</span>
                )}
              </label>
            )}
          </div>
          <p className="text-xs text-slate-500">{spec.keterangan}.</p>
          <div className="flex justify-end">
            <Button type="submit" fullWidth={false} size="sm" isLoading={sibuk} disabled={!bisaTambah} icon={<Plus className="h-4 w-4" aria-hidden="true" />}>
              Tambah peran
            </Button>
          </div>
        </form>
        )}

        <div className="flex justify-end">
          <Button type="button" variant="secondary" fullWidth={false} onClick={onClose}>
            Tutup
          </Button>
        </div>
      </div>
    </ModalShell>
  );
}
