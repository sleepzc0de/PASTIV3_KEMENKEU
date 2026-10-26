"use client";

import { type ReactNode, useEffect, useState } from "react";
import axios from "axios";
import { type Dokumentasi, getDokumentasi } from "@/lib/apiAkses";
import { akarApi, alamatDasarData } from "@/lib/apiAksesDokumentasi";
import { teksBatas } from "@/lib/apiAksesFormat";
import { Alert } from "@/components/ui/Alert";
import { SkeletonCards } from "@/components/ui/Skeleton";
import { DokumentasiContoh } from "./DokumentasiContoh";
import { DokumentasiDataset } from "./DokumentasiDataset";
import { Kode, KotakKode } from "./KotakKode";

const BAGIAN = [
  { id: "mulai", label: "Mulai cepat" },
  { id: "autentikasi", label: "Autentikasi" },
  { id: "endpoint", label: "Endpoint" },
  { id: "parameter", label: "Parameter daftar data" },
  { id: "respons", label: "Respons dan paginasi" },
  { id: "batas", label: "Batas laju dan kuota" },
  { id: "galat", label: "Kode galat" },
  { id: "contoh", label: "Contoh kode" },
  { id: "dataset", label: "Referensi dataset" },
];

const CONTOH_RESPONS = `{
  "success": true,
  "message": "OK",
  "data": {
    "dataset": "aset.tanah",
    "jumlah": 2,
    "baris": [
      { "id": 1201, "kode_satker": "123456", "...": "...", "disinkronkan_pada": "2026-10-20T01:15:00Z" },
      { "id": 1202, "kode_satker": "123456", "...": "...", "disinkronkan_pada": "2026-10-20T01:15:00Z" }
    ],
    "ada_berikutnya": true,
    "kursor_berikutnya": 1202,
    "total": 18342,
    "disinkronkan_terakhir": "2026-10-20T01:15:00Z"
  }
}`;

const CONTOH_GALAT = `{
  "success": false,
  "message": "Batas permintaan terlampaui. Coba lagi setelah waktu yang ditunjukkan Retry-After",
  "code": "batas_laju_terlampaui",
  "retry_after_detik": 12
}`;

function Bagian({ id, judul, children }: { id: string; judul: string; children: ReactNode }) {
  return (
    <section id={`dok-${id}`} aria-labelledby={`dok-${id}-judul`} className="scroll-mt-24 space-y-3">
      <h2 id={`dok-${id}-judul`} className="text-base font-semibold text-slate-900">
        {judul}
      </h2>
      {children}
    </section>
  );
}

function Paragraf({ children }: { children: ReactNode }) {
  return <p className="text-sm leading-relaxed text-slate-600">{children}</p>;
}

function Daftar({ children }: { children: ReactNode }) {
  return <ul className="list-disc space-y-1.5 pl-5 text-sm leading-relaxed text-slate-600 marker:text-slate-300">{children}</ul>;
}

