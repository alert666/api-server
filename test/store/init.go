package store_test

import (
	"github.com/alert666/api-server/base/config"
	"github.com/alert666/api-server/base/data"
	"github.com/alert666/api-server/base/log"
	"gorm.io/gorm"
)

var (
	db *gorm.DB
)

func init() {
	var err error
	config.LoadConfig("../../config.yaml")
	db, _, err = data.NewDB()
	if err != nil {
		panic(err)
	}
	log.NewLogger()
}
