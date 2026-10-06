// Wawasan analitik untuk Dashboard Aset: penjelasan berbasis angka dari ringkasan Digitalisasi Aset (hasil sinkronisasi SLDK). Fungsi murni (tanpa
// React), supaya bisa dites langsung dengan Node. Setiap wawasan menyebut angkanya sendiri; ambang ada di blok konstanta di bawah.

import type { DGCount, DGDatasetKey, DGOverviewData } from "./api";
import { ASSET_KEYS, DATASET_LABEL, NILAI_KEYS, asuransiLabel, mergeCounts, perJumlah, perNilai } from "../components/digitalisasi/digitalisasi.ts";
import { compactRupiah, formatNumber, kondisiTone, pct, share } from "./dasbor.ts";

export type TingkatWawasanAset = "penting" | "perhatian" | "info" | "baik";

// Bentuknya sama dengan AnWawasan (wawasan Pengadaan) supaya satu komponen daftar dipakai untuk keduanya.
export interface WawasanAset {
  bagian: "ikhtisar" | "sebaran" | "kondisi" | "kelengkapan" | "data";
  tingkat: TingkatWawasanAset;
  judul: string;
  isi: string;
}

// ---- ambang ----
const UE1_TERPUSAT = 0.6; // satu UE1 memegang sekian bagian nilai aset: dianggap terpusat (perhatian)
const BERAT_PENTING = 0.1; // bagian aset rusak berat dari yang kondisinya terisi
const BERAT_PERHATIAN = 0.03;
const KOORDINAT_PENTING = 0.25; // bagian aset tanpa koordinat valid
const KOORDINAT_PERHATIAN = 0.05;
const FOTO_PERHATIAN = 0.3; // bagian aset tanpa foto
const FOTO_INFO = 0.1;
const ASURANSI_BAIK = 0.8; // bagian gedung yang diasuransikan
const ASURANSI_RENDAH = 0.4;
const KELENGKAPAN_SATKER = 0.3; // bagian satker induk tanpa data gedung kantor utama/tanah
const STATUS_HUKUM_KOSONG = 0.2;
const DATA_LAMA_HARI = 14; // sinkronisasi tertua yang masih dianggap segar
const HARI_MS = 24 * 60 * 60 * 1000;

const URUTAN: Record<TingkatWawasanAset, number> = { penting: 0, perhatian: 1, info: 2, baik: 3 };

const nf = (n: number) => formatNumber(n, 0);

function sum(keys: DGDatasetKey[], d: DGOverviewData, f: (a: DGOverviewData["aset"][number]) => number): number {
  return keys.reduce((acc, k) => {
    const a = d.aset.find((x) => x.key === k);
    return acc + (a ? f(a) : 0);
  }, 0);
}

// Dataset dengan nilai terbesar menurut `f` (undefined bila semuanya nol).
function terbanyak(d: DGOverviewData, f: (a: DGOverviewData["aset"][number]) => number): { label: string; n: number } | undefined {
  let best: { label: string; n: number } | undefined;
  for (const a of d.aset) {
    const n = f(a);
    if (n > 0 && (!best || n > best.n)) best = { label: DATASET_LABEL[a.key], n };
  }
  return best;
}

