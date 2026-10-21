"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import axios from "axios";
import { useSearchParams } from "next/navigation";
import { Activity, ChevronLeft, ChevronRight, RefreshCw, Search } from "lucide-react";
import { type DaftarRiwayat, type Kelompok, type OpsiPantau, type Pemantauan, type RiwayatPermintaan, getOpsiPantau, getPantau, getRiwayat } from "@/lib/apiAkses";
import {
  FILTER_PANTAU_BAWAAN,
  type FilterPantau,
  RENTANG_PANTAU,
  type RentangPantau,
  formatJumlah,
  formatMsAkses,
  galatRentangPantau,
  jumlahDibatasi,
  jumlahGalat,
  kueriPantau,
  teksBatas,
  teksPersen,
} from "@/lib/apiAksesFormat";
import { teksRelatif, waktuWIB } from "@/lib/auditFilter";
import { adalahSuperadmin } from "@/lib/peran";
import { useDashboard } from "@/lib/dashboard-context";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { ChartCard, DataTable, StatTile } from "@/components/ui/charts";
import { DetailEntry } from "@/components/ui/DetailEntry";
import { EmptyState } from "@/components/ui/EmptyState";
import { GrafikGaris } from "@/components/ui/GrafikGaris";
import { ModalShell } from "@/components/ui/ModalShell";
import { SkeletonCards, SkeletonTable } from "@/components/ui/Skeleton";
import { BatangKuota, LencanaHasilAkses } from "./lencana";

const inputCls =
  "w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition-all placeholder:text-slate-400 hover:border-slate-400 focus:border-blue-500 focus:shadow-glow";

const PER_HALAMAN = 25;

function pesanGalat(err: unknown, cadangan: string): string {
  return axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : cadangan;
}

const WARNA = { jumlah: "#3358e0", gagal: "#d03b3b", dibatasi: "#fab219", latensi: "#0f766e" };

