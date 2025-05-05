package database

import (
    "context"
    "errors"
    "time"

    "github.com/google/uuid"
    "gorm.io/gorm"
)

type LostReport struct {
    LostID     int64     `gorm:"column:lost_id;primaryKey;autoIncrement" json:"lost_id"`
    UserID     uuid.UUID `gorm:"column:user_id;type:uuid"            json:"user_id"`
    Timestamp  time.Time `gorm:"column:timestamp"                    json:"timestamp"`
    VehicleID  int64     `gorm:"column:vehicle_id"                   json:"vehicle_id"`
    Address    string    `gorm:"column:address"                      json:"address"`
    Status     string    `gorm:"column:status"                       json:"status"`
    DetectedID int64     `gorm:"column:detected_id"                  json:"detected_id"`
}

func (LostReport) TableName() string {
    return "lost_report"
}

func CreateLostReport(ctx context.Context, db *gorm.DB, lr *LostReport) error {
    return db.WithContext(ctx).Create(lr).Error
}

func GetLostReportByID(ctx context.Context, db *gorm.DB, id int64) (*LostReport, error) {
    var lr LostReport
    if err := db.WithContext(ctx).
        First(&lr, "lost_id = ?", id).Error; err != nil {
        return nil, err
    }
    return &lr, nil
}

func ListLostReports(ctx context.Context, db *gorm.DB, filters map[string]interface{}) ([]LostReport, error) {
    var list []LostReport
    q := db.WithContext(ctx)
    if len(filters) > 0 {
        q = q.Where(filters)
    }
    if err := q.Find(&list).Error; err != nil {
        return nil, err
    }
    return list, nil
}

func UpdateLostReport(ctx context.Context, db *gorm.DB, id int64, updates map[string]interface{}) error {
    res := db.WithContext(ctx).
        Model(&LostReport{}).
        Where("lost_id = ?", id).
        Updates(updates)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return errors.New("no lost_report record updated")
    }
    return nil
}

func DeleteLostReport(ctx context.Context, db *gorm.DB, id int64) error {
    res := db.WithContext(ctx).
        Delete(&LostReport{}, "lost_id = ?", id)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return errors.New("no lost_report record deleted")
    }
    return nil
}
