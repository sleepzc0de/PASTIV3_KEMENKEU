"use client";

import { useCallback, useEffect, useId, useRef, useState } from "react";
import Link from "next/link";
import { ArrowLeft, ChevronDown, Lock, RotateCcw, SkipForward, Hourglass } from "lucide-react";
import { SapaDetail, SapaSaya, SapaTahapDetail, SapaUsulan, getSapaPenjualan, reopenSapaTahap, skipSapaTahap } from "@/lib/sapa";
import { Alert } from "@/components/ui/Alert";
import { Segmented } from "../digitalisasi/controls";
import { formatDateTime } from "@/lib/dasbor";
import { AlurRingkas } from "./AlurRingkas";
import { KanalBadge, PeranBadge, StatusBadge, StepCircle } from "./badges";
import { DokumenList } from "./DokumenList";
import { EksternalPanel } from "./EksternalPanel";
import { ErrorBox, NoticeBox, PrimaryButton, SecondaryButton, TextAreaField } from "./fields";
import { NDSatkerForm, NDUE1Form } from "./NDForms";
import { useSapa } from "./SapaGate";
import { ErrorInfo, Lihat, errorInfo, errorStatus, formatTanggal, giliranSaatIni, lihatAwal, peranLabel, tahapDilihat } from "./sapa";
import { BAForm, TimForm } from "./TimForm";

// Halaman satu usulan penjualan: linimasa 10 tahap. Tampilan mengikuti peran: pengguna Satker/Kanwil/UE1 langsung melihat
// formulir tahap miliknya (ringkasan alur tetap menunjukkan posisi usulan di seluruh alur), sedangkan admin melihat semua
// tahap dan bisa menyaring per peran. Tahap yang sedang berjalan terbuka; tiap tahap menampilkan formulir, panel pencatatan
// (tahap di aplikasi lain), atau alasan mengapa belum bisa dikerjakan.
export function PenjualanDetail({ id }: { id: string }) {
  const saya = useSapa();
  const [lihat, setLihat] = useState<Lihat>(() => lihatAwal(saya.admin));
  const [detail, setDetail] = useState<SapaDetail | null>(null);
  const [error, setError] = useState<{ message: string; hilang: boolean } | null>(null);
  const aktif = useRef(true);

  const muat = useCallback(async () => {
    try {
      const res = await getSapaPenjualan(id);
      if (aktif.current) {
        setDetail(res.data);
        setError(null);
      }
    } catch (err) {
      if (aktif.current) {
        const st = errorStatus(err);
        setError({ message: st === 404 ? "Usulan tidak ditemukan atau Anda tidak memiliki akses ke usulan ini." : errorInfo(err, "Gagal memuat usulan").message, hilang: st === 404 });
      }
    }
  }, [id]);

  useEffect(() => {
    aktif.current = true;
    muat();
    return () => {
      aktif.current = false;
    };
  }, [muat]);

  const kembali = (
    <Link href="/dashboard/sapa/penjualan" className="inline-flex items-center gap-1.5 text-sm font-medium text-blue-600 hover:text-blue-700">
      <ArrowLeft className="h-4 w-4" aria-hidden="true" />
      Daftar usulan
    </Link>
  );

  if (error && !detail) {
    return (
      <div className="space-y-3">
        {kembali}
        <Alert message={error.message} />
        {!error.hilang && (
          <button type="button" onClick={muat} className="text-sm font-medium text-blue-600 hover:text-blue-700">
            Coba lagi
          </button>
        )}
      </div>
    );
  }
  if (!detail) return <div role="status" aria-label="Memuat usulan" className="h-64 animate-pulse rounded-xl bg-slate-100" />;

  const selesai = detail.tahap.filter((t) => t.status === "selesai" || t.status === "dilewati").length;
  const tampil = tahapDilihat(detail.tahap, lihat, saya.peran);
  const giliran = giliranSaatIni(detail.tahap, detail.tahap_saat_ini, saya.peran, saya.admin);

  // Dari ringkasan alur: tahap milik peran lain dibuka dengan beralih ke tampilan semua tahap, lalu digulir ke kartunya.
  const pilihTahap = (t: SapaTahapDetail) => {
    if (!tampil.some((x) => x.kunci === t.kunci)) setLihat("semua");
    setTimeout(() => document.getElementById(`kartu-${t.kunci}`)?.scrollIntoView({ behavior: "smooth", block: "start" }), 60);
  };

  return (
    <div className="space-y-5">
      {kembali}
      <Ringkasan usulan={detail.usulan} selesai={selesai} total={detail.tahap.length} sudahSelesai={detail.selesai} />
      {error && <Alert message={error.message} />}

      <AlurRingkas tahap={detail.tahap} aktifKunci={detail.tahap_saat_ini} peranSaya={saya.admin ? "" : saya.peran} onPilih={pilihTahap} />

      <GiliranBanner
        giliran={giliran}
        admin={saya.admin}
        onLihatSemua={() => setLihat("semua")}
        lihat={lihat}
        tahapSayaTuntas={detail.tahap.filter((t) => t.peran === saya.peran).every((t) => t.status === "selesai" || t.status === "dilewati")}
      />

      <div className="flex flex-wrap items-center justify-between gap-3">
        <PilihTampilan lihat={lihat} onChange={setLihat} saya={saya} />
        <p className="text-xs text-slate-500">
          Menampilkan {tampil.length} dari {detail.tahap.length} tahap
        </p>
      </div>

      <ol className="space-y-3">
        {tampil.map((t, i) => (
          <TahapCard
            key={t.kunci}
            t={t}
            usulan={detail.usulan}
            saya={saya}
            aktif={t.kunci === detail.tahap_saat_ini}
            terakhir={i === tampil.length - 1}
            onChanged={muat}
          />
        ))}
      </ol>
    </div>
  );
}

