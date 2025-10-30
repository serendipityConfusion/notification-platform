package ioc

import (
	"github.com/serendipityConfusion/notification-platform/internal/pkg/database/metrics"
	"github.com/serendipityConfusion/notification-platform/internal/pkg/database/tracing"
	"github.com/serendipityConfusion/notification-platform/internal/repository/dao"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDB() *gorm.DB {
	db, err := gorm.Open(mysql.Open(viper.GetString("mysql.dsn")), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	dao.InitTable(db)
	if err = db.Use(metrics.NewGormMetricsPlugin()); err != nil {
		panic(err)
	}
	if err = db.Use(tracing.NewGormTracingPlugin()); err != nil {
		panic(err)
	}
	return db
}
