"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ChevronLeft, ChevronRight, Plus, Search, Loader2, CheckCircle2 } from "lucide-react";
import { SapaHalaman, SapaRingkasan, SapaSatkerRef, SapaSaya, cariSapaSatker, createSapaPenjualan, listSapaPenjualan } from "@/lib/sapa";
import { Alert } from "@/components/ui/Alert";
import { ModalShell } from "@/components/ui/ModalShell";
import { formatDateTime } from "@/lib/dasbor";
import { Segmented } from "../digitalisasi/controls";
import { PeranBadge } from "./badges";
import { ErrorBox, NoticeBox, PrimaryButton, SecondaryButton, TextField, inputCls } from "./fields";
import { useSapa } from "./SapaGate";
import { ErrorInfo, bersihKodeSatker, errorInfo, peranLabel } from "./sapa";

const PER_HALAMAN = 20;

type StatusFilter = "" | "berjalan" | "selesai";

// Daftar usulan penjualan yang boleh dilihat pengguna (satker: satkernya; UE1: di bawahnya; kanwil dan admin: semua).
export function PenjualanList() {
  const saya = useSapa();
  const [q, setQ] = useState("");
  const [cari, setCari] = useState("");
  const [status, setStatus] = useState<StatusFilter>("");
  const [halaman, setHalaman] = useState(1);
  const [data, setData] = useState<SapaHalaman | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [memuat, setMemuat] = useState(true);
  const [buatBaru, setBuatBaru] = useState(false);
  const idPermintaan = useRef(0);

  // Pencarian ditunda sebentar setelah mengetik supaya tidak memanggil server di setiap huruf.
  useEffect(() => {
    const t = setTimeout(() => {
      setCari(q.trim());
      setHalaman(1);
    }, 350);
    return () => clearTimeout(t);
  }, [q]);

  const muat = useCallback(async () => {
    const id = ++idPermintaan.current;
    setMemuat(true);
    try {
      const res = await listSapaPenjualan({ q: cari || undefined, status: status || undefined, halaman, per_halaman: PER_HALAMAN });
      if (id === idPermintaan.current) {
        setData(res.data);
        setError(null);
      }
    } catch (err) {
      if (id === idPermintaan.current) setError(errorInfo(err, "Gagal memuat daftar usulan").message);
    } finally {
      if (id === idPermintaan.current) setMemuat(false);
    }
  }, [cari, status, halaman]);

  useEffect(() => {
    muat();
  }, [muat]);

  const totalHalaman = data ? Math.max(1, Math.ceil(data.total / data.per_halaman)) : 1;

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="text-sm text-slate-600">
          Masuk sebagai <span className="font-semibold text-slate-900">{saya.nama}</span>{" "}
          {saya.admin ? <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-700">Admin</span> : <PeranBadge peran={saya.peran} />}
          {!saya.admin && kodeCakupan(saya) && <span className="ml-2 font-mono text-xs text-slate-500">{kodeCakupan(saya)}</span>}
        </div>
        {saya.boleh_membuat && (
          <PrimaryButton onClick={() => setBuatBaru(true)}>
            <Plus className="h-4 w-4" aria-hidden="true" />
            Buat usulan penjualan
          </PrimaryButton>
        )}
      </div>

      <div className="flex flex-wrap items-end gap-3">
        <div className="min-w-[14rem] flex-1">
          <label htmlFor="sapa-cari" className="mb-1.5 block text-xs font-medium text-slate-600">
            Cari
          </label>
          <div className="relative">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" aria-hidden="true" />
            <input
              id="sapa-cari"
              type="search"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              maxLength={100}
              placeholder="Noreg, nama atau kode satker"
              className={inputCls + " pl-9"}
            />
          </div>
        </div>
        <Segmented
          label="Status usulan"
          value={status}
          onChange={(v) => {
            setStatus(v);
            setHalaman(1);
          }}
          options={[
            { value: "", label: "Semua" },
            { value: "berjalan", label: "Berjalan" },
            { value: "selesai", label: "Selesai" },
          ]}
        />
      </div>

      {error && (
        <div className="space-y-2">
          <Alert message={error} />
          <button type="button" onClick={muat} className="text-sm font-medium text-blue-600 hover:text-blue-700">
            Coba lagi
          </button>
        </div>
      )}

      {!data && !error && <div role="status" aria-label="Memuat daftar usulan" className="h-48 animate-pulse rounded-xl bg-slate-100" />}

      {data && (
        <div className={memuat ? "opacity-60 transition-opacity" : "transition-opacity"} aria-busy={memuat}>
          {data.usulan.length === 0 ? (
            <div className="rounded-xl border border-dashed border-slate-300 px-4 py-10 text-center text-sm text-slate-500">
              {cari || status ? "Tidak ada usulan yang cocok dengan pencarian." : saya.boleh_membuat ? "Belum ada usulan penjualan. Mulai dengan tombol “Buat usulan penjualan”." : "Belum ada usulan penjualan yang dapat Anda lihat."}
            </div>
          ) : (
            <ul className="space-y-2.5">
              {data.usulan.map((u) => (
                <BarisUsulan key={u.id} u={u} sayaPeran={saya.peran} admin={saya.admin} />
              ))}
            </ul>
          )}

          {data.total > data.per_halaman && (
            <nav aria-label="Halaman" className="mt-4 flex items-center justify-between text-sm text-slate-600">
              <span>
                {data.total} usulan · halaman {data.halaman} dari {totalHalaman}
              </span>
              <div className="flex gap-2">
                <SecondaryButton onClick={() => setHalaman((h) => Math.max(1, h - 1))} disabled={halaman <= 1 || memuat}>
                  <ChevronLeft className="h-4 w-4" aria-hidden="true" />
                  Sebelumnya
                </SecondaryButton>
                <SecondaryButton onClick={() => setHalaman((h) => h + 1)} disabled={halaman >= totalHalaman || memuat}>
                  Berikutnya
                  <ChevronRight className="h-4 w-4" aria-hidden="true" />
                </SecondaryButton>
              </div>
            </nav>
          )}
        </div>
      )}

      {buatBaru && <BuatUsulanModal onClose={() => setBuatBaru(false)} saya={saya} />}
    </div>
  );
}

