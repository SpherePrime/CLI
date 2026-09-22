// Package main is the entry point for the Prime CLI.
package main

import (
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/SpherePrime/CLI/internal/cmd"
	_ "github.com/SpherePrime/CLI/internal/dns"
	_ "github.com/SpherePrime/CLI/vendordeps/joho/godotenv/autoload"
)

func main() {
	if os.Getenv("PRIME_PROFILE") != "" {
		go func() {
			slog.Info("Serving pprof at localhost:6060")
			if httpErr := http.ListenAndServe("localhost:6060", nil); httpErr != nil {
				slog.Error("Failed to pprof listen", "error", httpErr)
			}
		}()
	}

	cmd.Execute()
}
