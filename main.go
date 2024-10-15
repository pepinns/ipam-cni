package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/containernetworking/cni/pkg/version"
	"log"
	"os"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ipam"
)

var logger *log.Logger

// Initialize the logger with log level and log file
func initLogger(logFile string, logLevel string) {
	// Set default log file if not provided
	if logFile == "" {
		logFile = "/opt/cni/bin/dummy-cni.log"
	}

	// Open log file
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	// Set logger based on log level
	logger = log.New(file, "dummy-cni: ", log.LstdFlags|log.Lshortfile)
	if logLevel == "DEBUG" {
		logger.SetFlags(log.LstdFlags | log.Lshortfile)
	}
}

// dummyConf is a simple struct to capture any passed CNI configuration
type dummyConf struct {
	types.NetConf
	LogLevel string `json:"log_level"`
	LogFile  string `json:"log_file"`
}

// Load the CNI configuration from stdin
func loadConfigFile(bytes []byte) (*dummyConf, error) {
	conf := &dummyConf{}
	if err := json.Unmarshal(bytes, conf); err != nil {
		return nil, fmt.Errorf("Failed to load configuration data, error = %+v", err)
	}
	initLogger(conf.LogFile, conf.LogLevel)
	return conf, nil
}

// cmdAdd handles the ADD command
func cmdAdd(args *skel.CmdArgs) error {

	// Load and log the configuration
	conf, err := loadConfigFile(args.StdinData)
	if err != nil {
		logger.Printf("Error loading configuration: %v", err)
		return err
	}
	logger.Printf("CNI Configuration: %+v", conf)

	// Initialize an L2 default result.
	result := &current.Result{
		CNIVersion: conf.CNIVersion,
		Interfaces: []*current.Interface{
			{
				Name:    conf.Name,
				Sandbox: args.Netns,
			},
		},
	}

	// run the IPAM plugin and get back the config to apply
	r, err := ipam.ExecAdd(conf.IPAM.Type, args.StdinData)
	if err != nil {
		return err
	}
	ipamResult, err := current.NewResultFromResult(r)
	if err != nil {
		return err
	}
	if len(ipamResult.IPs) == 0 {
		return errors.New("IPAM plugin returned missing IP config")
	}
	logger.Printf("IPAM Result: %+v", ipamResult)
	for _, ipc := range ipamResult.IPs {
		// all addresses belong to the same interface
		ipc.Interface = current.Int(0)
	}
	result.IPs = ipamResult.IPs
	result.Routes = ipamResult.Routes
	// logging the result before returning it
	logger.Printf("Result: %+v", result)

	// Return the result as JSON
	return types.PrintResult(result, conf.CNIVersion)
}

// cmdDel handles the DEL command
func cmdDel(args *skel.CmdArgs) error {
	conf, err := loadConfigFile(args.StdinData)
	if err != nil {
		return err
	}
	err = ipam.ExecDel(conf.IPAM.Type, args.StdinData)
	if err != nil {
		return err
	}
	return err
}

// cmdCheck handles the CHECK command
func cmdCheck(args *skel.CmdArgs) error {
	return nil
}

func main() {
	// Use the skel.PluginMainFuncs pattern to register the Add, Check, and Del commands
	skel.PluginMainFuncs(skel.CNIFuncs{Add: cmdAdd, Del: cmdDel, Check: cmdCheck}, version.All, "A simple CNI plugin that logs inputs and returns valid JSON.")

}
