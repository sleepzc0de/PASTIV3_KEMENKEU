"use client";

import { useState } from "react";
import Link from "next/link";
import { AnEkatalog, AnHasil, AnKontrak, AnPemilihan, AnRUP, AnWawasan } from "@/lib/api";
import { bagi, formatAngka, formatPersen, formatTanggal, rupiahRingkas } from "@/lib/pengadaan";
import { BarList, ChartCard, DataTable, StatTile } from "@/components/ui/charts";
import { AngkaPokok, Corong, DaftarWawasan, Histogram, Komposisi, KolomBulanan, WARNA } from "./grafik";
import { KOLOM_TABEL, KartuKosong, Subjudul, keBarItems, keBarisTabel, keKomposisi, saring } from "./dasborUtil";

const TAUTAN_TARIK = { href: "/dashboard/pengadaan-terpadu/penarikan", label: "Buka halaman Penarikan Data" };

const WARNA_CORONG: Record<string, string> = {
  Tender: "#2a78d6",
  "Non-tender": "#eb6834",
  "E-Purchasing": "#1baf7a",
  "Belum diproses": WARNA.netral,
};

function Grid({ children, kolom = 2 }: { children: React.ReactNode; kolom?: 2 | 3 | 4 }) {
  const kelas = { 2: "lg:grid-cols-2", 3: "lg:grid-cols-3", 4: "sm:grid-cols-2 lg:grid-cols-4" }[kolom];
  return <div className={`grid gap-4 ${kelas}`}>{children}</div>;
}

function tentangBagian(hasil: AnHasil, kunci: string) {
  return hasil.galat?.[kunci] ? `Bagian ini gagal dihitung (${hasil.galat[kunci]}). Coba hitung ulang; bila berulang, periksa log server.` : "Belum ada data untuk bagian ini.";
}

// Enam wawasan paling mendesak; sisanya dibuka dengan tombol.
function WawasanRingkas({ wawasan }: { wawasan: AnWawasan[] }) {
  const [semua, setSemua] = useState(false);
  const tampil = semua ? wawasan : wawasan.slice(0, 6);
  return (
    <div className="space-y-3">
      <DaftarWawasan wawasan={tampil} />
      {wawasan.length > 6 && (
        <button type="button" onClick={() => setSemua((v) => !v)} className="text-xs font-medium text-blue-600 hover:text-blue-700">
          {semua ? "Tampilkan lebih sedikit" : `Tampilkan semua wawasan (${wawasan.length})`}
        </button>
      )}
    </div>
  );
}

// ============================================================
// Ikhtisar
// ============================================================

