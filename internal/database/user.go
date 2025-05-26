package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type User struct {
	UserID     string    `json:"user_id"`    // UUID as string
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
	Password   string    `json:"password"` // Sebaiknya disimpan sebagai hash
	NIK        string    `json:"nik"`
	KTPImageID *int64    `json:"ktp_image_id,omitempty"` // Menggunakan pointer agar bisa null, JSON tag diubah
	CreatedAt  time.Time `json:"created_at"`
}

func CreateSingleUser(db *sql.DB, u User, ctx context.Context) error {
	query := `
        INSERT INTO users (user_id, name, email, phone, password, nik, ktp_image_id, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
        RETURNING user_id`
	// Password harus di-hash sebelum disimpan. Ini hanya contoh query.
	return db.QueryRowContext(ctx, query,
		u.UserID, u.Name, u.Email, u.Phone, u.Password, u.NIK, u.KTPImageID, u.CreatedAt,
	).Scan(&u.UserID)
}

func CreateManyUser(db *sql.DB, users []User, ctx context.Context) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // Rollback jika terjadi error sebelum Commit

	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO users (user_id, name, email, phone, password, nik, ktp_image_id, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
        RETURNING user_id`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range users {
		// Password harus di-hash sebelum disimpan.
		err := stmt.QueryRowContext(ctx,
			users[i].UserID, users[i].Name, users[i].Email, users[i].Phone, users[i].Password, users[i].NIK, users[i].KTPImageID, users[i].CreatedAt,
		).Scan(&users[i].UserID)
		if err != nil {
			// tx.Rollback() sudah di-defer, jadi akan dijalankan jika return error
			return err
		}
	}
	return tx.Commit()
}

func FindSingleUser(db *sql.DB, email string, ctx context.Context) (*User, error) {
	q := `SELECT user_id, name, email, phone, password, nik, ktp_image_id, created_at FROM users WHERE email = $1 LIMIT 1`
	row := db.QueryRowContext(ctx, q, email)
	var u User
	err := row.Scan(&u.UserID, &u.Name, &u.Email, &u.Phone, &u.Password, &u.NIK, &u.KTPImageID, &u.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &u, nil
}

// FindUserByID mengambil user berdasarkan UserID
func FindUserByID(db *sql.DB, userID string, ctx context.Context) (*User, error) {
	q := `SELECT user_id, name, email, phone, password, nik, ktp_image_id, created_at FROM users WHERE user_id = $1 LIMIT 1`
	row := db.QueryRowContext(ctx, q, userID)
	var u User
	err := row.Scan(&u.UserID, &u.Name, &u.Email, &u.Phone, &u.Password, &u.NIK, &u.KTPImageID, &u.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &u, nil
}

func FindManyUser(db *sql.DB, ctx context.Context) ([]User, error) {
	q := `SELECT user_id, name, email, phone, password, nik, ktp_image_id, created_at FROM users`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		err := rows.Scan(&u.UserID, &u.Name, &u.Email, &u.Phone, &u.Password, &u.NIK, &u.KTPImageID, &u.CreatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateSingleUser memperbarui data user.
// Sebaiknya pisahkan update password menjadi fungsi tersendiri.
// Field yang tidak ingin diupdate bisa diabaikan dalam query atau menggunakan COALESCE.
func UpdateSingleUser(db *sql.DB, userID string, data User, ctx context.Context) error {
	// Contoh query yang hanya update field tertentu jika disediakan.
	// Ini memerlukan penyesuaian lebih lanjut berdasarkan field mana yang boleh diupdate.
	// Untuk kesederhanaan, query ini mengupdate semua field yang ada di struct User.
	// Perhatikan bahwa `created_at` biasanya tidak diupdate.
	// `password` juga sebaiknya diupdate melalui proses khusus (misal, dengan verifikasi password lama).
	q := `
        UPDATE users SET name=$1, email=$2, phone=$3, nik=$4, ktp_image_id=$5
        WHERE user_id=$6`
	_, err := db.ExecContext(ctx, q,
		data.Name, data.Email, data.Phone, data.NIK, data.KTPImageID, userID)
	if err != nil {
		return err
	}
	// Anda bisa menambahkan pengecekan RowsAffected jika perlu
	return nil
}

func DeleteSingleUser(db *sql.DB, userID string, ctx context.Context) error {
	q := `DELETE FROM users WHERE user_id=$1`
	res, err := db.ExecContext(ctx, q, userID)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err // Error saat mengambil RowsAffected
	}
	if count == 0 {
		return errors.New("user not found or no rows deleted")
	}
	return nil
}