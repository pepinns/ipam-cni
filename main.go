package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/containernetworking/plugins/pkg/ipam"
	"log"
	"os"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
)

var logger *log.Logger

// dummyConf is a simple struct to capture any passed CNI configuration
type dummyConf struct {
	CNIVersion string `json:"cniVersion"`
	Name       string `json:"name"`
	types.NetConf
}

// Initialize the logger
func initLogger() {
	logFile, err := os.OpenFile("/opt/cni/bin/simple-cni.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	logger = log.New(logFile, "simple-cni: ", log.LstdFlags|log.Lshortfile)
}

// Load the CNI configuration from stdin
func loadConfigFile(stdinData []byte) (*dummyConf, error) {
	conf := &dummyConf{}
	if err := json.Unmarshal(stdinData, conf); err != nil {
		return nil, fmt.Errorf("Failed to load configuration data: %v", err)
	}
	return conf, nil
}

func logEnvVars() {
	logger.Println("Logging environment variables:")
	for _, env := range os.Environ() {
		if len(env) > 4 && env[:4] == "CNI_" {
			logger.Println(env)
		}
	}
}

func logArgs(args *skel.CmdArgs) {
	logger.Printf("Logging arguments:")
	logger.Printf("ContainerID: %s", args.ContainerID)
	logger.Printf("Netns: %s", args.Netns)
	logger.Printf("IfName: %s", args.IfName)
	logger.Printf("Args: %s", args.Args)
	logger.Printf("Path: %s", args.Path)
	logger.Printf("StdinData: %s", string(args.StdinData))
}

// cmdAdd handles the ADD command
func cmdAdd(args *skel.CmdArgs) error {
	logEnvVars()
	logArgs(args)

	// Load and log the configuration
	conf, err := loadConfigFile(args.StdinData)
	if err != nil {
		logger.Printf("Error loading configuration: %v", err)
		return err
	}
	logger.Printf("CNI Configuration: %+v", conf)

	// Initialize an L2 default result.
	result := &types100.Result{
		CNIVersion: conf.CNIVersion,
		Interfaces: []*types100.Interface{
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
	ipamResult, err := types100.NewResultFromResult(r)
	if err != nil {
		return err
	}
	if len(ipamResult.IPs) == 0 {
		return errors.New("IPAM plugin returned missing IP config")
	}
	logger.Printf("IPAM Result: %+v", ipamResult)
	for _, ipc := range ipamResult.IPs {
		// all addresses belong to the same interface
		ipc.Interface = types100.Int(0)
	}
	result.IPs = ipamResult.IPs
	result.Routes = ipamResult.Routes
	// logging the result before returning it
	logger.Printf("Result: %+v", result)

	// Return the result as JSON
	return types.PrintResult(result, conf.CNIVersion)
}

// cmdCheck handles the CHECK command
func cmdCheck(args *skel.CmdArgs) error {
	logEnvVars()
	logArgs(args)
	return nil
}

// cmdDel handles the DELETE command
func cmdDel(args *skel.CmdArgs) error {
	logEnvVars()
	logArgs(args)
	return nil
}

func main() {
	initLogger()
	logger.Println("Starting simple-cni")

	// Use the skel.PluginMainFuncs pattern to register the Add, Check, and Del commands
	skel.PluginMainFuncs(skel.CNIFuncs{Add: cmdAdd, Del: cmdDel, Check: cmdCheck}, version.All, "A simple CNI plugin that logs inputs and returns valid JSON.")
}
