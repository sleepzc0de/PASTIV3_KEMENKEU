package monitor

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"pasti-v3-backend/audit"
)

// Penilaian resource: mengubah pengukuran dan riwayat menjadi kesimpulan "cukup", "perhatian", atau "kritis" per resource beserta tindakan yang disarankan
// (mis. "Tambah RAM"). Murni (tanpa I/O) supaya aturannya mudah diuji. Ambang di bawah ini sengaja konservatif dan dijelaskan pada rincian tiap penilaian.

// Ambang penilaian (persen kecuali disebut lain).
const (
	cpuRataPerhatian     = 60.0 // rata-rata 24 jam
	cpuRataKritis        = 80.0
	cpuPuncakPerhatian   = 95.0 // puncak 24 jam, bila rata-rata 24 jam sudah >= cpuRataMinPuncak
	cpuRataMinPuncak     = 30.0
	loadPerCorePerhatian = 1.5
	loadPerCoreKritis    = 3.0

	memPerhatian = 85.0 // puncak 24 jam (terpakai = total - tersedia, cache tidak dihitung)
	memKritis    = 92.0
	swapAktif    = 25.0 // persen swap terpakai yang dianggap aktif menukar memori

	diskPerhatian      = 80.0
	diskKritis         = 90.0
	hariPenuhPerhatian = 30.0
	hariPenuhKritis    = 7.0

	poolPerhatian = 70.0 // puncak koneksi terpakai / batas
	poolKritis    = 90.0

	respons95Perhatian = 1000.0 // ms, permintaan ringan
	respons95Kritis    = 3000.0
	minPermintaan      = 30

	galatPerhatian     = 1.0 // % permintaan 5xx
	galatKritis        = 5.0
	minPermintaanGalat = 50

	goroutinePerhatian = 10000
	goroutineKritis    = 50000
	rssPerhatian       = 75.0
	rssKritis          = 90.0

	pleKritis    = 100.0 // detik
	plePerhatian = 300.0
	hitPerhatian = 90.0 // buffer cache hit ratio
)

// Masukan: semua yang dibutuhkan penilaian.
type Masukan struct {
	Sistem       Sistem
	Proses       Proses
	DB           Database
	JamTerakhir  JendelaHTTP
	H24, H7      *Agregat
	LajuDiskHari *float64 // byte per hari (nil bila belum cukup data)
	LajuDBHari   *float64 // MB per hari
	Audit        audit.Statistik
}