export function Ikhtisar({ hasil, wawasan }: { hasil: AnHasil; wawasan: AnWawasan[] }) {
  const { rup, pemilihan, kontrak, ekatalog, corong, pembanding } = hasil;
  const nilaiEkat = (ekatalog?.v5.nilai ?? 0) + (ekatalog?.v6.nilai ?? 0);
  const ubahPagu = pembanding && pembanding.rup_pagu > 0 && rup ? ((rup.total_pagu - pembanding.rup_pagu) / pembanding.rup_pagu) * 100 : null;
  const efisiensi = pemilihan?.efisiensi;

  return (
    <div className="space-y-6">
      <Grid kolom={4}>
        <AngkaPokok
          label="Pagu RUP (penyedia)"
          nilai={rupiahRingkas(rup?.total_pagu)}
          keterangan={
            <>
              {formatAngka(rup?.total_paket)} paket
              {ubahPagu !== null && (
                <span className={ubahPagu >= 0 ? "text-slate-600" : "text-slate-600"}>
                  {" "}
                  · {ubahPagu >= 0 ? "naik" : "turun"} {formatPersen(Math.abs(ubahPagu))} dari {pembanding?.tahun}
                </span>
              )}
            </>
          }
        />
        <AngkaPokok
          label="Tender & non-tender"
          nilai={formatAngka((pemilihan?.tender_jumlah ?? 0) + (pemilihan?.non_tender_jumlah ?? 0))}
          keterangan={`${formatAngka(pemilihan?.tender_jumlah)} tender · ${formatAngka(pemilihan?.non_tender_jumlah)} non-tender · pagu ${rupiahRingkas((pemilihan?.tender_pagu ?? 0) + (pemilihan?.non_tender_pagu ?? 0))}`}
        />
        <AngkaPokok
          label="Nilai kontrak hasil pemilihan"
          nilai={rupiahRingkas(pemilihan?.nilai_kontrak)}
          keterangan={`${formatAngka((kontrak?.tender_jumlah ?? 0) + (kontrak?.non_tender_jumlah ?? 0))} kontrak tercatat di e-kontrak`}
        />
        <AngkaPokok label="E-Purchasing (V5 + V6)" nilai={rupiahRingkas(nilaiEkat)} keterangan={`V5 ${rupiahRingkas(ekatalog?.v5.nilai)} · V6 ${rupiahRingkas(ekatalog?.v6.nilai)}`} />
      </Grid>

      <div className="grid gap-4 lg:grid-cols-5">
        <div className="lg:col-span-3">
          <ChartCard
            title="Dari perencanaan ke proses pengadaan"
            subtitle="Setiap paket RUP aktif dicocokkan dengan pengumuman tender, non-tender, atau paket e-purchasing lewat kode RUP"
            table={corong ? <DataTable columns={KOLOM_TABEL} rows={keBarisTabel(corong.tahap)} /> : undefined}
          >
            {corong ? <Corong tahap={corong.tahap} total={corong.total_pagu} warna={WARNA_CORONG} /> : <p className="py-6 text-center text-sm text-slate-400">{tentangBagian(hasil, "corong")}</p>}
            {corong && corong.total_paket > 0 && (
              <p className="mt-4 text-xs text-slate-500">
                Total {formatAngka(corong.total_paket)} paket RUP aktif dengan pagu {rupiahRingkas(corong.total_pagu)}. Paket yang cocok dengan beberapa saluran dihitung di saluran pertama menurut urutan di atas.
              </p>
            )}
          </ChartCard>
        </div>
        <div className="lg:col-span-2">
          <ChartCard
            title="Efisiensi harga"
            subtitle="Selisih HPS dan nilai kontrak pada paket yang sudah selesai"
            table={
              efisiensi && efisiensi.sampel > 0 ? (
                <DataTable columns={["Rentang", "Jumlah paket"]} rows={efisiensi.sebaran.map((s) => [s.label, formatAngka(s.jumlah)])} />
              ) : undefined
            }
          >
            {efisiensi && efisiensi.sampel > 0 ? (
              <div className="space-y-4">
                <div>
                  <p className="text-3xl font-bold tracking-tight text-slate-900 tabular-nums">{formatPersen(efisiensi.persen)}</p>
                  <p className="mt-0.5 text-xs text-slate-500">
                    HPS {rupiahRingkas(efisiensi.total_hps)} menjadi kontrak {rupiahRingkas(efisiensi.total_kontrak)} · median per paket {formatPersen(efisiensi.median)} · {formatAngka(efisiensi.sampel)} paket
                  </p>
                </div>
                <Histogram
                  item={efisiensi.sebaran.map((s, i) => ({ label: s.label, jumlah: s.jumlah, warna: i === 0 ? WARNA.penting : WARNA.merek, tebal: i === 0 && s.jumlah > 0 }))}
                  ringkasan={`Sebaran efisiensi harga per paket: ${efisiensi.sebaran.map((s) => `${s.label} ${s.jumlah} paket`).join(", ")}`}
                />
              </div>
            ) : (
              <p className="py-6 text-center text-sm text-slate-400">Belum ada paket selesai yang punya HPS dan nilai kontrak</p>
            )}
          </ChartCard>
        </div>
      </div>

      <div>
        <Subjudul deskripsi="Dihitung otomatis dari data yang tersimpan. Angka dalam kalimat mengikuti filter tahun dan KLPD di atas.">Wawasan analitik</Subjudul>
        <WawasanRingkas wawasan={wawasan} />
      </div>

      <ChartCard
        title="Nilai kontrak per bulan"
        subtitle="Menurut tanggal kontrak di e-kontrak (tender dan non-tender)"
        table={kontrak ? <DataTable columns={["Bulan", "Jumlah", "Nilai"]} rows={kontrak.per_bulan.map((b) => [String(b.bulan), formatAngka(b.jumlah), rupiahRingkas(b.nilai)])} /> : undefined}
      >
        {kontrak ? <KolomBulanan seri={[{ nama: "Nilai kontrak", warna: WARNA.merek, data: kontrak.per_bulan }]} satuanJumlah="kontrak" /> : <p className="py-6 text-center text-sm text-slate-400">{tentangBagian(hasil, "kontrak")}</p>}
      </ChartCard>
    </div>
  );
}

