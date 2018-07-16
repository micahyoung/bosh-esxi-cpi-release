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
	logger.DebugWithDetails("vmrun-runner", "bin: %+s", vmrunBinPath)

	return &VmrunRunnerImpl{vmrunBinPath: vmrunBinPath, logger: logger}
}

func (c VmrunRunnerImpl) CliCommand(args []string, flagMap map[string]string) (string, error) {
	commandArgs := []string{}
	commandArgs = append(commandArgs, args...)

	for option, value := range flagMap {
		commandArgs = append(commandArgs, fmt.Sprintf("-%s=%s", option, value))
	}

	c.logger.DebugWithDetails("vmrun-runner", "args: ", commandArgs)

	command := exec.Command(c.vmrunBinPath, commandArgs...)

	resultBytes, err := command.CombinedOutput()
	result := string(resultBytes)

	c.logger.DebugWithDetails("vmrun-runner", "results: ", result)

	return result, err
}
