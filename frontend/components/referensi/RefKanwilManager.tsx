"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ChevronLeft, ChevronRight, CircleAlert, Download, Pencil, Plus, Search, Trash2 } from "lucide-react";
import axios from "axios";
import { HasilTarikKanwil, RefKanwil, RefKanwilDaftar, deleteRefKanwil, getRefKanwil, putRefKanwil, tambahKanwilDariSatker, tarikKanwilDariSLDK } from "@/lib/api";
import { useDashboard } from "@/lib/dashboard-context";
import { formatDate } from "@/lib/format";
import { LABEL_SUMBER_KANWIL, ringkasHasilTarik, saringKanwil } from "@/lib/refKanwil";
import { segarkanRefKanwil } from "@/lib/useRefKanwil";
import { useRefUE1 } from "@/lib/useRefUE1";
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

const PER_HALAMAN = 50;
const BELUM_TAMPIL_AWAL = 8;

interface Isian {
  kode: string;
  nama: string;
  singkatan: string;
  urutan: string;
  aktif: boolean;
  baru: boolean;
}

const ISIAN_BARU: Isian = { kode: "", nama: "", singkatan: "", urutan: "100", aktif: true, baru: true };

const WARNA_SUMBER: Record<RefKanwil["sumber"], string> = {
  manual: "bg-violet-50 text-violet-700 ring-violet-200",
  satker: "bg-amber-50 text-amber-800 ring-amber-200",
  sldk: "bg-sky-50 text-sky-700 ring-sky-200",
};

