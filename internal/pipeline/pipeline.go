package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type Config struct {
	LeftPath     string
	RightPath    string
	OutBase      string
	RunID        string
	ReconBin     string
	AuditpackBin string
	Label        string
}

type Result struct {
	RunDir  string
	TreeDir string
	PackDir string
}

func Run(ctx context.Context, cfg Config) (Result, error) {
	if cfg.ReconBin == "" {
		cfg.ReconBin = "recon"
	}
	if cfg.AuditpackBin == "" {
		cfg.AuditpackBin = "auditpack"
	}
	if cfg.Label == "" {
		cfg.Label = "job:" + cfg.RunID
	}

	runDir := filepath.Join(cfg.OutBase, cfg.RunID)
	treeDir := filepath.Join(runDir, "tree")
	inputsDir := filepath.Join(treeDir, "inputs")
	workDir := filepath.Join(treeDir, "work")
	packDir := filepath.Join(runDir, "pack")

	_ = os.RemoveAll(runDir)
	for _, d := range []string{inputsDir, workDir, packDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return Result{}, err
		}
	}

	// Copy inputs to stable names
	leftDst := filepath.Join(inputsDir, "left.csv")
	rightDst := filepath.Join(inputsDir, "right.csv")
	if err := copyFile(cfg.LeftPath, leftDst); err != nil {
		return Result{}, err
	}
	if err := copyFile(cfg.RightPath, rightDst); err != nil {
		return Result{}, err
	}

	// Run recon
	reconCmd := exec.CommandContext(ctx, cfg.ReconBin,
		"run",
		"--left", leftDst,
		"--right", rightDst,
		"--out", workDir,
	)
	reconOut, reconErr := runCombined(reconCmd)

	// If recon fails, record deterministic evidence (but still pack it).
	if reconErr != nil {
		errPath := filepath.Join(treeDir, "error.txt")
		_ = os.WriteFile(errPath, []byte(reconOut), 0o644)
	}

	// Always build pack (success OR failure)
	auditCmd := exec.CommandContext(ctx, cfg.AuditpackBin,
		"run",
		"--in", treeDir,
		"--out", packDir,
		"--label", cfg.Label,
	)
	auditOut, auditErr := runCombined(auditCmd)
	if auditErr != nil {
		return Result{RunDir: runDir, TreeDir: treeDir, PackDir: packDir},
			fmt.Errorf("auditpack run failed: %w\n%s", auditErr, auditOut)
	}

	// Verify pack
	verifyCmd := exec.CommandContext(ctx, cfg.AuditpackBin, "verify", "--pack", packDir)
	verifyOut, verifyErr := runCombined(verifyCmd)
	if verifyErr != nil {
		return Result{RunDir: runDir, TreeDir: treeDir, PackDir: packDir},
			fmt.Errorf("auditpack verify failed: %w\n%s", verifyErr, verifyOut)
	}

	if reconErr != nil {
		return Result{RunDir: runDir, TreeDir: treeDir, PackDir: packDir},
			fmt.Errorf("recon failed (pack still produced + verified). See tree/error.txt\n%s", reconOut)
	}

	return Result{RunDir: runDir, TreeDir: treeDir, PackDir: packDir}, nil
}

func runCombined(cmd *exec.Cmd) (string, error) {
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}
