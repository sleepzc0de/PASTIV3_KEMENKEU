package sapa

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemRepo adalah Repo dalam memori. Dipakai oleh tes (paket ini dan handler) supaya aturan bisnis dan lapisan HTTP bisa
// diuji tanpa database; tidak dipakai saat aplikasi berjalan.
type MemRepo struct {
	mu sync.Mutex

	Pengguna []PeranRow            // daftar pengguna yang bisa diberi peran
	Satker   map[string]SatkerInfo // kode 18 digit -> satker (data Digitalisasi Aset)

	peran                       map[string]PeranInfo
	kasus                       map[int64]*KasusInfo
	tahap                       map[int64]map[string]TahapRow
	dokumen                     map[int64]*memDok
	template                    map[string][]*memTpl
	refUE1                      map[string]RefUE1
	bmn                         *RefBMN // diisi DefaultRefBMN() saat pertama dipakai
	urut                        map[int]int
	nextKasus, nextDok, nextTpl int64
}

type memDok struct {
	info   DokumenInfo
	berkas []byte
}

type memTpl struct {
	info   TemplateInfo
	berkas []byte
}

func NewMemRepo() *MemRepo {
	return &MemRepo{
		Satker: map[string]SatkerInfo{}, peran: map[string]PeranInfo{}, kasus: map[int64]*KasusInfo{},
		tahap: map[int64]map[string]TahapRow{}, dokumen: map[int64]*memDok{}, template: map[string][]*memTpl{},
		refUE1: map[string]RefUE1{}, urut: map[int]int{},
	}
}

func (m *MemRepo) PeranPengguna(_ context.Context, userID string) (*PeranInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.peran[userID]; ok {
		return &p, nil
	}
	return nil, nil
}

func (m *MemRepo) DaftarPeran(_ context.Context, q string, limit int) ([]PeranRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	q = strings.ToLower(q)
	out := []PeranRow{}
	for _, u := range m.Pengguna {
		if q != "" && !strings.Contains(strings.ToLower(u.Username+" "+u.Nama+" "+u.Email), q) {
			continue
		}
		if p, ok := m.peran[u.UserID]; ok {
			u.Peran, u.KodeSatker, u.KodeUE1 = p.Peran, p.KodeSatker, p.KodeUE1
		}
		out = append(out, u)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *MemRepo) SimpanPeran(_ context.Context, userID, peran, kodeSatker, kodeUE1, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Pengguna) > 0 {
		ada := false
		for _, u := range m.Pengguna {
			ada = ada || u.UserID == userID
		}
		if !ada {
			return ErrTidakDitemukan
		}
	}
	if peran == "" {
		delete(m.peran, userID)
		return nil
	}
	m.peran[userID] = PeranInfo{Peran: peran, KodeSatker: kodeSatker, KodeUE1: kodeUE1}
	return nil
}

func (m *MemRepo) BuatPenjualan(_ context.Context, in BuatInput) (KasusInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tahun := time.Now().Year()
	m.urut[tahun]++
	m.nextKasus++
	now := time.Now().UTC()
	k := &KasusInfo{ID: m.nextKasus, Noreg: fmt.Sprintf("PJ-%d-%05d", tahun, m.urut[tahun]), KodeSatker: in.KodeSatker, NamaSatker: in.NamaSatker,
		KodeUE1: in.KodeUE1, DibuatOleh: in.Oleh, DibuatPada: now, DiperbaruiPada: now}
	m.kasus[k.ID] = k
	m.tahap[k.ID] = map[string]TahapRow{}
	return *k, nil
}

func (m *MemRepo) AmbilPenjualan(_ context.Context, id int64) (*KasusInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if k, ok := m.kasus[id]; ok {
		c := *k
		return &c, nil
	}
	return nil, nil
}

func (m *MemRepo) statusLocked(id int64) StatusTahap {
	s := StatusTahap{}
	for k, r := range m.tahap[id] {
		s[k] = r.Status
	}
	return s
}