function Tabel({ kepala, lebar, children }: { kepala: string[]; lebar: string; children: ReactNode }) {
  return (
    <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm">
      <table className={`w-full ${lebar} text-sm`}>
        <thead className="bg-slate-50 text-xs text-slate-500">
          <tr>
            {kepala.map((k) => (
              <th key={k} scope="col" className="px-4 py-2.5 text-left font-medium">
                {k}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 align-top">{children}</tbody>
      </table>
    </div>
  );
}

const sel = "px-4 py-2.5";

// Dokumentasi API Akses Data untuk superadmin: cara memakai API oleh aplikasi klien. Rujukan teknis (endpoint, parameter, header, kode galat, dataset beserta kolom dan penyaringnya,
// dan batas bawaan) dibaca dari backend, yang menyusunnya dari kode yang sama dengan yang melayani klien, jadi tidak dapat menyimpang dari perilaku sebenarnya. Uraian dan contoh
// ditulis di sini. akar: alamat server PASTI tanpa garis miring akhir.
export function DokumentasiIsi({ dok, akar }: { dok: Dokumentasi; akar: string }) {
  const [kodeContoh, setKodeContoh] = useState(() => (dok.dataset.find((x) => x.kode === "aset.tanah") ?? dok.dataset[0])?.kode ?? "");

  const dasar = alamatDasarData(akar, "", dok.alamat_dasar);
  const headerMinta = dok.header.filter((h) => h.arah === "permintaan");
  const headerJawab = dok.header.filter((h) => h.arah === "respons");
  const b = dok.bawaan;

  const pakaiContoh = (kode: string) => {
    setKodeContoh(kode);
    document.getElementById("dok-contoh")?.scrollIntoView({ behavior: "smooth", block: "start" });
  };

  return (
    <div className="lg:grid lg:grid-cols-[13rem_minmax(0,1fr)] lg:gap-8">
      <nav aria-label="Daftar isi dokumentasi" className="mb-5 lg:sticky lg:top-4 lg:mb-0 lg:self-start">
        <ul className="flex gap-1.5 overflow-x-auto pb-1 lg:flex-col lg:gap-0.5 lg:overflow-visible lg:pb-0">
          {BAGIAN.map((x) => (
            <li key={x.id} className="shrink-0">
              <a
                href={`#dok-${x.id}`}
                className="block whitespace-nowrap rounded-lg px-3 py-1.5 text-sm text-slate-600 transition-colors hover:bg-slate-100 hover:text-slate-900 focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500"
              >
                {x.label}
              </a>
            </li>
          ))}
        </ul>
      </nav>

      <div className="min-w-0 space-y-9">
        <Bagian id="mulai" judul="Mulai cepat">
          <Paragraf>
            API Akses Data memberi aplikasi lain akses <b>baca</b> ke data aset (hasil Digitalisasi Aset) dan data pengadaan (Inaproc) yang disimpan PASTI. Semua permintaan memakai <Kode>GET</Kode>, jawabannya JSON, dan
            pemanggilnya dikenali dari <b>kunci API</b>, bukan akun pengguna PASTI.
          </Paragraf>
          <div className="space-y-1.5">
            <p className="text-xs font-medium text-slate-500">Alamat dasar</p>
            <KotakKode kode={dasar} judul="Semua alamat di bawah ini relatif terhadap alamat dasar" labelSalin="Salin alamat dasar" />
            <p className="text-xs text-slate-500">
              Alamat ini diturunkan dari alamat aplikasi PASTI yang sedang Anda buka. Bila API dipasang di alamat lain, berikan alamat yang benar kepada pihak klien.
            </p>
          </div>
          <ol className="list-decimal space-y-1.5 pl-5 text-sm leading-relaxed text-slate-600 marker:font-medium marker:text-slate-400">
            <li>
              Daftarkan aplikasi di tab <b>Klien</b>: tentukan dataset yang boleh dibuka, batas pemakaian, alamat IP, dan UE1 bila perlu. Tidak ada pendaftaran mandiri.
            </li>
            <li>Terbitkan kunci API dan serahkan kepada PIC lewat jalur yang aman. Kunci hanya ditampilkan sekali; bila hilang, terbitkan kunci baru.</li>
            <li>
              Aplikasi klien memanggil API dari <b>servernya</b> dengan header <Kode>Authorization: Bearer</Kode>, lalu menarik data halaman demi halaman dengan kursor. Lihat <a className="text-blue-600 underline-offset-2 hover:underline" href="#dok-contoh">contoh kode</a>.
            </li>
          </ol>
        </Bagian>

        <Bagian id="autentikasi" judul="Autentikasi">
          <Paragraf>Kirim kunci di header setiap permintaan, memakai salah satu dari dua header berikut.</Paragraf>
          <KotakKode
            kode={`Authorization: Bearer pasti_xxxxxxxxxx_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n# atau\nX-API-Key: pasti_xxxxxxxxxx_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`}
            judul="Header permintaan"
            labelSalin="Salin contoh header autentikasi"
          />
          <Tabel kepala={["Header permintaan", "Arti"]} lebar="min-w-[28rem]">
            {headerMinta.map((h) => (
              <tr key={h.nama}>
                <td className={`${sel} font-mono text-xs text-slate-800`}>{h.nama}</td>
                <td className={`${sel} text-slate-600`}>{h.arti}</td>
              </tr>
            ))}
          </Tabel>
          <Daftar>
            <li>
              Kunci <b>tidak boleh</b> ditaruh di alamat (query string): alamat tercatat di log proxy dan riwayat peramban, sehingga kunci seperti itu ditolak.
            </li>
            <li>Simpan kunci sebagai rahasia (variabel lingkungan atau pengelola rahasia) dan panggil API dari server, bukan dari peramban atau aplikasi seluler. API ini tidak membuka CORS untuk peramban.</li>
            <li>
              Kunci dapat dibatasi ke alamat IP tertentu; permintaan dari alamat lain ditolak <Kode>403</Kode> (<Kode>alamat_ditolak</Kode>).
            </li>
            <li>Untuk pergantian rutin, rotasi kunci: kunci lama tetap berlaku selama masa tenggang (lihat tab Batas laju dan kuota) supaya klien sempat berganti.</li>
          </Daftar>
        </Bagian>

        <Bagian id="endpoint" judul="Endpoint">
          <Tabel kepala={["Metode", "Alamat", "Isi", "Kuota"]} lebar="min-w-[44rem]">
            {dok.endpoint.map((e) => (
              <tr key={e.path}>
                <td className={`${sel} font-mono text-xs font-semibold text-emerald-700`}>{e.metode}</td>
                <td className={`${sel} whitespace-nowrap font-mono text-xs text-slate-800`}>{e.path}</td>
                <td className={`${sel} text-slate-600`}>{e.isi}</td>
                <td className={`${sel} whitespace-nowrap text-slate-600`}>{e.kuota ? "Ya" : "Tidak"}</td>
              </tr>
            ))}
          </Tabel>
          <Paragraf>
            Kode dataset di <Kode>/katalog</Kode>, <Kode>/saya</Kode>, dan scope klien berawalan kelompoknya (<Kode>aset.tanah</Kode>, <Kode>pengadaan.tender_pengumuman</Kode>). Pada alamat dipakai bagian setelah titik:
            <Kode>/aset/tanah</Kode>. Hanya dataset yang diizinkan untuk kunci itu yang dapat dibuka; dataset lain dijawab <Kode>403</Kode> (<Kode>dataset_ditolak</Kode>). Dataset pengadaan yang dibuka lewat
            alamat <Kode>/aset/...</Kode> (atau sebaliknya) dijawab <Kode>404</Kode>. Kolom <Kode>id</Kode> adalah penanda baris untuk paginasi dan untuk permintaan satu baris.
          </Paragraf>
        </Bagian>

        <Bagian id="parameter" judul="Parameter daftar data">
          <Paragraf>
            Berlaku untuk <Kode>/aset/{"{dataset}"}</Kode> dan <Kode>/pengadaan/{"{dataset}"}</Kode>. Penyaring yang <b>tidak didukung</b> dataset itu ditolak <Kode>400</Kode>, tidak diabaikan diam-diam, supaya klien tidak
            menerima data tanpa penyaring yang dikiranya berlaku. Penyaring tiap dataset ada di <a className="text-blue-600 underline-offset-2 hover:underline" href="#dok-dataset">referensi dataset</a>.
          </Paragraf>
          <Tabel kepala={["Parameter", "Arti"]} lebar="min-w-[28rem]">
            {dok.parameter.map((p) => (
              <tr key={p.nama}>
                <td className={`${sel} font-mono text-xs text-slate-800`}>{p.nama}</td>
                <td className={`${sel} text-slate-600`}>{p.uraian}</td>
              </tr>
            ))}
          </Tabel>
          <Paragraf>
            Pengaturan sekarang: <Kode>per_halaman</Kode> bawaan {b.halaman_bawaan.toLocaleString("id-ID")} baris, maksimal {b.halaman_maks.toLocaleString("id-ID")} baris (nilai lebih besar dipotong ke maksimal);
            kata pencarian <Kode>q</Kode> minimal {b.min_pencarian} karakter.
          </Paragraf>
        </Bagian>

        <Bagian id="respons" judul="Respons dan paginasi">
          <Paragraf>Respons sukses selalu berbentuk berikut. Data diambil halaman demi halaman dengan <b>kursor</b>, bukan nomor halaman, sehingga tetap cepat pada tabel besar dan tidak melewatkan atau menggandakan baris.</Paragraf>
          <KotakKode kode={CONTOH_RESPONS} judul="GET /aset/tanah?per_halaman=2&hitung=1" labelSalin="Salin contoh respons" />
          <Tabel kepala={["Bidang", "Arti"]} lebar="min-w-[28rem]">
            {[
              ["baris", "Daftar objek, satu per baris data. Nama kolom huruf kecil; nilai desimal berupa angka."],
              ["jumlah", "Jumlah baris pada respons ini."],
              ["ada_berikutnya", "true bila masih ada halaman berikutnya."],
              ["kursor_berikutnya", "Nilai untuk parameter kursor pada permintaan berikutnya; null bila data sudah habis. Dataset aset: bilangan. Dataset pengadaan: teks buram."],
              ["total", "Jumlah seluruh baris yang cocok; hanya ada bila diminta dengan hitung=1 (lebih lambat, pakai seperlunya)."],
              ["disinkronkan_terakhir", "Waktu sinkronisasi terakhir dataset ini. Setiap baris juga memuat disinkronkan_pada."],
            ].map(([n, a]) => (
              <tr key={n}>
                <td className={`${sel} font-mono text-xs text-slate-800`}>{n}</td>
                <td className={`${sel} text-slate-600`}>{a}</td>
              </tr>
            ))}
          </Tabel>
          <Daftar>
            <li>
              Menarik seluruh data: ulangi permintaan dengan <Kode>kursor</Kode> dari respons sebelumnya sampai <Kode>ada_berikutnya</Kode> bernilai <Kode>false</Kode>.
            </li>
            <li>
              <b>Aset</b>: <Kode>id</Kode> berupa bilangan dan <b>berubah setelah tiap sinkronisasi</b>. Jangan menyimpannya sebagai kunci jangka panjang; pakai kode aset (mis. <Kode>kode_register_*</Kode>, <Kode>kode_satker</Kode>, <Kode>nup_*</Kode>).
            </li>
            <li>
              <b>Pengadaan</b>: <Kode>id</Kode> berupa teks buram (jangan ditafsirkan). Pakai kode paket sebagai kunci jangka panjang (<Kode>kd_rup</Kode>, <Kode>kd_tender</Kode>, <Kode>kd_nontender</Kode>, <Kode>no_kontrak</Kode>, dan
              sebagainya). Satu paket dapat punya beberapa baris (mis. pengumuman per versi, peserta, anggaran per sumber dana), dan <Kode>kd_rup</Kode> pada beberapa dataset dapat memuat beberapa kode dipisah <Kode>;</Kode>.
            </li>
            <li>Data PASTI disalin dari SLDK (aset) dan Inaproc (pengadaan) secara berkala; data berubah hanya saat sinkronisasi, jadi tarik secukupnya.</li>
          </Daftar>
        </Bagian>

        <Bagian id="batas" judul="Batas laju dan kuota">
          <Paragraf>
            Bawaan sekarang: <b>{teksBatas(b)}</b>. Batas laju memakai jendela geser; kuota memakai periode tetap yang pulih pada kelipatan periode sejak epoch UTC. Superadmin dapat menetapkan batas khusus per klien.
            Hanya pembacaan data yang menghabiskan kuota; <Kode>/katalog</Kode>, <Kode>/saya</Kode>, dan <Kode>/saya/penggunaan</Kode> tetap dikenai batas laju. Permintaan yang ditolak tidak mengurangi jatah.
          </Paragraf>
          <Tabel kepala={["Header respons", "Arti"]} lebar="min-w-[28rem]">
            {headerJawab.map((h) => (
              <tr key={h.nama}>
                <td className={`${sel} whitespace-nowrap font-mono text-xs text-slate-800`}>{h.nama}</td>
                <td className={`${sel} text-slate-600`}>{h.arti}</td>
              </tr>
            ))}
          </Tabel>
          <Daftar>
            <li>
              Baca <Kode>X-RateLimit-Remaining</Kode> dan <Kode>X-Quota-Remaining</Kode>, lalu perlambat sebelum habis. Saat menerima <Kode>429</Kode>, tunggu selama <Kode>Retry-After</Kode> sebelum mengulang.
            </li>
            <li>Gunakan per_halaman besar daripada banyak permintaan kecil.</li>
          </Daftar>
        </Bagian>

        <Bagian id="galat" judul="Kode galat">
          <Tabel kepala={["HTTP", "code", "Arti"]} lebar="min-w-[40rem]">
            {dok.galat.map((g) => (
              <tr key={`${g.http}-${g.kode ?? ""}`}>
                <td className={`${sel} font-mono text-xs font-semibold text-slate-800`}>{g.http}</td>
                <td className={`${sel} font-mono text-xs text-slate-700`}>{g.kode ?? "-"}</td>
                <td className={`${sel} text-slate-600`}>{g.arti}</td>
              </tr>
            ))}
          </Tabel>
          <KotakKode kode={CONTOH_GALAT} judul="Contoh isi respons 429" labelSalin="Salin contoh respons galat" />
          <Paragraf>
            Isi galat selalu memuat <Kode>success: false</Kode> dan <Kode>message</Kode>; <Kode>code</Kode> ada pada galat gerbang, dan <Kode>retry_after_detik</Kode> pada penolakan yang bisa diulang. Setiap respons
            membawa <Kode>X-Request-ID</Kode>; sertakan saat melapor agar permintaan dapat dilacak di halaman Pemantauan penarikan data.
          </Paragraf>
        </Bagian>

        <Bagian id="contoh" judul="Contoh kode">
          <DokumentasiContoh dok={dok} akar={akar} kodeDataset={kodeContoh} onKodeDataset={setKodeContoh} />
        </Bagian>

        <Bagian id="dataset" judul="Referensi dataset">
          <Alert
            tone="info"
            message="Data yang tidak pernah diberikan: kolom data pribadi aset (mis. nama penghuni rumah negara), tautan foto, dan nama pengguna bangunan; pada pengadaan, data orang (nama, NIP, jabatan, kontak PPK, Pokja, dan petugas lain), NPWP, rekening bank, isian bebas petugas, tautan dokumen, dan riwayat BAP, BAST, atau penilaian kinerja. Nama dan kode penyedia diberikan."
          />
          <Paragraf>
            <b>Batas UE1 pada data pengadaan.</b> Inaproc hanya mencatat kode satker 6 digit, tanpa UE1. Untuk klien yang dibatasi ke UE1, atau bila klien memakai <Kode>kode_ue1</Kode>, PASTI menentukan satker milik UE1
            itu dari data aset; pengadaan satker yang tidak ada di data aset PASTI tidak terlihat. Dataset yang tidak memuat kode satker dibatasi lewat kode paketnya, dan baris yang paketnya tidak ditemukan pada data satker
            tidak ikut bagi klien yang dibatasi UE1.
          </Paragraf>
          <DokumentasiDataset dok={dok} akar={akar} onPakaiContoh={pakaiContoh} />
        </Bagian>
      </div>
    </div>
  );
}

// Memuat bahan dokumentasi dari backend (khusus superadmin) lalu menampilkannya.
export function DokumentasiApi() {
  const [dok, setDok] = useState<Dokumentasi | null>(null);
  const [galat, setGalat] = useState("");
  const [akar, setAkar] = useState("");

  useEffect(() => {
    setAkar(akarApi(process.env.NEXT_PUBLIC_API_ROOT_URL, window.location.origin));
    getDokumentasi()
      .then(setDok)
      .catch((err) => setGalat(axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : "Gagal memuat dokumentasi"));
  }, []);

  if (galat) return <Alert message={galat} />;
  if (!dok) return <SkeletonCards count={3} />;
  return <DokumentasiIsi dok={dok} akar={akar} />;
}