function BarisUsulan({ u, sayaPeran, admin }: { u: SapaRingkasan; sayaPeran: string; admin: boolean }) {
  const persen = Math.round((u.tahap_selesai / u.tahap_total) * 100);
  const giliranSaya = !u.selesai && (admin || u.peran_saat_ini === sayaPeran);
  return (
    <li>
      <Link
        href={`/dashboard/sapa/penjualan/${u.id}`}
        className="block rounded-xl border border-slate-200 bg-white p-3.5 transition hover:border-blue-300 hover:shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 sm:p-4"
      >
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="min-w-0">
            <p className="text-sm font-bold text-slate-900">{u.noreg}</p>
            <p className="mt-0.5 break-words text-sm text-slate-700">{u.nama_satker}</p>
            <p className="mt-0.5 font-mono text-xs text-slate-500">{u.kode_satker}</p>
          </div>
          <div className="flex flex-wrap items-center gap-1.5">
            {u.selesai ? (
              <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700">
                <CheckCircle2 className="h-3.5 w-3.5" aria-hidden="true" />
                Selesai
              </span>
            ) : (
              <>
                {giliranSaya && <span className="rounded-full bg-blue-600 px-2 py-0.5 text-[11px] font-semibold text-white">Menunggu tindakan Anda</span>}
                <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-600">Berjalan</span>
              </>
            )}
          </div>
        </div>
        <div className="mt-3 grid gap-3 sm:grid-cols-[1fr_12rem] sm:items-center">
          <p className="min-w-0 text-xs text-slate-600">
            {u.selesai ? (
              "Seluruh tahap selesai"
            ) : (
              <>
                Tahap saat ini: <span className="font-medium text-slate-800">{u.tahap_saat_ini_label}</span> <span className="text-slate-400">·</span> {peranLabel(u.peran_saat_ini)}
              </>
            )}
          </p>
          <div>
            <div className="mb-1 flex justify-between text-[11px] text-slate-500">
              <span>
                {u.tahap_selesai}/{u.tahap_total} tahap
              </span>
              <span>{formatDateTime(u.diperbarui_pada)}</span>
            </div>
            <div className="h-1.5 overflow-hidden rounded-full bg-slate-200" role="progressbar" aria-valuemin={0} aria-valuemax={u.tahap_total} aria-valuenow={u.tahap_selesai} aria-label={`Kemajuan ${u.noreg}`}>
              <div className="h-full rounded-full bg-emerald-500" style={{ width: `${persen}%` }} />
            </div>
          </div>
        </div>
      </Link>
    </li>
  );
}

// Kode peran aplikasi yang membatasi usulan pengguna: 6 digit (Satker), 9 digit (Kanwil), atau 5 digit (UE1).
function kodeCakupan(saya: SapaSaya): string {
  if (saya.peran === "satker") return saya.kode_satker6;
  if (saya.peran === "kanwil") return saya.kode_kanwil;
  if (saya.peran === "ue1") return saya.kode_ue1;
  return "";
}

