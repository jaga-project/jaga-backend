package database

import (
	"context"
	"database/sql"
	"errors"
)

type Camera struct {
    CameraID   int    `json:"camera_id"` // serial
    Name       string `json:"name"`
    IPCamera   string `json:"ip_camera"`
    Location   string `json:"location"`
    Address    string `json:"address"`
    IsActive   bool   `json:"is_active"`
}

func CreateCamera(ctx context.Context, db *sql.DB, c *Camera) error {
    query := `INSERT INTO cameras (name, ip_camera, location, address, is_active)
              VALUES ($1, $2, $3, $4, $5) RETURNING camera_id`
    return db.QueryRowContext(ctx, query, c.Name, c.IPCamera, c.Location, c.Address, c.IsActive).Scan(&c.CameraID)
}

func GetCameraByID(ctx context.Context, db *sql.DB, id int64) (*Camera, error) {
    var cam Camera
    query := `SELECT camera_id, name, ip_camera, location, address, is_active FROM cameras WHERE camera_id = $1`
    err := db.QueryRowContext(ctx, query, id).Scan(
        &cam.CameraID, &cam.Name, &cam.IPCamera, &cam.Location, &cam.Address, &cam.IsActive,
    )
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, errors.New("camera not found")
        }
        return nil, err
    }
    return &cam, nil
}

func ListCameras(ctx context.Context, db *sql.DB) ([]Camera, error) {
    query := `SELECT camera_id, name, ip_camera, location, address, is_active FROM cameras`
    rows, err := db.QueryContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var list []Camera
    for rows.Next() {
        var cam Camera
        if err := rows.Scan(&cam.CameraID, &cam.Name, &cam.IPCamera, &cam.Location, &cam.Address, &cam.IsActive); err != nil {
            return nil, err
        }
        list = append(list, cam)
    }
    return list, nil
}

func UpdateCamera(ctx context.Context, db *sql.DB, id int64, c *Camera) error {
    query := `UPDATE cameras SET name=$1, ip_camera=$2, location=$3, address=$4, is_active=$5 WHERE camera_id=$6`
    res, err := db.ExecContext(ctx, query, c.Name, c.IPCamera, c.Location, c.Address, c.IsActive, id)
    if err != nil {
        return err
    }
    count, err := res.RowsAffected()
    if err == nil && count == 0 {
        return errors.New("no camera record updated")
    }
    return err
}

func DeleteCamera(ctx context.Context, db *sql.DB, id int64) error {
    res, err := db.ExecContext(ctx, `DELETE FROM cameras WHERE camera_id = $1`, id)
    if err != nil {
        return err
    }
    count, err := res.RowsAffected()
    if err == nil && count == 0 {
        return errors.New("no camera record deleted")
    }
    return err
}
