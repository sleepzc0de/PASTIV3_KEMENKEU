"use client";

import { useMemo, useState } from "react";
import { CalendarClock, Loader2, Save } from "lucide-react";
import { PenarikanOtomatis, PenarikanPengaturan, PenarikanStatus, savePenarikanPengaturan } from "@/lib/api";
import { formatAngka, formatWaktu, jam, kalimatJadwal, kalimatKebijakan, kelompokkanDataset, waktuRelatif } from "@/lib/pengadaan";
import { Alert } from "@/components/ui/Alert";
import { useToast } from "@/components/ui/Toast";
import { errorMessage } from "../digitalisasi/useDigitalisasi";
import { KELAS_INPUT } from "./lencana";

type Form = Omit<PenarikanPengaturan, "diubah" | "diubah_oleh" | "bawaan_server">;

const JAM = Array.from({ length: 24 }, (_, i) => i);

function dariPengaturan(p: PenarikanPengaturan): Form {
  const { diubah: _d, diubah_oleh: _o, bawaan_server: _b, ...form } = p;
  return { ...form, dataset: [...p.dataset] };
}

// Pengaturan penarikan otomatis (bawaan: tiap 2 hari). Superadmin bisa mengubah; pengguna lain hanya membaca.
export function JadwalPanel({
  status,
  isAdmin,
  onDisimpan,
}: {
  status: PenarikanStatus;
  isAdmin: boolean;
  onDisimpan: (p: PenarikanPengaturan, o: PenarikanOtomatis) => void;
}) {
  const toast = useToast();
  const { pengaturan, otomatis } = status;
  const [form, setForm] = useState<Form>(() => dariPengaturan(pengaturan));
  const [sibuk, setSibuk] = useState(false);
  const [galat, setGalat] = useState("");

  const kelompok = useMemo(() => kelompokkanDataset(status.datasets.filter((d) => d.otomatis), status.kelompok), [status.datasets, status.kelompok]);
  const idOtomatis = useMemo(() => status.datasets.filter((d) => d.otomatis).map((d) => d.id), [status.datasets]);
  const terpilih = new Set(form.dataset);
  const berubah = JSON.stringify(form) !== JSON.stringify(dariPengaturan(pengaturan));
  const baca = !isAdmin;

  const set = <K extends keyof Form>(k: K, v: Form[K]) => setForm((f) => ({ ...f, [k]: v }));
  const toggle = (id: string) => set("dataset", terpilih.has(id) ? form.dataset.filter((x) => x !== id) : [...form.dataset, id]);

  const simpan = async () => {
    setSibuk(true);
    setGalat("");
    try {
      const res = await savePenarikanPengaturan(form);
      onDisimpan(res.data.pengaturan, res.data.otomatis);
      setForm(dariPengaturan(res.data.pengaturan));
      toast.success("Pengaturan penarikan otomatis disimpan.");
    } catch (err) {
      setGalat(errorMessage(err, "Gagal menyimpan pengaturan"));
    } finally {
      setSibuk(false);
    }
  };

  const tugasPerPutaran = useMemo(() => {
    // Perkiraan jumlah tugas per putaran dari pilihan di formulir: dataset per KLPD+tahun dan transaksi dikali tahun, sisanya sekali.
    let n = 0;
    for (const d of status.datasets) {
      if (!terpilih.has(d.id)) continue;
      n += d.mode === "klpd_tahun" || d.mode === "transaksi" ? form.jumlah_tahun : 1;
    }
    return n;
  }, [status.datasets, form.dataset, form.jumlah_tahun]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="space-y-5">
      <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm sm:p-5">
        <div className="flex items-start gap-3">
          <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600">
            <CalendarClock className="h-5 w-5" aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <h3 className="text-sm font-semibold text-slate-900">Penarikan otomatis</h3>
            <p className="mt-0.5 text-sm text-slate-600">{kalimatJadwal(otomatis, pengaturan)}</p>
            <p className="mt-1 text-xs text-slate-500">
              {otomatis.terakhir_otomatis ? `Penarikan otomatis berhasil terakhir ${waktuRelatif(otomatis.terakhir_otomatis)} (${formatWaktu(otomatis.terakhir_otomatis)}). ` : "Belum pernah ada penarikan otomatis yang berhasil. "}
              {otomatis.jatuh_tempo > 0 ? `${formatAngka(otomatis.jatuh_tempo)} dari ${formatAngka(otomatis.jumlah_tugas)} tugas sudah jatuh tempo (percobaan ulang langsung berjalan; yang reguler menunggu jendela jam).` : `${formatAngka(otomatis.jumlah_tugas)} tugas per putaran, semuanya masih segar.`}
              {pengaturan.bawaan_server ? " Pengaturan masih memakai nilai bawaan server." : pengaturan.diubah ? ` Terakhir diubah ${formatWaktu(pengaturan.diubah)}${isAdmin && pengaturan.diubah_oleh ? ` oleh ${pengaturan.diubah_oleh}` : ""}.` : ""}
            </p>
          </div>
        </div>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm sm:p-5">
        <h3 className="text-sm font-semibold text-slate-900">Bila penarikan gagal</h3>
        <p className="mt-1 text-sm text-slate-600">{kalimatKebijakan(otomatis.maks_percobaan, otomatis.istirahat_jam)}</p>
        <ul className="mt-2 list-disc space-y-1 pl-5 text-xs text-slate-500">
          <li>Percobaan ulang tidak menunggu jendela jam, tetapi tetap lewat antrean tunggal: tidak pernah dua penarikan berjalan bersamaan.</li>
          <li>Hanya yang berstatus gagal dihitung. Dibatalkan, dilewati, dan terhenti karena server dimulai ulang tidak menghabiskan kesempatan; satu penarikan sukses menutup siklus.</li>
          <li>Bila banyak tugas gagal berturut-turut (Inaproc atau jaringan bermasalah), antrean berhenti dan penarikan otomatis ditahan sebentar supaya kesempatan tidak habis percuma.</li>
          <li>Batas permintaan Inaproc (1.000 per 60 detik, 5.000 per jam) dijaga otomatis; bila jatah habis, penarikan menunggu, bukan gagal.</li>
        </ul>
        {otomatis.istirahat > 0 && <p className="mt-2 text-xs font-medium text-amber-700">{formatAngka(otomatis.istirahat)} tugas sedang istirahat saat ini.</p>}
        {otomatis.ditahan_sampai && <p className="mt-1 text-xs font-medium text-amber-700">Penarikan otomatis ditahan sementara sampai {formatWaktu(otomatis.ditahan_sampai)}.</p>}
      </section>

      {baca && <Alert tone="info" message="Hanya superadmin yang dapat mengubah pengaturan penarikan otomatis." />}
      {galat && <Alert message={galat} />}

      <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm sm:p-5">
        <h3 className="text-sm font-semibold text-slate-900">Jadwal</h3>
        <label className="mt-3 inline-flex cursor-pointer items-center gap-3">
          <input type="checkbox" role="switch" checked={form.aktif} disabled={baca} onChange={(e) => set("aktif", e.target.checked)} className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" />
          <span className="text-sm text-slate-800">Aktifkan penarikan otomatis</span>
        </label>

        <div className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div>
            <label htmlFor="jd-interval" className="mb-1 block text-xs font-medium text-slate-600">
              Diulang setiap
            </label>
            <div className="flex items-center gap-2">
              <input id="jd-interval" type="number" min={1} max={30} value={form.interval_hari} disabled={baca} onChange={(e) => set("interval_hari", Number(e.target.value))} className={KELAS_INPUT} />
              <span className="shrink-0 text-sm text-slate-600">hari</span>
            </div>
            <p className="mt-1 text-xs text-slate-500">Bawaan 2 hari. Maksimal 30.</p>
          </div>
          <div>
            <label htmlFor="jd-mulai" className="mb-1 block text-xs font-medium text-slate-600">
              Mulai paling awal ({otomatis.zona})
            </label>
            <select id="jd-mulai" value={form.jam_mulai} disabled={baca} onChange={(e) => set("jam_mulai", Number(e.target.value))} className={KELAS_INPUT}>
              {JAM.map((j) => (
                <option key={j} value={j}>
                  {jam(j)}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label htmlFor="jd-akhir" className="mb-1 block text-xs font-medium text-slate-600">
              Batas mulai ({otomatis.zona})
            </label>
            <select id="jd-akhir" value={form.jam_akhir} disabled={baca} onChange={(e) => set("jam_akhir", Number(e.target.value))} className={KELAS_INPUT}>
              {JAM.map((j) => (
                <option key={j} value={j}>
                  {jam(j)}
                </option>
              ))}
            </select>
            <p className="mt-1 text-xs text-slate-500">Jam sama = kapan saja. Penarikan yang sudah berjalan boleh melewati batas ini.</p>
          </div>
          <div>
            <label htmlFor="jd-jeda" className="mb-1 block text-xs font-medium text-slate-600">
              Jeda antar tugas
            </label>
            <div className="flex items-center gap-2">
              <input id="jd-jeda" type="number" min={0} max={60} value={form.jeda_detik} disabled={baca} onChange={(e) => set("jeda_detik", Number(e.target.value))} className={KELAS_INPUT} />
              <span className="shrink-0 text-sm text-slate-600">detik</span>
            </div>
            <p className="mt-1 text-xs text-slate-500">Mengurangi risiko ditolak batas laju Inaproc (429).</p>
          </div>
        </div>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm sm:p-5">
        <h3 className="text-sm font-semibold text-slate-900">Cakupan</h3>
        <div className="mt-3 grid gap-4 sm:grid-cols-2">
          <div>
            <label htmlFor="jd-klpd" className="mb-1 block text-xs font-medium text-slate-600">
              Kode KLPD
            </label>
            <input id="jd-klpd" value={form.kode_klpd} maxLength={20} disabled={baca} onChange={(e) => set("kode_klpd", e.target.value)} className={KELAS_INPUT} />
          </div>
          <div>
            <label htmlFor="jd-tahun" className="mb-1 block text-xs font-medium text-slate-600">
              Tahun yang ditarik
            </label>
            <select id="jd-tahun" value={form.jumlah_tahun} disabled={baca} onChange={(e) => set("jumlah_tahun", Number(e.target.value))} className={KELAS_INPUT}>
              {[1, 2, 3, 4, 5].map((n) => (
                <option key={n} value={n}>
                  {n === 1 ? "Tahun berjalan saja" : `Tahun berjalan dan ${n - 1} tahun sebelumnya`}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="mt-5 flex flex-wrap items-center justify-between gap-2">
          <p className="text-sm font-medium text-slate-800">
            Dataset yang ikut <span className="font-normal text-slate-500">({form.dataset.length} dipilih · sekitar {formatAngka(tugasPerPutaran)} tugas per putaran)</span>
          </p>
          {!baca && (
            <div className="flex gap-1.5 text-xs">
              <button type="button" onClick={() => set("dataset", idOtomatis)} className="rounded-md border border-slate-300 bg-white px-2.5 py-1 font-medium text-slate-700 hover:bg-slate-50">
                Pilih semua
              </button>
              <button type="button" onClick={() => set("dataset", [])} className="rounded-md px-2.5 py-1 font-medium text-slate-500 hover:bg-slate-100">
                Kosongkan
              </button>
            </div>
          )}
        </div>
        <p className="mt-1 text-xs text-slate-500">Dataset rujukan per kode (penyedia, komoditas, distributor, produk penyedia) tidak bisa otomatis karena Inaproc tidak menyediakan daftar &quot;semua&quot;; tarik manual dengan kodenya.</p>

        <div className="mt-3 grid gap-4 lg:grid-cols-2">
          {kelompok.map((k) => (
            <fieldset key={k.id} className="rounded-xl border border-slate-200 p-3">
              <legend className="px-1 text-xs font-semibold text-slate-700">{k.nama}</legend>
              <ul className="space-y-1.5">
                {k.subkelompok.flatMap((s) => s.datasets).map((d) => (
                  <li key={d.id}>
                    <label className="flex cursor-pointer items-start gap-2 text-sm text-slate-700">
                      <input type="checkbox" checked={terpilih.has(d.id)} disabled={baca} onChange={() => toggle(d.id)} className="mt-0.5 h-4 w-4 shrink-0 rounded border-slate-300 text-blue-600 focus:ring-blue-500" />
                      <span className="min-w-0">{d.nama}</span>
                    </label>
                  </li>
                ))}
              </ul>
            </fieldset>
          ))}
        </div>

        {!baca && (
          <div className="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-slate-100 pt-4">
            <p className="text-xs text-slate-500">Perubahan berlaku pada pemeriksaan jadwal berikutnya (tiap 15 menit); server tidak perlu dimulai ulang.</p>
            <div className="flex gap-2">
              <button type="button" onClick={() => setForm(dariPengaturan(pengaturan))} disabled={!berubah || sibuk} className="rounded-lg px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100 disabled:opacity-50">
                Batalkan perubahan
              </button>
              <button type="button" onClick={simpan} disabled={!berubah || sibuk || (form.aktif && form.dataset.length === 0)} className="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-55">
                {sibuk ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" /> : <Save className="h-4 w-4" aria-hidden="true" />}
                Simpan pengaturan
              </button>
            </div>
          </div>
        )}
        {!baca && form.aktif && form.dataset.length === 0 && <p className="mt-2 text-xs text-amber-700">Pilih minimal satu dataset, atau matikan penarikan otomatis.</p>}
      </section>
    </div>
  );
}
