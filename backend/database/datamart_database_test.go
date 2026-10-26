package database

import (
	"testing"

	"pasti-v3-backend/config"
)

// Koneksi datamart sumber sinkronisasi Digitalisasi Aset. Port 1 di localhost tidak mendengarkan, jadi ping gagal seketika (tanpa menunggu batas waktu).

func pasangCfgDatamart(t *testing.T, c config.Config) {
	t.Helper()
	cfgLama, martLama := config.Cfg, DatamartDB
	config.Cfg = &c
	DatamartDB = nil
	t.Cleanup(func() {
		if DatamartDB != nil {
			DatamartDB.Close()
		}
		config.Cfg, DatamartDB = cfgLama, martLama
	})
}

func TestConnectDatamartMempertahankanHandleWalauPingGagal(t *testing.T) {
	pasangCfgDatamart(t, config.Config{DatamartDBHost: "127.0.0.1", DatamartDBPort: "1", DatamartDBUser: "u", DatamartDBPassword: "p", DatamartDBName: "pasti_datamart"})
	ConnectDatamart()
	if DatamartDB == nil {
		t.Error("handle datamart harus tetap ada walau ping gagal (dicoba lagi saat dipakai)")
	}
}

func TestConnectDatamartTanpaHostTidakMembuatHandle(t *testing.T) {
	pasangCfgDatamart(t, config.Config{})
	ConnectDatamart()
	if DatamartDB != nil {
		t.Error("tanpa DATAMART_DB_HOST tidak boleh ada handle")
	}
}
