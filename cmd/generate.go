/*
Copyright © 2023 Chris Collins 'collins.christopher@gmail.com'
*/
package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"

	pkgconfig "github.com/clcollins/srepd/pkg/config"
	"github.com/clcollins/srepd/pkg/launcher"
	"github.com/spf13/cobra"
)

var (
	generateOut   string
	generateForce bool
)

// configGenerateCmd prints a complete, annotated config with every supported
// key at its default value (#324) — for users who prefer editing a file over
// the wizard. A generated file routes into the wizard on next launch (OB-1)
// because its required keys are empty.
var configGenerateCmd = &cobra.Command{
	Use:          "generate",
	Short:        "Print a complete annotated config with default values",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigGenerate(cmd.OutOrStdout(), generateOut, generateForce)
	},
}

func init() {
	configGenerateCmd.Flags().StringVar(&generateOut, "out", "", "write to a file instead of stdout")
	configGenerateCmd.Flags().BoolVar(&generateForce, "force", false, "overwrite an existing --out file")
	configCmd.AddCommand(configGenerateCmd)
}

// runConfigGenerate writes the annotated config to w, or to outPath (0600,
// refusing to overwrite unless force) when outPath is non-empty.
func detectEnvironment() *pkgconfig.GenerateEnvironment {
	env := &pkgconfig.GenerateEnvironment{}

	detected := launcher.DetectTerminals(exec.LookPath, os.Getenv, runtime.GOOS)
	for _, dt := range detected {
		env.Terminals = append(env.Terminals, dt.Command)
	}

	if e := os.Getenv("EDITOR"); e != "" {
		env.Editor = e
	} else if v := os.Getenv("VISUAL"); v != "" {
		env.Editor = v
	}

	if _, err := exec.LookPath("claude"); err == nil {
		env.AgentCLI = pkgconfig.DefaultOptionalKeys["agent_cli_command"]
	}

	// Cluster login: prefer ocm backplane, offer ocm-container as alternative
	if _, err := exec.LookPath("ocm"); err == nil {
		env.ClusterLoginCmds = append(env.ClusterLoginCmds, "ocm backplane login %%CLUSTER_ID%%")
	}
	if _, err := exec.LookPath("ocm-container"); err == nil {
		env.ClusterLoginCmds = append(env.ClusterLoginCmds, "ocm-container --cluster-id %%CLUSTER_ID%%")
	}

	return env
}

func runConfigGenerate(w io.Writer, outPath string, force bool) error {
	data := pkgconfig.GenerateAnnotatedConfig(detectEnvironment())

	if outPath == "" {
		_, err := w.Write(data)
		return err
	}

	if _, err := os.Stat(outPath); err == nil && !force {
		return fmt.Errorf("%s already exists — use --force to overwrite", outPath)
	}

	// 0600 up front: the user will paste a PagerDuty token into this file.
	if err := os.WriteFile(outPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write %s: %w", outPath, err)
	}
	if err := os.Chmod(outPath, 0600); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", outPath, err)
	}
	return nil
}
