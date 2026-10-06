package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/analitik"
	"pasti-v3-backend/database"
	"pasti-v3-backend/utils"
)

// Dasbor Pengadaan terpadu: agregat dari tabel lokal (hasil penarikan) yang saling terhubung lewat kode RUP, ditambah wawasan
// berbasis angka dari package analitik. Tiap bagian dihitung terpisah: kegagalan satu bagian tidak menggagalkan bagian lain.

const (
	analitikTimeout   = 45 * time.Second
	analitikCacheTTL  = 2 * time.Minute
	hariPeringatan    = 60 // kontrak yang berakhir dalam sekian hari disorot
	maksKontrakDaftar = 10
)

// Dataset yang dipakai dasbor; yang masih kosong untuk KLPD dan tahun terpilih dilaporkan sebagai "belum ditarik".
var datasetDasbor = []string{
	"rup/paket-penyedia", "rup/paket-swakelola", "rup/paket-swakelola-terumumkan", "rup/program-master",
	"tender/pengumuman", "tender/peserta-tender", "tender/tender-selesai", "tender/tender-selesai-nilai",
	"tender/non-tender-pengumuman", "tender/non-tender-selesai",
	"tender/tender-ekontrak-kontrak", "tender/non-tender-ekontrak-kontrak",
	"ekatalog-archive/paket-e-purchasing", "ekatalog/paket-e-purchasing", "ekatalog/e-purchasing-by-produk",
}

// Penyaring RUP yang masih berlaku: tidak dihapus dan masih aktif (kolom kosong dianggap berlaku).
const aktifRUP = `ISNULL(status_delete_rup,0)=0 AND ISNULL(status_aktif_rup,1)=1`

// pengumpul menjalankan query agregat untuk satu KLPD dan tahun. Setiap query memakai @p1 = kode KLPD, @p2 = tahun, @p3 = hari ini.
type pengumpul struct {
	ctx     context.Context
	db      *sql.DB
	klpd    string
	tahun   string
	hariIni time.Time
}

func (p *pengumpul) args() []interface{} { return []interface{}{p.klpd, p.tahun, p.hariIni} }

