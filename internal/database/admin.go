package database

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "time"
)

type Admin struct {
    UserID     string    `json:"user_id"`
    AdminLevel int       `json:"admin_level"`
    CreatedAt  time.Time `json:"created_at"`
}

func IsUserAdmin(db *sql.DB, userID string) (bool, error) {
    query := `SELECT EXISTS(SELECT 1 FROM admins WHERE user_id = $1)`
    var isAdmin bool
    err := db.QueryRow(query, userID).Scan(&isAdmin)
    if err != nil {
        if err == sql.ErrNoRows {
            return false, nil
        }
        return false, fmt.Errorf("error checking admin status: %w", err)
    }
    return isAdmin, nil
}

func CreateAdminTx(ctx context.Context, tx *sql.Tx, a *Admin) error {
    query := `INSERT INTO admins (user_id, admin_level, created_at) VALUES ($1, $2, $3)`
    _, err := tx.ExecContext(ctx, query, a.UserID, a.AdminLevel, a.CreatedAt)
    return err
}

func GetAdminByUserID(ctx context.Context, db *sql.DB, userID string) (*Admin, error) {
    var a Admin
    query := `SELECT user_id, admin_level, created_at FROM admins WHERE user_id = $1`
    err := db.QueryRowContext(ctx, query, userID).Scan(&a.UserID, &a.AdminLevel, &a.CreatedAt)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, errors.New("admin not found")
        }
        return nil, err
    }
    return &a, nil
}

func ListAdmin(ctx context.Context, db *sql.DB) ([]Admin, error) {
    query := `SELECT user_id, admin_level, created_at FROM admins`
    rows, err := db.QueryContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var list []Admin
    for rows.Next() {
        var a Admin
        if err := rows.Scan(&a.UserID, &a.AdminLevel, &a.CreatedAt); err != nil {
            return nil, err
        }
        list = append(list, a)
    }
    return list, nil
}

func UpdateAdmin(ctx context.Context, db *sql.DB, userID string, a *Admin) error {
    query := `UPDATE admins SET admin_level=$1, created_at=$2 WHERE user_id=$3`
    res, err := db.ExecContext(ctx, query, a.AdminLevel, a.CreatedAt, userID)
    if err != nil {
        return err
    }
    count, err := res.RowsAffected()
    if err == nil && count == 0 {
        return errors.New("no admin record updated")
    }
    return err
}

func DeleteAdmin(ctx context.Context, db *sql.DB, userID string) error {
    query := `DELETE FROM admins WHERE user_id = $1`
    res, err := db.ExecContext(ctx, query, userID)
    if err != nil {
        return fmt.Errorf("error deleting admin ID %s: %w", userID, err)
    }
    count, err := res.RowsAffected()
    if err != nil {
        return fmt.Errorf("error getting rows affected for admin ID %s delete: %w", userID, err)
    }
    if count == 0 {
        return sql.ErrNoRows 
    }
    return nil
}

func DeleteAdminTx(ctx context.Context, tx *sql.Tx, userID string) error {
    query := `DELETE FROM admins WHERE user_id = $1`
    res, err := tx.ExecContext(ctx, query, userID)
    if err != nil {
        return fmt.Errorf("error executing delete for admin ID %s in tx: %w", userID, err)
    }
    count, err := res.RowsAffected()
    if err != nil {
        return fmt.Errorf("error getting rows affected for admin ID %s delete in tx: %w", userID, err)
    }
    if count == 0 {
        return sql.ErrNoRows 
    }
    return nil
}