// Pemantauan penarikan data oleh klien API (khusus superadmin, hanya membaca): jumlah permintaan menurut klien/endpoint/waktu, pemakaian kuota saat ini, galat dan 429,
// latensi, dan riwayat permintaan terakhir.
export function MonitoringView() {
  const { profile, isLoadingProfile } = useDashboard();
  const bolehLihat = adalahSuperadmin(profile?.peran, profile?.role);
  const params = useSearchParams();

  const [filter, setFilter] = useState<FilterPantau>(() => ({ ...FILTER_PANTAU_BAWAAN, klien_id: /^\d+$/.test(params.get("klien_id") ?? "") ? (params.get("klien_id") as string) : "" }));
  const [opsi, setOpsi] = useState<OpsiPantau | null>(null);
  const [data, setData] = useState<Pemantauan | null>(null);
  const [galat, setGalat] = useState("");
  const [memuat, setMemuat] = useState(false);
  const [otomatis, setOtomatis] = useState(false);
  const [versi, setVersi] = useState(0);

  const [hasil, setHasil] = useState("");
  const [ip, setIp] = useState("");
  const [q, setQ] = useState("");
  const [teksIp, setTeksIp] = useState("");
  const [teksQ, setTeksQ] = useState("");
  const [halaman, setHalaman] = useState(1);
  const [riwayat, setRiwayat] = useState<DaftarRiwayat | null>(null);
  const [galatRiwayat, setGalatRiwayat] = useState("");
  const [memuatRiwayat, setMemuatRiwayat] = useState(false);
  const [terpilih, setTerpilih] = useState<RiwayatPermintaan | null>(null);
  const pertama = useRef(true);

  const galatTanggal = galatRentangPantau(filter);
  const kueri = useMemo(() => kueriPantau(filter), [filter]);

  useEffect(() => {
    if (!bolehLihat) return;
    getOpsiPantau().then(setOpsi).catch(() => setOpsi(null));
  }, [bolehLihat]);

  useEffect(() => {
    if (pertama.current) {
      pertama.current = false;
      return;
    }
    const t = setTimeout(() => {
      setIp(teksIp.trim());
      setQ(teksQ.trim());
      setHalaman(1);
    }, 400);
    return () => clearTimeout(t);
  }, [teksIp, teksQ]);

  useEffect(() => {
    if (!bolehLihat || galatTanggal) return;
    const ac = new AbortController();
    setMemuat(true);
    getPantau(kueri, ac.signal)
      .then((r) => {
        setData(r);
        setGalat("");
      })
      .catch((err) => {
        if (axios.isCancel(err)) return;
        setGalat(pesanGalat(err, "Gagal memuat data pemantauan"));
      })
      .finally(() => {
        if (!ac.signal.aborted) setMemuat(false);
      });
    return () => ac.abort();
  }, [bolehLihat, kueri, galatTanggal, versi]);

  useEffect(() => {
    if (!bolehLihat || galatTanggal) return;
    const ac = new AbortController();
    setMemuatRiwayat(true);
    const p: Record<string, string | number> = { ...kueri, halaman, per_halaman: PER_HALAMAN };
    if (hasil) p.hasil = hasil;
    if (ip) p.ip = ip;
    if (q) p.q = q;
    getRiwayat(p, ac.signal)
      .then((r) => {
        setRiwayat(r);
        setGalatRiwayat("");
      })
      .catch((err) => {
        if (axios.isCancel(err)) return;
        setGalatRiwayat(pesanGalat(err, "Gagal memuat riwayat permintaan"));
      })
      .finally(() => {
        if (!ac.signal.aborted) setMemuatRiwayat(false);
      });
    return () => ac.abort();
  }, [bolehLihat, kueri, galatTanggal, hasil, ip, q, halaman, versi]);

  useEffect(() => {
    if (!bolehLihat || !otomatis) return;
    const t = setInterval(() => {
      if (!document.hidden) setVersi((v) => v + 1);
    }, 30_000);
    return () => clearInterval(t);
  }, [bolehLihat, otomatis]);

  const ubah = useCallback((p: Partial<FilterPantau>) => {
    setFilter((f) => ({ ...f, ...p }));
    setHalaman(1);
  }, []);

  const waktuGrafik = useMemo(() => (data?.deret ?? []).map((t) => t.waktu), [data]);
  const denganTanggal = Boolean(data && (data.per_hari || new Date(data.sampai).getTime() - new Date(data.dari).getTime() > 36 * 3600_000));

  if (isLoadingProfile) return <SkeletonCards count={3} />;
  if (!bolehLihat) return <Alert tone="warning" message="Pemantauan API Akses Data hanya dapat dibuka oleh superadmin." />;

  const r = data?.ringkasan;
  const sekarang = new Date();
  const jumlahHalaman = riwayat ? Math.max(1, Math.ceil(riwayat.total / riwayat.per_halaman)) : 1;
  const terjatuh = data ? data.pencatat.dijatuhkan + data.pencatat.gagal : 0;

  return (
    <div className="space-y-5">
      <p className="rounded-xl border border-blue-100 bg-blue-50/60 px-4 py-3 text-sm text-slate-700">
        Memantau penarikan data oleh aplikasi klien: jumlah permintaan, galat, permintaan yang ditolak karena batas laju atau kuota (429), dan waktu respons. Isi data yang ditarik <b>tidak</b> disimpan; kunci API tidak pernah
        dicatat. Waktu ditampilkan dalam WIB.
        {data && ` Riwayat per permintaan disimpan ${data.pengaturan.log_retensi_hari} hari; hitungan per jam ${data.pengaturan.agregat_retensi_hari} hari.`}
      </p>

      <div className="flex flex-wrap items-end gap-3">
        <div className="min-w-[11rem]">
          <label htmlFor="pt-rentang" className="mb-1 block text-xs font-medium text-slate-600">Rentang waktu</label>
          <select id="pt-rentang" value={filter.rentang} onChange={(e) => ubah({ rentang: e.target.value as RentangPantau })} className={inputCls}>
            {RENTANG_PANTAU.map((x) => (
              <option key={x.kunci} value={x.kunci}>{x.label}</option>
            ))}
          </select>
        </div>
        {filter.rentang === "kustom" && (
          <>
            <div>
              <label htmlFor="pt-dari" className="mb-1 block text-xs font-medium text-slate-600">Dari tanggal</label>
              <input id="pt-dari" type="date" value={filter.dari} max={filter.sampai || undefined} onChange={(e) => ubah({ dari: e.target.value })} className={inputCls} />
            </div>
            <div>
              <label htmlFor="pt-sampai" className="mb-1 block text-xs font-medium text-slate-600">Sampai tanggal</label>
              <input id="pt-sampai" type="date" value={filter.sampai} min={filter.dari || undefined} onChange={(e) => ubah({ sampai: e.target.value })} className={inputCls} />
            </div>
          </>
        )}
        <div className="min-w-[12rem]">
          <label htmlFor="pt-klien" className="mb-1 block text-xs font-medium text-slate-600">Klien</label>
          <select id="pt-klien" value={filter.klien_id} onChange={(e) => ubah({ klien_id: e.target.value })} className={inputCls}>
            <option value="">Semua klien</option>
            {opsi?.klien.map((k) => (
              <option key={k.id} value={k.id}>{k.nama} ({k.kode})</option>
            ))}
          </select>
        </div>
        <div className="min-w-[12rem]">
          <label htmlFor="pt-endpoint" className="mb-1 block text-xs font-medium text-slate-600">Endpoint</label>
          <select id="pt-endpoint" value={filter.endpoint} onChange={(e) => ubah({ endpoint: e.target.value })} className={inputCls}>
            <option value="">Semua endpoint</option>
            {opsi?.endpoint.map((e) => (
              <option key={e.kode} value={e.kode}>{e.label} ({e.kode})</option>
            ))}
          </select>
        </div>
        <div className="ml-auto flex flex-wrap items-center gap-3">
          <label className="flex cursor-pointer items-center gap-2 text-xs text-slate-600">
            <input type="checkbox" checked={otomatis} onChange={(e) => setOtomatis(e.target.checked)} className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" />
            Segarkan otomatis (30 detik)
          </label>
          <Button size="sm" variant="secondary" fullWidth={false} icon={<RefreshCw className="h-4 w-4" aria-hidden="true" />} onClick={() => setVersi((v) => v + 1)}>
            Segarkan
          </Button>
        </div>
      </div>
      {galatTanggal && <Alert tone="warning" message={galatTanggal} />}
      {galat && <Alert message={galat} />}
      {terjatuh > 0 && data && (
        <Alert tone="warning" message={`${terjatuh} riwayat permintaan tidak tercatat sejak aplikasi mulai (${data.pencatat.dijatuhkan} karena antrean penuh, ${data.pencatat.gagal} karena gagal menulis ke database). Hitungan per jam tetap lengkap bila hanya antrean yang penuh. Periksa kesehatan database.`} />
      )}

      {!r && !galat && !galatTanggal && <SkeletonCards count={4} />}
      {r && data && (
        <>
          <div className={`grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-7 ${memuat ? "opacity-70" : ""}`}>
            <StatTile label="Permintaan" value={formatJumlah(r.jumlah)} sub={`${formatJumlah(data.klien_aktif)} klien aktif`} />
            <StatTile label="Berhasil" value={formatJumlah(r.sukses)} sub={`${teksPersen(r.sukses, r.jumlah)} dari total`} />
            <StatTile label="Galat dan ditolak" value={formatJumlah(jumlahGalat(r))} sub={`${formatJumlah(r.galat_klien)} permintaan · ${formatJumlah(r.ditolak)} kunci/IP/scope`} />
            <StatTile label="Dibatasi (429)" value={formatJumlah(jumlahDibatasi(r))} sub={`${formatJumlah(r.batas_laju)} laju · ${formatJumlah(r.kuota_habis)} kuota`} />
            <StatTile label="Galat server (5xx)" value={formatJumlah(r.galat_server)} sub={r.galat_server > 0 ? teksPersen(r.galat_server, r.jumlah) + " dari total" : undefined} />
            <StatTile label="Waktu respons rata-rata" value={formatMsAkses(r.lat_rata_ms)} sub={`p50 ${formatMsAkses(r.lat_p50_ms)} · p95 ${formatMsAkses(r.lat_p95_ms)} · maks ${formatMsAkses(r.lat_maks_ms)}`} />
            <StatTile label="Baris data ditarik" value={formatJumlah(r.baris)} />
          </div>

          <div className="grid gap-4 xl:grid-cols-2">
            <GrafikGaris
              judul={data.per_hari ? "Permintaan per hari" : "Permintaan per jam"}
              subjudul="Seluruh permintaan, yang gagal, dan yang dibatasi (429)"
              waktu={waktuGrafik}
              format={(v) => Math.round(v).toLocaleString("id-ID")}
              denganTanggal={denganTanggal}
              pesanKosong="Belum ada permintaan dari klien pada rentang dan penyaring ini."
              deret={[
                { nama: "Permintaan", warna: WARNA.jumlah, nilai: data.deret.map((t) => t.jumlah) },
                { nama: "Gagal", warna: WARNA.gagal, nilai: data.deret.map((t) => t.gagal) },
                { nama: "Dibatasi (429)", warna: WARNA.dibatasi, nilai: data.deret.map((t) => t.dibatasi) },
              ]}
            />
            <GrafikGaris
              judul="Waktu respons rata-rata"
              subjudul="Rata-rata per titik waktu (milidetik)"
              waktu={waktuGrafik}
              format={(v) => formatMsAkses(v)}
              denganTanggal={denganTanggal}
              pesanKosong="Belum ada permintaan dari klien pada rentang dan penyaring ini."
              deret={[{ nama: "Rata-rata", warna: WARNA.latensi, nilai: data.deret.map((t) => (t.jumlah > 0 ? t.lat_rata_ms : null)) }]}
            />
          </div>

          <ChartCard
            title="Pemakaian kuota saat ini"
            subtitle="Keadaan sekarang untuk klien yang disetujui, tidak bergantung pada rentang waktu di atas"
            table={
              data.pemakaian.length > 0 ? (
                <DataTable
                  columns={["Klien", "Kuota terpakai", "Sisa kuota", "Sisa jatah laju", "Batas"]}
                  rows={data.pemakaian.map((p) => [p.nama, formatJumlah(p.kuota_terpakai), formatJumlah(p.kuota_sisa), formatJumlah(p.limit_sisa), teksBatas(p.batas)])}
                />
              ) : undefined
            }
          >
            {data.pemakaian.length === 0 ? (
              <p className="py-6 text-center text-sm text-slate-400">Belum ada klien yang disetujui.</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full min-w-[44rem] text-sm">
                  <thead className="text-xs text-slate-500">
                    <tr className="border-b border-slate-200">
                      <th scope="col" className="py-2 pr-3 text-left font-medium">Klien</th>
                      <th scope="col" className="px-3 py-2 text-left font-medium">Kuota periode ini</th>
                      <th scope="col" className="px-3 py-2 text-right font-medium">Sisa jatah laju</th>
                      <th scope="col" className="px-3 py-2 text-left font-medium">Batas</th>
                      <th scope="col" className="px-3 py-2 text-left font-medium">Kuota pulih</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {[...data.pemakaian].sort((a, b) => b.kuota_persen - a.kuota_persen).map((p) => (
                      <tr key={p.klien_id}>
                        <th scope="row" className="py-2.5 pr-3 text-left font-normal">
                          <p className="font-medium text-slate-900">{p.nama}</p>
                          <p className="font-mono text-[11px] text-slate-400">{p.kode}{p.override ? " · batas khusus" : ""}</p>
                        </th>
                        <td className="px-3 py-2.5">
                          <BatangKuota terpakai={p.kuota_terpakai} total={p.batas.kuota_permintaan} persen={p.kuota_persen} />
                        </td>
                        <td className="px-3 py-2.5 text-right tabular-nums text-slate-700">
                          {formatJumlah(p.limit_sisa)} <span className="text-xs text-slate-400">/ {formatJumlah(p.batas.limit_permintaan)}</span>
                        </td>
                        <td className="px-3 py-2.5 text-xs text-slate-600">{teksBatas(p.batas)}</td>
                        <td className="whitespace-nowrap px-3 py-2.5 text-xs text-slate-600">{waktuWIB(p.kuota_reset, false)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </ChartCard>

          <div className="grid gap-4 xl:grid-cols-2">
            <TabelKelompok judul="Menurut klien" subjudul="Terbanyak dulu" kolomPertama="Klien" baris={data.per_klien} namaBaris={(g) => (g.klien_id ? (g.nama || g.kode || `Klien ${g.klien_id}`) : "Kunci tidak dikenal")} subBaris={(g) => (g.klien_id ? g.kode : undefined)} />
            <TabelKelompok judul="Menurut endpoint" subjudul="Pola rute, tanpa nilai parameter" kolomPertama="Endpoint" baris={data.per_endpoint} namaBaris={(g) => g.kunci} mono />
          </div>
        </>
      )}

      <section className="space-y-3">
        <h2 className="flex items-center gap-2 text-base font-semibold text-slate-900">
          <Activity className="h-4 w-4 text-blue-600" aria-hidden="true" />
          Riwayat permintaan terakhir
        </h2>
        <div className="flex flex-wrap items-end gap-3">
          <div className="min-w-[11rem]">
            <label htmlFor="rw-hasil" className="mb-1 block text-xs font-medium text-slate-600">Hasil</label>
            <select id="rw-hasil" value={hasil} onChange={(e) => { setHasil(e.target.value); setHalaman(1); }} className={inputCls}>
              <option value="">Semua hasil</option>
              {opsi?.hasil.map((h) => (
                <option key={h.kode} value={h.kode}>{h.label}</option>
              ))}
            </select>
          </div>
          <div className="min-w-[10rem]">
            <label htmlFor="rw-ip" className="mb-1 block text-xs font-medium text-slate-600">Alamat IP</label>
            <input id="rw-ip" type="search" value={teksIp} onChange={(e) => setTeksIp(e.target.value)} maxLength={64} placeholder="mis. 10.20" className={inputCls} />
          </div>
          <div className="relative min-w-[14rem] flex-1 sm:max-w-sm">
            <label htmlFor="rw-q" className="mb-1 block text-xs font-medium text-slate-600">Cari</label>
            <Search className="pointer-events-none absolute bottom-3 left-3 h-4 w-4 text-slate-400" aria-hidden="true" />
            <input id="rw-q" type="search" value={teksQ} onChange={(e) => setTeksQ(e.target.value)} maxLength={100} placeholder="ID permintaan, dataset, endpoint, awalan kunci" className={`${inputCls} pl-9`} />
          </div>
        </div>
        {galatRiwayat && <Alert message={galatRiwayat} />}
        {!riwayat && !galatRiwayat && !galatTanggal && <SkeletonTable rows={6} cols={6} />}
        {riwayat && (
          <>
            <p className="text-sm text-slate-600" aria-live="polite">
              <span className="font-semibold text-slate-900">{riwayat.total.toLocaleString("id-ID")}</span> permintaan
              {memuatRiwayat && <span className="ml-2 text-xs text-slate-400">memuat…</span>}
              {riwayat.total > 0 && <span className="text-slate-400"> · halaman {riwayat.halaman} dari {jumlahHalaman.toLocaleString("id-ID")}</span>}
            </p>
            {riwayat.riwayat.length === 0 ? (
              <EmptyState icon={Activity} title="Tidak ada permintaan yang cocok" description="Ubah rentang waktu atau penyaring. Riwayat per permintaan hanya disimpan beberapa hari." />
            ) : (
              <div className={`overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm ${memuatRiwayat ? "opacity-70" : ""}`}>
                <table className="w-full min-w-[56rem] text-sm">
                  <thead className="bg-slate-50 text-xs text-slate-500">
                    <tr>
                      <th scope="col" className="px-4 py-2.5 text-left font-medium">Waktu (WIB)</th>
                      <th scope="col" className="px-4 py-2.5 text-left font-medium">Klien</th>
                      <th scope="col" className="px-4 py-2.5 text-left font-medium">Endpoint</th>
                      <th scope="col" className="px-4 py-2.5 text-left font-medium">Hasil</th>
                      <th scope="col" className="px-4 py-2.5 text-right font-medium">Lama</th>
                      <th scope="col" className="px-4 py-2.5 text-right font-medium">Baris</th>
                      <th scope="col" className="px-4 py-2.5 text-left font-medium">IP</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {riwayat.riwayat.map((u) => (
                      <tr
                        key={u.id}
                        tabIndex={0}
                        role="button"
                        aria-label={`Lihat rincian permintaan ${u.metode} ${u.endpoint}`}
                        onClick={() => setTerpilih(u)}
                        onKeyDown={(e) => {
                          if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            setTerpilih(u);
                          }
                        }}
                        className="cursor-pointer outline-none hover:bg-slate-50 focus-visible:bg-blue-50/60"
                      >
                        <td className="whitespace-nowrap px-4 py-2.5 text-xs tabular-nums text-slate-600" title={teksRelatif(u.waktu, sekarang)}>{waktuWIB(u.waktu)}</td>
                        <td className="max-w-[14rem] px-4 py-2.5">
                          {u.klien_nama ? (
                            <>
                              <p className="truncate font-medium text-slate-900">{u.klien_nama}</p>
                              <p className="font-mono text-[11px] text-slate-400">{u.klien_kode}</p>
                            </>
                          ) : (
                            <span className="text-xs italic text-slate-400">kunci tidak dikenal</span>
                          )}
                        </td>
                        <td className="max-w-[18rem] px-4 py-2.5">
                          <p className="truncate font-mono text-xs text-slate-800">{u.metode} {u.endpoint}</p>
                          {u.dataset && <p className="truncate text-[11px] text-slate-400">{u.dataset}</p>}
                        </td>
                        <td className="px-4 py-2.5">
                          <LencanaHasilAkses hasil={u.hasil} />
                          <span className="ml-1.5 text-[11px] tabular-nums text-slate-400">{u.status_http}</span>
                        </td>
                        <td className="whitespace-nowrap px-4 py-2.5 text-right text-xs tabular-nums text-slate-600">{formatMsAkses(u.durasi_ms)}</td>
                        <td className="px-4 py-2.5 text-right text-xs tabular-nums text-slate-600">{u.baris === undefined ? "-" : formatJumlah(u.baris)}</td>
                        <td className="whitespace-nowrap px-4 py-2.5 font-mono text-xs text-slate-600">{u.ip || "-"}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            {jumlahHalaman > 1 && (
              <div className="flex items-center justify-end gap-2">
                <Button size="sm" variant="secondary" fullWidth={false} disabled={halaman <= 1 || memuatRiwayat} icon={<ChevronLeft className="h-4 w-4" aria-hidden="true" />} onClick={() => setHalaman((h) => Math.max(1, h - 1))}>
                  Sebelumnya
                </Button>
                <Button size="sm" variant="secondary" fullWidth={false} disabled={halaman >= jumlahHalaman || memuatRiwayat} onClick={() => setHalaman((h) => h + 1)}>
                  Berikutnya
                  <ChevronRight className="h-4 w-4" aria-hidden="true" />
                </Button>
              </div>
            )}
          </>
        )}
      </section>

      {terpilih && (
        <ModalShell title="Rincian permintaan" subtitle={`${terpilih.metode} ${terpilih.endpoint}`} onClose={() => setTerpilih(null)} size="md">
          <div className="space-y-4 p-4 sm:p-6">
            <div className="flex flex-wrap items-center gap-2">
              <LencanaHasilAkses hasil={terpilih.hasil} />
              <span className="text-xs tabular-nums text-slate-500">HTTP {terpilih.status_http}</span>
            </div>
            <DetailEntry
              fields={[
                ["Waktu (WIB)", waktuWIB(terpilih.waktu)],
                ["Waktu (UTC)", terpilih.waktu],
                ["Klien", terpilih.klien_nama ? `${terpilih.klien_nama} (${terpilih.klien_kode})` : "Kunci tidak dikenal atau tidak sah"],
                ["Awalan kunci", terpilih.kunci_prefix ? `pasti_${terpilih.kunci_prefix}_…` : undefined],
                ["Dataset", terpilih.dataset],
                ["Lama proses", formatMsAkses(terpilih.durasi_ms)],
                ["Baris dikembalikan", terpilih.baris === undefined ? undefined : formatJumlah(terpilih.baris)],
                ["Alamat IP", terpilih.ip],
                ["User agent", terpilih.user_agent],
                ["ID permintaan", terpilih.request_id],
              ]}
            />
            <p className="rounded-lg bg-slate-50 px-3 py-2 text-xs text-slate-500">
              Yang tersimpan hanya metadata permintaan. Kunci API, isi respons, dan parameter pencarian tidak dicatat. ID permintaan sama dengan header <span className="font-mono">X-Request-ID</span> dan dapat dicocokkan
              dengan Log Audit serta Log Server.
            </p>
          </div>
        </ModalShell>
      )}
    </div>
  );
}

function TabelKelompok({
  judul,
  subjudul,
  kolomPertama,
  baris,
  namaBaris,
  subBaris,
  mono,
}: {
  judul: string;
  subjudul: string;
  kolomPertama: string;
  baris: Kelompok[];
  namaBaris: (g: Kelompok) => string;
  subBaris?: (g: Kelompok) => string | undefined;
  mono?: boolean;
}) {
  return (
    <ChartCard
      title={judul}
      subtitle={subjudul}
      table={baris.length > 0 ? <DataTable columns={[kolomPertama, "Permintaan", "Berhasil", "Galat", "429", "5xx", "p95"]} rows={baris.map((g) => [namaBaris(g), formatJumlah(g.jumlah), formatJumlah(g.sukses), formatJumlah(g.galat_klien + g.ditolak), formatJumlah(jumlahDibatasi(g)), formatJumlah(g.galat_server), formatMsAkses(g.lat_p95_ms)])} /> : undefined}
    >
      {baris.length === 0 ? (
        <p className="py-6 text-center text-sm text-slate-400">Tidak ada data</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[30rem] text-sm">
            <thead className="text-xs text-slate-500">
              <tr className="border-b border-slate-200">
                <th scope="col" className="py-2 pr-3 text-left font-medium">{kolomPertama}</th>
                <th scope="col" className="px-2 py-2 text-right font-medium">Permintaan</th>
                <th scope="col" className="px-2 py-2 text-right font-medium">Berhasil</th>
                <th scope="col" className="px-2 py-2 text-right font-medium">Galat</th>
                <th scope="col" className="px-2 py-2 text-right font-medium">429</th>
                <th scope="col" className="px-2 py-2 text-right font-medium">p95</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {baris.slice(0, 15).map((g) => (
                <tr key={g.kunci}>
                  <th scope="row" className="max-w-[14rem] py-2 pr-3 text-left font-normal">
                    <p className={`truncate text-slate-900 ${mono ? "font-mono text-xs" : "font-medium"}`}>{namaBaris(g)}</p>
                    {subBaris?.(g) && <p className="font-mono text-[11px] text-slate-400">{subBaris(g)}</p>}
                  </th>
                  <td className="px-2 py-2 text-right tabular-nums text-slate-900">{formatJumlah(g.jumlah)}</td>
                  <td className="px-2 py-2 text-right tabular-nums text-slate-700">{formatJumlah(g.sukses)}</td>
                  <td className="px-2 py-2 text-right tabular-nums text-slate-700">{formatJumlah(g.galat_klien + g.ditolak + g.galat_server)}</td>
                  <td className={`px-2 py-2 text-right tabular-nums ${jumlahDibatasi(g) > 0 ? "font-semibold text-amber-700" : "text-slate-700"}`}>{formatJumlah(jumlahDibatasi(g))}</td>
                  <td className="whitespace-nowrap px-2 py-2 text-right text-xs tabular-nums text-slate-600">{formatMsAkses(g.lat_p95_ms)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {baris.length > 15 && <p className="mt-2 text-xs text-slate-400">Menampilkan 15 teratas dari {baris.length}. Gunakan tombol Tabel untuk melihat semuanya.</p>}
        </div>
      )}
    </ChartCard>
  );
}
