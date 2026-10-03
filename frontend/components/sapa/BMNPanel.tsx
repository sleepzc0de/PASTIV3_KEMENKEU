"use client";

import { useCallback, useEffect, useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { SapaJenisBMN, SapaRefBMN, SapaSatuanBMN, deleteSapaJenisBMN, deleteSapaSatuanBMN, listSapaBMN, saveSapaJenisBMN, saveSapaSatuanBMN } from "@/lib/sapa";
import { Alert } from "@/components/ui/Alert";
import { CheckField, ErrorBox, NoticeBox, PrimaryButton, SecondaryButton, SelectInput, TextField } from "./fields";
import { ErrorInfo, errorInfo, susunSatuanJenis, urutanBerikut } from "./sapa";

// Pengaturan jenis BMN dan satuan jumlahnya. Tiap jenis hanya boleh memakai satuan yang dicentang padanya, sehingga pengguna
// tidak bisa memilih pasangan yang tidak masuk akal di formulir Nota Dinas (mis. Tanah dalam "unit").
export function BMNPanel() {
  const [data, setData] = useState<SapaRefBMN | null>(null);
  const [error, setError] = useState<string | null>(null);

  const muat = useCallback(async () => {
    try {
      const res = await listSapaBMN();
      setData({ jenis: (res.data.jenis ?? []).map((j) => ({ ...j, satuan: j.satuan ?? [] })), satuan: res.data.satuan ?? [] });
      setError(null);
    } catch (err) {
      setError(errorInfo(err, "Gagal memuat daftar jenis BMN").message);
    }
  }, []);

  useEffect(() => {
    void muat();
  }, [muat]);

  if (error) return <Alert message={error} />;
  if (!data) return <div role="status" aria-label="Memuat daftar BMN" className="h-40 animate-pulse rounded-xl bg-slate-100" />;

  return (
    <div className="space-y-6">
      <NoticeBox>
        Pengguna memilih <b>satu jenis BMN</b> pada Nota Dinas usulan Satker, lalu satuan jumlahnya hanya bisa dipilih dari satuan yang dicentang untuk jenis itu. Contoh: Tanah
        hanya &ldquo;bidang&rdquo;, Kendaraan Bermotor hanya &ldquo;unit&rdquo;. Jenis atau satuan yang dinonaktifkan tidak muncul lagi di formulir.
      </NoticeBox>
      <PanelSatuan data={data} onChanged={muat} />
      <PanelJenis data={data} onChanged={muat} />
    </div>
  );
}

// ---------------------------------------------------------------- satuan

function PanelSatuan({ data, onChanged }: { data: SapaRefBMN; onChanged: () => Promise<void> }) {
  const [nama, setNama] = useState("");
  const [busy, setBusy] = useState(false);
  const [galat, setGalat] = useState<ErrorInfo | null>(null);
  const [hapus, setHapus] = useState<string | null>(null);

  const jalankan = async (aksi: () => Promise<unknown>, gagal: string) => {
    setBusy(true);
    setGalat(null);
    try {
      await aksi();
      await onChanged();
      return true;
    } catch (err) {
      setGalat(errorInfo(err, gagal));
      return false;
    } finally {
      setBusy(false);
    }
  };

  const tambah = async () => {
    const n = nama.trim();
    if (data.satuan.some((s) => s.nama.toLowerCase() === n.toLowerCase())) {
      setGalat({ message: `Satuan "${n}" sudah ada`, errors: [] });
      return;
    }
    if (await jalankan(() => saveSapaSatuanBMN({ nama: n, aktif: true, urutan: urutanBerikut(data.satuan) }), "Gagal menambah satuan")) setNama("");
  };

  const pemakai = (s: SapaSatuanBMN) => data.jenis.filter((j) => j.satuan.some((x) => x.toLowerCase() === s.nama.toLowerCase())).length;

  return (
    <section className="space-y-3 rounded-xl border border-slate-200 bg-white p-4">
      <div>
        <h3 className="text-sm font-semibold text-slate-900">Satuan jumlah BMN</h3>
        <p className="mt-0.5 text-xs text-slate-500">Daftar satuan yang tersedia. Satuan baru dipasangkan ke jenis BMN di bagian bawah.</p>
      </div>

      <form
        className="flex flex-wrap items-end gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (nama.trim()) void tambah();
        }}
      >
        <TextField label="Satuan baru" value={nama} onChange={setNama} maxLength={30} placeholder="mis. meter persegi" className="w-full sm:w-64" />
        <PrimaryButton type="submit" busy={busy} disabled={!nama.trim()}>
          <Plus className="h-4 w-4" aria-hidden="true" />
          Tambah satuan
        </PrimaryButton>
      </form>
      <ErrorBox error={galat} />

      {data.satuan.length === 0 ? (
        <p className="rounded-lg border border-dashed border-slate-300 px-3 py-5 text-center text-sm text-slate-500">Belum ada satuan.</p>
      ) : (
        <ul className="divide-y divide-slate-100 rounded-lg border border-slate-200">
          {data.satuan.map((s) => {
            const n = pemakai(s);
            return (
              <li key={s.nama} className="flex flex-wrap items-center justify-between gap-3 px-3 py-2.5">
                <div className="min-w-0">
                  <p className={`text-sm font-medium ${s.aktif ? "text-slate-900" : "text-slate-400 line-through"}`}>{s.nama}</p>
                  <p className="text-xs text-slate-500">{n === 0 ? "Belum dipakai jenis mana pun" : `Dipakai ${n} jenis BMN`}</p>
                </div>
                {hapus === s.nama ? (
                  <div className="flex items-center gap-2 text-sm">
                    <span className="text-slate-600">Hapus &ldquo;{s.nama}&rdquo;?</span>
                    <SecondaryButton onClick={() => setHapus(null)}>Batal</SecondaryButton>
                    <PrimaryButton
                      onClick={async () => {
                        if (await jalankan(() => deleteSapaSatuanBMN(s.nama), "Gagal menghapus satuan")) setHapus(null);
                      }}
                      busy={busy}
                      className="!bg-red-600 hover:!bg-red-700"
                    >
                      Hapus
                    </PrimaryButton>
                  </div>
                ) : (
                  <div className="flex items-center gap-3">
                    <CheckField
                      label="Aktif"
                      checked={s.aktif}
                      disabled={busy}
                      onChange={(aktif) => void jalankan(() => saveSapaSatuanBMN({ ...s, aktif }), "Gagal mengubah satuan")}
                    />
                    <button
                      type="button"
                      onClick={() => {
                        setGalat(null);
                        setHapus(s.nama);
                      }}
                      aria-label={`Hapus satuan ${s.nama}`}
                      className="rounded p-2 text-slate-400 hover:bg-red-50 hover:text-red-600"
                    >
                      <Trash2 className="h-4 w-4" aria-hidden="true" />
                    </button>
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

// ---------------------------------------------------------------- jenis

function PanelJenis({ data, onChanged }: { data: SapaRefBMN; onChanged: () => Promise<void> }) {
  return (
    <section className="space-y-3">
      <div>
        <h3 className="text-sm font-semibold text-slate-900">Jenis BMN dan satuan yang diizinkan</h3>
        <p className="mt-0.5 text-xs text-slate-500">Centang satuan yang masuk akal untuk tiap jenis, lalu pilih satuan bawaan yang terisi otomatis saat jenis dipilih.</p>
      </div>
      {data.jenis.map((j) => (
        <KartuJenis key={j.nama} jenis={j} semuaSatuan={data.satuan} urutan={j.urutan} onChanged={onChanged} />
      ))}
      <KartuJenis key={`baru-${data.jenis.length}`} jenis={null} semuaSatuan={data.satuan} urutan={urutanBerikut(data.jenis)} onChanged={onChanged} />
    </section>
  );
}

interface Draf {
  nama: string;
  aktif: boolean;
  satuan: string[];
  bawaan: string;
}

const dariJenis = (j: SapaJenisBMN | null): Draf => (j ? { nama: j.nama, aktif: j.aktif, satuan: j.satuan, bawaan: j.satuan_bawaan } : { nama: "", aktif: true, satuan: [], bawaan: "" });

function KartuJenis({
  jenis,
  semuaSatuan,
  urutan,
  onChanged,
}: {
  jenis: SapaJenisBMN | null; // null: formulir jenis baru
  semuaSatuan: SapaSatuanBMN[];
  urutan: number;
  onChanged: () => Promise<void>;
}) {
  const baru = jenis === null;
  const [draf, setDraf] = useState<Draf>(() => dariJenis(jenis));
  const [busy, setBusy] = useState(false);
  const [galat, setGalat] = useState<ErrorInfo | null>(null);
  const [hapus, setHapus] = useState(false);

  // Data dimuat ulang setelah menyimpan: draf mengikuti data terbaru, tetapi hanya bila isi jenis ini yang berubah (menyimpan satu
  // kartu tidak boleh membuang suntingan yang belum disimpan di kartu lain).
  const sidik = JSON.stringify(jenis);
  useEffect(() => {
    setDraf(dariJenis(jenis));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sidik]);

  const asal = dariJenis(jenis);
  const susun = susunSatuanJenis(semuaSatuan, draf.satuan, draf.bawaan);
  const berubah = baru ? draf.nama.trim() !== "" : draf.aktif !== asal.aktif || susun.bawaan !== asal.bawaan || susun.satuan.join("\n") !== asal.satuan.join("\n");
  const siap = draf.nama.trim() !== "" && susun.satuan.length > 0 && berubah;

  const centang = (nama: string, on: boolean) =>
    setDraf((d) => ({ ...d, satuan: on ? [...d.satuan, nama] : d.satuan.filter((x) => x.toLowerCase() !== nama.toLowerCase()) }));

  const simpan = async () => {
    setBusy(true);
    setGalat(null);
    try {
      await saveSapaJenisBMN({ nama: draf.nama.trim(), aktif: draf.aktif, urutan: jenis?.urutan ?? urutan, satuan: susun.satuan, satuan_bawaan: susun.bawaan });
      if (baru) setDraf(dariJenis(null));
      await onChanged();
    } catch (err) {
      setGalat(errorInfo(err, "Gagal menyimpan jenis BMN"));
    } finally {
      setBusy(false);
    }
  };

  const hapusJenis = async () => {
    setBusy(true);
    setGalat(null);
    try {
      await deleteSapaJenisBMN(asal.nama);
      await onChanged();
    } catch (err) {
      setGalat(errorInfo(err, "Gagal menghapus jenis BMN"));
      setHapus(false);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className={`space-y-3 rounded-xl border p-4 ${baru ? "border-dashed border-slate-300 bg-slate-50/60" : "border-slate-200 bg-white"}`}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        {baru ? (
          <TextField label="Tambah jenis BMN" value={draf.nama} onChange={(nama) => setDraf((d) => ({ ...d, nama }))} maxLength={100} placeholder="mis. Persediaan" className="w-full sm:w-80" />
        ) : (
          <div className="min-w-0">
            <h4 className={`text-sm font-semibold ${draf.aktif ? "text-slate-900" : "text-slate-400"}`}>{asal.nama}</h4>
            {!asal.aktif && <span className="mt-1 inline-block rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-500">Nonaktif: tidak muncul di formulir</span>}
          </div>
        )}
        {!baru &&
          (hapus ? (
            <div className="flex items-center gap-2 text-sm">
              <span className="text-slate-600">Hapus jenis ini?</span>
              <SecondaryButton onClick={() => setHapus(false)}>Batal</SecondaryButton>
              <PrimaryButton onClick={hapusJenis} busy={busy} className="!bg-red-600 hover:!bg-red-700">
                Hapus
              </PrimaryButton>
            </div>
          ) : (
            <button
              type="button"
              onClick={() => {
                setGalat(null);
                setHapus(true);
              }}
              aria-label={`Hapus jenis ${asal.nama}`}
              className="rounded p-2 text-slate-400 hover:bg-red-50 hover:text-red-600"
            >
              <Trash2 className="h-4 w-4" aria-hidden="true" />
            </button>
          ))}
      </div>

      <fieldset>
        <legend className="mb-1.5 text-xs font-medium text-slate-600">Satuan yang diizinkan</legend>
        {semuaSatuan.length === 0 ? (
          <p className="text-xs text-slate-500">Tambahkan satuan lebih dulu di bagian atas.</p>
        ) : (
          <div className="grid gap-x-4 gap-y-2 sm:grid-cols-2 lg:grid-cols-3">
            {semuaSatuan.map((s) => (
              <CheckField
                key={s.nama}
                label={s.aktif ? s.nama : `${s.nama} (nonaktif)`}
                checked={susun.satuan.some((x) => x.toLowerCase() === s.nama.toLowerCase())}
                onChange={(on) => centang(s.nama, on)}
                disabled={busy}
              />
            ))}
          </div>
        )}
      </fieldset>

      <div className="grid gap-3 sm:grid-cols-2">
        <SelectInput
          label="Satuan bawaan"
          value={susun.bawaan}
          onChange={(bawaan) => setDraf((d) => ({ ...d, bawaan }))}
          options={susun.satuan}
          disabled={busy || susun.satuan.length === 0}
          placeholder={susun.satuan.length === 0 ? "Centang satuan dulu" : "Pilih…"}
          hint="Terpilih otomatis saat pengguna memilih jenis ini."
        />
        {!baru && (
          <div className="self-end pb-1">
            <CheckField label="Aktif (tampil di formulir)" checked={draf.aktif} onChange={(aktif) => setDraf((d) => ({ ...d, aktif }))} disabled={busy} />
          </div>
        )}
      </div>

      <ErrorBox error={galat} />
      <div className="flex justify-end gap-2">
        {!baru && berubah && (
          <SecondaryButton onClick={() => setDraf(dariJenis(jenis))} disabled={busy}>
            Batalkan perubahan
          </SecondaryButton>
        )}
        <PrimaryButton onClick={simpan} busy={busy} disabled={!siap}>
          {baru ? (
            <>
              <Plus className="h-4 w-4" aria-hidden="true" />
              Tambah jenis
            </>
          ) : (
            "Simpan perubahan"
          )}
        </PrimaryButton>
      </div>
    </div>
  );
}