// Pemilih sudut pandang. Pengguna biasa: tahap peran sendiri atau semua; admin: semua atau per peran.
function PilihTampilan({ lihat, onChange, saya }: { lihat: Lihat; onChange: (l: Lihat) => void; saya: SapaSaya }) {
  if (saya.admin) {
    return (
      <Segmented<Lihat>
        label="Tahap yang ditampilkan"
        value={lihat}
        onChange={onChange}
        options={[
          { value: "semua", label: "Semua tahap" },
          { value: "satker", label: "Satuan Kerja" },
          { value: "kanwil", label: "Kantor Wilayah" },
          { value: "ue1", label: "Unit Eselon I" },
        ]}
      />
    );
  }
  return (
    <Segmented<Lihat>
      label="Tahap yang ditampilkan"
      value={lihat}
      onChange={onChange}
      options={[
        { value: "saya", label: `Tahap ${peranLabel(saya.peran)}` },
        { value: "semua", label: "Semua tahap" },
      ]}
    />
  );
}

// Pesan singkat tentang siapa yang sedang ditunggu, supaya pengguna tahu apa yang harus dikerjakan tanpa membaca seluruh alur.
function GiliranBanner({
  giliran,
  admin,
  lihat,
  onLihatSemua,
  tahapSayaTuntas,
}: {
  giliran: ReturnType<typeof giliranSaatIni>;
  admin: boolean;
  lihat: Lihat;
  onLihatSemua: () => void;
  tahapSayaTuntas: boolean; // semua tahap milik peran pengguna sudah selesai/dilewati
}) {
  if (giliran.jenis === "selesai") {
    return <NoticeBox tone="ok">Seluruh tahap usulan ini sudah selesai.</NoticeBox>;
  }
  if (giliran.jenis === "saya") {
    return (
      <NoticeBox tone="info">
        {admin ? "Tahap berjalan: " : "Giliran Anda: "}
        <span className="font-semibold">{giliran.label}</span>
        {admin && <span className="text-xs"> (dikerjakan oleh {peranLabel(giliran.peran ?? "")})</span>}
      </NoticeBox>
    );
  }
  return (
    <NoticeBox tone="warn">
      <span>
        {tahapSayaTuntas ? "Tahap Anda sudah selesai. " : ""}Saat ini giliran <span className="font-semibold">{peranLabel(giliran.peran ?? "")}</span>: {giliran.label}.
        {tahapSayaTuntas ? "" : " Tahap Anda terbuka setelah tahap-tahap sebelumnya selesai."}
      </span>
      {lihat !== "semua" && (
        <button type="button" onClick={onLihatSemua} className="ml-2 font-medium underline underline-offset-2 hover:no-underline">
          Lihat semua tahap
        </button>
      )}
    </NoticeBox>
  );
}

