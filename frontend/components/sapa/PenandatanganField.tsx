"use client";

import { SapaPenandatangan } from "@/lib/sapa";
import { TextField } from "./fields";
import { PegawaiPicker } from "./PegawaiPicker";

const TANPA_TERPAKAI: Set<string> = new Set();

// Pejabat penandatangan: cari di HRIS2 lalu nama (dengan gelar), NIP, dan jabatan terisi otomatis. Semua bidang tetap bisa
// diubah atau diketik manual (mis. pejabat pelaksana tugas, atau sesi HRIS2 tidak tersedia).
// denganNIP=false untuk surat yang tidak memuat NIP (Nota Dinas UE1): NIP dari HRIS2 tidak disimpan.
export function PenandatanganField({
  value,
  onChange,
  denganNIP,
  labelNama = "Nama",
  labelJabatan = "Jabatan",
}: {
  value: SapaPenandatangan;
  onChange: (p: SapaPenandatangan) => void;
  denganNIP: boolean;
  labelNama?: string;
  labelJabatan?: string;
}) {
  return (
    <div className="space-y-3">
      <PegawaiPicker
        label="Cari pejabat di HRIS2"
        kataSukses="dipilih sebagai penandatangan"
        terpakai={TANPA_TERPAKAI}
        onPilih={(p) => onChange({ nama: p.nama, nip: denganNIP ? p.nip : value.nip, jabatan: p.jabatan || value.jabatan })}
      />
      <div className={`grid gap-3 ${denganNIP ? "sm:grid-cols-3" : "sm:grid-cols-2"}`}>
        <TextField label={labelNama} required value={value.nama} onChange={(x) => onChange({ ...value, nama: x })} maxLength={150} />
        {denganNIP && (
          <TextField
            label="NIP (18 digit)"
            required
            value={value.nip}
            onChange={(x) => onChange({ ...value, nip: x.replace(/[^\d\s]/g, "") })}
            inputMode="numeric"
            maxLength={24}
          />
        )}
        <TextField label={labelJabatan} required value={value.jabatan} onChange={(x) => onChange({ ...value, jabatan: x })} maxLength={300} />
      </div>
    </div>
  );
}
