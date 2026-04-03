package main

import (
	"fmt"
	"runtime"
)

// printVersion outputs version information including build-time details.
func printVersion() {
	fmt.Printf("glpictl-ai %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
}
