"use client";

import { useEffect, useState } from "react";
import axios from "axios";
import { ChevronLeft, ChevronRight, Search, Users } from "lucide-react";
import { type DaftarPenggunaAudit, getAuditPengguna } from "@/lib/audit";
import { type FilterAudit, PER_HALAMAN_AUDIT, kueriAudit, labelPeran, teksRelatif, waktuWIB } from "@/lib/auditFilter";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";
import { SkeletonTable } from "@/components/ui/Skeleton";

// Ringkasan aktivitas tiap pengguna pada rentang waktu yang dipilih (yang terakhir aktif lebih dulu): siapa saja yang aktif, berapa kali gagal, dan dari mana terakhir.
export function AuditPengguna({ filter, versi, onLihat }: { filter: FilterAudit; versi: number; onLihat: (userId: string, username: string) => void }) {
  const [cari, setCari] = useState("");
  const [kata, setKata] = useState("");
  const [halaman, setHalaman] = useState(1);
  const [data, setData] = useState<DaftarPenggunaAudit | null>(null);
  const [galat, setGalat] = useState("");
  const [memuat, setMemuat] = useState(false);

  useEffect(() => {
    const t = setTimeout(() => {
      setKata(cari.trim());
      setHalaman(1);
    }, 400);
    return () => clearTimeout(t);
  }, [cari]);

  // Hanya rentang waktu dan kata pencarian yang berlaku di tab ini.
  useEffect(() => {
    const ac = new AbortController();
    setMemuat(true);
    const k = kueriAudit({ ...filter, kategori: "", aksi: "", hasil: "semua", ip: "", q: "", user_id: "", username: kata, halaman }, new Date(), PER_HALAMAN_AUDIT);
    getAuditPengguna(k, ac.signal)
      .then((r) => {
        setData(r.data);
        setGalat("");
      })
      .catch((err) => {
        if (axios.isCancel(err)) return;
        setGalat(axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : "Gagal memuat ringkasan per pengguna");
      })
      .finally(() => {
        if (!ac.signal.aborted) setMemuat(false);
      });
    return () => ac.abort();
  }, [filter.rentang, filter.dari, filter.sampai, kata, halaman, versi]); // eslint-disable-line react-hooks/exhaustive-deps

  const sekarang = new Date();
  const jumlahHalaman = data ? Math.max(1, Math.ceil(data.total / data.per_halaman)) : 1;

  return (
    <div className="space-y-4">
      <div className="relative min-w-[14rem] sm:max-w-md">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
        <input
          type="search"
          value={cari}
          onChange={(e) => setCari(e.target.value)}
          maxLength={100}
          placeholder="Cari username"
          aria-label="Cari pengguna menurut username"
          className="w-full rounded-xl border border-slate-300 bg-white py-2.5 pl-9 pr-3 text-sm outline-none focus:border-blue-500 focus:shadow-glow"
        />
      </div>
      {galat && <Alert message={galat} />}
      {!data && !galat && <SkeletonTable rows={6} cols={6} />}
      {data && (
        <>
          <p className="text-sm text-slate-600" aria-live="polite">
            <span className="font-semibold text-slate-900">{data.total.toLocaleString("id-ID")}</span> pengguna tercatat aktif pada rentang ini
            {memuat && <span className="ml-2 text-xs text-slate-400">memuat…</span>}
          </p>
          {data.pengguna.length === 0 ? (
            <EmptyState icon={Users} title="Tidak ada pengguna yang cocok" description="Belum ada aktivitas pengguna pada rentang waktu ini, atau kata pencarian tidak cocok." />
          ) : (
            <div className={`overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm ${memuat ? "opacity-70" : ""}`}>
              <table className="w-full min-w-[52rem] text-sm">
                <thead className="bg-slate-50 text-xs text-slate-500">
                  <tr>
                    <th scope="col" className="px-4 py-2.5 text-left font-medium">Pengguna</th>
                    <th scope="col" className="px-4 py-2.5 text-right font-medium">Aktivitas</th>
                    <th scope="col" className="px-4 py-2.5 text-right font-medium">Gagal</th>
                    <th scope="col" className="px-4 py-2.5 text-left font-medium">Terakhir aktif</th>
                    <th scope="col" className="px-4 py-2.5 text-left font-medium">Login terakhir</th>
                    <th scope="col" className="px-4 py-2.5 text-left font-medium">IP terakhir</th>
                    <th scope="col" className="px-4 py-2.5 text-right font-medium"><span className="sr-only">Tindakan</span></th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {data.pengguna.map((p) => (
                    <tr key={p.user_id} className="hover:bg-slate-50">
                      <td className="max-w-[16rem] px-4 py-2.5">
                        <p className="truncate font-medium text-slate-900">{p.nama_lengkap || p.username}</p>
                        <p className="truncate text-xs text-slate-500">
                          {p.nama_lengkap ? `${p.username} · ` : ""}
                          {labelPeran(p.peran_terakhir) || "tanpa peran"}
                          {p.aktif === false && <span className="ml-1.5 rounded bg-slate-100 px-1.5 py-0.5 font-medium text-slate-600">nonaktif</span>}
                          {p.aktif === undefined && <span className="ml-1.5 rounded bg-slate-100 px-1.5 py-0.5 font-medium text-slate-600">akun dihapus</span>}
                        </p>
                      </td>
                      <td className="px-4 py-2.5 text-right tabular-nums">{p.jumlah.toLocaleString("id-ID")}</td>
                      <td className={`px-4 py-2.5 text-right tabular-nums ${p.gagal > 0 ? "font-semibold text-red-700" : "text-slate-400"}`}>{p.gagal.toLocaleString("id-ID")}</td>
                      <td className="whitespace-nowrap px-4 py-2.5 text-xs text-slate-700" title={waktuWIB(p.aktif_terakhir)}>
                        {teksRelatif(p.aktif_terakhir, sekarang)}
                      </td>
                      <td className="whitespace-nowrap px-4 py-2.5 text-xs text-slate-700" title={p.login_terakhir ? waktuWIB(p.login_terakhir) : undefined}>
                        {p.login_terakhir ? teksRelatif(p.login_terakhir, sekarang) : <span className="text-slate-300">-</span>}
                      </td>
                      <td className="whitespace-nowrap px-4 py-2.5 font-mono text-xs text-slate-600">{p.ip_terakhir || "-"}</td>
                      <td className="px-4 py-2.5 text-right">
                        <button type="button" onClick={() => onLihat(p.user_id, p.username)} className="text-xs font-medium text-blue-700 hover:underline">
                          Lihat aktivitas
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {data.total > data.per_halaman && (
            <nav aria-label="Halaman ringkasan per pengguna" className="flex items-center justify-between gap-3">
              <Button size="sm" variant="secondary" fullWidth={false} disabled={halaman <= 1 || memuat} icon={<ChevronLeft className="h-4 w-4" aria-hidden="true" />} onClick={() => setHalaman((h) => h - 1)}>
                Sebelumnya
              </Button>
              <span className="text-xs text-slate-500">
                Halaman {data.halaman} dari {jumlahHalaman.toLocaleString("id-ID")}
              </span>
              <Button size="sm" variant="secondary" fullWidth={false} disabled={halaman >= jumlahHalaman || memuat} onClick={() => setHalaman((h) => h + 1)}>
                Berikutnya
                <ChevronRight className="h-4 w-4" aria-hidden="true" />
              </Button>
            </nav>
          )}
        </>
      )}
    </div>
  );
}
