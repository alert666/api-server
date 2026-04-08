package store_test

import (
	"github.com/alert666/api-server/base/conf"
	"github.com/alert666/api-server/base/data"
	"github.com/alert666/api-server/base/log"
	"gorm.io/gorm"
)

var (
	db *gorm.DB
)

func init() {
	var err error
	conf.LoadConfig("../../config.yaml")
	db, _, err = data.NewDB()
	if err != nil {
		panic(err)
	}
	log.NewLogger()
}
