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

func (c ClientImpl) StartVM(vmName string) (string, error) {
	var result string
	var err error

	result, err = c.startVM(vmName)
	if err != nil {
		c.logger.ErrorWithDetails("driver", "starting VM", err, result)
		return result, err
	}

	result, err = c.waitForVMStart(vmName)
	if err != nil {
		c.logger.ErrorWithDetails("driver", "waiting for VM to start", err, result)
		return result, err
	}

	return result, nil
}

func (c ClientImpl) waitForVMStart(vmName string) (string, error) {
	for {
		var vmState string
		var err error

		if vmState, err = c.vmState(vmName); err != nil {
			return vmState, err
		}

		if vmState == STATE_POWER_ON {
			break
		}

		time.Sleep(1 * time.Second)
	}

	return "", nil
}

func (c ClientImpl) startVM(vmName string) (string, error) {
	args := []string{"start", c.vmxPath(vmName), "nogui"}
	//args := []string{"start", c.vmxPath(vmName)}

	return c.vmrunRunner.CliCommand(args, nil)
}

func (c ClientImpl) HasVM(vmName string) bool {
	if _, err := os.Stat(c.vmxPath(vmName)); err != nil {
		return false
	}

	return true
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
	var result string
	var err error
	var diskDeviceName string

	//diskDeviceName, err := c.getVMDiskName(vmName, diskId)
	if err != nil {
		c.logger.ErrorWithDetails("govc", "getVMDiskName", err, diskDeviceName)
		return err
	}

	//result, err := c.detachDisk(vmName, diskDeviceName)
	if err != nil {
		c.logger.ErrorWithDetails("govc", "DetachDisk", err, result)
		return err
	}
	return nil
}

func (c ClientImpl) DestroyDisk(diskName string) error {
	var pathFound bool
	var result string
	var err error

	diskPath := fmt.Sprintf(`%s.vmdk`, diskName)
	_ = diskPath
	//pathFound, err := c.datastorePathExists(diskPath)
	if err != nil {
		c.logger.ErrorWithDetails("govc", "finding Path", err, pathFound)
		return err
	}

	if pathFound {
		//result, err := c.deleteDatastoreObject(diskPath)
		if err != nil {
			c.logger.ErrorWithDetails("govc", "delete VM files", err, result)
			return err
		}
	}

	return nil
}

