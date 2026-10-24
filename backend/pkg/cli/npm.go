package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

// NpmInstallParams describes a single npm install invocation.
type NpmInstallParams struct {
	NodePath   string   // absolute path to the bundled node binary
	NpmPath    string   // absolute path to the bundled npm executable
	Source     string   // npm package name (e.g. "lark-cli", "@scope/pkg") or absolute local dir
	TargetDir  string   // {data_dir}/plugins/cli/{name}; --prefix lands here
	CLIDataDir string   // private configuration/cache root; defaults to TargetDir/.runtime-data
	ParentEnv  []string // typically os.Environ(); BuildEnv-style scrubbing is applied
}

// NpmInstall runs `npm install <source> --prefix <targetDir> --no-audit --no-fund` using the bundled npm.
// Configuration, caches, logs and temporary files use the private data directory.
// Network/certificate environment settings are still inherited.
func NpmInstall(ctx context.Context, p NpmInstallParams) error {
	if p.NodePath == "" {
		return errors.New("npm: NodePath required")
	}
	if p.NpmPath == "" {
		return errors.New("npm: NpmPath required")
	}
	if p.Source == "" {
		return errors.New("npm: Source required")
	}
	if p.TargetDir == "" {
		return errors.New("npm: TargetDir required")
	}
	if !filepath.IsAbs(p.TargetDir) {
		return errors.New("npm: TargetDir must be absolute")
	}
	if err := os.MkdirAll(p.TargetDir, 0o755); err != nil {
		return err
	}

	dataDir := p.CLIDataDir
	if dataDir == "" {
		dataDir = filepath.Join(p.TargetDir, ".runtime-data")
	}
	env, err := PrepareEnv(EnvParams{
		Isolation:  IsolationIsolated,
		CLIDataDir: dataDir,
		RuntimeBin: filepath.Dir(p.NpmPath),
		PluginBin:  filepath.Join(p.TargetDir, "node_modules", ".bin"),
		ParentEnv:  p.ParentEnv,
	})
	if err != nil {
		return err
	}

	args := []string{
		"install",
		p.Source,
		"--prefix", p.TargetDir,
		"--no-audit",
		"--no-fund",
	}

	// node cannot execute the Windows npm.cmd shell wrapper.
	npmScript := p.NpmPath
	if filepath.Ext(npmScript) == ".cmd" {
		npmScript = filepath.Join(filepath.Dir(npmScript), "node_modules", "npm", "bin", "npm-cli.js")
	}
	res, err := Run(ctx, RunParams{
		Executable: p.NodePath,
		Argv:       append([]string{npmScript}, args...),
		Env:        env,
		WorkingDir: p.TargetDir,
		OutputMode: OutputText,
		TimeoutSec: MaxTimeoutSeconds, // npm installs of larger packages can take minutes
	})
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return errors.New("npm install failed: " + res.Stderr)
	}
	return nil
}