export function wawasanAset(d: DGOverviewData, sekarang: number = Date.now()): WawasanAset[] {
  const out: WawasanAset[] = [];
  const totalAset = d.aset.reduce((a, x) => a + x.jumlah, 0);

  // ---- Ikhtisar ----
  if (totalAset > 0 || d.satker.total > 0) {
    const tanah = sum(["tanah"], d, (a) => a.jumlah);
    const gedung = sum(["gedung_kantor_utama", "gedung_lainnya"], d, (a) => a.jumlah);
    const hunian = sum(["rusunara", "rumah_negara", "mess_rumah_negara"], d, (a) => a.jumlah);
    const nilai = sum(NILAI_KEYS, d, (a) => a.nilai);
    out.push({
      bagian: "ikhtisar",
      tingkat: "info",
      judul: "Skala aset yang dikelola",
      isi:
        `${nf(d.satker.induk)} satker induk (${nf(d.satker.anak)} anak satker) tercatat mengelola ${nf(totalAset)} aset: ${nf(tanah)} tanah, ${nf(gedung)} gedung, ` +
        `dan ${nf(hunian)} hunian (rusunara, rumah negara, mess). Nilai tercatat ${compactRupiah(nilai)} untuk tanah, gedung, rusunara, dan mess; ` +
        `rumah negara belum punya kolom nilai.`,
    });
  }

  // ---- Sebaran per UE1 (nilai) ----
  const ue1 = d.ue1.map((u) => ({ label: u.label, v: perNilai(u.per) })).filter((u) => u.v > 0).sort((a, b) => b.v - a.v);
  const nilaiUE1 = ue1.reduce((a, u) => a + u.v, 0);
  if (ue1.length >= 2 && nilaiUE1 > 0) {
    const s1 = share(ue1[0].v, nilaiUE1);
    out.push({
      bagian: "sebaran",
      tingkat: s1 >= UE1_TERPUSAT ? "perhatian" : "info",
      judul: s1 >= UE1_TERPUSAT ? "Nilai aset terpusat pada satu unit eselon I" : "Sebaran nilai aset per unit eselon I",
      isi:
        `${ue1[0].label} memegang ${pct(s1)} nilai aset (${compactRupiah(ue1[0].v)}), disusul ${ue1[1].label} (${pct(share(ue1[1].v, nilaiUE1))}). ` +
        `${nf(ue1.length)} unit eselon I punya aset bernilai.` +
        (s1 >= UE1_TERPUSAT ? " Perubahan kondisi atau status aset di unit ini sangat memengaruhi total." : ""),
    });
  }

  // ---- Sebaran per provinsi (jumlah) ----
  const prov = d.provinsi.map((p) => ({ nama: p.nama, v: perJumlah(p.per) })).filter((p) => p.v > 0).sort((a, b) => b.v - a.v);
  const totalProv = prov.reduce((a, p) => a + p.v, 0);
  if (prov.length >= 2 && totalProv > 0) {
    const top3 = prov.slice(0, 3).reduce((a, p) => a + p.v, 0);
    out.push({
      bagian: "sebaran",
      tingkat: "info",
      judul: "Sebaran aset per provinsi",
      isi: `Aset tersebar di ${nf(prov.length)} provinsi. ${prov[0].nama} terbanyak (${nf(prov[0].v)} aset, ${pct(share(prov[0].v, totalProv))}); tiga provinsi teratas memuat ${pct(share(top3, totalProv))} dari seluruh aset.`,
    });
  }

  // ---- Kondisi ----
  let berat = 0;
  let ringan = 0;
  let terisi = 0;
  let nilaiBerat = 0;
  const beratPer: { label: string; n: number }[] = [];
  for (const k of ASSET_KEYS) {
    let beratK = 0;
    for (const c of (d.kondisi[k] ?? []) as DGCount[]) {
      if (c.k === null || c.k.trim() === "") continue;
      terisi += c.jumlah;
      const tone = kondisiTone(c.k);
      if (tone === "berat") {
        berat += c.jumlah;
        beratK += c.jumlah;
        nilaiBerat += c.nilai;
      } else if (tone === "ringan") {
        ringan += c.jumlah;
      }
    }
    if (beratK > 0) beratPer.push({ label: DATASET_LABEL[k], n: beratK });
  }
  if (terisi > 0) {
    const sBerat = share(berat, terisi);
    const sRingan = share(ringan, terisi);
    const tingkat: TingkatWawasanAset = sBerat >= BERAT_PENTING ? "penting" : sBerat >= BERAT_PERHATIAN ? "perhatian" : "baik";
    const teratas = [...beratPer].sort((a, b) => b.n - a.n)[0];
    out.push({
      bagian: "kondisi",
      tingkat,
      judul: tingkat === "baik" ? "Kondisi aset umumnya baik" : "Aset rusak berat perlu ditindaklanjuti",
      isi:
        (berat > 0
          ? `${nf(berat)} aset (${pct(sBerat)}) berkondisi rusak berat${nilaiBerat > 0 ? `, bernilai tercatat ${compactRupiah(nilaiBerat)}` : ""}, dan ${nf(ringan)} (${pct(sRingan)}) rusak ringan`
          : `Tidak ada aset berkondisi rusak berat; ${nf(ringan)} (${pct(sRingan)}) rusak ringan`) +
        `, dari ${nf(terisi)} aset yang kondisinya terisi.` +
        (teratas ? ` Rusak berat terbanyak pada ${teratas.label} (${nf(teratas.n)}).` : ""),
    });
  }

  // ---- Kelengkapan: koordinat ----
  const geo = d.aset.filter((a) => a.geo);
  const geoTotal = geo.reduce((a, x) => a + x.jumlah, 0);
  if (geoTotal > 0) {
    const bertitik = geo.reduce((a, x) => a + x.bertitik, 0);
    const tanpa = geo.reduce((a, x) => a + x.tanpa_koordinat, 0);
    const luar = geo.reduce((a, x) => a + x.di_luar_indonesia, 0);
    const sBelum = share(tanpa + luar, geoTotal);
    const tingkat: TingkatWawasanAset = sBelum >= KOORDINAT_PENTING ? "penting" : sBelum >= KOORDINAT_PERHATIAN ? "perhatian" : "baik";
    const teratas = terbanyak({ ...d, aset: geo }, (a) => a.tanpa_koordinat);
    out.push({
      bagian: "kelengkapan",
      tingkat,
      judul: tingkat === "baik" ? "Koordinat aset lengkap" : "Koordinat aset belum lengkap",
      isi:
        `${nf(bertitik)} dari ${nf(geoTotal)} aset (${pct(share(bertitik, geoTotal))}) punya koordinat valid di wilayah Indonesia dan tampil di peta. ` +
        `${nf(tanpa)} belum punya koordinat${luar > 0 ? ` dan ${nf(luar)} berkoordinat di luar Indonesia (kemungkinan salah isi)` : ""}.` +
        (teratas ? ` Belum berkoordinat terbanyak pada ${teratas.label} (${nf(teratas.n)}).` : ""),
    });
  }

  // ---- Kelengkapan: foto ----
  const tanpaFoto = d.aset.reduce((a, x) => a + x.tanpa_foto, 0);
  if (totalAset > 0) {
    const sFoto = share(tanpaFoto, totalAset);
    const teratas = terbanyak(d, (a) => a.tanpa_foto);
    out.push({
      bagian: "kelengkapan",
      tingkat: sFoto >= FOTO_PERHATIAN ? "perhatian" : sFoto >= FOTO_INFO ? "info" : "baik",
      judul: sFoto >= FOTO_INFO ? "Foto aset belum lengkap" : "Foto aset hampir lengkap",
      isi: `${nf(tanpaFoto)} aset (${pct(sFoto)}) belum punya foto.` + (teratas && tanpaFoto > 0 ? ` Terbanyak pada ${teratas.label} (${nf(teratas.n)}).` : ""),
    });
  }

  // ---- Kelengkapan: kondisi kosong ----
  const tanpaKondisi = d.aset.reduce((a, x) => a + x.tanpa_kondisi, 0);
  if (totalAset > 0 && share(tanpaKondisi, totalAset) >= BERAT_PERHATIAN) {
    const teratas = terbanyak(d, (a) => a.tanpa_kondisi);
    out.push({
      bagian: "kelengkapan",
      tingkat: "perhatian",
      judul: "Kondisi sebagian aset belum terisi",
      isi: `${nf(tanpaKondisi)} aset (${pct(share(tanpaKondisi, totalAset))}) belum punya data kondisi, sehingga tidak ikut dalam penilaian kondisi di atas.` + (teratas ? ` Terbanyak pada ${teratas.label} (${nf(teratas.n)}).` : ""),
    });
  }

  // ---- Asuransi gedung ----
  const asuransi = mergeCounts([d.asuransi.gedung_kantor_utama, d.asuransi.gedung_lainnya], asuransiLabel);
  const totalGedung = asuransi.reduce((a, x) => a + x.jumlah, 0);
  if (totalGedung > 0) {
    const nAsuransi = asuransi.find((x) => x.label === "Diasuransikan")?.jumlah ?? 0;
    const nBelum = asuransi.find((x) => x.label === "Belum diasuransikan")?.jumlah ?? 0;
    const sAsuransi = share(nAsuransi, totalGedung);
    out.push({
      bagian: "kelengkapan",
      tingkat: sAsuransi >= ASURANSI_BAIK ? "baik" : sAsuransi < ASURANSI_RENDAH ? "perhatian" : "info",
      judul: sAsuransi >= ASURANSI_BAIK ? "Sebagian besar gedung diasuransikan" : "Cakupan asuransi gedung",
      isi: `${nf(nAsuransi)} dari ${nf(totalGedung)} gedung (${pct(sAsuransi)}) tercatat diasuransikan; ${nf(nBelum)} (${pct(share(nBelum, totalGedung))}) belum, dan sisanya tanpa data.`,
    });
  }

  // ---- Kelengkapan satker induk ----
  const kel = d.kelengkapan;
  if (kel.satker_induk > 0) {
    const sKantor = share(kel.induk_tanpa_kantor_utama, kel.satker_induk);
    const sTanah = share(kel.induk_tanpa_tanah, kel.satker_induk);
    out.push({
      bagian: "kelengkapan",
      tingkat: Math.max(sKantor, sTanah) >= KELENGKAPAN_SATKER ? "perhatian" : "info",
      judul: "Kelengkapan data satker induk",
      isi:
        `${nf(kel.induk_tanpa_kantor_utama)} dari ${nf(kel.satker_induk)} satker induk (${pct(sKantor)}) belum punya data gedung kantor utama, dan ` +
        `${nf(kel.induk_tanpa_tanah)} (${pct(sTanah)}) belum punya data tanah di salinan SLDK. Bisa berarti datanya memang belum diinput atau belum terhubung ke satker.`,
    });
  }

  // ---- Status hukum tanah ----
  const hukumTanah = d.status_hukum.tanah ?? [];
  const totalHukum = hukumTanah.reduce((a, x) => a + x.jumlah, 0);
  if (totalHukum > 0) {
    const kosong = hukumTanah.filter((x) => x.k === null || x.k.trim() === "").reduce((a, x) => a + x.jumlah, 0);
    if (share(kosong, totalHukum) >= STATUS_HUKUM_KOSONG) {
      out.push({
        bagian: "kelengkapan",
        tingkat: "perhatian",
        judul: "Status hukum tanah belum lengkap",
        isi: `${nf(kosong)} dari ${nf(totalHukum)} tanah (${pct(share(kosong, totalHukum))}) belum punya status hukum, padahal status ini menentukan kepastian penguasaan tanah.`,
      });
    }
  }

  // ---- Hunian ----
  const rn = d.aset.find((a) => a.key === "rumah_negara")?.jumlah ?? 0;
  const penghuni = [...d.status_penghuni].sort((a, b) => b.jumlah - a.jumlah);
  if (rn > 0 && penghuni.length > 0) {
    const totalP = penghuni.reduce((a, x) => a + x.jumlah, 0);
    const kamar = Object.values(d.hunian).reduce((a, x) => a + (Number.isFinite(x) ? x : 0), 0);
    out.push({
      bagian: "sebaran",
      tingkat: "info",
      judul: "Hunian dan penghuni",
      isi:
        `Dari ${nf(rn)} rumah negara, status penghuni terbanyak "${penghuni[0].k ?? "(tidak ada data)"}" (${nf(penghuni[0].jumlah)}, ${pct(share(penghuni[0].jumlah, totalP))}). ` +
        `Rusunara dan mess memiliki ${nf(kamar)} kamar tidur.`,
    });
  }

  // ---- Kesegaran data ----
  const sinkron = d.sinkron;
  if (sinkron.length > 0) {
    const belum = sinkron.filter((s) => !s.terakhir_sukses);
    const sudah = sinkron.filter((s) => s.terakhir_sukses);
    if (sudah.length === 0) {
      out.push({ bagian: "data", tingkat: "penting", judul: "Data belum pernah disinkronkan", isi: "Seluruh dataset belum pernah disalin dari SLDK, jadi angka di dasbor ini belum mencerminkan keadaan sebenarnya." });
    } else {
      const tertua = sudah.reduce((a, s) => (new Date(s.terakhir_sukses!) < new Date(a.terakhir_sukses!) ? s : a));
      const hari = Math.max(0, Math.floor((sekarang - new Date(tertua.terakhir_sukses!).getTime()) / HARI_MS));
      const label = DATASET_LABEL[tertua.dataset];
      if (belum.length > 0) {
        out.push({
          bagian: "data",
          tingkat: "perhatian",
          judul: "Sebagian dataset belum disinkronkan",
          isi: `${nf(belum.length)} dari ${nf(sinkron.length)} dataset (${belum.map((s) => DATASET_LABEL[s.dataset]).join(", ")}) belum pernah disalin dari SLDK, sehingga dasbor belum lengkap.`,
        });
      } else if (hari >= DATA_LAMA_HARI) {
        out.push({
          bagian: "data",
          tingkat: "perhatian",
          judul: "Data sudah lama tidak disinkronkan",
          isi: `Dataset terlama (${label}) terakhir disalin ${nf(hari)} hari lalu. Sinkronkan ulang agar angka mengikuti SLDK.`,
        });
      } else {
        out.push({
          bagian: "data",
          tingkat: "baik",
          judul: "Data masih segar",
          isi: `Semua dataset disalin dari SLDK dalam ${nf(hari)} hari terakhir (terlama: ${label}).`,
        });
      }
    }
  }

  // Yang terpenting lebih dulu; urutan semula dipertahankan untuk tingkat yang sama.
  return out.map((w, i) => ({ w, i })).sort((a, b) => URUTAN[a.w.tingkat] - URUTAN[b.w.tingkat] || a.i - b.i).map((x) => x.w);
}
