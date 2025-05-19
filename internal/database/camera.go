package database

import (
    "context"
    "errors"

    "gorm.io/gorm"
)

type Camera struct {
    CameraID   int    `json:"camera_id"` // serial
    Name       string `json:"name"`
    IPCamera   string `json:"ip_camera"`
    Location   string `json:"location"`
    Address    string `json:"address"`
    IsActive   bool   `json:"is_active"`
}

func (Camera) TableName() string {
    return "cameras"
}

func CreateCamera(ctx context.Context, db *gorm.DB, c *Camera) error {
    return db.WithContext(ctx).Create(c).Error
}

func GetCameraByID(ctx context.Context, db *gorm.DB, id int64) (*Camera, error) {
    var cam Camera
    if err := db.WithContext(ctx).
        First(&cam, "camera_id = ?", id).Error; err != nil {
        return nil, err
    }
    return &cam, nil
}

func ListCameras(ctx context.Context, db *gorm.DB, filters map[string]interface{}) ([]Camera, error) {
    var list []Camera
    q := db.WithContext(ctx)
    if len(filters) > 0 {
        q = q.Where(filters)
    }
    if err := q.Find(&list).Error; err != nil {
        return nil, err
    }
    return list, nil
}

func UpdateCamera(ctx context.Context, db *gorm.DB, id int64, updates map[string]interface{}) error {
    res := db.WithContext(ctx).
        Model(&Camera{}).
        Where("camera_id = ?", id).
        Updates(updates)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return errors.New("no camera record updated")
    }
    return nil
}

func DeleteCamera(ctx context.Context, db *gorm.DB, id int64) error {
    res := db.WithContext(ctx).
        Delete(&Camera{}, "camera_id = ?", id)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return errors.New("no camera record deleted")
    }
    return nil
}
