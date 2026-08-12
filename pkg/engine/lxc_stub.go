//go:build !cgo

package engine

import (
	"context"
	"fmt"
	"io"
)

func NewLXCEngine(socketPath string) Engine {
	return &lxcStub{}
}

type lxcStub struct{}

func (l *lxcStub) Name() string                                              { return "lxc" }
func (l *lxcStub) ID() string                                                { return "" }
func (l *lxcStub) CreateEnvironment(_ context.Context, _ string, _ []Mount) error { return fmt.Errorf("lxc engine requires cgo and liblxc-dev") }
func (l *lxcStub) Run(_ context.Context, _ RunConfig) error                  { return fmt.Errorf("lxc engine requires cgo and liblxc-dev") }
func (l *lxcStub) RunOutput(_ context.Context, _ RunConfig) (string, error)  { return "", fmt.Errorf("lxc engine requires cgo and liblxc-dev") }
func (l *lxcStub) CopyTo(_ context.Context, _ string, _ string) error        { return fmt.Errorf("lxc engine requires cgo and liblxc-dev") }
func (l *lxcStub) CopyFrom(_ context.Context, _ string, _ string) error      { return fmt.Errorf("lxc engine requires cgo and liblxc-dev") }
func (l *lxcStub) CopyTarStream(_ context.Context, _ io.Reader, _ string) error { return fmt.Errorf("lxc engine requires cgo and liblxc-dev") }
func (l *lxcStub) Destroy(_ context.Context) error                            { return nil }