// skalar menjalankan query satu baris; tanpa baris (mis. median dari himpunan kosong) dianggap nilai nol.
func (p *pengumpul) skalar(q string, tujuan ...interface{}) error {
	err := p.db.QueryRowContext(p.ctx, q, p.args()...).Scan(tujuan...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

// kelompok membaca baris (label, jumlah, nilai).
func (p *pengumpul) kelompok(q string) ([]analitik.Pasangan, error) {
	rows, err := p.db.QueryContext(p.ctx, q, p.args()...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []analitik.Pasangan{}
	for rows.Next() {
		var x analitik.Pasangan
		var label sql.NullString
		if err := rows.Scan(&label, &x.Jumlah, &x.Nilai); err != nil {
			return nil, err
		}
		x.Label = label.String
		out = append(out, x)
	}
	return out, rows.Err()
}

// bulan membaca baris (bulan, jumlah, nilai) dan mengisinya menjadi 12 bulan penuh.
func (p *pengumpul) bulan(q string) ([]analitik.TitikBulan, error) {
	rows, err := p.db.QueryContext(p.ctx, q, p.args()...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]analitik.TitikBulan, 12)
	for i := range out {
		out[i].Bulan = i + 1
	}
	for rows.Next() {
		var b int
		var j int64
		var n float64
		if err := rows.Scan(&b, &j, &n); err != nil {
			return nil, err
		}
		if b >= 1 && b <= 12 {
			out[b-1].Jumlah, out[b-1].Nilai = j, n
		}
	}
	return out, rows.Err()
}

// lbl: label kelompok dari satu kolom; kosong menjadi "(tidak diisi)".
func lbl(kolom string) string {
	return "ISNULL(NULLIF(LTRIM(RTRIM(" + kolom + ")),''),'(tidak diisi)')"
}

// sqlKelompok menyusun query pengelompokan teratas. jumlah = ekspresi hitung (mis. COUNT_BIG(*)), nilai = ekspresi yang dijumlahkan.
// Semua potongan SQL berasal dari kode ini, bukan dari masukan pengguna.
func sqlKelompok(label, jumlah, nilai, dari, where string, top int, urutJumlah bool) string {
	urut := "nilai DESC, jumlah DESC"
	if urutJumlah {
		urut = "jumlah DESC, nilai DESC"
	}
	return fmt.Sprintf(`SELECT TOP (%d) %s AS label, %s AS jumlah, ISNULL(SUM(%s),0) AS nilai FROM %s WHERE %s GROUP BY %s ORDER BY %s`,
		top, label, jumlah, nilai, dari, where, label, urut)
}

// tambahLainnya menambahkan kelompok "Lainnya" bila jumlah seluruhnya lebih besar dari jumlah yang tampil.
func tambahLainnya(daftar []analitik.Pasangan, totalJumlah int64, totalNilai float64) []analitik.Pasangan {
	var j int64
	var n float64
	for _, x := range daftar {
		j += x.Jumlah
		n += x.Nilai
	}
	if totalJumlah > j && totalNilai-n > 0 {
		daftar = append(daftar, analitik.Pasangan{Label: "Lainnya", Jumlah: totalJumlah - j, Nilai: totalNilai - n})
	}
	return daftar
}

const (
	nilaiPagu = "CAST(ISNULL(pagu,0) AS DECIMAL(38,2))"
	hitung    = "COUNT_BIG(*)"
)

// ---- bagian RUP ----

func (p *pengumpul) rup() (*analitik.RUP, error) {
	const dari = "inaproc_paket_penyedia"
	where := "kd_klpd=@p1 AND tahun_anggaran=@p2 AND " + aktifRUP
	r := &analitik.RUP{}
	if err := p.skalar("SELECT COUNT_BIG(*), ISNULL(SUM("+nilaiPagu+"),0) FROM "+dari+" WHERE "+where, &r.TotalPaket, &r.TotalPagu); err != nil {
		return nil, err
	}
	var err error
	if r.PerMetode, err = p.kelompok(sqlKelompok(lbl("metode_pengadaan"), hitung, nilaiPagu, dari, where, 8, false)); err != nil {
		return nil, err
	}
	r.PerMetode = tambahLainnya(r.PerMetode, r.TotalPaket, r.TotalPagu)
	if r.PerJenis, err = p.kelompok(sqlKelompok(lbl("jenis_pengadaan"), hitung, nilaiPagu, dari, where, 8, false)); err != nil {
		return nil, err
	}
	if r.TopSatker, err = p.kelompok(sqlKelompok(lbl("nama_satker"), hitung, nilaiPagu, dari, where, 10, false)); err != nil {
		return nil, err
	}
	if r.StatusUmumkan, err = p.kelompok(sqlKelompok(lbl("status_umumkan_rup"), hitung, nilaiPagu, dari, where, 8, false)); err != nil {
		return nil, err
	}
	if r.StatusUKM, err = p.kelompok(sqlKelompok(lbl("status_ukm"), hitung, nilaiPagu, dari, where, 6, false)); err != nil {
		return nil, err
	}
	if r.StatusPDN, err = p.kelompok(sqlKelompok(lbl("status_pdn"), hitung, nilaiPagu, dari, where, 6, false)); err != nil {
		return nil, err
	}
	if r.PerBulanPemilihan, err = p.bulan("SELECT MONTH(tgl_awal_pemilihan), COUNT_BIG(*), ISNULL(SUM(" + nilaiPagu + "),0) FROM " + dari + " WHERE " + where +
		" AND tgl_awal_pemilihan IS NOT NULL AND YEAR(tgl_awal_pemilihan) = CAST(@p2 AS INT) GROUP BY MONTH(tgl_awal_pemilihan)"); err != nil {
		return nil, err
	}
	if err := p.skalar("SELECT COUNT_BIG(*) FROM inaproc_paket_swakelola WHERE kd_klpd=@p1 AND tahun_anggaran=@p2", &r.PaketSwakelola); err != nil {
		return nil, err
	}
	if err := p.skalar("SELECT ISNULL(SUM("+nilaiPagu+"),0) FROM inaproc_paket_swakelola_terumumkan WHERE kd_klpd=@p1 AND tahun_anggaran=@p2 AND "+aktifRUP, &r.PaguSwakelola); err != nil {
		return nil, err
	}
	if err := p.skalar("SELECT ISNULL(SUM(CAST(ISNULL(pagu_program,0) AS DECIMAL(38,2))),0) FROM inaproc_program_master WHERE kd_klpd=@p1 AND tahun_anggaran=@p2 AND ISNULL(is_deleted,0)=0", &r.PaguProgram); err != nil {
		return nil, err
	}
	return r, nil
}

// ---- bagian pemilihan (tender dan non-tender) ----

// Pengumuman terbaru per tender / non-tender (bila sebuah tender punya beberapa versi, hanya versi terbaru yang dihitung).
const (
	cteTender = `WITH tp AS (SELECT * FROM (SELECT *, ROW_NUMBER() OVER (PARTITION BY kd_tender ORDER BY ISNULL(versi_tender,0) DESC, tgl_pengumuman_tender DESC) rn
		FROM inaproc_tender_pengumuman WHERE kd_klpd=@p1 AND tahun_anggaran=@p2) x WHERE rn=1) `
	cteNonTender = `WITH np AS (SELECT * FROM (SELECT *, ROW_NUMBER() OVER (PARTITION BY kd_nontender ORDER BY ISNULL(versi_nontender,0) DESC, tgl_pengumuman_nontender DESC) rn
		FROM inaproc_non_tender_pengumuman WHERE kd_klpd=@p1 AND tahun_anggaran=@p2) x WHERE rn=1) `

	// Paket selesai yang punya HPS dan nilai kontrak: efisiensi (HPS - kontrak) / HPS.
	efisiensiGabung = `(SELECT (hps - nilai_kontrak) * 100.0 / hps AS e, hps AS hps, nilai_kontrak AS kontrak FROM inaproc_tender_selesai_nilai
		WHERE kd_klpd=@p1 AND tahun_anggaran=@p2 AND hps>0 AND nilai_kontrak>0
		UNION ALL SELECT (hps - nilai_kontrak) * 100.0 / hps, hps, nilai_kontrak FROM inaproc_non_tender_selesai
		WHERE kd_klpd=@p1 AND tahun_anggaran=@p2 AND hps>0 AND nilai_kontrak>0) t`

	// Penyedia dikelompokkan menurut nama (huruf besar, tanpa spasi tepi), dari tender dan non-tender.
	cteVendor = `WITH v AS (SELECT kunci, MIN(nama) AS nama, SUM(nilai) AS tot, COUNT_BIG(*) AS jml FROM (
		SELECT UPPER(LTRIM(RTRIM(nama_penyedia))) AS kunci, nama_penyedia AS nama, ISNULL(nilai_kontrak,0) AS nilai FROM inaproc_tender_selesai_nilai
		WHERE kd_klpd=@p1 AND tahun_anggaran=@p2 AND LTRIM(RTRIM(ISNULL(nama_penyedia,''))) <> ''
		UNION ALL SELECT UPPER(LTRIM(RTRIM(nama_penyedia))), nama_penyedia, ISNULL(nilai_kontrak,0) FROM inaproc_non_tender_selesai
		WHERE kd_klpd=@p1 AND tahun_anggaran=@p2 AND LTRIM(RTRIM(ISNULL(nama_penyedia,''))) <> '') x GROUP BY kunci) `

	selesaiKontrakTender = `SELECT ISNULL(SUM(nilai_kontrak),0) FROM inaproc_tender_selesai_nilai WHERE kd_klpd=@p1 AND tahun_anggaran=@p2`
	selesaiKontrakNon    = `SELECT ISNULL(SUM(nilai_kontrak),0) FROM inaproc_non_tender_selesai WHERE kd_klpd=@p1 AND tahun_anggaran=@p2`

	// Rentang efisiensi: 1 = di atas HPS, 2 = 0-5%, 3 = 5-10%, 4 = 10-20%, 5 = di atas 20%.
	rentangEfisiensi = `CASE WHEN e < 0 THEN 1 WHEN e < 5 THEN 2 WHEN e < 10 THEN 3 WHEN e < 20 THEN 4 ELSE 5 END`
)

var labelRentangEfisiensi = [...]string{"Di atas HPS", "0-5%", "5-10%", "10-20%", "Di atas 20%"}

func (p *pengumpul) pemilihan() (*analitik.Pemilihan, error) {
	m := &analitik.Pemilihan{}
	var err error

	// Tender dan non-tender yang diumumkan.
	if err = p.skalar(cteTender+"SELECT COUNT_BIG(*), ISNULL(SUM(CAST(ISNULL(pagu,0) AS DECIMAL(38,2))),0), ISNULL(SUM(CAST(ISNULL(hps,0) AS DECIMAL(38,2))),0) FROM tp",
		&m.TenderJumlah, &m.TenderPagu, &m.TenderHPS); err != nil {
		return nil, err
	}
	if err = p.skalar(cteNonTender+"SELECT COUNT_BIG(*), ISNULL(SUM(CAST(ISNULL(pagu,0) AS DECIMAL(38,2))),0), ISNULL(SUM(CAST(ISNULL(hps,0) AS DECIMAL(38,2))),0) FROM np",
		&m.NonTenderJumlah, &m.NonTenderPagu, &m.NonTenderHPS); err != nil {
		return nil, err
	}
	nilaiTender := "CAST(ISNULL(pagu,0) AS DECIMAL(38,2))"
	if m.StatusTender, err = p.kelompok(cteTender + sqlKelompok(lbl("status_tender"), hitung, nilaiTender, "tp", "1=1", 8, true)); err != nil {
		return nil, err
	}
	if m.MetodeTender, err = p.kelompok(cteTender + sqlKelompok(lbl("mtd_pemilihan"), hitung, nilaiTender, "tp", "1=1", 8, false)); err != nil {
		return nil, err
	}
	if m.JenisTender, err = p.kelompok(cteTender + sqlKelompok(lbl("jenis_pengadaan"), hitung, nilaiTender, "tp", "1=1", 8, false)); err != nil {
		return nil, err
	}
	if m.MetodeNonTender, err = p.kelompok(cteNonTender + sqlKelompok(lbl("mtd_pemilihan"), hitung, nilaiTender, "np", "1=1", 8, false)); err != nil {
		return nil, err
	}
	if m.PerBulanTender, err = p.bulan(cteTender + `SELECT MONTH(tgl_pengumuman_tender), COUNT_BIG(*), ISNULL(SUM(` + nilaiTender + `),0) FROM tp
		WHERE tgl_pengumuman_tender IS NOT NULL AND YEAR(tgl_pengumuman_tender) = CAST(@p2 AS INT) GROUP BY MONTH(tgl_pengumuman_tender)`); err != nil {
		return nil, err
	}
	if m.PerBulanNonTender, err = p.bulan(cteNonTender + `SELECT MONTH(tgl_pengumuman_nontender), COUNT_BIG(*), ISNULL(SUM(` + nilaiTender + `),0) FROM np
		WHERE tgl_pengumuman_nontender IS NOT NULL AND YEAR(tgl_pengumuman_nontender) = CAST(@p2 AS INT) GROUP BY MONTH(tgl_pengumuman_nontender)`); err != nil {
		return nil, err
	}

	// Nilai kontrak hasil pemilihan (tender dan non-tender selesai).
	var kt, kn float64
	if err = p.skalar(selesaiKontrakTender, &kt); err != nil {
		return nil, err
	}
	if err = p.skalar(selesaiKontrakNon, &kn); err != nil {
		return nil, err
	}
	m.NilaiKontrak = kt + kn
	if err = p.skalar("SELECT COUNT_BIG(*) FROM inaproc_tender_selesai WHERE kd_klpd=@p1 AND tahun_anggaran=@p2", &m.TenderSelesai); err != nil {
		return nil, err
	}

	// Efisiensi harga.
	e := &m.Efisiensi
	if err = p.skalar("SELECT COUNT_BIG(*), ISNULL(SUM(hps),0), ISNULL(SUM(kontrak),0) FROM "+efisiensiGabung, &e.Sampel, &e.TotalHPS, &e.TotalKontrak); err != nil {
		return nil, err
	}
	if e.TotalHPS > 0 {
		e.Persen = (e.TotalHPS - e.TotalKontrak) / e.TotalHPS * 100
	}
	if e.Sampel > 0 {
		if err = p.skalar("SELECT TOP 1 CAST(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY e) OVER () AS FLOAT) FROM "+efisiensiGabung, &e.Median); err != nil {
			return nil, err
		}
	}
	e.Sebaran = make([]analitik.Pasangan, len(labelRentangEfisiensi))
	for i, l := range labelRentangEfisiensi {
		e.Sebaran[i].Label = l
	}
	if err = p.perRentang("SELECT b, COUNT_BIG(*) FROM (SELECT "+rentangEfisiensi+" AS b FROM "+efisiensiGabung+") x GROUP BY b", e.Sebaran); err != nil {
		return nil, err
	}

	// Persaingan: jumlah peserta per tender.
	if m.Persaingan, err = p.persaingan(); err != nil {
		return nil, err
	}

	// Waktu proses pemilihan.
	const hariProses = `(SELECT DATEDIFF(DAY, tgl_pengumuman_tender, tgl_penetapan_pemenang) AS d FROM inaproc_tender_selesai
		WHERE kd_klpd=@p1 AND tahun_anggaran=@p2 AND tgl_pengumuman_tender IS NOT NULL AND tgl_penetapan_pemenang IS NOT NULL) x WHERE d BETWEEN 0 AND 730`
	w := &m.WaktuProses
	if err = p.skalar("SELECT COUNT_BIG(*), ISNULL(AVG(CAST(d AS FLOAT)),0) FROM "+hariProses, &w.Sampel, &w.Rata); err != nil {
		return nil, err
	}
	if w.Sampel > 0 {
		if err = p.skalar("SELECT TOP 1 CAST(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY d) OVER () AS FLOAT) FROM "+hariProses, &w.Median); err != nil {
			return nil, err
		}
	}

	// Pasar penyedia.
	if err = p.skalar(cteVendor+`SELECT COUNT_BIG(*), ISNULL(SUM(v.tot),0),
		ISNULL(SUM(POWER(CAST(v.tot AS FLOAT) * 100.0 / NULLIF(CAST(g.gt AS FLOAT),0), 2)),0)
		FROM v CROSS JOIN (SELECT SUM(tot) AS gt FROM v) g`, &m.Pasar.JumlahPenyedia, &m.Pasar.TotalNilai, &m.Pasar.HHI); err != nil {
		return nil, err
	}
	if m.Pasar.Top, err = p.kelompok(cteVendor + "SELECT TOP 10 nama AS label, jml AS jumlah, tot AS nilai FROM v ORDER BY tot DESC, jml DESC"); err != nil {
		return nil, err
	}
	return m, nil
}

// perRentang mengisi jumlah ke daftar menurut nomor rentang 1..len(daftar).
func (p *pengumpul) perRentang(q string, daftar []analitik.Pasangan) error {
	rows, err := p.db.QueryContext(p.ctx, q, p.args()...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var b int
		var j int64
		if err := rows.Scan(&b, &j); err != nil {
			return err
		}
		if b >= 1 && b <= len(daftar) {
			daftar[b-1].Jumlah = j
		}
	}
	return rows.Err()
}

var labelPeserta = [...]string{"1 peserta", "2 peserta", "3 peserta", "4-5 peserta", "6+ peserta"}

func (p *pengumpul) persaingan() (analitik.Persaingan, error) {
	var c analitik.Persaingan
	rows, err := p.db.QueryContext(p.ctx, `SELECT n, COUNT_BIG(*) FROM (SELECT COUNT_BIG(*) AS n FROM inaproc_peserta_tender
		WHERE kd_klpd=@p1 AND tahun_anggaran=@p2 AND kd_tender IS NOT NULL GROUP BY kd_tender) x GROUP BY n`, p.args()...)
	if err != nil {
		return c, err
	}
	defer rows.Close()
	c.Sebaran = make([]analitik.Pasangan, len(labelPeserta))
	for i, l := range labelPeserta {
		c.Sebaran[i].Label = l
	}
	var totalPeserta int64
	for rows.Next() {
		var n, tender int64
		if err := rows.Scan(&n, &tender); err != nil {
			return c, err
		}
		c.TenderBerpeserta += tender
		totalPeserta += n * tender
		if n == 1 {
			c.SatuPeserta += tender
		}
		switch {
		case n <= 1:
			c.Sebaran[0].Jumlah += tender
		case n == 2:
			c.Sebaran[1].Jumlah += tender
		case n == 3:
			c.Sebaran[2].Jumlah += tender
		case n <= 5:
			c.Sebaran[3].Jumlah += tender
		default:
			c.Sebaran[4].Jumlah += tender
		}
	}
	if err := rows.Err(); err != nil {
		return c, err
	}
	if c.TenderBerpeserta > 0 {
		c.RataPeserta = float64(totalPeserta) / float64(c.TenderBerpeserta)
	}
	return c, nil
}

// ---- bagian kontrak ----

// Kontrak terbaru per nomor kontrak (addendum membuat versi baru; hanya versi terbaru yang dihitung), tender dan non-tender digabung.
const cteKontrak = `WITH ks AS (
	SELECT 'Tender' AS jenis, no_kontrak, nama_paket, nama_penyedia, ISNULL(nilai_kontrak,0) AS nilai, status_kontrak, tgl_kontrak, tgl_kontrak_akhir,
		CASE WHEN ISNULL(versi_addendum,0) > 0 OR LOWER(ISNULL(apakah_addendum,'')) IN ('ya','y','true','1') THEN 1 ELSE 0 END AS addendum
	FROM (SELECT *, ROW_NUMBER() OVER (PARTITION BY kd_tender, no_kontrak ORDER BY ISNULL(versi_addendum,0) DESC) rn
		FROM inaproc_tender_ekontrak_kontrak WHERE kd_klpd=@p1 AND tahun_anggaran=@p2) a WHERE rn=1
	UNION ALL
	SELECT 'Non-tender', no_kontrak, nama_paket, nama_penyedia, ISNULL(nilai_kontrak,0), status_kontrak, tgl_kontrak, tgl_kontrak_akhir,
		CASE WHEN ISNULL(versi_addendum,0) > 0 OR LOWER(ISNULL(apakah_addendum,'')) IN ('ya','y','true','1') THEN 1 ELSE 0 END
	FROM (SELECT *, ROW_NUMBER() OVER (PARTITION BY kd_nontender, no_kontrak ORDER BY ISNULL(versi_addendum,0) DESC) rn
		FROM inaproc_non_tender_ekontrak_kontrak WHERE kd_klpd=@p1 AND tahun_anggaran=@p2) b WHERE rn=1) `

// Kontrak yang sudah selesai atau berhenti tidak disorot sebagai "segera berakhir".
const kontrakBerjalan = `tgl_kontrak_akhir >= @p3 AND tgl_kontrak_akhir < DATEADD(DAY, 61, @p3)
	AND LOWER(ISNULL(status_kontrak,'')) NOT LIKE '%selesai%' AND LOWER(ISNULL(status_kontrak,'')) NOT LIKE '%putus%' AND LOWER(ISNULL(status_kontrak,'')) NOT LIKE '%batal%'`

func (p *pengumpul) kontrak() (*analitik.Kontrak, error) {
	k := &analitik.Kontrak{HariPeringatan: hariPeringatan}
	rows, err := p.db.QueryContext(p.ctx, cteKontrak+`SELECT jenis, COUNT_BIG(*), ISNULL(SUM(nilai),0), ISNULL(SUM(addendum),0) FROM ks GROUP BY jenis`, p.args()...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var jenis string
		var jml, add int64
		var nilai float64
		if err := rows.Scan(&jenis, &jml, &nilai, &add); err != nil {
			rows.Close()
			return nil, err
		}
		if jenis == "Tender" {
			k.TenderJumlah, k.TenderNilai = jml, nilai
		} else {
			k.NonTenderJumlah, k.NonTenderNilai = jml, nilai
		}
		k.Addendum += add
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	if k.Status, err = p.kelompok(cteKontrak + sqlKelompok(lbl("status_kontrak"), hitung, "nilai", "ks", "1=1", 8, true)); err != nil {
		return nil, err
	}
	if k.PerBulan, err = p.bulan(cteKontrak + `SELECT MONTH(tgl_kontrak), COUNT_BIG(*), ISNULL(SUM(nilai),0) FROM ks
		WHERE tgl_kontrak IS NOT NULL AND YEAR(tgl_kontrak) = CAST(@p2 AS INT) GROUP BY MONTH(tgl_kontrak)`); err != nil {
		return nil, err
	}
	if err = p.skalar(cteKontrak+"SELECT COUNT_BIG(*), ISNULL(SUM(nilai),0) FROM ks WHERE "+kontrakBerjalan, &k.BerakhirDalam, &k.NilaiBerakhir); err != nil {
		return nil, err
	}
	if k.BerakhirDalam > 0 {
		rows, err := p.db.QueryContext(p.ctx, cteKontrak+fmt.Sprintf(`SELECT TOP (%d) jenis, ISNULL(no_kontrak,''), ISNULL(nama_paket,''), ISNULL(nama_penyedia,''), nilai, tgl_kontrak_akhir,
			DATEDIFF(DAY, @p3, tgl_kontrak_akhir) FROM ks WHERE %s ORDER BY tgl_kontrak_akhir, nilai DESC`, maksKontrakDaftar, kontrakBerjalan), p.args()...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var x analitik.KontrakBerakhir
			if err := rows.Scan(&x.Jenis, &x.NoKontrak, &x.NamaPaket, &x.Penyedia, &x.Nilai, &x.Berakhir, &x.SisaHari); err != nil {
				return nil, err
			}
			k.AkanBerakhir = append(k.AkanBerakhir, x)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return k, nil
}

// ---- bagian E-Katalog ----

func (p *pengumpul) ekatalog() (*analitik.Ekatalog, error) {
	e := &analitik.Ekatalog{}
	var err error

	// V5 (arsip): satu paket (kd_paket) bisa punya beberapa baris produk.
	const v5 = "inaproc_ekatalog_paket_epurchasing"
	w5 := "kd_klpd=@p1 AND tahun_anggaran=@p2"
	if err = p.skalar("SELECT COUNT_BIG(DISTINCT kd_paket), ISNULL(SUM(total_harga),0) FROM "+v5+" WHERE "+w5, &e.V5.Paket, &e.V5.Nilai); err != nil {
		return nil, err
	}
	if e.V5.PerBulan, err = p.bulan("SELECT MONTH(tanggal_buat_paket), COUNT_BIG(DISTINCT kd_paket), ISNULL(SUM(total_harga),0) FROM " + v5 + " WHERE " + w5 +
		" AND tanggal_buat_paket IS NOT NULL AND YEAR(tanggal_buat_paket) = CAST(@p2 AS INT) GROUP BY MONTH(tanggal_buat_paket)"); err != nil {
		return nil, err
	}
	if e.V5.TopKomoditas, err = p.kelompok(`SELECT TOP 10 ISNULL((SELECT TOP 1 NULLIF(nama_komoditas,'') FROM inaproc_ekatalog_komoditas k WHERE k.kd_komoditas = g.kd), 'Komoditas ' + g.kd) AS label, g.jml, g.nilai
		FROM (SELECT ISNULL(kd_komoditas,'-') AS kd, COUNT_BIG(DISTINCT kd_paket) AS jml, ISNULL(SUM(total_harga),0) AS nilai FROM ` + v5 + ` WHERE ` + w5 + ` GROUP BY ISNULL(kd_komoditas,'-')) g
		ORDER BY g.nilai DESC`); err != nil {
		return nil, err
	}
	if e.V5.TopPenyedia, err = p.kelompok(`SELECT TOP 10 ISNULL((SELECT TOP 1 NULLIF(nama_penyedia,'') FROM inaproc_ekatalog_penyedia k WHERE k.kd_penyedia = g.kd), 'Penyedia ' + g.kd) AS label, g.jml, g.nilai
		FROM (SELECT ISNULL(kd_penyedia,'-') AS kd, COUNT_BIG(DISTINCT kd_paket) AS jml, ISNULL(SUM(total_harga),0) AS nilai FROM ` + v5 + ` WHERE ` + w5 + ` GROUP BY ISNULL(kd_penyedia,'-')) g
		ORDER BY g.nilai DESC`); err != nil {
		return nil, err
	}
	if e.V5.Status, err = p.kelompok(sqlKelompok("ISNULL(NULLIF(LTRIM(RTRIM(paket_status_str)),''),"+lbl("status_paket")+")", "COUNT_BIG(DISTINCT kd_paket)", "total_harga", v5, w5, 8, true)); err != nil {
		return nil, err
	}

	// V6.
	const v6 = "inaproc_ekatalog6_paket_epurchasing"
	w6 := "kode_klpd=@p1 AND fiscal_year=@p2"
	if err = p.skalar("SELECT COUNT_BIG(DISTINCT order_id), ISNULL(SUM(total),0), COUNT_BIG(DISTINCT CASE WHEN is_swasta=1 THEN order_id END), ISNULL(SUM(CASE WHEN is_swasta=1 THEN total ELSE 0 END),0) FROM "+v6+" WHERE "+w6,
		&e.V6.Order, &e.V6.Nilai, &e.V6.OrderSwasta, &e.V6.NilaiSwasta); err != nil {
		return nil, err
	}
	if e.V6.PerBulan, err = p.bulan("SELECT MONTH(order_date), COUNT_BIG(DISTINCT order_id), ISNULL(SUM(total),0) FROM " + v6 + " WHERE " + w6 +
		" AND order_date IS NOT NULL AND YEAR(order_date) = CAST(@p2 AS INT) GROUP BY MONTH(order_date)"); err != nil {
		return nil, err
	}
	if e.V6.TopPenyedia, err = p.kelompok(`SELECT TOP 10 ISNULL((SELECT TOP 1 NULLIF(nama_penyedia,'') FROM inaproc_ekatalog6_penyedia k WHERE k.kode_penyedia = g.kd), 'Penyedia ' + g.kd) AS label, g.jml, g.nilai
		FROM (SELECT ISNULL(kode_penyedia,'-') AS kd, COUNT_BIG(DISTINCT order_id) AS jml, ISNULL(SUM(total),0) AS nilai FROM ` + v6 + ` WHERE ` + w6 + ` GROUP BY ISNULL(kode_penyedia,'-')) g
		ORDER BY g.nilai DESC`); err != nil {
		return nil, err
	}
	if e.V6.Status, err = p.kelompok(sqlKelompok(lbl("status"), "COUNT_BIG(DISTINCT order_id)", "total", v6, w6, 8, true)); err != nil {
		return nil, err
	}

	// Transaksi per produk (V6).
	const tr = "inaproc_ekatalog6_epurchasing_produk"
	wt := "kode_klpd=@p1 AND tahun=@p2"
	if err = p.skalar("SELECT COUNT_BIG(*), ISNULL(SUM(nilai_transaksi),0) FROM "+tr+" WHERE "+wt, &e.V6.TransaksiBaris, &e.V6.TransaksiNilai); err != nil {
		return nil, err
	}
	if e.V6.TopKategori, err = p.kelompok(sqlKelompok(lbl("kategori_1"), hitung, "nilai_transaksi", tr, wt, 10, false)); err != nil {
		return nil, err
	}
	return e, nil
}

// ---- corong dan pembanding ----

// Corong: setiap paket RUP aktif dimasukkan ke tahap pertama yang cocok menurut kode RUP: tender, non-tender, e-purchasing (V5 atau V6),
// atau belum diproses. Satu paket yang muncul di beberapa tahap dihitung di tahap pertama menurut urutan itu.
func (p *pengumpul) corong() (*analitik.Corong, error) {
	rows, err := p.db.QueryContext(p.ctx, `
		WITH rup AS (SELECT kd_rup, MAX(`+nilaiPagu+`) AS pagu FROM inaproc_paket_penyedia
			WHERE kd_klpd=@p1 AND tahun_anggaran=@p2 AND `+aktifRUP+` AND kd_rup IS NOT NULL GROUP BY kd_rup),
		t AS (SELECT DISTINCT LTRIM(RTRIM(s.value)) AS kd_rup FROM inaproc_tender_pengumuman x CROSS APPLY STRING_SPLIT(x.kd_rup, ';') s
			WHERE x.kd_klpd=@p1 AND x.tahun_anggaran=@p2 AND x.kd_rup IS NOT NULL AND LTRIM(RTRIM(s.value)) <> ''),
		nt AS (SELECT DISTINCT LTRIM(RTRIM(s.value)) AS kd_rup FROM inaproc_non_tender_pengumuman x CROSS APPLY STRING_SPLIT(x.kd_rup, ';') s
			WHERE x.kd_klpd=@p1 AND x.tahun_anggaran=@p2 AND x.kd_rup IS NOT NULL AND LTRIM(RTRIM(s.value)) <> ''),
		ec AS (SELECT DISTINCT LTRIM(RTRIM(s.value)) AS kd_rup FROM inaproc_ekatalog_paket_epurchasing x CROSS APPLY STRING_SPLIT(x.kd_rup, ';') s
			WHERE x.kd_klpd=@p1 AND x.tahun_anggaran=@p2 AND x.kd_rup IS NOT NULL AND LTRIM(RTRIM(s.value)) <> ''
			UNION SELECT DISTINCT LTRIM(RTRIM(s.value)) FROM inaproc_ekatalog6_paket_epurchasing x CROSS APPLY STRING_SPLIT(x.rup_code, ';') s
			WHERE x.kode_klpd=@p1 AND x.fiscal_year=@p2 AND x.rup_code IS NOT NULL AND LTRIM(RTRIM(s.value)) <> '')
		SELECT tahap, COUNT_BIG(*), ISNULL(SUM(pagu),0) FROM (
			SELECT CASE WHEN t.kd_rup IS NOT NULL THEN 'Tender' WHEN nt.kd_rup IS NOT NULL THEN 'Non-tender'
				WHEN ec.kd_rup IS NOT NULL THEN 'E-Purchasing' ELSE 'Belum diproses' END AS tahap, rup.pagu
			FROM rup LEFT JOIN t ON t.kd_rup = rup.kd_rup LEFT JOIN nt ON nt.kd_rup = rup.kd_rup LEFT JOIN ec ON ec.kd_rup = rup.kd_rup) x
		GROUP BY tahap`, p.args()...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	urut := []string{"Tender", "Non-tender", "E-Purchasing", "Belum diproses"}
	peta := map[string]analitik.Pasangan{}
	c := &analitik.Corong{}
	for rows.Next() {
		var x analitik.Pasangan
		if err := rows.Scan(&x.Label, &x.Jumlah, &x.Nilai); err != nil {
			return nil, err
		}
		peta[x.Label] = x
		c.TotalPaket += x.Jumlah
		c.TotalPagu += x.Nilai
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, l := range urut {
		x := peta[l]
		x.Label = l
		c.Tahap = append(c.Tahap, x)
	}
	return c, nil
}

// pembanding membaca angka pokok tahun sebelumnya; nil bila tidak ada datanya.
func (p *pengumpul) pembanding(tahunLalu string) (*analitik.Pembanding, error) {
	q := *p
	q.tahun = tahunLalu
	b := &analitik.Pembanding{Tahun: tahunLalu}
	if err := q.skalar("SELECT COUNT_BIG(*), ISNULL(SUM("+nilaiPagu+"),0) FROM inaproc_paket_penyedia WHERE kd_klpd=@p1 AND tahun_anggaran=@p2 AND "+aktifRUP, &b.RUPPaket, &b.RUPPagu); err != nil {
		return nil, err
	}
	if err := q.skalar(cteTender+"SELECT COUNT_BIG(*) FROM tp", &b.TenderJumlah); err != nil {
		return nil, err
	}
	var kt, kn float64
	if err := q.skalar(selesaiKontrakTender, &kt); err != nil {
		return nil, err
	}
	if err := q.skalar(selesaiKontrakNon, &kn); err != nil {
		return nil, err
	}
	b.NilaiKontrak = kt + kn
	if b.RUPPaket == 0 && b.TenderJumlah == 0 {
		return nil, nil
	}
	return b, nil
}

// ---- perakitan ----

var reTahunAnalitik = regexp.MustCompile(`^(19|20)[0-9]{2}$`)

// HitungAnalitik menghitung dasbor untuk satu KLPD dan tahun. Tiap bagian berjalan sendiri; yang gagal dicatat di Galat dan dibiarkan nil.
func HitungAnalitik(ctx context.Context, db *sql.DB, klpd, tahun string, sekarang time.Time) *analitik.Hasil {
	hari := sekarang.In(zonaWIB)
	p := &pengumpul{ctx: ctx, db: db, klpd: klpd, tahun: tahun, hariIni: time.Date(hari.Year(), hari.Month(), hari.Day(), 0, 0, 0, 0, time.UTC)}
	h := &analitik.Hasil{Tahun: tahun, KodeKLPD: klpd, Galat: map[string]string{}, Sekarang: sekarang}

	var mu sync.Mutex
	var wg sync.WaitGroup
	jalankan := func(nama string, f func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[INAPROC ANALITIK ERROR] panic pada bagian %s: %v", nama, r)
					mu.Lock()
					h.Galat[nama] = "kesalahan internal"
					mu.Unlock()
				}
			}()
			if err := f(); err != nil {
				log.Printf("[INAPROC ANALITIK ERROR] bagian %s (%s/%s): %v", nama, klpd, tahun, err)
				mu.Lock()
				h.Galat[nama] = "gagal dibaca"
				mu.Unlock()
			}
		}()
	}
	jalankan(analitik.BagianRUP, func() (err error) { r, err := p.rup(); h.RUP = r; return err })
	jalankan(analitik.BagianPemilihan, func() (err error) { r, err := p.pemilihan(); h.Pemilihan = r; return err })
	jalankan(analitik.BagianKontrak, func() (err error) { r, err := p.kontrak(); h.Kontrak = r; return err })
	jalankan(analitik.BagianEkatalog, func() (err error) { r, err := p.ekatalog(); h.Ekatalog = r; return err })
	jalankan(analitik.BagianCorong, func() (err error) { r, err := p.corong(); h.Corong = r; return err })
	jalankan("pembanding", func() error {
		var th int
		if _, err := fmt.Sscanf(tahun, "%d", &th); err != nil {
			return nil
		}
		b, err := p.pembanding(fmt.Sprint(th - 1))
		h.Pembanding = b
		return err
	})
	jalankan(analitik.BagianData, func() error {
		for _, id := range datasetDasbor {
			d, ok := DatasetByID(id)
			if !ok {
				continue
			}
			n, err := d.hitung(ctx, db, PenyaringData{KodeKLPD: klpd, Tahun: tahun})
			if err != nil {
				return err
			}
			if n == 0 {
				h.DatasetKosong = append(h.DatasetKosong, d.Nama)
			}
		}
		var t sql.NullTime
		if err := db.QueryRowContext(ctx, `SELECT MAX(started_at) FROM inaproc_sync_log WHERE status = 'success'`).Scan(&t); err == nil && t.Valid {
			// started_at disimpan sebagai waktu server aplikasi (time.Now()), jadi dibaca apa adanya.
			x := t.Time
			h.TerakhirTarik = &x
		}
		return nil
	})
	wg.Wait()
	delete(h.Galat, analitik.BagianData) // kelengkapan data hanya pelengkap; kegagalannya tidak ditampilkan sebagai bagian gagal
	if len(h.Galat) == 0 {
		h.Galat = nil
	}
	sort.Strings(h.DatasetKosong)
	return h
}

// tahunTersedia: tahun yang punya data di tabel utama untuk KLPD ini, terbaru dulu.
func tahunTersediaAnalitik(ctx context.Context, db *sql.DB, klpd string) []string {
	rows, err := db.QueryContext(ctx, `
		SELECT tahun FROM (
			SELECT tahun_anggaran AS tahun FROM inaproc_paket_penyedia WHERE kd_klpd=@p1
			UNION SELECT tahun_anggaran FROM inaproc_tender_pengumuman WHERE kd_klpd=@p1
			UNION SELECT tahun_anggaran FROM inaproc_tender_selesai_nilai WHERE kd_klpd=@p1
			UNION SELECT tahun_anggaran FROM inaproc_non_tender_pengumuman WHERE kd_klpd=@p1
			UNION SELECT tahun_anggaran FROM inaproc_ekatalog_paket_epurchasing WHERE kd_klpd=@p1
			UNION SELECT fiscal_year FROM inaproc_ekatalog6_paket_epurchasing WHERE kode_klpd=@p1
		) x WHERE tahun IS NOT NULL ORDER BY tahun DESC`, klpd)
	if err != nil {
		log.Println("[INAPROC ANALITIK WARN] gagal membaca daftar tahun:", err)
		return []string{}
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil && reTahunAnalitik.MatchString(t) {
			out = append(out, t)
		}
	}
	return out
}

// ---- cache ----

type entriAnalitik struct {
	hasil   *analitik.Hasil
	wawasan []analitik.Wawasan
	dibuat  time.Time
}

var (
	cacheAnalitikMu sync.Mutex
	cacheAnalitik   = map[string]entriAnalitik{}
)

func kosongkanCacheAnalitik() {
	cacheAnalitikMu.Lock()
	cacheAnalitik = map[string]entriAnalitik{}
	cacheAnalitikMu.Unlock()
}

// ---- HTTP ----

// GetInaprocAnalitik: dasbor Pengadaan terpadu. Query: kode_klpd (bawaan K10), tahun (bawaan: tahun terbaru yang punya data), segarkan=1
// (abaikan cache).
func GetInaprocAnalitik(c *gin.Context) {
	klpd := strings.TrimSpace(c.DefaultQuery("kode_klpd", kemenkeuKLPDCode))
	if klpd == "" || len(klpd) > 20 {
		utils.ErrorResponse(c, http.StatusBadRequest, "Kode KLPD tidak valid")
		return
	}
	tahun := strings.TrimSpace(c.Query("tahun"))
	if tahun != "" && !reTahunAnalitik.MatchString(tahun) {
		utils.ErrorResponse(c, http.StatusBadRequest, "Tahun harus berupa empat digit")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), analitikTimeout)
	defer cancel()
	tersedia := tahunTersediaAnalitik(ctx, database.DB, klpd)
	if tahun == "" {
		switch {
		case len(tersedia) > 0:
			tahun = tersedia[0]
		default:
			tahun = fmt.Sprint(time.Now().In(zonaWIB).Year())
		}
	}

	kunci := klpd + "|" + tahun
	if c.Query("segarkan") != "1" {
		cacheAnalitikMu.Lock()
		e, ada := cacheAnalitik[kunci]
		cacheAnalitikMu.Unlock()
		if ada && time.Since(e.dibuat) < analitikCacheTTL {
			utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil dasbor", gin.H{"hasil": e.hasil, "wawasan": e.wawasan, "tahun_tersedia": tersedia, "dibuat": e.dibuat, "dari_cache": true})
			return
		}
	}

	now := time.Now()
	hasil := HitungAnalitik(ctx, database.DB, klpd, tahun, now)
	wawasan := analitik.Susun(hasil)
	if wawasan == nil {
		wawasan = []analitik.Wawasan{}
	}
	cacheAnalitikMu.Lock()
	cacheAnalitik[kunci] = entriAnalitik{hasil: hasil, wawasan: wawasan, dibuat: now}
	cacheAnalitikMu.Unlock()
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil dasbor", gin.H{"hasil": hasil, "wawasan": wawasan, "tahun_tersedia": tersedia, "dibuat": now, "dari_cache": false})
}
