// Fungsi murni untuk menampilkan sumber data sinkronisasi Digitalisasi Aset: datamart (tabel DIGITALISASI_* yang dibentuk dari SLDK oleh script di sisi database; aplikasi tidak
// pernah membaca SLDK untuk dataset ini). Tanpa React dan tanpa alias "@/", jadi bisa dites dengan Node.
import type { DGSumberInfo } from "./api";

// Datamart yang terakhir disegarkan lebih dari ini dianggap usang: sinkronisasi hanya menyalin data lama.
export const BATAS_USANG_HARI = 14;

// Judul dialog konfirmasi "Mulai sinkronisasi".
export const JUDUL_KONFIRMASI = "Mulai sinkronisasi dari datamart?";

export type NadaPemberitahuan = "warning" | "info";

export interface PemberitahuanSumber {
  kunci: "tidak_tersedia" | "datamart" | "tidak_terjangkau" | "belum_disegarkan" | "penyegaran_gagal" | "datamart_usang";
  nada: NadaPemberitahuan;
  teks: string;
}

// Label untuk lencana riwayat: hanya riwayat LAMA (dari masa aplikasi masih membaca SLDK langsung) yang diberi tanda; sinkronisasi sekarang selalu dari datamart.
export function labelSumberLama(s: string | null | undefined): string | null {
  return s === "sldk" ? "SLDK (lama)" : null;
}

const HARI = 24 * 60 * 60 * 1000;

// Umur dalam hari penuh sejak iso; null bila tidak diketahui atau tidak terbaca.
export function umurHari(iso: string | null | undefined, sekarang: number): number | null {
  if (!iso) return null;
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return null;
  return Math.max(0, Math.floor((sekarang - t) / HARI));
}

// Bisa-tidaknya sinkronisasi dimulai dari sisi sumber data: datamart harus dikonfigurasi di server.
export function bisaMulai(sumber: DGSumberInfo | undefined): boolean {
  return !!sumber && sumber.tersedia;
}

// Pemberitahuan di halaman Sinkronisasi tentang dari mana data dibaca dan seberapa segar. fmt memformat waktu (ISO) untuk teks.
export function pemberitahuanSumber(s: DGSumberInfo | undefined, sekarang: number, fmt: (iso: string) => string): PemberitahuanSumber[] {
  if (!s) return [];
  if (!s.tersedia) {
    return [
      {
        kunci: "tidak_tersedia",
        nada: "warning",
        teks: "Datamart belum dikonfigurasi di server (DATAMART_DB_*), jadi sinkronisasi Digitalisasi Aset tidak bisa dijalankan. Aplikasi membaca data hanya dari datamart, tidak langsung dari SLDK.",
      },
    ];
  }

  const out: PemberitahuanSumber[] = [];
  const per = s.data_per ? ` (data per ${fmt(s.data_per)})` : "";
  out.push({
    kunci: "datamart",
    nada: "info",
    teks: `Sinkronisasi menyalin tabel DIGITALISASI_* dari datamart${per}, bukan langsung dari SLDK. Datamart disegarkan terjadwal oleh administrator database.`,
  });

  if (!s.terjangkau) {
    out.push({
      kunci: "tidak_terjangkau",
      nada: "warning",
      teks: "Datamart dikonfigurasi, tetapi tidak dapat dihubungi atau jejak penyegarannya tidak terbaca; sinkronisasi akan gagal sampai ini diperbaiki.",
    });
    return out;
  }
  if (!s.data_per) {
    out.push({
      kunci: "belum_disegarkan",
      nada: "warning",
      teks: "Datamart belum pernah berhasil disegarkan (belum ada penyegaran sukses di dbo.datamart_digitalisasi_run), jadi belum ada data untuk disalin.",
    });
    return out;
  }
  if (s.penyegaran_terakhir === "gagal") {
    out.push({
      kunci: "penyegaran_gagal",
      nada: "warning",
      teks: "Penyegaran datamart yang terakhir gagal. Data yang disalin berasal dari penyegaran sukses sebelumnya. Periksa dbo.datamart_digitalisasi_run di datamart.",
    });
  }
  const umur = umurHari(s.data_per, sekarang);
  if (umur !== null && umur > BATAS_USANG_HARI) {
    out.push({
      kunci: "datamart_usang",
      nada: "warning",
      teks: `Datamart terakhir disegarkan ${umur} hari lalu. Jadwalkan penyegaran rutin (EXEC dbo.datamart_digitalisasi_segarkan) agar data yang disalin tetap baru.`,
    });
  }
  return out;
}