func (c ClientImpl) DestroyVM(vmName string) (string, error) {
	var result string
	var err error
	var vmState string

	vmState, err = c.vmState(vmName)
	if err != nil {
		return result, err
	}

	if vmState == STATE_POWER_ON {
		result, err = c.stopVm(vmName)
		if err != nil {
			return result, err
		}
	}

	result, err = c.destroyVm(vmName)
	if err != nil {
		return result, err
	}

	return result, nil
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

func (c ClientImpl) replaceVm(currentVmName string, vmUpdateFunc func(string) error) error {
	var err error

	updateVmName := fmt.Sprintf("%s-updated", currentVmName)

	//TODO: enable when we confirm how VM lifecycle needs to be managed when chanages happen
	//result, err = c.stopVm(currentVmName)
	//if err != nil {
	//	return err
	//}

	_, err = c.cloneVm(currentVmName, updateVmName)
	if err != nil {
		return err
	}

	err = vmUpdateFunc(updateVmName)
	if err != nil {
		return err
	}

	_, err = c.destroyVm(currentVmName)
	if err != nil {
		return err
	}

	_, err = c.cloneVm(updateVmName, currentVmName)
	if err != nil {
		return err
	}

	_, err = c.destroyVm(updateVmName)
	if err != nil {
		return err
	}

	return nil
}

func (c ClientImpl) stopVm(vmName string) (string, error) {
	args := []string{"stop", c.vmxPath(vmName)}

	return c.vmrunRunner.CliCommand(args, nil)
}

func (c ClientImpl) destroyVm(vmName string) (string, error) {
	args := []string{"deleteVM", c.vmxPath(vmName)}

	return c.vmrunRunner.CliCommand(args, nil)
}

func (c ClientImpl) addNetwork(vmName string, networkName string, macAddress string) error {
	return c.vmxBuilder.AddNetworkInterface(networkName, macAddress, c.vmxPath(vmName))
}

func (c ClientImpl) setVMResources(vmName string, cpuCount int, ramMB int) error {
	return c.vmxBuilder.SetVMResources(cpuCount, ramMB, c.vmxPath(vmName))
}

//
//func (c ClientImpl) registerDatastoreVm(stemcellVmName string, cloneVmName string) (string, error) {
//	vmxPath := fmt.Sprintf("%s/%s.vmx", cloneVmName, stemcellVmName)
//	flags := map[string]string{
//		"name": cloneVmName,
//	}
//	args := []string{vmxPath}
//
//	return c.runner.CliCommand("vm.register", flags, args)
//}
//
//
//func (c ClientImpl) upload(cloneVmName string, localPath string, datastorePath string) (string, error) {
//	flags := map[string]string{}
//	args := []string{localPath, datastorePath}
//
//	return c.runner.CliCommand("datastore.upload", flags, args)
//}
//
//
//func (c ClientImpl) ejectCdrom(cloneVmName string) (string, error) {
//	flags := map[string]string{
//		"vm": cloneVmName,
//	}
//
//	return c.runner.CliCommand("device.cdrom.eject", flags, nil)
//}
//
//func (c ClientImpl) insertCdrom(cloneVmName string, datastorePath string) (string, error) {
//	flags := map[string]string{
//		"vm": cloneVmName,
//	}
//	args := []string{datastorePath}
//
//	return c.runner.CliCommand("device.cdrom.insert", flags, args)
//}
//
//func (c ClientImpl) connectCdrom(cloneVmName string) (string, error) {
//	flags := map[string]string{
//		"vm": cloneVmName,
//	}
//	args := []string{"cdrom-3000"}
//
//	return c.runner.CliCommand("device.connect", flags, args)
//}
//
//func (c ClientImpl) disconnectCdrom(cloneVmName string) (string, error) {
//	flags := map[string]string{
//		"vm": cloneVmName,
//	}
//	args := []string{"cdrom-3000"}
//
//	return c.runner.CliCommand("device.disconnect", flags, args)
//}
//
//func (c ClientImpl) powerOnVm(cloneVmName string) (string, error) {
//	flags := map[string]string{
//		"on": "true",
//	}
//	args := []string{cloneVmName}
//
//	return c.runner.CliCommand("vm.power", flags, args)
//}
//
//func (c ClientImpl) answerCopyQuestion(cloneVmName string) (string, error) {
//	flags := map[string]string{
//		"vm":     cloneVmName,
//		"answer": "2",
//	}
//
//	return c.runner.CliCommand("vm.question", flags, nil)
//}
//
//func (c ClientImpl) stopVM(cloneVmName string) (string, error) {
//	flags := map[string]string{
//		"off": "true",
//	}
//	args := []string{cloneVmName}
//
//	return c.runner.CliCommand("vm.power", flags, args)
//}
//
//func (c ClientImpl) destroyVm(vmName string) (string, error) {
//	flags := map[string]string{}
//	args := []string{vmName}
//
//	return c.runner.CliCommand("vm.destroy", flags, args)
//}
//
//func (c ClientImpl) deleteDatastoreObject(datastorePath string) (string, error) {
//	flags := map[string]string{
//		"f": "true",
//	}
//	args := []string{datastorePath}
//
//	return c.runner.CliCommand("datastore.rm", flags, args)
//}
//
func (c ClientImpl) vmState(vmName string) (string, error) {
	args := []string{"list"}

	result, err := c.vmrunRunner.CliCommand(args, nil)
	if err != nil {
		return result, err
	}

	if strings.Contains(result, c.vmxPath(vmName)) {
		return STATE_POWER_ON, nil
	}

	return STATE_POWER_OFF, nil
}

//
//func (c ClientImpl) datastorePathExists(datastorePath string) (bool, error) {
//	flags := map[string]string{}
//
//	result, err := c.runner.CliCommand("datastore.ls", flags, nil)
//	if err != nil {
//		return false, err
//	}
//
//	var response []struct{ File []struct{ Path string } }
//	err = json.Unmarshal([]byte(result), &response)
//	if err != nil {
//		return false, fmt.Errorf("error: %+v\nresult: %s\n", err, result)
//	}
//
//	files := response[0].File
//	found := false
//	for i := range files {
//		file := files[i].Path
//		if file == datastorePath {
//			found = true
//			break
//		}
//	}
//
//	return found, nil
//}
//
//func (c ClientImpl) createDisk(diskId string, diskMB int) (string, error) {
//	diskPath := fmt.Sprintf(`%s.vmdk`, diskId)
//	diskSize := fmt.Sprintf(`%dMB`, diskMB)
//	flags := map[string]string{
//		"size": diskSize,
//	}
//	args := []string{diskPath}
//
//	result, err := c.runner.CliCommand("datastore.disk.create", flags, args)
//	if err != nil {
//		return result, err
//	}
//
//	return result, err
//}
//

//
//func (c ClientImpl) attachDisk(vmName string, diskId string) (string, error) {
//	diskPath := fmt.Sprintf(`%s.vmdk`, diskId)
//	flags := map[string]string{
//		"vm":   vmName,
//		"disk": diskPath,
//		"link": "true",
//	}
//
//	result, err := c.runner.CliCommand("vm.disk.attach", flags, nil)
//	if err != nil {
//		return result, err
//	}
//
//	return result, nil
//}
//
//func (c ClientImpl) getVMDiskName(vmName string, diskId string) (string, error) {
//	flags := map[string]string{
//		"json": "true",
//		"vm":   vmName,
//	}
//
//	result, err := c.runner.CliCommand("device.info", flags, nil)
//	if err != nil {
//		return result, err
//	}
//
//	var response struct {
//		Devices []struct {
//			Name    string
//			Backing struct {
//				Parent struct {
//					FileName string
//				}
//			}
//		}
//	}
//	err = json.Unmarshal([]byte(result), &response)
//	if err != nil {
//		return result, err
//	}
//
//	foundDevice := ""
//	for _, device := range response.Devices {
//		if strings.Contains(device.Backing.Parent.FileName, diskId) {
//			foundDevice = device.Name
//		}
//	}
//
//	return foundDevice, nil
//}
//
//func (c ClientImpl) detachDisk(vmName string, diskName string) (string, error) {
//	flags := map[string]string{
//		"vm":   vmName,
//		"keep": "true",
//	}
//	args := []string{diskName}
//
//	result, err := c.runner.CliCommand("device.remove", flags, args)
//	if err != nil {
//		return result, err
//	}
//
//	return result, nil
//}
