package main

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/gesquive/dyngo/dns"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var version = "v0.2.0-dev"
var dirty = ""

var cfgFile string
var logPath string

var displayVersion string
var showVersion bool
var debug bool

var log = logrus.New()

func main() {
	displayVersion = fmt.Sprintf("dyngo %s%s",
		version,
		dirty)
	Execute(displayVersion)
}

// RootCmd handles all of our arguments/options
var RootCmd = &cobra.Command{
	Use:   "dyngo",
	Short: "Use digitalocean as your DDNS service",
	Long: `A service application that watches your external IP for changes
and updates a DigitalOcean domain record when a change is detected`,
	PersistentPreRun: preRun,
	Run:              run,
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
		"Path to log file (default \"-\")")

	RootCmd.PersistentFlags().BoolVar(&showVersion, "version", false,
		"Display the version number and exit")
	RootCmd.PersistentFlags().BoolP("run-once", "o", false,
		"Only run once and exit")

	RootCmd.PersistentFlags().BoolP("ipv4", "4", true,
		"Check for our WAN IPv4 address")
	RootCmd.PersistentFlags().BoolP("ipv6", "6", true,
		"Check for our WAN IPv6 address")

	RootCmd.PersistentFlags().StringP("sync-interval", "i", "60m",
		"The duration between DNS updates")

	RootCmd.PersistentFlags().BoolVarP(&debug, "debug", "D", false,
		"Include debug statements in log output")
	RootCmd.PersistentFlags().MarkHidden("debug")

	viper.SetEnvPrefix("dyngo")
	viper.AutomaticEnv()
	viper.BindEnv("log-file")
	viper.BindEnv("run-once")
	viper.BindEnv("sync-interval")
	viper.BindEnv("ipv4")
	viper.BindEnv("ipv6")

	viper.BindPFlag("log_file", RootCmd.PersistentFlags().Lookup("log-file"))
	viper.BindPFlag("service.run_once", RootCmd.PersistentFlags().Lookup("run-once"))
	viper.BindPFlag("service.sync_interval", RootCmd.PersistentFlags().Lookup("sync-interval"))
	viper.BindPFlag("ip_check.ipv4", RootCmd.PersistentFlags().Lookup("ipv4"))
	viper.BindPFlag("ip_check.ipv6", RootCmd.PersistentFlags().Lookup("ipv6"))

	viper.SetDefault("log_file", "-")
	viper.SetDefault("service.sync_interval", "60m")
	viper.SetDefault("ip_check.ipv4_urls", []string{})
	viper.SetDefault("ip_check.ipv6_urls", []string{})
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	}

	viper.SetConfigName("config") // name of config file (without extension)
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.config/dyngo") // adding home directory as first search path
	viper.AddConfigPath("/etc/dyngo")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return
		}
		if !showVersion {
			fmt.Println("Error opening config: ", err)
		}
	}
}

func preRun(cmd *cobra.Command, args []string) {
	if showVersion {
		fmt.Println(displayVersion)
		os.Exit(0)
	}

	log.SetFormatter(&prefixed.TextFormatter{
		TimestampFormat: time.RFC3339,
	})

	if debug {
		log.SetLevel(logrus.DebugLevel)
	} else {
		log.SetLevel(logrus.InfoLevel)
	}

	log.Debug("Running with debug turned on")
}

func run(cmd *cobra.Command, args []string) {
	logFilePath := getLogFilePath(viper.GetString("log_file"))
	log.Debugf("config: log_file=%s", logFilePath)
	if logFilePath == "" || logFilePath == "-" {
		log.SetOutput(os.Stdout)
	} else {
		logFile, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening log file=%v", err)
		}
		defer logFile.Close()
		log.SetOutput(logFile)
	}

	log.Debugf("config: file=%s", viper.ConfigFileUsed())
	checkIPv4 := viper.GetBool("ip_check.ipv4")
	checkIPv6 := viper.GetBool("ip_check.ipv6")
	log.Debugf("config: ipv4=%t ipv6=%t", checkIPv4, checkIPv6)
	if !checkIPv4 && !checkIPv6 {
		log.Errorf("IP checks for both IPv4 & IPv6 are turned off!")
		os.Exit(2)
	}

	if checkIPv4 {
		log.Debugf("config: ipv4_urls=%q", viper.GetStringSlice("ip_check.ipv4_urls"))
	}
	if checkIPv6 {
		log.Debugf("config: ipv6_urls=%q", viper.GetStringSlice("ip_check.ipv6_urls"))
	}

	dns.IntializeLogging(log)
	dnsProviders, err := getDNSProviders()
	if err != nil {
		log.Errorf("could not parse dns_providers: %v", err)
	}
	log.Debugf("config: found %d dns providers", len(dnsProviders))

	// domain := viper.GetString("domain")
	// if len(domain) == 0 {
	// 	log.Error("No domain specified")
	// 	os.Exit(2)
	// }
	// token := viper.GetString("token")
	// if len(token) == 0 {
	// 	log.Error("No DO token found")
	// 	os.Exit(2)
	// // }
	// log.Debugf("config: domain=%s token=%s...",
	// 	viper.GetString("domain"),
	// 	viper.GetString("token")[:5])

	if viper.GetBool("service.run_once") {
		RunSync(dnsProviders)
	} else {
		interval, err := time.ParseDuration(viper.GetString("service.sync_interval"))
		if err != nil {
			log.Errorf("config: the given sync value is invalid sync_interval=%s err=%s",
				viper.GetString("service.sync_interval"), err)
			os.Exit(1)
		}
		RunService(dnsProviders, interval)
	}
}

func getLogFilePath(defaultPath string) (logPath string) {
	fi, err := os.Stat(defaultPath)
	if err == nil && fi.IsDir() {
		logPath = path.Join(defaultPath, "dyngo.log")
	} else {
		logPath = defaultPath
	}
	return
}
