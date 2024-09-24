package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/containernetworking/cni/pkg/version"
	"io"
	"log"
	"os"

	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ipam"
)

type DummyCni struct {
	Log *log.Logger
}

type dummyConf struct {
	cnitypes.NetConf
}

func loadConfigFile(bytes []byte) (*dummyConf, error) {
	conf := &dummyConf{}
	if err := json.Unmarshal(bytes, conf); err != nil {
		return nil, fmt.Errorf("Failed to load configuration data, error = %+v", err)
	}
	return conf, nil
}

func WrapSkel(callBack func(*dummyConf, *skel.CmdArgs) error) func(*skel.CmdArgs) error {
	return func(args *skel.CmdArgs) error {
		conf, err := loadConfigFile(args.StdinData)
		if err != nil {
			return err
		}
		return callBack(conf, args)
	}
}

func (me *DummyCni) Add(config *dummyConf, args *skel.CmdArgs) error {
	// run the IPAM plugin and get back the config to apply
	me.Log.Printf("Got ADD Args=%s container=%s ifname=%s netns=%s path=%s \nstdindata:\n%s\n", args.Args, args.ContainerID, args.IfName, args.Netns, args.Path, args.StdinData)
	r, err := ipam.ExecAdd(config.IPAM.Type, args.StdinData)
	if err != nil {
		me.Log.Printf("Error during ExecAdd: %s", err)
		return err
	}
	// Convert whatever the IPAM result was into the current Result type
	result, err := current.NewResultFromResult(r)
	if err != nil {
		me.Log.Printf("Error during NewResultFromResult: %s", err)
		return err
	}

	me.Log.Printf("Got result version %s \n%+v", result.CNIVersion, result)
	if len(result.IPs) == 0 {
		me.Log.Printf("NO IPs returned %+v", result)
		return errors.New("IPAM plugin returned missing IP config")
	}
	result.Interfaces = []*current.Interface{{
		Name: config.Name,
	}}

	for _, ip := range result.IPs {
		me.Log.Printf("Got IP: %s", ip.String())
		ip.Interface = current.Int(0)
	}

	for _, route := range result.Routes {
		me.Log.Printf("Got Route: %s", route.GW.String())
	}

	err = result.PrintTo(me.Log.Writer())
	if err != nil {
		me.Log.Printf("Error during result.PrintTo: %s", err)
	}
	return cnitypes.PrintResult(result, config.CNIVersion)
}

func (me *DummyCni) Delete(config *dummyConf, args *skel.CmdArgs) error {
	err := ipam.ExecDel(config.IPAM.Type, args.StdinData)
	if err != nil {
		return err
	}
	return nil
}

func (me *DummyCni) Check(config *dummyConf, args *skel.CmdArgs) error {
	err := ipam.ExecCheck(config.IPAM.Type, args.StdinData)
	if err != nil {
		return err
	}
	return nil
}

// initLogger initializes a logger that writes to both a file and stderr
func initLogger(logFilePath string, podName string) (*log.Logger, error) {
	// Open the log file for writing, create it if it doesn't exist
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("Failed to open log file: %v", err)
	}

	// Create a multi-writer that writes to both stderr and the file
	multiWriter := io.MultiWriter(os.Stderr, logFile)

	// Create a logger that writes to the multi-writer
	logger := log.New(multiWriter, "dummy-cni ("+podName+") ", log.Ldate|log.Ltime|log.Lshortfile)

	return logger, nil
}

func main() {
	// Get the pod name from the environment variable
	podName := os.Getenv("K8S_POD_NAME")

	// Initialize the logger to log to both stderr and a log file
	logger, err := initLogger("/opt/cni/bin/dummy-cni.log", podName)
	if err != nil {
		// If the logger initialization fails, log the error and exit
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Create a DummyCni instance with the initialized logger
	myCni := &DummyCni{
		Log: logger,
	}

	// Set up the CNI plugin commands: Add, Check, and Delete
	skel.PluginMain(WrapSkel(myCni.Add), WrapSkel(myCni.Check), WrapSkel(myCni.Delete), version.All, "dummy-cni plugin")
}
