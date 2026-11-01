//go:build !windows

package checker

import (
	"context"
	"os"
)

func checkpointOpenContext(ctx context.Context, path string) (*os.File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, checkpointContextError(ctx, err)
	}
	return f, nil
}

func checkpointReplaceContext(ctx context.Context, oldpath, newpath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(oldpath, newpath); err != nil {
		return checkpointContextError(ctx, err)
	}
	return nil
}