function Ringkasan({ usulan, selesai, total, sudahSelesai }: { usulan: SapaUsulan; selesai: number; total: number; sudahSelesai: boolean }) {
  const persen = Math.round((selesai / total) * 100);
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50/60 p-4 sm:p-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs font-medium uppercase tracking-wide text-slate-500">Noreg usulan penjualan</p>
          <h2 className="break-words text-xl font-bold text-slate-900">{usulan.noreg}</h2>
          <p className="mt-1 break-words text-sm text-slate-700">{usulan.nama_satker}</p>
        </div>
        {sudahSelesai && (
          <span className="rounded-full bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700 ring-1 ring-inset ring-emerald-200">Seluruh tahap selesai</span>
        )}
      </div>
      <dl className="mt-3 grid gap-x-6 gap-y-1 text-xs text-slate-500 sm:grid-cols-3">
        <div>
          <dt className="inline">Kode satker: </dt>
          <dd className="inline font-mono text-slate-700">{usulan.kode_satker}</dd>
        </div>
        <div>
          <dt className="inline">Kode UE1: </dt>
          <dd className="inline font-mono text-slate-700">{usulan.kode_ue1}</dd>
        </div>
        <div>
          <dt className="inline">Dibuat: </dt>
          <dd className="inline text-slate-700">
            {usulan.dibuat_oleh || "-"}, {formatDateTime(usulan.dibuat_pada)}
          </dd>
        </div>
      </dl>
      <div className="mt-4">
        <div className="mb-1 flex justify-between text-xs text-slate-500">
          <span>Kemajuan</span>
          <span>
            {selesai} dari {total} tahap
          </span>
        </div>
        <div
          role="progressbar"
          aria-valuemin={0}
          aria-valuemax={total}
          aria-valuenow={selesai}
          aria-label="Kemajuan tahap"
          className="h-2 overflow-hidden rounded-full bg-slate-200"
        >
          <div className="h-full rounded-full bg-emerald-500 transition-all" style={{ width: `${persen}%` }} />
        </div>
      </div>
    </div>
  );
}

function TahapCard({ t, usulan, saya, aktif, terakhir, onChanged }: { t: SapaTahapDetail; usulan: SapaUsulan; saya: SapaSaya; aktif: boolean; terakhir: boolean; onChanged: () => void }) {
  const [open, setOpen] = useState(aktif);
  // Tahap yang menjadi tahap berjalan (mis. setelah tahap sebelumnya selesai) otomatis terbuka.
  useEffect(() => {
    if (aktif) setOpen(true);
  }, [aktif]);

  const panelId = `tahap-${t.kunci}`;
  return (
    <li id={`kartu-${t.kunci}`} className="relative scroll-mt-20">
      {!terakhir && <span aria-hidden="true" className="absolute left-4 top-9 -bottom-3 w-px bg-slate-200" />}
      <div className={`rounded-xl border bg-white ${aktif ? "border-blue-300 shadow-sm" : "border-slate-200"}`}>
        <button
          type="button"
          onClick={() => setOpen((o) => !o)}
          aria-expanded={open}
          aria-controls={panelId}
          className="flex w-full items-start gap-3 rounded-xl p-3 text-left hover:bg-slate-50 sm:p-4"
        >
          <StepCircle no={t.urutan} status={t.status} aktif={aktif} />
          <div className="min-w-0 flex-1">
            <p className="text-sm font-semibold text-slate-900">{t.label}</p>
            <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
              <PeranBadge peran={t.peran} />
              {t.kanal && <KanalBadge kanal={t.kanal} />}
              <StatusBadge status={t.status} />
              {aktif && t.boleh_aksi && t.status !== "selesai" && (
                <span className="rounded-full bg-blue-600 px-2 py-0.5 text-[11px] font-semibold text-white">Menunggu tindakan Anda</span>
              )}
            </div>
          </div>
          <ChevronDown className={`mt-1 h-5 w-5 shrink-0 text-slate-400 transition-transform ${open ? "rotate-180" : ""}`} aria-hidden="true" />
        </button>
        {open && (
          <div id={panelId} className="space-y-4 border-t border-slate-100 p-3 sm:p-4">
            <TahapBody t={t} usulan={usulan} saya={saya} onChanged={onChanged} />
          </div>
        )}
      </div>
    </li>
  );
}

