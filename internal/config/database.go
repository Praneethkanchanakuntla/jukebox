package config

import (
	"database/sql"
	"fmt"
	"net"
	"time"

	"github.com/go-sql-driver/mysql"
)

type DataBaseConnection struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

func (c *DataBaseConnection) DSN() string {
	mc := mysql.NewConfig()
	mc.User = c.User
	mc.Passwd = c.Password
	mc.Net = "tcp"
	mc.Addr = net.JoinHostPort(c.Host, c.Port)
	mc.DBName = c.Database
	mc.ParseTime = true
	return mc.FormatDSN()
}

func CreateConnection() (*sql.DB, error) {
	cfg, err := LoadDatabase()
	if err != nil {
		return nil, fmt.Errorf("loading database config: %w", err)
	}

	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return db, nil
}