// fmtAngka menulis angka dengan koma desimal gaya Indonesia dan pemisah ribuan titik.
func fmtAngka(v float64, desimal int) string {
	s := strconv.FormatFloat(math.Abs(v), 'f', desimal, 64)
	bulat, pecah := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		bulat, pecah = s[:i], s[i+1:]
	}
	var b strings.Builder
	for i, r := range bulat {
		if i > 0 && (len(bulat)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	out := b.String()
	if pecah != "" {
		out += "," + pecah
	}
	if v < 0 {
		out = "-" + out
	}
	return out
}

// FmtBytes menulis ukuran dalam satuan yang sesuai ("3,9 GB").
func FmtBytes(b uint64) string {
	f := float64(b)
	for _, u := range []string{"B", "KB", "MB", "GB", "TB"} {
		if f < 1024 || u == "TB" {
			if u == "B" {
				return fmt.Sprintf("%d B", b)
			}
			return fmtAngka(f, 1) + " " + u
		}
		f /= 1024
	}
	return ""
}

func fmtMB(mb float64) string { return FmtBytes(uint64(mb * 1024 * 1024)) }

func pf(v float64) *float64 { return &v }

func nilaiAtau(p *float64) string {
	if p == nil {
		return "belum ada data"
	}
	return fmtAngka(*p, 1)
}

// tingkat menggabungkan dua status ke yang lebih buruk.
func lebihBuruk(a, b string) string {
	urut := map[string]int{StatusTakAdaData: 0, StatusCukup: 1, StatusPerhatian: 2, StatusKritis: 3}
	if urut[b] > urut[a] {
		return b
	}
	return a
}

// ambil menentukan status dari nilai menurut dua ambang (naik): >= kritis, >= perhatian, selain itu cukup.
func ambil(nilai, perhatian, kritis float64) string {
	switch {
	case nilai >= kritis:
		return StatusKritis
	case nilai >= perhatian:
		return StatusPerhatian
	}
	return StatusCukup
}

func tindakan(status, tambah, pantau string) string {
	switch status {
	case StatusKritis:
		return "Perlu ditambah: " + tambah
	case StatusPerhatian:
		return "Pantau: " + pantau
	case StatusCukup:
		return "Tidak perlu ditambah"
	}
	return "Belum dapat dinilai"
}

// Evaluasi menilai semua resource. Urutannya tetap: resource server dulu, lalu kinerja aplikasi.
func Evaluasi(m Masukan) []Kapasitas {
	return []Kapasitas{
		nilaiCPU(m), nilaiMemori(m), nilaiDisk(m), nilaiDBPenyimpanan(m), nilaiDBKoneksi(m), nilaiMemoriSQL(m),
		nilaiRespons(m), nilaiGalat(m), nilaiProses(m), nilaiDBTersambung(m), nilaiAudit(m),
	}
}

func nilaiCPU(m Masukan) Kapasitas {
	k := Kapasitas{Kunci: "cpu", Nama: "CPU server", Kelompok: KelompokResource, Satuan: "%", Sekarang: m.Sistem.CPUPersen, Batas: pf(100), Status: StatusTakAdaData}
	if m.H24 != nil {
		k.Puncak24j, k.Rata24j = m.H24.CPUMaks, m.H24.CPURata
	}
	if m.H7 != nil {
		k.Puncak7h = m.H7.CPUMaks
	}
	if k.Sekarang == nil && k.Rata24j == nil {
		k.Rincian = []string{"Pemakaian CPU belum dapat dibaca (pengukuran pertama memerlukan satu selang waktu, atau platform ini tidak mendukungnya)."}
		k.Tindakan = tindakan(k.Status, "", "")
		return k
	}
	k.Status = StatusCukup
	if k.Rata24j != nil {
		k.Status = lebihBuruk(k.Status, ambil(*k.Rata24j, cpuRataPerhatian, cpuRataKritis))
		if k.Puncak24j != nil && *k.Puncak24j >= cpuPuncakPerhatian && *k.Rata24j >= cpuRataMinPuncak {
			k.Status = lebihBuruk(k.Status, StatusPerhatian)
		}
	} else if k.Sekarang != nil {
		k.Status = lebihBuruk(k.Status, ambil(*k.Sekarang, cpuRataPerhatian, cpuRataKritis))
	}
	k.Rincian = append(k.Rincian, fmt.Sprintf("Sekarang %s%% · rata-rata 24 jam %s%% · puncak 24 jam %s%% · puncak 7 hari %s%%.",
		nilaiAtau(k.Sekarang), nilaiAtau(k.Rata24j), nilaiAtau(k.Puncak24j), nilaiAtau(k.Puncak7h)))
	if m.H24 != nil && m.H24.Load1Maks != nil && m.Sistem.CPUJumlah > 0 {
		per := *m.H24.Load1Maks / float64(m.Sistem.CPUJumlah)
		k.Status = lebihBuruk(k.Status, ambil(per, loadPerCorePerhatian, loadPerCoreKritis))
		k.Rincian = append(k.Rincian, fmt.Sprintf("Beban (load average 1 menit) puncak 24 jam %s untuk %d core = %s per core; di atas 1 per core berarti ada proses yang antre menunggu CPU.",
			fmtAngka(*m.H24.Load1Maks, 2), m.Sistem.CPUJumlah, fmtAngka(per, 2)))
	}
	if m.Sistem.Kontainer != nil && m.Sistem.Kontainer.CPUBatas != nil {
		k.Rincian = append(k.Rincian, fmt.Sprintf("Container dibatasi %s core.", fmtAngka(*m.Sistem.Kontainer.CPUBatas, 1)))
	}
	k.Rincian = append(k.Rincian, fmt.Sprintf("Ambang: perhatian bila rata-rata 24 jam >= %.0f%%, perlu ditambah bila >= %.0f%% (atau beban >= %.1f per core).", cpuRataPerhatian, cpuRataKritis, loadPerCoreKritis))
	k.Tindakan = tindakan(k.Status, fmt.Sprintf("vCPU (server memiliki %d core)", m.Sistem.CPUJumlah), "belum perlu menambah CPU, tetapi lihat apakah ada tugas berat (sinkronisasi/penarikan) yang menyebabkannya")
	return k
}

func nilaiMemori(m Masukan) Kapasitas {
	k := Kapasitas{Kunci: "memori", Nama: "Memori (RAM) server", Kelompok: KelompokResource, Satuan: "%", Batas: pf(100), Status: StatusTakAdaData}
	if m.Sistem.MemTotal > 0 {
		k.Sekarang = pf(m.Sistem.MemPersen)
	}
	if m.H24 != nil {
		k.Puncak24j = m.H24.MemPersenMaks
	}
	if m.H7 != nil {
		k.Puncak7h = m.H7.MemPersenMaks
	}
	if k.Sekarang == nil && k.Puncak24j == nil {
		k.Rincian = []string{"Memori server belum dapat dibaca di platform ini."}
		k.Tindakan = tindakan(k.Status, "", "")
		return k
	}
	acuan := k.Sekarang
	if k.Puncak24j != nil {
		acuan = k.Puncak24j
	}
	k.Status = ambil(*acuan, memPerhatian, memKritis)
	k.Rincian = append(k.Rincian, fmt.Sprintf("Sekarang %s%% (%s terpakai dari %s; cache disk tidak dihitung) · puncak 24 jam %s%% · puncak 7 hari %s%%.",
		nilaiAtau(k.Sekarang), FmtBytes(m.Sistem.MemTerpakai), FmtBytes(m.Sistem.MemTotal), nilaiAtau(k.Puncak24j), nilaiAtau(k.Puncak7h)))
	if m.Sistem.SwapTotal > 0 {
		sw := float64(m.Sistem.SwapTerpakai) / float64(m.Sistem.SwapTotal) * 100
		k.Rincian = append(k.Rincian, fmt.Sprintf("Swap terpakai %s%% (%s dari %s).", fmtAngka(sw, 1), FmtBytes(m.Sistem.SwapTerpakai), FmtBytes(m.Sistem.SwapTotal)))
		if sw >= swapAktif && *acuan >= 80 {
			k.Status = lebihBuruk(k.Status, StatusKritis)
			k.Rincian = append(k.Rincian, "Memori menukar ke swap saat hampir penuh: tanda kuat memori kurang.")
		}
	}
	if m.Sistem.Kontainer != nil && m.Sistem.Kontainer.MemBatas != nil {
		k.Rincian = append(k.Rincian, "Container dibatasi memorinya sebesar "+FmtBytes(*m.Sistem.Kontainer.MemBatas)+".")
	}
	k.Rincian = append(k.Rincian, fmt.Sprintf("Ambang: perhatian bila puncak 24 jam >= %.0f%%, perlu ditambah bila >= %.0f%%.", memPerhatian, memKritis))
	k.Tindakan = tindakan(k.Status, "RAM (saat ini "+FmtBytes(m.Sistem.MemTotal)+")", "memori mulai ketat; periksa proses yang paling banyak memakai memori sebelum menambah")
	return k
}

// hariHinggaPenuh memperkirakan hari sampai ruang habis dari sisa ruang dan laju pertumbuhan per hari.
func hariHinggaPenuh(sisa, lajuPerHari float64) (float64, bool) {
	if lajuPerHari <= 0 || sisa < 0 {
		return 0, false
	}
	return sisa / lajuPerHari, true
}

func nilaiDisk(m Masukan) Kapasitas {
	k := Kapasitas{Kunci: "disk", Nama: "Disk server", Kelompok: KelompokResource, Satuan: "%", Batas: pf(100), Status: StatusTakAdaData}
	if m.Sistem.DiskTotal > 0 {
		k.Sekarang = pf(m.Sistem.DiskPersen)
	}
	if m.H24 != nil {
		k.Puncak24j = m.H24.DiskPersenMaks
	}
	if m.H7 != nil {
		k.Puncak7h = m.H7.DiskPersenMaks
	}
	if k.Sekarang == nil {
		k.Rincian = []string{"Ruang disk belum dapat dibaca di platform ini."}
		k.Tindakan = tindakan(k.Status, "", "")
		return k
	}
	k.Status = ambil(*k.Sekarang, diskPerhatian, diskKritis)
	k.Rincian = append(k.Rincian, fmt.Sprintf("Terpakai %s%% (%s dari %s; sisa %s) pada %s.", fmtAngka(*k.Sekarang, 1), FmtBytes(m.Sistem.DiskTerpakai), FmtBytes(m.Sistem.DiskTotal), FmtBytes(m.Sistem.DiskBebas), m.Sistem.DiskJalur))
	if m.LajuDiskHari != nil {
		k.Rincian = append(k.Rincian, fmt.Sprintf("Pertumbuhan perkiraan %s per hari.", FmtBytes(uint64(math.Max(*m.LajuDiskHari, 0)))))
		if hari, ok := hariHinggaPenuh(float64(m.Sistem.DiskBebas), *m.LajuDiskHari); ok {
			k.Rincian = append(k.Rincian, fmt.Sprintf("Dengan laju itu, disk penuh dalam kira-kira %s hari.", fmtAngka(hari, 0)))
			switch {
			case hari < hariPenuhKritis:
				k.Status = lebihBuruk(k.Status, StatusKritis)
			case hari < hariPenuhPerhatian:
				k.Status = lebihBuruk(k.Status, StatusPerhatian)
			}
		}
	} else {
		k.Rincian = append(k.Rincian, "Laju pertumbuhan belum dapat diperkirakan (perlu riwayat minimal 12 jam).")
	}
	k.Rincian = append(k.Rincian, fmt.Sprintf("Ambang: perhatian bila terpakai >= %.0f%%, perlu ditambah bila >= %.0f%% atau diperkirakan penuh dalam < %.0f hari.", diskPerhatian, diskKritis, hariPenuhKritis))
	k.Tindakan = tindakan(k.Status, "kapasitas disk (atau bersihkan berkas/log lama)", "ruang disk mulai berkurang; periksa log Docker dan berkas besar")
	return k
}

func nilaiDBPenyimpanan(m Masukan) Kapasitas {
	k := Kapasitas{Kunci: "db_penyimpanan", Nama: "Penyimpanan database", Kelompok: KelompokResource, Satuan: "%", Batas: pf(100), Status: StatusTakAdaData}
	sql := m.DB.SQL
	if sql == nil || sql.UkuranMB == 0 {
		k.Rincian = []string{"Ukuran database belum dapat dibaca (akun database aplikasi mungkin tidak boleh membaca sys.database_files)."}
		k.Tindakan = tindakan(k.Status, "", "")
		return k
	}
	var persenVolume *float64
	var sisaByte float64
	for _, v := range sql.Volume {
		if persenVolume == nil || v.Persen > *persenVolume {
			persenVolume, sisaByte = pf(v.Persen), float64(v.Bebas)
		}
	}
	k.Rincian = append(k.Rincian, fmt.Sprintf("Ukuran file database %s", fmtMB(sql.UkuranMB))+func() string {
		if sql.TerpakaiMB != nil {
			return fmt.Sprintf(", terisi %s.", fmtMB(*sql.TerpakaiMB))
		}
		return "."
	}())
	if persenVolume != nil {
		k.Sekarang = persenVolume
		k.Status = ambil(*persenVolume, diskPerhatian, diskKritis)
		k.Rincian = append(k.Rincian, fmt.Sprintf("Volume tempat file database berada terpakai %s%% (sisa %s).", fmtAngka(*persenVolume, 1), FmtBytes(uint64(sisaByte))))
		if m.LajuDBHari != nil {
			k.Rincian = append(k.Rincian, fmt.Sprintf("Database tumbuh kira-kira %s per hari.", fmtMB(math.Max(*m.LajuDBHari, 0))))
			if hari, ok := hariHinggaPenuh(sisaByte/1024/1024, *m.LajuDBHari); ok {
				k.Rincian = append(k.Rincian, fmt.Sprintf("Dengan laju itu, volume database penuh dalam kira-kira %s hari.", fmtAngka(hari, 0)))
				switch {
				case hari < hariPenuhKritis:
					k.Status = lebihBuruk(k.Status, StatusKritis)
				case hari < hariPenuhPerhatian:
					k.Status = lebihBuruk(k.Status, StatusPerhatian)
				}
			}
		}
	} else {
		// Tanpa info volume: nilai terhadap batas ukuran file bila ada.
		for _, f := range sql.File {
			if f.MaksMB != nil && *f.MaksMB > 0 {
				p := f.UkuranMB / *f.MaksMB * 100
				if k.Sekarang == nil || p > *k.Sekarang {
					k.Sekarang = pf(p)
				}
			}
		}
		if k.Sekarang != nil {
			k.Status = ambil(*k.Sekarang, diskPerhatian, diskKritis)
			k.Rincian = append(k.Rincian, fmt.Sprintf("File database mencapai %s%% dari batas ukuran yang ditetapkan.", fmtAngka(*k.Sekarang, 1)))
		} else {
			k.Status = StatusCukup
			k.Rincian = append(k.Rincian, "Ruang disk volume database tidak dapat dibaca (butuh izin VIEW SERVER STATE) dan file database tidak dibatasi ukurannya, jadi sisa ruang tidak dapat dinilai; cek langsung di server database.")
		}
	}
	k.Rincian = append(k.Rincian, fmt.Sprintf("Ambang: perhatian bila volume terpakai >= %.0f%%, perlu ditambah bila >= %.0f%% atau diperkirakan penuh dalam < %.0f hari.", diskPerhatian, diskKritis, hariPenuhKritis))
	k.Tindakan = tindakan(k.Status, "kapasitas disk server database", "ruang disk database mulai berkurang; rencanakan penambahan atau arsip data lama")
	return k
}

func nilaiDBKoneksi(m Masukan) Kapasitas {
	k := Kapasitas{Kunci: "db_koneksi", Nama: "Koneksi database (pool aplikasi)", Kelompok: KelompokResource, Satuan: "koneksi", Status: StatusTakAdaData}
	p := m.DB.Utama
	if p == nil || p.Batas <= 0 {
		k.Rincian = []string{"Pool koneksi database belum dapat dibaca."}
		k.Tindakan = tindakan(k.Status, "", "")
		return k
	}
	k.Batas = pf(float64(p.Batas))
	k.Sekarang = pf(float64(p.Dipakai))
	puncak := float64(p.Dipakai)
	var tunggu int64
	if m.H24 != nil && m.H24.DBDipakaiMaks != nil {
		k.Puncak24j = pf(float64(*m.H24.DBDipakaiMaks))
		puncak = math.Max(puncak, float64(*m.H24.DBDipakaiMaks))
		tunggu = m.H24.DBTungguTotal
	}
	if m.H7 != nil && m.H7.DBDipakaiMaks != nil {
		k.Puncak7h = pf(float64(*m.H7.DBDipakaiMaks))
	}
	rasio := puncak / float64(p.Batas) * 100
	k.Status = ambil(rasio, poolPerhatian, poolKritis)
	if tunggu > 0 {
		// Permintaan yang pernah harus menunggu koneksi berarti pool sempat habis, walau hanya sesaat.
		k.Status = lebihBuruk(k.Status, StatusPerhatian)
		if rasio >= poolKritis {
			k.Status = StatusKritis
		}
	}
	k.Rincian = append(k.Rincian, fmt.Sprintf("Koneksi dipakai sekarang %d dari batas %d · puncak 24 jam %s (%s%% dari batas).", p.Dipakai, p.Batas, nilaiAtau(k.Puncak24j), fmtAngka(rasio, 0)))
	k.Rincian = append(k.Rincian, fmt.Sprintf("Permintaan yang harus menunggu koneksi bebas: %d dalam 24 jam terakhir (%d sejak aplikasi mulai, total waktu tunggu %s ms).", tunggu, p.Menunggu, fmtAngka(p.MenungguMS, 0)))
	k.Rincian = append(k.Rincian, fmt.Sprintf("Ambang: perhatian bila puncak >= %.0f%% dari batas atau ada permintaan menunggu, perlu ditambah bila puncak >= %.0f%% dan ada yang menunggu.", poolPerhatian, poolKritis))
	k.Tindakan = tindakan(k.Status, fmt.Sprintf("batas koneksi database (SetMaxOpenConns, saat ini %d) bila server database masih longgar; periksa juga query yang menahan koneksi lama", p.Batas), "pool koneksi cukup padat; periksa query yang lambat")
	return k
}

func nilaiMemoriSQL(m Masukan) Kapasitas {
	k := Kapasitas{Kunci: "db_memori", Nama: "Memori SQL Server", Kelompok: KelompokResource, Satuan: "detik (PLE)", Status: StatusTakAdaData}
	s := m.DB.SQL
	if s == nil || (s.PLE == nil && s.BufferHitRatio == nil) {
		k.Rincian = []string{"Page life expectancy belum dapat dibaca (butuh izin VIEW SERVER STATE pada akun database aplikasi), sehingga tekanan memori SQL Server tidak dapat dinilai dari sini."}
		k.Tindakan = tindakan(k.Status, "", "")
		return k
	}
	k.Status = StatusCukup
	if s.PLE != nil {
		k.Sekarang = s.PLE
		switch {
		case *s.PLE < pleKritis:
			k.Status = StatusKritis
		case *s.PLE < plePerhatian:
			k.Status = StatusPerhatian
		}
		k.Rincian = append(k.Rincian, fmt.Sprintf("Page life expectancy %s detik: berapa lama halaman data bertahan di memori sebelum tergusur. Makin kecil, makin sering SQL Server membaca ulang dari disk.", fmtAngka(*s.PLE, 0)))
	}
	if s.BufferHitRatio != nil {
		if *s.BufferHitRatio < hitPerhatian {
			k.Status = lebihBuruk(k.Status, StatusPerhatian)
		}
		k.Rincian = append(k.Rincian, fmt.Sprintf("Buffer cache hit ratio %s%% (permintaan data yang terlayani dari memori).", fmtAngka(*s.BufferHitRatio, 1)))
	}
	if s.MemFisikMB != nil {
		k.Rincian = append(k.Rincian, "Memori server database "+fmtMB(*s.MemFisikMB)+func() string {
			if s.MemProsesMB != nil {
				return ", dipakai SQL Server " + fmtMB(*s.MemProsesMB) + "."
			}
			return "."
		}())
	}
	k.Rincian = append(k.Rincian, fmt.Sprintf("Ambang: perhatian bila PLE < %.0f detik atau hit ratio < %.0f%%, perlu ditambah bila PLE < %.0f detik.", plePerhatian, hitPerhatian, pleKritis))
	k.Tindakan = tindakan(k.Status, "RAM server database (atau kurangi data yang dipindai query)", "memori SQL Server mulai ketat setelah beban berat; pantau saat jam sibuk")
	return k
}

func nilaiRespons(m Masukan) Kapasitas {
	k := Kapasitas{Kunci: "respons", Nama: "Waktu respons aplikasi", Kelompok: KelompokKinerja, Satuan: "ms (p95)", Status: StatusTakAdaData}
	j := m.JamTerakhir
	if j.Total < minPermintaan {
		k.Rincian = []string{fmt.Sprintf("Baru %d permintaan pada 1 jam terakhir; dibutuhkan minimal %d untuk menilai waktu respons.", j.Total, minPermintaan)}
		k.Tindakan = tindakan(k.Status, "", "")
		return k
	}
	k.Sekarang = pf(j.P95MS)
	if m.H24 != nil {
		k.Puncak24j = m.H24.LatP95Maks
	}
	k.Status = ambil(j.P95MS, respons95Perhatian, respons95Kritis)
	k.Rincian = append(k.Rincian, fmt.Sprintf("1 jam terakhir: %d permintaan, rata-rata %s ms, p50 %s ms, p95 %s ms, p99 %s ms (ekspor, impor, sinkronisasi, dan penarikan tidak dihitung).",
		j.Total, fmtAngka(j.RataMS, 0), fmtAngka(j.P50MS, 0), fmtAngka(j.P95MS, 0), fmtAngka(j.P99MS, 0)))
	k.Rincian = append(k.Rincian, fmt.Sprintf("Ambang: perhatian bila p95 >= %.0f ms, kritis bila >= %.0f ms. 95%% permintaan selesai lebih cepat dari p95.", respons95Perhatian, respons95Kritis))
	k.Rincian = append(k.Rincian, "Lambat belum tentu karena resource kurang: lihat tabel rute terlambat, dan cocokkan dengan CPU, memori, dan koneksi database di atas.")
	k.Tindakan = tindakan(k.Status, "resource yang jenuh pada saat yang sama (lihat CPU, memori, dan koneksi database), atau optimalkan rute terlambat", "periksa rute terlambat di bawah")
	if k.Status == StatusKritis {
		k.Tindakan = "Lambat: " + strings.TrimPrefix(k.Tindakan, "Perlu ditambah: ")
	}
	return k
}

func nilaiGalat(m Masukan) Kapasitas {
	k := Kapasitas{Kunci: "galat", Nama: "Galat server (5xx)", Kelompok: KelompokKinerja, Satuan: "%", Status: StatusTakAdaData}
	j := m.JamTerakhir
	if j.Total < minPermintaanGalat {
		k.Rincian = []string{fmt.Sprintf("Baru %d permintaan pada 1 jam terakhir; dibutuhkan minimal %d untuk menilai tingkat galat.", j.Total, minPermintaanGalat)}
		k.Tindakan = tindakan(k.Status, "", "")
		return k
	}
	k.Sekarang = pf(j.PersenGalat)
	k.Status = ambil(j.PersenGalat, galatPerhatian, galatKritis)
	k.Rincian = append(k.Rincian, fmt.Sprintf("1 jam terakhir: %d dari %d permintaan berakhir galat server 5xx (%s%%); galat 4xx (permintaan salah/ditolak) %d.", j.Galat5xx, j.Total, fmtAngka(j.PersenGalat, 2), j.Galat4xx))
	k.Rincian = append(k.Rincian, fmt.Sprintf("Ambang: perhatian bila >= %.0f%%, kritis bila >= %.0f%% dari permintaan.", galatPerhatian, galatKritis))
	k.Tindakan = tindakan(k.Status, "—", "lihat rute dengan galat di bawah dan log aplikasi")
	if k.Status == StatusKritis {
		k.Tindakan = "Periksa segera: banyak permintaan berakhir galat server (lihat rute dengan galat dan log aplikasi)"
	}
	return k
}

func nilaiProses(m Masukan) Kapasitas {
	k := Kapasitas{Kunci: "proses", Nama: "Proses aplikasi (memori dan goroutine)", Kelompok: KelompokKinerja, Satuan: "goroutine", Status: StatusCukup}
	k.Sekarang = pf(float64(m.Proses.Goroutine))
	if m.H24 != nil && m.H24.GoroutineMaks != nil {
		k.Puncak24j = pf(float64(*m.H24.GoroutineMaks))
	}
	if m.H7 != nil && m.H7.GoroutineMaks != nil {
		k.Puncak7h = pf(float64(*m.H7.GoroutineMaks))
	}
	puncak := m.Proses.Goroutine
	if m.H24 != nil && m.H24.GoroutineMaks != nil && *m.H24.GoroutineMaks > puncak {
		puncak = *m.H24.GoroutineMaks
	}
	switch {
	case puncak >= goroutineKritis:
		k.Status = StatusKritis
	case puncak >= goroutinePerhatian:
		k.Status = StatusPerhatian
	}
	k.Rincian = append(k.Rincian, fmt.Sprintf("Goroutine sekarang %d (puncak 24 jam %s). Angka yang terus naik tanpa turun menandakan kebocoran.", m.Proses.Goroutine, nilaiAtau(k.Puncak24j)))
	acuan := m.Sistem.MemTotal
	nama := "memori server"
	if m.Sistem.Kontainer != nil && m.Sistem.Kontainer.MemBatas != nil {
		acuan, nama = *m.Sistem.Kontainer.MemBatas, "batas memori container"
	}
	if m.Proses.RSS != nil {
		rss := *m.Proses.RSS
		if m.H24 != nil && m.H24.RSSMaks != nil && *m.H24.RSSMaks > rss {
			rss = *m.H24.RSSMaks
		}
		k.Rincian = append(k.Rincian, fmt.Sprintf("Memori fisik proses aplikasi %s (puncak 24 jam %s); heap Go %s.", FmtBytes(*m.Proses.RSS), FmtBytes(rss), FmtBytes(m.Proses.HeapDipakai)))
		if acuan > 0 {
			p := float64(rss) / float64(acuan) * 100
			k.Status = lebihBuruk(k.Status, ambil(p, rssPerhatian, rssKritis))
			k.Rincian = append(k.Rincian, fmt.Sprintf("Itu %s%% dari %s (%s).", fmtAngka(p, 1), nama, FmtBytes(acuan)))
		}
	} else {
		k.Rincian = append(k.Rincian, fmt.Sprintf("Heap Go %s; memori fisik proses tidak dapat dibaca di platform ini.", FmtBytes(m.Proses.HeapDipakai)))
	}
	k.Rincian = append(k.Rincian, fmt.Sprintf("Ambang: perhatian bila goroutine >= %d atau memori proses >= %.0f%% dari batas, kritis bila >= %d atau >= %.0f%%.", goroutinePerhatian, rssPerhatian, goroutineKritis, rssKritis))
	k.Tindakan = tindakan(k.Status, "memori untuk container/server aplikasi", "periksa kemungkinan kebocoran; mulai ulang aplikasi bila terus naik")
	if k.Status == StatusKritis {
		k.Tindakan = "Periksa segera: goroutine atau memori proses sangat tinggi (kemungkinan kebocoran atau memori kurang)"
	}
	return k
}

func nilaiDBTersambung(m Masukan) Kapasitas {
	k := Kapasitas{Kunci: "db_tersambung", Nama: "Ketersambungan database", Kelompok: KelompokKinerja, Satuan: "ms (ping)", Status: StatusTakAdaData}
	p := m.DB.Utama
	if p == nil {
		k.Tindakan = tindakan(k.Status, "", "")
		return k
	}
	if !p.Tersambung {
		k.Status = StatusKritis
		k.Rincian = append(k.Rincian, "Database aplikasi tidak dapat dihubungi: "+p.Galat)
		k.Tindakan = "Periksa segera: koneksi ke database aplikasi gagal"
		return k
	}
	k.Status = StatusCukup
	k.Sekarang = pf(p.PingMS)
	k.Rincian = append(k.Rincian, fmt.Sprintf("Database aplikasi menjawab dalam %s ms.", fmtAngka(p.PingMS, 1)))
	if p.PingMS >= 250 {
		k.Status = StatusPerhatian
		k.Rincian = append(k.Rincian, "Jawaban database lambat (>= 250 ms untuk satu round-trip): periksa jaringan atau beban server database.")
	}
	if s := m.DB.SLDK; s != nil {
		if s.Tersambung {
			k.Rincian = append(k.Rincian, fmt.Sprintf("SLDK menjawab dalam %s ms.", fmtAngka(s.PingMS, 1)))
		} else {
			k.Status = lebihBuruk(k.Status, StatusPerhatian)
			k.Rincian = append(k.Rincian, "SLDK tidak dapat dihubungi: "+s.Galat+" (sinkronisasi data aset tidak akan berjalan).")
		}
	}
	k.Tindakan = tindakan(k.Status, "—", "periksa koneksi ke server database/SLDK")
	return k
}

func nilaiAudit(m Masukan) Kapasitas {
	k := Kapasitas{Kunci: "audit", Nama: "Pencatatan log audit", Kelompok: KelompokKinerja, Satuan: "entri hilang", Status: StatusCukup}
	a := m.Audit
	hilang := a.Dijatuhkan + a.Gagal
	k.Sekarang = pf(float64(hilang))
	k.Rincian = append(k.Rincian, fmt.Sprintf("Entri diterima %d, tertulis %d, antrean %d dari %d.", a.Diterima, a.Ditulis, a.Antrean, a.Kapasitas))
	if hilang > 0 {
		k.Status = StatusPerhatian
		k.Rincian = append(k.Rincian, fmt.Sprintf("%d entri audit tidak tercatat sejak aplikasi mulai (%d karena antrean penuh, %d karena gagal menulis ke database).", hilang, a.Dijatuhkan, a.Gagal))
	}
	k.Tindakan = tindakan(k.Status, "—", "ada aktivitas yang tidak tercatat; periksa kesehatan database dan log aplikasi")
	return k
}

// Gabungkan menentukan status keseluruhan dan daftar resource server yang perlu ditambah.
func Gabungkan(ks []Kapasitas) (status string, perluDitambah []string) {
	status = StatusTakAdaData
	perluDitambah = []string{}
	for _, k := range ks {
		status = lebihBuruk(status, k.Status)
		if k.Status == StatusKritis && k.Kelompok == KelompokResource {
			perluDitambah = append(perluDitambah, k.Nama)
		}
	}
	return status, perluDitambah
}