// ============================================================
// Perencanaan (RUP)
// ============================================================

export function BagianRUP({ hasil, wawasan }: { hasil: AnHasil; wawasan: AnWawasan[] }) {
  const r: AnRUP | null = hasil.rup;
  if (!r || (r.total_paket === 0 && r.paket_swakelola === 0)) {
    return <KartuKosong judul="Belum ada data perencanaan" isi={`${tentangBagian(hasil, "rup")} Tarik dataset Paket Penyedia, Paket Swakelola, dan Program Master untuk KLPD ${hasil.kode_klpd} tahun ${hasil.tahun}.`} tautan={TAUTAN_TARIK} />;
  }
  const sudah = r.status_umumkan.filter((s) => /umumkan/i.test(s.label) && !/belum|tidak/i.test(s.label));
  const nilaiSudah = sudah.reduce((a, s) => a + s.nilai, 0);

  return (
    <div className="space-y-6">
      <Grid kolom={4}>
        <StatTile label="Paket penyedia" value={formatAngka(r.total_paket)} sub={`pagu ${rupiahRingkas(r.total_pagu)}`} />
        <StatTile label="Paket swakelola" value={formatAngka(r.paket_swakelola)} sub={r.pagu_swakelola > 0 ? `pagu terumumkan ${rupiahRingkas(r.pagu_swakelola)}` : "pagu belum ada di data"} />
        <StatTile label="Pagu program" value={rupiahRingkas(r.pagu_program)} sub={r.pagu_program > 0 ? `paket penyedia setara ${formatPersen(bagi(r.total_pagu, r.pagu_program))}` : "program master belum ditarik"} />
        <StatTile label="Terumumkan" value={formatPersen(bagi(nilaiSudah, r.total_pagu))} sub={`dari pagu paket penyedia (${rupiahRingkas(nilaiSudah)})`} />
      </Grid>

      <ChartCard
        title="Rencana pemilihan per bulan"
        subtitle="Pagu menurut bulan mulai pemilihan yang dijadwalkan (paket yang punya jadwal)"
        table={<DataTable columns={["Bulan", "Paket", "Pagu"]} rows={r.per_bulan_pemilihan.map((b) => [String(b.bulan), formatAngka(b.jumlah), rupiahRingkas(b.nilai)])} />}
      >
        <KolomBulanan seri={[{ nama: "Pagu rencana pemilihan", warna: WARNA.merek, data: r.per_bulan_pemilihan }]} />
      </ChartCard>

      <Grid>
        <ChartCard title="Pagu menurut metode pengadaan" subtitle="Metode terbesar dulu" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(r.per_metode)} />}>
          <BarList items={keBarItems(r.per_metode)} wide />
        </ChartCard>
        <ChartCard title="Pagu menurut jenis pengadaan" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(r.per_jenis)} />}>
          <Komposisi bagian={keKomposisi(r.per_jenis)} ringkasan={`Komposisi pagu menurut jenis pengadaan: ${r.per_jenis.map((j) => j.label).join(", ")}`} />
        </ChartCard>
        <ChartCard title="Satuan kerja dengan pagu terbesar" subtitle="10 teratas" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(r.top_satker)} />}>
          <BarList items={keBarItems(r.top_satker)} wide />
        </ChartCard>
        <ChartCard title="Status pengumuman RUP" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(r.status_umumkan)} />}>
          <Komposisi bagian={keKomposisi(r.status_umumkan)} ringkasan={`Komposisi pagu menurut status pengumuman: ${r.status_umumkan.map((j) => j.label).join(", ")}`} />
        </ChartCard>
        <ChartCard title="Keberpihakan UKM" subtitle="Status UKM paket penyedia, menurut pagu" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(r.status_ukm)} />}>
          <Komposisi bagian={keKomposisi(r.status_ukm)} ringkasan={`Komposisi pagu menurut status UKM: ${r.status_ukm.map((j) => j.label).join(", ")}`} />
        </ChartCard>
        <ChartCard title="Produk dalam negeri (PDN)" subtitle="Status PDN paket penyedia, menurut pagu" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(r.status_pdn)} />}>
          <Komposisi bagian={keKomposisi(r.status_pdn)} ringkasan={`Komposisi pagu menurut status PDN: ${r.status_pdn.map((j) => j.label).join(", ")}`} />
        </ChartCard>
      </Grid>

      <div>
        <Subjudul>Wawasan perencanaan</Subjudul>
        <DaftarWawasan wawasan={saring(wawasan, "rup")} kosong="Tidak ada temuan khusus pada perencanaan." />
      </div>
    </div>
  );
}

