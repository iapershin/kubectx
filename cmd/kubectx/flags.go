// Copyright 2021 Google LLC
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

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ahmetb/kubectx/internal/cmdutil"
)

// UnsupportedOp indicates an unsupported flag.
type UnsupportedOp struct{ Err error }

func (op UnsupportedOp) Run(_, _ io.Writer) error {
	return op.Err
}

// parseArgs looks at flags (excl. executable name, i.e. argv[0])
// and decides which operation should be taken.
func parseArgs(argv []string) Op {
	if len(argv) == 0 {
		if cmdutil.IsInteractiveMode(os.Stdout) {
			return InteractiveSwitchOp{SelfCmd: os.Args[0]}
		}
		return ListOp{}
	}

	if argv[0] == "-d" {
		if len(argv) == 1 {
			if cmdutil.IsInteractiveMode(os.Stdout) {
				return InteractiveDeleteOp{SelfCmd: os.Args[0]}
			} else {
				return UnsupportedOp{Err: fmt.Errorf("'-d' needs arguments")}
			}
		}
		return DeleteOp{Contexts: argv[1:]}
	}

	var namespace string
	var namespaceIndex = -1

	for i := 0; i < len(argv); i++ {
		if argv[i] == "-n" || argv[i] == "--namespace" {
			if i+1 >= len(argv) {
				return UnsupportedOp{Err: fmt.Errorf("'-n' requires a namespace argument")}
			}
			namespace = argv[i+1]
			namespaceIndex = i
			break
		}
	}

	var remainingArgs []string
	if namespaceIndex >= 0 {
		remainingArgs = append(argv[:namespaceIndex], argv[namespaceIndex+2:]...)
	} else {
		remainingArgs = argv
	}

	if len(remainingArgs) == 0 {
		if namespace != "" {
			return UnsupportedOp{Err: fmt.Errorf("context name is required when using -n flag")}
		}
		if cmdutil.IsInteractiveMode(os.Stdout) {
			return InteractiveSwitchOp{SelfCmd: os.Args[0]}
		}
		return ListOp{}
	}

	if len(remainingArgs) == 1 {
		v := remainingArgs[0]
		if v == "--help" || v == "-h" {
			return HelpOp{}
		}
		if v == "--version" || v == "-V" {
			return VersionOp{}
		}
		if v == "--current" || v == "-c" {
			return CurrentOp{}
		}
		if v == "--unset" || v == "-u" {
			return UnsetOp{}
		}

		if new, old, ok := parseRenameSyntax(v); ok {
			return RenameOp{New: new, Old: old}
		}

		if strings.HasPrefix(v, "-") && v != "-" {
			return UnsupportedOp{Err: fmt.Errorf("unsupported option '%s'", v)}
		}
		return SwitchOp{Target: v, Namespace: namespace}
	}
	return UnsupportedOp{Err: fmt.Errorf("too many arguments")}
}
