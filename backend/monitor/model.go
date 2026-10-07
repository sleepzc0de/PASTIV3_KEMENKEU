package monitor

import "time"

// Struktur data yang dikirim ke halaman Monitor Resource. Angka yang tidak dapat diukur di platform ini bernilai nil (bukan nol) supaya tidak dibaca "0 persen".

// Sistem: keadaan server (atau container) tempat aplikasi berjalan.
type Sistem struct {
	OS              string     `json:"os"`
	Arsitektur      string     `json:"arsitektur"`
	CPUJumlah       int        `json:"cpu_jumlah"`
	CPUPersen       *float64   `json:"cpu_persen"`        // pemakaian CPU server 0-100 (rata-rata sejak pengukuran sebelumnya)
	CPUProsesPersen *float64   `json:"cpu_proses_persen"` // CPU proses aplikasi, persen dari satu core (bisa melebihi 100)
	Load1           *float64   `json:"load1"`
	Load5           *float64   `json:"load5"`
	Load15          *float64   `json:"load15"`
	MemTotal        uint64     `json:"mem_total"`
	MemTerpakai     uint64     `json:"mem_terpakai"`
	MemTersedia     uint64     `json:"mem_tersedia"`
	MemPersen       float64    `json:"mem_persen"`
	SwapTotal       uint64     `json:"swap_total"`
	SwapTerpakai    uint64     `json:"swap_terpakai"`
	DiskJalur       string     `json:"disk_jalur"`
	DiskTotal       uint64     `json:"disk_total"`
	DiskTerpakai    uint64     `json:"disk_terpakai"`
	DiskBebas       uint64     `json:"disk_bebas"`
	DiskPersen      float64    `json:"disk_persen"`
	UptimeDetik     *int64     `json:"uptime_detik"` // sejak server menyala
	Kontainer       *Kontainer `json:"kontainer,omitempty"`
	Catatan         []string   `json:"catatan,omitempty"` // bagian yang tidak dapat diukur beserta alasannya
}

// Kontainer: batas resource container (cgroup). Nil bila tidak berjalan di container yang dibatasi.
type Kontainer struct {
	MemBatas *uint64  `json:"mem_batas"` // byte; nil = tanpa batas
	MemPakai *uint64  `json:"mem_pakai"`
	CPUBatas *float64 `json:"cpu_batas"` // jumlah core; nil = tanpa batas
}

// Proses: keadaan proses aplikasi (runtime Go).
type Proses struct {
	VersiGo        string  `json:"versi_go"`
	UptimeDetik    int64   `json:"uptime_detik"`
	Goroutine      int     `json:"goroutine"`
	HeapDipakai    uint64  `json:"heap_dipakai"`
	HeapDariOS     uint64  `json:"heap_dari_os"`
	MemDariOS      uint64  `json:"mem_dari_os"` // total memori yang diminta runtime dari OS
	RSS            *uint64 `json:"rss"`         // memori fisik proses (Linux)
	GCJumlah       uint32  `json:"gc_jumlah"`
	GCJedaTerakhir float64 `json:"gc_jeda_terakhir_ms"`
	FDTerbuka      *int    `json:"fd_terbuka"`
}

// PoolDB: keadaan kolam koneksi database/sql.
type PoolDB struct {
	Nama       string  `json:"nama"`
	Tersambung bool    `json:"tersambung"`
	PingMS     float64 `json:"ping_ms"`
	Batas      int     `json:"batas"` // MaxOpenConnections
	Terbuka    int     `json:"terbuka"`
	Dipakai    int     `json:"dipakai"`
	Menganggur int     `json:"menganggur"`
	Menunggu   int64   `json:"menunggu_total"` // permintaan koneksi yang pernah harus menunggu sejak proses mulai
	MenungguMS float64 `json:"menunggu_ms_total"`
	Galat      string  `json:"galat,omitempty"`
}

// FileDB: satu file database (data atau log).
type FileDB struct {
	Nama       string   `json:"nama"`
	Jenis      string   `json:"jenis"` // ROWS atau LOG
	UkuranMB   float64  `json:"ukuran_mb"`
	TerpakaiMB *float64 `json:"terpakai_mb"`
	MaksMB     *float64 `json:"maks_mb"` // nil = tidak dibatasi
}

// VolumeDB: volume disk tempat file database berada.
type VolumeDB struct {
	Titik  string  `json:"titik"`
	Total  uint64  `json:"total"`
	Bebas  uint64  `json:"bebas"`
	Persen float64 `json:"persen"`
}

