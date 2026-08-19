"use client";

import { X, Loader2, MapPin, Phone, Calendar, Briefcase, BadgeCheck, Building2 } from "lucide-react";
import { useEffect, useState } from "react";
import axios from "axios";
import { searchPegawaiByNIP } from "@/lib/api";

interface PegawaiDetailModalProps {
  nip: string;
  onClose: () => void;
}

interface JabatanEntry {
  namaJabatan?: string;
  statusJabatan?: string;
  esl1?: string;
  esl2?: string;
  esl3?: string;
  esl4?: string;
  organisasi?: string;
  tanggalMulai?: string;
  jenisJabatan?: string;
}

function formatDate(iso?: string): string {
  if (!iso) return "-";
  try {
    return new Date(iso).toLocaleDateString("id-ID", { day: "numeric", month: "long", year: "numeric" });
  } catch {
    return iso;
  }
}

export function PegawaiDetailModal({ nip, onClose }: PegawaiDetailModalProps) {
  const [profile, setProfile] = useState<Record<string, unknown> | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);
    setError(null);

    searchPegawaiByNIP(nip)
      .then((res) => {
        if (!cancelled) setProfile(res.data);
      })
      .catch((err) => {
        if (cancelled) return;
        if (axios.isAxiosError(err) && err.response) {
          setError(err.response.data?.message || "Gagal mengambil detail pegawai");
        } else {
          setError("Gagal terhubung ke server");
        }
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [nip]);

  const jabatanList = (profile?.jabatan as JabatanEntry[] | undefined) || [];
  const pangkat = profile?.pangkat as Record<string, unknown> | undefined;
  const status = profile?.status as Record<string, unknown> | undefined;
  const gravatar = profile?.gravatar as string | undefined;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-xl bg-white shadow-xl">
        <div className="flex items-center justify-between border-b border-slate-200 px-6 py-4">
          <h2 className="text-base font-semibold text-slate-900">Detail Profil Pegawai</h2>
          <button onClick={onClose} className="rounded-md p-1 text-slate-400 hover:bg-slate-100">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="p-6">
          {isLoading && (
            <div className="flex justify-center py-16">
              <Loader2 className="h-6 w-6 animate-spin text-blue-600" />
            </div>
          )}

          {error && (
            <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>
          )}

          {!isLoading && !error && profile && (
            <div className="space-y-6">
              {/* Header profil */}
              <div className="flex items-center gap-4">
                <div className="flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-full bg-blue-100">
                  {gravatar ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={gravatar} alt="Foto profil" className="h-full w-full object-cover" />
                  ) : (
                    <span className="text-lg font-bold text-blue-600">
                      {String(profile.nama || "?").charAt(0)}
                    </span>
                  )}
                </div>
                <div>
                  <p className="text-lg font-bold text-slate-900">
                    {String(profile.nama || "-")}
                    {profile.gelarBelakang ? `, ${profile.gelarBelakang}` : ""}
                  </p>
                  <p className="text-sm text-slate-500">NIP: {String(profile.nip18 || nip)}</p>
                  {status && (
                    <span className="mt-1 inline-block rounded-full bg-green-50 px-2.5 py-0.5 text-xs font-medium text-green-700">
                      {String(status.uraian || "-")}
                    </span>
                  )}
                </div>
              </div>

              {/* Info dasar */}
              <div className="grid grid-cols-1 gap-3 rounded-lg border border-slate-100 bg-slate-50 p-4 sm:grid-cols-2">
                <DetailRow icon={MapPin} label="Tempat, Tanggal Lahir" value={`${profile.tempatLahir || "-"}, ${formatDate(profile.tanggalLahir as string)}`} />
                <DetailRow icon={Phone} label="No. HP" value={String(profile.noHp || "-")} />
                <DetailRow icon={BadgeCheck} label="Jenis Kelamin" value={String(profile.jenisKelamin || "-")} />
                <DetailRow icon={Building2} label="Satuan Kerja" value={String(profile.namaSatker || "-")} />
                <DetailRow icon={Building2} label="Kode Satker" value={String(profile.kdSatker || "-")} />
                <DetailRow icon={BadgeCheck} label="Email" value={String(profile.email || "-")} />
              </div>

              {/* Pangkat */}
              {pangkat && (
                <div>
                  <p className="mb-2 text-sm font-semibold text-slate-700">Pangkat / Golongan</p>
                  <div className="rounded-lg border border-slate-200 p-3 text-sm">
                    <p className="font-medium text-slate-900">
                      {String(pangkat.namaPangkat || "-")} ({String(pangkat.kodeGolongan || "-")})
                    </p>
                    <p className="text-xs text-slate-500">
                      TMT: {formatDate(pangkat.tanggalMulai as string)}
                    </p>
                  </div>
                </div>
              )}

              {/* Jabatan */}
              {jabatanList.length > 0 && (
                <div>
                  <p className="mb-2 flex items-center gap-1.5 text-sm font-semibold text-slate-700">
                    <Briefcase className="h-4 w-4" />
                    Riwayat / Jabatan Saat Ini
                  </p>
                  <div className="space-y-3">
                    {jabatanList.map((j, idx) => (
                      <div key={idx} className="rounded-lg border border-slate-200 p-3 text-sm">
                        <div className="flex items-center justify-between">
                          <p className="font-medium text-slate-900">{j.namaJabatan || "-"}</p>
                          <span className="rounded-full bg-blue-50 px-2 py-0.5 text-[11px] font-medium text-blue-700">
                            {j.statusJabatan || "-"}
                          </span>
                        </div>
                        <p className="mt-1 text-xs text-slate-500">{j.organisasi || "-"}</p>
                        <p className="mt-1 flex items-center gap-1 text-xs text-slate-400">
                          <Calendar className="h-3 w-3" />
                          TMT: {formatDate(j.tanggalMulai)}
                        </p>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function DetailRow({ icon: Icon, label, value }: { icon: React.ElementType; label: string; value: string }) {
  return (
    <div className="flex items-start gap-2">
      <Icon className="mt-0.5 h-4 w-4 shrink-0 text-slate-400" />
      <div>
        <p className="text-xs text-slate-500">{label}</p>
        <p className="text-sm font-medium text-slate-800">{value}</p>
      </div>
    </div>
  );
}