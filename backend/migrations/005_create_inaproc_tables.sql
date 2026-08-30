package dto

type InaprocErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details"`
}

type InaprocErrorResponse struct {
	Success bool               `json:"success"`
	Error   InaprocErrorDetail `json:"error"`
}

// InaprocKajiUlangRaw merepresentasikan 1 baris data dari respons Inaproc,
// dipakai untuk unmarshal sebelum disimpan ke database.
type InaprocKajiUlangRaw struct {
	DatamartID      string `json:"datamart_id"`
	TahunAnggaran   string `json:"tahun_anggaran"`
	KdKLPD          string `json:"kd_klpd"`
	NamaKLPD        string `json:"nama_klpd"`
	JenisKLPD       string `json:"jenis_klpd"`
	KdSatker        string `json:"kd_satker"`
	KdSatkerStr     string `json:"kd_satker_str"`
	NamaSatker      string `json:"nama_satker"`
	KdRupLama       string `json:"kd_rup_lama"`
	KdRupBaru       string `json:"kd_rup_baru"`
	JenisPaket      string `json:"jenis_paket"`
	JenisRevisi     string `json:"jenis_revisi"`
	AlasanKajiUlang string `json:"alasan_kajiulang"`
	TglKajiUlang    string `json:"tgl_kaji_ulang"`
	EventDate       string `json:"_event_date"`
	InsertedDate    string `json:"_inserted_date"`
}

type InaprocMeta struct {
	Limit   int    `json:"limit"`
	HasMore bool   `json:"has_more"`
	Cursor  string `json:"cursor"`
}

type InaprocHistoryKajiUlangResponse struct {
	Success bool                   `json:"success"`
	Data    []InaprocKajiUlangRaw  `json:"data"`
	Meta    InaprocMeta            `json:"meta"`
}

type SyncKajiUlangRequest struct {
	KodeKLPD   string `json:"kode_klpd" binding:"required"`
	Tahun      string `json:"tahun" binding:"required"`
	JenisPaket string `json:"jenis_paket"`
}