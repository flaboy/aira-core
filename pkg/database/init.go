package database

import (
	"fmt"
	"log"

	"github.com/flaboy/aira-core/pkg/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var (
	db *gorm.DB
)

func Database() *gorm.DB {
	return db
}

type DbInfo struct {
	DbHost     string
	DbPort     int
	DbUser     string
	DbPassword string
	DbName     string
}

var gormConfig *gorm.Config

func connectDatabase(params DbInfo) error {
	dbInfo := ""
	var driver gorm.Dialector
	dbInfo = fmt.Sprintf("%s:%s@(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", params.DbUser, params.DbPassword, params.DbHost, params.DbPort, params.DbName)
	driver = mysql.Open(dbInfo)

	var err error

	db, err = gorm.Open(driver, gormConfig)
	if err != nil {
		log.Fatal("sql.Open failed", "error", err)
		return err
	}
	return nil
}

func initDBConfig() {
	gormConfig = &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	}
	gormConfig.NamingStrategy = schema.NamingStrategy{
		SingularTable: false,
	}

}

func Start() error {
	initDBConfig()
	params := DbInfo{}
	params.DbHost = config.Config.MYSQL_HOST
	params.DbPort = config.Config.MYSQL_PORT
	params.DbUser = config.Config.MYSQL_USER
	params.DbPassword = config.Config.MYSQL_PASSWORD
	params.DbName = config.Config.MYSQL_DBNAME

	return connectDatabase(params)
}
