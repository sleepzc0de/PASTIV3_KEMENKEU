"use client";

import { useCallback, useEffect, useState } from "react";
import axios from "axios";
import { Database as IkonDB } from "lucide-react";
import { type PoolDB, type RingkasanMonitor, type TabelDB, getMonitorTabel } from "@/lib/monitor";
import { formatAngka, formatBytes, formatMB, formatMs, formatPersen, formatRingkas, statusDariPersen } from "@/lib/monitorFormat";
import { waktuWIB } from "@/lib/auditFilter";
import { Alert } from "@/components/ui/Alert";
import { SkeletonTable } from "@/components/ui/Skeleton";
import { BarisInfo, LencanaStatus, Panel, Pengukur } from "./bagian";

function KartuPool({ p }: { p: PoolDB }) {
  const persen = p.batas > 0 ? (p.dipakai / p.batas) * 100 : null;
  return (
    <Panel
      judul={p.nama}
      aksi={<LencanaStatus status={p.tersambung ? "cukup" : "kritis"} />}
      deskripsi={p.tersambung ? `Menjawab dalam ${formatMs(p.ping_ms)}` : p.galat ? `Tidak dapat dihubungi: ${p.galat}` : "Tidak dapat dihubungi"}
    >
      <Pengukur
        label="Koneksi sedang dipakai"
        nilai={persen}
        teks={persen === null ? "-" : `${p.dipakai} dari ${p.batas}`}
        status={persen === null ? "tidak_ada_data" : statusDariPersen(persen, 70, 90)}
        sub={`${p.terbuka} terbuka, ${p.menganggur} menganggur`}
      />
      <div className="mt-3">
        <BarisInfo
          baris={[
            ["Batas koneksi (pool)", formatRingkas(p.batas)],
            ["Permintaan yang harus menunggu koneksi", `${formatRingkas(p.menunggu_total)} kali sejak aplikasi mulai`],
            ["Total waktu menunggu", formatMs(p.menunggu_ms_total)],
          ]}
        />
      </div>
    </Panel>
  );
}

