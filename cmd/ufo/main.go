// Command ufo finds a username across social networks by checking a list
// of site URL templates for existing accounts.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/isanjaymenon/ufo/internal/check"
	"github.com/isanjaymenon/ufo/internal/sites"
)

const (
	catalogName        = "urls.json"
	defaultConcurrency = 10
	requestTimeout     = 10 * time.Second
)

// version is set by GoReleaser via ldflags.
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ufo: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		username    = flag.String("u", "", "Username to search for")
		urlFile     = flag.String("f", "", "File containing URLs to check (JSON array)")
		concurrency = flag.Int("c", defaultConcurrency, "Number of concurrent requests")
		outputFile  = flag.String("o", "", "Output file for results (optional)")
		verbose     = flag.Bool("v", false, "Print request errors to stderr")
		showVersion = flag.Bool("version", false, "Print version")
		help        = flag.Bool("h", false, "Show help")
	)
	flag.Parse()

	if *help {
		showHelp()
		return nil
	}
	if *showVersion {
		fmt.Println(version)
		return nil
	}
	if *username == "" {
		return fmt.Errorf("username is required, use -u or -h for help")
	}

	catalog := *urlFile
	if catalog == "" {
		catalog = findCatalog()
	}

	list, err := sites.Load(catalog)
	if err != nil {
		return err
	}

	var out io.Writer = os.Stdout
	if *outputFile != "" {
		f, err := os.Create(*outputFile)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	runner := &check.Runner{
		Username:    *username,
		Concurrency: *concurrency,
		Timeout:     requestTimeout,
	}
	if *verbose {
		runner.OnError = func(err error) {
			fmt.Fprintf(os.Stderr, "ufo: %v\n", err)
		}
	}
	return runner.Run(ctx, list, func(url string) {
		fmt.Fprintln(out, url)
	})
}

// showHelp displays usage information for the tool.
func showHelp() {
	fmt.Println("UFO - Username Finding Object")
	fmt.Println("\nUsage:")
	fmt.Println("  ufo -u <username> [-f <url_file>] [-c <concurrency>] [-o <output_file>] [-v]")
	fmt.Println("\nOptions:")
	fmt.Println("  -u string    Username to search for (required)")
	fmt.Println("  -f string    File containing site list (default urls.json beside the binary or in the current directory)")
	fmt.Println("  -c int       Number of concurrent requests (default 10)")
	fmt.Println("  -o string    Output file for results (optional)")
	fmt.Println("  -v           Print request errors to stderr")
	fmt.Println("  -version     Print version")
	fmt.Println("  -h           Show this help message")
}

// findCatalog returns urls.json from the current directory, or next to the
// executable (GitHub release archives ship the catalog beside the binary).
func findCatalog() string {
	if _, err := os.Stat(catalogName); err == nil {
		return catalogName
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), catalogName)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return catalogName
}
