package main

import (
	"os"
	"testing"
)

func TestMainHelp(_ *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"errant", "--help"}

	main()
}
