//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Build builds Caddy with the etcd storage module
func Build() error {
	fmt.Println("Building Caddy with etcd storage module...")
	
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %v", err)
	}

	// Create xcaddy.go in a temp directory
	tmpDir, err := os.MkdirTemp("", "caddy-build")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	xcaddyFile := filepath.Join(tmpDir, "xcaddy.go")
	xcaddyContent := fmt.Sprintf(`package main

import (
	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	_ "github.com/BTBurke/caddy-etcd"
)

func main() {
	caddycmd.Main()
}`)

	if err := os.WriteFile(xcaddyFile, []byte(xcaddyContent), 0644); err != nil {
		return fmt.Errorf("failed to write xcaddy.go: %v", err)
	}

	// Build using the temp xcaddy.go
	buildCmd := fmt.Sprintf("go build -o %s/caddy %s", cwd, xcaddyFile)
	if err := sh.Run("sh", "-c", buildCmd); err != nil {
		return fmt.Errorf("build failed: %v", err)
	}

	fmt.Println("Build successful! The caddy binary is in the current directory")
	return nil
}

// Test runs the test suite
func Test() error {
	return sh.Run("go", "test", "-v", "./...")
}

// Clean removes build artifacts
func Clean() error {
	return sh.Run("rm", "-f", "caddy")
}
