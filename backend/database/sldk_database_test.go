package database

import (
	"testing"

	"pasti-v3-backend/config"
)

// Koneksi sumber data sinkronisasi Digitalisasi Aset. Port 1 di localhost tidak mendengarkan, jadi ping gagal seketika (tanpa menunggu batas waktu).

func pasangCfgSumber(t *testing.T, c config.Config) {
	t.Helper()
	cfgLama, sldkLama, konfLama, martLama := config.Cfg, SLDKDB, SLDKDBTerkonfigurasi, DatamartDB
	config.Cfg = &c
	SLDKDB, SLDKDBTerkonfigurasi, DatamartDB = nil, nil, nil
	t.Cleanup(func() {
		if SLDKDBTerkonfigurasi != nil {
			SLDKDBTerkonfigurasi.Close()
		}
		if DatamartDB != nil {
			DatamartDB.Close()
		}
		config.Cfg, SLDKDB, SLDKDBTerkonfigurasi, DatamartDB = cfgLama, sldkLama, konfLama, martLama
	})
}

func TestConnectSLDKMempertahankanHandleWalauPingGagal(t *testing.T) {
	pasangCfgSumber(t, config.Config{SLDKDBHost: "127.0.0.1", SLDKDBPort: "1", SLDKDBUser: "u", SLDKDBPassword: "p", SLDKDBName: "Interchange"})
	ConnectSLDK()
	if SLDKDB != nil {
		t.Error("SLDKDB hanya boleh terisi bila SLDK terjangkau saat server dimulai")
	}
	if SLDKDBTerkonfigurasi == nil {
		t.Error("handle SLDK harus tetap ada walau ping gagal, supaya sinkronisasi dapat mencoba lagi nanti dan beralih ke datamart")
	}
}

func TestConnectSLDKTanpaHostTidakMembuatHandle(t *testing.T) {
	pasangCfgSumber(t, config.Config{})
	ConnectSLDK()
	if SLDKDB != nil || SLDKDBTerkonfigurasi != nil {
		t.Error("tanpa SLDK_DB_HOST tidak boleh ada handle")
	}
}

func TestConnectDatamartMempertahankanHandleWalauPingGagal(t *testing.T) {
	pasangCfgSumber(t, config.Config{DatamartDBHost: "127.0.0.1", DatamartDBPort: "1", DatamartDBUser: "u", DatamartDBPassword: "p", DatamartDBName: "pasti_datamart"})
	ConnectDatamart()
	if DatamartDB == nil {
		t.Error("handle datamart harus tetap ada walau ping gagal (dicoba lagi saat dipakai)")
	}
}

func TestConnectDatamartTanpaHostTidakMembuatHandle(t *testing.T) {
	pasangCfgSumber(t, config.Config{})
	ConnectDatamart()
	if DatamartDB != nil {
		t.Error("tanpa DATAMART_DB_HOST tidak boleh ada handle")
	}
}
