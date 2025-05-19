package database

import (
    "context"
    "errors"
    "gorm.io/gorm"
)

// Vehicle represents a row in the vehicles table.
type Vehicle struct {
    VehicleID    int    `json:"vehicle_id"` // serial
    VehicleName  string `json:"vehicle_name"`
    Color        string `json:"color"`
    UserID       string `json:"user_id"` // uuid
    PlateNumber  string `json:"plate_number"`
}

func (Vehicle) TableName() string {
    return "vehicle"
}

func CreateVehicle(ctx context.Context, db *gorm.DB, v *Vehicle) error {
    return db.WithContext(ctx).Create(v).Error
}

func GetVehicleByID(ctx context.Context, db *gorm.DB, id int64) (*Vehicle, error) {
    var v Vehicle
    if err := db.WithContext(ctx).
        First(&v, "vehicle_id = ?", id).Error; err != nil {
        return nil, err
    }
    return &v, nil
}

func ListVehicles(ctx context.Context, db *gorm.DB, filters map[string]interface{}) ([]Vehicle, error) {
    var list []Vehicle
    q := db.WithContext(ctx)
    if len(filters) > 0 {
        q = q.Where(filters)
    }
    if err := q.Find(&list).Error; err != nil {
        return nil, err
    }
    return list, nil
}

func UpdateVehicle(ctx context.Context, db *gorm.DB, id int64, updates map[string]interface{}) error {
    res := db.WithContext(ctx).
        Model(&Vehicle{}).
        Where("vehicle_id = ?", id).
        Updates(updates)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return errors.New("no vehicle record updated")
    }
    return nil
}

func DeleteVehicle(ctx context.Context, db *gorm.DB, id int64) error {
    res := db.WithContext(ctx).
        Delete(&Vehicle{}, "vehicle_id = ?", id)
    if res.Error != nil {
        return res.Error
    }
    if res.RowsAffected == 0 {
        return errors.New("no vehicle record deleted")
    }
    return nil
}
