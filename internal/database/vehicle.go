package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type OwnershipType string

const (
	OwnershipPribadi  OwnershipType = "Pribadi"
	OwnershipKeluarga OwnershipType = "Keluarga"
)

type Vehicle struct {
	VehicleID   int64          `json:"vehicle_id"`
	VehicleName string         `json:"vehicle_name"`
	Color       string         `json:"color"`
	UserID      string         `json:"user_id"`
	PlateNumber string         `json:"plate_number"`
	STNKImageID sql.NullInt64  `json:"stnk_image_id"`
	KKImageID   sql.NullInt64  `json:"kk_image_id"`
	Ownership   sql.NullString `json:"ownership"`
}

func ListVehicles(ctx context.Context, db *sql.DB) ([]Vehicle, error) {
	query := `SELECT vehicle_id, vehicle_name, color, user_id, plate_number, stnk_image_id, kk_image_id, ownership FROM vehicle`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying vehicles: %w", err)
	}
	defer rows.Close()
	var vehicles []Vehicle
	for rows.Next() {
		var v Vehicle
		err := rows.Scan(
			&v.VehicleID,
			&v.VehicleName,
			&v.Color,
			&v.UserID,
			&v.PlateNumber,
			&v.STNKImageID,
			&v.KKImageID,
			&v.Ownership,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning vehicle row: %w", err)
		}
		vehicles = append(vehicles, v)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating vehicle rows: %w", err)
	}
	return vehicles, nil
}

func ListVehiclesByUserID(ctx context.Context, db Querier, userID string) ([]Vehicle, error) {
    query := `SELECT vehicle_id, vehicle_name, color, user_id, plate_number, stnk_image_id, kk_image_id, ownership 
              FROM vehicle WHERE user_id=$1 ORDER BY vehicle_id ASC` 
    rows, err := db.QueryContext(ctx, query, userID)
    if err != nil {
        return nil, fmt.Errorf("error querying vehicles by user_id: %w", err)
    }
    defer rows.Close()

    var vehicles []Vehicle
    for rows.Next() {
        var v Vehicle
        err := rows.Scan(
            &v.VehicleID,
            &v.VehicleName,
            &v.Color,
            &v.UserID,
            &v.PlateNumber,
            &v.STNKImageID,
            &v.KKImageID,
            &v.Ownership,
        )
        if err != nil {
            return nil, fmt.Errorf("error scanning vehicle row for user_id %s: %w", userID, err)
        }
        vehicles = append(vehicles, v)
    }
    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating vehicle rows for user_id %s: %w", userID, err)
    }
    return vehicles, nil
}

func GetVehicleByID(ctx context.Context, db Querier, id int64) (*Vehicle, error) {
	query := `SELECT vehicle_id, vehicle_name, color, user_id, plate_number, stnk_image_id, kk_image_id, ownership 
              FROM vehicle WHERE vehicle_id=$1`
	var v Vehicle
	err := db.QueryRowContext(ctx, query, id).Scan(
		&v.VehicleID,
		&v.VehicleName,
		&v.Color,
		&v.UserID,
		&v.PlateNumber,
		&v.STNKImageID,
		&v.KKImageID,
		&v.Ownership,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("vehicle not found")
		}
		return nil, fmt.Errorf("error scanning vehicle by id: %w", err)
	}
	return &v, nil
}

func GetVehicleByPlate(ctx context.Context, db Querier, plateNumber string) (*Vehicle, error) {
	query := `SELECT vehicle_id, vehicle_name, color, user_id, plate_number, stnk_image_id, kk_image_id, ownership 
              FROM vehicle WHERE plate_number=$1`
	var v Vehicle
	err := db.QueryRowContext(ctx, query, plateNumber).Scan(
		&v.VehicleID,
		&v.VehicleName,
		&v.Color,
		&v.UserID,
		&v.PlateNumber,
		&v.STNKImageID,
		&v.KKImageID,
		&v.Ownership,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("vehicle not found")
		}
		return nil, fmt.Errorf("error scanning vehicle by plate: %w", err)
	}
	return &v, nil
}

func CreateVehicleTx(ctx context.Context, tx *sql.Tx, v *Vehicle) error {
	query := `INSERT INTO vehicle (vehicle_name, color, user_id, plate_number, stnk_image_id, kk_image_id, ownership)
              VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING vehicle_id`
	err := tx.QueryRowContext(ctx, query, v.VehicleName, v.Color, v.UserID, v.PlateNumber, v.STNKImageID, v.KKImageID, v.Ownership).Scan(&v.VehicleID)
	if err != nil {
		return fmt.Errorf("error creating vehicle in tx: %w", err)
	}
	return nil
}

func UpdateVehicleTx(ctx context.Context, tx *sql.Tx, id int64, updates map[string]interface{}) error {
    if len(updates) == 0 {
        return fmt.Errorf("no fields provided for vehicle update")
    }

		allowedColumns := map[string]bool{
        "vehicle_name":  true,
        "color":         true,
        "plate_number":  true,
        "stnk_image_id": true,
        "kk_image_id":   true,
        "ownership":     true,
    }

    var queryBuilder strings.Builder
    args := make([]interface{}, 0, len(updates)+1)
    argCount := 1

    queryBuilder.WriteString("UPDATE vehicle SET ")

    for col, val := range updates {
        if _, ok := allowedColumns[col]; !ok {
            return fmt.Errorf("invalid or forbidden column for update: %s", col)
        }
        
        queryBuilder.WriteString(fmt.Sprintf("%s = $%d, ", col, argCount))
        args = append(args, val)
        argCount++
    }

    finalQuery := strings.TrimSuffix(queryBuilder.String(), ", ")
    finalQuery += fmt.Sprintf(" WHERE vehicle_id = $%d", argCount)
    args = append(args, id)

    res, err := tx.ExecContext(ctx, finalQuery, args...)
    if err != nil {
        return fmt.Errorf("error executing vehicle update in tx: %w", err)
    }

    count, err := res.RowsAffected()
    if err != nil {
        return fmt.Errorf("error getting rows affected after vehicle update in tx: %w", err)
    }
    if count == 0 {
        return sql.ErrNoRows 
    }
    return nil
}

func DeleteVehicleTx(ctx context.Context, tx *sql.Tx, id int64) error {
	query := `DELETE FROM vehicle WHERE vehicle_id=$1`
	res, err := tx.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting vehicle in tx: %w", err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected after vehicle delete in tx: %w", err)
	}
	if count == 0 {
		return sql.ErrNoRows 
	}
	return nil
}
