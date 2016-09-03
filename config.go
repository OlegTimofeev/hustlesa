package main

import (
	"github.com/itimofeev/hustlesa/parser"
	"github.com/kelseyhightower/envconfig"
)

type configEnv struct {
	DbEnv
}

// Config interface
type Config interface {
	Db() *DbEnv
}

// DbEnv db settings
type DbEnv struct {
	URL          string `envconfig:"HUSTLESA_DB_URL"`
	MaxIdleConns int    `envconfig:"HUSTLESA_DB_MAX_IDLE_CONNS" default:"4"`
	MaxOpenConns int    `envconfig:"HUSTLESA_DB_MAX_OPEN_CONNS" default:"16"`
	StrictMode   bool   `envconfig:"HUSTLESA_DB_STRICT_MODE" default:"false"`
}

// Db settings
func (env configEnv) Db() *DbEnv {
	return &env.DbEnv
}

// ReadConfig read config from environment
func ReadConfig() Config {
	var c configEnv
	err := envconfig.Process("hustlesa", &c)

	parser.CheckErr(err, "read envconfig")

	return &c
}
