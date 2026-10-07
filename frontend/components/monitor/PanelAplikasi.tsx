"use client";

import type { RingkasanMonitor, RuteLambat } from "@/lib/monitor";
import { formatAngka, formatBytes, formatDurasi, formatMs, formatRingkas } from "@/lib/monitorFormat";
import { StatTile } from "@/components/ui/charts";
import { EmptyState } from "@/components/ui/EmptyState";
import { Panel } from "./bagian";

function TabelRute({ rute, kolomGalat }: { rute: RuteLambat[]; kolomGalat?: boolean }) {
  return (
    <div className="overflow-x-auto rounded-xl border border-slate-200">
      <table className="w-full min-w-max text-sm">
        <thead className="bg-slate-50 text-xs text-slate-500">
          <tr>
            <th scope="col" className="px-3 py-2 text-left font-medium">Rute</th>
            <th scope="col" className="px-3 py-2 text-right font-medium">Permintaan</th>
            <th scope="col" className="px-3 py-2 text-right font-medium">Rata-rata</th>
            <th scope="col" className="px-3 py-2 text-right font-medium">p95</th>
            <th scope="col" className="px-3 py-2 text-right font-medium">Terlama</th>
            <th scope="col" className="px-3 py-2 text-right font-medium">Galat 5xx</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100">
          {rute.map((r) => (
            <tr key={r.metode + r.rute} className="hover:bg-slate-50">
              <td className="px-3 py-2">
                <span className="mr-2 rounded bg-slate-100 px-1.5 py-0.5 font-mono text-[11px] font-semibold text-slate-600">{r.metode}</span>
                <span className="font-mono text-xs text-slate-800">{r.rute}</span>
                {r.berat && <span className="ml-2 rounded-full bg-sky-50 px-2 py-0.5 text-[11px] font-medium text-sky-700 ring-1 ring-inset ring-sky-200" title="Memang berat (ekspor, impor, sinkronisasi, atau penarikan): tidak dihitung pada waktu respons keseluruhan">berat</span>}
              </td>
              <td className="px-3 py-2 text-right tabular-nums">{formatRingkas(r.jumlah)}</td>
              <td className="px-3 py-2 text-right tabular-nums">{formatMs(r.rata_ms)}</td>
              <td className={`px-3 py-2 text-right tabular-nums ${!kolomGalat && r.p95_ms >= 1000 && !r.berat ? "font-semibold text-amber-700" : ""}`}>{formatMs(r.p95_ms)}</td>
              <td className="px-3 py-2 text-right tabular-nums">{formatMs(r.maks_ms)}</td>
              <td className={`px-3 py-2 text-right tabular-nums ${r.galat_5xx > 0 ? "font-semibold text-red-700" : "text-slate-400"}`}>{formatRingkas(r.galat_5xx)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// Proses aplikasi (runtime Go) dan kinerja permintaan HTTP.
export function PanelAplikasi({ data }: { data: RingkasanMonitor }) {
  const p = data.proses;
  const h = data.http;
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <StatTile label="Permintaan berjalan" value={formatRingkas(h.berjalan)} sub="sedang diproses sekarang" />
        <StatTile label="Goroutine" value={formatRingkas(p.goroutine)} sub="utas ringan Go; naik terus = kebocoran" />
        <StatTile label="Memori proses" value={p.rss === null ? formatBytes(p.mem_dari_os) : formatBytes(p.rss)} sub={p.rss === null ? "dari OS (RSS tidak tersedia)" : `heap Go ${formatBytes(p.heap_dipakai)}`} />
        <StatTile label="Jeda GC terakhir" value={formatMs(p.gc_jeda_terakhir_ms)} sub={`${formatRingkas(p.gc_jumlah)} siklus sejak mulai`} />
        <StatTile label="Aplikasi berjalan" value={formatDurasi(p.uptime_detik)} sub={p.versi_go} />
        <StatTile label="Berkas terbuka" value={p.fd_terbuka === null ? "-" : formatRingkas(p.fd_terbuka)} sub={p.fd_terbuka === null ? "tidak tersedia di platform ini" : "soket dan berkas"} />
        <StatTile label="Heap dari OS" value={formatBytes(p.heap_dari_os)} sub="yang diminta runtime untuk heap" />
        <StatTile label="Total dari OS" value={formatBytes(p.mem_dari_os)} sub="seluruh memori yang diminta runtime" />
      </div>

      <Panel judul="Permintaan HTTP" deskripsi="Waktu respons dihitung dari permintaan biasa; ekspor, impor, sinkronisasi, dan penarikan memang lama sehingga hanya dihitung jumlahnya. Rute dikelompokkan menurut pola, bukan alamat lengkap.">
        <div className="overflow-x-auto rounded-xl border border-slate-200">
          <table className="w-full min-w-max text-sm">
            <thead className="bg-slate-50 text-xs text-slate-500">
              <tr>
                <th scope="col" className="px-3 py-2 text-left font-medium">Jendela</th>
                <th scope="col" className="px-3 py-2 text-right font-medium">Permintaan</th>
                <th scope="col" className="px-3 py-2 text-right font-medium">Per detik</th>
                <th scope="col" className="px-3 py-2 text-right font-medium">4xx</th>
                <th scope="col" className="px-3 py-2 text-right font-medium">5xx</th>
                <th scope="col" className="px-3 py-2 text-right font-medium">Rata-rata</th>
                <th scope="col" className="px-3 py-2 text-right font-medium">p50</th>
                <th scope="col" className="px-3 py-2 text-right font-medium">p95</th>
                <th scope="col" className="px-3 py-2 text-right font-medium">p99</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {h.jendela.map((j) => (
                <tr key={j.nama} className="hover:bg-slate-50">
                  <th scope="row" className="whitespace-nowrap px-3 py-2 text-left font-medium text-slate-800">{j.nama}</th>
                  <td className="px-3 py-2 text-right tabular-nums">{formatRingkas(j.total)}</td>
                  <td className="px-3 py-2 text-right tabular-nums">{formatAngka(j.per_detik, 2)}</td>
                  <td className="px-3 py-2 text-right tabular-nums">{formatRingkas(j.galat_4xx)}</td>
                  <td className={`px-3 py-2 text-right tabular-nums ${j.galat_5xx > 0 ? "font-semibold text-red-700" : ""}`}>
                    {formatRingkas(j.galat_5xx)}
                    {j.galat_5xx > 0 && <span className="ml-1 text-xs font-normal">({formatAngka(j.persen_galat_5xx, 1)}%)</span>}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">{formatMs(j.rata_ms)}</td>
                  <td className="px-3 py-2 text-right tabular-nums">{formatMs(j.p50_ms)}</td>
                  <td className="px-3 py-2 text-right tabular-nums">{formatMs(j.p95_ms)}</td>
                  <td className="px-3 py-2 text-right tabular-nums">{formatMs(j.p99_ms)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="mt-2 text-xs text-slate-500">p95: 95% permintaan selesai lebih cepat dari angka ini.</p>
      </Panel>

      <div className="grid gap-4 xl:grid-cols-2">
        <Panel judul="Rute paling lambat" deskripsi="Diurutkan menurut p95 sejak aplikasi mulai (minimal 5 permintaan).">
          {h.rute_lambat.length === 0 ? <EmptyState compact title="Belum ada data rute" description="Rute muncul setelah dipakai beberapa kali." /> : <TabelRute rute={h.rute_lambat} />}
        </Panel>
        <Panel judul="Rute dengan galat server" deskripsi="Rute yang pernah menjawab galat 5xx sejak aplikasi mulai.">
          {h.rute_galat.length === 0 ? <EmptyState compact title="Tidak ada galat 5xx" description="Tidak ada rute yang menjawab galat server sejak aplikasi mulai." /> : <TabelRute rute={h.rute_galat} kolomGalat />}
        </Panel>
      </div>
    </div>
  );
}
