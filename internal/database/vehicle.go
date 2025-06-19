package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// OwnershipType mendefinisikan tipe kepemilikan kendaraan.
type OwnershipType string

const (
	OwnershipPribadi  OwnershipType = "Pribadi"
	OwnershipKeluarga OwnershipType = "Keluarga"
)

// type Querier interface {
//     ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
//     QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) // Tambahkan ini
//     QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
// }
// Vehicle merepresentasikan struktur data untuk tabel vehicle.
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

// ListVehicles mengambil semua vehicle dari database.
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

// ListVehiclesByUserID mengambil semua vehicle dari database berdasarkan UserID.
func ListVehiclesByUserID(ctx context.Context, db Querier, userID string) ([]Vehicle, error) {
    query := `SELECT vehicle_id, vehicle_name, color, user_id, plate_number, stnk_image_id, kk_image_id, ownership 
              FROM vehicle WHERE user_id=$1 ORDER BY vehicle_id ASC` // Menambahkan ORDER BY untuk konsistensi
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
    // Tidak perlu mengembalikan error jika tidak ada kendaraan, cukup slice kosong.
    return vehicles, nil
}

// GetVehicleByID mengambil satu vehicle berdasarkan ID.
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

// GetVehicleByPlate mengambil satu vehicle berdasarkan nomor plat.
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

// CreateVehicleTx membuat vehicle baru dalam sebuah transaksi.
func CreateVehicleTx(ctx context.Context, tx *sql.Tx, v *Vehicle) error {
	query := `INSERT INTO vehicle (vehicle_name, color, user_id, plate_number, stnk_image_id, kk_image_id, ownership)
              VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING vehicle_id`
	err := tx.QueryRowContext(ctx, query, v.VehicleName, v.Color, v.UserID, v.PlateNumber, v.STNKImageID, v.KKImageID, v.Ownership).Scan(&v.VehicleID)
	if err != nil {
		return fmt.Errorf("error creating vehicle in tx: %w", err)
	}
	return nil
}

// UpdateVehicleTx memperbarui vehicle dalam sebuah transaksi.
// Anda perlu menyesuaikan query builder ini jika Anda memilikinya.
// Contoh ini mengasumsikan Anda membangun query secara dinamis atau memperbarui semua field yang relevan.
func UpdateVehicleTx(ctx context.Context, tx *sql.Tx, id int64, v *Vehicle) error {
	// Contoh query jika semua field yang bisa diubah diupdate:
	// Anda mungkin memiliki logika yang lebih kompleks untuk membangun query ini
	// berdasarkan field mana yang benar-benar diubah.
	var queryBuilder strings.Builder
	var args []interface{}
	argCount := 1

	queryBuilder.WriteString("UPDATE vehicle SET ")

	if v.VehicleName != "" { // Asumsi jika kosong tidak diupdate, atau Anda punya cara lain menandai perubahan
		queryBuilder.WriteString(fmt.Sprintf("vehicle_name=$%d, ", argCount))
		args = append(args, v.VehicleName)
		argCount++
	}
	if v.Color != "" {
		queryBuilder.WriteString(fmt.Sprintf("color=$%d, ", argCount))
		args = append(args, v.Color)
		argCount++
	}
	// UserID biasanya tidak diupdate, tapi jika iya, tambahkan di sini
	if v.PlateNumber != "" {
		queryBuilder.WriteString(fmt.Sprintf("plate_number=$%d, ", argCount))
		args = append(args, v.PlateNumber)
		argCount++
	}
	if v.STNKImageID.Valid || (v.STNKImageID == sql.NullInt64{} && v.STNKImageID.Int64 == 0) { // Memungkinkan set ke NULL
		queryBuilder.WriteString(fmt.Sprintf("stnk_image_id=$%d, ", argCount))
		args = append(args, v.STNKImageID)
		argCount++
	}
	if v.KKImageID.Valid || (v.KKImageID == sql.NullInt64{} && v.KKImageID.Int64 == 0) { // Memungkinkan set ke NULL
		queryBuilder.WriteString(fmt.Sprintf("kk_image_id=$%d, ", argCount))
		args = append(args, v.KKImageID)
		argCount++
	}
	if v.Ownership.Valid || (v.Ownership == sql.NullString{} && v.Ownership.String == "") { // Memungkinkan set ke NULL
		queryBuilder.WriteString(fmt.Sprintf("ownership=$%d, ", argCount))
		args = append(args, v.Ownership)
		argCount++
	}

	// Hapus koma terakhir dan spasi
	finalQuery := strings.TrimSuffix(queryBuilder.String(), ", ")
	if argCount == 1 { // Tidak ada field yang diupdate
		return fmt.Errorf("no fields provided for vehicle update")
	}

	finalQuery += fmt.Sprintf(" WHERE vehicle_id=$%d", argCount)
	args = append(args, id)

	res, err := tx.ExecContext(ctx, finalQuery, args...)
	if err != nil {
		return fmt.Errorf("error updating vehicle in tx: %w", err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected after vehicle update in tx: %w", err)
	}
	if count == 0 {
		return sql.ErrNoRows // Atau error kustom "no vehicle record updated or record not found"
	}
	return nil
}

// DeleteVehicleTx menghapus vehicle dari database.
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
		return sql.ErrNoRows // Atau error kustom "no vehicle record deleted or record not found"
	}
	return nil
}
