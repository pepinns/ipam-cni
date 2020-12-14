package main

import (
	"errors"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ipam"
)

type IpamCni struct {
	CniVersion string
	PluginType string
}

func (me *IpamCni) Add(args *skel.CmdArgs) error {
	// run the IPAM plugin and get back the config to apply
	r, err := ipam.ExecAdd(me.PluginType, args.StdinData)
	if err != nil {
		return err
	}
	// Convert whatever the IPAM result was into the current Result type
	result, err := current.NewResultFromResult(r)
	if err != nil {
		return err
	}

	if len(result.IPs) == 0 {
		return errors.New("IPAM plugin returned missing IP config")
	}
	return types.PrintResult(result, me.CniVersion)
}
func (me *IpamCni) Delete(args *skel.CmdArgs) error {
	err := ipam.ExecDel(me.PluginType, args.StdinData)
	if err != nil {
		return err
	}
	return nil
}

func (me *IpamCni) Check(args *skel.CmdArgs) error {
	err := ipam.ExecCheck(me.PluginType, args.StdinData)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	myCni := &IpamCni{
		PluginType: "ip-only",
		CniVersion: "0.3.1",
	}
	skel.PluginMain(myCni.Add, myCni.Check, myCni.Delete, version.All, "")
}
