package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

    "fmt"
)

type Suspect struct {
    SuspectID  int64     `json:"suspect_id"`
    DetectedID int64     `json:"detected_id"`
    LostID     int64     `json:"lost_id"`
    Score      float64   `json:"score"`
    Rank       int       `json:"rank"`
    CreatedAt  time.Time `json:"created_at"`
}

type SuspectResult struct {
    SuspectID         int64
    Score        float64
    DetectedTimestamp time.Time
    // Dari tabel 'images'
    EvidenceImagePath sql.NullString
    // Dari tabel 'camera'
    CameraID        int64
    CameraName      string
    CameraLatitude  float64
    CameraLongitude float64
}

// GetSuspectsByLostReportID mengambil semua data suspect yang terkait dengan satu lost_report.
func GetSuspectsByLostReportID(ctx context.Context, db *sql.DB, lostReportID int) ([]SuspectResult, error) {
    query := `
        SELECT 
            s.suspect_id,
            s.score,
            d.timestamp,
            img.storage_path,
            c.camera_id,
            c.name,
            c.latitude,
            c.longitude
        FROM suspect s
        JOIN detected d ON s.detected_id = d.detected_id
        JOIN cameras c ON d.camera_id = c.camera_id
        -- Menggunakan LEFT JOIN untuk gambar, agar jika gambar tidak ada, data tetap muncul
        LEFT JOIN images img ON d.person_image_id = img.image_id
        WHERE s.lost_id = $1
        ORDER BY s.score DESC;
    `

    rows, err := db.QueryContext(ctx, query, lostReportID)
    if err != nil {
        return nil, fmt.Errorf("failed to query suspects for lost report id %d: %w", lostReportID, err)
    }
    defer rows.Close()

    var results []SuspectResult
    for rows.Next() {
        var res SuspectResult
        if err := rows.Scan(
            &res.SuspectID,
            &res.Score,
            &res.DetectedTimestamp,
            &res.EvidenceImagePath,
            &res.CameraID,
            &res.CameraName,
            &res.CameraLatitude,
            &res.CameraLongitude,
        ); err != nil {
            return nil, fmt.Errorf("failed to scan suspect row: %w", err)
        }
        results = append(results, res)
    }

    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("error after iterating suspect rows: %w", err)
    }

    return results, nil
}

func CreateSuspect(ctx context.Context, db *sql.DB, s *Suspect) error {
    query := `INSERT INTO suspect (detected_id, lost_id, score, rank, created_at)
              VALUES ($1, $2, $3, $4, $5) RETURNING suspect_id`
    return db.QueryRowContext(ctx, query, s.DetectedID, s.LostID, s.Score, s.Rank, s.CreatedAt).Scan(&s.SuspectID)
}

func GetSuspectByID(ctx context.Context, db *sql.DB, id int64) (*Suspect, error) {
    var s Suspect
    query := `SELECT suspect_id, detected_id, lost_id, score, rank, created_at FROM suspect WHERE suspect_id = $1`
    err := db.QueryRowContext(ctx, query, id).Scan(&s.SuspectID, &s.DetectedID, &s.LostID, &s.Score, &s.Rank, &s.CreatedAt)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, errors.New("suspect not found")
        }
        return nil, err
    }
    return &s, nil
}

func ListSuspects(ctx context.Context, db *sql.DB) ([]Suspect, error) {
    query := `SELECT suspect_id, detected_id, lost_id, score, rank, created_at FROM suspect`
    rows, err := db.QueryContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var list []Suspect
    for rows.Next() {
        var s Suspect
        if err := rows.Scan(&s.SuspectID, &s.DetectedID, &s.LostID, &s.Score, &s.Rank, &s.CreatedAt); err != nil {
            return nil, err
        }
        list = append(list, s)
    }
    return list, nil
}

func UpdateSuspect(ctx context.Context, db *sql.DB, id int64, s *Suspect) error {
    query := `UPDATE suspect SET detected_id=$1, lost_id=$2, score=$3, rank=$4, created_at=$5 WHERE suspect_id=$6`
    res, err := db.ExecContext(ctx, query, s.DetectedID, s.LostID, s.Score, s.Rank, s.CreatedAt, id)
    if err != nil {
        return err
    }
    count, err := res.RowsAffected()
    if err == nil && count == 0 {
        return errors.New("no suspect record updated")
    }
    return err
}

func DeleteSuspect(ctx context.Context, db *sql.DB, id int64) error {
    res, err := db.ExecContext(ctx, `DELETE FROM suspect WHERE suspect_id = $1`, id)
    if err != nil {
        return err
    }
    count, err := res.RowsAffected()
    if err == nil && count == 0 {
        return errors.New("no suspect record deleted")
    }
    return err
}
