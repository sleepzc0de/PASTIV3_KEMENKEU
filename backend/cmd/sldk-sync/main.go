// Command sldk-sync menyiapkan data ringkasan untuk halaman Ringkasan dan Pemantauan (Data Aset SLDK).
//
// Tabel aset SLDK (DJKN.SIMAN2_M_ASET) berisi ~132 juta baris (~206 GB), jadi dashboard tidak bisa menghitung
// langsung ke sana. Perintah ini memindainya SEKALI di luar jam kerja dan menyimpan hasil agregasinya di
// database PASTI. Memakai konfigurasi dan driver yang sama dengan aplikasi (backend/.env).
//
//	pasti-sldk-sync probe                 # periksa SLDK tanpa memindai tabel besar (baca-saja, ringan)
//	pasti-sldk-sync ringkasan -dry-run    # tampilkan rencana dan SQL-nya saja
//	pasti-sldk-sync ringkasan             # pindai dan simpan (meminta konfirmasi)
//
// Di server (image backend sudah memuat biner ini):
//
//	docker compose --env-file deploy.env run --rm --no-deps --entrypoint ./pasti-sldk-sync backend probe
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
)

func usage(w io.Writer) {
	fmt.Fprint(w, `Pemakaian: pasti-sldk-sync <perintah> [opsi]

Perintah:
  probe       Memeriksa SLDK tanpa memindai tabel besar (baca-saja): skema, index, referensi, cakupan K/L,
              dan menguji query agregat pada sampel kecil.
  ringkasan   Memindai tabel aset SLDK satu kali dan menyimpan hasil agregasinya untuk dashboard.

Opsi:
  -kl <kode>      batasi ke satu K/L menurut kodenya (mis. 015); bawaan: SLDK_KL_KODE di .env
  -timeout <dur>  batas waktu (probe bawaan 10m, ringkasan bawaan 3h)

Opsi khusus ringkasan:
  -dry-run        hanya tampilkan rencana dan SQL, tanpa menyentuh SLDK
  -yes            lewati pertanyaan konfirmasi (wajib bila tidak ada terminal, mis. cron)
`)
}

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help" {
		usage(os.Stdout)
		return
	}
	cmd, args := os.Args[1], os.Args[2:]

	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	kl := fs.String("kl", "", "kode K/L")
	timeout := fs.Duration("timeout", 0, "batas waktu")
	dryRun := fs.Bool("dry-run", false, "hanya tampilkan rencana")
	yes := fs.Bool("yes", false, "lewati konfirmasi")

	switch cmd {
	case "probe", "ringkasan":
	default:
		fmt.Fprintf(os.Stderr, "Perintah tidak dikenal: %q\n\n", cmd)
		usage(os.Stderr)
		os.Exit(2)
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	config.LoadConfig()
	cfg := config.Cfg
	if cfg.SLDKAssetTable == "" {
		fmt.Fprintln(os.Stderr, "SLDK_ASSET_TABLE belum dikonfigurasi di .env")
		os.Exit(1)
	}
	scope := strings.TrimSpace(*kl)
	if scope == "" {
		scope = cfg.SLDKKLKode
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "probe":
		d := *timeout
		if d == 0 {
			d = 10 * time.Minute
		}
		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		connectSLDKOrExit()
		runProbe(ctx, database.SLDKDB, probeOptions{AssetTable: cfg.SLDKAssetTable, Scope: scope, Out: os.Stdout})

	case "ringkasan":
		d := *timeout
		if d == 0 {
			d = 3 * time.Hour
		}
		query, _ := aggregatePlan(cfg.SLDKAssetTable, scope)
		scopeText := "seluruh K/L"
		if scope != "" {
			scopeText = "K/L " + scope
		}
		fmt.Printf("Tabel     : %s\nCakupan   : %s\nBatas waktu: %s\n\nSQL agregat:\n%s\n\n", cfg.SLDKAssetTable, scopeText, d, query)
		if *dryRun {
			fmt.Println("(dry-run: tidak ada yang dijalankan)")
			return
		}
		if !*yes && !confirm() {
			fmt.Println("Dibatalkan.")
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		database.Connect()
		connectSLDKOrExit()
		if err := runRingkasan(ctx, database.SLDKDB, database.DB, syncOptions{AssetTable: cfg.SLDKAssetTable, Scope: scope, Out: os.Stdout}); err != nil {
			fmt.Fprintln(os.Stderr, "\nGAGAL:", err)
			os.Exit(1)
		}
	}
}

func connectSLDKOrExit() {
	database.ConnectSLDK()
	if database.SLDKDB == nil {
		fmt.Fprintln(os.Stderr, "Tidak bisa terhubung ke SLDK. Periksa SLDK_DB_* di .env dan whitelist IP server ini di SLDK.")
		os.Exit(1)
	}
}

// confirm meminta persetujuan sebelum pemindaian berat. Tanpa terminal (cron, docker tanpa -it) jawabannya tidak
// bisa dibaca, jadi -yes wajib.
func confirm() bool {
	st, err := os.Stdin.Stat()
	if err != nil || st.Mode()&os.ModeCharDevice == 0 {
		fmt.Fprintln(os.Stderr, "Tidak ada terminal interaktif untuk konfirmasi. Jalankan dengan -yes bila Anda yakin.")
		return false
	}
	fmt.Print("Perintah ini memindai tabel aset SLDK (ratusan GB) dan membebani server SLDK. Jalankan di luar jam kerja.\nKetik 'ya' untuk melanjutkan: ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.EqualFold(strings.TrimSpace(line), "ya")
}
