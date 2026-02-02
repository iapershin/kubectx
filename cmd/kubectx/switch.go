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
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/ahmetb/kubectx/internal/cmdutil"
	"github.com/ahmetb/kubectx/internal/kubeconfig"
	"github.com/ahmetb/kubectx/internal/printer"
)

// SwitchOp indicates intention to switch contexts.
type SwitchOp struct {
	Target    string // '-' for back and forth, or NAME
	Namespace string // namespace to switch to after context switch (optional)
}

func (op SwitchOp) Run(_, stderr io.Writer) error {
	var newCtx string
	var err error
	if op.Target == "-" {
		newCtx, err = swapContext()
	} else {
		newCtx, err = switchContext(op.Target)
	}
	if err != nil {
		return errors.Wrap(err, "failed to switch context")
	}

	// Switch namespace if specified
	if op.Namespace != "" {
		kc := new(kubeconfig.Kubeconfig).WithLoader(kubeconfig.DefaultLoader)
		defer kc.Close()
		if err := kc.Parse(); err != nil {
			return errors.Wrap(err, "kubeconfig error")
		}

		toNS, err := switchNamespace(kc, newCtx, op.Namespace, false)
		if err != nil {
			return errors.Wrap(err, "failed to switch namespace")
		}
		err = printer.Success(stderr, "Switched to context \"%s\" and namespace \"%s\".",
			printer.SuccessColor.Sprint(newCtx), printer.SuccessColor.Sprint(toNS))
		return errors.Wrap(err, "print error")
	}

	err = printer.Success(stderr, "Switched to context \"%s\".", printer.SuccessColor.Sprint(newCtx))
	return errors.Wrap(err, "print error")
}

// switchContext switches to specified context name.
func switchContext(name string) (string, error) {
	prevCtxFile, err := kubectxPrevCtxFile()
	if err != nil {
		return "", errors.Wrap(err, "failed to determine state file")
	}

	kc := new(kubeconfig.Kubeconfig).WithLoader(kubeconfig.DefaultLoader)
	defer kc.Close()
	if err := kc.Parse(); err != nil {
		return "", errors.Wrap(err, "kubeconfig error")
	}

	prev := kc.GetCurrentContext()
	if !kc.ContextExists(name) {
		return "", errors.Errorf("no context exists with the name: \"%s\"", name)
	}
	if err := kc.ModifyCurrentContext(name); err != nil {
		return "", err
	}
	if err := kc.Save(); err != nil {
		return "", errors.Wrap(err, "failed to save kubeconfig")
	}

	if prev != name {
		if err := writeLastContext(prevCtxFile, prev); err != nil {
			return "", errors.Wrap(err, "failed to save previous context name")
		}
	}
	return name, nil
}

// swapContext switches to previously switch context.
func swapContext() (string, error) {
	prevCtxFile, err := kubectxPrevCtxFile()
	if err != nil {
		return "", errors.Wrap(err, "failed to determine state file")
	}
	prev, err := readLastContext(prevCtxFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to read previous context file")
	}
	if prev == "" {
		return "", errors.New("no previous context found")
	}
	return switchContext(prev)
}

// switchNamespace switches to the specified namespace in the given context.
func switchNamespace(kc *kubeconfig.Kubeconfig, ctx string, ns string, force bool) (string, error) {
	curNS, err := kc.NamespaceOfContext(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get current namespace")
	}

	f := NewNSFile(ctx)

	if !force {
		ok, err := namespaceExists(kc, ns)
		if err != nil {
			return "", errors.Wrap(err, "failed to query if namespace exists (is cluster accessible?)")
		}
		if !ok {
			return "", errors.Errorf("no namespace exists with name \"%s\"", ns)
		}
	}

	if err := kc.SetNamespace(ctx, ns); err != nil {
		return "", errors.Wrapf(err, "failed to change to namespace \"%s\"", ns)
	}
	if err := kc.Save(); err != nil {
		return "", errors.Wrap(err, "failed to save kubeconfig file")
	}
	if curNS != ns {
		if err := f.Save(curNS); err != nil {
			return "", errors.Wrap(err, "failed to save the previous namespace to file")
		}
	}
	return ns, nil
}

func namespaceExists(kc *kubeconfig.Kubeconfig, ns string) (bool, error) {
	// for tests
	if os.Getenv("_MOCK_NAMESPACES") != "" {
		return ns == "ns1" || ns == "ns2", nil
	}

	clientset, err := newKubernetesClientSet(kc)
	if err != nil {
		return false, errors.Wrap(err, "failed to initialize k8s REST client")
	}

	namespace, err := clientset.CoreV1().Namespaces().Get(context.Background(), ns, metav1.GetOptions{})
	if errors2.IsNotFound(err) {
		return false, nil
	}
	return namespace != nil, errors.Wrapf(err, "failed to query "+
		"namespace %q from k8s API", ns)
}

func newKubernetesClientSet(kc *kubeconfig.Kubeconfig) (*kubernetes.Clientset, error) {
	b, err := kc.Bytes()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert in-memory kubeconfig to yaml")
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig(b)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize config")
	}
	return kubernetes.NewForConfig(cfg)
}

// NSFile manages namespace state files for contexts.
type NSFile struct {
	dir string
	ctx string
}

var nsFileDefaultDir = filepath.Join(cmdutil.HomeDir(), ".kube", "kubens")

func NewNSFile(ctx string) NSFile {
	return NSFile{dir: nsFileDefaultDir, ctx: ctx}
}

func (f NSFile) path() string {
	fn := f.ctx
	if isWindows() {
		// bug 230: eks clusters contain ':' in ctx name, not a valid file name for win32
		fn = strings.ReplaceAll(fn, ":", "__")
	}
	return filepath.Join(f.dir, fn)
}

// Load reads the previous namespace setting, or returns empty if not exists.
func (f NSFile) Load() (string, error) {
	b, err := ioutil.ReadFile(f.path())
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(bytes.TrimSpace(b)), nil
}

// Save stores the previous namespace information in the file.
func (f NSFile) Save(value string) error {
	d := filepath.Dir(f.path())
	if err := os.MkdirAll(d, 0755); err != nil {
		return err
	}
	return ioutil.WriteFile(f.path(), []byte(value), 0644)
}

// isWindows determines if the process is running on windows OS.
func isWindows() bool {
	if os.Getenv("_FORCE_GOOS") == "windows" { // for testing
		return true
	}
	return runtime.GOOS == "windows"
}