function BuatUsulanModal({ onClose, saya }: { onClose: () => void; saya: SapaSaya }) {
  const router = useRouter();
  // Peran Satker hanya boleh membuat usulan untuk satkernya (kode 6 digit). Kode lengkap diisi dari data aset bila ada; bila satkernya belum ada di
  // data aset, kodenya diketik dan backend memastikan karakter ke-10 sampai ke-15 sama dengan kode peran.
  const batasSatker = saya.peran === "satker" && !saya.admin;
  const pilihan = batasSatker ? saya.satker_pilihan : [];
  const kunci = batasSatker && pilihan.length === 1;
  const [kode, setKode] = useState(bersihKodeSatker(batasSatker ? saya.kode_satker : ""));
  const [ref, setRef] = useState<SapaSatkerRef | null>(null);
  const [mencari, setMencari] = useState(false);
  const [galatCari, setGalatCari] = useState<string | null>(null);
  const [nama, setNama] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ErrorInfo | null>(null);
  const lengkap = kode.length === 18;

  // Setelah 18 digit terisi, nama satker dan UE1 dicari di data Digitalisasi Aset.
  useEffect(() => {
    setRef(null);
    setGalatCari(null);
    if (!lengkap) return;
    let batal = false;
    setMencari(true);
    cariSapaSatker(kode)
      .then((res) => {
        if (!batal) setRef(res.data);
      })
      .catch((err) => {
        if (!batal) setGalatCari(errorInfo(err, "Gagal mencari satker").message);
      })
      .finally(() => {
        if (!batal) setMencari(false);
      });
    return () => {
      batal = true;
    };
  }, [kode, lengkap]);

  const satker = ref?.satker ?? null;
  const butuhNama = lengkap && !mencari && ref !== null && satker === null;
  const bisaKirim = lengkap && !mencari && ref !== null && (satker !== null || nama.trim() !== "");

  const kirim = async () => {
    setBusy(true);
    setError(null);
    try {
      const res = await createSapaPenjualan({ kode_satker: kode, nama_satker: satker ? undefined : nama.trim() });
      router.push(`/dashboard/sapa/penjualan/${res.data.id}`);
    } catch (err) {
      setError(errorInfo(err, "Gagal membuat usulan"));
      setBusy(false);
    }
  };

  return (
    <ModalShell title="Buat usulan penjualan" subtitle="Usulan baru mendapat Noreg dan dimulai dari tahap Pembentukan Tim" onClose={onClose}>
      <div className="space-y-4 p-4 sm:p-6">
        <TextField
          label="Kode satker (18 digit)"
          required
          value={kode}
          onChange={(v) => setKode(bersihKodeSatker(v))}
          inputMode="numeric"
          maxLength={24}
          disabled={kunci}
          hint={
            kunci
              ? "Usulan hanya dapat dibuat untuk satker Anda."
              : batasSatker
                ? `Kode harus memuat kode satker Anda (${saya.kode_satker6}) pada karakter ke-10 sampai ke-15. Akhiran KP dibuang otomatis.`
                : "Tempel kode satker; akhiran KP dibuang otomatis."
          }
        />
        {pilihan.length > 1 && (
          <div className="flex flex-wrap gap-2" role="group" aria-label="Pilih satker Anda">
            {pilihan.map((p) => (
              <button
                key={p.kode}
                type="button"
                onClick={() => setKode(bersihKodeSatker(p.kode))}
                aria-pressed={kode === bersihKodeSatker(p.kode)}
                className="rounded-lg border border-slate-200 px-2.5 py-1 text-left text-xs text-slate-700 hover:bg-slate-50 aria-pressed:border-blue-500 aria-pressed:bg-blue-50"
              >
                <span className="font-mono">{bersihKodeSatker(p.kode)}</span> <span className="text-slate-500">{p.nama}</span>
              </button>
            ))}
          </div>
        )}
        {lengkap && mencari && (
          <p className="inline-flex items-center gap-2 text-sm text-slate-500">
            <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
            Mencari satker…
          </p>
        )}
        {galatCari && <Alert message={galatCari} />}
        {satker && (
          <dl className="grid gap-x-6 gap-y-2 rounded-lg border border-emerald-200 bg-emerald-50/60 p-3 text-sm sm:grid-cols-2">
            <div className="sm:col-span-2">
              <dt className="text-xs text-slate-500">Nama satker</dt>
              <dd className="font-semibold text-slate-900">{satker.nama}</dd>
            </div>
            <div>
              <dt className="text-xs text-slate-500">Kabupaten/kota</dt>
              <dd className="text-slate-800">{satker.kab_kota || "-"}</dd>
            </div>
            <div>
              <dt className="text-xs text-slate-500">Unit Eselon I</dt>
              <dd className="text-slate-800">{ref?.ue1 ? ref.ue1.nama : `Kode ${ref?.kode_ue1 ?? ""}`}</dd>
            </div>
          </dl>
        )}
        {ref && !ref.ue1 && lengkap && (
          <NoticeBox tone="warn">
            Referensi Unit Eselon I untuk kode {ref.kode_ue1} belum diisi admin, jadi tujuan Nota Dinas (Sekretaris UE1) harus diketik manual. Admin dapat melengkapinya di Pengaturan SAPA.
          </NoticeBox>
        )}
        {butuhNama && (
          <>
            <NoticeBox tone="warn">Kode ini tidak ditemukan di data Digitalisasi Aset (belum disinkronkan atau bukan satker KL 015). Isi nama satker secara manual.</NoticeBox>
            <TextField label="Nama satker" required value={nama} onChange={setNama} maxLength={300} />
          </>
        )}
        <ErrorBox error={error} />
        <div className="flex justify-end gap-2 border-t border-slate-100 pt-4">
          <SecondaryButton onClick={onClose}>Batal</SecondaryButton>
          <PrimaryButton onClick={kirim} busy={busy} disabled={!bisaKirim}>
            Buat usulan
          </PrimaryButton>
        </div>
      </div>
    </ModalShell>
  );
}
