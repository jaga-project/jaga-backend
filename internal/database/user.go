package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"fmt"
	"strings"
)

type User struct {
	UserID     string    `json:"user_id"`    
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
	Password   string    `json:"password"` 
	NIK        string    `json:"nik"`
	KTPImageID *int64    `json:"ktp_image_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func CreateUserTx(ctx context.Context, tx *sql.Tx, u *User) error {
    query := `
        INSERT INTO users (user_id, name, email, phone, password, nik, ktp_image_id, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`

    var ktpImage sql.NullInt64
    if u.KTPImageID != nil {
        ktpImage = sql.NullInt64{Int64: *u.KTPImageID, Valid: true}
    }

    _, err := tx.ExecContext(ctx, query,
        u.UserID, u.Name, u.Email, u.Phone, u.Password, u.NIK, ktpImage, u.CreatedAt,
    )
    return err
}

func CreateManyUser(db *sql.DB, users []User, ctx context.Context) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() 

	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO users (user_id, name, email, phone, password, nik, ktp_image_id, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`) 
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range users {
		var ktpImage sql.NullInt64
		if users[i].KTPImageID != nil {
			ktpImage = sql.NullInt64{Int64: *users[i].KTPImageID, Valid: true}
		}
		_, err := stmt.ExecContext(ctx,
			users[i].UserID, users[i].Name, users[i].Email, users[i].Phone, users[i].Password, users[i].NIK, ktpImage, users[i].CreatedAt,
		)
		if err != nil {
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
		if errors.Is(err, sql.ErrNoRows) { 
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &u, nil
}

func FindUserByID(db *sql.DB, userID string, ctx context.Context) (*User, error) {
	q := `SELECT user_id, name, email, phone, password, nik, ktp_image_id, created_at FROM users WHERE user_id = $1 LIMIT 1`
	row := db.QueryRowContext(ctx, q, userID)
	var u User
	err := row.Scan(&u.UserID, &u.Name, &u.Email, &u.Phone, &u.Password, &u.NIK, &u.KTPImageID, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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

func UpdateUserTx(ctx context.Context, tx *sql.Tx, userID string, updates map[string]interface{}) error {
    if len(updates) == 0 {
        return errors.New("no fields provided for user update")
    }

    allowedColumns := map[string]bool{
        "name":         true,
        "email":        true,
        "phone":        true,
        "password":     true,
        "nik":          true,
        "ktp_image_id": true,
    }

    var queryBuilder strings.Builder
    args := make([]interface{}, 0, len(updates)+1)
    argCount := 1

    queryBuilder.WriteString("UPDATE users SET ")

    for col, val := range updates {
        if _, ok := allowedColumns[col]; !ok {
            return fmt.Errorf("invalid or forbidden column for update: %s", col)
        }
        queryBuilder.WriteString(fmt.Sprintf("%s = $%d, ", col, argCount))
        args = append(args, val)
        argCount++
    }

    finalQuery := strings.TrimSuffix(queryBuilder.String(), ", ")
    finalQuery += fmt.Sprintf(" WHERE user_id = $%d", argCount)
    args = append(args, userID)

    res, err := tx.ExecContext(ctx, finalQuery, args...)
    if err != nil {
        return fmt.Errorf("error executing user update in tx: %w", err)
    }

    count, err := res.RowsAffected()
    if err != nil {
        return fmt.Errorf("error getting rows affected after user update in tx: %w", err)
    }
    if count == 0 {
        return sql.ErrNoRows 
    }
    return nil
}

func DeleteUserTx(ctx context.Context, tx *sql.Tx, userID string) error {
    q := `DELETE FROM users WHERE user_id=$1`
    res, err := tx.ExecContext(ctx, q, userID)
    if err != nil {
        return fmt.Errorf("error deleting user ID %s in tx: %w", userID, err)
    }
    count, err := res.RowsAffected()
    if err != nil {
        return fmt.Errorf("error getting rows affected for user ID %s delete in tx: %w", userID, err)
    }
    if count == 0 {
        return sql.ErrNoRows
    }
    return nil
}