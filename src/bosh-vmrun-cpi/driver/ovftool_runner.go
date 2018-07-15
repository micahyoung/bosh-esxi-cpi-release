package driver

import (
	"fmt"
	"os/exec"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type OvftoolRunnerImpl struct {
	ovftoolBinPath string
	logger         boshlog.Logger
}

func NewOvftoolRunner(ovftoolBinPath string, logger boshlog.Logger) OvftoolRunner {
	logger.DebugWithDetails("ovftool-runner", "bin: %+s", ovftoolBinPath)

	return &OvftoolRunnerImpl{ovftoolBinPath: ovftoolBinPath, logger: logger}
}

func (c OvftoolRunnerImpl) CliCommand(args []string, flagMap map[string]string) (string, error) {
	commandArgs := []string{}
	for option, value := range flagMap {
		commandArgs = append(commandArgs, fmt.Sprintf("--%s=%s", option, value))
	}
	commandArgs = append(commandArgs, args...)

	c.logger.DebugWithDetails("ovftool-runner", "args: %+v", commandArgs)

	command := exec.Command(c.ovftoolBinPath, commandArgs...)

	resultBytes, err := command.CombinedOutput()

	return string(resultBytes), err
}
