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

package vgopath

import (
	"os"

	"github.com/ironcore-dev/vgopath/internal/cmd/version"
	"github.com/ironcore-dev/vgopath/internal/cmd/vgopath/exec"
	"github.com/ironcore-dev/vgopath/internal/link"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	var (
		opts   link.Options
		dstDir string
	)

	cmd := &cobra.Command{
		Use:   "vgopath",
		Short: "Create and operate on virtual GOPATHs",
		Long: `Create a 'virtual' GOPATH at the specified directory.

vgopath will setup a GOPATH folder structure, ensuring that any tool used
to the traditional setup will function as normal.

The target module will be mirrored to where its go.mod path (the line
after 'module') points at.
`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(dstDir, opts)
		},
	}

	opts.AddFlags(cmd.Flags())
	cmd.Flags().StringVarP(&dstDir, "dst-dir", "o", "", "Destination directory.")
	_ = cmd.MarkFlagRequired("dst-dir")

	cmd.AddCommand(
		exec.Command(),
		version.Command(os.Stdout),
	)

	return cmd
}

func Run(dstDir string, opts link.Options) error {
	return link.Link(dstDir, opts)
}
