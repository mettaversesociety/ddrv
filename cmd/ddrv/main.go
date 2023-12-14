package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	zl "github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
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

var config Config

var ddrvCmd = &cobra.Command{Use: "ddrv"}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of the application",
	Long:  `All software has versions. This is yours.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ddrv: v%s\n", version)
	},
}

func main() {
	// Set the maximum number of operating system threads to use.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Setup cobra
	cobra.OnInitialize(initConfig)
	ddrvCmd.AddCommand(versionCmd)

	// Define flags for ddrv settings
	ddrvCmd.PersistentFlags().String("token", "", "Discord bot/user token")
	ddrvCmd.PersistentFlags().Int("token-type", 0, "Type of token used for Discord API")
	ddrvCmd.PersistentFlags().String("channels", "", "List of Discord channel IDs")
	ddrvCmd.PersistentFlags().Int("chunk-size", 0, "Maximum size of chunks sent via Discord webhook")

	// Define flags for dataprovider settings
	ddrvCmd.PersistentFlags().String("bolt-dbpath", "", "File path for BoltDB database file")
	ddrvCmd.PersistentFlags().String("postgres-dburl", "", "PostgreSQL database URL")

	// Define flags for frontend FTP settings
	ddrvCmd.PersistentFlags().String("ftp-addr", "", "FTP server address")
	ddrvCmd.PersistentFlags().String("ftp-username", "", "FTP server username")
	ddrvCmd.PersistentFlags().String("ftp-password", "", "FTP server password")
	ddrvCmd.PersistentFlags().Bool("ftp-async-write", false, "Enable concurrent file uploads to Discord")

	// Define flags for frontend HTTP settings
	ddrvCmd.PersistentFlags().String("http-addr", "", "HTTP server address")
	ddrvCmd.PersistentFlags().String("https-addr", "", "HTTPS server address")
	ddrvCmd.PersistentFlags().String("https-keypath", "", "Path to the HTTPS private key file.")
	ddrvCmd.PersistentFlags().String("https-crtpath", "", "Path to the HTTPS certificate file.")
	ddrvCmd.PersistentFlags().String("http-username", "", "HTTP server username")
	ddrvCmd.PersistentFlags().String("http-password", "", "HTTP server password")
	ddrvCmd.PersistentFlags().Bool("http-guest-mode", false, "Enable guest mode for HTTP interface")
	ddrvCmd.PersistentFlags().Bool("http-async-write", false, "Enable concurrent file uploads to Discord")

	// Define global flags
	ddrvCmd.PersistentFlags().Bool("debug", false, "Enable more verbose logs")

	ddrvCmd.Run = func(command *cobra.Command, args []string) {

		// Setup logger
		log.Logger = zl.New(zl.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).With().Timestamp().Logger()
		zl.SetGlobalLevel(zl.InfoLevel)
		debug, _ := command.PersistentFlags().GetBool("debug")
		if debug {
			zl.SetGlobalLevel(zl.DebugLevel)
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

	// Execute Cobra
	if err := ddrvCmd.Execute(); err != nil {
		log.Fatal().Err(err)
	}
}

func initConfig() {
	// Setup config
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config/")
	viper.AddConfigPath("$HOME/.config/ddrv/")
	if err := viper.ReadInConfig(); err != nil {
		// Use a type assertion to check if the error is ConfigFileNotFoundError
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			// Handle the case where the config file was not found
			log.Warn().Msg("No config file found. Using command line args")
		} else {
			log.Fatal().Err(err).Msg("failed to read config file")
		}
	}

	viper.BindPFlag("ddrv.token", ddrvCmd.PersistentFlags().Lookup("token"))
	viper.BindPFlag("ddrv.token_type", ddrvCmd.PersistentFlags().Lookup("token-type"))
	viper.BindPFlag("ddrv.channels", ddrvCmd.PersistentFlags().Lookup("channels"))
	viper.BindPFlag("ddrv.chunk_size", ddrvCmd.PersistentFlags().Lookup("chunk-size"))
	viper.BindPFlag("dataprovider.boltdb.db_path", ddrvCmd.PersistentFlags().Lookup("bolt-dbpath"))
	viper.BindPFlag("dataprovider.postgres.db_url", ddrvCmd.PersistentFlags().Lookup("postgres-dburl"))
	viper.BindPFlag("frontend.ftp.addr", ddrvCmd.PersistentFlags().Lookup("ftp-addr"))
	viper.BindPFlag("frontend.ftp.username", ddrvCmd.PersistentFlags().Lookup("ftp-username"))
	viper.BindPFlag("frontend.ftp.password", ddrvCmd.PersistentFlags().Lookup("ftp-password"))
	viper.BindPFlag("frontend.ftp.async_write", ddrvCmd.PersistentFlags().Lookup("ftp-async-write"))
	viper.BindPFlag("frontend.http.addr", ddrvCmd.PersistentFlags().Lookup("http-addr"))
	viper.BindPFlag("frontend.https.addr", ddrvCmd.PersistentFlags().Lookup("https-addr"))
	viper.BindPFlag("frontend.https.keypath", ddrvCmd.PersistentFlags().Lookup("https-keypath"))
	viper.BindPFlag("frontend.https.certpath", ddrvCmd.PersistentFlags().Lookup("https-crtpath"))
	viper.BindPFlag("frontend.http.username", ddrvCmd.PersistentFlags().Lookup("http-username"))
	viper.BindPFlag("frontend.http.password", ddrvCmd.PersistentFlags().Lookup("http-password"))
	viper.BindPFlag("frontend.http.guest_mode", ddrvCmd.PersistentFlags().Lookup("http-guest-mode"))

	err := viper.Unmarshal(&config)
	if err != nil {
		log.Fatal().Str("c", "config").Err(err).Msg("failed to decode config into struct")
	}
}
