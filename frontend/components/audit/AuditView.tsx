"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import axios from "axios";
import { ChevronLeft, ChevronRight, Download, ListChecks, RefreshCw, Search, ShieldAlert, Users } from "lucide-react";
import {
  type DaftarAudit,
  type EntriAudit,
  type OpsiAudit,
  type ResAuditRingkasan,
  getAuditLog,
  getAuditOpsi,
  getAuditRingkasan,
  unduhAuditCsv,
} from "@/lib/audit";
import {
  FILTER_BAWAAN,
  type FilterAudit,
  PER_HALAMAN_AUDIT,
  RENTANG_AUDIT,
  type RentangKunci,
  filterBerubah,
  galatRentang,
  kelasHasil,
  kueriAudit,
  labelPeran,
  ringkasUserAgent,
  waktuWIB,
} from "@/lib/auditFilter";
import { adalahSuperadmin } from "@/lib/peran";
import { useDashboard } from "@/lib/dashboard-context";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { StatTile } from "@/components/ui/charts";
import { EmptyState } from "@/components/ui/EmptyState";
import { SkeletonCards, SkeletonTable } from "@/components/ui/Skeleton";
import { Tabs } from "@/components/ui/Tabs";
import { useToast } from "@/components/ui/Toast";
import { AuditDetail } from "./AuditDetail";
import { AuditPengguna } from "./AuditPengguna";
import { RingkasanAktivitas } from "./RingkasanAktivitas";
import { LencanaHasil } from "./lencana";
import { simpanBerkas } from "./unduh";

type Tab = "aktivitas" | "pengguna";

const TAB = [
  { key: "aktivitas" as const, label: "Aktivitas", icon: ListChecks },
  { key: "pengguna" as const, label: "Per pengguna", icon: Users },
];

const inputCls =
  "w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition-all placeholder:text-slate-400 hover:border-slate-400 focus:border-blue-500 focus:shadow-glow";

function pesanGalat(err: unknown, cadangan: string): string {
  return axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : cadangan;
}

