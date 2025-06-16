package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// OwnershipType adalah tipe untuk merepresentasikan opsi kepemilikan.
type OwnershipType string

const (
	OwnershipPribadi  OwnershipType = "Pribadi"
	OwnershipKeluarga OwnershipType = "Keluarga"
)

type Vehicle struct {
	VehicleID    int64         `json:"vehicle_id"` // Diubah ke int64 untuk konsistensi dengan ID lain
	VehicleName  string        `json:"vehicle_name"`
	Color        string        `json:"color"`
	UserID       string        `json:"user_id"` // Asumsi UserID adalah string (UUID)
	PlateNumber  string        `json:"plate_number"`
	STNKImageID  sql.NullInt64 `json:"stnk_image_id,omitempty"` // Pointer ke int64 atau sql.NullInt64
	KKImageID    sql.NullInt64 `json:"kk_image_id,omitempty"`   // ID untuk gambar Kartu Keluarga
	Ownership    OwnershipType `json:"ownership"`               // Kepemilikan: "Pribadi" atau "Keluarga"
}

// CreateVehicleTx inserts a new vehicle record into the database within a transaction.
func CreateVehicleTx(ctx context.Context, tx *sql.Tx, v *Vehicle) error {
	query := `INSERT INTO vehicle (vehicle_name, color, user_id, plate_number, stnk_image_id, kk_image_id, ownership) 
              VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING vehicle_id`
	err := tx.QueryRowContext(ctx, query, v.VehicleName, v.Color, v.UserID, v.PlateNumber, v.STNKImageID, v.KKImageID, v.Ownership).Scan(&v.VehicleID)
	if err != nil {
		return fmt.Errorf("error creating vehicle in tx: %w", err)
	}
	return nil
}

// CreateVehicle (fungsi asli, bisa dipertimbangkan untuk dihapus jika CreateVehicleTx selalu digunakan dari handler)
// func CreateVehicle(ctx context.Context, db *sql.DB, v *Vehicle) error {
// 	query := `INSERT INTO vehicle (vehicle_name, color, user_id, plate_number, stnk_image_id, kk_image_id, ownership) 
//               VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING vehicle_id`
// 	return db.QueryRowContext(ctx, query, v.VehicleName, v.Color, v.UserID, v.PlateNumber, v.STNKImageID, v.KKImageID, v.Ownership).Scan(&v.VehicleID)
// }

func GetVehicleByID(ctx context.Context, db *sql.DB, id int64) (*Vehicle, error) {
	var v Vehicle
	query := `SELECT vehicle_id, vehicle_name, color, user_id, plate_number, stnk_image_id, kk_image_id, ownership 
              FROM vehicle WHERE vehicle_id = $1`
	err := db.QueryRowContext(ctx, query, id).Scan(
		&v.VehicleID, &v.VehicleName, &v.Color, &v.UserID, &v.PlateNumber, &v.STNKImageID, &v.KKImageID, &v.Ownership,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) { // Lebih baik menggunakan errors.Is
			return nil, errors.New("vehicle not found")
		}
		return nil, fmt.Errorf("error getting vehicle by ID: %w", err)
	}
	return &v, nil
}

func GetVehicleByPlate(ctx context.Context, db *sql.DB, plate string) (*Vehicle, error) {
	var v Vehicle
	query := `SELECT vehicle_id, vehicle_name, color, user_id, plate_number, stnk_image_id, kk_image_id, ownership 
              FROM vehicle WHERE plate_number = $1`
	err := db.QueryRowContext(ctx, query, plate).Scan(
		&v.VehicleID, &v.VehicleName, &v.Color, &v.UserID, &v.PlateNumber, &v.STNKImageID, &v.KKImageID, &v.Ownership,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("vehicle not found")
		}
		return nil, fmt.Errorf("error getting vehicle by plate: %w", err)
	}
	return &v, nil
}

