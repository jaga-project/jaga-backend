package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	StatusLostReportBelumDiproses  = "BELUM_DIPROSES"
	StatusLostReportSedangDiproses = "SEDANG_DIPROSES"
	StatusLostReportSudahDitemukan = "SUDAH_DITEMUKAN"
)

type LostReport struct {
	LostID                int       `json:"lost_id"`
	UserID                string    `json:"user_id"`
	Timestamp             time.Time `json:"timestamp"`
	VehicleID             int       `json:"vehicle_id"`
	Address               string    `json:"address"`
	Status                string    `json:"status"`
	MotorEvidenceImageID  *int64    `json:"motor_evidence_image_id,omitempty"`  // Pointer agar bisa null
	PersonEvidenceImageID *int64    `json:"person_evidence_image_id,omitempty"` // Pointer agar bisa null
}

func CreateLostReportTx(ctx context.Context, tx *sql.Tx, lr *LostReport) error {
	query := `INSERT INTO lost_report (user_id, timestamp, vehicle_id, address, status, motor_evidence_image_id, person_evidence_image_id)
              VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING lost_id`
	err := tx.QueryRowContext(ctx, query, lr.UserID, lr.Timestamp, lr.VehicleID, lr.Address, lr.Status, lr.MotorEvidenceImageID, lr.PersonEvidenceImageID).Scan(&lr.LostID)
	if err != nil {
		return fmt.Errorf("error creating lost report in tx: %w", err)
	}
	return nil
}

func GetLostReportByID(ctx context.Context, db *sql.DB, id int) (*LostReport, error) {
	var lr LostReport
	query := `SELECT lost_id, user_id, timestamp, vehicle_id, address, status, motor_evidence_image_id, person_evidence_image_id 
              FROM lost_report WHERE lost_id = $1`
	err := db.QueryRowContext(ctx, query, id).Scan(
		&lr.LostID, &lr.UserID, &lr.Timestamp, &lr.VehicleID, &lr.Address, &lr.Status, &lr.MotorEvidenceImageID, &lr.PersonEvidenceImageID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("lost_report not found")
		}
		return nil, fmt.Errorf("error getting lost report by ID %d: %w", id, err)
	}
	return &lr, nil
}

func ListLostReports(ctx context.Context, db *sql.DB, statusFilter string) ([]LostReport, error) {
	var queryBuilder strings.Builder
	queryBuilder.WriteString(`SELECT lost_id, user_id, timestamp, vehicle_id, address, status, motor_evidence_image_id, person_evidence_image_id FROM lost_report`)

	var args []interface{}
	paramIndex := 1

	if statusFilter != "" {
		queryBuilder.WriteString(fmt.Sprintf(" WHERE status = $%d", paramIndex))
		args = append(args, statusFilter)
		paramIndex++
	}

	queryBuilder.WriteString(" ORDER BY timestamp DESC")

	rows, err := db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("error listing lost reports: %w", err)
	}
	defer rows.Close()

	var list []LostReport
	for rows.Next() {
		var lr LostReport
		if errScan := rows.Scan(&lr.LostID, &lr.UserID, &lr.Timestamp, &lr.VehicleID, &lr.Address, &lr.Status, &lr.MotorEvidenceImageID, &lr.PersonEvidenceImageID); errScan != nil {
			return nil, fmt.Errorf("error scanning lost report row: %w", errScan)
		}
		list = append(list, lr)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error after iterating lost report rows: %w", err)
	}
	return list, nil
}

func ListLostReportsByUserID(ctx context.Context, db *sql.DB, userID string) ([]LostReport, error) {
    query := `SELECT lost_id, user_id, timestamp, vehicle_id, address, status, motor_evidence_image_id, person_evidence_image_id 
              FROM lost_report 
              WHERE user_id = $1 
              ORDER BY timestamp DESC`

    rows, err := db.QueryContext(ctx, query, userID)
    if err != nil {
        return nil, fmt.Errorf("error listing lost reports for user ID %s: %w", userID, err)
    }
    defer rows.Close()

    var list []LostReport
    for rows.Next() {
        var lr LostReport
        if errScan := rows.Scan(&lr.LostID, &lr.UserID, &lr.Timestamp, &lr.VehicleID, &lr.Address, &lr.Status, &lr.MotorEvidenceImageID, &lr.PersonEvidenceImageID); errScan != nil {
            return nil, fmt.Errorf("error scanning lost report row for user ID %s: %w", userID, errScan)
        }
        list = append(list, lr)
    }
    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("error after iterating lost report rows for user ID %s: %w", userID, err)
    }
    return list, nil
}

func UpdateLostReport(ctx context.Context, db *sql.DB, id int, lr *LostReport) error {
	query := `UPDATE lost_report SET 
                user_id=$1, 
                timestamp=$2, 
                vehicle_id=$3, 
                address=$4, 
                status=$5, 
                motor_evidence_image_id=$6, 
                person_evidence_image_id=$7 
              WHERE lost_id=$8`
	res, err := db.ExecContext(ctx, query, lr.UserID, lr.Timestamp, lr.VehicleID, lr.Address, lr.Status, lr.MotorEvidenceImageID, lr.PersonEvidenceImageID, id)
	if err != nil {
		return fmt.Errorf("error updating lost report ID %d: %w", id, err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected for lost report ID %d update: %w", id, err)
	}
	if count == 0 {
		return errors.New("no lost_report record updated or no changes made")
	}
	return nil
}

func DeleteLostReport(ctx context.Context, db *sql.DB, id int) error {
	res, err := db.ExecContext(ctx, `DELETE FROM lost_report WHERE lost_id = $1`, id)
	if err != nil {
		return fmt.Errorf("error deleting lost report ID %d: %w", id, err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected for lost report ID %d delete: %w", id, err)
	}
	if count == 0 {
		return errors.New("no lost_report record deleted")
	}
	return nil
}
