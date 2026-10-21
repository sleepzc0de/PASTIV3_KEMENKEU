"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import axios from "axios";
import { ChevronLeft, ChevronRight, Plus, RefreshCw, Search, Users } from "lucide-react";
import { type DaftarKlien, type InfoDataset, type KunciBaru, type Pengaturan, type StatusKlien, getDaftarKlien } from "@/lib/apiAkses";
import { KETERANGAN_STATUS, LABEL_STATUS, URUTAN_STATUS } from "@/lib/apiAksesFormat";
import { teksRelatif, waktuWIB } from "@/lib/auditFilter";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { SkeletonTable } from "@/components/ui/Skeleton";
import { useToast } from "@/components/ui/Toast";
import { KlienBaruModal } from "./KlienBaruModal";
import { KlienDetail } from "./KlienDetail";
import { KunciBaruModal } from "./KunciBaruModal";
import { LencanaStatus } from "./lencana";

const PER_HALAMAN = 25;

function pesanGalat(err: unknown, cadangan: string): string {
  return axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : cadangan;
}

// Daftar aplikasi klien API: penyaring status (dengan jumlahnya), pencarian, dan rincian per klien. Pendaftaran yang menunggu keputusan tampil paling atas.
export function KlienManager({ bawaan }: { bawaan: Pengaturan | null }) {
  const [status, setStatus] = useState<StatusKlien | "">("");
  const [cari, setCari] = useState("");
  const [q, setQ] = useState("");
  const [halaman, setHalaman] = useState(1);
  const [data, setData] = useState<DaftarKlien | null>(null);
  const [dataset, setDataset] = useState<InfoDataset[]>([]);
  const [galat, setGalat] = useState("");
  const [memuat, setMemuat] = useState(false);
  const [versi, setVersi] = useState(0);
  const [terbuka, setTerbuka] = useState<number | null>(null);
  const [bukaBaru, setBukaBaru] = useState(false);
  const [kunciBaru, setKunciBaru] = useState<{ kunci: KunciBaru; judul: string } | null>(null);
  const toast = useToast();
  const pertama = useRef(true);

  // Pencarian diterapkan setelah jeda supaya tidak meminta data pada setiap ketukan.
  useEffect(() => {
    if (pertama.current) {
      pertama.current = false;
      return;
    }
    const t = setTimeout(() => {
      setQ(cari.trim());
      setHalaman(1);
    }, 400);
    return () => clearTimeout(t);
  }, [cari]);

  useEffect(() => {
    const ac = new AbortController();
    setMemuat(true);
    const params: Record<string, string | number> = { halaman, per_halaman: PER_HALAMAN };
    if (status) params.status = status;
    if (q) params.q = q;
    getDaftarKlien(params, ac.signal)
      .then((r) => {
        setData(r);
        setDataset(r.dataset);
        setGalat("");
      })
      .catch((err) => {
        if (axios.isCancel(err)) return;
        setGalat(pesanGalat(err, "Gagal memuat daftar klien"));
      })
      .finally(() => {
        if (!ac.signal.aborted) setMemuat(false);
      });
    return () => ac.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status, q, halaman, versi]);

  const muatUlang = useCallback(() => setVersi((v) => v + 1), []);
  const jumlahHalaman = data ? Math.max(1, Math.ceil(data.total / PER_HALAMAN)) : 1;
  const sekarang = new Date();

  return (
    <div className="space-y-4">
      <p className="rounded-xl border border-blue-100 bg-blue-50/60 px-4 py-3 text-sm text-slate-700">
        Aplikasi lain <b>didaftarkan oleh superadmin</b>: pihak luar menghubungi administrator PASTI (tidak ada formulir pendaftaran mandiri), lalu Anda mendaftarkannya di sini. Kunci API terbit saat klien
        dibuat dan hanya ditampilkan sekali. Setiap klien hanya dapat membaca dataset yang Anda izinkan, dengan batas laju dan kuota yang dapat diatur per klien. Semua keputusan tercatat di Log Audit.
      </p>

      <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Saring menurut status">
        <button
          type="button"
          onClick={() => { setStatus(""); setHalaman(1); }}
          aria-pressed={status === ""}
          className={`rounded-full px-3.5 py-1.5 text-sm font-medium ring-1 ring-inset transition ${status === "" ? "bg-blue-600 text-white ring-blue-600" : "bg-white text-slate-600 ring-slate-300 hover:bg-slate-50"}`}
        >
          Semua
        </button>
        {URUTAN_STATUS.map((s) => (
          <button
            key={s}
            type="button"
            title={KETERANGAN_STATUS[s]}
            onClick={() => { setStatus(s); setHalaman(1); }}
            aria-pressed={status === s}
            className={`rounded-full px-3.5 py-1.5 text-sm font-medium ring-1 ring-inset transition ${status === s ? "bg-blue-600 text-white ring-blue-600" : "bg-white text-slate-600 ring-slate-300 hover:bg-slate-50"}`}
          >
            {LABEL_STATUS[s]}
            {data && <span className={`ml-1.5 tabular-nums ${status === s ? "text-blue-100" : s === "menunggu" && data.per_status[s] > 0 ? "font-bold text-amber-700" : "text-slate-400"}`}>{data.per_status[s] ?? 0}</span>}
          </button>
        ))}
        <div className="relative ml-auto min-w-[14rem] flex-1 sm:max-w-xs">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
          <input
            type="search"
            value={cari}
            onChange={(e) => setCari(e.target.value)}
            maxLength={100}
            placeholder="Cari aplikasi, instansi, PIC, atau kode"
            aria-label="Cari klien API"
            className="w-full rounded-xl border border-slate-300 bg-white py-2.5 pl-9 pr-3 text-sm outline-none focus:border-blue-500 focus:shadow-glow"
          />
        </div>
        <Button size="sm" variant="secondary" fullWidth={false} icon={<RefreshCw className="h-4 w-4" aria-hidden="true" />} onClick={muatUlang}>
          Segarkan
        </Button>
        <Button size="sm" fullWidth={false} disabled={!data} icon={<Plus className="h-4 w-4" aria-hidden="true" />} onClick={() => setBukaBaru(true)}>
          Daftarkan aplikasi
        </Button>
      </div>

      {galat && <Alert message={galat} />}
      {!data && !galat && <SkeletonTable rows={6} cols={6} />}
      {data && (
        <>
          <p className="text-sm text-slate-600" aria-live="polite">
            <span className="font-semibold text-slate-900">{data.total.toLocaleString("id-ID")}</span> klien
            {memuat && <span className="ml-2 text-xs text-slate-400">memuat…</span>}
            {data.total > 0 && <span className="text-slate-400"> · halaman {data.halaman} dari {jumlahHalaman.toLocaleString("id-ID")}</span>}
          </p>
          {data.klien.length === 0 ? (
            <EmptyState icon={Users} title="Belum ada klien" description={status || q ? "Ubah atau hapus penyaring." : "Belum ada aplikasi yang didaftarkan. Pilih Daftarkan aplikasi untuk menambah klien API pertama."} />
          ) : (
            <div className={`overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm ${memuat ? "opacity-70" : ""}`}>
              <table className="w-full min-w-[52rem] text-sm">
                <thead className="bg-slate-50 text-xs text-slate-500">
                  <tr>
                    <th scope="col" className="px-4 py-2.5 text-left font-medium">Aplikasi</th>
                    <th scope="col" className="px-4 py-2.5 text-left font-medium">Instansi dan PIC</th>
                    <th scope="col" className="px-4 py-2.5 text-left font-medium">Dataset</th>
                    <th scope="col" className="px-4 py-2.5 text-left font-medium">Status</th>
                    <th scope="col" className="px-4 py-2.5 text-left font-medium">Didaftarkan</th>
                    <th scope="col" className="px-4 py-2.5 text-left font-medium">Terakhir dipakai</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {data.klien.map((k) => (
                    <tr
                      key={k.id}
                      tabIndex={0}
                      role="button"
                      aria-label={`Lihat rincian ${k.nama_aplikasi}`}
                      onClick={() => setTerbuka(k.id)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          setTerbuka(k.id);
                        }
                      }}
                      className={`cursor-pointer outline-none hover:bg-slate-50 focus-visible:bg-blue-50/60 ${k.status === "menunggu" ? "bg-amber-50/30" : ""}`}
                    >
                      <td className="max-w-[16rem] px-4 py-2.5">
                        <p className="truncate font-medium text-slate-900">{k.nama_aplikasi}</p>
                        <p className="font-mono text-[11px] text-slate-400">{k.kode}</p>
                      </td>
                      <td className="max-w-[16rem] px-4 py-2.5">
                        <p className="truncate text-slate-800">{k.instansi}</p>
                        <p className="truncate text-xs text-slate-500">{k.pic_nama}</p>
                      </td>
                      <td className="whitespace-nowrap px-4 py-2.5 text-xs text-slate-600">
                        {k.status === "menunggu" || k.status === "ditolak" ? `${k.dataset_diminta.length} diminta` : `${k.dataset_diberikan.length} dataset`}
                      </td>
                      <td className="px-4 py-2.5">
                        <LencanaStatus status={k.status} />
                      </td>
                      <td className="whitespace-nowrap px-4 py-2.5 text-xs text-slate-600" title={waktuWIB(k.didaftarkan_at)}>
                        {teksRelatif(k.didaftarkan_at, sekarang)}
                      </td>
                      <td className="whitespace-nowrap px-4 py-2.5 text-xs text-slate-600">{k.terakhir_dipakai ? teksRelatif(k.terakhir_dipakai, sekarang) : <span className="text-slate-400">Belum pernah</span>}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {jumlahHalaman > 1 && (
            <div className="flex items-center justify-end gap-2">
              <Button size="sm" variant="secondary" fullWidth={false} disabled={halaman <= 1 || memuat} icon={<ChevronLeft className="h-4 w-4" aria-hidden="true" />} onClick={() => setHalaman((h) => Math.max(1, h - 1))}>
                Sebelumnya
              </Button>
              <Button size="sm" variant="secondary" fullWidth={false} disabled={halaman >= jumlahHalaman || memuat} onClick={() => setHalaman((h) => h + 1)}>
                Berikutnya
                <ChevronRight className="h-4 w-4" aria-hidden="true" />
              </Button>
            </div>
          )}
        </>
      )}

      {bukaBaru && (
        <KlienBaruModal
          dataset={dataset}
          bawaan={bawaan}
          onTutup={() => setBukaBaru(false)}
          onSukses={(klien, kunci) => {
            setBukaBaru(false);
            if (kunci) setKunciBaru({ kunci, judul: `Klien ${klien.nama_aplikasi} didaftarkan: kunci API pertama` });
            toast.success("Klien didaftarkan dan disetujui");
            muatUlang();
          }}
        />
      )}
      {kunciBaru && <KunciBaruModal kunci={kunciBaru.kunci.kunci} judul={kunciBaru.judul} onTutup={() => setKunciBaru(null)} />}
      {terbuka !== null && <KlienDetail id={terbuka} dataset={dataset} bawaan={bawaan} onTutup={() => setTerbuka(null)} onBerubah={muatUlang} />}
    </div>
  );
}
