// Fungsi murni untuk tab Dokumentasi pada API Akses Data (tanpa React dan tanpa axios, jadi bisa diuji dengan Node): alamat dasar, pencarian dataset, penyaring yang
// didukung, dan contoh kode. Rujukan teknis (dataset, kolom, penyaring, kode galat) datang dari backend; tidak ada daftar dataset yang ditulis ulang di sini. Tipe ada di
// lib/apiAkses.ts.
import type { DatasetDok, ParameterDok } from "./apiAkses";

export type Bahasa = "curl" | "python" | "node";
export type Skenario = "satu" | "semua";

export const BAHASA: { key: Bahasa; label: string }[] = [
  { key: "curl", label: "curl" },
  { key: "python", label: "Python" },
  { key: "node", label: "Node.js" },
];

export const SKENARIO: { key: Skenario; label: string; uraian: string }[] = [
  { key: "satu", label: "Satu halaman", uraian: "Satu permintaan, lalu membaca baris dan penanda halaman berikutnya." },
  { key: "semua", label: "Tarik semua data", uraian: "Mengulang dengan kursor sampai ada_berikutnya bernilai false, dan menunggu Retry-After saat menerima 429." },
];

export const AWALAN_BAWAAN = "/api/data/v1";

// Alamat server PASTI tanpa garis miring akhir: NEXT_PUBLIC_API_ROOT_URL bila diatur, kalau tidak alamat halaman ini.
export function akarApi(akar: string | undefined, origin: string | undefined): string {
  return (akar || origin || "").trim().replace(/\/+$/, "");
}

export function alamatDasarData(akar: string | undefined, origin: string | undefined, awalan: string = AWALAN_BAWAAN): string {
  return akarApi(akar, origin) + awalan;
}

// Parameter yang berlaku untuk semua dataset daftar; sisanya adalah penyaring yang hanya berlaku pada dataset tertentu.
const PARAMETER_UMUM = new Set(["per_halaman", "kursor", "hitung"]);

export interface PenyaringDataset {
  didukung: string[];
  ditolak: string[]; // dipakai pada dataset ini menghasilkan 400, bukan diabaikan
}

// Penyaring yang didukung dataset (dari katalog) dan yang ditolak dengan 400 (parameter lain yang diuraikan backend tetapi bukan penyaring dataset ini).
export function penyaringDataset(ds: Pick<DatasetDok, "penyaring">, parameter: ParameterDok[]): PenyaringDataset {
  const semua = parameter.map((p) => p.nama).filter((n) => !PARAMETER_UMUM.has(n));
  const didukung = semua.filter((n) => ds.penyaring.includes(n));
  const tambahan = ds.penyaring.filter((n) => !semua.includes(n));
  return { didukung: [...didukung, ...tambahan], ditolak: semua.filter((n) => !ds.penyaring.includes(n)) };
}

// Mencari dataset menurut kode, nama, uraian, subkelompok, dan nama kolom. Semua kata harus cocok (tanpa membedakan huruf besar atau kecil).
export function cariDataset<T extends Pick<DatasetDok, "kode" | "label" | "deskripsi" | "subkelompok" | "kolom">>(dataset: T[], kata: string): T[] {
  const kunci = kata.trim().toLowerCase().split(/\s+/).filter(Boolean);
  if (kunci.length === 0) return dataset;
  return dataset.filter((d) => {
    const isi = [d.kode, d.label, d.deskripsi, d.subkelompok ?? "", ...d.kolom.map((k) => k.nama)].join("\n").toLowerCase();
    return kunci.every((k) => isi.includes(k));
  });
}

// Tipe kolom katalog dalam istilah JSON yang diterima pemanggil.
export function tipeJson(tipe: string): string {
  switch (tipe) {
    case "teks":
      return "string";
    case "bilangan":
      return "integer";
    case "desimal":
      return "number";
    case "waktu":
      return "string (ISO 8601)";
    case "logika":
      return "boolean";
    default:
      return tipe;
  }
}

type Pasangan = [string, string];

// Penyaring untuk contoh: yang paling khas dataset (KLPD dan tahun untuk pengadaan), kalau tidak ada UE1. Dataset tanpa satu pun penyaring itu mendapat contoh tanpa penyaring.
export function contohParameter(ds: Pick<DatasetDok, "penyaring">, tahun: number): Pasangan[] {
  const p: Pasangan[] = [];
  if (ds.penyaring.includes("kode_klpd")) p.push(["kode_klpd", "K10"]);
  if (ds.penyaring.includes("tahun")) p.push(["tahun", String(tahun)]);
  if (p.length === 0 && ds.penyaring.includes("kode_ue1")) p.push(["kode_ue1", "01504"]);
  return p;
}

export function teksKueri(param: Pasangan[]): string {
  return param.map(([k, v]) => `${k}=${encodeURIComponent(v)}`).join("&");
}

// Alamat lengkap dataset di server PASTI (alamat dataset dari backend sudah memuat awalan /api/data/v1), dengan kueri bila ada.
export function urlDataset(akar: string, ds: Pick<DatasetDok, "alamat">, param: Pasangan[] = []): string {
  const k = teksKueri(param);
  return `${akar}${ds.alamat}${k ? `?${k}` : ""}`;
}

export interface OpsiContoh {
  akar: string; // alamat server PASTI, tanpa garis miring akhir
  ds: Pick<DatasetDok, "alamat" | "penyaring">;
  tahun: number;
  halamanBawaan: number; // baris per permintaan bila per_halaman tidak diisi
  halamanMaks: number; // batas per_halaman menurut pengaturan
}