function TahapBody({ t, usulan, saya, onChanged }: { t: SapaTahapDetail; usulan: SapaUsulan; saya: SapaSaya; onChanged: () => void }) {
  const [mengubah, setMengubah] = useState(false);
  // Tahap yang boleh dikerjakan di luar aplikasi (SK Tim, Berita Acara): kotak centang menggantikan formulir dengan isian
  // keterangan dokumen. Formulir tetap terpasang (hanya disembunyikan) supaya isiannya tidak hilang bila centang dibatalkan.
  const [luar, setLuar] = useState(false);
  const tuntas = t.status === "selesai" || t.status === "dilewati";
  const bisaLewati = t.boleh_dilewati && !tuntas;
  // Boleh dikerjakan: peran sesuai, tahap sebelumnya sudah beres, dan (bila sudah selesai) belum ada tahap sesudahnya yang selesai.
  const bisaMengubah = t.boleh_aksi && t.dapat_dikerjakan && (!tuntas || t.dapat_diubah);
  const tampilkanPanel = bisaMengubah && (!tuntas || mengubah);

  return (
    <>
      <p className="text-sm text-slate-600">{t.keterangan}</p>

      {t.status === "dilewati" && (
        <NoticeBox>
          <p className="font-medium">Dilewati: dikerjakan di luar aplikasi</p>
          {t.catatan && <p className="mt-0.5 whitespace-pre-line break-words text-xs">{t.catatan}</p>}
        </NoticeBox>
      )}
      {t.jenis === "eksternal" && t.status === "selesai" && (
        <dl className="grid gap-x-6 gap-y-1 rounded-lg bg-emerald-50/60 p-3 text-sm sm:grid-cols-3">
          <Ringkas label="Nomor" nilai={t.nomor || "-"} />
          <Ringkas label="Tanggal" nilai={formatTanggal(t.tanggal)} />
          <Ringkas label="Dicatat oleh" nilai={t.diperbarui_oleh || "-"} />
          {t.catatan && <Ringkas label="Catatan" nilai={t.catatan} lebar />}
        </dl>
      )}

      <DokumenList dokumen={t.dokumen} />

      {!t.dapat_dikerjakan && (
        <NoticeBox>
          <span className="inline-flex items-start gap-1.5">
            <Hourglass className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <span>Belum dapat dikerjakan: {t.alasan_terkunci}.</span>
          </span>
        </NoticeBox>
      )}
      {t.dapat_dikerjakan && !t.boleh_aksi && !tuntas && (
        <NoticeBox>
          <span className="inline-flex items-start gap-1.5">
            <Hourglass className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <span>
              Menunggu <span className="font-semibold">{peranLabel(t.peran)}</span> mengerjakan tahap ini.
              {t.status === "draft" && " Draf sudah tersimpan."}
            </span>
          </span>
        </NoticeBox>
      )}
      {t.boleh_aksi && t.dapat_dikerjakan && tuntas && !t.dapat_diubah && (
        <NoticeBox>
          <span className="inline-flex items-start gap-1.5">
            <Lock className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <span>Tahap ini terkunci karena tahap sesudahnya sudah selesai, agar dokumen tidak berselisih. Admin dapat membuka ulang tahap sesudahnya.</span>
          </span>
        </NoticeBox>
      )}

      {bisaMengubah && tuntas && !mengubah && (
        <div className="flex flex-wrap gap-2">
          <SecondaryButton onClick={() => setMengubah(true)}>
            {t.jenis === "form" ? (t.status === "dilewati" ? "Buat dokumen di aplikasi" : "Ubah isian dan buat ulang dokumen") : "Ubah catatan"}
          </SecondaryButton>
        </div>
      )}

      {tampilkanPanel && (
        <div className="space-y-4">
          {bisaLewati && <KotakLuarAplikasi tahap={t} checked={luar} onChange={setLuar} />}
          {bisaLewati && luar && <LewatiTahap usulanId={usulan.id} tahap={t} onChanged={onChanged} />}
          <div className={bisaLewati && luar ? "hidden" : "space-y-4"}>
            {t.jenis === "eksternal" ? (
              <EksternalPanel usulanId={usulan.id} tahap={t} sudahSelesai={t.status === "selesai"} onChanged={onChanged} />
            ) : (
              <FormTahap t={t} usulan={usulan} saya={saya} onChanged={onChanged} />
            )}
          </div>
          {mengubah && (
            <div className="flex justify-end">
              <button type="button" onClick={() => setMengubah(false)} className="text-sm font-medium text-slate-500 hover:text-slate-700">
                Tutup formulir
              </button>
            </div>
          )}
        </div>
      )}

      {saya.admin && tuntas && t.dapat_diubah && <BukaUlang usulanId={usulan.id} tahap={t} onChanged={onChanged} />}
    </>
  );
}

function Ringkas({ label, nilai, lebar }: { label: string; nilai: string; lebar?: boolean }) {
  return (
    <div className={lebar ? "sm:col-span-3" : ""}>
      <dt className="text-xs text-slate-500">{label}</dt>
      <dd className="whitespace-pre-line break-words font-medium text-slate-800">{nilai}</dd>
    </div>
  );
}

