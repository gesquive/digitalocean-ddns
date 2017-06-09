package main

import (
	"fmt"
	"os"
	"path"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var version = "v0.1.7"
var dirty = ""

var cfgFile string
var logPath string

var displayVersion string
var showVersion bool
var verbose bool
var debug bool

func main() {
	displayVersion = fmt.Sprintf("digitalocean-ddns %s%s",
		version,
		dirty)
	Execute(displayVersion)
}

// RootCmd handles all of our arguments/options
var RootCmd = &cobra.Command{
	Use:   "digitalocean-ddns",
	Short: "Use digitalocean as your DDNS service",
	Long: `A service application that watches your external IP for changes
and updates a DigitalOcean domain record when a change is detected`,
	Run: run,
}

// Execute is the starting point
func Execute(version string) {
	displayVersion = version
	RootCmd.SetHelpTemplate(fmt.Sprintf("%s\nVersion:\n  github.com/gesquive/%s\n",
		RootCmd.HelpTemplate(), displayVersion))
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"Path to a specific config file (default \"./config.yaml\")")
	RootCmd.PersistentFlags().String("log-file", "",
		"Path to log file (default \"/var/log/digitalocean-ddns.log\")")

	RootCmd.PersistentFlags().BoolVar(&showVersion, "version", false,
		"Display the version number and exit")
	RootCmd.PersistentFlags().BoolP("run-once", "o", false,
		"Only run once and exit")

	RootCmd.PersistentFlags().StringP("token", "t", "",
		"The DigitalOcean API token to authenticate with")
	RootCmd.PersistentFlags().StringP("domain", "d", "",
		"The DigitalOcean domain record to update")
	RootCmd.PersistentFlags().StringP("sync-interval", "i", "60m",
		"The duration between DNS updates")
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
		"Print logs to stdout instead of file")

	RootCmd.PersistentFlags().BoolVarP(&debug, "debug", "D", false,
		"Include debug statements in log output")
	RootCmd.PersistentFlags().MarkHidden("debug")

	viper.SetEnvPrefix("doddns")
	viper.AutomaticEnv()
	viper.BindEnv("token")
	viper.BindEnv("domain")
	viper.BindEnv("sync-interval")
	viper.BindEnv("run-once")
	viper.BindEnv("log-file")

	viper.BindPFlag("token", RootCmd.PersistentFlags().Lookup("token"))
	viper.BindPFlag("domain", RootCmd.PersistentFlags().Lookup("domain"))
	viper.BindPFlag("sync_interval", RootCmd.PersistentFlags().Lookup("sync-interval"))
	viper.BindPFlag("run_once", RootCmd.PersistentFlags().Lookup("run-once"))
	viper.BindPFlag("log_file", RootCmd.PersistentFlags().Lookup("log-file"))

	viper.SetDefault("log_file", "/var/log/digitalocean-ddns.log")
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

	viper.SetConfigName("config") // name of config file (without extension)
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.config/digitalocean-ddns") // adding home directory as first search path
	viper.AddConfigPath("/etc/digitalocean-ddns")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		if !showVersion {
			fmt.Println("Error opening config: ", err)
		}
	}
}

func run(cmd *cobra.Command, args []string) {
	if showVersion {
		fmt.Println(displayVersion)
		os.Exit(0)
	}

	log.SetFormatter(&prefixed.TextFormatter{
		TimestampFormat: time.RFC3339,
	})

	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	logFilePath := getLogFilePath(viper.GetString("log_file"))
	log.Debugf("config: log_file=%s", logFilePath)
	if verbose {
		log.SetOutput(os.Stdout)
	} else {
		logFile, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening log file=%v", err)
		}
		defer logFile.Close()
		log.SetOutput(logFile)
	}

	log.Infof("config: file=%s", viper.ConfigFileUsed())
	log.Debugf("config: domain=%s token=%s...",
		viper.GetString("domain"),
		viper.GetString("token")[:5])

	if viper.GetBool("run_once") {
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
}

func getLogFilePath(defaultPath string) (logPath string) {
	fi, err := os.Stat(defaultPath)
	if err == nil && fi.IsDir() {
		logPath = path.Join(defaultPath, "digitalocean-ddns.log")
	} else {
		logPath = defaultPath
	}
	return
}
