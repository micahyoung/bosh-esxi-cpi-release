package driver

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type ClientImpl struct {
	config             Config
	vmrunRunner        VmrunRunner
	ovftoolRunner      OvftoolRunner
	vmxBuilder         VmxBuilder
	vdiskmanagerRunner VdiskmanagerRunner
	logger             boshlog.Logger
}

var (
	STATE_NOT_FOUND         = "state-not-found"
	STATE_POWER_ON          = "state-on"
	STATE_POWER_OFF         = "state-off"
	STATE_BLOCKING_QUESTION = "state-blocking-question"
)

func NewClient(vmrunRunner VmrunRunner, ovftoolRunner OvftoolRunner, vdiskmanagerRunner VdiskmanagerRunner, vmxBuilder VmxBuilder, config Config, logger boshlog.Logger) Client {
	return ClientImpl{vmrunRunner: vmrunRunner, ovftoolRunner: ovftoolRunner, vdiskmanagerRunner: vdiskmanagerRunner, vmxBuilder: vmxBuilder, config: config, logger: logger}
}

func (c ClientImpl) vmxPath(vmName string) string {
	return filepath.Join(c.config.VmPath(), fmt.Sprintf("%s.vmwarevm", vmName), fmt.Sprintf("%s.vmx", vmName))
}

func (c ClientImpl) ephemeralDiskPath(vmName string) string {
	baseDir := filepath.Join(c.config.VmPath(), "ephemeral-disks")
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		os.MkdirAll(baseDir, 0755)
	}

	return filepath.Join(baseDir, fmt.Sprintf("%s.vmdk", vmName))
}

func (c ClientImpl) persistentDiskPath(diskId string) string {
	baseDir := filepath.Join(c.config.VmPath(), "persistent-disks")
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		os.MkdirAll(baseDir, 0755)
	}

	return filepath.Join(baseDir, fmt.Sprintf("%s.vmdk", diskId))
}

func (c ClientImpl) envIsoPath(vmName string) string {
	baseDir := filepath.Join(c.config.VmPath(), "env-isos")
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		os.MkdirAll(baseDir, 0755)
	}

	return filepath.Join(baseDir, fmt.Sprintf("%s.iso", vmName))
}

func (c ClientImpl) ImportOvf(ovfPath string, vmName string) (bool, error) {
	flags := map[string]string{
		"sourceType": "OVF",
		"targetType": "VMX",
		"name":       vmName,
	}
	args := []string{ovfPath, c.config.VmPath()}

	result, err := c.ovftoolRunner.CliCommand(args, flags)
	if err != nil {
		c.logger.ErrorWithDetails("client", "import ovf", err, result)
		return false, err
	}

	return true, nil
}

func (c ClientImpl) CloneVM(sourceVmName string, cloneVmName string) (string, error) {
	var result string
	var err error

	result, err = c.cloneVm(sourceVmName, cloneVmName)
	if err != nil {
		c.logger.ErrorWithDetails("client", "clone stemcell", err, result)
		return result, err
	}

	err = c.initHardware(cloneVmName)
	if err != nil {
		c.logger.ErrorWithDetails("client", "configuring vm hardware", err)
		return result, err
	}

	return result, nil
}

func (c ClientImpl) SetVMNetworkAdapter(vmName string, networkName string, macAddress string) error {
	var err error

	err = c.addNetwork(vmName, networkName, macAddress)
	if err != nil {
		c.logger.ErrorWithDetails("driver", "adding network", err, vmName, networkName, macAddress)
		return err
	}

	return nil
}

func (c ClientImpl) SetVMResources(vmName string, cpus int, ram int) error {
	err := c.setVMResources(vmName, cpus, ram)
	if err != nil {
		c.logger.ErrorWithDetails("driver", "setting vm cpu and ram", err)
		return err
	}

	return nil
}

func (c ClientImpl) UpdateVMIso(vmName string, localIsoPath string) error {
	var err error

	isoBytes, err := ioutil.ReadFile(localIsoPath)
	if err != nil {
		c.logger.ErrorWithDetails("driver", "reading generated iso", err)
		return err
	}

	err = ioutil.WriteFile(c.envIsoPath(vmName), isoBytes, 0644)
	if err != nil {
		c.logger.ErrorWithDetails("driver", "writing vm iso contents", err)
		return err
	}

	err = c.vmxBuilder.AttachCdrom(c.envIsoPath(vmName), c.vmxPath(vmName))
	if err != nil {
		c.logger.ErrorWithDetails("govc", "connecting ENV cdrom", err)
		return err
	}

	return nil
}

func (c ClientImpl) StartVM(vmName string) error {
	var err error

	err = c.startVM(vmName)
	if err != nil {
		c.logger.ErrorWithDetails("driver", "starting VM", err)
		return err
	}

	//TODO: switchto vmrun waitForIP
	err = c.waitForVMStart(vmName)
	if err != nil {
		c.logger.ErrorWithDetails("driver", "waiting for VM to start", err)
		return err
	}

	time.Sleep(10 * time.Second)

	return nil
}

func (c ClientImpl) waitForVMStart(vmName string) error {
	for {
		var vmState string
		var err error

		if vmState, err = c.vmState(vmName); err != nil {
			return err
		}

		if vmState == STATE_POWER_ON {
			break
		}

		time.Sleep(1 * time.Second)
	}

	return nil
}

func (c ClientImpl) startVM(vmName string) error {
	args := []string{"start", c.vmxPath(vmName), "nogui"}
	//args := []string{"start", c.vmxPath(vmName)}

	_, err := c.vmrunRunner.CliCommand(args, nil)
	return err
}

