package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Suspect struct {
	SuspectID   int64     `json:"suspect_id"`
	DetectedID  int64     `json:"detected_id"`
	LostID      int64     `json:"lost_id"`
	PersonScore float64   `json:"person_score"`
	MotorScore  float64   `json:"motor_score"`
	FinalScore  float64   `json:"final_score"`
	CreatedAt   time.Time `json:"created_at"`
}

type SuspectResult struct {
	SuspectID         int64
	PersonScore       float64
	MotorScore        float64
	FinalScore        float64
	DetectedTimestamp time.Time
	EvidenceImagePath sql.NullString
	CameraID          int64
	CameraName        string
	CameraLatitude    float64
	CameraLongitude   float64
}

func GetSuspectsByLostReportID(ctx context.Context, db *sql.DB, lostReportID int) ([]SuspectResult, error) {
	query := `
        SELECT
            s.suspect_id,
            s.person_score,
            s.motor_score,
            s.final_score,
            d.timestamp,
            img.storage_path,
            c.camera_id,
            c.name,
            c.latitude,
            c.longitude
        FROM suspect s
        JOIN detected d ON s.detected_id = d.detected_id
        JOIN cameras c ON d.camera_id = c.camera_id
        LEFT JOIN images img ON d.person_image_id = img.image_id OR d.motorcycle_image_id = img.image_id
        WHERE s.lost_id = $1
        ORDER BY s.final_score DESC;
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
			&res.PersonScore,
			&res.MotorScore,
			&res.FinalScore,
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
	query := `INSERT INTO suspect (detected_id, lost_id, person_score, motor_score, final_score, created_at)
              VALUES ($1, $2, $3, $4, $5, $6) RETURNING suspect_id`
	return db.QueryRowContext(ctx, query, s.DetectedID, s.LostID, s.PersonScore, s.MotorScore, s.FinalScore, s.CreatedAt).Scan(&s.SuspectID)
}

func CreateSuspectTx(ctx context.Context, tx *sql.Tx, s *Suspect) error {
    query := `INSERT INTO suspects (lost_id, detected_id, person_score, motor_score, final_score, created_at)
              VALUES ($1, $2, $3, $4, $5, $6) RETURNING suspect_id`
    err := tx.QueryRowContext(ctx, query, s.LostID, s.DetectedID, s.PersonScore, s.MotorScore, s.FinalScore, s.CreatedAt).Scan(&s.SuspectID)
    if err != nil {
        return fmt.Errorf("error creating suspect in transaction: %w", err)
    }
    return nil
}

func CreateManySuspects(ctx context.Context, db *sql.DB, suspects []*Suspect) error {
    if len(suspects) == 0 {
        return nil // Tidak ada yang perlu dilakukan
    }

    // Mulai transaksi
    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    // Pastikan transaksi di-rollback jika ada error
    defer tx.Rollback()

    // Siapkan statement INSERT. Ini lebih efisien daripada membuat query string yang panjang.
    stmt, err := tx.PrepareContext(ctx, `INSERT INTO suspect (detected_id, lost_id, person_score, motor_score, final_score, created_at) VALUES ($1, $2, $3, $4, $5, $6)`)
    if err != nil {
        return fmt.Errorf("failed to prepare statement: %w", err)
    }
    defer stmt.Close()

    // Eksekusi statement untuk setiap suspect
    for _, s := range suspects {
        _, err := stmt.ExecContext(ctx, s.DetectedID, s.LostID, s.PersonScore, s.MotorScore, s.FinalScore, s.CreatedAt)
        if err != nil {
            // Jika ada satu saja yang gagal, seluruh transaksi akan di-rollback.
            return fmt.Errorf("failed to execute statement for suspect with detected_id %d: %w", s.DetectedID, err)
        }
    }

    // Jika semua berhasil, commit transaksi
    return tx.Commit()
}

func GetSuspectByID(ctx context.Context, db *sql.DB, id int64) (*Suspect, error) {
	var s Suspect
	query := `SELECT suspect_id, detected_id, lost_id, person_score, motor_score, final_score, created_at FROM suspect WHERE suspect_id = $1`
	err := db.QueryRowContext(ctx, query, id).Scan(&s.SuspectID, &s.DetectedID, &s.LostID, &s.PersonScore, &s.MotorScore, &s.FinalScore, &s.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("suspect not found")
		}
		return nil, err
	}
	return &s, nil
}

func ListSuspects(ctx context.Context, db *sql.DB) ([]Suspect, error) {
	query := `SELECT suspect_id, detected_id, lost_id, person_score, motor_score, final_score, created_at FROM suspect ORDER BY created_at DESC`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Suspect
	for rows.Next() {
		var s Suspect
		if err := rows.Scan(&s.SuspectID, &s.DetectedID, &s.LostID, &s.PersonScore, &s.MotorScore, &s.FinalScore, &s.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, nil
}

func UpdateSuspect(ctx context.Context, db *sql.DB, id int64, s *Suspect) error {
	query := `UPDATE suspect SET detected_id=$1, lost_id=$2, person_score=$3, motor_score=$4, final_score=$5 WHERE suspect_id=$6`
	res, err := db.ExecContext(ctx, query, s.DetectedID, s.LostID, s.PersonScore, s.MotorScore, s.FinalScore, id)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err == nil && count == 0 {
		return errors.New("no suspect record updated or record not found")
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
		return errors.New("no suspect record deleted or record not found")
	}
	return err
}