// TabelDB: satu tabel menurut ukuran.
type TabelDB struct {
	Nama     string  `json:"nama"`
	Baris    int64   `json:"baris"`
	UkuranMB float64 `json:"ukuran_mb"`
}

// InfoSQL: keadaan SQL Server dan database aplikasi. Bagian yang butuh izin lebih tinggi (VIEW SERVER STATE) dilewati dan dijelaskan di Catatan.
type InfoSQL struct {
	Versi          string     `json:"versi"`
	Edisi          string     `json:"edisi"`
	NamaDB         string     `json:"nama_db"`
	UkuranMB       float64    `json:"ukuran_mb"`   // total file data + log
	TerpakaiMB     *float64   `json:"terpakai_mb"` // ruang terpakai di dalam file
	File           []FileDB   `json:"file"`
	Volume         []VolumeDB `json:"volume"`
	CPUJumlah      *int       `json:"cpu_jumlah"`
	MemFisikMB     *float64   `json:"mem_fisik_mb"`         // memori server database
	MemProsesMB    *float64   `json:"mem_proses_mb"`        // dipakai proses SQL Server
	PLE            *float64   `json:"page_life_expectancy"` // detik; di bawah ~300 tanda tekanan memori
	BufferHitRatio *float64   `json:"buffer_hit_ratio"`     // persen
	SesiPengguna   *int       `json:"sesi_pengguna"`
	Catatan        []string   `json:"catatan,omitempty"`
	DiukurPada     time.Time  `json:"diukur_pada"`
}

// Database: pool koneksi dan keadaan SQL Server.
type Database struct {
	Utama *PoolDB  `json:"utama"`
	SLDK  *PoolDB  `json:"sldk,omitempty"`
	SQL   *InfoSQL `json:"sql,omitempty"`
}

// RuteLambat: statistik satu rute HTTP.
type RuteLambat struct {
	Rute     string  `json:"rute"`
	Metode   string  `json:"metode"`
	Jumlah   int64   `json:"jumlah"`
	Galat5xx int64   `json:"galat_5xx"`
	RataMS   float64 `json:"rata_ms"`
	P95MS    float64 `json:"p95_ms"`
	MaksMS   float64 `json:"maks_ms"`
	Berat    bool    `json:"berat"` // memang berat (ekspor, impor, sinkronisasi, penarikan): tidak dihitung pada p95 keseluruhan
}

// JendelaHTTP: ringkasan permintaan HTTP pada satu jendela waktu.
type JendelaHTTP struct {
	Nama        string  `json:"nama"`
	Total       int64   `json:"total"`
	Galat4xx    int64   `json:"galat_4xx"`
	Galat5xx    int64   `json:"galat_5xx"`
	RataMS      float64 `json:"rata_ms"`
	P50MS       float64 `json:"p50_ms"`
	P95MS       float64 `json:"p95_ms"`
	P99MS       float64 `json:"p99_ms"`
	PerDetik    float64 `json:"per_detik"`
	PersenGalat float64 `json:"persen_galat_5xx"`
}

// HTTP: kinerja permintaan HTTP aplikasi.
type HTTP struct {
	Berjalan   int64         `json:"berjalan"` // permintaan yang sedang diproses
	Jendela    []JendelaHTTP `json:"jendela"`  // 5 menit terakhir, 1 jam terakhir, sejak proses mulai
	RuteLambat []RuteLambat  `json:"rute_lambat"`
	RuteGalat  []RuteLambat  `json:"rute_galat"`
}

// Agregat: ringkasan riwayat (dari monitor_snapshot) pada satu rentang waktu.
type Agregat struct {
	Jumlah         int       `json:"jumlah"` // banyak snapshot
	Dari           time.Time `json:"dari"`
	CPURata        *float64  `json:"cpu_rata"`
	CPUMaks        *float64  `json:"cpu_maks"`
	Load1Maks      *float64  `json:"load1_maks"`
	MemTotal       *uint64   `json:"mem_total"`
	MemMaks        *uint64   `json:"mem_maks"`
	MemPersenMaks  *float64  `json:"mem_persen_maks"`
	SwapMaks       *uint64   `json:"swap_maks"`
	DiskPersenMaks *float64  `json:"disk_persen_maks"`
	RSSMaks        *uint64   `json:"rss_maks"`
	GoroutineMaks  *int      `json:"goroutine_maks"`
	DBDipakaiMaks  *int      `json:"db_dipakai_maks"`
	DBBatas        *int      `json:"db_batas"`
	DBTungguTotal  int64     `json:"db_tunggu_total"`
	LatP95Maks     *float64  `json:"lat_p95_maks"`
	Req5xx         int64     `json:"req_5xx"`
	ReqTotal       int64     `json:"req_total"`
}

