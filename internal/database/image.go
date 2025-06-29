package database

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "time"
)

type Image struct {
    ImageID          int64     `json:"image_id"`
    StoragePath      string    `json:"storage_path"`
    FilenameOriginal string    `json:"filename_original,omitempty"`
    MimeType         string    `json:"mime_type,omitempty"`
    SizeBytes        int64     `json:"size_bytes,omitempty"`
    UploadedAt       time.Time `json:"uploaded_at"`
}

type Querier interface {
    ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
    QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) 
    QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

func GetImageStoragePath(ctx context.Context, q Querier, imageID int64) (string, error) {
    query := `SELECT storage_path FROM images WHERE image_id = $1`
    var storagePath string
    err := q.QueryRowContext(ctx, query, imageID).Scan(&storagePath)
    if err != nil {
        if err == sql.ErrNoRows {
            return "", sql.ErrNoRows 
        }
        return "", fmt.Errorf("error getting image storage path for ID %d: %w", imageID, err)
    }
    return storagePath, nil
}

func CreateImageTx(ctx context.Context, tx *sql.Tx, img *Image) error {
    query := `INSERT INTO images (storage_path, filename_original, mime_type, size_bytes, uploaded_at)
              VALUES ($1, $2, $3, $4, NOW()) RETURNING image_id, uploaded_at`
    fmt.Printf("DEBUG DB CreateImageTx: Attempting to insert image. Path: %s, OriginalName: %s, Mime: %s, Size: %d\n", img.StoragePath, img.FilenameOriginal, img.MimeType, img.SizeBytes)
    err := tx.QueryRowContext(ctx, query, img.StoragePath, img.FilenameOriginal, img.MimeType, img.SizeBytes).Scan(&img.ImageID, &img.UploadedAt)
    if err != nil {
        fmt.Printf("ERROR DB CreateImageTx: Failed to insert/scan image: %v\n", err)
        return err
    }
    fmt.Printf("DEBUG DB CreateImageTx: Successfully inserted image. ImageID: %d, UploadedAt: %v\n", img.ImageID, img.UploadedAt)
    return nil
}

func GetImageByID(ctx context.Context, db *sql.DB, id int64) (*Image, error) {
    var img Image
    query := `SELECT image_id, storage_path, filename_original, mime_type, size_bytes, uploaded_at
              FROM images WHERE image_id = $1`
    err := db.QueryRowContext(ctx, query, id).Scan(
        &img.ImageID, &img.StoragePath, &img.FilenameOriginal, &img.MimeType, &img.SizeBytes, &img.UploadedAt,
    )
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) { 
            return nil, errors.New("image not found")
        }
        return nil, fmt.Errorf("error getting image by ID %d: %w", id, err)
    }
    return &img, nil
}

func DeleteImageTx(ctx context.Context, tx *sql.Tx, id int64) error {
    query := `DELETE FROM images WHERE image_id = $1`
    res, err := tx.ExecContext(ctx, query, id)
    if err != nil {
        return fmt.Errorf("error deleting image ID %d in tx: %w", id, err)
    }
    count, err := res.RowsAffected()
    if err != nil {
        return fmt.Errorf("error getting rows affected for image ID %d delete in tx: %w", id, err)
    }
    if count == 0 {
        return errors.New("no image record deleted in tx or image not found")
    }
    return nil
}

func UpdateImageMetadataTx(ctx context.Context, tx *sql.Tx, id int64, newFilenameOriginal string) error {
    query := `UPDATE images SET filename_original = $1 WHERE image_id = $2`
    res, err := tx.ExecContext(ctx, query, newFilenameOriginal, id)
    if err != nil {
        return fmt.Errorf("error updating image metadata for ID %d in tx: %w", id, err)
    }
    count, err := res.RowsAffected()
    if err != nil {
        return fmt.Errorf("error getting rows affected for image ID %d metadata update in tx: %w", id, err)
    }
    if count == 0 {
        return errors.New("no image record updated in tx or image not found")
    }
    return nil
}

func GetImageStoragePathAndDeleteTx(ctx context.Context, tx *sql.Tx, imageID int64) (string, error) {
    var storagePath string

    err := tx.QueryRowContext(ctx, "SELECT storage_path FROM images WHERE image_id = $1", imageID).Scan(&storagePath)
    if err != nil {
        if err == sql.ErrNoRows {
            return "", nil
        }
        return "", fmt.Errorf("failed to get image path before delete (id: %d): %w", imageID, err)
    }

    _, err = tx.ExecContext(ctx, "DELETE FROM images WHERE image_id = $1", imageID)
    if err != nil {
        return "", fmt.Errorf("failed to delete image record in transaction (id: %d): %w", imageID, err)
    }

    return storagePath, nil
}