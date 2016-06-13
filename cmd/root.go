package cmd

import (
	"fmt"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var cfgFile string

var displayVersion string
var showVersion bool
var singleRun bool
var verbose bool
var debug bool

// RootCmd handles all of our arguments/options
var RootCmd = &cobra.Command{
	Use:   "digitalocean-ddns",
	Short: "Use digitalocean as your DDNS service",
	Long: `A service application that watches your external IP for changes
and updates a DigitalOcean domain when a change is detected`,
	Run: func(cmd *cobra.Command, args []string) {
		if showVersion {
			fmt.Println(displayVersion)
			os.Exit(1)
		}

		log.SetFormatter(&prefixed.TextFormatter{
			TimestampFormat: time.RFC3339,
		})
		log.SetOutput(os.Stdout)
		log.SetLevel(log.InfoLevel)

		if debug {
			log.SetLevel(log.DebugLevel)
		}
		log.Infof("config: file=%s", viper.ConfigFileUsed())
		log.Debugf("config: domain=%s token=%s...",
			viper.GetString("domain"),
			viper.GetString("token")[:5])

		if singleRun {
			RunSync(viper.GetString("token"), viper.GetString("domain"))
		} else {
			interval, err := time.ParseDuration(viper.GetString("sync_interval"))
			if err != nil {
				log.Errorf("config: the given sync value is invalid sync_interval=%s err=%s",
					viper.GetString("sync_interval"), err)
				os.Exit(1)
			}
			RunService(viper.GetString("token"), viper.GetString("domain"),
				interval)
		}
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(version string) {
	displayVersion = version
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is ./config.yaml)")
	RootCmd.PersistentFlags().StringP("token", "t", "",
		"the DigitalOcean API token to authenticate with")
	RootCmd.PersistentFlags().StringP("domain", "d", "",
		"the DigitalOcean domain record to update")
	RootCmd.PersistentFlags().BoolVar(&showVersion, "version", false,
		"Display the version number and exit.")
	RootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false,
		"Print logs to stdout instead of file")
	RootCmd.PersistentFlags().BoolVar(&debug, "debug", false,
		"Include debug statements in log output")
	RootCmd.PersistentFlags().StringP("sync-interval", "i", "60m",
		"When running as a service, the duration between sync checks (default: 60m)")
	RootCmd.PersistentFlags().BoolVarP(&singleRun, "sync", "s", false,
		"Only run one sync and exit.")

	viper.SetEnvPrefix("doddns")
	viper.AutomaticEnv()
	viper.BindEnv("token")
	viper.BindEnv("domain")
	viper.BindEnv("sync-interval")

	viper.BindPFlag("token", RootCmd.PersistentFlags().Lookup("token"))
	viper.BindPFlag("domain", RootCmd.PersistentFlags().Lookup("domain"))
	viper.BindPFlag("sync_interval", RootCmd.PersistentFlags().Lookup("sync-interval"))

	viper.SetDefault("sync_interval", "60m")
	viper.SetDefault("url_list", []string{
		"http://icanhazip.com",
		"http://whatismyip.akamai.com/",
		"http://whatsmyip.me/",
		"http://wtfismyip.com/text",
		"http://api.ipify.org/",
		"http://ip.catnapgames.com",
		"http://ip.ryansanden.com",
	})
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	}

	viper.SetConfigName("config.yml") // name of config file (without extension)
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.config/digitalocean-ddns") // adding home directory as first search path
	viper.AddConfigPath("/etc/digitalocean-ddns")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Error opening config: ", err)
	}
}
