package database

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Users struct {
    UserID    uuid.UUID `db:"user_id"    json:"user_id"`
    Name      string    `db:"name"       json:"name"`
    Email     string    `db:"email"      json:"email"`
    Phone     string    `db:"phone"      json:"phone"`
    Password  string    `db:"password"   json:"-"`            // omit password di JSON
    NIK       string    `db:"nik"        json:"nik"`
    KTPPhoto  string    `db:"ktp_photo"  json:"ktp_photo"`
    CreatedAt time.Time `db:"created_at" json:"created_at"`
}

func (Users) TableName() string {
	return "users"
}

func CreateUser(ctx context.Context, db *gorm.DB, u *Users) error {
	return db.WithContext(ctx).Create(u).Error
}

func GetUserByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*Users, error) {
	var u Users
	if err := db.WithContext(ctx).
			First(&u, "user_id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, err
			}
			return nil, err
	}
	return &u, nil
}

func ListUsers(ctx context.Context, db *gorm.DB, where map[string]interface{}) ([]Users, error) {
	var list []Users
	q := db.WithContext(ctx)
	if len(where) > 0 {
			q = q.Where(where)
	}
	if err := q.Find(&list).Error; err != nil {
			return nil, err
	}
	return list, nil
}

func UpdateUser(ctx context.Context, db *gorm.DB, id uuid.UUID, updates map[string]interface{}) error {
	res := db.WithContext(ctx).
			Model(&Users{}).
			Where("user_id = ?", id).
			Updates(updates)
	if res.Error != nil {
			return res.Error
	}
	if res.RowsAffected == 0 {
			return errors.New("no record updated")
	}
	return nil
}

func DeleteUser(ctx context.Context, db *gorm.DB, id uuid.UUID) error {
	res := db.WithContext(ctx).
			Delete(&Users{}, "user_id = ?", id)
	if res.Error != nil {
			return res.Error
	}
	if res.RowsAffected == 0 {
			return errors.New("no record deleted")
	}
	return nil
}