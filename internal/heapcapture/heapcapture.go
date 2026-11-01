// Package heapcapture carries an opt-in diagnostic hook across checker calls.
package heapcapture

import "context"

type key struct{}

// With installs a hook invoked at the end of the first available search.
func With(ctx context.Context, capture func()) context.Context {
	return context.WithValue(ctx, key{}, capture)
}

// Enabled reports whether a diagnostic hook was installed.
func Enabled(ctx context.Context) bool {
	_, ok := ctx.Value(key{}).(func())
	return ok
}

// Capture invokes the diagnostic hook, if one was installed.
func Capture(ctx context.Context) {
	if capture, ok := ctx.Value(key{}).(func()); ok {
		capture()
	}
}
