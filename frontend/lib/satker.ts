// Fungsi murni untuk tabel keterhubungan satker (aset dan pengadaan) di Dashboard: penyaringan, pengurutan, dan angka turunannya. Tanpa React
// supaya bisa diuji dengan Node (lib/satker.test.mjs).
import type { SatkerRingkasan, SatkerStatus, SatkerTerhubung } from "./api";

export type UrutSatker = "aset" | "pengadaan" | "nilai" | "kode" | "nama";

export interface SaringanSatker {
  q: string;
  status: SatkerStatus | "";
  ue1: string; // kode UE1; "(kosong)" = satker tanpa UE1; "" = semua
}

export const URUT_LABEL: Record<UrutSatker, string> = {
  aset: "Jumlah aset (terbanyak)",
  pengadaan: "Jumlah paket pengadaan (terbanyak)",
  nilai: "Pagu RUP (terbesar)",
  kode: "Kode satker",
  nama: "Nama satker",
};

// Jumlah paket pengadaan satker: RUP + tender + non-tender (satu paket yang ada di ketiganya dihitung tiga kali; ini ukuran keaktifan, bukan total paket).
export function jumlahPengadaan(s: SatkerTerhubung): number {
  return s.pengadaan.rup_paket + s.pengadaan.tender + s.pengadaan.non_tender;
}

const lipat = (x: string) => x.toLocaleLowerCase("id-ID");

// Setiap kata kunci harus ada pada kode, nama (aset maupun Inaproc), atau label UE1; huruf besar/kecil diabaikan.
export function saringSatker(daftar: SatkerTerhubung[], s: SaringanSatker): SatkerTerhubung[] {
  const kata = lipat(s.q.trim()).split(/\s+/).filter(Boolean);
  return daftar.filter((x) => {
    if (s.status && x.status !== s.status) return false;
    if (s.ue1) {
      const kode = x.kode_ue1 || "(kosong)";
      if (kode !== s.ue1) return false;
    }
    if (kata.length === 0) return true;
    const jerami = lipat(`${x.kode} ${x.nama} ${x.nama_pengadaan ?? ""} ${x.ue1}`);
    return kata.every((k) => jerami.includes(k));
  });
}

// Mengembalikan salinan terurut; urutan semula dipertahankan untuk nilai yang sama (stabil), lalu kode sebagai pemutus seri.
export function urutkanSatker(daftar: SatkerTerhubung[], urut: UrutSatker): SatkerTerhubung[] {
  const nilai = (x: SatkerTerhubung): number => {
    switch (urut) {
      case "aset":
        return x.jumlah_aset;
      case "pengadaan":
        return jumlahPengadaan(x);
      case "nilai":
        return x.pengadaan.rup_pagu;
      default:
        return 0;
    }
  };
  return daftar
    .map((x, i) => ({ x, i }))
    .sort((a, b) => {
      if (urut === "kode") return a.x.kode.localeCompare(b.x.kode) || a.i - b.i;
      if (urut === "nama") return (a.x.nama || "￿").localeCompare(b.x.nama || "￿", "id") || a.x.kode.localeCompare(b.x.kode) || a.i - b.i;
      return nilai(b.x) - nilai(a.x) || a.x.kode.localeCompare(b.x.kode) || a.i - b.i;
    })
    .map((e) => e.x);
}

// Pilihan UE1 untuk penyaring: kode yang benar-benar ada di daftar, terurut menurut kode ("(kosong)" terakhir).
export function kodeUE1Daftar(daftar: SatkerTerhubung[]): string[] {
  const set = new Set<string>();
  for (const x of daftar) set.add(x.kode_ue1 || "(kosong)");
  return [...set].sort((a, b) => (a === "(kosong)" ? 1 : b === "(kosong)" ? -1 : a.localeCompare(b)));
}

// Berapa satker tiap status pada daftar (untuk angka di tombol penyaring status).
export function hitungPerStatus(daftar: SatkerTerhubung[]): Record<SatkerStatus | "semua", number> {
  const out = { semua: daftar.length, terhubung: 0, hanya_aset: 0, hanya_pengadaan: 0 };
  for (const x of daftar) out[x.status] += 1;
  return out;
}

// Persen satker pada data aset yang punya pengadaan di tahun itu (0 bila tidak ada satker aset).
export function persenTerhubung(r: SatkerRingkasan): number {
  return r.satker_aset > 0 ? (r.terhubung / r.satker_aset) * 100 : 0;
}

// Petunjuk bagi pengguna bila angka keterhubungan mencurigakan: kode yang tidak cocok bisa berarti formatnya berbeda, bukan satkernya tidak ada.
export interface Petunjuk {
  tingkat: "info" | "perhatian";
  isi: string;
}

export function petunjukKeterhubungan(r: SatkerRingkasan, tahun: string): Petunjuk[] {
  const p: Petunjuk[] = [];
  if (r.satker_aset === 0 && r.satker_pengadaan === 0) {
    p.push({ tingkat: "info", isi: "Belum ada data aset maupun data pengadaan untuk dihubungkan. Jalankan sinkronisasi Digitalisasi Aset dan penarikan data Pengadaan Terpadu." });
    return p;
  }
  if (r.satker_aset === 0) p.push({ tingkat: "perhatian", isi: "Data aset belum ada, jadi semua satker pengadaan tampil sebagai \"hanya pengadaan\". Jalankan sinkronisasi Digitalisasi Aset." });
  if (r.satker_pengadaan === 0) p.push({ tingkat: "perhatian", isi: `Belum ada data pengadaan untuk tahun ${tahun}, jadi semua satker tampil sebagai "hanya aset". Pilih tahun lain atau tarik data Pengadaan.` });
  if (r.satker_aset > 0 && r.satker_pengadaan > 0) {
    const tak = r.hanya_pengadaan / r.satker_pengadaan;
    if (r.terhubung === 0) {
      p.push({ tingkat: "perhatian", isi: "Tidak ada satker yang cocok antara data aset dan pengadaan. Periksa format kode: kode 6 digit pada data aset (karakter ke-10 sampai ke-15 kode satker) harus sama dengan kd_satker_str pada Inaproc." });
    } else if (tak > 0.3) {
      p.push({ tingkat: "info", isi: `${Math.round(tak * 100)}% satker pengadaan tidak ditemukan pada data aset. Bisa karena satker itu belum punya data BMN di SLDK, atau kodenya berbeda; periksa contohnya pada daftar "Hanya pengadaan".` });
    }
  }
  if (r.aset_tanpa_kode > 0) p.push({ tingkat: "info", isi: `${r.aset_tanpa_kode.toLocaleString("id-ID")} baris data aset tidak punya kode satker yang dapat dibaca, sehingga tidak ikut dihubungkan.` });
  return p;
}
