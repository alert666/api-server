package data

import (
	"fmt"

	"github.com/alert666/api-server/base/conf"
	"github.com/alert666/api-server/store"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDB() (*gorm.DB, func(), error) {
	dsn, err := conf.GetMysqlDsn()
	if err != nil {
		return nil, nil, err
	}
	var dbLogger logger.Interface
	// 开启mysql日志
	if viper.GetBool("mysql.debug") {
		dbLogger = logger.Default.LogMode(logger.Info)
		zap.L().Info("enable debug mode on the database")
	}

	dbInstance, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   dbLogger,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("exception in initializing mysql database, %w", err)
	}

	// 确保数据库连接已建立
	sqlDB, err := dbInstance.DB()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to obtain database connection, %w", err)
	}

	// 尝试Ping数据库以确保连接有效
	err = sqlDB.Ping()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to obtain database connection, %w", err)
	}

	sqlDB.SetMaxOpenConns(conf.GetMysqlMaxOpenConns())
	sqlDB.SetMaxIdleConns(conf.GetMysqlMaxIdleConns())
	sqlDB.SetConnMaxLifetime(conf.GetMysqlMaxLifetime())

	zap.L().Info("db connect success")
	store.SetDefault(dbInstance)
	return dbInstance, func() { _ = sqlDB.Close() }, nil
}
