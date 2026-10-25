// Fungsi murni untuk menampilkan sumber data sinkronisasi Digitalisasi Aset (SLDK, dan datamart sebagai cadangan). Tanpa React dan tanpa alias "@/", jadi bisa dites dengan Node.
import type { DGSumberInfo } from "./api";

// Datamart yang terakhir disegarkan lebih dari ini dianggap usang: bila SLDK gagal, data yang dipakai sudah lama.
export const BATAS_USANG_HARI = 14;

export type NadaPemberitahuan = "warning" | "info";

export interface PemberitahuanSumber {
  kunci: "tidak_tersedia" | "mode_datamart" | "hanya_datamart" | "cadangan" | "datamart_bermasalah" | "datamart_usang";
  nada: NadaPemberitahuan;
  teks: string;
}

export function labelSumber(s: string | null | undefined): string {
  return s === "datamart" ? "Datamart" : "SLDK";
}

const HARI = 24 * 60 * 60 * 1000;

// Umur dalam hari penuh sejak iso; null bila tidak diketahui atau tidak terbaca.
export function umurHari(iso: string | null | undefined, sekarang: number): number | null {
  if (!iso) return null;
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return null;
  return Math.max(0, Math.floor((sekarang - t) / HARI));
}

// Bisa-tidaknya sinkronisasi dimulai dari sisi sumber data. Backend lama (tanpa sumber) hanya mengenal sldk_tersedia.
export function bisaMulai(sumber: DGSumberInfo | undefined, sldkTersedia: boolean): boolean {
  return sumber ? sumber.tersedia : sldkTersedia;
}

// Pemberitahuan di halaman Sinkronisasi tentang dari mana data dibaca. fmt memformat waktu (ISO) untuk teks.
export function pemberitahuanSumber(s: DGSumberInfo | undefined, sekarang: number, fmt: (iso: string) => string): PemberitahuanSumber[] {
  if (!s) return [];
  const per = s.datamart_disegarkan ? ` (data per ${fmt(s.datamart_disegarkan)})` : "";
  const out: PemberitahuanSumber[] = [];

  if (!s.tersedia) {
    let teks = "Tidak ada sumber data untuk sinkronisasi: SLDK_DB_* (dan DATAMART_DB_* sebagai cadangan) belum diisi di server. Periksa juga izin akses IP server di SLDK.";
    if (s.mode === "datamart" && !s.datamart) teks = 'Mode sumber "datamart" dipilih, tetapi DATAMART_DB_* belum diisi di server, jadi sinkronisasi tidak bisa dijalankan.';
    if (s.mode === "sldk" && !s.sldk) teks = 'Mode sumber "sldk" dipilih, tetapi SLDK_DB_* belum diisi di server, jadi sinkronisasi tidak bisa dijalankan.';
    return [{ kunci: "tidak_tersedia", nada: "warning", teks }];
  }

  const pakaiDatamart = s.mode !== "sldk" && s.datamart;
  if (s.mode === "datamart") {
    out.push({ kunci: "mode_datamart", nada: "info", teks: `Sinkronisasi membaca datamart (mode sumber: datamart), bukan SLDK${per}.` });
  } else if (!s.sldk && s.datamart) {
    out.push({ kunci: "hanya_datamart", nada: "info", teks: `SLDK_DB_* belum diisi, jadi sinkronisasi membaca datamart${per}.` });
  } else if (s.cadangan) {
    out.push({
      kunci: "cadangan",
      nada: "info",
      teks: `Cadangan aktif: bila SLDK tidak dapat dibaca, sinkronisasi beralih ke datamart${per}. Datamart hanya dipakai bila lebih baru daripada data yang sudah ada di PASTI.`,
    });
  }

  if (pakaiDatamart) {
    if (!s.datamart_terjangkau) {
      out.push({
        kunci: "datamart_bermasalah",
        nada: "warning",
        teks: "Datamart dikonfigurasi, tetapi tidak dapat dihubungi atau belum pernah berhasil disegarkan; cadangan tidak akan bekerja sampai ini diperbaiki.",
      });
    } else {
      const umur = umurHari(s.datamart_disegarkan, sekarang);
      if (umur !== null && umur > BATAS_USANG_HARI) {
        out.push({
          kunci: "datamart_usang",
          nada: "warning",
          teks: `Datamart terakhir disegarkan ${umur} hari lalu. Jadwalkan penyegaran rutin (EXEC dbo.datamart_sldk_segarkan) agar cadangan tetap berguna.`,
        });
      }
    }
  }
  return out;
}

// Judul dan sumber yang disebut pada dialog konfirmasi "Mulai sinkronisasi".
export function judulKonfirmasi(s: DGSumberInfo | undefined): string {
  if (s && (s.mode === "datamart" || (!s.sldk && s.datamart))) return "Mulai sinkronisasi dari datamart?";
  return "Mulai sinkronisasi dari SLDK?";
}
