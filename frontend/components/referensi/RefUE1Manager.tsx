"use client";

import { useCallback, useEffect, useState } from "react";
import { CircleAlert, Pencil, Plus, Trash2 } from "lucide-react";
import axios from "axios";
import { RefUE1, RefUE1Daftar, deleteRefUE1, getRefUE1, putRefUE1 } from "@/lib/api";
import { useDashboard } from "@/lib/dashboard-context";
import { formatDate } from "@/lib/format";
import { segarkanRefUE1 } from "@/lib/useRefUE1";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { EmptyState } from "@/components/ui/EmptyState";
import { ModalShell } from "@/components/ui/ModalShell";
import { SkeletonTable } from "@/components/ui/Skeleton";
import { useToast } from "@/components/ui/Toast";

function pesanGalat(err: unknown, cadangan: string): string {
  return axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : cadangan;
}

interface Isian {
  kode: string;
  nama: string;
  singkatan: string;
  urutan: string;
  aktif: boolean;
  baru: boolean;
}

const ISIAN_BARU: Isian = { kode: "", nama: "", singkatan: "", urutan: "100", aktif: true, baru: true };

// Kelola referensi Unit Eselon I: kode 5 digit -> uraian dan singkatan. Dipakai Digitalisasi Aset, Dashboard, ekspor, dan SAPA. Hanya
// admin/superadmin yang boleh mengubah; pengguna lain melihat daftarnya saja.
export function RefUE1Manager() {
  const { profile } = useDashboard();
  const toast = useToast();
  const bolehUbah = profile ? ["admin", "superadmin"].includes(profile.role) : false;

  const [data, setData] = useState<RefUE1Daftar | null>(null);
  const [error, setError] = useState("");
  const [form, setForm] = useState<Isian | null>(null);
  const [formError, setFormError] = useState("");
  const [menyimpan, setMenyimpan] = useState(false);
  const [hapus, setHapus] = useState<RefUE1 | null>(null);
  const [menghapus, setMenghapus] = useState(false);

  const muat = useCallback(async () => {
    try {
      const res = await getRefUE1();
      setData(res.data);
      setError("");
    } catch (err) {
      setError(pesanGalat(err, "Gagal memuat referensi UE1"));
    }
  }, []);

  useEffect(() => {
    void muat();
  }, [muat]);

  const ubah = (r: RefUE1) => {
    setFormError("");
    setForm({ kode: r.kode, nama: r.nama, singkatan: r.singkatan, urutan: String(r.urutan), aktif: r.aktif, baru: false });
  };

  const simpan = async () => {
    if (!form) return;
    const urutan = Number(form.urutan);
    if (!Number.isInteger(urutan) || urutan < 0 || urutan > 9999) {
      setFormError("Urutan harus bilangan bulat 0 sampai 9999.");
      return;
    }
    setMenyimpan(true);
    setFormError("");
    try {
      await putRefUE1(form.kode, { nama: form.nama, singkatan: form.singkatan, urutan, aktif: form.aktif });
      toast.success(`UE1 ${form.kode} disimpan.`);
      setForm(null);
      await muat();
      void segarkanRefUE1(); // label di halaman lain ikut berubah
    } catch (err) {
      setFormError(pesanGalat(err, "Gagal menyimpan"));
    } finally {
      setMenyimpan(false);
    }
  };

  const konfirmasiHapus = async () => {
    if (!hapus) return;
    setMenghapus(true);
    try {
      await deleteRefUE1(hapus.kode);
      toast.success(`UE1 ${hapus.kode} dihapus.`);
      setHapus(null);
      await muat();
      void segarkanRefUE1();
    } catch (err) {
      toast.error(pesanGalat(err, "Gagal menghapus"));
    } finally {
      setMenghapus(false);
    }
  };

  const kodeSah = /^\d{5}$/.test(form?.kode ?? "");
  const bisaSimpan = Boolean(form && kodeSah && form.nama.trim());

  return (
    <div className="space-y-5">
      <p className="rounded-xl border border-blue-100 bg-blue-50/60 px-4 py-3 text-sm text-slate-700">
        Referensi ini menerjemahkan kode UE1 (mis. <span className="font-mono">01504</span>) menjadi uraian dan singkatan di Digitalisasi Aset, Dashboard, berkas unduhan, dan SAPA.
        Bila ada perubahan organisasi, ubah di sini; tampilan di seluruh aplikasi ikut berubah tanpa mengubah data aset.
      </p>

      {error && <Alert message={error} />}

      {data && data.belum_terdaftar.length > 0 && (
        <section className="rounded-xl border border-amber-200 bg-amber-50 p-4" aria-labelledby="ue1-belum">
          <h2 id="ue1-belum" className="flex items-center gap-2 text-sm font-semibold text-amber-900">
            <CircleAlert className="h-4 w-4" aria-hidden="true" />
            {data.belum_terdaftar.length} kode UE1 di data aset belum punya referensi
          </h2>
          <p className="mt-1 text-xs text-amber-800">Kode ini tampil sebagai &ldquo;UE1 &lt;kode&gt;&rdquo; tanpa uraian sampai ditambahkan.</p>
          <ul className="mt-3 flex flex-wrap gap-2">
            {data.belum_terdaftar.map((b) => (
              <li key={b.kode} className="inline-flex items-center gap-2 rounded-lg bg-white px-3 py-1.5 text-sm shadow-sm ring-1 ring-amber-200">
                <span className="font-mono font-semibold text-slate-800">{b.kode}</span>
                <span className="text-xs text-slate-500">{b.satker} satker</span>
                {bolehUbah && (
                  <button
                    type="button"
                    onClick={() => {
                      setFormError("");
                      setForm({ ...ISIAN_BARU, kode: b.kode });
                    }}
                    className="text-xs font-medium text-blue-700 hover:underline"
                  >
                    Tambahkan
                  </button>
                )}
              </li>
            ))}
          </ul>
        </section>
      )}

      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-slate-600">
          {data ? (
            <>
              <span className="font-semibold text-slate-900">{data.daftar.length}</span> unit eselon I
              {data.daftar.some((r) => !r.aktif) && <span className="text-slate-400"> · {data.daftar.filter((r) => !r.aktif).length} nonaktif</span>}
            </>
          ) : (
            "Memuat..."
          )}
        </p>
        {bolehUbah && (
          <Button
            size="sm"
            icon={<Plus className="h-4 w-4" aria-hidden="true" />}
            fullWidth={false}
            onClick={() => {
              setFormError("");
              setForm(ISIAN_BARU);
            }}
          >
            Tambah UE1
          </Button>
        )}
      </div>

      {!data && !error && <SkeletonTable rows={6} />}
      {data && data.daftar.length === 0 && (
        <EmptyState title="Belum ada referensi UE1" description={bolehUbah ? "Tambahkan UE1 pertama dengan tombol di atas." : "Hubungi administrator untuk mengisinya."} />
      )}

      {data && data.daftar.length > 0 && (
        <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm">
          <table className="w-full min-w-max text-sm">
            <thead className="bg-slate-50 text-xs text-slate-500">
              <tr>
                <th scope="col" className="px-4 py-2.5 text-left font-medium">Kode</th>
                <th scope="col" className="px-4 py-2.5 text-left font-medium">Singkatan</th>
                <th scope="col" className="px-4 py-2.5 text-left font-medium">Uraian</th>
                <th scope="col" className="px-4 py-2.5 text-right font-medium">Urutan</th>
                <th scope="col" className="px-4 py-2.5 text-left font-medium">Status</th>
                <th scope="col" className="px-4 py-2.5 text-left font-medium">Diubah</th>
                {bolehUbah && <th scope="col" className="px-4 py-2.5 text-right font-medium"><span className="sr-only">Tindakan</span></th>}
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {data.daftar.map((r) => (
                <tr key={r.kode} className={r.aktif ? "hover:bg-slate-50" : "bg-slate-50/60 text-slate-400"}>
                  <td className="px-4 py-2.5 font-mono font-semibold text-slate-800">{r.kode}</td>
                  <td className="px-4 py-2.5 font-medium">{r.singkatan || <span className="text-slate-300">-</span>}</td>
                  <td className="max-w-[28rem] px-4 py-2.5">{r.nama}</td>
                  <td className="px-4 py-2.5 text-right tabular-nums">{r.urutan}</td>
                  <td className="px-4 py-2.5">
                    <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset ${r.aktif ? "bg-emerald-50 text-emerald-700 ring-emerald-200" : "bg-slate-100 text-slate-500 ring-slate-200"}`}>
                      {r.aktif ? "Aktif" : "Nonaktif"}
                    </span>
                  </td>
                  <td className="whitespace-nowrap px-4 py-2.5 text-xs text-slate-500">
                    {r.diubah_pada ? formatDate(r.diubah_pada) : "-"}
                    {r.diubah_oleh && <span className="block text-slate-400">{r.diubah_oleh}</span>}
                  </td>
                  {bolehUbah && (
                    <td className="whitespace-nowrap px-4 py-2.5 text-right">
                      <button type="button" onClick={() => ubah(r)} aria-label={`Ubah UE1 ${r.kode}`} className="rounded p-2 text-slate-400 hover:bg-slate-100 hover:text-slate-700">
                        <Pencil className="h-4 w-4" aria-hidden="true" />
                      </button>
                      <button type="button" onClick={() => setHapus(r)} aria-label={`Hapus UE1 ${r.kode}`} className="rounded p-2 text-slate-400 hover:bg-red-50 hover:text-red-600">
                        <Trash2 className="h-4 w-4" aria-hidden="true" />
                      </button>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {form && (
        <ModalShell title={form.baru ? "Tambah Unit Eselon I" : `Ubah UE1 ${form.kode}`} subtitle="Kode, uraian, dan singkatan tampil di seluruh aplikasi." onClose={() => !menyimpan && setForm(null)} size="md">
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (bisaSimpan && !menyimpan) void simpan();
            }}
            className="space-y-4 px-4 py-4 sm:px-6"
          >
            <label className="block text-sm">
              <span className="mb-1.5 block font-medium text-slate-700">Kode UE1 (5 digit)</span>
              <input
                value={form.kode}
                onChange={(e) => setForm({ ...form, kode: e.target.value.replace(/\D/g, "").slice(0, 5) })}
                inputMode="numeric"
                autoComplete="off"
                disabled={!form.baru}
                required
                className="w-full rounded-xl border border-slate-300 bg-white px-3.5 py-2.5 font-mono text-sm outline-none focus:border-blue-500 focus:shadow-glow disabled:bg-slate-100 disabled:text-slate-500"
              />
            </label>
            <label className="block text-sm">
              <span className="mb-1.5 block font-medium text-slate-700">Uraian</span>
              <input
                value={form.nama}
                onChange={(e) => setForm({ ...form, nama: e.target.value })}
                maxLength={200}
                required
                className="w-full rounded-xl border border-slate-300 bg-white px-3.5 py-2.5 text-sm outline-none focus:border-blue-500 focus:shadow-glow"
                placeholder="mis. DIREKTORAT JENDERAL PAJAK"
              />
            </label>
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="block text-sm">
                <span className="mb-1.5 block font-medium text-slate-700">Singkatan</span>
                <input
                  value={form.singkatan}
                  onChange={(e) => setForm({ ...form, singkatan: e.target.value })}
                  maxLength={30}
                  className="w-full rounded-xl border border-slate-300 bg-white px-3.5 py-2.5 text-sm outline-none focus:border-blue-500 focus:shadow-glow"
                  placeholder="mis. DJP"
                />
              </label>
              <label className="block text-sm">
                <span className="mb-1.5 block font-medium text-slate-700">Urutan tampil</span>
                <input
                  value={form.urutan}
                  onChange={(e) => setForm({ ...form, urutan: e.target.value.replace(/\D/g, "").slice(0, 4) })}
                  inputMode="numeric"
                  className="w-full rounded-xl border border-slate-300 bg-white px-3.5 py-2.5 text-sm tabular-nums outline-none focus:border-blue-500 focus:shadow-glow"
                />
              </label>
            </div>
            <label className="flex cursor-pointer items-start gap-2.5 text-sm text-slate-700">
              <input type="checkbox" checked={form.aktif} onChange={(e) => setForm({ ...form, aktif: e.target.checked })} className="mt-0.5 h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" />
              <span>
                Aktif
                <span className="block text-xs text-slate-500">UE1 yang sudah tidak ada dapat dinonaktifkan; kodenya tetap diterjemahkan pada data lama.</span>
              </span>
            </label>
            {formError && <Alert message={formError} />}
            <div className="flex justify-end gap-2 pt-1">
              <Button type="button" variant="secondary" fullWidth={false} onClick={() => setForm(null)} disabled={menyimpan}>
                Batal
              </Button>
              <Button type="submit" fullWidth={false} isLoading={menyimpan} disabled={!bisaSimpan}>
                Simpan
              </Button>
            </div>
          </form>
        </ModalShell>
      )}

      {hapus && (
        <ConfirmDialog
          title={`Hapus UE1 ${hapus.kode}?`}
          message={
            <>
              Referensi <b>{hapus.singkatan || hapus.nama}</b> dihapus. Data aset tidak berubah; kode {hapus.kode} kembali tampil sebagai &ldquo;UE1 {hapus.kode}&rdquo; tanpa uraian. Untuk UE1
              yang sudah tidak dipakai, lebih baik <b>nonaktifkan</b> agar uraiannya tetap terbaca pada data lama.
            </>
          }
          confirmLabel="Hapus"
          tone="danger"
          busy={menghapus}
          onConfirm={konfirmasiHapus}
          onCancel={() => !menghapus && setHapus(null)}
        />
      )}
    </div>
  );
}
