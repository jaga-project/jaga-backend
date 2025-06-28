package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Detected struct {
	DetectedID        int             `json:"detected_id"`
	CameraID          int             `json:"camera_id"`
	PersonImageID     sql.NullInt64   `json:"person_image_id,omitempty"`     // ID gambar deteksi orang
	MotorcycleImageID sql.NullInt64   `json:"motorcycle_image_id,omitempty"` // ID gambar deteksi motor
	Timestamp         time.Time       `json:"timestamp"`
}

// CreateDetected inserts a new detected record (original function, can be kept or removed if CreateDetectedTx is always used)
func CreateDetected(ctx context.Context, db *sql.DB, d *Detected) error {
	query := `INSERT INTO detected (camera_id, person_image_id, motorcycle_image_id, timestamp)
              VALUES ($1, $2, $3, $4) RETURNING detected_id`
	return db.QueryRowContext(ctx, query, d.CameraID, d.PersonImageID, d.MotorcycleImageID, d.Timestamp).Scan(&d.DetectedID)
}

// CreateDetectedTx inserts a new detected record within a transaction
func CreateDetectedTx(ctx context.Context, tx *sql.Tx, d *Detected) error {
	query := `INSERT INTO detected (camera_id, person_image_id, motorcycle_image_id, timestamp)
              VALUES ($1, $2, $3, $4) RETURNING detected_id`
	return tx.QueryRowContext(ctx, query, d.CameraID, d.PersonImageID, d.MotorcycleImageID, d.Timestamp).Scan(&d.DetectedID)
}

// GetDetectedByID retrieves a detected record by ID
func GetDetectedByID(ctx context.Context, db *sql.DB, id int) (*Detected, error) {
	var d Detected
	query := `SELECT detected_id, camera_id, person_image_id, motorcycle_image_id, timestamp 
              FROM detected WHERE detected_id = $1`
	err := db.QueryRowContext(ctx, query, id).Scan(&d.DetectedID, &d.CameraID, &d.PersonImageID, &d.MotorcycleImageID, &d.Timestamp)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) { // Lebih baik menggunakan errors.Is
			return nil, errors.New("detected not found")
		}
		return nil, err
	}
	return &d, nil
}

// ListDetectedByTimestampRange retrieves detected records within a given timestamp range
func ListDetectedByTimestampRange(ctx context.Context, db *sql.DB, startTime, endTime time.Time) ([]Detected, error) {
	query := `SELECT detected_id, camera_id, person_image_id, motorcycle_image_id, timestamp 
              FROM detected 
              WHERE timestamp >= $1 AND timestamp <= $2 
              ORDER BY timestamp DESC` // Atau ASC, sesuai kebutuhan

	rows, err := db.QueryContext(ctx, query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("error querying detected by timestamp range: %w", err)
	}
	defer rows.Close()

	var detectedList []Detected
	for rows.Next() {
		var d Detected
		if err := rows.Scan(&d.DetectedID, &d.CameraID, &d.PersonImageID, &d.MotorcycleImageID, &d.Timestamp); err != nil {
			return nil, fmt.Errorf("error scanning detected record: %w", err)
		}
		detectedList = append(detectedList, d)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error after iterating detected rows: %w", err)
	}
	return detectedList, nil
}

// ListDetectedByCoordinates mengambil daftar deteksi dari kamera yang berada dalam radius tertentu (dalam kilometer)
// dari sebuah titik koordinat yang diberikan.
func ListDetectedByCoordinates(ctx context.Context, db *sql.DB, lat, lon, radiusKm float64) ([]Detected, error) {
	// Query ini menggunakan rumus Haversine untuk menghitung jarak antara dua titik di permukaan bola.
	// 6371 adalah radius rata-rata Bumi dalam kilometer.
	query := `
        SELECT d.detected_id, d.camera_id, d.person_image_id, d.motorcycle_image_id, d.timestamp
        FROM detected d
        JOIN cameras c ON d.camera_id = c.camera_id
        WHERE (
            6371 * acos(
                cos(radians($1)) * cos(radians(c.latitude)) *
                cos(radians(c.longitude) - radians($2)) +
                sin(radians($1)) * sin(radians(c.latitude))
            )
        ) <= $3
        ORDER BY d.timestamp DESC;
    `

	rows, err := db.QueryContext(ctx, query, lat, lon, radiusKm)
	if err != nil {
		return nil, fmt.Errorf("error querying detected by coordinates: %w", err)
	}
	defer rows.Close()

	var detectedList []Detected
	for rows.Next() {
		var d Detected
		if err := rows.Scan(&d.DetectedID, &d.CameraID, &d.PersonImageID, &d.MotorcycleImageID, &d.Timestamp); err != nil {
			return nil, fmt.Errorf("error scanning detected record: %w", err)
		}
		detectedList = append(detectedList, d)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error after iterating detected rows: %w", err)
	}
	return detectedList, nil
}