function FormTahap({ t, usulan, saya, onChanged }: { t: SapaTahapDetail; usulan: SapaUsulan; saya: SapaSaya; onChanged: () => void }) {
  const props = { usulan, tahap: t, saya, onChanged };
  switch (t.kunci) {
    case "tim":
      return <TimForm {...props} />;
    case "ba":
      return <BAForm {...props} />;
    case "nd_satker":
      return <NDSatkerForm {...props} />;
    case "nd_ue1":
      return <NDUE1Form {...props} />;
  }
  return <Alert message="Formulir untuk tahap ini belum tersedia." />;
}

// Kotak centang untuk tahap yang boleh dikerjakan di luar aplikasi (SK Tim, Berita Acara).
function KotakLuarAplikasi({ tahap, checked, onChange }: { tahap: SapaTahapDetail; checked: boolean; onChange: (v: boolean) => void }) {
  const id = useId();
  const nama = tahap.label.replace(/^Penyusunan\s+/i, ""); // "Pembentukan Tim", "Berita Acara Penelitian"
  return (
    <label
      htmlFor={id}
      className={`flex cursor-pointer items-start gap-3 rounded-xl border p-3.5 transition-colors ${checked ? "border-blue-300 bg-blue-50/70" : "border-slate-200 bg-white hover:border-slate-300"}`}
    >
      <input
        id={id}
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="mt-0.5 h-5 w-5 shrink-0 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
      />
      <span className="min-w-0">
        <span className="block text-sm font-semibold text-slate-900">{nama} sudah dibuat di luar aplikasi</span>
        <span className="mt-0.5 block text-xs leading-relaxed text-slate-500">
          Centang bila dokumennya sudah dibuat dan ditetapkan di luar aplikasi. Tahap ini dilewati dan Anda dapat langsung melanjutkan ke tahap berikutnya.
        </span>
      </span>
    </label>
  );
}

// Keterangan dokumen yang dibuat di luar aplikasi (mis. nomor dan tanggalnya) wajib dicatat, lalu tahap dilewati.
function LewatiTahap({ usulanId, tahap, onChanged }: { usulanId: string; tahap: SapaTahapDetail; onChanged: () => void }) {
  const [catatan, setCatatan] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ErrorInfo | null>(null);

  const kirim = async () => {
    setBusy(true);
    setError(null);
    try {
      await skipSapaTahap(usulanId, tahap.kunci, catatan);
      onChanged();
    } catch (err) {
      setError(errorInfo(err, "Gagal melewati tahap"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-3 rounded-xl border border-slate-200 bg-slate-50 p-3.5">
      <TextAreaField
        label="Keterangan dokumen"
        required
        value={catatan}
        onChange={setCatatan}
        maxLength={1000}
        rows={2}
        placeholder="mis. SK Tim Nomor KEP-12/2026 tanggal 2 Januari 2026"
        hint="Minimal 5 karakter. Catat nomor dan tanggal dokumennya agar tercatat di usulan."
      />
      <ErrorBox error={error} />
      <div className="flex justify-end">
        <PrimaryButton onClick={kirim} busy={busy} disabled={catatan.trim().length < 5}>
          <SkipForward className="h-4 w-4" aria-hidden="true" />
          Simpan dan lewati tahap
        </PrimaryButton>
      </div>
    </div>
  );
}
function BukaUlang({ usulanId, tahap, onChanged }: { usulanId: string; tahap: SapaTahapDetail; onChanged: () => void }) {
  const [konfirmasi, setKonfirmasi] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ErrorInfo | null>(null);

  const kirim = async () => {
    setBusy(true);
    setError(null);
    try {
      await reopenSapaTahap(usulanId, tahap.kunci);
      setKonfirmasi(false);
      onChanged();
    } catch (err) {
      setError(errorInfo(err, "Gagal membuka ulang tahap"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-2 border-t border-dashed border-slate-200 pt-3">
      {!konfirmasi ? (
        <button type="button" onClick={() => setKonfirmasi(true)} className="inline-flex items-center gap-1.5 text-sm font-medium text-amber-700 hover:text-amber-800">
          <RotateCcw className="h-4 w-4" aria-hidden="true" />
          Buka ulang tahap (admin)
        </button>
      ) : (
        <div className="space-y-2 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
          <p>
            Tahap &ldquo;{tahap.label}&rdquo; akan kembali menjadi draf dan dapat dikerjakan ulang. Dokumen yang sudah dibuat tetap tersimpan sebagai riwayat.
          </p>
          <div className="flex gap-2">
            <SecondaryButton onClick={() => setKonfirmasi(false)}>Batal</SecondaryButton>
            <PrimaryButton onClick={kirim} busy={busy}>
              Ya, buka ulang
            </PrimaryButton>
          </div>
        </div>
      )}
      <ErrorBox error={error} />
    </div>
  );
}
