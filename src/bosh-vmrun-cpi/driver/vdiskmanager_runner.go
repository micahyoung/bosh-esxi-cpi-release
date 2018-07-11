package driver

import (
	"fmt"
	"os/exec"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type VdiskmanagerRunnerImpl struct {
	vmdiskmanagerBinPath string
	logger               boshlog.Logger
}

func NewVdiskmanagerRunner(vmdiskmanagerBinPath string, logger boshlog.Logger) VdiskmanagerRunner {
	return VdiskmanagerRunnerImpl{vmdiskmanagerBinPath: vmdiskmanagerBinPath, logger: logger}
}

func (p VdiskmanagerRunnerImpl) CreateDisk(diskPath string, diskMB int) error {
	var err error

	_, err = p.run([]string{"-c", diskPath}, map[string]string{
		"s": fmt.Sprintf("%dMB", diskMB),
		"t": "0", //single growable virtual disk
	})

	return err
}

func (c VdiskmanagerRunnerImpl) run(args []string, flagMap map[string]string) (string, error) {
	commandArgs := []string{}
	for option, value := range flagMap {
		commandArgs = append(commandArgs, fmt.Sprintf("-%s %s", option, value))
	}
	commandArgs = append(commandArgs, args...)

	c.logger.Debug("vdiskmanager-runner", fmt.Sprintf("%+v", commandArgs))

	command := exec.Command(c.vmdiskmanagerBinPath, commandArgs...)

	resultBytes, err := command.CombinedOutput()

	return string(resultBytes), err
}
