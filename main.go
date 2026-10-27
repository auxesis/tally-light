// Command tally-light turns a Hue lamp red while a chosen camera is in use.
package main

import (
	"fmt"
	"os"

	"github.com/auxesis/tally-light/internal/camera"
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
	case "list-cameras":
		listCameras()
	case "run":
		run(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: tally-light <run|list-cameras|version>")
}

func listCameras() {
	cams, err := camera.List()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, c := range cams {
		state := "not in use"
		if c.InUse {
			state = "in use"
		}
		fmt.Printf("%s\t%s\t%s\n", c.Name, c.UID, state)
	}
}
