package database

import (
	"fmt"
	"log"
	"strings"

	"github.com/flaboy/aira-core/pkg/config"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
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
	DbType     string
	DbHost     string
	DbPort     int
	DbUser     string
	DbPassword string
	DbName     string
	DbSchema   string
}

var gormConfig *gorm.Config

func connectDatabase(params DbInfo) error {
	var driver gorm.Dialector
	var err error

	switch strings.ToLower(params.DbType) {
	case "pgsql", "postgresql":
		// PostgreSQL DSN format: host=localhost user=gorm password=gorm dbname=gorm port=9920 sslmode=disable TimeZone=Asia/Shanghai
		dbInfo := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=%s",
			params.DbHost, params.DbUser, params.DbPassword, params.DbName, params.DbPort, config.Config.DefaultTimezone)
		if params.DbSchema != "" {
			dbInfo += fmt.Sprintf(" search_path=%s", params.DbSchema)
		}
		driver = postgres.Open(dbInfo)
	case "mysql":
		// MySQL DSN format: user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local
		dbInfo := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			params.DbUser, params.DbPassword, params.DbHost, params.DbPort, params.DbName)
		driver = mysql.Open(dbInfo)
	default:
		log.Fatal("unsupported database type", "type", params.DbType)
		return fmt.Errorf("unsupported database type: %s", params.DbType)
	}

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
	params.DbType = config.Config.DB_TYPE
	params.DbHost = config.Config.DB_HOST
	params.DbPort = config.Config.DB_PORT
	params.DbUser = config.Config.DB_USER
	params.DbPassword = config.Config.DB_PASSWORD
	params.DbName = config.Config.DB_DBNAME
	params.DbSchema = config.Config.DB_SCHEMA

	return connectDatabase(params)
}
