package database

import (
    "context"
    "errors"
    "time"

    "github.com/google/uuid"
    "gorm.io/gorm"
)

type Admins struct {
    UserID     uuid.UUID `gorm:"column:user_id;type:uuid;primaryKey" json:"user_id"`
    AdminLevel int       `gorm:"column:admin_level" json:"admin_level"`
    CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (Admins) TableName() string {
    return "admins"
}

func CreateAdmin(ctx context.Context, db *gorm.DB, a *Admins) error {
    return db.WithContext(ctx).Create(a).Error
}

func GetAdminByUserID(ctx context.Context, db *gorm.DB, userID uuid.UUID) (*Admins, error) {
    var a Admins
    if err := db.WithContext(ctx).
        First(&a, "user_id = ?", userID).Error; err != nil {
        return nil, err
    }
    return &a, nil
}

func ListAdmins(ctx context.Context, db *gorm.DB, where map[string]interface{}) ([]Admins, error) {
    var list []Admins
    q := db.WithContext(ctx)
    if len(where) > 0 {
        q = q.Where(where)
    }
    if err := q.Find(&list).Error; err != nil {
        return nil, err
    }
    return list, nil
}

func UpdateAdmin(ctx context.Context, db *gorm.DB, userID uuid.UUID, updates map[string]interface{}) error {
    res := db.WithContext(ctx).
        Model(&Admins{}).
        Where("user_id = ?", userID).
        Updates(updates)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return errors.New("no admin record updated")
    }
    return nil
}

func DeleteAdmin(ctx context.Context, db *gorm.DB, userID uuid.UUID) error {
    res := db.WithContext(ctx).
        Delete(&Admins{}, "user_id = ?", userID)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return errors.New("no admin record deleted")
    }
    return nil
}
