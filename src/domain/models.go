package domain

type User struct {
	Username string `json:"username"`
	Password string `json:"-"`
	Enabled  bool   `json:"enabled"`
}

type UserStats struct {
	Username      string `json:"username"`
	UploadBytes   int64  `json:"upload_bytes"`
	DownloadBytes int64  `json:"download_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}