func (c ClientImpl) HasVM(vmName string) bool {
	return c.vmExists(vmName)
}

func (c ClientImpl) vmExists(vmName string) bool {
	if _, err := os.Stat(c.vmxPath(vmName)); err != nil {
		return false
	} else {
		return true
	}
}

func (c ClientImpl) CreateEphemeralDisk(vmName string, diskMB int) error {
	var err error

	err = c.vdiskmanagerRunner.CreateDisk(c.ephemeralDiskPath(vmName), diskMB)
	if err != nil {
		c.logger.ErrorWithDetails("driver", "CreateEphemeralDisk create", err)
		return err
	}

	err = c.vmxBuilder.AttachDisk(c.ephemeralDiskPath(vmName), c.vmxPath(vmName))
	if err != nil {
		c.logger.ErrorWithDetails("driver", "CreateEphemeralDisk attach", err)
		return err
	}

	return nil
}

func (c ClientImpl) CreateDisk(diskId string, diskMB int) error {
	var err error

	err = c.vdiskmanagerRunner.CreateDisk(c.persistentDiskPath(diskId), diskMB)
	if err != nil {
		c.logger.ErrorWithDetails("driver", "CreateDisk", err)
		return err
	}
	return nil
}

func (c ClientImpl) AttachDisk(vmName string, diskId string) error {
	var err error

	err = c.vmxBuilder.AttachDisk(c.persistentDiskPath(diskId), c.vmxPath(vmName))
	if err != nil {
		c.logger.ErrorWithDetails("govc", "AttachDisk", err)
		return err
	}
	return nil
}

func (c ClientImpl) DetachDisk(vmName string, diskId string) error {
	var err error

	err = c.vmxBuilder.DetachDisk(c.persistentDiskPath(vmName), c.vmxPath(vmName))
	if err != nil {
		c.logger.ErrorWithDetails("govc", "DetachDisk", err)
		return err
	}
	return nil
}

func (c ClientImpl) DestroyDisk(diskId string) error {
	var err error

	err = os.Remove(c.persistentDiskPath(diskId))
	if err != nil {
		c.logger.ErrorWithDetails("driver", "DestroyDisk", err)
		return err
	}

	return nil
}

func (c ClientImpl) StopVM(vmName string) error {
	var err error
	var vmState string

	vmState, err = c.vmState(vmName)
	if err != nil {
		return err
	}

	if vmState == STATE_POWER_ON {
		err = c.stopVM(vmName)
		if err != nil {
			return err
		}
	}

	return nil
}

//TODO: add more graceful handling of locked vmx (when stopped but GUI has them open)
func (c ClientImpl) DestroyVM(vmName string) error {
	var err error
	var vmState string

	vmState, err = c.vmState(vmName)
	if err != nil {
		return err
	}

	if vmState == STATE_POWER_ON {
		err = c.stopVM(vmName)
		if err != nil {
			return err
		}
	}

	vmState, err = c.vmState(vmName)
	if err != nil {
		return err
	}

	if vmState == STATE_POWER_OFF {
		err = c.destroyVm(vmName)
		if err != nil {
			return err
		}
	}

	//attempt to cleanup ephemeral disk, ignore error
	_ = os.Remove(c.ephemeralDiskPath(vmName))

	return nil
}

func (c ClientImpl) GetVMInfo(vmName string) (VMInfo, error) {
	vmInfo, err := c.vmxBuilder.VMInfo(c.vmxPath(vmName))
	if err != nil {
		return vmInfo, err
	}
	return vmInfo, err
}

func (c ClientImpl) cloneVm(sourceVmName string, targetVmName string) (string, error) {
	flags := map[string]string{
		"name":                targetVmName,
		"sourceType":          "VMX",
		"allowAllExtraConfig": "true",
		"exportFlags":         "extraconfig,mac,uuid",
		"targetType":          "VMX",
	}
	args := []string{c.vmxPath(sourceVmName), c.config.VmPath()}

	return c.ovftoolRunner.CliCommand(args, flags)
}

func (c ClientImpl) initHardware(vmName string) error {
	return c.vmxBuilder.InitHardware(c.vmxPath(vmName))
}

func (c ClientImpl) stopVM(vmName string) error {
	args := []string{"stop", c.vmxPath(vmName)}

	_, err := c.vmrunRunner.CliCommand(args, nil)
	return err
}

func (c ClientImpl) destroyVm(vmName string) error {
	args := []string{"deleteVM", c.vmxPath(vmName)}

	_, err := c.vmrunRunner.CliCommand(args, nil)
	return err
}

func (c ClientImpl) addNetwork(vmName string, networkName string, macAddress string) error {
	return c.vmxBuilder.AddNetworkInterface(networkName, macAddress, c.vmxPath(vmName))
}

func (c ClientImpl) setVMResources(vmName string, cpuCount int, ramMB int) error {
	return c.vmxBuilder.SetVMResources(cpuCount, ramMB, c.vmxPath(vmName))
}

func (c ClientImpl) vmState(vmName string) (string, error) {
	args := []string{"list"}

	result, err := c.vmrunRunner.CliCommand(args, nil)
	if err != nil {
		return result, err
	}

	if !c.vmExists(vmName) {
		return STATE_NOT_FOUND, nil
	}

	if strings.Contains(result, vmName) {
		return STATE_POWER_ON, nil
	}

	return STATE_POWER_OFF, nil
}