// ============================================================
// Pemilihan (tender dan non-tender)
// ============================================================

export function BagianPemilihan({ hasil, wawasan }: { hasil: AnHasil; wawasan: AnWawasan[] }) {
  const p: AnPemilihan | null = hasil.pemilihan;
  if (!p || (p.tender_jumlah === 0 && p.non_tender_jumlah === 0 && p.efisiensi.sampel === 0)) {
    return <KartuKosong judul="Belum ada data pemilihan" isi={`${tentangBagian(hasil, "pemilihan")} Tarik dataset Pengumuman Tender, Peserta Tender, Tender Selesai, Nilai Tender Selesai, dan Non-Tender untuk KLPD ${hasil.kode_klpd} tahun ${hasil.tahun}.`} tautan={TAUTAN_TARIK} />;
  }
  const e = p.efisiensi;
  const ps = p.persaingan;
  const pasar = p.pasar;
  const pTeratas = pasar.top[0] ? bagi(pasar.top[0].nilai, pasar.total_nilai) : 0;

  return (
    <div className="space-y-6">
      <Grid kolom={4}>
        <StatTile label="Tender diumumkan" value={formatAngka(p.tender_jumlah)} sub={`pagu ${rupiahRingkas(p.tender_pagu)} · HPS ${rupiahRingkas(p.tender_hps)}`} />
        <StatTile label="Non-tender diumumkan" value={formatAngka(p.non_tender_jumlah)} sub={`pagu ${rupiahRingkas(p.non_tender_pagu)} · HPS ${rupiahRingkas(p.non_tender_hps)}`} />
        <StatTile label="Nilai kontrak" value={rupiahRingkas(p.nilai_kontrak)} sub={`${formatAngka(p.tender_selesai)} tender selesai`} />
        <StatTile label="Lama pemilihan tender" value={p.waktu_proses.sampel > 0 ? `${formatAngka(p.waktu_proses.median)} hari` : "-"} sub={p.waktu_proses.sampel > 0 ? `median, rata-rata ${formatAngka(p.waktu_proses.rata)} hari (${formatAngka(p.waktu_proses.sampel)} tender)` : "belum ada data tanggal penetapan"} />
      </Grid>

      <ChartCard
        title="Pengumuman per bulan"
        subtitle="Tender dan non-tender menurut bulan pengumuman"
        table={
          <DataTable
            columns={["Bulan", "Tender", "Non-tender"]}
            rows={p.per_bulan_tender.map((b, i) => [String(b.bulan), `${formatAngka(b.jumlah)} (${rupiahRingkas(b.nilai)})`, `${formatAngka(p.per_bulan_non_tender[i]?.jumlah)} (${rupiahRingkas(p.per_bulan_non_tender[i]?.nilai)})`])}
          />
        }
      >
        <KolomBulanan
          metrik="jumlah"
          satuanJumlah="paket"
          seri={[
            { nama: "Tender", warna: WARNA.seri1, data: p.per_bulan_tender },
            { nama: "Non-tender", warna: WARNA.seri2, data: p.per_bulan_non_tender },
          ]}
        />
      </ChartCard>

      <Grid>
        <ChartCard title="Status tender" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(p.status_tender)} />}>
          <BarList items={keBarItems(p.status_tender, "jumlah", "tender")} />
        </ChartCard>
        <ChartCard title="Metode pemilihan tender" subtitle="Menurut pagu" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(p.metode_tender)} />}>
          <BarList items={keBarItems(p.metode_tender)} />
        </ChartCard>
        <ChartCard title="Metode pengadaan non-tender" subtitle="Menurut pagu" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(p.metode_non_tender)} />}>
          <BarList items={keBarItems(p.metode_non_tender)} />
        </ChartCard>
        <ChartCard title="Jenis pengadaan tender" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(p.jenis_tender)} />}>
          <Komposisi bagian={keKomposisi(p.jenis_tender)} ringkasan={`Komposisi pagu tender menurut jenis pengadaan: ${p.jenis_tender.map((j) => j.label).join(", ")}`} />
        </ChartCard>
      </Grid>

      <Grid>
        <ChartCard
          title="Persaingan tender"
          subtitle="Jumlah peserta per tender"
          table={ps.tender_berpeserta > 0 ? <DataTable columns={["Peserta", "Jumlah tender"]} rows={ps.sebaran.map((s) => [s.label, formatAngka(s.jumlah)])} /> : undefined}
        >
          {ps.tender_berpeserta > 0 ? (
            <div className="space-y-4">
              <div className="flex flex-wrap gap-x-8 gap-y-2">
                <div>
                  <p className="text-2xl font-bold text-slate-900 tabular-nums">{formatAngka(ps.rata_peserta, 1)}</p>
                  <p className="text-xs text-slate-500">rata-rata peserta per tender</p>
                </div>
                <div>
                  <p className="text-2xl font-bold text-slate-900 tabular-nums">{formatPersen(bagi(ps.satu_peserta, ps.tender_berpeserta))}</p>
                  <p className="text-xs text-slate-500">
                    tender hanya satu peserta ({formatAngka(ps.satu_peserta)} dari {formatAngka(ps.tender_berpeserta)})
                  </p>
                </div>
              </div>
              <Histogram
                item={ps.sebaran.map((s, i) => ({ label: s.label, jumlah: s.jumlah, warna: i === 0 ? WARNA.penting : WARNA.merek, tebal: i === 0 && s.jumlah > 0 }))}
                satuan="tender"
                ringkasan={`Sebaran jumlah peserta per tender: ${ps.sebaran.map((s) => `${s.label} ${s.jumlah} tender`).join(", ")}`}
              />
            </div>
          ) : (
            <p className="py-6 text-center text-sm text-slate-400">Data Peserta Tender belum ada</p>
          )}
        </ChartCard>

        <ChartCard
          title="Penyedia dengan nilai kontrak terbesar"
          subtitle="Tender dan non-tender selesai, 10 teratas"
          table={pasar.top.length > 0 ? <DataTable columns={["Penyedia", "Kontrak", "Nilai", "Porsi"]} rows={pasar.top.map((x) => [x.label, formatAngka(x.jumlah), rupiahRingkas(x.nilai), formatPersen(bagi(x.nilai, pasar.total_nilai))])} /> : undefined}
        >
          {pasar.top.length > 0 ? (
            <div className="space-y-4">
              <BarList
                wide
                items={pasar.top.map((x, i) => ({ key: `${x.label}-${i}`, label: x.label, value: x.nilai, display: rupiahRingkas(x.nilai), note: formatPersen(bagi(x.nilai, pasar.total_nilai), 0) }))}
              />
              <p className="text-xs text-slate-500">
                {formatAngka(pasar.jumlah_penyedia)} penyedia, total {rupiahRingkas(pasar.total_nilai)}. Penyedia terbesar menguasai {formatPersen(pTeratas)}; indeks HHI {formatAngka(pasar.hhi)} (di bawah 1.500 tidak terkonsentrasi, 1.500-2.500 sedang, di atas 2.500 tinggi).
              </p>
            </div>
          ) : (
            <p className="py-6 text-center text-sm text-slate-400">Data nilai tender selesai belum ada</p>
          )}
        </ChartCard>
      </Grid>

      {e.sampel > 0 && (
        <p className="text-xs text-slate-500">
          Efisiensi harga: {formatPersen(e.persen)} dari {formatAngka(e.sampel)} paket (median {formatPersen(e.median)}). Rinciannya ada di tab Ikhtisar.
        </p>
      )}

      <div>
        <Subjudul>Wawasan pemilihan</Subjudul>
        <DaftarWawasan wawasan={saring(wawasan, "pemilihan")} kosong="Tidak ada temuan khusus pada proses pemilihan." />
      </div>
    </div>
  );
}

