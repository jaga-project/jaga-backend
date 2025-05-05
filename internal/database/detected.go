package database

import (
    "context"
    "errors"
    "time"

    "gorm.io/gorm"
)

type Detected struct {
    DetectedID int64     `gorm:"column:detected_id;primaryKey;autoIncrement" json:"detected_id"`
    VehicleID  int64     `gorm:"column:vehicle_id"                     json:"vehicle_id"`
    CameraID   int64     `gorm:"column:camera_id"                      json:"camera_id"`
    ImageURL   string    `gorm:"column:image_url"                      json:"image_url"`
    Timestamp  time.Time `gorm:"column:timestamp"                      json:"timestamp"`
    IsSuspect  bool      `gorm:"column:is_suspect"                     json:"is_suspect"`
}

func (Detected) TableName() string {
    return "detected"
}

func CreateDetected(ctx context.Context, db *gorm.DB, d *Detected) error {
    return db.WithContext(ctx).Create(d).Error
}

func GetDetectedByID(ctx context.Context, db *gorm.DB, id int64) (*Detected, error) {
    var d Detected
    if err := db.WithContext(ctx).
        First(&d, "detected_id = ?", id).Error; err != nil {
        return nil, err
    }
    return &d, nil
}

func ListDetected(ctx context.Context, db *gorm.DB, filters map[string]interface{}) ([]Detected, error) {
    var list []Detected
    q := db.WithContext(ctx)
    if len(filters) > 0 {
        q = q.Where(filters)
    }
    if err := q.Find(&list).Error; err != nil {
        return nil, err
    }
    return list, nil
}

func UpdateDetected(ctx context.Context, db *gorm.DB, id int64, updates map[string]interface{}) error {
    res := db.WithContext(ctx).
        Model(&Detected{}).
        Where("detected_id = ?", id).
        Updates(updates)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return errors.New("no detected record updated")
    }
    return nil
}

func DeleteDetected(ctx context.Context, db *gorm.DB, id int64) error {
    res := db.WithContext(ctx).
        Delete(&Detected{}, "detected_id = ?", id)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return errors.New("no detected record deleted")
    }
    return nil
}
