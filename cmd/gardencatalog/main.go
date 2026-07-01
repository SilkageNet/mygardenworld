package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/cataloggen"
)

func main() {
	var opts cataloggen.Options
	var timeout time.Duration
	flag.StringVar(&opts.MiniRoot, "mini", "tmp/mini", "unpacked mini-program directory or parent directory")
	flag.StringVar(&opts.CDNBase, "cdn", "", "override Cocos CDN base URL")
	flag.StringVar(&opts.StateOutput, "state", "internal/state/catalog_data.json", "backend catalog JSON output")
	flag.StringVar(&opts.WebOutput, "web", "web/src/lib/game/catalog.json", "frontend catalog JSON output")
	flag.DurationVar(&timeout, "timeout", 90*time.Second, "network timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result, err := cataloggen.Generate(ctx, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gardencatalog: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("mini: %s\n", result.MiniRoot)
	if result.Version != "" {
		fmt.Printf("version: %s\n", result.Version)
	}
	fmt.Printf("cdn: %s\n", result.CDNBase)
	fmt.Printf("resource config: %s\n", result.ResourceConfigURL)
	fmt.Printf("g-data: %s\n", result.GameDataURL)
	fmt.Printf("tables: %d, items: %d, flowers: %d, farm lands: %d\n", result.Tables, result.Items, result.Flowers, result.FarmLands)
	fmt.Printf("removed asset fields/values: %d\n", result.RemovedAssets)
	fmt.Printf("wrote: %s\n", result.StateOutput)
	fmt.Printf("wrote: %s\n", result.WebOutput)
}
