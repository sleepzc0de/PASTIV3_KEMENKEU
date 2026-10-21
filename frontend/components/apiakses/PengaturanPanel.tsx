"use client";

import { useEffect, useState } from "react";
import axios from "axios";
import { Gauge, RotateCcw, Save } from "lucide-react";
import { type Pengaturan, type RentangPengaturan, getPengaturan, putPengaturan } from "@/lib/apiAkses";
import { teksBatas, teksPeriode } from "@/lib/apiAksesFormat";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { SkeletonCards } from "@/components/ui/Skeleton";
import { useToast } from "@/components/ui/Toast";

const inputCls =
  "w-full rounded-xl border bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition-all placeholder:text-slate-400 hover:border-slate-400 focus:border-blue-500 focus:shadow-glow";

type Teks = Record<keyof Omit<Pengaturan, "pendaftaran_dibuka">, string>;

const KOLOM: { kunci: keyof Teks; label: string; satuan: string; bantuan: string }[] = [
  { kunci: "limit_permintaan", label: "Batas laju: permintaan", satuan: "permintaan", bantuan: "Jumlah permintaan yang boleh diterima dalam satu jendela (jendela geser, akurat per permintaan)." },
  { kunci: "limit_jendela_detik", label: "Batas laju: jendela", satuan: "detik", bantuan: "Panjang jendela. Bawaan 1.000 permintaan per 60 detik." },
  { kunci: "kuota_permintaan", label: "Kuota: permintaan", satuan: "permintaan", bantuan: "Jumlah permintaan yang boleh diterima dalam satu periode kuota." },
  { kunci: "kuota_periode_detik", label: "Kuota: periode", satuan: "detik", bantuan: "Panjang periode; kuota pulih tiap kelipatan periode (3600 = tiap jam tepat). Bawaan 5.000 per jam." },
  { kunci: "halaman_maks", label: "Baris maksimal per permintaan", satuan: "baris", bantuan: "Batas ukuran satu halaman data." },
  { kunci: "kunci_masa_berlaku_hari", label: "Masa berlaku kunci baru", satuan: "hari", bantuan: "0 = kunci tidak kedaluwarsa. Berlaku untuk kunci yang diterbitkan setelah ini." },
  { kunci: "rotasi_tenggang_jam", label: "Masa tenggang rotasi", satuan: "jam", bantuan: "Kunci lama tetap berlaku sekian jam setelah rotasi (0 = langsung dihentikan)." },
  { kunci: "log_retensi_hari", label: "Simpan riwayat permintaan", satuan: "hari", bantuan: "Riwayat per permintaan dihapus otomatis setelah sekian hari." },
  { kunci: "agregat_retensi_hari", label: "Simpan hitungan per jam", satuan: "hari", bantuan: "Dasar grafik dan ringkasan jangka panjang." },
];

function keTeks(p: Pengaturan): Teks {
  const t = {} as Teks;
  for (const k of KOLOM) t[k.kunci] = String(p[k.kunci]);
  return t;
}

