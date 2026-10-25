"use client";

import { useCallback, useEffect, useState } from "react";
import axios from "axios";
import Link from "next/link";
import { Activity, Ban, BadgeCheck, KeyRound, Pencil, PlusCircle, RefreshCw, RotateCw, ShieldCheck, XCircle } from "lucide-react";
import {
  type InfoDataset,
  type Klien,
  type KunciBaru,
  type KunciInfo,
  type Pengaturan,
  type PemakaianKlien,
  deleteKunci,
  getDetailKlien,
  postAktifkan,
  postCabut,
  postKunci,
  postSetujui,
  postTolak,
  putKlien,
} from "@/lib/apiAkses";
import { type NilaiFormAkses, formatJumlah, gabungDaftar, nilaiAwalAkses, susunBodyAkses, teksBatas, teksOverride } from "@/lib/apiAksesFormat";
import { teksRelatif, waktuWIB } from "@/lib/auditFilter";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { DetailEntry } from "@/components/ui/DetailEntry";
import { ModalShell } from "@/components/ui/ModalShell";
import { SkeletonCards } from "@/components/ui/Skeleton";
import { useToast } from "@/components/ui/Toast";
import { FormAkses } from "./FormAkses";
import { KunciBaruModal } from "./KunciBaruModal";
import { BatangKuota, LencanaStatus } from "./lencana";

type Mode = "lihat" | "setujui" | "tolak" | "ubah" | "cabut" | "kunci";

function pesanGalat(err: unknown, cadangan: string): string {
  return axios.isAxiosError(err) && err.response?.data?.message ? err.response.data.message : cadangan;
}

function statusKunci(k: KunciInfo): { teks: string; kelas: string } {
  if (k.dicabut_at) return { teks: "Dicabut", kelas: "bg-red-50 text-red-700 ring-red-200" };
  if (k.kedaluwarsa_at && new Date(k.kedaluwarsa_at).getTime() <= Date.now()) return { teks: "Kedaluwarsa", kelas: "bg-slate-100 text-slate-600 ring-slate-300" };
  return { teks: "Aktif", kelas: "bg-emerald-50 text-emerald-700 ring-emerald-200" };
}

const areaCls =
  "w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition-all placeholder:text-slate-400 hover:border-slate-400 focus:border-blue-500 focus:shadow-glow";

