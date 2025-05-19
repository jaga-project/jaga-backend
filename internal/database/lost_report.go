package database

import (
    "context"
    "errors"
    "time"

    "gorm.io/gorm"
)

type LostReport struct {
    LostID     int       `json:"lost_id"` // serial
    UserID     string    `json:"user_id"` // uuid
    Timestamp  time.Time `json:"timestamp"`
    VehicleID  int       `json:"vehicle_id"`
    Address    string    `json:"address"`
    Status     string    `json:"status"`
    DetectedID int       `json:"detected_id"`
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
