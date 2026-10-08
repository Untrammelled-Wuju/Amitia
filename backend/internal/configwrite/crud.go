package configwrite

import "gorm.io/gorm"

func Create[T any](db *gorm.DB, cfg *T, active bool, checks ...func(*gorm.DB) error) error {
	return Transaction(db, func(tx *gorm.DB) error {
		if err := checkConfiguration(tx, checks); err != nil {
			return err
		}
		if active {
			if err := tx.Model(new(T)).Where("is_active = 1").Update("is_active", 0).Error; err != nil {
				return err
			}
		}
		return tx.Create(cfg).Error
	})
}

func Update[T any](db *gorm.DB, id int, updates map[string]interface{}, checks ...func(*gorm.DB) error) error {
	return Transaction(db, func(tx *gorm.DB) error {
		if err := checkConfiguration(tx, checks); err != nil {
			return err
		}
		if err := tx.First(new(T), id).Error; err != nil {
			return err
		}
		active := updates["is_active"]
		if active == true || active == 1 || active == float64(1) {
			if err := tx.Model(new(T)).Where("is_active = 1 AND id <> ?", id).Update("is_active", 0).Error; err != nil {
				return err
			}
		}
		return tx.Model(new(T)).Where("id = ?", id).Updates(updates).Error
	})
}

func Delete[T any](db *gorm.DB, id int, checks ...func(*gorm.DB) error) error {
	return Transaction(db, func(tx *gorm.DB) error {
		if err := checkConfiguration(tx, checks); err != nil {
			return err
		}
		target := new(T)
		if err := tx.First(target, id).Error; err != nil {
			return err
		}
		return tx.Delete(target).Error
	})
}

func Activate[T any](db *gorm.DB, id int, checks ...func(*gorm.DB) error) error {
	return Transaction(db, func(tx *gorm.DB) error {
		if err := checkConfiguration(tx, checks); err != nil {
			return err
		}
		if err := tx.First(new(T), id).Error; err != nil {
			return err
		}
		if err := tx.Model(new(T)).Where("is_active = 1").Update("is_active", 0).Error; err != nil {
			return err
		}
		return tx.Model(new(T)).Where("id = ?", id).Update("is_active", 1).Error
	})
}

func checkConfiguration(tx *gorm.DB, checks []func(*gorm.DB) error) error {
	for _, check := range checks {
		if err := check(tx); err != nil {
			return err
		}
	}
	return nil
}