// ListDetectedByProximityAndTimestamp mengambil daftar deteksi dalam radius dari sebuah titik koordinat DAN dalam rentang waktu tertentu.
func ListDetectedByProximityAndTimestamp(ctx context.Context, db *sql.DB, lat, lon, radiusKm float64, startTime, endTime time.Time) ([]Detected, error) {
	// Query ini menggabungkan rumus Haversine untuk jarak dengan filter rentang waktu.
	// 6371 adalah radius rata-rata Bumi dalam kilometer.
	query := `
        SELECT d.detected_id, d.camera_id, d.person_image_id, d.motorcycle_image_id, d.timestamp
        FROM detected d
        JOIN cameras c ON d.camera_id = c.camera_id
        WHERE 
            d.timestamp BETWEEN $1 AND $2
            AND
            (
                6371 * acos(
                    cos(radians($3)) * cos(radians(c.latitude)) *
                    cos(radians(c.longitude) - radians($4)) +
                    sin(radians($3)) * sin(radians(c.latitude))
                )
            ) <= $5
        ORDER BY d.timestamp DESC;
    `

	rows, err := db.QueryContext(ctx, query, startTime, endTime, lat, lon, radiusKm)
	if err != nil {
		return nil, fmt.Errorf("error querying detected by proximity and timestamp: %w", err)
	}
	defer rows.Close()

	var detectedList []Detected
	for rows.Next() {
		var d Detected
		if err := rows.Scan(&d.DetectedID, &d.CameraID, &d.PersonImageID, &d.MotorcycleImageID, &d.Timestamp); err != nil {
			return nil, fmt.Errorf("error scanning detected record: %w", err)
		}
		detectedList = append(detectedList, d)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error after iterating detected rows: %w", err)
	}
	return detectedList, nil
}

// ListDetected retrieves all detected records (fungsi asli bisa tetap ada)
func ListDetected(ctx context.Context, db *sql.DB) ([]Detected, error) {
	query := `SELECT detected_id, camera_id, person_image_id, motorcycle_image_id, timestamp 
              FROM detected ORDER BY timestamp DESC`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var detectedList []Detected
	for rows.Next() {
		var d Detected
		if err := rows.Scan(&d.DetectedID, &d.CameraID, &d.PersonImageID, &d.MotorcycleImageID, &d.Timestamp); err != nil {
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
	query := `UPDATE detected SET camera_id=$1, person_image_id=$2, motorcycle_image_id=$3, timestamp=$4 
              WHERE detected_id=$5`
	res, err := db.ExecContext(ctx, query, d.CameraID, d.PersonImageID, d.MotorcycleImageID, d.Timestamp, id)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("no detected record updated or record not found")
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
		return err
	}
	if count == 0 {
		return errors.New("no detected record deleted or record not found")
	}
	return nil
}

// DeleteDetectedTx deletes a detected record from the database within a given transaction.
func DeleteDetectedTx(ctx context.Context, tx *sql.Tx, id int) error {
    query := `DELETE FROM detected WHERE detected_id = $1`
    res, err := tx.ExecContext(ctx, query, id)
    if err != nil {
        return fmt.Errorf("error executing delete for detected record ID %d in tx: %w", id, err)
    }

    count, err := res.RowsAffected()
    if err != nil {
        return fmt.Errorf("error getting rows affected for detected ID %d delete in tx: %w", id, err)
    }

    if count == 0 {
        // Menggunakan sql.ErrNoRows konsisten dengan error saat Get tidak menemukan apa-apa.
        // Ini menandakan bahwa tidak ada baris yang dihapus.
        return sql.ErrNoRows
    }
    return nil
}
