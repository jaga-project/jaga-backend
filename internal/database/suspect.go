package database

import (
    "context"
    "errors"
    "time"

    "gorm.io/gorm"
)

type Suspect struct {
    SuspectID  int64     `json:"suspect_id"` // bigserial
    DetectedID int64     `json:"detected_id"`
    LostID     int64     `json:"lost_id"`
    Score      float64   `json:"score"`
    Rank       int       `json:"rank"`
    CreatedAt  time.Time `json:"created_at"`
}

func (Suspect) TableName() string {
    return "suspect"
}

func CreateSuspect(ctx context.Context, db *gorm.DB, s *Suspect) error {
    return db.WithContext(ctx).Create(s).Error
}

func GetSuspectByID(ctx context.Context, db *gorm.DB, id int64) (*Suspect, error) {
    var s Suspect
    if err := db.WithContext(ctx).
        First(&s, "suspect_id = ?", id).Error; err != nil {
        return nil, err
    }
    return &s, nil
}

func ListSuspects(ctx context.Context, db *gorm.DB, filters map[string]interface{}) ([]Suspect, error) {
    var list []Suspect
    q := db.WithContext(ctx)
    if len(filters) > 0 {
        q = q.Where(filters)
    }
    if err := q.Find(&list).Error; err != nil {
        return nil, err
    }
    return list, nil
}

func UpdateSuspect(ctx context.Context, db *gorm.DB, id int64, updates map[string]interface{}) error {
    res := db.WithContext(ctx).
        Model(&Suspect{}).
        Where("suspect_id = ?", id).
        Updates(updates)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return errors.New("no suspect record updated")
    }
    return nil
}

func DeleteSuspect(ctx context.Context, db *gorm.DB, id int64) error {
    res := db.WithContext(ctx).
        Delete(&Suspect{}, "suspect_id = ?", id)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return errors.New("no suspect record deleted")
    }
    return nil
}
