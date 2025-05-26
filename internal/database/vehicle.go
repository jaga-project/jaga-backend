package database

import (
	"context"
	"database/sql"
	"errors"
)

// Vehicle represents a row in the vehicles table.
type Vehicle struct {
    VehicleID    int    `json:"vehicle_id"`
    VehicleName  string `json:"vehicle_name"`
    Color        string `json:"color"`
    UserID       string `json:"user_id"`
    PlateNumber  string `json:"plate_number"`
}

func CreateVehicle(ctx context.Context, db *sql.DB, v *Vehicle) error {
    query := `INSERT INTO vehicle (vehicle_name, color, user_id, plate_number) VALUES ($1, $2, $3, $4) RETURNING vehicle_id`
    return db.QueryRowContext(ctx, query, v.VehicleName, v.Color, v.UserID, v.PlateNumber).Scan(&v.VehicleID)
}

func GetVehicleByID(ctx context.Context, db *sql.DB, id int64) (*Vehicle, error) {
    var v Vehicle
    query := `SELECT vehicle_id, vehicle_name, color, user_id, plate_number FROM vehicle WHERE vehicle_id = $1`
    err := db.QueryRowContext(ctx, query, id).Scan(&v.VehicleID, &v.VehicleName, &v.Color, &v.UserID, &v.PlateNumber)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, errors.New("vehicle not found")
        }
        return nil, err
    }
    return &v, nil
}

func GetVehicleByPlate(ctx context.Context, db *sql.DB, plate string) (*Vehicle, error) {
    var v Vehicle
    query := `SELECT vehicle_id, vehicle_name, color, user_id, plate_number FROM vehicle WHERE plate_number = $1`
    err := db.QueryRowContext(ctx, query, plate).Scan(&v.VehicleID, &v.VehicleName, &v.Color, &v.UserID, &v.PlateNumber)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, errors.New("vehicle not found")
        }
        return nil, err
    }
    return &v, nil
}

func ListVehicles(ctx context.Context, db *sql.DB) ([]Vehicle, error) {
    query := `SELECT vehicle_id, vehicle_name, color, user_id, plate_number FROM vehicle`
    rows, err := db.QueryContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var vehicles []Vehicle
    for rows.Next() {
        var v Vehicle
        if err := rows.Scan(&v.VehicleID, &v.VehicleName, &v.Color, &v.UserID, &v.PlateNumber); err != nil {
            return nil, err
        }
        vehicles = append(vehicles, v)
    }
    return vehicles, nil
}

func UpdateVehicle(ctx context.Context, db *sql.DB, id int64, v *Vehicle) error {
    query := `UPDATE vehicle SET vehicle_name=$1, color=$2, user_id=$3, plate_number=$4 WHERE vehicle_id=$5`
    res, err := db.ExecContext(ctx, query, v.VehicleName, v.Color, v.UserID, v.PlateNumber, id)
    if err != nil {
        return err
    }
    count, err := res.RowsAffected()
    if err == nil && count == 0 {
        return errors.New("no vehicle record updated")
    }
    return err
}

func DeleteVehicle(ctx context.Context, db *sql.DB, id int64) error {
    res, err := db.ExecContext(ctx, `DELETE FROM vehicle WHERE vehicle_id=$1`, id)
    if err != nil {
        return err
    }
    count, err := res.RowsAffected()
    if err == nil && count == 0 {
        return errors.New("no vehicle record deleted")
    }
    return err
}
