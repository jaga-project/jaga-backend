package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type User struct {
	UserID    string    `json:"user_id"`    // UUID as string
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Password  string    `json:"password"`
	NIK       string    `json:"nik"`
	KTPPhoto  string    `json:"ktp_photo"`
	CreatedAt time.Time `json:"created_at"`
}

func CreateSingleUser(db *sql.DB, u User, ctx context.Context) error {
	query := `
        INSERT INTO users (user_id, name, email, phone, password, nik, ktp_photo, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
        RETURNING user_id`
	return db.QueryRowContext(ctx, query,
		u.UserID, u.Name, u.Email, u.Phone, u.Password, u.NIK, u.KTPPhoto, u.CreatedAt,
	).Scan(&u.UserID)
}

func CreateManyUser(db *sql.DB, users []User, ctx context.Context) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
        INSERT INTO users (user_id, name, email, phone, password, nik, ktp_photo, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
        RETURNING user_id`
	for i := range users {
		err := tx.QueryRowContext(ctx, query,
			users[i].UserID, users[i].Name, users[i].Email, users[i].Phone, users[i].Password, users[i].NIK, users[i].KTPPhoto, users[i].CreatedAt,
		).Scan(&users[i].UserID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func FindSingleUser(db *sql.DB, query User, ctx context.Context) (*User, error) {
	q := `SELECT user_id, name, email, phone, password, nik, ktp_photo, created_at FROM users WHERE email = $1 LIMIT 1`
	row := db.QueryRowContext(ctx, q, query.Email)
	var u User
	err := row.Scan(&u.UserID, &u.Name, &u.Email, &u.Phone, &u.Password, &u.NIK, &u.KTPPhoto, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	return &u, err
}

func FindManyUser(db *sql.DB, ctx context.Context) ([]User, error) {
	q := `SELECT user_id, name, email, phone, password, nik, ktp_photo, created_at FROM users`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		err := rows.Scan(&u.UserID, &u.Name, &u.Email, &u.Phone, &u.Password, &u.NIK, &u.KTPPhoto, &u.CreatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func UpdateSingleUser(db *sql.DB, userID string, data User, ctx context.Context) error {
	q := `
        UPDATE users SET name=$1, email=$2, phone=$3, password=$4, nik=$5, ktp_photo=$6, created_at=$7
        WHERE user_id=$8`
	_, err := db.ExecContext(ctx, q,
		data.Name, data.Email, data.Phone, data.Password, data.NIK, data.KTPPhoto, data.CreatedAt, userID)
	return err
}

func DeleteSingleUser(db *sql.DB, userID string, ctx context.Context) error {
	q := `DELETE FROM users WHERE user_id=$1`
	res, err := db.ExecContext(ctx, q, userID)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err == nil && count == 0 {
		return errors.New("no matched data")
	}
	return err
}