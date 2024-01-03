// Copyright 2023 IronCore authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exec

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"github.com/ironcore-dev/vgopath/internal/link"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	var (
		opts   link.Options
		dstDir string
		shell  bool
	)

	cmd := &cobra.Command{
		Use:   "exec -- command [args...]",
		Short: "Run an executable in a virtual GOPATH.",
		Args: func(cmd *cobra.Command, args []string) error {
			if !shell {
				return cobra.MinimumNArgs(1)(cmd, args)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			executable, executableArgs := executableAndArgs(args, shell)
			return Run(dstDir, executable, opts, executableArgs)
		},
	}

	opts.AddFlags(cmd.Flags())
	cmd.Flags().StringVarP(&dstDir, "dst-dir", "o", "", "Destination directory. If empty, a temporary directory will be created.")
	cmd.Flags().BoolVarP(&shell, "shell", "s", false, "Whether to run the command in a shell.")

	return cmd
}

func executableAndArgs(args []string, doShell bool) (string, []string) {
	if !doShell {
		return args[0], args[1:]
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	return shell, []string{"-c", args[0]}
}

func Run(dstDir, executable string, opts link.Options, args []string) error {
	if dstDir == "" {
		var err error
		dstDir, err = os.MkdirTemp("", "vgopath")
		if err != nil {
			return fmt.Errorf("error creating temp directory: %w", err)
		}
		defer func() { _ = os.RemoveAll(dstDir) }()
	}

	if err := link.Link(dstDir, opts); err != nil {
		return err
	}

	cmd := exec.Command(executable, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = dstDir

	cmd.Env = mkEnv(dstDir)
	return cmd.Run()
}

var filterEnvRegexp = regexp.MustCompile(`^(GOPATH|GO111MODULE)=`)

func mkEnv(gopath string) []string {
	env := os.Environ()
	res := make([]string, 0, len(env)+2)

	for _, kv := range env {
		if !filterEnvRegexp.MatchString(kv) {
			res = append(res, kv)
		}
	}

	return append(res,
		fmt.Sprintf("GOPATH=%s", gopath),
		"GO111MODULE=off",
	)
}