// Pengaturan global batas laju dan kuota (bawaan semua klien), plus masa berlaku kunci, tenggang rotasi, dan retensi. Tersimpan di database dan berlaku dalam hitungan
// detik tanpa deploy ulang. Klien yang punya batas khusus tidak terpengaruh.
export function PengaturanPanel({ onTersimpan }: { onTersimpan?: (p: Pengaturan) => void }) {
  const toast = useToast();
  const [awal, setAwal] = useState<Pengaturan | null>(null);
  const [pabrik, setPabrik] = useState<Pengaturan | null>(null);
  const [rentang, setRentang] = useState<RentangPengaturan>({});
  const [teks, setTeks] = useState<Teks | null>(null);
  const [dibuka, setDibuka] = useState(true);
  const [galat, setGalat] = useState("");
  const [galatKolom, setGalatKolom] = useState<Record<string, string>>({});
  const [sibuk, setSibuk] = useState(false);

  useEffect(() => {
    getPengaturan()
      .then((r) => {
        setAwal(r.pengaturan);
        setPabrik(r.bawaan_pabrik);
        setRentang(r.rentang);
        setTeks(keTeks(r.pengaturan));
        setDibuka(r.pengaturan.pendaftaran_dibuka);
      })
      .catch((err) => setGalat(axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : "Gagal memuat pengaturan"));
  }, []);

  if (galat) return <Alert message={galat} />;
  if (!awal || !teks || !pabrik) return <SkeletonCards count={3} />;

  const susun = (): Pengaturan | null => {
    const g: Record<string, string> = {};
    const nilai = { pendaftaran_dibuka: dibuka } as Pengaturan;
    for (const k of KOLOM) {
      const s = teks[k.kunci].trim();
      const r = rentang[k.kunci];
      if (!/^\d+$/.test(s) || !Number.isSafeInteger(Number(s))) {
        g[k.kunci] = "Isi dengan bilangan bulat";
        continue;
      }
      const n = Number(s);
      if (r && (n < r.min || n > r.maks)) g[k.kunci] = `Harus antara ${r.min.toLocaleString("id-ID")} dan ${r.maks.toLocaleString("id-ID")}`;
      (nilai as unknown as Record<string, number>)[k.kunci] = n;
    }
    setGalatKolom(g);
    return Object.keys(g).length ? null : nilai;
  };

  const simpan = async () => {
    const p = susun();
    if (!p) return;
    setSibuk(true);
    try {
      const r = await putPengaturan(p);
      setAwal(r.data.pengaturan);
      setTeks(keTeks(r.data.pengaturan));
      onTersimpan?.(r.data.pengaturan);
      toast.success("Pengaturan disimpan dan langsung berlaku");
    } catch (err) {
      toast.error(axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : "Gagal menyimpan pengaturan");
    } finally {
      setSibuk(false);
    }
  };

  const pabrikkan = () => {
    setTeks(keTeks(pabrik));
    setDibuka(pabrik.pendaftaran_dibuka);
    setGalatKolom({});
  };

  const berubah = KOLOM.some((k) => teks[k.kunci].trim() !== String(awal[k.kunci])) || dibuka !== awal.pendaftaran_dibuka;
  const pratinjau = (() => {
    const n = (k: keyof Teks) => Number(teks[k]);
    if ([n("limit_permintaan"), n("limit_jendela_detik"), n("kuota_permintaan"), n("kuota_periode_detik")].some((v) => !Number.isFinite(v) || v <= 0)) return "-";
    return teksBatas({ limit_permintaan: n("limit_permintaan"), limit_jendela_detik: n("limit_jendela_detik"), kuota_permintaan: n("kuota_permintaan"), kuota_periode_detik: n("kuota_periode_detik") });
  })();

  return (
    <div className="space-y-5">
      <p className="rounded-xl border border-blue-100 bg-blue-50/60 px-4 py-3 text-sm text-slate-700">
        Nilai di sini adalah <b>bawaan untuk semua klien</b>. Tersimpan di database dan berlaku dalam beberapa detik tanpa deploy ulang. Klien yang diberi <b>batas khusus</b> (di rincian klien) tidak terpengaruh. Batas laju
        memakai jendela geser; kuota memakai periode tetap yang pulih tiap kelipatan periode.
      </p>

      <div className="flex items-center gap-3 rounded-2xl border border-slate-200 bg-slate-50/60 px-4 py-3">
        <Gauge className="h-5 w-5 shrink-0 text-blue-600" aria-hidden="true" />
        <div className="min-w-0">
          <p className="text-xs font-medium text-slate-500">Bawaan yang akan berlaku</p>
          <p className="break-words text-sm font-semibold text-slate-900">{pratinjau}</p>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {KOLOM.map((k) => (
          <div key={k.kunci}>
            <label htmlFor={`pg-${k.kunci}`} className="mb-1 block text-xs font-medium text-slate-700">
              {k.label} <span className="font-normal text-slate-400">({k.satuan})</span>
            </label>
            <input
              id={`pg-${k.kunci}`}
              inputMode="numeric"
              value={teks[k.kunci]}
              onChange={(e) => setTeks({ ...teks, [k.kunci]: e.target.value })}
              aria-invalid={galatKolom[k.kunci] ? true : undefined}
              className={`${inputCls} ${galatKolom[k.kunci] ? "border-red-400" : "border-slate-300"}`}
            />
            {galatKolom[k.kunci] ? <p className="mt-1 text-xs font-medium text-red-600">{galatKolom[k.kunci]}</p> : <p className="mt-1 text-xs text-slate-500">{k.bantuan}</p>}
            {(k.kunci === "limit_jendela_detik" || k.kunci === "kuota_periode_detik") && /^\d+$/.test(teks[k.kunci]) && Number(teks[k.kunci]) > 0 && (
              <p className="mt-0.5 text-xs text-slate-400">= {teksPeriode(Number(teks[k.kunci]))}</p>
            )}
          </div>
        ))}
      </div>

      <label className="flex cursor-pointer items-start gap-2.5 rounded-xl border border-slate-200 bg-white p-3.5 text-sm text-slate-700">
        <input type="checkbox" checked={dibuka} onChange={(e) => setDibuka(e.target.checked)} className="mt-0.5 h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" />
        <span>
          <span className="block font-medium text-slate-900">Pendaftaran aplikasi baru dibuka</span>
          <span className="block text-xs text-slate-500">Bila dimatikan, formulir publik menolak pendaftaran baru; klien yang sudah ada tidak terpengaruh.</span>
        </span>
      </label>

      <div className="flex flex-wrap justify-end gap-2">
        <Button fullWidth={false} variant="secondary" icon={<RotateCcw className="h-4 w-4" aria-hidden="true" />} onClick={pabrikkan} disabled={sibuk}>
          Kembalikan ke nilai pabrik
        </Button>
        <Button fullWidth={false} isLoading={sibuk} disabled={!berubah} icon={<Save className="h-4 w-4" aria-hidden="true" />} onClick={() => void simpan()}>
          Simpan pengaturan
        </Button>
      </div>
    </div>
  );
}
