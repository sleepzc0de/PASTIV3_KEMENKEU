package models

import "time"

type Employee struct {
	ID                string    `json:"id"`
	SSOSub            string    `json:"sso_sub"`
	NIP               string    `json:"nip"`
	NIP9              string    `json:"nip9"`
	NIK               string    `json:"nik"`
	Name              string    `json:"name"`
	Email             string    `json:"email"`
	PreferredUsername string    `json:"preferred_username"`
	Jabatan           string    `json:"jabatan"`
	JenisJabatan      string    `json:"jenis_jabatan"`
	Satker            string    `json:"satker"`
	KodeSatker        string    `json:"kode_satker"`
	Organisasi        string    `json:"organisasi"`
	KodeOrganisasi    string    `json:"kode_organisasi"`
	KodeKL            string    `json:"kode_kl"`
	NamaKL            string    `json:"nama_kl"`
	PhoneNumber       string    `json:"phone_number"`
	Picture           string    `json:"picture"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
