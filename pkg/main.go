// main.go
// This file contains the main entry point for the router

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"github.com/wadawe/request-router/pkg/backend"
	"github.com/wadawe/request-router/pkg/config"
	"github.com/wadawe/request-router/pkg/service"
	"github.com/wadawe/request-router/pkg/utils"
)

// Params filled by build.go script
var (
	buildVersion string // Version is the app `X.Y.Z(-abc)?`` version
	buildCommit  string // Commit is the git commit sha1
)

// Command line flags
var (
	pidFileFlag    = flag.String("pidfile", "", "Path to pid file")
	configFileFlag = flag.String("config", "", "Configuration file to use")
	logDirFlag     = flag.String("log-dir", "", "Default log directory")
	agentFlag      = flag.String("agent", "request-router", "User agent value to use for database requests")
	helpFlag       = flag.Bool("help", false, "Print this help message")
	dryRunFlag     = flag.Bool("dry-run", false, "Run the service in dry-run mode")
	versionFlag    = flag.Bool("version", false, "Print the current router version")
)

var (
	routerManager *service.RouterManager
	stopping      bool = false
)

func main() {
	var err error
	log.Printf("request-router v%s (git: %s)", buildVersion, buildCommit)
	handleFlags()
	writePIDFile()
	config.SetVersion(buildVersion)
	config.SetUserAgent(*agentFlag)
	utils.SetupLogDirectory(logDirFlag)

	// Read configuration
	cfg, err := config.ReadConfig(*configFileFlag)
	if err != nil {
		log.Fatalf("Error on config: %s", err)
	}

	// Handle signals
	SigMutex := &sync.Mutex{}
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGINT)
	go func() {

		// Continuously listen for signals and act accordingly
		for sig := range c {
			SigMutex.Lock()
			switch sig {

			// Handle termination signals
			case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT: // 15,3,2
				log.Printf("Received signal (%q): stopping...", sig)
				stopping = true
				routerManager.Stop()
				utils.CloseLogHandlers()
				os.Exit(0)

			// Handle reload signal
			case syscall.SIGHUP: // 1
				log.Printf("Received signal (%q): reloading...", sig)
				err := ReloadBackend()
				if err != nil {
					log.Fatalf("Error on router reload: %s", err)
				} else {
					log.Printf("Router reloaded successfully!")
				}

			// Handle unknown signals
			default:
				log.Printf("Received signal (%q): doing nothing...", sig)
			}
			SigMutex.Unlock()
		}

	}()

	err = StartRouterService(cfg)
	if err != nil {
		log.Fatalf("Error on router start: %s", err)
	}
}

// Handle command line flags
func handleFlags() {
	flag.Usage = func() {
		fmt.Println("See the README.md for more information about the request-router.")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *versionFlag {
		// Version already printed in main()
		os.Exit(0)
	}
	if *helpFlag {
		flag.PrintDefaults()
		os.Exit(0)
	}
}

// Write the PID to the specified file
func writePIDFile() {
	if *pidFileFlag == "" {
		return
	}

	err := os.MkdirAll(filepath.Dir(*pidFileFlag), 0700)
	if err != nil {
		log.Fatalf("Error on pidfile create: %s", err)
	}
	pid := strconv.Itoa(os.Getpid())
	log.Printf("Writing pid %s to %s", pid, *pidFileFlag)
	if err := os.WriteFile(*pidFileFlag, []byte(pid), 0644); err != nil {
		log.Fatalf("Error on pidfile write: %s", err)
	}
}

// Start the RouterService
func StartRouterService(cfg *config.ConfigFile) error {
	log.Printf("Starting router service v%s (main.StartRouterService)", buildVersion)

	var err error
	routerManager, err = service.NewRouterManager(cfg)
	if err != nil {
		return err
	}

	if *dryRunFlag {
		log.Println("Dry run mode: exiting early!")
		os.Exit(0)
	}

	// Keep restarting routerManager.Start() unless a stop signal is received
	// This ensures the service remains available if Start() ever exits
	for {
		if stopping {
			break
		}
		routerManager.Start()
	}
	return nil
}

// Reload the backend configuration
func ReloadBackend() error {
	log.Printf("Reloading router config v%s (main.ReloadBackend)", buildVersion)
	newcfg, err := config.ReadConfig(*configFileFlag)
	if err != nil {
		return err
	}
	return backend.LoadConfig(newcfg)
}