// Database aplikasi: pool koneksi dan keadaan SQL Server (versi, ukuran, memori, tabel terbesar).
export function PanelDatabase({ data }: { data: RingkasanMonitor }) {
  const db = data.database;
  const sql = db.sql;
  const [tabel, setTabel] = useState<TabelDB[] | null>(null);
  const [catatanTabel, setCatatanTabel] = useState("");
  const [galat, setGalat] = useState("");

  const muatTabel = useCallback(async () => {
    try {
      const r = await getMonitorTabel();
      setTabel(r.data.tabel);
      setCatatanTabel(r.data.catatan);
      setGalat("");
    } catch (err) {
      setGalat(axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : "Gagal memuat ukuran tabel");
    }
  }, []);

  useEffect(() => {
    void muatTabel();
  }, [muatTabel]);

  const maksTabel = Math.max(1, ...(tabel ?? []).map((t) => t.ukuran_mb));
  const terisi = sql && sql.terpakai_mb !== null && sql.ukuran_mb > 0 ? (sql.terpakai_mb / sql.ukuran_mb) * 100 : null;

  return (
    <div className="space-y-4">
      <div className="grid gap-4 lg:grid-cols-2">
        {db.utama && <KartuPool p={db.utama} />}
        {db.sldk && <KartuPool p={db.sldk} />}
        {!db.sldk && (
          <Panel judul="Database SLDK">
            <p className="text-sm text-slate-600">Koneksi SLDK tidak dikonfigurasi (SLDK_DB_* kosong), jadi sinkronisasi data aset dari SLDK tidak berjalan.</p>
          </Panel>
        )}
      </div>

      {sql ? (
        <>
          <div className="grid gap-4 lg:grid-cols-2">
            <Panel judul="SQL Server" deskripsi={`Diukur ${waktuWIB(sql.diukur_pada, false)} WIB (disimpan sementara 1 menit)`}>
              <BarisInfo
                baris={[
                  ["Versi", sql.versi || "-"],
                  ["Edisi", sql.edisi || "-"],
                  ["Database", sql.nama_db || "-"],
                  ["Ukuran file (data + log)", formatMB(sql.ukuran_mb)],
                  ["Terisi di dalam file", sql.terpakai_mb === null ? "-" : `${formatMB(sql.terpakai_mb)}${terisi !== null ? ` (${formatPersen(terisi, 0)})` : ""}`],
                  ["Sesi pengguna", sql.sesi_pengguna === null ? "-" : formatRingkas(sql.sesi_pengguna)],
                ]}
              />
            </Panel>
            <Panel judul="Memori dan CPU server database" deskripsi="Butuh izin VIEW SERVER STATE pada akun database aplikasi.">
              <BarisInfo
                baris={[
                  ["CPU server database", sql.cpu_jumlah === null ? "-" : `${sql.cpu_jumlah} core`],
                  ["Memori fisik server database", sql.mem_fisik_mb === null ? "-" : formatMB(sql.mem_fisik_mb)],
                  ["Dipakai proses SQL Server", sql.mem_proses_mb === null ? "-" : formatMB(sql.mem_proses_mb)],
                  ["Page life expectancy", sql.page_life_expectancy === null ? "-" : `${formatAngka(sql.page_life_expectancy, 0)} detik`],
                  ["Buffer cache hit ratio", sql.buffer_hit_ratio === null ? "-" : formatPersen(sql.buffer_hit_ratio, 2)],
                ]}
              />
              <p className="mt-2 text-xs text-slate-500">Page life expectancy di bawah ±300 detik menandakan SQL Server sering membaca ulang dari disk karena memori kurang.</p>
            </Panel>
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <Panel judul="File database">
              <div className="overflow-x-auto rounded-xl border border-slate-200">
                <table className="w-full min-w-max text-sm">
                  <thead className="bg-slate-50 text-xs text-slate-500">
                    <tr>
                      <th scope="col" className="px-3 py-2 text-left font-medium">File</th>
                      <th scope="col" className="px-3 py-2 text-left font-medium">Jenis</th>
                      <th scope="col" className="px-3 py-2 text-right font-medium">Ukuran</th>
                      <th scope="col" className="px-3 py-2 text-right font-medium">Terisi</th>
                      <th scope="col" className="px-3 py-2 text-right font-medium">Batas</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100">
                    {sql.file.map((f) => (
                      <tr key={f.nama}>
                        <td className="px-3 py-2 font-mono text-xs">{f.nama}</td>
                        <td className="px-3 py-2 text-xs">{f.jenis === "ROWS" ? "Data" : f.jenis === "LOG" ? "Log transaksi" : f.jenis}</td>
                        <td className="px-3 py-2 text-right tabular-nums">{formatMB(f.ukuran_mb)}</td>
                        <td className="px-3 py-2 text-right tabular-nums">{f.terpakai_mb === null ? "-" : formatMB(f.terpakai_mb)}</td>
                        <td className="px-3 py-2 text-right tabular-nums">{f.maks_mb === null ? "tidak dibatasi" : formatMB(f.maks_mb)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </Panel>
            <Panel judul="Ruang disk volume database" deskripsi="Butuh izin VIEW SERVER STATE.">
              {sql.volume.length === 0 ? (
                <p className="text-sm text-slate-600">Tidak terbaca. Cek ruang disk langsung di server database.</p>
              ) : (
                <div className="space-y-4">
                  {sql.volume.map((v) => (
                    <Pengukur key={v.titik} label={v.titik} nilai={v.persen} teks={formatPersen(v.persen)} status={statusDariPersen(v.persen, 80, 90)} sub={`sisa ${formatBytes(v.bebas)} dari ${formatBytes(v.total)}`} />
                  ))}
                </div>
              )}
            </Panel>
          </div>

          {sql.catatan && sql.catatan.length > 0 && (
            <Alert tone="info" message={`Sebagian pengukuran database dilewati karena izin akun database aplikasi terbatas: ${sql.catatan.join(" · ")}`} />
          )}
        </>
      ) : (
        <Alert tone="warning" message="Keadaan SQL Server belum dapat dibaca. Pastikan database aplikasi tersambung." />
      )}

      <Panel judul="Tabel terbesar" deskripsi="Ukuran tabel pada database aplikasi (15 terbesar), untuk melihat data mana yang paling banyak memakai ruang.">
        {galat && <Alert message={galat} />}
        {!tabel && !galat && <SkeletonTable rows={5} cols={3} />}
        {catatanTabel && <Alert tone="info" message={catatanTabel} />}
        {tabel && tabel.length > 0 && (
          <ul className="mt-1 space-y-2">
            {tabel.map((t) => (
              <li key={t.nama} className="grid grid-cols-[minmax(0,14rem)_minmax(0,1fr)_auto] items-center gap-3 text-sm">
                <span className="flex min-w-0 items-center gap-1.5 truncate font-mono text-xs text-slate-800" title={t.nama}>
                  <IkonDB className="h-3.5 w-3.5 shrink-0 text-slate-400" aria-hidden="true" />
                  {t.nama}
                </span>
                <span className="h-2.5 overflow-hidden rounded-full bg-slate-100" aria-hidden="true">
                  <span className="block h-full rounded-full bg-blue-600" style={{ width: `${Math.max(1, (t.ukuran_mb / maksTabel) * 100)}%` }} />
                </span>
                <span className="whitespace-nowrap text-right text-xs tabular-nums text-slate-600">
                  <span className="font-semibold text-slate-900">{formatMB(t.ukuran_mb)}</span> · {formatRingkas(t.baris)} baris
                </span>
              </li>
            ))}
          </ul>
        )}
      </Panel>
    </div>
  );
}
