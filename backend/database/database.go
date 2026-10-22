package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"pasti-v3-backend/config"

	_ "github.com/microsoft/go-mssqldb"
)

var DB *sql.DB

// Percobaan menyambung ke database saat aplikasi menyala. Database yang ada di server terpisah bisa saja belum siap persis saat container backend menyala (server baru
// restart, jaringan belum naik); mencoba beberapa kali dulu mencegah container jatuh-bangun (crash loop) sebelum akhirnya menyerah.
const (
	maksPercobaanSambung = 12
	jedaPercobaanSambung = 5 * time.Second
)

// aturKolam menerapkan pengaturan kolam koneksi dari konfigurasi. Kolam yang terlalu kecil membuat permintaan menumpuk menunggu koneksi; terlalu besar membebani server
// database. Umur koneksi dibuat pendek karena firewall di antara dua server kerap memutus sambungan yang diam lama, dan sambungan mati itu baru ketahuan saat dipakai.
func aturKolam(db *sql.DB, cfg *config.Config, maksBawaan, idleBawaan int) {
	maks, idle := cfg.DBMaksKoneksi, cfg.DBMaksIdle
	if maks <= 0 {
		maks = maksBawaan
	}
	if idle <= 0 || idle > maks {
		idle = idleBawaan
		if idle > maks {
			idle = maks
		}
	}
	umur, diam := cfg.DBKoneksiMaksUmurMenit, cfg.DBKoneksiMaksIdleMenit
	if umur <= 0 {
		umur = 10
	}
	if diam <= 0 {
		diam = 3
	}
	db.SetMaxOpenConns(maks)
	db.SetMaxIdleConns(idle)
	db.SetConnMaxLifetime(time.Duration(umur) * time.Minute)
	db.SetConnMaxIdleTime(time.Duration(diam) * time.Minute)
}

// TarikKoneksi: string koneksi dengan batas waktu supaya server database yang tidak menjawab tidak menahan permintaan selamanya.
func stringKoneksi(host, user, password, port, name string) string {
	return fmt.Sprintf(
		"server=%s;user id=%s;password=%s;port=%s;database=%s;encrypt=true;TrustServerCertificate=true;dial timeout=10;connection timeout=30;app name=PASTI V3 Backend",
		host, user, password, port, name,
	)
}

func Connect() {
	cfg := config.Cfg

	db, err := sql.Open("sqlserver", stringKoneksi(cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBPort, cfg.DBName))
	if err != nil {
		log.Fatal("[FATAL] Gagal membuat koneksi database:", err)
	}
	aturKolam(db, cfg, 50, 25)

	var pingErr error
	for i := 1; i <= maksPercobaanSambung; i++ {
		ctx, batal := context.WithTimeout(context.Background(), 20*time.Second)
		pingErr = db.PingContext(ctx)
		batal()
		if pingErr == nil {
			break
		}
		if i < maksPercobaanSambung {
			log.Printf("[WARN] Belum bisa terhubung ke SQL Server (percobaan %d/%d), mencoba lagi dalam %s: %v", i, maksPercobaanSambung, jedaPercobaanSambung, pingErr)
			time.Sleep(jedaPercobaanSambung)
		}
	}
	if pingErr != nil {
		log.Fatal("[FATAL] Gagal terhubung ke SQL Server:", pingErr)
	}

	log.Println("[INFO] Berhasil terhubung ke SQL Server 2022:", cfg.DBName)
	// Jati diri server yang sebenarnya dituju (alamat di DB_HOST bisa dipetakan infrastruktur ke server lain): tercatat di docker logs setiap kali backend menyala.
	ctx, batal := context.WithTimeout(context.Background(), 5*time.Second)
	if id, err := BacaIdentitas(ctx, db); err != nil {
		log.Println("[WARN] Identitas server database tidak terbaca:", err)
	} else {
		log.Println("[INFO] Server database:", id.Ringkas())
	}
	batal()
	DB = db
}

// Siap memeriksa apakah database menjawab (dengan batas waktu pendek). Hasilnya disimpan beberapa detik supaya pemeriksa kesehatan yang sering memanggil
// tidak ikut membebani database.
var (
	muSiap       sync.Mutex
	siapTerakhir time.Time
	siapHasil    error
)

const masaSiap = 3 * time.Second

func Siap() error {
	if DB == nil {
		return fmt.Errorf("database belum tersambung")
	}
	muSiap.Lock()
	defer muSiap.Unlock()
	if time.Since(siapTerakhir) < masaSiap {
		return siapHasil
	}
	ctx, batal := context.WithTimeout(context.Background(), 2*time.Second)
	defer batal()
	siapHasil = DB.PingContext(ctx)
	siapTerakhir = time.Now()
	return siapHasil
}
