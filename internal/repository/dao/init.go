package dao

import "gorm.io/gorm"

func InitTable(db *gorm.DB) {
	// todo AutoMigrate all tables
	db.AutoMigrate(
		Notification{},
		CallbackLog{},
		Quota{},
	)
}
