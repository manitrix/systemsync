//go:build !linux

package system

import (
	"context"
	"fmt"
	"github.com/manitrix/systemsync/pkg/types"
	"runtime"
	"time"
)

func Collect(context.Context, time.Duration) (types.System, error) {
	return types.System{}, fmt.Errorf("Linux /proc and statfs required; current OS is %s", runtime.GOOS)
}