// Log audit aktivitas pengguna: siapa melakukan apa, kapan, dari mana, dan hasilnya. Khusus superadmin (backend menolak peran lain dengan 403; halaman ini juga menolak
// menampilkan isinya). Hanya membaca: entri tidak bisa diubah atau dihapus dari aplikasi.
export function AuditView() {
  const { profile, isLoadingProfile } = useDashboard();
  const toast = useToast();
  const bolehLihat = adalahSuperadmin(profile?.peran, profile?.role);

  const [tab, setTab] = useState<Tab>("aktivitas");
  const [filter, setFilter] = useState<FilterAudit>(FILTER_BAWAAN);
  // Isian teks diketik bebas, lalu diterapkan ke filter setelah jeda supaya tidak meminta data pada setiap ketukan.
  const [teks, setTeks] = useState({ username: "", ip: "", q: "" });
  const [opsi, setOpsi] = useState<OpsiAudit | null>(null);
  const [daftar, setDaftar] = useState<DaftarAudit | null>(null);
  const [ringkasan, setRingkasan] = useState<ResAuditRingkasan | null>(null);
  const [galat, setGalat] = useState("");
  const [galatRingkasan, setGalatRingkasan] = useState("");
  const [memuat, setMemuat] = useState(false);
  const [terpilih, setTerpilih] = useState<EntriAudit | null>(null);
  const [mengekspor, setMengekspor] = useState(false);
  const [otomatis, setOtomatis] = useState(false);
  const [versi, setVersi] = useState(0); // dinaikkan untuk memuat ulang dengan filter yang sama
  const [sekarang, setSekarang] = useState(() => new Date());
  const pertama = useRef(true);

  const galatTanggal = galatRentang(filter);

  // Terapkan isian teks ke filter setelah jeda; kembali ke halaman 1.
  useEffect(() => {
    if (pertama.current) {
      pertama.current = false;
      return;
    }
    const t = setTimeout(() => setFilter((f) => (f.username === teks.username && f.ip === teks.ip && f.q === teks.q ? f : { ...f, ...teks, halaman: 1 })), 400);
    return () => clearTimeout(t);
  }, [teks]);

  useEffect(() => {
    if (!bolehLihat) return;
    getAuditOpsi().then((r) => setOpsi(r.data)).catch(() => setOpsi(null));
  }, [bolehLihat]);

  // Daftar entri: dimuat ulang saat filter berubah. Permintaan lama dibatalkan supaya jawaban yang telat tidak menimpa yang baru.
  useEffect(() => {
    if (!bolehLihat || tab !== "aktivitas" || galatTanggal) return;
    const ac = new AbortController();
    setMemuat(true);
    const now = new Date();
    setSekarang(now);
    getAuditLog(kueriAudit(filter, now, PER_HALAMAN_AUDIT), ac.signal)
      .then((r) => {
        setDaftar(r.data);
        setGalat("");
      })
      .catch((err) => {
        if (axios.isCancel(err)) return;
        setGalat(pesanGalat(err, "Gagal memuat log audit"));
      })
      .finally(() => {
        if (!ac.signal.aborted) setMemuat(false);
      });
    return () => ac.abort();
  }, [bolehLihat, tab, filter, galatTanggal, versi]);

  // Ringkasan hanya bergantung pada rentang waktu (bukan pada filter lain).
  useEffect(() => {
    if (!bolehLihat || galatTanggal) return;
    const ac = new AbortController();
    const k = kueriAudit({ ...FILTER_BAWAAN, rentang: filter.rentang, dari: filter.dari, sampai: filter.sampai }, new Date());
    if (filter.rentang === "semua") k.dari = "2000-01-01"; // ringkasan tanpa batas awal bawaannya 24 jam terakhir, jadi "semua waktu" harus menyebut batas awal yang jauh
    getAuditRingkasan(k, ac.signal)
      .then((r) => {
        setRingkasan(r.data);
        setGalatRingkasan("");
      })
      .catch((err) => {
        if (axios.isCancel(err)) return;
        setGalatRingkasan(pesanGalat(err, "Gagal memuat ringkasan"));
      });
    return () => ac.abort();
  }, [bolehLihat, filter.rentang, filter.dari, filter.sampai, galatTanggal, versi]);

  useEffect(() => {
    if (!bolehLihat || !otomatis) return;
    const t = setInterval(() => {
      if (!document.hidden) setVersi((v) => v + 1);
    }, 30_000);
    return () => clearInterval(t);
  }, [bolehLihat, otomatis]);

  const ubah = useCallback((p: Partial<FilterAudit>) => setFilter((f) => ({ ...f, ...p, halaman: p.halaman ?? 1 })), []);

  const aturUlang = () => {
    setFilter(FILTER_BAWAAN);
    setTeks({ username: "", ip: "", q: "" });
  };

  const lihatPengguna = (userId: string, username: string) => {
    setTerpilih(null);
    setTab("aktivitas");
    setTeks({ username: "", ip: "", q: "" });
    setFilter((f) => ({ ...FILTER_BAWAAN, rentang: f.rentang, dari: f.dari, sampai: f.sampai, user_id: userId, username: "", halaman: 1 }));
    toast.success(`Menampilkan aktivitas ${username || "pengguna"}`);
  };

  const ekspor = async () => {
    setMengekspor(true);
    try {
      const { blob, nama } = await unduhAuditCsv(kueriAudit(filter, new Date()));
      simpanBerkas(blob, nama);
      toast.success("Log audit diekspor ke CSV (maksimal 50.000 baris terbaru). Pengeksporan ini tercatat di log audit.");
      setVersi((v) => v + 1);
    } catch (err) {
      toast.error(pesanGalat(err, "Gagal mengekspor log audit"));
    } finally {
      setMengekspor(false);
    }
  };

  const jumlahHalaman = daftar ? Math.max(1, Math.ceil(daftar.total / daftar.per_halaman)) : 1;
  const berubah = filterBerubah(filter);
  const pengguna = useMemo(() => (filter.user_id ? daftar?.entri.find((e) => e.user_id === filter.user_id) : undefined), [filter.user_id, daftar]);

  if (isLoadingProfile) return <SkeletonCards count={3} />;
  if (!bolehLihat) return <Alert tone="warning" message="Log audit hanya dapat dibuka oleh superadmin." />;

  const r = ringkasan?.ringkasan;
  const perekam = ringkasan?.perekam;

  return (
    <div className="space-y-5">
      <p className="rounded-xl border border-blue-100 bg-blue-50/60 px-4 py-3 text-sm text-slate-700">
        Mencatat siapa mengubah data, mengekspor atau mengunduh berkas, mencari data pegawai, login (berhasil dan gagal), dan setiap akses yang ditolak. <b>Isi permintaan, kata sandi, dan token tidak disimpan.</b>{" "}
        Entri hanya bisa dibaca; tidak ada cara mengubah atau menghapusnya dari aplikasi. Waktu ditampilkan dalam WIB.
        {ringkasan && ringkasan.retensi_hari > 0 ? ` Entri disimpan ${ringkasan.retensi_hari} hari.` : ""}
      </p>

      {perekam && (perekam.dijatuhkan > 0 || perekam.gagal > 0) && (
        <Alert tone="warning" message={`${perekam.dijatuhkan + perekam.gagal} aktivitas tidak tercatat sejak aplikasi mulai (${perekam.dijatuhkan} karena antrean penuh, ${perekam.gagal} karena gagal menulis ke database). Periksa kesehatan database.`} />
      )}

      <div className="flex flex-wrap items-end gap-3">
        <div className="min-w-[11rem]">
          <label htmlFor="audit-rentang" className="mb-1 block text-xs font-medium text-slate-600">
            Rentang waktu
          </label>
          <select id="audit-rentang" value={filter.rentang} onChange={(e) => ubah({ rentang: e.target.value as RentangKunci })} className={inputCls}>
            {RENTANG_AUDIT.map((x) => (
              <option key={x.kunci} value={x.kunci}>
                {x.label}
              </option>
            ))}
          </select>
        </div>
        {filter.rentang === "kustom" && (
          <>
            <div>
              <label htmlFor="audit-dari" className="mb-1 block text-xs font-medium text-slate-600">
                Dari tanggal
              </label>
              <input id="audit-dari" type="date" value={filter.dari} max={filter.sampai || undefined} onChange={(e) => ubah({ dari: e.target.value })} className={inputCls} />
            </div>
            <div>
              <label htmlFor="audit-sampai" className="mb-1 block text-xs font-medium text-slate-600">
                Sampai tanggal
              </label>
              <input id="audit-sampai" type="date" value={filter.sampai} min={filter.dari || undefined} onChange={(e) => ubah({ sampai: e.target.value })} className={inputCls} />
            </div>
          </>
        )}
        <div className="ml-auto flex flex-wrap items-center gap-3">
          <label className="flex cursor-pointer items-center gap-2 text-xs text-slate-600">
            <input type="checkbox" checked={otomatis} onChange={(e) => setOtomatis(e.target.checked)} className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" />
            Segarkan otomatis (30 detik)
          </label>
          <Button size="sm" variant="secondary" fullWidth={false} icon={<RefreshCw className="h-4 w-4" aria-hidden="true" />} onClick={() => setVersi((v) => v + 1)}>
            Segarkan
          </Button>
          <Button size="sm" fullWidth={false} isLoading={mengekspor} disabled={Boolean(galatTanggal)} icon={<Download className="h-4 w-4" aria-hidden="true" />} onClick={() => void ekspor()}>
            Ekspor CSV
          </Button>
        </div>
      </div>
      {galatTanggal && <Alert tone="warning" message={galatTanggal} />}

      {galatRingkasan && <Alert message={galatRingkasan} />}
      {!r && !galatRingkasan && !galatTanggal && <SkeletonCards count={3} />}
      {r && (
        <>
          <div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6">
            <StatTile label="Aktivitas" value={r.total.toLocaleString("id-ID")} sub={`${r.berhasil.toLocaleString("id-ID")} berhasil · ${r.gagal.toLocaleString("id-ID")} gagal`} />
            <StatTile label="Pengguna aktif" value={r.pengguna_aktif.toLocaleString("id-ID")} sub="yang tercatat melakukan sesuatu" />
            <StatTile label="Login berhasil" value={r.login_berhasil.toLocaleString("id-ID")} />
            <StatTile label="Login gagal" value={r.login_gagal.toLocaleString("id-ID")} sub={r.login_gagal > 0 ? "lihat sumber di bawah" : undefined} />
            <StatTile label="Akses ditolak" value={r.ditolak.toLocaleString("id-ID")} sub="mencoba fitur di luar haknya" />
            <StatTile label="Ekspor / unduhan" value={r.ekspor.toLocaleString("id-ID")} />
          </div>
          <RingkasanAktivitas r={r} opsi={opsi} onKategori={(kode) => { setTab("aktivitas"); ubah({ kategori: kode, aksi: "" }); }} onAksi={(kode) => { setTab("aktivitas"); ubah({ aksi: kode, kategori: "" }); }} onIP={(ip) => { setTab("aktivitas"); setTeks((t) => ({ ...t, ip })); }} />
        </>
      )}

      <div className="flex flex-wrap items-center justify-between gap-3">
        <Tabs tabs={TAB} value={tab} onChange={setTab} label="Tampilan log audit" idPrefix="audit" />
      </div>

      {tab === "pengguna" && !galatTanggal && (
        <AuditPengguna
          filter={filter}
          versi={versi}
          onLihat={lihatPengguna}
        />
      )}

      {tab === "aktivitas" && (
        <>
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
            <div>
              <label htmlFor="audit-kategori" className="mb-1 block text-xs font-medium text-slate-600">
                Kategori
              </label>
              <select id="audit-kategori" value={filter.kategori} onChange={(e) => ubah({ kategori: e.target.value, aksi: "" })} className={inputCls}>
                <option value="">Semua kategori</option>
                {opsi?.kategori.map((k) => (
                  <option key={k.kode} value={k.kode}>
                    {k.label}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label htmlFor="audit-aksi" className="mb-1 block text-xs font-medium text-slate-600">
                Aksi
              </label>
              <select id="audit-aksi" value={filter.aksi} onChange={(e) => ubah({ aksi: e.target.value })} className={inputCls}>
                <option value="">Semua aksi</option>
                {opsi?.aksi
                  .filter((a) => !filter.kategori || a.kategori === filter.kategori)
                  .map((a) => (
                    <option key={a.kode} value={a.kode}>
                      {a.label} ({a.kode})
                    </option>
                  ))}
              </select>
            </div>
            <div>
              <label htmlFor="audit-hasil" className="mb-1 block text-xs font-medium text-slate-600">
                Hasil
              </label>
              <select id="audit-hasil" value={filter.hasil} onChange={(e) => ubah({ hasil: e.target.value as FilterAudit["hasil"] })} className={inputCls}>
                <option value="semua">Semua hasil</option>
                <option value="berhasil">Berhasil</option>
                <option value="gagal">Gagal atau ditolak</option>
              </select>
            </div>
            <div>
              <label htmlFor="audit-username" className="mb-1 block text-xs font-medium text-slate-600">
                Pengguna (username)
              </label>
              <input id="audit-username" type="search" value={teks.username} onChange={(e) => setTeks((t) => ({ ...t, username: e.target.value }))} maxLength={100} placeholder="mis. budi" className={inputCls} />
            </div>
            <div>
              <label htmlFor="audit-ip" className="mb-1 block text-xs font-medium text-slate-600">
                Alamat IP
              </label>
              <input id="audit-ip" type="search" value={teks.ip} onChange={(e) => setTeks((t) => ({ ...t, ip: e.target.value }))} maxLength={64} placeholder="mis. 10.1.2" className={inputCls} />
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <div className="relative min-w-[14rem] flex-1 sm:max-w-md">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
              <input
                type="search"
                value={teks.q}
                onChange={(e) => setTeks((t) => ({ ...t, q: e.target.value }))}
                maxLength={100}
                placeholder="Cari uraian, rute, objek, atau rincian"
                aria-label="Cari di log audit"
                className="w-full rounded-xl border border-slate-300 bg-white py-2.5 pl-9 pr-3 text-sm outline-none focus:border-blue-500 focus:shadow-glow"
              />
            </div>
            {filter.user_id && (
              <span className="inline-flex items-center gap-2 rounded-full bg-blue-50 px-3 py-1 text-xs font-medium text-blue-800 ring-1 ring-inset ring-blue-200">
                Hanya pengguna: {pengguna?.username || pengguna?.nama_lengkap || filter.user_id.slice(0, 8)}
                <button type="button" onClick={() => ubah({ user_id: "" })} className="font-semibold hover:underline" aria-label="Hapus filter pengguna">
                  ×
                </button>
              </span>
            )}
            {berubah && (
              <button type="button" onClick={aturUlang} className="text-sm font-medium text-blue-700 hover:underline">
                Atur ulang filter
              </button>
            )}
          </div>

          {galat && <Alert message={galat} />}
          {!daftar && !galat && !galatTanggal && <SkeletonTable rows={8} cols={6} />}
          {daftar && (
            <>
              <p className="text-sm text-slate-600" aria-live="polite">
                <span className="font-semibold text-slate-900">{daftar.total.toLocaleString("id-ID")}</span> aktivitas
                {memuat && <span className="ml-2 text-xs text-slate-400">memuat…</span>}
                {daftar.total > 0 && (
                  <span className="text-slate-400">
                    {" "}
                    · halaman {daftar.halaman} dari {jumlahHalaman.toLocaleString("id-ID")}
                  </span>
                )}
              </p>
              {daftar.entri.length === 0 ? (
                <EmptyState icon={ShieldAlert} title="Tidak ada aktivitas yang cocok" description={berubah ? "Ubah atau atur ulang filter." : "Belum ada aktivitas tercatat pada rentang waktu ini."} />
              ) : (
                <div className={`overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm ${memuat ? "opacity-70" : ""}`}>
                  <table className="w-full min-w-[56rem] text-sm">
                    <thead className="bg-slate-50 text-xs text-slate-500">
                      <tr>
                        <th scope="col" className="px-4 py-2.5 text-left font-medium">Waktu (WIB)</th>
                        <th scope="col" className="px-4 py-2.5 text-left font-medium">Pengguna</th>
                        <th scope="col" className="px-4 py-2.5 text-left font-medium">Aktivitas</th>
                        <th scope="col" className="px-4 py-2.5 text-left font-medium">Hasil</th>
                        <th scope="col" className="px-4 py-2.5 text-left font-medium">IP · perangkat</th>
                        <th scope="col" className="px-4 py-2.5 text-right font-medium">Lama</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100">
                      {daftar.entri.map((e) => (
                        <tr
                          key={e.id}
                          tabIndex={0}
                          role="button"
                          aria-label={`Lihat rincian: ${e.label}`}
                          onClick={() => setTerpilih(e)}
                          onKeyDown={(ev) => {
                            if (ev.key === "Enter" || ev.key === " ") {
                              ev.preventDefault();
                              setTerpilih(e);
                            }
                          }}
                          className="cursor-pointer outline-none hover:bg-slate-50 focus-visible:bg-blue-50/60"
                        >
                          <td className="whitespace-nowrap px-4 py-2.5 text-xs tabular-nums text-slate-600">{waktuWIB(e.waktu)}</td>
                          <td className="max-w-[14rem] px-4 py-2.5">
                            {e.username ? (
                              <>
                                <p className="truncate font-medium text-slate-900">{e.nama_lengkap || e.username}</p>
                                <p className="truncate text-xs text-slate-500">
                                  {e.nama_lengkap ? `${e.username} · ` : ""}
                                  {labelPeran(e.peran, e.kode_peran) || "tanpa peran"}
                                </p>
                              </>
                            ) : (
                              <span className="text-xs italic text-slate-400">tanpa pengguna</span>
                            )}
                          </td>
                          <td className="max-w-[26rem] px-4 py-2.5">
                            <p className="break-words text-slate-900">{e.label}</p>
                            <p className="truncate font-mono text-[11px] text-slate-400">{e.aksi}</p>
                          </td>
                          <td className="px-4 py-2.5">
                            <LencanaHasil kelas={kelasHasil(e)} />
                          </td>
                          <td className="whitespace-nowrap px-4 py-2.5 text-xs text-slate-600">
                            <p className="font-mono">{e.ip || "-"}</p>
                            <p className="text-slate-400">{ringkasUserAgent(e.user_agent)}</p>
                          </td>
                          <td className="whitespace-nowrap px-4 py-2.5 text-right text-xs tabular-nums text-slate-500">{e.durasi_ms} ms</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
              {daftar.total > daftar.per_halaman && (
                <nav aria-label="Halaman log audit" className="flex items-center justify-between gap-3">
                  <Button size="sm" variant="secondary" fullWidth={false} disabled={filter.halaman <= 1 || memuat} icon={<ChevronLeft className="h-4 w-4" aria-hidden="true" />} onClick={() => setFilter((f) => ({ ...f, halaman: f.halaman - 1 }))}>
                    Lebih baru
                  </Button>
                  <span className="text-xs text-slate-500">
                    Halaman {daftar.halaman} dari {jumlahHalaman.toLocaleString("id-ID")}
                  </span>
                  <Button size="sm" variant="secondary" fullWidth={false} disabled={filter.halaman >= jumlahHalaman || memuat} onClick={() => setFilter((f) => ({ ...f, halaman: f.halaman + 1 }))}>
                    Lebih lama
                    <ChevronRight className="h-4 w-4" aria-hidden="true" />
                  </Button>
                </nav>
              )}
            </>
          )}
          <p className="text-[11px] text-slate-400">Terakhir dimuat {waktuWIB(sekarang.toISOString())} WIB.</p>
        </>
      )}

      {terpilih && <AuditDetail e={terpilih} opsi={opsi} onTutup={() => setTerpilih(null)} onLihatPengguna={lihatPengguna} />}
    </div>
  );
}
