package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Image struct {
	ImageID          int64     `json:"image_id"`
	StoragePath      string    `json:"storage_path"`
	FilenameOriginal *string   `json:"filename_original,omitempty"` // Pointer agar bisa null
	MimeType         *string   `json:"mime_type,omitempty"`
	SizeBytes        *int64    `json:"size_bytes,omitempty"`
	UploadedAt       time.Time `json:"uploaded_at"`
	// UploaderUserID   *string   `json:"uploader_user_id,omitempty"` // Jika ingin melacak siapa yang mengupload
}

// CreateImage menyisipkan record gambar baru dan mengembalikan ID-nya.
func CreateImage(ctx context.Context, db *sql.DB, img *Image) (int64, error) {
	query := `INSERT INTO images (storage_path, filename_original, mime_type, size_bytes)
              VALUES ($1, $2, $3, $4) RETURNING image_id, uploaded_at`
	// Jika UploaderUserID diaktifkan, tambahkan ke query dan parameter:
	// query := `INSERT INTO images (storage_path, filename_original, mime_type, size_bytes, uploader_user_id)
	//           VALUES ($1, $2, $3, $4, $5) RETURNING image_id, uploaded_at`
	err := db.QueryRowContext(ctx, query,
		img.StoragePath, img.FilenameOriginal, img.MimeType, img.SizeBytes,
		// img.UploaderUserID, // Jika UploaderUserID diaktifkan
	).Scan(&img.ImageID, &img.UploadedAt) // Ambil juga uploaded_at yang di-generate DB
	if err != nil {
		return 0, err
	}
	return img.ImageID, nil
}

func GetImageByID(ctx context.Context, db *sql.DB, id int64) (*Image, error) {
	var img Image
	// Jika UploaderUserID diaktifkan, tambahkan ke SELECT list:
	// query := `SELECT image_id, storage_path, filename_original, mime_type, size_bytes, uploaded_at, uploader_user_id
	//           FROM images WHERE image_id = $1`
	query := `SELECT image_id, storage_path, filename_original, mime_type, size_bytes, uploaded_at
              FROM images WHERE image_id = $1`
	err := db.QueryRowContext(ctx, query, id).Scan(
		&img.ImageID, &img.StoragePath, &img.FilenameOriginal, &img.MimeType, &img.SizeBytes, &img.UploadedAt,
		// &img.UploaderUserID, // Jika UploaderUserID diaktifkan
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("image not found")
		}
		return nil, err
	}
	return &img, nil
}

// Fungsi lain seperti DeleteImage, dll.
// DeleteImage akan berguna jika sebuah gambar tidak lagi direferensikan atau perlu dihapus secara eksplisit.
func DeleteImage(ctx context.Context, db *sql.DB, id int64) error {
	// Penting: Sebelum menghapus dari DB, Anda juga harus menghapus file fisiknya dari storage.
	// Logika itu sebaiknya ada di server handler, bukan di sini.
	// Fungsi ini hanya menghapus record dari database.
	query := `DELETE FROM images WHERE image_id = $1`
	res, err := db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("no image record deleted or image not found")
	}
	return nil
}

// UpdateImage mungkin jarang diperlukan kecuali untuk metadata (misalnya, mengubah filename_original).
// Mengubah storage_path, mime_type, atau size_bytes biasanya berarti file itu sendiri telah berubah,
// yang mungkin lebih cocok ditangani sebagai upload baru dan penghapusan yang lama.
func UpdateImageMetadata(ctx context.Context, db *sql.DB, id int64, newFilenameOriginal string) error {
	query := `UPDATE images SET filename_original = $1 WHERE image_id = $2`
	res, err := db.ExecContext(ctx, query, newFilenameOriginal, id)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("no image record updated or image not found")
	}
	return nil
}