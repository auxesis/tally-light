// Command tally-light turns a Hue lamp red while a chosen camera is in use.
package main

import (
	"fmt"
	"os"
	"os/user"

	"github.com/auxesis/tally-light/internal/camera"
	"github.com/auxesis/tally-light/internal/config"
	"github.com/auxesis/tally-light/internal/service"
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
	case "install":
		install()
	case "uninstall":
		uninstall()
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: tally-light <run|install|uninstall|list-cameras|version>")
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

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func install() {
	if os.Geteuid() != 0 {
		fail("run with sudo")
	}
	name := os.Getenv("SUDO_USER")
	if name == "" || name == "root" {
		fail("run with sudo from the account the service should run as")
	}
	u, err := user.Lookup(name)
	if err != nil {
		fail("looking up %s: %v", name, err)
	}
	// A broken config would make launchd restart the service in a loop.
	if _, err := config.Load(config.DefaultPath(u.HomeDir)); err != nil {
		fail("%v", err)
	}
	self, err := os.Executable()
	if err != nil {
		fail("finding own binary: %v", err)
	}
	if err := service.Install(self, name, u.HomeDir); err != nil {
		fail("%v", err)
	}
	fmt.Printf("installed %s, running as %s\nlogs: %s/Library/Logs/tally-light.log\n", service.Label, name, u.HomeDir)
}

func uninstall() {
	if os.Geteuid() != 0 {
		fail("run with sudo")
	}
	if err := service.Uninstall(); err != nil {
		fail("%v", err)
	}
	fmt.Println("uninstalled " + service.Label)
}
