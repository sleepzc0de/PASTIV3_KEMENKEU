"use client";

import { useEffect, useMemo, useState } from "react";
import axios from "axios";
import { type RentangRiwayat, type RiwayatMonitor, getMonitorRiwayat } from "@/lib/monitor";
import { RENTANG_RIWAYAT, ambilKolom, formatAngka, formatBytes, formatRingkas } from "@/lib/monitorFormat";
import { Alert } from "@/components/ui/Alert";
import { GrafikGaris } from "@/components/ui/GrafikGaris";
import { Tabs } from "@/components/ui/Tabs";
import { SkeletonCards } from "@/components/ui/Skeleton";

const BIRU = "#3358e0";
const JINGGA = "#e8890c";
const HIJAU = "#0ca30c";
const MERAH = "#d03b3b";
const UNGU = "#7c3aed";

const persen = (v: number) => `${formatAngka(v, v >= 10 ? 0 : 1)}%`;
const mb = (v: number) => formatBytes(v * 1024 * 1024);

// Riwayat pemakaian resource sebagai grafik: server, aplikasi, database, dan permintaan HTTP. Rentang 1 jam dari pengukuran di memori (tiap 30 detik); yang lebih panjang dari
// snapshot yang disimpan tiap 5 menit di database.
export function PanelRiwayat() {
  const [rentang, setRentang] = useState<RentangRiwayat>("24j");
  const [data, setData] = useState<RiwayatMonitor | null>(null);
  const [galat, setGalat] = useState("");

  useEffect(() => {
    let batal = false;
    const ac = new AbortController();
    const muat = async () => {
      try {
        const r = await getMonitorRiwayat(rentang, ac.signal);
        if (!batal) {
          setData(r.data);
          setGalat("");
        }
      } catch (err) {
        if (batal || axios.isCancel(err)) return;
        setGalat(axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : "Gagal memuat riwayat");
      }
    };
    setData(null);
    void muat();
    const t = setInterval(() => {
      if (!document.hidden) void muat();
    }, rentang === "1j" ? 30_000 : 120_000);
    return () => {
      batal = true;
      ac.abort();
      clearInterval(t);
    };
  }, [rentang]);

  const titik = useMemo(() => data?.titik ?? [], [data]);
  const waktu = useMemo(() => titik.map((t) => t.waktu), [titik]);
  const lebihDariSehari = rentang === "7h" || rentang === "30h";
  const kolom = (k: Parameters<typeof ambilKolom>[1], skala = 1) => ambilKolom(titik, k, skala);

  const g = {
    cpu: [
      { nama: "Rata-rata", warna: BIRU, nilai: kolom("cpu_rata") },
      { nama: "Puncak", warna: JINGGA, nilai: kolom("cpu_maks") },
    ],
    memDisk: [
      { nama: "Memori (puncak)", warna: BIRU, nilai: kolom("mem_persen") },
      { nama: "Disk", warna: JINGGA, nilai: kolom("disk_persen") },
    ],
    load: [{ nama: "Beban 1 menit (puncak)", warna: BIRU, nilai: kolom("load1") }],
    prosesMem: [
      { nama: "Memori proses (RSS)", warna: BIRU, nilai: kolom("rss", 1024 * 1024) },
      { nama: "Heap Go", warna: JINGGA, nilai: kolom("heap", 1024 * 1024) },
    ],
    goroutine: [{ nama: "Goroutine (puncak)", warna: UNGU, nilai: kolom("goroutine") }],
    dbKoneksi: [
      { nama: "Koneksi dipakai (puncak)", warna: BIRU, nilai: kolom("db_dipakai") },
      { nama: "Permintaan menunggu koneksi", warna: MERAH, nilai: kolom("db_tunggu") },
    ],
    dbUkuran: [{ nama: "Ukuran file database", warna: BIRU, nilai: kolom("db_ukuran_mb") }],
    req: [
      { nama: "Permintaan", warna: BIRU, nilai: kolom("req_total") },
      { nama: "Galat 5xx", warna: MERAH, nilai: kolom("req_5xx") },
    ],
    respons: [
      { nama: "Rata-rata", warna: BIRU, nilai: kolom("lat_rata_ms") },
      { nama: "p95", warna: JINGGA, nilai: kolom("lat_p95_ms") },
    ],
  };
  const ms = (v: number) => (v >= 1000 ? `${formatAngka(v / 1000, 1)} dtk` : `${formatAngka(v, 0)} ms`);
  const sub = RENTANG_RIWAYAT.find((r) => r.kunci === rentang)?.lebar;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Tabs tabs={RENTANG_RIWAYAT.map((r) => ({ key: r.kunci, label: r.label }))} value={rentang} onChange={setRentang} label="Rentang riwayat" idPrefix="riwayat" />
        <p className="text-xs text-slate-500">{data ? `${formatRingkas(titik.length)} titik, ${sub}. Waktu dalam WIB.` : ""}</p>
      </div>
      {galat && <Alert message={galat} />}
      {!data && !galat && <SkeletonCards count={4} />}
      {data && (
        <div className="grid gap-4 xl:grid-cols-2">
          <GrafikGaris judul="CPU server" subjudul="Pemakaian CPU seluruh server" waktu={waktu} deret={g.cpu} format={persen} maksY={100} denganTanggal={lebihDariSehari} />
          <GrafikGaris judul="Memori dan disk server" subjudul="Persen terpakai (memori: puncak pada jendela itu)" waktu={waktu} deret={g.memDisk} format={persen} maksY={100} denganTanggal={lebihDariSehari} />
          <GrafikGaris judul="Waktu respons aplikasi" subjudul="Permintaan biasa (ekspor, impor, sinkronisasi tidak dihitung)" waktu={waktu} deret={g.respons} format={ms} denganTanggal={lebihDariSehari} />
          <GrafikGaris judul="Permintaan HTTP" subjudul="Jumlah permintaan dan galat server (5xx) per titik" waktu={waktu} deret={g.req} format={(v) => formatRingkas(v)} denganTanggal={lebihDariSehari} />
          <GrafikGaris judul="Koneksi database" subjudul="Pool koneksi aplikasi: yang dipakai dan yang harus menunggu" waktu={waktu} deret={g.dbKoneksi} format={(v) => formatRingkas(v)} denganTanggal={lebihDariSehari} />
          <GrafikGaris judul="Memori proses aplikasi" subjudul="Memori fisik proses dan heap Go" waktu={waktu} deret={g.prosesMem} format={mb} denganTanggal={lebihDariSehari} />
          <GrafikGaris judul="Goroutine" subjudul="Utas ringan Go; naik terus tanpa turun menandakan kebocoran" waktu={waktu} deret={g.goroutine} format={(v) => formatRingkas(v)} denganTanggal={lebihDariSehari} />
          <GrafikGaris judul="Ukuran database" subjudul="Total ukuran file data dan log" waktu={waktu} deret={g.dbUkuran} format={mb} denganTanggal={lebihDariSehari} />
          {titik.some((t) => t.load1 !== null) && (
            <GrafikGaris judul="Beban server (load average)" subjudul="Puncak beban 1 menit; di atas jumlah core berarti proses antre" waktu={waktu} deret={g.load} format={(v) => formatAngka(v, 1)} denganTanggal={lebihDariSehari} />
          )}
        </div>
      )}
    </div>
  );
}