const AUTH_CURL = 'curl -H "Authorization: Bearer $PASTI_API_KEY"';

// Contoh kode siap salin. Semua contoh membaca kunci dari lingkungan (PASTI_API_KEY), tidak pernah menaruhnya di kode atau alamat.
export function contohKode(bahasa: Bahasa, skenario: Skenario, o: OpsiContoh): string {
  const filter = contohParameter(o.ds, o.tahun);
  const dasar = urlDataset(o.akar, o.ds);
  const perHalaman = skenario === "semua" ? o.halamanMaks : o.halamanBawaan;
  const pasangan: Pasangan[] = [...filter, ["per_halaman", String(perHalaman)]];

  if (bahasa === "curl") {
    if (skenario === "satu") return `${AUTH_CURL} \\\n  "${urlDataset(o.akar, o.ds, pasangan)}"`;
    return [
      "# Halaman pertama",
      `${AUTH_CURL} \\`,
      `  "${urlDataset(o.akar, o.ds, pasangan)}"`,
      "",
      "# Halaman berikutnya: isi kursor dengan data.kursor_berikutnya dari respons sebelumnya.",
      "# Ulangi sampai data.ada_berikutnya bernilai false.",
      `${AUTH_CURL} \\`,
      `  "${urlDataset(o.akar, o.ds, [...pasangan, ["kursor", "ISI_DENGAN_KURSOR_BERIKUTNYA"]])}"`,
    ].join("\n");
  }

  if (bahasa === "python") {
    const kamus = `{${pasangan.map(([k, v]) => (k === "per_halaman" ? `"${k}": ${v}` : `"${k}": "${v}"`)).join(", ")}}`;
    if (skenario === "satu") {
      return [
        "import os, requests",
        "",
        "r = requests.get(",
        `    "${dasar}",`,
        '    headers={"Authorization": "Bearer " + os.environ["PASTI_API_KEY"]},',
        `    params=${kamus},`,
        "    timeout=60,",
        ")",
        "r.raise_for_status()",
        'data = r.json()["data"]',
        'print(data["jumlah"], data["ada_berikutnya"])',
        'for baris in data["baris"]:',
        "    ...",
      ].join("\n");
    }
    const argumen = filter.map(([k, v]) => `${k}="${v}"`).join(", ");
    return [
      "import os, time, requests",
      "",
      `URL = "${dasar}"`,
      "s = requests.Session()",
      's.headers["Authorization"] = "Bearer " + os.environ["PASTI_API_KEY"]',
      "",
      "def tarik(**param):",
      "    kursor = None",
      "    while True:",
      `        p = dict(param, per_halaman=${perHalaman})`,
      "        if kursor:",
      '            p["kursor"] = kursor',
      "        r = s.get(URL, params=p, timeout=60)",
      "        if r.status_code == 429:  # hormati Retry-After",
      '            time.sleep(int(r.headers.get("Retry-After", "5")))',
      "            continue",
      "        r.raise_for_status()",
      '        d = r.json()["data"]',
      '        yield from d["baris"]',
      '        if not d["ada_berikutnya"]:',
      "            return",
      '        kursor = d["kursor_berikutnya"]',
      "",
      `for baris in tarik(${argumen}):`,
      "    ...",
    ].join("\n");
  }

  // Node.js 18+ (fetch bawaan); disimpan sebagai berkas .mjs agar await tingkat atas berlaku.
  const objek = (p: Pasangan[]) => (p.length ? `{ ${p.map(([k, v]) => `${k}: "${v}"`).join(", ")} }` : "{}");
  if (skenario === "satu") {
    return [
      `const url = new URL("${dasar}");`,
      `url.search = new URLSearchParams(${objek(pasangan)}).toString();`,
      'const r = await fetch(url, { headers: { Authorization: "Bearer " + process.env.PASTI_API_KEY } });',
      'if (!r.ok) throw new Error(r.status + " " + (await r.text()));',
      "const { data } = await r.json();",
      "console.log(data.jumlah, data.ada_berikutnya);",
      "for (const baris of data.baris) {",
      "  // ...",
      "}",
    ].join("\n");
  }
  return [
    `const URL_DATA = "${dasar}";`,
    "const KUNCI = process.env.PASTI_API_KEY;",
    "",
    "async function* tarik(param = {}) {",
    "  let kursor;",
    "  for (;;) {",
    `    const q = new URLSearchParams({ ...param, per_halaman: "${perHalaman}" });`,
    '    if (kursor) q.set("kursor", kursor);',
    '    const r = await fetch(URL_DATA + "?" + q, { headers: { Authorization: "Bearer " + KUNCI } });',
    "    if (r.status === 429) {  // hormati Retry-After",
    '      await new Promise((selesai) => setTimeout(selesai, Number(r.headers.get("Retry-After") ?? 5) * 1000));',
    "      continue;",
    "    }",
    '    if (!r.ok) throw new Error(r.status + " " + (await r.text()));',
    "    const { data } = await r.json();",
    "    yield* data.baris;",
    "    if (!data.ada_berikutnya) return;",
    "    kursor = data.kursor_berikutnya;",
    "  }",
    "}",
    "",
    `for await (const baris of tarik(${objek(filter)})) {`,
    "  // ...",
    "}",
  ].join("\n");
}