func (m *MemRepo) DaftarPenjualan(_ context.Context, sc Scope, f FilterDaftar) ([]KasusInfo, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var all []KasusInfo
	q := strings.ToLower(f.Q)
	for _, k := range m.kasus {
		switch {
		case sc.TidakAda:
			continue
		case sc.Semua:
		case sc.Kode18 != "":
			if k.KodeSatker != sc.Kode18 {
				continue
			}
		case sc.KodeUE1 != "":
			if k.KodeUE1 != sc.KodeUE1 {
				continue
			}
		default:
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(k.Noreg+" "+k.NamaSatker+" "+k.KodeSatker), q) {
			continue
		}
		selesai := Selesai(m.statusLocked(k.ID))
		if f.Status == "selesai" && !selesai || f.Status == "berjalan" && selesai {
			continue
		}
		all = append(all, *k)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID > all[j].ID })
	total := len(all)
	if f.Offset >= total {
		return []KasusInfo{}, total, nil
	}
	end := total
	if f.Limit > 0 && f.Offset+f.Limit < total {
		end = f.Offset + f.Limit
	}
	return all[f.Offset:end], total, nil
}

func (m *MemRepo) StatusTahapBanyak(_ context.Context, ids []int64) (map[int64]StatusTahap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[int64]StatusTahap{}
	for _, id := range ids {
		out[id] = m.statusLocked(id)
	}
	return out, nil
}

func (m *MemRepo) TahapPenjualan(_ context.Context, id int64) (map[string]TahapRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]TahapRow{}
	for k, r := range m.tahap[id] {
		out[k] = r
	}
	return out, nil
}

func (m *MemRepo) SimpanTahap(_ context.Context, id int64, t TahapRow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.kasus[id]; !ok {
		return fmt.Errorf("usulan %d tidak ada", id)
	}
	m.tahap[id][t.Kunci] = t
	m.kasus[id].DiperbaruiPada = time.Now().UTC()
	return nil
}

func (m *MemRepo) SimpanDokumen(_ context.Context, d DokumenBaru) (DokumenInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextDok++
	info := DokumenInfo{ID: m.nextDok, PenjualanID: d.PenjualanID, Tahap: d.Tahap, Jenis: d.Jenis, JenisLabel: labelJenis(d.Jenis), NamaFile: d.NamaFile, Ukuran: len(d.Berkas),
		Peringatan: d.Peringatan, DibuatOleh: d.Oleh, DibuatPada: time.Now().UTC()}
	m.dokumen[info.ID] = &memDok{info: info, berkas: append([]byte(nil), d.Berkas...)}
	return info, nil
}

