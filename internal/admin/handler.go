package admin

import (
	"mime/multipart"
	"path-api/internal/db"
	"time"
)

type Handler struct {
	db *db.Queries
}

func New(db *db.Queries) *Handler {
	return &Handler{db}
}

type UpdateGuruRequest struct {
	KodeKelas string `json:"kode_kelas" param:"kode_kelas" validate:"required"`
	RoleGuru  bool   `json:"role_guru" validate:"required"`
}

type UploadKalenderRequest struct {
	FileCSV *multipart.FileHeader `form:"file_csv" validate:"required"`
}

type UpdateGuruPayload struct {
	RoleGuru bool `json:"role_guru" validate:"required" example:"true"`
}

type ListAdminResponse struct {
	ID             int32     `json:"id" example:"1"`
	Nama           string    `json:"nama" example:"Admin"`
	Email          string    `json:"email" example:"admin@example.com"`
	Phone          string    `json:"phone" example:"08123456789"`
	ProfilePicture string    `json:"profile_picture" example:"uploads/pfp/admin.jpg"`
	Role           string    `json:"role" example:"admin"`
	Admin          bool      `json:"admin" example:"true"`
	Dibuat         time.Time `json:"dibuat" example:"2026-01-01T07:00:00+07:00"`
}

type ListGuruResponse struct {
	KelasID      int32  `json:"kelas_id" example:"1"`
	NamaKelas    string `json:"nama_kelas" example:"Matematika"`
	PenggunaID   int32  `json:"pengguna_id" example:"2"`
	NamaPengguna string `json:"nama_pengguna" example:"Guru Budi"`
}

type AdminPenggunaItem struct {
	ID             int32    `json:"id" example:"2"`
	Nama           string   `json:"nama" example:"Eko Kusnandar"`
	Email          string   `json:"email" example:"eko@example.com"`
	Phone          string   `json:"phone" example:"08123456789"`
	ProfilePicture string   `json:"profile_picture" example:""`
	Role           string   `json:"role" example:"guru"`
	Kelas          []string `json:"kelas"`
}

type ListGuruMuridResponse struct {
	Guru  []AdminPenggunaItem `json:"guru"`
	Murid []AdminPenggunaItem `json:"murid"`
}

type UpdateRolePenggunaRequest struct {
	PenggunaID int32  `param:"pengguna_id" validate:"required,gt=0"`
	Role       string `json:"role" validate:"required"`
}

type AdminKelasResponse struct {
	ID           int32     `json:"id" example:"1"`
	Nama         string    `json:"nama" example:"X PPLG 1"`
	Deskripsi    string    `json:"deskripsi" example:"Pemrograman Dasar"`
	Tingkat      int16     `json:"tingkat" example:"10"`
	TahunAjaran  string    `json:"tahun_ajaran" example:"2026/2027"`
	KodeKelas    string    `json:"kode_kelas" example:"ABC123"`
	Dibuat       time.Time `json:"dibuat"`
	Pengampu     *int32    `json:"pengampu,omitempty" example:"2"`
	NamaPengampu string    `json:"nama_pengampu" example:"Eko Kusnandar"`
	Guru         bool      `json:"guru" example:"false"`
}

type AdminCreateKelasRequest struct {
	Nama        string `json:"nama" validate:"required"`
	Deskripsi   string `json:"deskripsi"`
	Tingkat     int16  `json:"tingkat" validate:"required,gt=0"`
	TahunAjaran string `json:"tahun_ajaran" validate:"required"`
}

type UploadKelasCSVRequest struct {
	FileCSV *multipart.FileHeader `form:"file_csv" validate:"required"`
}

type UpdatePengampuKelasRequest struct {
	KodeKelas  string `json:"-" param:"kode_kelas" validate:"required"`
	PengampuID *int32 `json:"pengampu_id"`
}

type CreateKalenderAgendaRequest struct {
	NamaKegiatan   string `json:"nama_kegiatan" validate:"required"`
	Keterangan     string `json:"keterangan"`
	TanggalMulai   string `json:"tanggal_mulai" validate:"required"`
	TanggalSelesai string `json:"tanggal_selesai"`
}

type KalenderAgendaResponse struct {
	ID             int32     `json:"id" example:"1"`
	NamaKegiatan   string    `json:"nama_kegiatan" example:"Rapat Orang Tua"`
	Keterangan     string    `json:"keterangan" example:"Pertemuan awal semester"`
	TanggalMulai   string    `json:"tanggal_mulai" example:"2026-11-20"`
	TanggalSelesai string    `json:"tanggal_selesai" example:"2026-11-21"`
	Dibuat         time.Time `json:"dibuat"`
}

