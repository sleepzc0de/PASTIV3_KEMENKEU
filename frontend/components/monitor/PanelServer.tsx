"use client";

import type { RingkasanMonitor } from "@/lib/monitor";
import { formatAngka, formatBytes, formatDurasi, formatPersen, statusDariPersen } from "@/lib/monitorFormat";
import { Alert } from "@/components/ui/Alert";
import { BarisInfo, Panel, Pengukur } from "./bagian";

// Keadaan server (atau container) tempat aplikasi berjalan.
export function PanelServer({ data }: { data: RingkasanMonitor }) {
  const s = data.sistem;
  const swapPersen = s.swap_total > 0 ? (s.swap_terpakai / s.swap_total) * 100 : null;
  const k = s.kontainer;
  return (
    <div className="space-y-4">
      <Panel judul="Pemakaian saat ini" deskripsi="Angka server (atau host container). Memori memakai definisi 'terpakai' = total dikurangi yang tersedia, sehingga cache disk tidak dihitung.">
        <div className="grid gap-5 sm:grid-cols-2">
          <Pengukur label="CPU server" nilai={s.cpu_persen} teks={s.cpu_persen === null ? "mengukur…" : formatPersen(s.cpu_persen)} status={statusDariPersen(s.cpu_persen, 60, 80)} sub={`${s.cpu_jumlah} core`} />
          <Pengukur
            label="Memori (RAM)"
            nilai={s.mem_total > 0 ? s.mem_persen : null}
            teks={s.mem_total > 0 ? formatPersen(s.mem_persen) : "-"}
            status={s.mem_total > 0 ? statusDariPersen(s.mem_persen, 85, 92) : "tidak_ada_data"}
            sub={s.mem_total > 0 ? `${formatBytes(s.mem_terpakai)} dari ${formatBytes(s.mem_total)} · tersedia ${formatBytes(s.mem_tersedia)}` : undefined}
          />
          <Pengukur
            label={`Disk (${s.disk_jalur || "-"})`}
            nilai={s.disk_total > 0 ? s.disk_persen : null}
            teks={s.disk_total > 0 ? formatPersen(s.disk_persen) : "-"}
            status={s.disk_total > 0 ? statusDariPersen(s.disk_persen, 80, 90) : "tidak_ada_data"}
            sub={s.disk_total > 0 ? `${formatBytes(s.disk_terpakai)} dari ${formatBytes(s.disk_total)} · sisa ${formatBytes(s.disk_bebas)}` : undefined}
          />
          <Pengukur
            label="Swap"
            nilai={swapPersen}
            teks={swapPersen === null ? "tidak ada swap" : formatPersen(swapPersen)}
            status={swapPersen === null ? "tidak_ada_data" : statusDariPersen(swapPersen, 10, 25)}
            sub={swapPersen === null ? undefined : `${formatBytes(s.swap_terpakai)} dari ${formatBytes(s.swap_total)}; swap terpakai berarti memori sempat penuh`}
          />
        </div>
      </Panel>

      <div className="grid gap-4 lg:grid-cols-2">
        <Panel judul="Server">
          <BarisInfo
            baris={[
              ["Sistem operasi", `${s.os} (${s.arsitektur})`],
              ["Jumlah core", String(s.cpu_jumlah)],
              ["Beban rata-rata (1 / 5 / 15 menit)", s.load1 === null ? "tidak tersedia di platform ini" : `${formatAngka(s.load1, 2)} / ${formatAngka(s.load5, 2)} / ${formatAngka(s.load15, 2)}`],
              ["Beban per core (1 menit)", s.load1 === null ? "-" : formatAngka(s.load1 / Math.max(1, s.cpu_jumlah), 2)],
              ["CPU proses aplikasi", s.cpu_proses_persen === null ? "-" : `${formatAngka(s.cpu_proses_persen, 1)}% dari satu core`],
              ["Server menyala sejak", s.uptime_detik === null ? "-" : formatDurasi(s.uptime_detik) + " lalu"],
            ]}
          />
          <p className="mt-2 text-xs text-slate-500">Beban rata-rata di atas 1 per core berarti ada proses yang antre menunggu CPU.</p>
        </Panel>

        <Panel judul="Batas container" deskripsi="Batas yang dipasang pada container aplikasi (Docker). Tanpa batas, container boleh memakai seluruh server.">
          {k ? (
            <BarisInfo
              baris={[
                ["Batas memori", k.mem_batas === null ? "tanpa batas" : formatBytes(k.mem_batas)],
                ["Memori terpakai container", k.mem_pakai === null ? "-" : formatBytes(k.mem_pakai)],
                ["Batas CPU", k.cpu_batas === null ? "tanpa batas" : `${formatAngka(k.cpu_batas, 1)} core`],
              ]}
            />
          ) : (
            <p className="text-sm text-slate-600">Tidak ada batas resource yang terdeteksi (tidak berjalan di container, atau container tanpa batas CPU dan memori).</p>
          )}
        </Panel>
      </div>

      {s.catatan && s.catatan.length > 0 && <Alert tone="info" message={s.catatan.join(" ")} />}
    </div>
  );
}
