package dto

type SyncKajiUlangRequest struct {
	KodeKLPD   string `json:"kode_klpd"`
	Tahun      string `json:"tahun" binding:"required"`
	JenisPaket string `json:"jenis_paket"`
}