// Verdict resource.
const (
	StatusCukup      = "cukup"
	StatusPerhatian  = "perhatian"
	StatusKritis     = "kritis"
	StatusTakAdaData = "tidak_ada_data"
)

// Kelompok penilaian: resource yang bisa ditambah (CPU, RAM, disk, koneksi) dan kinerja aplikasi (respons, galat, proses) yang tidak otomatis diatasi dengan menambah resource.
const (
	KelompokResource = "resource"
	KelompokKinerja  = "kinerja"
)

// TugasLatar: tugas besar di latar belakang yang menjelaskan lonjakan CPU, memori, atau koneksi.
type TugasLatar struct {
	Nama     string `json:"nama"`
	Berjalan bool   `json:"berjalan"`
	Info     string `json:"info,omitempty"`
}

// Kapasitas: penilaian satu resource beserta tindakannya.
type Kapasitas struct {
	Kunci     string   `json:"kunci"`
	Nama      string   `json:"nama"`
	Kelompok  string   `json:"kelompok"`
	Satuan    string   `json:"satuan"`
	Sekarang  *float64 `json:"sekarang"`
	Puncak24j *float64 `json:"puncak_24j"`
	Puncak7h  *float64 `json:"puncak_7h"`
	Rata24j   *float64 `json:"rata_24j"`
	Batas     *float64 `json:"batas"` // kapasitas (mis. 100 untuk persen, atau jumlah koneksi maksimal)
	Status    string   `json:"status"`
	Tindakan  string   `json:"tindakan"` // satu kalimat: perlu ditambah atau tidak
	Rincian   []string `json:"rincian"`  // alasan dan angka pendukung
	Saran     string   `json:"saran,omitempty"`
}

// Ringkasan: seluruh keadaan untuk halaman Monitor Resource.
type Ringkasan struct {
	Waktu         time.Time    `json:"waktu"`
	Status        string       `json:"status"` // terburuk dari semua kapasitas
	PerluDitambah []string     `json:"perlu_ditambah"`
	Sistem        Sistem       `json:"sistem"`
	Proses        Proses       `json:"proses"`
	Database      Database     `json:"database"`
	HTTP          HTTP         `json:"http"`
	Kapasitas     []Kapasitas  `json:"kapasitas"`
	Tugas         []TugasLatar `json:"tugas"`
	Riwayat24j    *Agregat     `json:"riwayat_24j,omitempty"`
	Riwayat7h     *Agregat     `json:"riwayat_7h,omitempty"`
	Pengukuran    struct {
		Aktif       bool      `json:"aktif"`
		IntervalDtk int       `json:"interval_detik"`
		MulaiPada   time.Time `json:"mulai_pada"`
		RetensiHari int       `json:"retensi_hari"`
	} `json:"pengukuran"`
}

// TitikRiwayat: satu titik pada grafik riwayat (rata-rata atau puncak pada jendela itu). Nilai nil = tidak ada data.
type TitikRiwayat struct {
	Waktu      time.Time `json:"waktu"`
	CPURata    *float64  `json:"cpu_rata"`
	CPUMaks    *float64  `json:"cpu_maks"`
	MemPersen  *float64  `json:"mem_persen"`
	DiskPersen *float64  `json:"disk_persen"`
	Load1      *float64  `json:"load1"`
	RSS        *float64  `json:"rss"`
	Heap       *float64  `json:"heap"`
	Goroutine  *float64  `json:"goroutine"`
	DBDipakai  *float64  `json:"db_dipakai"`
	DBTunggu   *float64  `json:"db_tunggu"`
	DBUkuranMB *float64  `json:"db_ukuran_mb"`
	ReqTotal   *float64  `json:"req_total"`
	Req5xx     *float64  `json:"req_5xx"`
	LatRataMS  *float64  `json:"lat_rata_ms"`
	LatP95MS   *float64  `json:"lat_p95_ms"`
}

// Riwayat: deret waktu untuk grafik.
type Riwayat struct {
	Rentang    string         `json:"rentang"`
	Dari       time.Time      `json:"dari"`
	Sampai     time.Time      `json:"sampai"`
	LebarTitik int            `json:"lebar_titik_menit"`
	Titik      []TitikRiwayat `json:"titik"`
}
