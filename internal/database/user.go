package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type User struct {
	UserID     string    `json:"user_id"`    
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
	Password   string    `json:"password"` 
	NIK        string    `json:"nik"`
	KTPImageID *int64    `json:"ktp_image_id,omitempty"` // Menggunakan pointer agar bisa null
	CreatedAt  time.Time `json:"created_at"`
}

// CreateSingleUserTx inserts a new user record into the database within a transaction.
func CreateSingleUserTx(ctx context.Context, tx *sql.Tx, u *User) error {
	query := `
        INSERT INTO users (user_id, name, email, phone, password, nik, ktp_image_id, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
        RETURNING user_id` // Anda bisa menghapus RETURNING user_id jika tidak perlu mengembalikan ID dari sini

	var ktpImage sql.NullInt64
	if u.KTPImageID != nil {
		ktpImage = sql.NullInt64{Int64: *u.KTPImageID, Valid: true}
	}
	// Jika Anda tidak menggunakan RETURNING user_id:
	_, err := tx.ExecContext(ctx, query,
		u.UserID, u.Name, u.Email, u.Phone, u.Password, u.NIK, ktpImage, u.CreatedAt,
	)
	return err
	// Jika Anda menggunakan RETURNING user_id:
	// return tx.QueryRowContext(ctx, query,
	// 	u.UserID, u.Name, u.Email, u.Phone, u.Password, u.NIK, ktpImage, u.CreatedAt,
	// ).Scan(&u.UserID) // Pastikan u.UserID adalah pointer jika Anda ingin mengupdate nilai di struct u
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
        RETURNING user_id`) // Hapus RETURNING jika tidak digunakan
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range users {
		var ktpImage sql.NullInt64
		if users[i].KTPImageID != nil {
			ktpImage = sql.NullInt64{Int64: *users[i].KTPImageID, Valid: true}
		}

		// Password harus di-hash sebelum disimpan.
		// Jika tidak menggunakan RETURNING:
		_, err := stmt.ExecContext(ctx,
			users[i].UserID, users[i].Name, users[i].Email, users[i].Phone, users[i].Password, users[i].NIK, ktpImage, users[i].CreatedAt,
		)
		// Jika menggunakan RETURNING:
		// err := stmt.QueryRowContext(ctx,
		// 	users[i].UserID, users[i].Name, users[i].Email, users[i].Phone, users[i].Password, users[i].NIK, ktpImage, users[i].CreatedAt,
		// ).Scan(&users[i].UserID)
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
		if errors.Is(err, sql.ErrNoRows) { // Cara yang lebih baik untuk memeriksa sql.ErrNoRows
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

// UpdateSingleUser memperbarui data user.
// Jika Anda ingin field 'updated_at' di database, Anda perlu menambahkannya kembali
// dan mengaturnya di sini, atau menggunakan trigger database.
func UpdateSingleUser(db *sql.DB, userID string, data User, ctx context.Context) error {
	// Query ini mengupdate field yang relevan.
	// Password sebaiknya diupdate melalui fungsi terpisah yang juga menangani hashing.
	// KTPImageID juga mungkin memerlukan logika khusus jika file KTP diubah.
	// Untuk contoh ini, kita asumsikan KTPImageID bisa diupdate langsung.
	q := `
        UPDATE users SET name=$1, email=$2, phone=$3, nik=$4, ktp_image_id=$5
        WHERE user_id=$6` // updated_at dihapus dari SET clause

	var ktpImage sql.NullInt64
	if data.KTPImageID != nil {
		ktpImage = sql.NullInt64{Int64: *data.KTPImageID, Valid: true}
	}

	res, err := db.ExecContext(ctx, q,
		data.Name, data.Email, data.Phone, data.NIK, ktpImage, userID)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("user not found or no rows updated") // Pesan error yang lebih spesifik
	}
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
		return err
	}
	if count == 0 {
		return errors.New("user not found or no rows deleted")
	}
	return nil
}