package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	name := filepath.Base(os.Args[0])
	if name == "git" {
		switch strings.Join(os.Args[1:], " ") {
		case "--version":
			fmt.Println("git version test")
		case "rev-parse --show-toplevel":
			cwd, err := os.Getwd()
			if err != nil {
				os.Exit(1)
			}
			fmt.Println(cwd)
		case "rev-parse --verify HEAD^{commit}":
			fmt.Println("commit")
		default:
			os.Exit(1)
		}
		return
	}
	if name == "archon" && len(os.Args) == 2 && os.Args[1] == "doctor" {
		fmt.Println("Installed providers: " + os.Getenv("DEVKIT_INSTALL_TEST_PROVIDERS"))
		return
	}
	fmt.Println("test-version")
}
