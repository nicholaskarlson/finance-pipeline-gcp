package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/nicholaskarlson/finance-pipeline-gcp/internal/pipeline"
	"github.com/nicholaskarlson/finance-pipeline-gcp/internal/runid"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "run":
		run(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `finance-pipeline-gcp

Commands:
  run   Run recon + auditpack on two CSVs

Examples:
  go run ./cmd/pipeline run --left left.csv --right right.csv --out ./out
  go run ./cmd/pipeline run --left left.csv --right right.csv --out ./out --run-id demo
`)
}

func run(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	left := fs.String("left", "", "path to left.csv")
	right := fs.String("right", "", "path to right.csv")
	out := fs.String("out", "./out", "output base directory")
	forceID := fs.String("run-id", "", "optional stable run id (default: sha256(left+right) prefix)")
	reconBin := fs.String("recon", "recon", "path to recon binary (or recon on PATH)")
	auditBin := fs.String("auditpack", "auditpack", "path to auditpack binary (or auditpack on PATH)")
	label := fs.String("label", "", "optional auditpack label (default: job:<run-id>)")
	_ = fs.Parse(args)

	if *left == "" || *right == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --left and --right are required")
		os.Exit(2)
	}

	id := *forceID
	if id == "" {
		var err error
		id, err = runid.FromFiles(*left, *right)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: compute run id: %v\n", err)
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	res, err := pipeline.Run(ctx, pipeline.Config{
		LeftPath:     *left,
		RightPath:    *right,
		OutBase:      *out,
		RunID:        id,
		ReconBin:     *reconBin,
		AuditpackBin: *auditBin,
		Label:        *label,
	})

	fmt.Printf("run_id=%s\nrun_dir=%s\npack_dir=%s\n", id, res.RunDir, res.PackDir)

	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