// Rincian satu klien API untuk superadmin: data pendaftaran, keputusan (setujui, tolak, cabut, aktifkan kembali), scope dan batas, pemakaian saat ini, dan kunci-kuncinya
// (terbitkan, rotasi, cabut). Setiap tindakan tercatat di Log Audit; kunci lengkap hanya muncul sekali pada layar penerbitan.
export function KlienDetail({
  id,
  dataset,
  bawaan,
  onTutup,
  onBerubah,
}: {
  id: number;
  dataset: InfoDataset[];
  bawaan: Pengaturan | null;
  onTutup: () => void;
  onBerubah: () => void;
}) {
  const toast = useToast();
  const [klien, setKlien] = useState<Klien | null>(null);
  const [pemakaian, setPemakaian] = useState<PemakaianKlien | null>(null);
  const [galat, setGalat] = useState("");
  const [mode, setMode] = useState<Mode>("lihat");
  const [form, setForm] = useState<NilaiFormAkses | null>(null);
  const [galatForm, setGalatForm] = useState<Record<string, string>>({});
  const [teks, setTeks] = useState("");
  const [labelKunci, setLabelKunci] = useState("");
  const [rotasi, setRotasi] = useState(false);
  const [sibuk, setSibuk] = useState(false);
  const [kunciBaru, setKunciBaru] = useState<{ kunci: KunciBaru; judul: string; dataset?: string } | null>(null);
  const [cabutKunci, setCabutKunci] = useState<KunciInfo | null>(null);
  const [aktifkan, setAktifkan] = useState(false);

  const muat = useCallback(async () => {
    try {
      const r = await getDetailKlien(id);
      setKlien(r.klien);
      setPemakaian(r.pemakaian);
      setGalat("");
    } catch (err) {
      setGalat(pesanGalat(err, "Gagal memuat rincian klien"));
    }
  }, [id]);

  useEffect(() => {
    void muat();
  }, [muat]);

  const mulai = (m: Mode) => {
    if (!klien) return;
    setGalatForm({});
    setTeks("");
    setLabelKunci("");
    setRotasi(false);
    if (m === "setujui") setForm(nilaiAwalAkses(klien, true));
    if (m === "ubah") setForm(nilaiAwalAkses(klien, false));
    setMode(m);
  };

  const selesai = async (pesan: string) => {
    toast.success(pesan);
    setMode("lihat");
    await muat();
    onBerubah();
  };

  const jalankan = async (fn: () => Promise<void>, cadangan: string) => {
    setSibuk(true);
    try {
      await fn();
    } catch (err) {
      toast.error(pesanGalat(err, cadangan));
    } finally {
      setSibuk(false);
    }
  };

  const simpanCakupan = () => {
    if (!klien || !form) return;
    const { body, galat: g } = susunBodyAkses(form);
    setGalatForm(g);
    if (!body) return;
    void jalankan(async () => {
      if (mode === "setujui") {
        const r = await postSetujui(klien.id, { ...body, label_kunci: labelKunci.trim() });
        if (r.data.kunci_baru) setKunciBaru({ kunci: r.data.kunci_baru, judul: "Pendaftaran disetujui: kunci API pertama", dataset: body.dataset_diberikan[0] });
        else toast.warning(r.message);
        await selesai("Pendaftaran disetujui");
      } else {
        await putKlien(klien.id, body);
        await selesai("Scope dan batas klien disimpan");
      }
    }, "Gagal menyimpan");
  };

  const tolak = () => {
    if (!klien) return;
    if (!teks.trim()) {
      toast.warning("Isi alasan terlebih dahulu");
      return;
    }
    void jalankan(async () => {
      if (mode === "tolak") await postTolak(klien.id, teks.trim());
      else await postCabut(klien.id, teks.trim());
      await selesai(mode === "tolak" ? "Pendaftaran ditolak" : "Akses klien dicabut dan semua kuncinya dihentikan");
    }, "Gagal menyimpan keputusan");
  };

  const terbitkan = () => {
    if (!klien) return;
    void jalankan(async () => {
      const r = await postKunci(klien.id, labelKunci.trim(), rotasi);
      setKunciBaru({ kunci: r.data, judul: rotasi ? "Kunci hasil rotasi" : "Kunci API baru", dataset: klien.dataset_diberikan[0] });
      await selesai(rotasi ? "Kunci dirotasi" : "Kunci diterbitkan");
    }, "Gagal menerbitkan kunci");
  };

  const aktifkanKembali = () => {
    if (!klien) return;
    void jalankan(async () => {
      const r = await postAktifkan(klien.id, "");
      setAktifkan(false);
      if (r.data.kunci_baru) setKunciBaru({ kunci: r.data.kunci_baru, judul: "Klien diaktifkan kembali: kunci API baru", dataset: klien.dataset_diberikan[0] });
      await selesai("Klien diaktifkan kembali");
    }, "Gagal mengaktifkan kembali");
  };

  const cabutSatuKunci = () => {
    if (!klien || !cabutKunci) return;
    const k = cabutKunci;
    void jalankan(async () => {
      await deleteKunci(klien.id, k.id, "");
      setCabutKunci(null);
      await selesai("Kunci dicabut");
    }, "Gagal mencabut kunci");
  };

  // Selama modal kunci baru atau dialog konfirmasi terbuka, Escape tidak boleh menutup rincian di bawahnya: menutupnya ikut menghapus modal kunci, dan kunci yang baru terbit
  // tidak dapat ditampilkan lagi.
  const dialogDiAtas = kunciBaru !== null || cabutKunci !== null || aktifkan;
  const subjudul = klien ? `${klien.kode} · ${klien.instansi}` : undefined;
  const sekarang = new Date();

  return (
    <>
      <ModalShell title={klien?.nama_aplikasi ?? "Klien API"} subtitle={subjudul} onClose={dialogDiAtas ? () => undefined : onTutup} size="xl">
        <div className="space-y-5 p-4 sm:p-6">
          {galat && <Alert message={galat} />}
          {!klien && !galat && <SkeletonCards count={3} />}
          {klien && (
            <>
              <div className="flex flex-wrap items-center gap-2">
                <LencanaStatus status={klien.status} />
                <span className="text-xs text-slate-500">
                  Didaftarkan {waktuWIB(klien.didaftarkan_at, false)} WIB
                  {klien.diputuskan_at ? ` · diputuskan ${waktuWIB(klien.diputuskan_at, false)} oleh ${klien.diputuskan_oleh || "-"}` : ""}
                </span>
                {klien.status === "disetujui" && (
                  <Link href={`/dashboard/api-akses/monitoring?klien_id=${klien.id}`} className="ml-auto inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-blue-700 hover:bg-blue-50">
                    <Activity className="h-4 w-4" aria-hidden="true" />
                    Lihat pemantauan
                  </Link>
                )}
              </div>

              <DetailEntry
                fields={[
                  ["Nama aplikasi", klien.nama_aplikasi],
                  ["Instansi", klien.instansi],
                  ["Unit", klien.unit],
                  ["PIC", [klien.pic_nama, klien.pic_jabatan].filter(Boolean).join(" · ")],
                  ["Email PIC", klien.pic_email],
                  ["Telepon PIC", klien.pic_telepon],
                  ["Didaftarkan dari alamat IP", klien.didaftarkan_ip],
                  ["Catatan superadmin", klien.catatan],
                ]}
              />
              <section>
                <h3 className="mb-1.5 text-sm font-semibold text-slate-700">Tujuan penggunaan</h3>
                <p className="whitespace-pre-wrap break-words rounded-xl border border-slate-200 bg-slate-50/60 px-3.5 py-3 text-sm text-slate-700">{klien.tujuan}</p>
              </section>

              <section className="grid gap-4 md:grid-cols-2">
                {(klien.status === "menunggu" || klien.status === "ditolak") && (
                  <div>
                    <h3 className="mb-1.5 text-sm font-semibold text-slate-700">Dataset diminta</h3>
                    <ChipDataset kode={klien.dataset_diminta} dataset={dataset} />
                  </div>
                )}
                {klien.status !== "menunggu" && klien.status !== "ditolak" && (
                  <div>
                    <h3 className="mb-1.5 text-sm font-semibold text-slate-700">Dataset yang diizinkan (scope)</h3>
                    <ChipDataset kode={klien.dataset_diberikan} dataset={dataset} kosong="Tidak ada dataset" />
                  </div>
                )}
              </section>

              {(klien.status === "disetujui" || klien.status === "dicabut") && (
                <section className="space-y-3">
                  <h3 className="flex items-center gap-1.5 text-sm font-semibold text-slate-700">
                    <ShieldCheck className="h-4 w-4" aria-hidden="true" />
                    Akses dan batas
                  </h3>
                  <DetailEntry
                    fields={[
                      ["Allowlist IP", klien.ip_diizinkan.length ? gabungDaftar(klien.ip_diizinkan) : "Semua alamat"],
                      ["Dibatasi ke UE1", klien.ue1_diizinkan.length ? gabungDaftar(klien.ue1_diizinkan) : "Semua UE1"],
                      ["Batas yang berlaku", teksBatas(klien.batas_berlaku)],
                      ["Batas khusus klien", teksOverride(klien) || "Tidak ada (mengikuti bawaan global)"],
                    ]}
                  />
                  {klien.status === "disetujui" && pemakaian && (
                    <div className="grid gap-4 rounded-xl border border-slate-200 p-3.5 sm:grid-cols-2">
                      <div>
                        <p className="mb-1 text-xs font-medium text-slate-600">Kuota periode berjalan</p>
                        <BatangKuota terpakai={pemakaian.kuota_terpakai} total={pemakaian.batas.kuota_permintaan} persen={pemakaian.kuota_persen} />
                        <p className="mt-1 text-xs text-slate-500">Pulih {waktuWIB(pemakaian.kuota_reset, false)} WIB</p>
                      </div>
                      <div>
                        <p className="mb-1 text-xs font-medium text-slate-600">Sisa jatah batas laju sekarang</p>
                        <p className="text-lg font-bold tabular-nums text-slate-900">
                          {formatJumlah(pemakaian.limit_sisa)} <span className="text-sm font-normal text-slate-500">dari {formatJumlah(pemakaian.batas.limit_permintaan)} permintaan</span>
                        </p>
                      </div>
                    </div>
                  )}
                </section>
              )}

              {/* ---- tindakan */}
              {mode === "lihat" && (
                <div className="flex flex-wrap gap-2 border-t border-slate-100 pt-4">
                  {klien.status === "menunggu" && (
                    <>
                      <Button fullWidth={false} icon={<BadgeCheck className="h-4 w-4" aria-hidden="true" />} onClick={() => mulai("setujui")}>
                        Setujui…
                      </Button>
                      <Button fullWidth={false} variant="secondary" icon={<XCircle className="h-4 w-4" aria-hidden="true" />} onClick={() => mulai("tolak")}>
                        Tolak…
                      </Button>
                    </>
                  )}
                  {klien.status === "disetujui" && (
                    <>
                      <Button fullWidth={false} variant="secondary" icon={<Pencil className="h-4 w-4" aria-hidden="true" />} onClick={() => mulai("ubah")}>
                        Ubah scope dan batas
                      </Button>
                      <Button fullWidth={false} variant="secondary" icon={<PlusCircle className="h-4 w-4" aria-hidden="true" />} onClick={() => { mulai("kunci"); setRotasi(false); }}>
                        Terbitkan kunci
                      </Button>
                      <Button fullWidth={false} variant="secondary" icon={<RotateCw className="h-4 w-4" aria-hidden="true" />} onClick={() => { mulai("kunci"); setRotasi(true); }}>
                        Rotasi kunci
                      </Button>
                      <Button fullWidth={false} variant="danger" icon={<Ban className="h-4 w-4" aria-hidden="true" />} onClick={() => mulai("cabut")}>
                        Cabut akses…
                      </Button>
                    </>
                  )}
                  {klien.status === "dicabut" && (
                    <Button fullWidth={false} icon={<RefreshCw className="h-4 w-4" aria-hidden="true" />} onClick={() => setAktifkan(true)}>
                      Aktifkan kembali…
                    </Button>
                  )}
                </div>
              )}

              {(mode === "setujui" || mode === "ubah") && form && (
                <section className="space-y-4 rounded-2xl border border-blue-200 bg-blue-50/40 p-4">
                  <h3 className="text-sm font-semibold text-slate-800">{mode === "setujui" ? "Setujui pendaftaran" : "Ubah scope dan batas"}</h3>
                  <FormAkses nilai={form} onUbah={(p) => setForm({ ...form, ...p })} dataset={dataset} bawaan={bawaan} galat={galatForm} diminta={klien.dataset_diminta} />
                  {mode === "setujui" && (
                    <div>
                      <label htmlFor="kd-label" className="mb-1 block text-xs font-medium text-slate-700">
                        Label kunci pertama (opsional)
                      </label>
                      <input id="kd-label" value={labelKunci} maxLength={100} onChange={(e) => setLabelKunci(e.target.value)} placeholder="mis. produksi" className={areaCls} />
                      <p className="mt-1 text-xs text-slate-500">Kunci API pertama diterbitkan sekaligus dan ditampilkan sekali. Hak akses mengikuti pilihan di atas, bukan permintaan awal pemohon.</p>
                    </div>
                  )}
                  <div className="flex flex-wrap justify-end gap-2">
                    <Button fullWidth={false} variant="secondary" onClick={() => setMode("lihat")} disabled={sibuk}>
                      Batal
                    </Button>
                    <Button fullWidth={false} isLoading={sibuk} onClick={simpanCakupan}>
                      {mode === "setujui" ? "Setujui dan terbitkan kunci" : "Simpan"}
                    </Button>
                  </div>
                </section>
              )}

              {(mode === "tolak" || mode === "cabut") && (
                <section className="space-y-3 rounded-2xl border border-slate-200 bg-slate-50/60 p-4">
                  <h3 className="text-sm font-semibold text-slate-800">{mode === "tolak" ? "Tolak pendaftaran" : "Cabut akses klien"}</h3>
                  {mode === "cabut" && <p className="text-xs text-slate-600">Semua kunci klien ini langsung berhenti berlaku. Mengaktifkan kembali berarti menerbitkan kunci baru; kunci lama tidak hidup lagi.</p>}
                  <label htmlFor="kd-alasan" className="block text-xs font-medium text-slate-700">
                    Alasan (wajib, tercatat di Log Audit)
                  </label>
                  <textarea id="kd-alasan" rows={3} maxLength={1000} value={teks} onChange={(e) => setTeks(e.target.value)} className={areaCls} />
                  <div className="flex flex-wrap justify-end gap-2">
                    <Button fullWidth={false} variant="secondary" onClick={() => setMode("lihat")} disabled={sibuk}>
                      Batal
                    </Button>
                    <Button fullWidth={false} variant="danger" isLoading={sibuk} onClick={tolak}>
                      {mode === "tolak" ? "Tolak pendaftaran" : "Cabut akses"}
                    </Button>
                  </div>
                </section>
              )}

              {mode === "kunci" && (
                <section className="space-y-3 rounded-2xl border border-slate-200 bg-slate-50/60 p-4">
                  <h3 className="text-sm font-semibold text-slate-800">{rotasi ? "Rotasi kunci" : "Terbitkan kunci baru"}</h3>
                  <p className="text-xs text-slate-600">
                    {rotasi
                      ? `Kunci baru diterbitkan; kunci aktif lainnya tetap berlaku selama masa tenggang${bawaan ? ` (${bawaan.rotasi_tenggang_jam} jam)` : ""}, lalu berhenti, supaya klien sempat berganti tanpa putus layanan. Cabut kunci lama sekarang bila diduga bocor.`
                      : "Satu klien boleh memegang beberapa kunci aktif (maksimal 5), mis. untuk lingkungan uji dan produksi."}
                  </p>
                  <div>
                    <label htmlFor="kd-label2" className="mb-1 block text-xs font-medium text-slate-700">
                      Label (opsional)
                    </label>
                    <input id="kd-label2" value={labelKunci} maxLength={100} onChange={(e) => setLabelKunci(e.target.value)} placeholder="mis. produksi, uji" className={areaCls} />
                  </div>
                  <div className="flex flex-wrap justify-end gap-2">
                    <Button fullWidth={false} variant="secondary" onClick={() => setMode("lihat")} disabled={sibuk}>
                      Batal
                    </Button>
                    <Button fullWidth={false} isLoading={sibuk} onClick={terbitkan}>
                      {rotasi ? "Rotasi dan terbitkan" : "Terbitkan kunci"}
                    </Button>
                  </div>
                </section>
              )}

              {/* ---- kunci */}
              {klien.status !== "menunggu" && klien.status !== "ditolak" && klien.kunci && (
                <section>
                  <h3 className="mb-2 flex items-center gap-1.5 text-sm font-semibold text-slate-700">
                    <KeyRound className="h-4 w-4" aria-hidden="true" />
                    Kunci API ({klien.kunci_aktif} aktif)
                  </h3>
                  {klien.kunci.length === 0 ? (
                    <p className="rounded-xl border border-dashed border-slate-300 bg-slate-50/60 px-4 py-6 text-center text-sm text-slate-500">Belum ada kunci.</p>
                  ) : (
                    <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white">
                      <table className="w-full min-w-[40rem] text-sm">
                        <thead className="bg-slate-50 text-xs text-slate-500">
                          <tr>
                            <th scope="col" className="px-3 py-2 text-left font-medium">Kunci</th>
                            <th scope="col" className="px-3 py-2 text-left font-medium">Dibuat</th>
                            <th scope="col" className="px-3 py-2 text-left font-medium">Kedaluwarsa</th>
                            <th scope="col" className="px-3 py-2 text-left font-medium">Terakhir dipakai</th>
                            <th scope="col" className="px-3 py-2 text-left font-medium">Status</th>
                            <th scope="col" className="relative px-3 py-2"><span className="sr-only">Tindakan</span></th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-100">
                          {klien.kunci.map((k) => {
                            const st = statusKunci(k);
                            return (
                              <tr key={k.id}>
                                <td className="px-3 py-2">
                                  <p className="font-mono text-xs text-slate-800">pasti_{k.prefix}_…</p>
                                  {k.label && <p className="text-xs text-slate-500">{k.label}</p>}
                                </td>
                                <td className="whitespace-nowrap px-3 py-2 text-xs text-slate-600">
                                  {waktuWIB(k.dibuat_at, false)}
                                  {k.dibuat_oleh && <span className="block text-slate-400">oleh {k.dibuat_oleh}</span>}
                                </td>
                                <td className="whitespace-nowrap px-3 py-2 text-xs text-slate-600">{k.kedaluwarsa_at ? waktuWIB(k.kedaluwarsa_at, false) : "Tidak kedaluwarsa"}</td>
                                <td className="whitespace-nowrap px-3 py-2 text-xs text-slate-600">
                                  {k.terakhir_dipakai_at ? teksRelatif(k.terakhir_dipakai_at, sekarang) : "Belum pernah"}
                                  {k.terakhir_ip && <span className="block font-mono text-slate-400">{k.terakhir_ip}</span>}
                                </td>
                                <td className="px-3 py-2">
                                  <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-semibold ring-1 ring-inset ${st.kelas}`}>{st.teks}</span>
                                  {k.dicabut_at && k.alasan_cabut && <span className="mt-0.5 block text-[11px] text-slate-400">{k.alasan_cabut}</span>}
                                </td>
                                <td className="px-3 py-2 text-right">
                                  {k.aktif && klien.status === "disetujui" && (
                                    <button type="button" onClick={() => setCabutKunci(k)} className="rounded-lg px-2.5 py-1.5 text-xs font-medium text-red-700 hover:bg-red-50">
                                      Cabut
                                    </button>
                                  )}
                                </td>
                              </tr>
                            );
                          })}
                        </tbody>
                      </table>
                    </div>
                  )}
                </section>
              )}
            </>
          )}
        </div>
      </ModalShell>

      {kunciBaru && <KunciBaruModal kunci={kunciBaru.kunci.kunci} judul={kunciBaru.judul} datasetContoh={kunciBaru.dataset} onTutup={() => setKunciBaru(null)} />}
      {cabutKunci && (
        <ConfirmDialog
          title="Cabut kunci ini?"
          message={
            <>
              Kunci <span className="font-mono">pasti_{cabutKunci.prefix}_…</span> langsung berhenti berlaku dan aplikasi yang memakainya akan menerima 401. Tindakan ini tidak dapat dibatalkan.
            </>
          }
          confirmLabel="Ya, cabut kunci"
          busy={sibuk}
          onConfirm={cabutSatuKunci}
          onCancel={() => setCabutKunci(null)}
        />
      )}
      {aktifkan && (
        <ConfirmDialog
          title="Aktifkan kembali klien ini?"
          tone="warning"
          message="Klien disetujui kembali dengan scope dan batas yang tersimpan. Kunci lama tidak hidup lagi; kunci baru diterbitkan dan ditampilkan sekali."
          confirmLabel="Aktifkan dan terbitkan kunci"
          busy={sibuk}
          onConfirm={aktifkanKembali}
          onCancel={() => setAktifkan(false)}
        />
      )}
    </>
  );
}

function ChipDataset({ kode, dataset, kosong = "-" }: { kode: string[]; dataset: InfoDataset[]; kosong?: string }) {
  if (kode.length === 0) return <p className="text-sm text-slate-500">{kosong}</p>;
  return (
    <ul className="flex flex-wrap gap-1.5">
      {kode.map((k) => (
        <li key={k} className="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-700" title={k}>
          {dataset.find((d) => d.kode === k)?.label ?? k}
        </li>
      ))}
    </ul>
  );
}
