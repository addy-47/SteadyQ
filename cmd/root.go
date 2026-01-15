package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"steadyq/internal/banner"
	"steadyq/internal/cli"
	"steadyq/internal/dummy"
	"steadyq/internal/runner"
	"steadyq/internal/tui/app"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	cfgFile string

	// CLI Flags
	url       string
	method    string
	body      string
	rate      int
	users     int
	duration  int
	rampUp    int
	rampDown  int
	timeout   int
	headers   []string
	outPrefix string
)

var rootCmd = &cobra.Command{
	Use:   "steadyq",
	Short: "SteadyQ - Modern Load Testing Tool",
	Long: `
SteadyQ is a high-performance, TUI-based load testing tool.

It supports two main modes:
1. TUI Mode (Default): Interactive Terminal UI
2. CLI Mode (Headless): Run with flags for CI/CD usage`,
	Run: func(cmd *cobra.Command, args []string) {
		// If CLI flags are provided, run headless
		if cmd.Flags().Changed("url") {
			runHeadless()
			return
		}

		// Otherwise, run TUI
		runTUI()
	},
}

func Execute() {
	// Custom Help with Banner
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Println(banner.GetString())
		cmd.Usage()
	})

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// dummy command?
	rootCmd.AddCommand(dummyCmd)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.steadyq.yaml)")

	rootCmd.Flags().StringVarP(&url, "url", "u", "", "Target URL (enables CLI mode)")
	rootCmd.Flags().StringVarP(&method, "method", "X", "GET", "HTTP Method")
	rootCmd.Flags().StringVarP(&body, "body", "b", "", "Request Body")
	rootCmd.Flags().IntVarP(&rate, "rate", "r", 10, "Target RPS (Open Loop)")
	rootCmd.Flags().IntVarP(&users, "users", "U", 0, "Target Users (Closed Loop, overrides rate)")
	rootCmd.Flags().IntVarP(&duration, "duration", "d", 10, "Duration in seconds")
	rootCmd.Flags().IntVar(&rampUp, "ramp-up", 0, "Ramp Up duration in seconds")
	rootCmd.Flags().IntVar(&rampDown, "ramp-down", 0, "Ramp Down duration in seconds")
	rootCmd.Flags().IntVar(&timeout, "timeout", 10, "Request timeout in seconds")
	rootCmd.Flags().StringSliceVarP(&headers, "header", "H", []string{}, "HTTP Header (e.g. \"Key: Value\")")
	rootCmd.Flags().StringVarP(&outPrefix, "out", "o", "", "Output filename prefix for auto-reporting")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home)
			viper.SetConfigType("yaml")
			viper.SetConfigName(".steadyq")
		}
	}
	viper.AutomaticEnv()
	viper.ReadInConfig()
}

// --- Runners ---

func runTUI() {
	// 2. Setup Default Runner (Idle)
	defaultCfg := runner.Config{
		TargetRPS: 10,
		SteadyDur: 10, // Default 10s
		Mode:      "rps",
		URL:       "http://localhost:8080/fast",
	}
	updates := make(runner.StatsUpdateChan, 100)
	run := runner.NewRunner(defaultCfg, updates)

	// 3. Launch TUI Application
	m := app.NewModel(run, updates)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running SteadyQ: %v\n", err)
		os.Exit(1)
	}
}

func runHeadless() {
	// Construct config from flags
	cfg := runner.Config{
		URL:        url,
		Method:     method,
		Body:       body,
		TargetRPS:  rate,
		SteadyDur:  duration,
		RampUp:     rampUp,
		RampDown:   rampDown,
		TimeoutSec: timeout,
		Mode:       "rps",
		OutPrefix:  outPrefix,
	}
	if users > 0 {
		cfg.Mode = "users"
		cfg.NumUsers = users
	}

	// Parse Headers
	cfg.Headers = make(map[string]string)
	for _, h := range headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			cfg.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	cli.Start(cfg)
}

// --- Dummy Subcommand ---
var dummyCmd = &cobra.Command{
	Use:   "dummy",
	Short: "Run internal dummy server",
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		dummy.Start(dummy.ServerConfig{Port: port})
		select {}
	},
}

func init() {
	dummyCmd.Flags().IntP("port", "p", 8080, "Port to run dummy server on")
}
