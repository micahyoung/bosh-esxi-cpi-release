package action

import (
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
	"github.com/cppforlife/bosh-cpi-go/apiv1"

	"bosh-govmomi-cpi/govc"
	"bosh-govmomi-cpi/vm"
)

type CreateVMMethod struct {
	govcClient      govc.GovcClient
	agentSettings   vm.AgentSettings
	agentOptions    apiv1.AgentOptions
	agentEnvFactory apiv1.AgentEnvFactory
	uuidGen         boshuuid.Generator
	logger          boshlog.Logger
}

func NewCreateVMMethod(govcClient govc.GovcClient, agentSettings vm.AgentSettings, agentOptions apiv1.AgentOptions, agentEnvFactory apiv1.AgentEnvFactory, uuidGen boshuuid.Generator, logger boshlog.Logger) CreateVMMethod {
	return CreateVMMethod{
		govcClient:      govcClient,
		agentSettings:   agentSettings,
		agentOptions:    agentOptions,
		agentEnvFactory: agentEnvFactory,
		uuidGen:         uuidGen,
		logger:          logger,
	}
}

func (c CreateVMMethod) CreateVM(
	agentID apiv1.AgentID, stemcellCID apiv1.StemcellCID,
	cloudProps apiv1.VMCloudProps, networks apiv1.Networks,
	associatedDiskCIDs []apiv1.DiskCID, vmEnv apiv1.VMEnv) (apiv1.VMCID, error) {

	vmUuid, _ := c.uuidGen.Generate()
	newVMCID := apiv1.NewVMCID(vmUuid)

	stemcellId := "cs-" + stemcellCID.AsString()
	vmId := "vm-" + vmUuid

	var vmProps vm.VMProps
	err := cloudProps.As(&vmProps)
	if err != nil {
		return newVMCID, err
	}

	_, err = c.govcClient.CloneVM(stemcellId, vmId)
	if err != nil {
		return newVMCID, err
	}

	err = c.govcClient.SetVMResources(vmId, vmProps.CPU, vmProps.RAM)
	if err != nil {
		return newVMCID, err
	}

	err = c.govcClient.SetVMNetworkAdapters(vmId, len(networks))
	if err != nil {
		return newVMCID, err
	}

	agentEnv := c.agentEnvFactory.ForVM(agentID, newVMCID, networks, vmEnv, c.agentOptions)
	agentEnv.AttachSystemDisk("0")

	err = c.govcClient.CreateEphemeralDisk(vmId, vmProps.Disk)
	if err != nil {
		return newVMCID, err
	}

	agentEnv.AttachEphemeralDisk("1")

	envIsoPath, err := c.agentSettings.GenerateAgentEnvIso(agentEnv)
	if err != nil {
		return newVMCID, err
	}

	_, err = c.govcClient.UpdateVMIso(vmId, envIsoPath)
	if err != nil {
		return newVMCID, err
	}
	c.agentSettings.Cleanup()

	_, err = c.govcClient.StartVM(vmId)
	if err != nil {
		return newVMCID, err
	}

	return newVMCID, nil
}
