package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type LostReport struct {
	LostID           int       `json:"lost_id"` // serial
	UserID           string    `json:"user_id"` // uuid
	Timestamp        time.Time `json:"timestamp"`
	VehicleID        int       `json:"vehicle_id"`
	Address          string    `json:"address"`
	Status           string    `json:"status"`
	DetectedID       *int      `json:"detected_id,omitempty"`      // Pointer agar bisa null
	EvidenceImageID *int64    `json:"evidence_image_id,omitempty"` // Pointer agar bisa null
}

func CreateLostReport(ctx context.Context, db *sql.DB, lr *LostReport) error {
	query := `INSERT INTO lost_report (user_id, timestamp, vehicle_id, address, status, detected_id, evidence_image_id)
              VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING lost_id`
	return db.QueryRowContext(ctx, query, lr.UserID, lr.Timestamp, lr.VehicleID, lr.Address, lr.Status, lr.DetectedID, lr.EvidenceImageID).Scan(&lr.LostID)
}

func GetLostReportByID(ctx context.Context, db *sql.DB, id int) (*LostReport, error) { // id diubah ke int
	var lr LostReport
	query := `SELECT lost_id, user_id, timestamp, vehicle_id, address, status, detected_id, evidence_image_id FROM lost_report WHERE lost_id = $1`
	err := db.QueryRowContext(ctx, query, id).Scan(
		&lr.LostID, &lr.UserID, &lr.Timestamp, &lr.VehicleID, &lr.Address, &lr.Status, &lr.DetectedID, &lr.EvidenceImageID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("lost_report not found")
		}
		return nil, err
	}
	return &lr, nil
}

func ListLostReports(ctx context.Context, db *sql.DB) ([]LostReport, error) {
	query := `SELECT lost_id, user_id, timestamp, vehicle_id, address, status, detected_id, evidence_image_id FROM lost_report`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []LostReport
	for rows.Next() {
		var lr LostReport
		if err := rows.Scan(&lr.LostID, &lr.UserID, &lr.Timestamp, &lr.VehicleID, &lr.Address, &lr.Status, &lr.DetectedID, &lr.EvidenceImageID); err != nil {
			return nil, err
		}
		list = append(list, lr)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func UpdateLostReport(ctx context.Context, db *sql.DB, id int, lr *LostReport) error { // id diubah ke int
	query := `UPDATE lost_report SET user_id=$1, timestamp=$2, vehicle_id=$3, address=$4, status=$5, detected_id=$6, evidence_image_id=$7 WHERE lost_id=$8`
	res, err := db.ExecContext(ctx, query, lr.UserID, lr.Timestamp, lr.VehicleID, lr.Address, lr.Status, lr.DetectedID, lr.EvidenceImageID, id)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("no lost_report record updated or no changes made")
	}
	return nil
}

func DeleteLostReport(ctx context.Context, db *sql.DB, id int) error { // id diubah ke int
	res, err := db.ExecContext(ctx, `DELETE FROM lost_report WHERE lost_id = $1`, id)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("no lost_report record deleted")
	}
	return nil
}