// ============================================================
// Kontrak
// ============================================================

export function BagianKontrak({ hasil, wawasan }: { hasil: AnHasil; wawasan: AnWawasan[] }) {
  const k: AnKontrak | null = hasil.kontrak;
  if (!k || k.tender_jumlah + k.non_tender_jumlah === 0) {
    return <KartuKosong judul="Belum ada data kontrak" isi={`${tentangBagian(hasil, "kontrak")} Tarik dataset Kontrak Tender dan Kontrak Non-Tender untuk KLPD ${hasil.kode_klpd} tahun ${hasil.tahun}.`} tautan={TAUTAN_TARIK} />;
  }
  const total = k.tender_jumlah + k.non_tender_jumlah;
  const akan = k.akan_berakhir ?? [];

  return (
    <div className="space-y-6">
      <Grid kolom={4}>
        <StatTile label="Kontrak tender" value={formatAngka(k.tender_jumlah)} sub={rupiahRingkas(k.tender_nilai)} />
        <StatTile label="Kontrak non-tender" value={formatAngka(k.non_tender_jumlah)} sub={rupiahRingkas(k.non_tender_nilai)} />
        <StatTile label="Kontrak beradendum" value={formatPersen(bagi(k.addendum, total))} sub={`${formatAngka(k.addendum)} dari ${formatAngka(total)} kontrak`} />
        <StatTile label={`Berakhir dalam ${k.hari_peringatan} hari`} value={formatAngka(k.berakhir_dalam)} sub={k.berakhir_dalam > 0 ? `nilai ${rupiahRingkas(k.nilai_berakhir)}` : "tidak ada"} />
      </Grid>

      <Grid>
        <ChartCard title="Nilai kontrak per bulan" subtitle="Menurut tanggal kontrak" table={<DataTable columns={["Bulan", "Kontrak", "Nilai"]} rows={k.per_bulan.map((b) => [String(b.bulan), formatAngka(b.jumlah), rupiahRingkas(b.nilai)])} />}>
          <KolomBulanan seri={[{ nama: "Nilai kontrak", warna: WARNA.merek, data: k.per_bulan }]} satuanJumlah="kontrak" />
        </ChartCard>
        <ChartCard title="Status kontrak" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(k.status, "kontrak")} />}>
          <BarList items={keBarItems(k.status, "jumlah", "kontrak")} />
        </ChartCard>
      </Grid>

      <section className="rounded-2xl border border-slate-200/80 bg-white p-4 shadow-sm sm:p-5">
        <h3 className="text-sm font-semibold text-slate-900">Kontrak yang segera berakhir</h3>
        <p className="mt-0.5 text-xs text-slate-500">Berakhir dalam {k.hari_peringatan} hari ke depan, tidak termasuk yang berstatus selesai, putus, atau batal. Menampilkan hingga 10 yang paling dekat.</p>
        {akan.length === 0 ? (
          <p className="py-6 text-center text-sm text-slate-400">Tidak ada kontrak yang berakhir dalam {k.hari_peringatan} hari</p>
        ) : (
          <div className="mt-3 overflow-x-auto">
            <table className="w-full min-w-max text-xs">
              <thead>
                <tr className="border-b border-slate-200 text-left text-slate-500">
                  <th scope="col" className="py-1.5 pr-3 font-medium">Paket</th>
                  <th scope="col" className="px-3 py-1.5 font-medium">Penyedia</th>
                  <th scope="col" className="px-3 py-1.5 font-medium">No. kontrak</th>
                  <th scope="col" className="px-3 py-1.5 text-right font-medium">Nilai</th>
                  <th scope="col" className="px-3 py-1.5 font-medium">Berakhir</th>
                  <th scope="col" className="py-1.5 pl-3 text-right font-medium">Sisa</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {akan.map((x, i) => (
                  <tr key={`${x.no_kontrak}-${i}`}>
                    <td className="max-w-[18rem] py-1.5 pr-3 text-slate-800">
                      <span className="line-clamp-2 break-words" title={x.nama_paket}>{x.nama_paket || "-"}</span>
                      <span className="text-[11px] text-slate-400">{x.jenis}</span>
                    </td>
                    <td className="max-w-[14rem] truncate px-3 py-1.5 text-slate-700" title={x.penyedia}>{x.penyedia || "-"}</td>
                    <td className="px-3 py-1.5 text-slate-600">{x.no_kontrak || "-"}</td>
                    <td className="whitespace-nowrap px-3 py-1.5 text-right tabular-nums text-slate-800">{rupiahRingkas(x.nilai)}</td>
                    <td className="whitespace-nowrap px-3 py-1.5 text-slate-600">{formatTanggal(x.berakhir)}</td>
                    <td className={`whitespace-nowrap py-1.5 pl-3 text-right font-medium tabular-nums ${x.sisa_hari <= 14 ? "text-red-700" : "text-slate-700"}`}>{x.sisa_hari} hari</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <div>
        <Subjudul>Wawasan kontrak</Subjudul>
        <DaftarWawasan wawasan={saring(wawasan, "kontrak")} kosong="Tidak ada temuan khusus pada kontrak." />
      </div>
    </div>
  );
}

// ============================================================
// E-Katalog
// ============================================================

export function BagianEkatalog({ hasil, wawasan }: { hasil: AnHasil; wawasan: AnWawasan[] }) {
  const e: AnEkatalog | null = hasil.ekatalog;
  if (!e || (e.v5.paket === 0 && e.v6.order === 0 && e.v6.transaksi_baris === 0)) {
    return <KartuKosong judul="Belum ada data E-Katalog" isi={`${tentangBagian(hasil, "ekatalog")} Tarik dataset Paket E-Purchasing (V5 dan V6) dan Transaksi per Produk untuk KLPD ${hasil.kode_klpd} tahun ${hasil.tahun}.`} tautan={TAUTAN_TARIK} />;
  }
  const bulan = (b: { bulan: number; jumlah: number; nilai: number }[]) => b.map((x) => [String(x.bulan), formatAngka(x.jumlah), rupiahRingkas(x.nilai)]);

  return (
    <div className="space-y-6">
      <Grid kolom={4}>
        <StatTile label="E-Purchasing V5" value={rupiahRingkas(e.v5.nilai)} sub={`${formatAngka(e.v5.paket)} paket`} />
        <StatTile label="E-Purchasing V6" value={rupiahRingkas(e.v6.nilai)} sub={`${formatAngka(e.v6.order)} order${e.v6.order_swasta > 0 ? ` · ${formatAngka(e.v6.order_swasta)} swasta` : ""}`} />
        <StatTile label="Transaksi per produk (V6)" value={rupiahRingkas(e.v6.transaksi_nilai)} sub={`${formatAngka(e.v6.transaksi_baris)} baris`} />
        <StatTile label="Porsi paket swasta (V6)" value={formatPersen(bagi(e.v6.nilai_swasta, e.v6.nilai))} sub={rupiahRingkas(e.v6.nilai_swasta)} />
      </Grid>

      <ChartCard
        title="Nilai e-purchasing per bulan"
        subtitle="E-Katalog V5 dan V6 menurut tanggal pesanan"
        table={<DataTable columns={["Bulan", "V5 jumlah", "V5 nilai", "V6 jumlah", "V6 nilai"]} rows={e.v5.per_bulan.map((b, i) => [String(b.bulan), formatAngka(b.jumlah), rupiahRingkas(b.nilai), formatAngka(e.v6.per_bulan[i]?.jumlah), rupiahRingkas(e.v6.per_bulan[i]?.nilai)])} />}
      >
        <KolomBulanan
          satuanJumlah="paket/order"
          seri={[
            { nama: "E-Katalog V5", warna: WARNA.seri1, data: e.v5.per_bulan },
            { nama: "E-Katalog V6", warna: WARNA.seri2, data: e.v6.per_bulan },
          ]}
        />
      </ChartCard>

      <Grid>
        <ChartCard title="Komoditas terbesar (V5)" subtitle="Menurut nilai, nama dari data Komoditas bila sudah ditarik" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(e.v5.top_komoditas)} />}>
          <BarList items={keBarItems(e.v5.top_komoditas)} wide />
        </ChartCard>
        <ChartCard title="Penyedia terbesar (V5)" subtitle="Nama dari data Penyedia bila sudah ditarik; selebihnya kode" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(e.v5.top_penyedia)} />}>
          <BarList items={keBarItems(e.v5.top_penyedia)} wide />
        </ChartCard>
        <ChartCard title="Penyedia terbesar (V6)" subtitle="Nama dari data Penyedia V6 bila sudah ditarik; selebihnya kode" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(e.v6.top_penyedia, "order")} />}>
          <BarList items={keBarItems(e.v6.top_penyedia)} wide />
        </ChartCard>
        <ChartCard title="Kategori produk terbesar (V6)" subtitle="Dari transaksi per produk, kategori tingkat 1" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(e.v6.top_kategori, "transaksi")} />}>
          <BarList items={keBarItems(e.v6.top_kategori)} wide />
        </ChartCard>
        <ChartCard title="Status paket V5" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(e.v5.status)} />}>
          <Komposisi bagian={keKomposisi(e.v5.status)} ringkasan={`Komposisi nilai paket V5 menurut status: ${e.v5.status.map((s) => s.label).join(", ")}`} />
        </ChartCard>
        <ChartCard title="Status order V6" table={<DataTable columns={KOLOM_TABEL} rows={keBarisTabel(e.v6.status, "order")} />}>
          <Komposisi bagian={keKomposisi(e.v6.status)} ringkasan={`Komposisi nilai order V6 menurut status: ${e.v6.status.map((s) => s.label).join(", ")}`} />
        </ChartCard>
      </Grid>

      <p className="text-xs text-slate-500">
        Nama komoditas dan penyedia muncul bila datanya sudah ditarik di <Link href="/dashboard/pengadaan-terpadu/penarikan" className="font-medium text-blue-600 hover:text-blue-700">Penarikan Data</Link> (dataset rujukan ditarik per kode).
      </p>

      <div>
        <Subjudul>Wawasan E-Katalog</Subjudul>
        <DaftarWawasan wawasan={saring(wawasan, "ekatalog")} kosong="Tidak ada temuan khusus pada e-purchasing." />
      </div>
    </div>
  );
}
