"use client";

import { useMemo, useState } from "react";
import { AlertTriangle, ChevronDown, Play, Plus } from "lucide-react";
import { PenarikanAktif, PenarikanDataset, PenarikanKuota, PenarikanStatus, startPenarikan } from "@/lib/api";
import {
  PilihanTarik,
  STATUS_TRANSAKSI,
  bangunPermintaan,
  daftarTahun,
  formatAngka,
  kelompokkanDataset,
  kesegaranDataset,
  kalimatJadwal,
  tahunValid,
  waktuRelatif,
} from "@/lib/pengadaan";
import { Alert } from "@/components/ui/Alert";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { useToast } from "@/components/ui/Toast";
import { errorMessage } from "../digitalisasi/useDigitalisasi";
import { KartuKuota, PanelBermasalah } from "./KuotaDanKegagalan";
import { KELAS_INPUT, LencanaKesegaran } from "./lencana";

interface Props {
  status: PenarikanStatus;
  aktif: PenarikanAktif | null;
  kuota: PenarikanKuota;
  isAdmin: boolean;
  onMulai: (a: PenarikanAktif) => void;
}

// Pilih dataset, KLPD, dan tahun, lalu tarik dari Inaproc. Dataset per KLPD+tahun dikali tahun terpilih; dataset rujukan per kode butuh kode.
export function TarikPanel({ status, aktif, kuota, isAdmin, onMulai }: Props) {
  const toast = useToast();
  const [klpd, setKlpd] = useState(status.kode_klpd);
  const [tahun, setTahun] = useState<string[]>(status.tahun_bawaan);
  const [tahunBaru, setTahunBaru] = useState("");
  const [dataset, setDataset] = useState<string[]>([]);
  const [kode, setKode] = useState<Record<string, string>>({});
  const [opsi, setOpsi] = useState<PilihanTarik["opsi"]>({});
  const [buka, setBuka] = useState<Record<string, boolean>>({ pengadaan: true });
  const [konfirmasi, setKonfirmasi] = useState(false);
  const [sibuk, setSibuk] = useState(false);
  const [galat, setGalat] = useState("");

  const kelompok = useMemo(() => kelompokkanDataset(status.datasets, status.kelompok), [status.datasets, status.kelompok]);
  const pilihan: PilihanTarik = { kodeKLPD: klpd, tahun, dataset, kode, opsi };
  const { permintaan, jumlahTugas, masalah } = useMemo(() => bangunPermintaan(status.datasets, pilihan), [status.datasets, klpd, tahun, dataset, kode, opsi]); // eslint-disable-line react-hooks/exhaustive-deps
  const terpilih = new Set(dataset);

  const pilihTahun = useMemo(() => {
    const dasar = daftarTahun(status.tahun_ini, 6);
    return Array.from(new Set([...dasar, ...tahun])).sort((a, b) => Number(b) - Number(a));
  }, [status.tahun_ini, tahun]);

  const bisaMulai = isAdmin && status.token_ada && !aktif && jumlahTugas > 0 && masalah.length === 0 && klpd.trim() !== "";

  const toggleDataset = (id: string) => setDataset((cur) => (cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id]));
  const toggleKelompok = (ds: PenarikanDataset[], nyala: boolean) => {
    const ids = ds.filter((d) => d.mode !== "kode").map((d) => d.id);
    setDataset((cur) => (nyala ? Array.from(new Set([...cur, ...ids])) : cur.filter((x) => !ids.includes(x))));
  };
  const setOpsiDataset = (id: string, o: Partial<PilihanTarik["opsi"][string]>) => setOpsi((cur) => ({ ...cur, [id]: { ...cur[id], ...o } }));

  const tambahTahun = () => {
    const t = tahunBaru.trim();
    if (!tahunValid(t)) return;
    setTahun((cur) => (cur.includes(t) ? cur : [...cur, t]));
    setTahunBaru("");
  };

  const mulai = async () => {
    setSibuk(true);
    setGalat("");
    try {
      const res = await startPenarikan(permintaan);
      onMulai(res.data.aktif);
      toast.success(`Penarikan dimulai: ${res.data.aktif.total} tugas.`);
      setKonfirmasi(false);
    } catch (err) {
      setGalat(errorMessage(err, "Gagal memulai penarikan"));
      setKonfirmasi(false);
    } finally {
      setSibuk(false);
    }
  };

  return (
    <div className="space-y-5">
      {!status.token_ada && <Alert tone="warning" message="Token Inaproc belum dikonfigurasi di server (INAPROC_TOKEN), jadi penarikan tidak bisa dijalankan." />}
      {!isAdmin && <Alert tone="info" message="Hanya admin yang dapat menjalankan penarikan. Anda tetap dapat melihat status data, riwayat, dasbor, dan mengekspor data." />}
      {galat && <Alert message={galat} />}

      <PanelBermasalah item={status.bermasalah} otomatis={status.otomatis} isAdmin={isAdmin} />
      <KartuKuota kuota={kuota} />

      <section className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm sm:p-5">
        <h3 className="text-sm font-semibold text-slate-900">Parameter penarikan</h3>
        <div className="mt-3 grid gap-4 sm:grid-cols-[12rem_minmax(0,1fr)]">
          <div>
            <label htmlFor="tarik-klpd" className="mb-1 block text-xs font-medium text-slate-600">
              Kode KLPD
            </label>
            <input id="tarik-klpd" value={klpd} onChange={(e) => setKlpd(e.target.value)} maxLength={20} className={KELAS_INPUT} placeholder="K10" disabled={!isAdmin} />
          </div>
          <div>
            <span id="tarik-tahun" className="mb-1 block text-xs font-medium text-slate-600">
              Tahun anggaran
            </span>
            <div className="flex flex-wrap items-center gap-1.5" role="group" aria-labelledby="tarik-tahun">
              {pilihTahun.map((t) => {
                const nyala = tahun.includes(t);
                return (
                  <button
                    key={t}
                    type="button"
                    aria-pressed={nyala}
                    disabled={!isAdmin}
                    onClick={() => setTahun((cur) => (nyala ? cur.filter((x) => x !== t) : [...cur, t]))}
                    className={`rounded-full border px-3 py-1 text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-60 ${
                      nyala ? "border-blue-600 bg-blue-600 text-white" : "border-slate-300 bg-white text-slate-600 hover:border-slate-400"
                    }`}
                  >
                    {t}
                  </button>
                );
              })}
              {isAdmin && (
                <span className="inline-flex items-center gap-1">
                  <input
                    value={tahunBaru}
                    onChange={(e) => setTahunBaru(e.target.value.replace(/\D/g, "").slice(0, 4))}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") {
                        e.preventDefault();
                        tambahTahun();
                      }
                    }}
                    inputMode="numeric"
                    placeholder="Tahun lain"
                    aria-label="Tambah tahun lain"
                    className="w-24 rounded-full border border-slate-300 px-3 py-1 text-xs outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20"
                  />
                  <button type="button" onClick={tambahTahun} disabled={!tahunValid(tahunBaru)} aria-label="Tambahkan tahun" className="rounded-full p-1.5 text-slate-500 hover:bg-slate-100 disabled:opacity-40">
                    <Plus className="h-3.5 w-3.5" />
                  </button>
                </span>
              )}
            </div>
            <p className="mt-1.5 text-xs text-slate-500">Dataset RUP, Tender, dan E-Purchasing ditarik per tahun terpilih. Dataset rujukan memakai KLPD saja atau kode yang Anda isi.</p>
          </div>
        </div>
      </section>

      <section className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h3 className="text-sm font-semibold text-slate-900">Dataset</h3>
            <p className="text-xs text-slate-500">
              {status.datasets.length} dataset dari Pengadaan, Tender, E-Katalog V5, dan E-Katalog V6. Data lama untuk KLPD dan tahun yang sama diganti hanya bila penarikannya berhasil.
            </p>
          </div>
          {isAdmin && (
            <div className="flex flex-wrap gap-1.5 text-xs">
              <button
                type="button"
                onClick={() => setDataset(status.datasets.filter((d) => d.otomatis).map((d) => d.id))}
                className="rounded-md border border-slate-300 bg-white px-2.5 py-1 font-medium text-slate-700 hover:bg-slate-50"
              >
                Pilih semua (tanpa per kode)
              </button>
              <button type="button" onClick={() => setDataset([])} className="rounded-md px-2.5 py-1 font-medium text-slate-500 hover:bg-slate-100">
                Kosongkan
              </button>
            </div>
          )}
        </div>

        {kelompok.map((k) => {
          const semua = k.subkelompok.flatMap((s) => s.datasets);
          const bisa = semua.filter((d) => d.mode !== "kode");
          const jumlahTerpilih = semua.filter((d) => terpilih.has(d.id)).length;
          const penuh = bisa.length > 0 && bisa.every((d) => terpilih.has(d.id));
          const terbuka = buka[k.id] ?? false;
          return (
            <div key={k.id} className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
              <div className="flex items-center gap-3 px-4 py-3">
                {isAdmin && (
                  <input
                    type="checkbox"
                    aria-label={`Pilih semua dataset ${k.nama}`}
                    checked={penuh}
                    ref={(el) => {
                      if (el) el.indeterminate = !penuh && bisa.some((d) => terpilih.has(d.id));
                    }}
                    onChange={(e) => toggleKelompok(semua, e.target.checked)}
                    className="h-4 w-4 shrink-0 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
                  />
                )}
                <button
                  type="button"
                  onClick={() => setBuka((cur) => ({ ...cur, [k.id]: !terbuka }))}
                  aria-expanded={terbuka}
                  className="flex min-w-0 flex-1 items-center justify-between gap-3 text-left"
                >
                  <span className="min-w-0">
                    <span className="block text-sm font-semibold text-slate-900">{k.nama}</span>
                    <span className="block text-xs text-slate-500">
                      {semua.length} dataset{jumlahTerpilih > 0 ? ` · ${jumlahTerpilih} dipilih` : ""}
                    </span>
                  </span>
                  <ChevronDown className={`h-4 w-4 shrink-0 text-slate-400 transition-transform ${terbuka ? "rotate-180" : ""}`} aria-hidden="true" />
                </button>
              </div>
              {terbuka && (
                <div className="border-t border-slate-100">
                  {k.subkelompok.map((s) => (
                    <div key={s.nama}>
                      <p className="bg-slate-50 px-4 py-1.5 text-[11px] font-semibold uppercase tracking-wide text-slate-500">{s.nama}</p>
                      <ul className="divide-y divide-slate-100">
                        {s.datasets.map((d) => (
                          <BarisDataset
                            key={d.id}
                            d={d}
                            dipilih={terpilih.has(d.id)}
                            isAdmin={isAdmin}
                            kode={kode[d.id] ?? ""}
                            opsi={opsi[d.id] ?? {}}
                            onToggle={() => toggleDataset(d.id)}
                            onKode={(v) => setKode((cur) => ({ ...cur, [d.id]: v }))}
                            onOpsi={(o) => setOpsiDataset(d.id, o)}
                          />
                        ))}
                      </ul>
                    </div>
                  ))}
                </div>
              )}
            </div>
          );
        })}
      </section>

      {isAdmin && (
        <div className="sticky bottom-3 z-10 rounded-2xl border border-slate-200 bg-white/95 p-3 shadow-lg backdrop-blur sm:p-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="min-w-0 text-sm">
              {jumlahTugas > 0 ? (
                <p className="font-medium text-slate-900">
                  {jumlahTugas} tugas siap ditarik <span className="font-normal text-slate-500">({dataset.length} dataset)</span>
                </p>
              ) : (
                <p className="text-slate-500">Pilih dataset untuk memulai.</p>
              )}
              {masalah.length > 0 && (
                <p className="mt-0.5 flex items-start gap-1.5 text-xs text-amber-700">
                  <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                  <span>{masalah[0]}{masalah.length > 1 ? ` (+${masalah.length - 1} lainnya)` : ""}</span>
                </p>
              )}
              {aktif && <p className="mt-0.5 text-xs text-slate-500">Menunggu penarikan yang sedang berjalan selesai.</p>}
            </div>
            <button
              type="button"
              disabled={!bisaMulai}
              onClick={() => setKonfirmasi(true)}
              className="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-55"
            >
              <Play className="h-4 w-4" aria-hidden="true" />
              Mulai penarikan
            </button>
          </div>
        </div>
      )}

      <p className="text-xs text-slate-500">{kalimatJadwal(status.otomatis, status.pengaturan)}</p>

      {konfirmasi && (
        <ConfirmDialog
          title="Mulai penarikan dari Inaproc?"
          tone="warning"
          confirmLabel="Mulai penarikan"
          cancelLabel="Kembali"
          busy={sibuk}
          onConfirm={mulai}
          onCancel={() => setKonfirmasi(false)}
          message={
            <div className="space-y-2">
              <p>
                {jumlahTugas} tugas akan ditarik dari {dataset.length} dataset untuk KLPD <span className="font-medium text-slate-900">{klpd.trim()}</span>
                {tahun.length > 0 ? `, tahun ${[...tahun].sort().join(", ")}` : ""}.
              </p>
              <p className="text-xs text-slate-500">
                Penarikan berjalan di server satu tugas demi satu tugas dan bisa dibatalkan. Inaproc membatasi laju permintaan: bila ditolak (429), tugas dicoba ulang otomatis setelah jeda. Data lama diganti hanya
                setelah seluruh halaman dari Inaproc berhasil diambil.
              </p>
            </div>
          }
        />
      )}
    </div>
  );
}

