package main

import (
	"os"
	"runtime"
	"time"

	zl "github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	dp "github.com/forscht/ddrv/internal/dataprovider"
	"github.com/forscht/ddrv/internal/dataprovider/bolt"
	"github.com/forscht/ddrv/internal/dataprovider/postgres"
	"github.com/forscht/ddrv/internal/ftp"
	"github.com/forscht/ddrv/internal/http"
	"github.com/forscht/ddrv/pkg/ddrv"
)

// Config represents the entire configuration as defined in the YAML file.
type Config struct {
	Ddrv struct {
		Token      string `mapstructure:"token"`
		TokenType  int    `mapstructure:"token_type"`
		Channels   string `mapstructure:"channels"`
		AsyncWrite bool   `mapstructure:"async_write"`
		ChunkSize  int    `mapstructure:"chunk_size"`
	} `mapstructure:"ddrv"`

	Dataprovider struct {
		Bolt     bolt.Config     `mapstructure:"boltdb"`
		Postgres postgres.Config `mapstructure:"postgres"`
	} `mapstructure:"dataprovider"`

	Frontend struct {
		FTP  ftp.Config  `mapstructure:"ftp"`
		HTTP http.Config `mapstructure:"http"`
	} `mapstructure:"frontend"`
}

func main() {
	// Set the maximum number of operating system threads to use.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Setup logger
	log.Logger = zl.New(zl.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).With().Timestamp().Logger()
	zl.SetGlobalLevel(zl.DebugLevel)

	// Setup config
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.config/ddrv/")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal().Str("c", "config").Err(err).Msg("failed to read config")
	}
	var config Config
	err := viper.Unmarshal(&config)
	if err != nil {
		log.Fatal().Str("c", "config").Err(err).Msg("failed to decode config into struct")
	}

	// Create a ddrv driver
	driver, err := ddrv.New((*ddrv.Config)(&config.Ddrv))
	if err != nil {
		log.Fatal().Err(err).Str("c", "main").Msg("failed to open ddrv driver")
	}

	// Load data provider
	var provider dp.DataProvider
	if config.Dataprovider.Bolt.DbPath != "" {
		provider = bolt.New(driver, &config.Dataprovider.Bolt)
	}
	if provider == nil && config.Dataprovider.Postgres.DbURL != "" {
		provider = postgres.New(&config.Dataprovider.Postgres, driver)
	}
	if provider == nil {
		config.Dataprovider.Bolt.DbPath = "./ddrv.db"
		provider = bolt.New(driver, &config.Dataprovider.Bolt)
	}
	dp.Load(provider)

	errCh := make(chan error)
	// Create and start ftp server
	go func() { errCh <- ftp.Serv(driver, &config.Frontend.FTP) }()
	// Create and start http server
	go func() { errCh <- http.Serv(driver, &config.Frontend.HTTP) }()

	log.Fatal().Msgf("ddrv: error %v", <-errCh)
}