func ListVehicles(ctx context.Context, db *sql.DB) ([]Vehicle, error) {
	query := `SELECT vehicle_id, vehicle_name, color, user_id, plate_number, stnk_image_id, kk_image_id, ownership 
              FROM vehicle`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error listing vehicles: %w", err)
	}
	defer rows.Close()

	var vehicles []Vehicle
	for rows.Next() {
		var v Vehicle
		if err := rows.Scan(&v.VehicleID, &v.VehicleName, &v.Color, &v.UserID, &v.PlateNumber, &v.STNKImageID, &v.KKImageID, &v.Ownership); err != nil {
			return nil, fmt.Errorf("error scanning vehicle row: %w", err)
		}
		vehicles = append(vehicles, v)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error after iterating vehicle rows: %w", err)
	}
	return vehicles, nil
}

// UpdateVehicleTx memperbarui vehicle dalam sebuah transaksi.
func UpdateVehicleTx(ctx context.Context, tx *sql.Tx, id int64, v *Vehicle) error {
    query := `UPDATE vehicle SET vehicle_name=$1, color=$2, user_id=$3, plate_number=$4, stnk_image_id=$5, kk_image_id=$6, ownership=$7
              WHERE vehicle_id=$8`
    res, err := tx.ExecContext(ctx, query, v.VehicleName, v.Color, v.UserID, v.PlateNumber, v.STNKImageID, v.KKImageID, v.Ownership, id)
    if err != nil {
        return fmt.Errorf("error updating vehicle in tx: %w", err)
    }
    count, err := res.RowsAffected()
    if err != nil {
        return fmt.Errorf("error getting rows affected after vehicle update in tx: %w", err)
    }
    if count == 0 {
        return sql.ErrNoRows // Mengembalikan sql.ErrNoRows agar bisa dicek dengan errors.Is
        // return errors.New("no vehicle record updated or vehicle not found") // Alternatif lama
    }
    return nil
}

// func UpdateVehicle(ctx context.Context, db *sql.DB, id int64, v *Vehicle) error {
// 	query := `UPDATE vehicle SET vehicle_name=$1, color=$2, user_id=$3, plate_number=$4, stnk_image_id=$5, kk_image_id=$6, ownership=$7 
//               WHERE vehicle_id=$8`
// 	res, err := db.ExecContext(ctx, query, v.VehicleName, v.Color, v.UserID, v.PlateNumber, v.STNKImageID, v.KKImageID, v.Ownership, id)
// 	if err != nil {
// 		return fmt.Errorf("error updating vehicle: %w", err)
// 	}
// 	count, err := res.RowsAffected()
// 	if err != nil {
// 		return fmt.Errorf("error getting rows affected after vehicle update: %w", err)
// 	}
// 	if count == 0 {
// 		return errors.New("no vehicle record updated or vehicle not found")
// 	}
// 	return nil
// }

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
        return sql.ErrNoRows // Mengembalikan sql.ErrNoRows jika tidak ada baris yang terhapus
        // return errors.New("no vehicle record deleted or vehicle not found") // Alternatif lama
    }
    return nil
}

// func DeleteVehicle(ctx context.Context, db *sql.DB, id int64) error {
// 	// Pertimbangkan apa yang terjadi dengan gambar STNK dan KK jika vehicle dihapus.
// 	// Mungkin Anda ingin menghapus record gambar dan file fisiknya juga.
// 	// Ini memerlukan pengambilan vehicle dulu untuk mendapatkan stnk_image_id dan kk_image_id.
// 	res, err := db.ExecContext(ctx, `DELETE FROM vehicle WHERE vehicle_id=$1`, id)
// 	if err != nil {
// 		return fmt.Errorf("error deleting vehicle: %w", err)
// 	}
// 	count, err := res.RowsAffected()
// 	if err != nil {
// 		return fmt.Errorf("error getting rows affected after vehicle delete: %w", err)
// 	}
// 	if count == 0 {
// 		return errors.New("no vehicle record deleted or vehicle not found")
// 	}
// 	return nil
// }
