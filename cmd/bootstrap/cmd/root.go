package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagOutdir   string
	flagLogLevel string
)

var rootCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Bootstrap a flow network",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setLogLevel()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagOutdir, "outdir", "o", "bootstrap",
		"output directory for generated files")
	rootCmd.PersistentFlags().StringVarP(&flagLogLevel, "loglevel", "l", "info",
		"log level (panic, fatal, error, warn, info, debug)")

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}

func setLogLevel() {
	switch flagLogLevel {
	case "panic":
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	case "fatal":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	default:
		log.Fatal().Str("loglevel", flagLogLevel).Msg("unsupported log level, choose one of \"panic\", \"fatal\", " +
			"\"error\", \"warn\", \"info\" or \"debug\"")
	}
}
