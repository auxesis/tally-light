// Command tally-light turns a Hue lamp red while a chosen camera is in use.
package main

import (
	"fmt"
	"os"
)

const version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version":
		fmt.Println("tally-light " + version)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: tally-light <version>")
}