function BarisDataset({
  d,
  dipilih,
  isAdmin,
  kode,
  opsi,
  onToggle,
  onKode,
  onOpsi,
}: {
  d: PenarikanDataset;
  dipilih: boolean;
  isAdmin: boolean;
  kode: string;
  opsi: { status?: string; kd1?: string; kd2?: string };
  onToggle: () => void;
  onKode: (v: string) => void;
  onOpsi: (o: { status?: string; kd1?: string; kd2?: string }) => void;
}) {
  const keadaan = kesegaranDataset(d);
  const terakhir = d.terakhir_sukses?.selesai ?? d.disinkron_at;
  return (
    <li className="px-4 py-3">
      <div className="flex items-start gap-3">
        {isAdmin && (
          <input type="checkbox" aria-label={`Pilih ${d.nama}`} checked={dipilih} onChange={onToggle} className="mt-1 h-4 w-4 shrink-0 rounded border-slate-300 text-blue-600 focus:ring-blue-500" />
        )}
        <div className="min-w-0 flex-1 space-y-1">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <p className="text-sm font-medium text-slate-900">{d.nama}</p>
            <LencanaKesegaran keadaan={keadaan} />
            {d.mode === "kode" && <span className="rounded-full bg-violet-50 px-2 py-0.5 text-[11px] font-medium text-violet-700">Per kode</span>}
            {!d.otomatis && <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-600">Hanya manual</span>}
          </div>
          <p className="text-xs text-slate-500">{d.deskripsi}</p>
          <p className="text-xs text-slate-500">
            {formatAngka(d.baris)} baris tersimpan{terakhir ? ` · terakhir ${waktuRelatif(terakhir)}` : ""}
            {d.terakhir && d.terakhir.status === "gagal" && d.terakhir.pesan ? <span className="text-red-600"> · {d.terakhir.pesan}</span> : null}
          </p>

          {dipilih && d.mode === "kode" && (
            <div className="pt-1">
              <label htmlFor={`kode-${d.id}`} className="mb-1 block text-xs font-medium text-slate-600">
                Kode yang dicari (pisahkan dengan spasi, koma, atau baris baru)
              </label>
              <textarea id={`kode-${d.id}`} value={kode} onChange={(e) => onKode(e.target.value)} rows={2} className={KELAS_INPUT} placeholder="mis. kode penyedia atau kode komoditas" />
            </div>
          )}
          {dipilih && d.mode === "kategori" && (
            <div className="grid gap-2 pt-1 sm:grid-cols-2">
              <div>
                <label htmlFor={`kd1-${d.id}`} className="mb-1 block text-xs font-medium text-slate-600">
                  Kode kategori tingkat 1 (kosong = daftar tingkat 1)
                </label>
                <input id={`kd1-${d.id}`} value={opsi.kd1 ?? ""} onChange={(e) => onOpsi({ kd1: e.target.value })} className={KELAS_INPUT} />
              </div>
              <div>
                <label htmlFor={`kd2-${d.id}`} className="mb-1 block text-xs font-medium text-slate-600">
                  Kode kategori tingkat 2 (opsional)
                </label>
                <input id={`kd2-${d.id}`} value={opsi.kd2 ?? ""} onChange={(e) => onOpsi({ kd2: e.target.value })} className={KELAS_INPUT} />
              </div>
            </div>
          )}
          {dipilih && d.perlu_status && (
            <div className="max-w-xs pt-1">
              <label htmlFor={`status-${d.id}`} className="mb-1 block text-xs font-medium text-slate-600">
                Status {d.mode === "transaksi" ? "transaksi (bawaan COMPLETED)" : "(opsional; kosong = semua status)"}
              </label>
              <input id={`status-${d.id}`} list={d.mode === "transaksi" ? "daftar-status-transaksi" : undefined} value={opsi.status ?? ""} onChange={(e) => onOpsi({ status: e.target.value })} className={KELAS_INPUT} />
            </div>
          )}
        </div>
      </div>
    </li>
  );
}

// Daftar pilihan status transaksi untuk isian status (dipasang sekali di halaman).
export function DaftarStatusTransaksi() {
  return (
    <datalist id="daftar-status-transaksi">
      {STATUS_TRANSAKSI.map((s) => (
        <option key={s} value={s} />
      ))}
    </datalist>
  );
}
