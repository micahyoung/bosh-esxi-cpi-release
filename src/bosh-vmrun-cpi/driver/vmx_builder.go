package driver

import (
	"io/ioutil"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/hooklift/govmx"
)

type VmxBuilderImpl struct {
	logger boshlog.Logger
}

func NewVmxBuilder(logger boshlog.Logger) VmxBuilder {
	return VmxBuilderImpl{logger: logger}
}

func (p VmxBuilderImpl) AddNetworkInterface(networkName, macAddress, vmxPath string) error {
	err := p.replaceVmx(vmxPath, func(vmxVM *vmx.VirtualMachine) *vmx.VirtualMachine {
		vmxVM.Ethernet = append(vmxVM.Ethernet, vmx.Ethernet{
			VNetwork:    networkName,
			Address:     macAddress,
			AddressType: "static",
			VirtualDev:  "vmxnet3",
			Present:     true,
		})

		return vmxVM
	})

	return err
}

func (p VmxBuilderImpl) SetVMResources(cpu int, mem int, vmxPath string) error {
	err := p.replaceVmx(vmxPath, func(vmxVM *vmx.VirtualMachine) *vmx.VirtualMachine {
		vmxVM.NumvCPUs = uint(cpu)
		vmxVM.Memsize = uint(mem)

		return vmxVM
	})

	return err
}

func (p VmxBuilderImpl) InitHardware(vmxPath string) error {
	err := p.replaceVmx(vmxPath, func(vmxVM *vmx.VirtualMachine) *vmx.VirtualMachine {
		vmxVM.VHVEnable = true
		vmxVM.Tools.SyncTime = true

		return vmxVM
	})

	return err
}

func (p VmxBuilderImpl) VMInfo(vmxPath string) (VMInfo, error) {
	vmxVM, err := p.getVmx(vmxPath)
	if err != nil {
		return VMInfo{}, err
	}

	//p.logger.DebugWithDetails("vmx-builder", "DEBUG: %+v", vmxVM)

	vmInfo := VMInfo{
		Name: vmxVM.DisplayName,
		CPUs: int(vmxVM.NumvCPUs),
		RAM:  int(vmxVM.Memsize),
	}

	for _, vmxNic := range vmxVM.Ethernet {
		vmInfo.NICs = append(vmInfo.NICs, struct {
			Network string
			MAC     string
		}{
			Network: vmxNic.VNetwork,
			MAC:     vmxNic.Address,
		})
	}

	return vmInfo, nil
}

func (p VmxBuilderImpl) GetVmx(vmxPath string) (*vmx.VirtualMachine, error) {
	return p.getVmx(vmxPath)
}

func (p VmxBuilderImpl) replaceVmx(vmxPath string, vmUpdateFunc func(*vmx.VirtualMachine) *vmx.VirtualMachine) error {
	vmxVM, err := p.getVmx(vmxPath)
	if err != nil {
		return err
	}

	vmxVM = vmUpdateFunc(vmxVM)

	err = p.writeVmx(vmxVM, vmxPath)
	if err != nil {
		return err
	}

	return nil
}

func (p VmxBuilderImpl) getVmx(vmxPath string) (*vmx.VirtualMachine, error) {
	var err error

	vmxBytes, err := ioutil.ReadFile(vmxPath)
	if err != nil {
		p.logger.ErrorWithDetails("vmx-builder", "reading file: %s", vmxPath)
		return nil, err
	}

	vmxVM := new(vmx.VirtualMachine)
	err = vmx.Unmarshal(vmxBytes, vmxVM)
	if err != nil {
		p.logger.ErrorWithDetails("vmx-builder", "unmarshaling file: %s", vmxPath)
		return nil, err
	}

	return vmxVM, nil
}

func (p VmxBuilderImpl) writeVmx(vmxVM *vmx.VirtualMachine, vmxPath string) error {
	var err error

	vmxBytes, err := vmx.Marshal(vmxVM)
	if err != nil {
		p.logger.ErrorWithDetails("vmx-builder", "marshaling content: %+v", vmxVM)
		return err
	}

	err = ioutil.WriteFile(vmxPath, vmxBytes, 0644)
	if err != nil {
		p.logger.ErrorWithDetails("vmx-builder", "writing file: %s", vmxPath)
		return err
	}

	return nil
}
