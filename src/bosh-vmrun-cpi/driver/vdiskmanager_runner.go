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
	logger.DebugWithDetails("vdiskmanager-runner", "bin: %+s", vmdiskmanagerBinPath)

	return VdiskmanagerRunnerImpl{vmdiskmanagerBinPath: vmdiskmanagerBinPath, logger: logger}
}

func (p VdiskmanagerRunnerImpl) CreateDisk(diskPath string, diskMB int) error {
	var err error

	_, err = p.run([]string{"-c", diskPath}, map[string]string{
		"s": fmt.Sprintf("%dMB", diskMB),
		"t": "0", //single growable virtual disk
	})
	if err != nil {
		return err
	}

	return nil
}

func (c VdiskmanagerRunnerImpl) run(args []string, flagMap map[string]string) (string, error) {
	commandArgs := []string{}
	for option, value := range flagMap {
		commandArgs = append(commandArgs, fmt.Sprintf("-%s %s", option, value))
	}
	commandArgs = append(commandArgs, args...)

	c.logger.DebugWithDetails("vdiskmanager-runner", "args:", commandArgs)

	command := exec.Command(c.vmdiskmanagerBinPath, commandArgs...)

	resultBytes, err := command.CombinedOutput()
	result := string(resultBytes)

	c.logger.DebugWithDetails("vdiskmanager-runner", "result:", result)

	return result, err
}
