package database

import (
    "context"
    "errors"
    "time"

    "github.com/google/uuid"
    "gorm.io/gorm"
)

type Admin struct {
    UserID      string    `json:"user_id"` // uuid
    AdminLevel  int       `json:"admin_level"`
    CreatedAt   time.Time `json:"created_at"`
}

func (Admin) TableName() string {
    return "Admin"
}

func CreateAdmin(ctx context.Context, db *gorm.DB, a *Admin) error {
    return db.WithContext(ctx).Create(a).Error
}

func GetAdminByUserID(ctx context.Context, db *gorm.DB, userID uuid.UUID) (*Admin, error) {
    var a Admin
    if err := db.WithContext(ctx).
        First(&a, "user_id = ?", userID).Error; err != nil {
        return nil, err
    }
    return &a, nil
}

func ListAdmin(ctx context.Context, db *gorm.DB, where map[string]interface{}) ([]Admin, error) {
    var list []Admin
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
        Model(&Admin{}).
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
        Delete(&Admin{}, "user_id = ?", userID)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return errors.New("no admin record deleted")
    }
    return nil
}
