"use client";

import { FormEvent, useEffect, useRef, useState } from "react";
import { LogOut, ShieldCheck } from "lucide-react";
import axios from "axios";
import { Pernyataan, getPernyataan, setujuiPernyataan } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { useDashboard } from "@/lib/dashboard-context";
import { GalatProfil, ProfilIsian, bolehKirimPersetujuan, rapikanProfil, validasiProfil } from "@/lib/persetujuan";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";

// Modal pernyataan penggunaan aplikasi. Tidak dapat ditutup (tanpa tombol Tutup, Escape, atau klik di luar): pengguna harus menyetujuinya atau keluar. Pengguna menyetujui
// dengan mengetik frasa yang diminta dan mencentang kotak; akun non-SSO sekaligus mengisi nama lengkap, NIP, dan email. Backend memeriksa ulang semuanya.
export function ModalPersetujuan() {
  const { logout } = useAuth();
  const { refetchProfile } = useDashboard();
  const [data, setData] = useState<Pernyataan | null>(null);
  const [memuat, setMemuat] = useState(true);
  const [galatMuat, setGalatMuat] = useState("");
  const [frasa, setFrasa] = useState("");
  const [dicentang, setDicentang] = useState(false);
  const [profil, setProfil] = useState<ProfilIsian>({ nama: "", nip: "", email: "" });
  const [sentuh, setSentuh] = useState(false); // semua galat isian tampil setelah pengguna mencoba mengirim
  const [disentuh, setDisentuh] = useState<Partial<Record<keyof ProfilIsian, boolean>>>({}); // galat satu isian tampil setelah pengguna meninggalkan isian itu
  const [galatServer, setGalatServer] = useState<GalatProfil>({});
  const [pesan, setPesan] = useState("");
  const [mengirim, setMengirim] = useState(false);
  const judulRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    let batal = false;
    getPernyataan()
      .then((res) => {
        if (batal) return;
        setData(res.data);
        setProfil({ nama: res.data.profil.nama, nip: res.data.profil.nip, email: res.data.profil.email });
      })
      .catch((err) => {
        if (!batal) setGalatMuat(axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : "Gagal memuat pernyataan. Coba muat ulang halaman.");
      })
      .finally(() => {
        if (!batal) setMemuat(false);
      });
    return () => {
      batal = true;
    };
  }, []);

  // Halaman di belakang tidak ikut menggulir, dan fokus pindah ke judul agar pembaca layar langsung membaca pernyataannya.
  useEffect(() => {
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    judulRef.current?.focus();
    return () => {
      document.body.style.overflow = overflow;
    };
  }, [data]);

  const isiProfil = data?.isi_profil ?? false;
  const galatLokal: GalatProfil = isiProfil ? validasiProfil(rapikanProfil(profil)) : {};
  const galat = (k: keyof ProfilIsian) => (sentuh || disentuh[k] ? (galatLokal[k] ?? galatServer[k]) : galatServer[k]);
  const tinggalkan = (k: keyof ProfilIsian) => () => setDisentuh((d) => ({ ...d, [k]: true }));
  const boleh = data !== null && bolehKirimPersetujuan({ frasa, dicentang, isiProfil, profil });

  const kirim = async (e: FormEvent) => {
    e.preventDefault();
    setSentuh(true);
    setPesan("");
    setGalatServer({});
    if (!boleh || !data) return;
    setMengirim(true);
    try {
      const p = rapikanProfil(profil);
      await setujuiPernyataan({ frasa, setuju: dicentang, ...(isiProfil ? { nama: p.nama, nip: p.nip, email: p.email } : {}) });
      refetchProfile();
    } catch (err) {
      if (axios.isAxiosError(err) && err.response) {
        setGalatServer((err.response.data?.galat as GalatProfil) ?? {});
        setPesan(err.response.data?.message || "Gagal menyimpan persetujuan");
      } else {
        setPesan("Gagal terhubung ke server");
      }
      setMengirim(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-slate-950/60 p-0 backdrop-blur-sm sm:items-center sm:p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="judul-persetujuan"
        className="max-h-[94dvh] w-full max-w-2xl animate-scale-in overflow-y-auto overscroll-contain rounded-t-3xl bg-white shadow-xl sm:rounded-2xl"
      >
        <div className="flex items-start gap-3 border-b border-slate-100 px-5 py-4 sm:px-6">
          <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600">
            <ShieldCheck className="h-5 w-5" aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <h2 id="judul-persetujuan" ref={judulRef} tabIndex={-1} className="text-base font-semibold tracking-tight text-slate-900 outline-none">
              {data?.judul ?? "Pernyataan Penggunaan Aplikasi PASTI V3"}
            </h2>
            <p className="mt-0.5 text-xs text-slate-500">Baca pernyataan berikut, lalu setujui untuk melanjutkan.</p>
          </div>
        </div>

        {memuat && <div role="status" aria-label="Memuat pernyataan" className="m-6 h-40 animate-pulse rounded-xl bg-slate-100" />}
        {galatMuat && (
          <div className="space-y-3 p-6">
            <Alert message={galatMuat} />
            <Button type="button" variant="secondary" fullWidth={false} onClick={() => logout("manual")} icon={<LogOut className="h-4 w-4" aria-hidden="true" />}>
              Keluar
            </Button>
          </div>
        )}

        {data && (
          <form onSubmit={kirim} noValidate className="space-y-5 px-5 py-5 sm:px-6">
            <ol className="list-decimal space-y-2.5 pl-5 text-sm leading-relaxed text-slate-700">
              {data.paragraf.map((p, i) => (
                <li key={i}>{p}</li>
              ))}
            </ol>

            {isiProfil && (
              <fieldset className="space-y-3 rounded-xl border border-slate-200 bg-slate-50/60 p-4">
                <legend className="px-1 text-sm font-semibold text-slate-800">Data diri (akun non-SSO)</legend>
                <p className="text-xs text-slate-500">Akun ini tidak masuk lewat SSO Kemenkeu, jadi identitas Anda diisi di sini. Email boleh email kedinasan atau pribadi.</p>
                <Input label="Nama lengkap" value={profil.nama} onChange={(e) => setProfil({ ...profil, nama: e.target.value })} onBlur={tinggalkan("nama")} error={galat("nama")} autoComplete="name" maxLength={120} />
                <Input label="NIP" value={profil.nip} onChange={(e) => setProfil({ ...profil, nip: e.target.value })} onBlur={tinggalkan("nip")} error={galat("nip")} inputMode="numeric" autoComplete="off" maxLength={30} />
                <Input
                  label="Email (kedinasan atau pribadi)"
                  type="email"
                  value={profil.email}
                  onChange={(e) => setProfil({ ...profil, email: e.target.value })}
                  onBlur={tinggalkan("email")}
                  error={galat("email")}
                  autoComplete="email"
                  maxLength={100}
                />
              </fieldset>
            )}

            <div className="space-y-3 rounded-xl border border-blue-100 bg-blue-50/50 p-4">
              <Input
                label={`Ketik "${data.frasa}" untuk menyetujui`}
                value={frasa}
                onChange={(e) => setFrasa(e.target.value)}
                autoComplete="off"
                autoCapitalize="characters"
                spellCheck={false}
                placeholder={data.frasa}
              />
              <label className="flex items-start gap-2.5 text-sm text-slate-700">
                <input type="checkbox" checked={dicentang} onChange={(e) => setDicentang(e.target.checked)} className="mt-0.5 h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" />
                <span>Saya telah membaca dan menyetujui pernyataan penggunaan aplikasi di atas.</span>
              </label>
            </div>

            {pesan && <Alert message={pesan} />}

            <div className="flex flex-col-reverse gap-2 border-t border-slate-100 pt-4 sm:flex-row sm:items-center sm:justify-between">
              <Button type="button" variant="ghost" fullWidth={false} onClick={() => logout("manual")} icon={<LogOut className="h-4 w-4" aria-hidden="true" />}>
                Keluar
              </Button>
              <Button type="submit" fullWidth={false} isLoading={mengirim} disabled={!boleh}>
                Setuju dan lanjutkan
              </Button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