// Kelola referensi Kantor Wilayah (Kanwil): kode 9 digit (9 karakter pertama kode satker) -> uraian dan singkatan. Isinya dari nama satker pada data Digitalisasi
// Aset, ditarik dari SLDK, atau diisi manual. Hanya superadmin yang boleh mengubah; pengguna lain melihat daftarnya saja.
export function RefKanwilManager() {
  const { profile } = useDashboard();
  const toast = useToast();
  const ue1 = useRefUE1();
  const bolehUbah = profile?.role === "superadmin";

  const [data, setData] = useState<RefKanwilDaftar | null>(null);
  const [error, setError] = useState("");
  const [q, setQ] = useState("");
  const [halaman, setHalaman] = useState(1);
  const [belumSemua, setBelumSemua] = useState(false);
  const [form, setForm] = useState<Isian | null>(null);
  const [formError, setFormError] = useState("");
  const [menyimpan, setMenyimpan] = useState(false);
  const [hapus, setHapus] = useState<RefKanwil | null>(null);
  const [menghapus, setMenghapus] = useState(false);
  const [konfirmasiTarik, setKonfirmasiTarik] = useState(false);
  const [menarik, setMenarik] = useState(false);
  const [hasilTarik, setHasilTarik] = useState<HasilTarikKanwil | null>(null);
  const [menambahSemua, setMenambahSemua] = useState(false);

  const muat = useCallback(async () => {
    try {
      const res = await getRefKanwil();
      setData(res.data);
      setError("");
    } catch (err) {
      setError(pesanGalat(err, "Gagal memuat referensi Kanwil"));
    }
  }, []);

  useEffect(() => {
    void muat();
  }, [muat]);

  const disaring = useMemo(() => (data ? saringKanwil(data.daftar, q) : []), [data, q]);
  const totalHalaman = Math.max(1, Math.ceil(disaring.length / PER_HALAMAN));
  const halamanAktif = Math.min(halaman, totalHalaman);
  const tampil = disaring.slice((halamanAktif - 1) * PER_HALAMAN, halamanAktif * PER_HALAMAN);

  const belum = data?.belum_terdaftar ?? [];
  const belumBersaran = belum.filter((b) => b.saran !== "").length;
  const belumTampil = belumSemua ? belum : belum.slice(0, BELUM_TAMPIL_AWAL);

  const ubah = (r: RefKanwil) => {
    setFormError("");
    setForm({ kode: r.kode, nama: r.nama, singkatan: r.singkatan, urutan: String(r.urutan), aktif: r.aktif, baru: false });
  };

  const sesudahPerubahan = async () => {
    await muat();
    void segarkanRefKanwil(); // label di halaman lain ikut berubah
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
      await putRefKanwil(form.kode, { nama: form.nama, singkatan: form.singkatan, urutan, aktif: form.aktif });
      toast.success(`Kanwil ${form.kode} disimpan.`);
      setForm(null);
      await sesudahPerubahan();
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
      await deleteRefKanwil(hapus.kode);
      toast.success(`Kanwil ${hapus.kode} dihapus.`);
      setHapus(null);
      await sesudahPerubahan();
    } catch (err) {
      toast.error(pesanGalat(err, "Gagal menghapus"));
    } finally {
      setMenghapus(false);
    }
  };

  const tambahSemua = async () => {
    setMenambahSemua(true);
    try {
      const res = await tambahKanwilDariSatker();
      toast.success(`${res.data.ditambahkan} Kanwil ditambahkan dari data satker.`);
      await sesudahPerubahan();
    } catch (err) {
      toast.error(pesanGalat(err, "Gagal menambahkan dari data satker"));
    } finally {
      setMenambahSemua(false);
    }
  };

  const tarik = async () => {
    setMenarik(true);
    try {
      const res = await tarikKanwilDariSLDK();
      setHasilTarik(res.data);
      setKonfirmasiTarik(false);
      toast.success("Referensi Kanwil ditarik dari SLDK.");
      await sesudahPerubahan();
    } catch (err) {
      setKonfirmasiTarik(false);
      toast.error(pesanGalat(err, "Gagal menarik dari SLDK"));
    } finally {
      setMenarik(false);
    }
  };

  const kodeSah = /^\d{9}$/.test(form?.kode ?? "");
  const bisaSimpan = Boolean(form && kodeSah && form.nama.trim());

  return (
    <div className="space-y-5">
      <p className="rounded-xl border border-blue-100 bg-blue-50/60 px-4 py-3 text-sm text-slate-700">
        Referensi ini menerjemahkan kode Kanwil (9 karakter pertama kode satker, mis. <span className="font-mono">015040199</span>) menjadi uraian dan singkatan pada peran pengguna dan
        rincian data aset. Uraian bisa diambil dari <b>nama satker</b> pada data Digitalisasi Aset, <b>ditarik dari SLDK</b> (tabel referensi korwil), atau diisi <b>manual</b>; isian
        manual tidak pernah ditimpa penarikan dari SLDK.
      </p>

      {error && <Alert message={error} />}

      {hasilTarik && (
        <div role="status" className="flex items-start justify-between gap-3 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-900">
          <p>
            <span className="font-semibold">Penarikan dari SLDK selesai.</span> {ringkasHasilTarik(hasilTarik)}
          </p>
          <button type="button" onClick={() => setHasilTarik(null)} className="shrink-0 text-xs font-medium text-emerald-800 hover:underline">
            Tutup
          </button>
        </div>
      )}

      {data && belum.length > 0 && (
        <section className="rounded-xl border border-amber-200 bg-amber-50 p-4" aria-labelledby="kanwil-belum">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 id="kanwil-belum" className="flex items-center gap-2 text-sm font-semibold text-amber-900">
                <CircleAlert className="h-4 w-4" aria-hidden="true" />
                {belum.length} kode Kanwil di data aset belum punya referensi
              </h2>
              <p className="mt-1 text-xs text-amber-800">
                Kode ini tampil sebagai &ldquo;Kanwil &lt;kode&gt;&rdquo; tanpa uraian sampai ditambahkan. Saran uraian diambil dari nama satker yang menyebut kantor wilayah atau kantor pusat
                pada kode itu; bila kosong, isi manual atau tarik dari SLDK.
              </p>
            </div>
            {bolehUbah && belumBersaran > 0 && (
              <Button size="sm" variant="secondary" fullWidth={false} isLoading={menambahSemua} onClick={() => void tambahSemua()}>
                Tambahkan {belumBersaran} yang punya saran
              </Button>
            )}
          </div>
          <ul className="mt-3 divide-y divide-amber-100 overflow-hidden rounded-lg bg-white ring-1 ring-amber-200">
            {belumTampil.map((b) => (
              <li key={b.kode} className="flex flex-wrap items-center gap-x-4 gap-y-1 px-3 py-2 text-sm">
                <span className="font-mono font-semibold text-slate-800">{b.kode}</span>
                <span className="text-xs text-slate-500">{ue1.label(b.kode_ue1)}</span>
                <span className="text-xs text-slate-500">{b.satker} satker</span>
                <span className={`min-w-0 flex-1 truncate text-xs ${b.saran ? "text-slate-700" : "text-slate-300"}`} title={b.saran || undefined}>
                  {b.saran ? `Saran: ${b.saran}` : "tanpa saran"}
                </span>
                {bolehUbah && (
                  <button
                    type="button"
                    onClick={() => {
                      setFormError("");
                      setForm({ ...ISIAN_BARU, kode: b.kode, nama: b.saran });
                    }}
                    className="text-xs font-medium text-blue-700 hover:underline"
                  >
                    Tambahkan
                  </button>
                )}
              </li>
            ))}
          </ul>
          {belum.length > BELUM_TAMPIL_AWAL && (
            <button type="button" onClick={() => setBelumSemua((v) => !v)} className="mt-2 text-xs font-medium text-amber-900 hover:underline">
              {belumSemua ? "Tampilkan lebih sedikit" : `Tampilkan semua ${belum.length} kode`}
            </button>
          )}
        </section>
      )}

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="relative min-w-[14rem] flex-1 sm:max-w-sm">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
          <input
            type="search"
            value={q}
            onChange={(e) => {
              setQ(e.target.value);
              setHalaman(1);
            }}
            maxLength={100}
            placeholder="Cari kode, uraian, atau singkatan"
            aria-label="Cari Kanwil"
            className="w-full rounded-xl border border-slate-300 bg-white py-2.5 pl-9 pr-3 text-sm outline-none focus:border-blue-500 focus:shadow-glow"
          />
        </div>
        {bolehUbah && (
          <div className="flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="secondary"
              icon={<Download className="h-4 w-4" aria-hidden="true" />}
              fullWidth={false}
              disabled={!data?.sldk_tersedia}
              title={data && !data.sldk_tersedia ? "Koneksi SLDK belum tersedia di server (SLDK_DB_*)" : undefined}
              onClick={() => setKonfirmasiTarik(true)}
            >
              Tarik dari SLDK
            </Button>
            <Button
              size="sm"
              icon={<Plus className="h-4 w-4" aria-hidden="true" />}
              fullWidth={false}
              onClick={() => {
                setFormError("");
                setForm(ISIAN_BARU);
              }}
            >
              Tambah Kanwil
            </Button>
          </div>
        )}
      </div>

      <p className="text-sm text-slate-600">
        {data ? (
          <>
            <span className="font-semibold text-slate-900">{q.trim() ? `${disaring.length} dari ${data.daftar.length}` : data.daftar.length}</span> Kanwil
            {data.daftar.some((r) => !r.aktif) && <span className="text-slate-400"> · {data.daftar.filter((r) => !r.aktif).length} nonaktif</span>}
          </>
        ) : (
          "Memuat..."
        )}
      </p>

      {!data && !error && <SkeletonTable rows={6} />}
      {data && data.daftar.length === 0 && (
        <EmptyState
          title="Belum ada referensi Kanwil"
          description={bolehUbah ? "Tambahkan dari data satker (bila ada saran di atas), tarik dari SLDK, atau isi manual." : "Hubungi administrator untuk mengisinya."}
        />
      )}
      {data && data.daftar.length > 0 && disaring.length === 0 && <EmptyState compact title="Tidak ada Kanwil yang cocok" description="Ubah kata kunci pencarian." />}

      {tampil.length > 0 && (
        <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm">
          <table className="w-full min-w-max text-sm">
            <thead className="bg-slate-50 text-xs text-slate-500">
              <tr>
                <th scope="col" className="px-4 py-2.5 text-left font-medium">Kode</th>
                <th scope="col" className="px-4 py-2.5 text-left font-medium">UE1</th>
                <th scope="col" className="px-4 py-2.5 text-left font-medium">Singkatan</th>
                <th scope="col" className="px-4 py-2.5 text-left font-medium">Uraian</th>
                <th scope="col" className="px-4 py-2.5 text-left font-medium">Sumber</th>
                <th scope="col" className="px-4 py-2.5 text-right font-medium">Urutan</th>
                <th scope="col" className="px-4 py-2.5 text-left font-medium">Status</th>
                <th scope="col" className="px-4 py-2.5 text-left font-medium">Diubah</th>
                {bolehUbah && <th scope="col" className="px-4 py-2.5 text-right font-medium"><span className="sr-only">Tindakan</span></th>}
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {tampil.map((r) => (
                <tr key={r.kode} className={r.aktif ? "hover:bg-slate-50" : "bg-slate-50/60 text-slate-400"}>
                  <td className="px-4 py-2.5 font-mono font-semibold text-slate-800">{r.kode}</td>
                  <td className="whitespace-nowrap px-4 py-2.5 text-xs text-slate-600">{ue1.label(r.kode_ue1)}</td>
                  <td className="px-4 py-2.5 font-medium">{r.singkatan || <span className="text-slate-300">-</span>}</td>
                  <td className="max-w-[28rem] px-4 py-2.5">{r.nama}</td>
                  <td className="whitespace-nowrap px-4 py-2.5">
                    <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset ${WARNA_SUMBER[r.sumber]}`}>{LABEL_SUMBER_KANWIL[r.sumber]}</span>
                    {r.sumber === "sldk" && r.status_sldk && <span className="ml-1.5 text-xs text-slate-400" title="Status di SLDK">{r.status_sldk}</span>}
                  </td>
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
                      <button type="button" onClick={() => ubah(r)} aria-label={`Ubah Kanwil ${r.kode}`} className="rounded p-2 text-slate-400 hover:bg-slate-100 hover:text-slate-700">
                        <Pencil className="h-4 w-4" aria-hidden="true" />
                      </button>
                      <button type="button" onClick={() => setHapus(r)} aria-label={`Hapus Kanwil ${r.kode}`} className="rounded p-2 text-slate-400 hover:bg-red-50 hover:text-red-600">
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

      {disaring.length > PER_HALAMAN && (
        <nav aria-label="Halaman" className="flex items-center justify-between text-sm text-slate-600">
          <span>
            Halaman {halamanAktif} dari {totalHalaman}
          </span>
          <div className="flex gap-2">
            <Button size="sm" variant="secondary" fullWidth={false} disabled={halamanAktif <= 1} icon={<ChevronLeft className="h-4 w-4" aria-hidden="true" />} onClick={() => setHalaman(halamanAktif - 1)}>
              Sebelumnya
            </Button>
            <Button size="sm" variant="secondary" fullWidth={false} disabled={halamanAktif >= totalHalaman} icon={<ChevronRight className="h-4 w-4" aria-hidden="true" />} onClick={() => setHalaman(halamanAktif + 1)}>
              Berikutnya
            </Button>
          </div>
        </nav>
      )}

      {form && (
        <ModalShell title={form.baru ? "Tambah Kanwil" : `Ubah Kanwil ${form.kode}`} subtitle="Kode, uraian, dan singkatan tampil pada peran pengguna dan rincian data aset." onClose={() => !menyimpan && setForm(null)} size="md">
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (bisaSimpan && !menyimpan) void simpan();
            }}
            className="space-y-4 px-4 py-4 sm:px-6"
          >
            <label className="block text-sm">
              <span className="mb-1.5 block font-medium text-slate-700">Kode Kanwil (9 digit)</span>
              <input
                value={form.kode}
                onChange={(e) => setForm({ ...form, kode: e.target.value.replace(/\D/g, "").slice(0, 9) })}
                inputMode="numeric"
                autoComplete="off"
                disabled={!form.baru}
                required
                className="w-full rounded-xl border border-slate-300 bg-white px-3.5 py-2.5 font-mono text-sm outline-none focus:border-blue-500 focus:shadow-glow disabled:bg-slate-100 disabled:text-slate-500"
              />
              <span className="mt-1 block text-xs text-slate-500">KL 3 digit + UE1 2 digit + wilayah 4 digit, sama dengan 9 karakter pertama kode satker.</span>
            </label>
            <label className="block text-sm">
              <span className="mb-1.5 block font-medium text-slate-700">Uraian</span>
              <input
                value={form.nama}
                onChange={(e) => setForm({ ...form, nama: e.target.value })}
                maxLength={200}
                required
                className="w-full rounded-xl border border-slate-300 bg-white px-3.5 py-2.5 text-sm outline-none focus:border-blue-500 focus:shadow-glow"
                placeholder="mis. KANTOR WILAYAH DJP JAKARTA PUSAT"
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
                  placeholder="mis. KW DJP JKT PUSAT"
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
                <span className="block text-xs text-slate-500">Kanwil yang sudah tidak ada dapat dinonaktifkan; kodenya tetap diterjemahkan pada data lama.</span>
              </span>
            </label>
            <p className="rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-500">Setelah disimpan, baris ini bersumber <b>Manual</b> dan tidak akan ditimpa penarikan dari SLDK.</p>
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
          title={`Hapus Kanwil ${hapus.kode}?`}
          message={
            <>
              Referensi <b>{hapus.singkatan || hapus.nama}</b> dihapus. Data aset dan peran pengguna tidak berubah; kode {hapus.kode} kembali tampil sebagai &ldquo;Kanwil {hapus.kode}&rdquo; tanpa
              uraian (dan muncul lagi di daftar &ldquo;belum punya referensi&rdquo; bila ada di data aset). Untuk Kanwil yang sudah tidak dipakai, lebih baik <b>nonaktifkan</b>.
            </>
          }
          confirmLabel="Hapus"
          tone="danger"
          busy={menghapus}
          onConfirm={konfirmasiHapus}
          onCancel={() => !menghapus && setHapus(null)}
        />
      )}

      {konfirmasiTarik && (
        <ConfirmDialog
          title="Tarik referensi Kanwil dari SLDK?"
          message={
            <>
              Aplikasi membaca tabel referensi korwil SLDK (<span className="font-mono text-xs">DJKN.SIMAN2_R_KORWIL</span>, KL 015), menambahkan kode yang belum ada, dan memperbarui uraian baris
              bersumber <b>Data satker</b> atau <b>SLDK</b>. Baris yang Anda isi atau ubah <b>manual</b> tidak ditimpa. Tabelnya kecil, jadi tidak membebani SLDK.
            </>
          }
          confirmLabel="Tarik sekarang"
          tone="warning"
          busy={menarik}
          onConfirm={() => void tarik()}
          onCancel={() => !menarik && setKonfirmasiTarik(false)}
        />
      )}
    </div>
  );
}
