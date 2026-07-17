/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/berkantsoytas/velox/internal/reporter"
	"github.com/berkantsoytas/velox/internal/requester"
	"github.com/berkantsoytas/velox/internal/runner"

	"github.com/spf13/cobra"
)

var (
	targetURL   string
	method      string
	requests    int
	concurrency int
	body        string
	headers     []string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "velox",
	Short: "Velox is a high-performance HTTP load testing tool",
	Long:  `Velox is a CLI tool designed to test the performance and robustness of HTTP APIs with a beautiful live dashboard.`,

	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
	Run: func(cmd *cobra.Command, args []string) {
		if targetURL == "" {
			fmt.Println("Error: URL required.")
			os.Exit(1)
		}

		if concurrency > requests {
			concurrency = requests
		}

		headerMap := make(map[string]string)
		for _, h := range headers {
			parts := strings.SplitN(h, ":", 2)
			if len(parts) == 2 {
				headerMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}

		reqCfg := requester.Config{
			Concurrency:      concurrency,
			Timeout:          30 * time.Second,
			DisableKeepAlive: false,
		}

		client := requester.NewRequester(reqCfg)

		runCfg := runner.Config{
			URL:         targetURL,
			Method:      strings.ToUpper(method),
			Requests:    requests,
			Concurrency: concurrency,
			Headers:     headerMap,
			Body:        []byte(body),
		}

		for {
			resultsChan := make(chan requester.Result, requests)
			go runner.Run(runCfg, client, resultsChan)

			restart, err := reporter.RunDashboard(resultsChan, requests)
			if err != nil {
				fmt.Printf("Error starting the dashboard: %v\n", err)
				os.Exit(1)
			}

			if !restart {
				break
			}
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.velox.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	rootCmd.Flags().StringVarP(&targetURL, "url", "u", "", "Target URL (e.g., http://localhost:8080)")
	rootCmd.Flags().StringVarP(&method, "method", "X", "GET", "HTTP Method (GET, POST, PUT, DELETE, etc.)")
	rootCmd.Flags().IntVarP(&requests, "requests", "n", 100, "Number of total requests to perform")
	rootCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 10, "Number of multiple requests to make at a time")
	rootCmd.Flags().StringVarP(&body, "data", "d", "", "HTTP Request Body (e.g., '{\"key\":\"value\"}')")
	rootCmd.Flags().StringSliceVarP(&headers, "header", "H", []string{}, "Custom HTTP headers (e.g., -H 'Auth: Bearer token')")
}
