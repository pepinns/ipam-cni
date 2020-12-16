package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ipam"
)

type DummyCni struct {
	Log *log.Logger
}

type dummyConf struct {
	types.NetConf
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

	err = result.PrintTo(me.Log.Writer())
	if err != nil {
		me.Log.Printf("Error during result.PrintTo: %s", err)
	}
	return types.PrintResult(result, config.CNIVersion)
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

func main() {
	podName := os.Getenv("K8S_POD_NAME")
	logger := log.New(
		os.Stderr,
		"dummy-cni ("+podName+") ", log.Ldate|log.LstdFlags)
	myCni := &DummyCni{
		Log: logger,
	}
	skel.PluginMain(WrapSkel(myCni.Add), WrapSkel(myCni.Check), WrapSkel(myCni.Delete), version.All, "dummy-cni to fetch an IP from IPAM without creating inter")
}