func (m *MemRepo) DaftarDokumen(_ context.Context, penjualanID int64) ([]DokumenInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []DokumenInfo{}
	for _, d := range m.dokumen {
		if d.info.PenjualanID == penjualanID {
			out = append(out, d.info)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (m *MemRepo) AmbilDokumen(_ context.Context, id int64) (*DokumenInfo, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.dokumen[id]
	if !ok {
		return nil, nil, nil
	}
	info := d.info
	return &info, append([]byte(nil), d.berkas...), nil
}

func (m *MemRepo) TemplateAktif(_ context.Context, kunci string) ([]byte, *TemplateInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.template[kunci] {
		if t.info.Aktif {
			info := t.info
			return append([]byte(nil), t.berkas...), &info, nil
		}
	}
	return nil, nil, nil
}

func (m *MemRepo) SimpanTemplate(_ context.Context, in TemplateBaru) (TemplateInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.template[in.Kunci] {
		t.info.Aktif = false
	}
	m.nextTpl++
	info := TemplateInfo{ID: m.nextTpl, Kunci: in.Kunci, Versi: len(m.template[in.Kunci]) + 1, NamaFile: in.NamaFile, Ukuran: len(in.Berkas),
		Aktif: true, Catatan: in.Catatan, DiunggahOleh: in.Oleh, DiunggahPada: time.Now().UTC()}
	m.template[in.Kunci] = append(m.template[in.Kunci], &memTpl{info: info, berkas: append([]byte(nil), in.Berkas...)})
	return info, nil
}

func (m *MemRepo) RiwayatTemplate(_ context.Context, kunci string) ([]TemplateInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []TemplateInfo{}
	for i := len(m.template[kunci]) - 1; i >= 0; i-- {
		out = append(out, m.template[kunci][i].info)
	}
	return out, nil
}

func (m *MemRepo) AmbilRefUE1(_ context.Context, kode string) (*RefUE1, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.refUE1[kode]; ok {
		return &r, nil
	}
	return nil, nil
}

func (m *MemRepo) DaftarRefUE1(_ context.Context) ([]RefUE1, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []RefUE1{}
	for _, r := range m.refUE1 {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kode < out[j].Kode })
	return out, nil
}

func (m *MemRepo) SimpanRefUE1(_ context.Context, r RefUE1, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refUE1[r.Kode] = r
	return nil
}

func (m *MemRepo) HapusRefUE1(_ context.Context, kode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.refUE1, kode)
	return nil
}

// ---------------------------------------------------------------- jenis BMN dan satuan

func (m *MemRepo) bmnLocked() *RefBMN {
	if m.bmn == nil {
		d := DefaultRefBMN()
		m.bmn = &d
	}
	return m.bmn
}

func (m *MemRepo) AmbilRefBMN(_ context.Context) (RefBMN, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.bmnLocked()
	out := RefBMN{Jenis: make([]JenisBMN, len(r.Jenis)), Satuan: append([]SatuanBMN{}, r.Satuan...)}
	for i, j := range r.Jenis {
		j.Satuan = append([]string{}, j.Satuan...)
		out.Jenis[i] = j
	}
	return out, nil
}

func (m *MemRepo) SimpanSatuanBMN(_ context.Context, s SatuanBMN, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.bmnLocked()
	for i := range r.Satuan {
		if samaNama(r.Satuan[i].Nama, s.Nama) {
			r.Satuan[i] = SatuanBMN{Nama: r.Satuan[i].Nama, Aktif: s.Aktif, Urutan: s.Urutan}
			return nil
		}
	}
	r.Satuan = append(r.Satuan, s)
	return nil
}

func (m *MemRepo) HapusSatuanBMN(_ context.Context, nama string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.bmnLocked()
	var sat []SatuanBMN
	for _, s := range r.Satuan {
		if !samaNama(s.Nama, nama) {
			sat = append(sat, s)
		}
	}
	r.Satuan = sat
	for i := range r.Jenis {
		var ks []string
		for _, s := range r.Jenis[i].Satuan {
			if !samaNama(s, nama) {
				ks = append(ks, s)
			}
		}
		r.Jenis[i].Satuan = ks
	}
	return nil
}

func (m *MemRepo) SimpanJenisBMN(_ context.Context, j JenisBMN, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.bmnLocked()
	j.Satuan = append([]string{}, j.Satuan...)
	for i := range r.Jenis {
		if samaNama(r.Jenis[i].Nama, j.Nama) {
			j.Nama = r.Jenis[i].Nama
			r.Jenis[i] = j
			return nil
		}
	}
	r.Jenis = append(r.Jenis, j)
	return nil
}

func (m *MemRepo) HapusJenisBMN(_ context.Context, nama string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.bmnLocked()
	var out []JenisBMN
	for _, j := range r.Jenis {
		if !samaNama(j.Nama, nama) {
			out = append(out, j)
		}
	}
	r.Jenis = out
	return nil
}

func (m *MemRepo) CariSatker(_ context.Context, kode18 string) (*SatkerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.Satker[kode18]; ok {
		return &s, nil
	}
	return nil, nil
}
