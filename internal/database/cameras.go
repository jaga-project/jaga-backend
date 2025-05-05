package database

import (
    "context"
    "errors"
    "time"

    "gorm.io/gorm"
)

type Camera struct {
    CameraID  int64     `gorm:"column:camera_id;primaryKey;autoIncrement" json:"camera_id"`
    Name      string    `gorm:"column:name"       json:"name"`
    IPCamera  string    `gorm:"column:ip_camera"  json:"ip_camera"`
    Location  string    `gorm:"column:location"   json:"location"`
    Address   string    `gorm:"column:address"    json:"address"`
    IsActive  bool      `gorm:"column:is_active"  json:"is_active"`
    CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
    UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
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
