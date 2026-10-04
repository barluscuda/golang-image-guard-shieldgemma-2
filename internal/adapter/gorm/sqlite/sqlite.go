package sqlite

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func Connect(dsn string, config *gorm.Config) (*gorm.DB, error) {
	if config == nil {
		config = &gorm.Config{}
	}
	if config.Logger == nil {
		config.Logger = gormlogger.Default.LogMode(gormlogger.Warn)
	}
	return gorm.Open(sqlite.Open(dsn), config)
}
