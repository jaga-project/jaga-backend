package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Detected struct {
	DetectedID       int       `json:"detected_id"`
	CameraID         int       `json:"camera_id"`
	DetectionImageID int64     `json:"detection_image_id"` // Diubah dari ImageURL ke DetectionImageID (BIGINT)
	Timestamp        time.Time `json:"timestamp"`
}

// CreateDetected inserts a new detected record
func CreateDetected(ctx context.Context, db *sql.DB, d *Detected) error {
	query := `INSERT INTO detected (camera_id, detection_image_id, timestamp)
              VALUES ($1, $2, $3) RETURNING detected_id`
	return db.QueryRowContext(ctx, query, d.CameraID, d.DetectionImageID, d.Timestamp).Scan(&d.DetectedID)
}

// GetDetectedByID retrieves a detected record by ID
func GetDetectedByID(ctx context.Context, db *sql.DB, id int) (*Detected, error) {
	var d Detected
	query := `SELECT detected_id, camera_id, detection_image_id, timestamp FROM detected WHERE detected_id = $1`
	err := db.QueryRowContext(ctx, query, id).Scan(&d.DetectedID, &d.CameraID, &d.DetectionImageID, &d.Timestamp)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("detected not found")
		}
		return nil, err
	}
	return &d, nil
}

// ListDetected retrieves all detected records
func ListDetected(ctx context.Context, db *sql.DB) ([]Detected, error) {
	query := `SELECT detected_id, camera_id, detection_image_id, timestamp FROM detected`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var detectedList []Detected
	for rows.Next() {
		var d Detected
		if err := rows.Scan(&d.DetectedID, &d.CameraID, &d.DetectionImageID, &d.Timestamp); err != nil {
			return nil, err
		}
		detectedList = append(detectedList, d)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return detectedList, nil
}

// UpdateDetected updates a detected record by ID
func UpdateDetected(ctx context.Context, db *sql.DB, id int, d *Detected) error {
	query := `UPDATE detected SET camera_id=$1, detection_image_id=$2, timestamp=$3 WHERE detected_id=$4`
	res, err := db.ExecContext(ctx, query, d.CameraID, d.DetectionImageID, d.Timestamp, id)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err // Error saat mengambil RowsAffected
	}
	if count == 0 {
		return errors.New("no detected record updated")
	}
	return nil
}

// DeleteDetected deletes a detected record by ID
func DeleteDetected(ctx context.Context, db *sql.DB, id int) error {
	res, err := db.ExecContext(ctx, `DELETE FROM detected WHERE detected_id=$1`, id)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err // Error saat mengambil RowsAffected
	}
	if count == 0 {
		return errors.New("no detected record deleted")
	}
	return nil
}
