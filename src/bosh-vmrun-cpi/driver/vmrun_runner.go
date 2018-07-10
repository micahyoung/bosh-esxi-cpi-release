package driver

import (
	"fmt"
	"os/exec"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type VmrunRunnerImpl struct {
	vmrunBinPath string
	logger       boshlog.Logger
}

func NewVmrunRunner(vmrunBinPath string, logger boshlog.Logger) VmrunRunner {
	//TODO: check that vmrun is on the path
	return &VmrunRunnerImpl{vmrunBinPath: vmrunBinPath, logger: logger}
}

func (c VmrunRunnerImpl) CliCommand(args []string, flagMap map[string]string) (string, error) {
	commandArgs := []string{}
	commandArgs = append(commandArgs, args...)

	for option, value := range flagMap {
		commandArgs = append(commandArgs, fmt.Sprintf("-%s=%s", option, value))
	}

	c.logger.Debug("vmrun-runner", fmt.Sprintf("%+v", commandArgs))

	command := exec.Command(c.vmrunBinPath, commandArgs...)

	resultBytes, err := command.CombinedOutput()

	return string(resultBytes), err
}
