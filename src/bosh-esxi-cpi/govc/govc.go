package govc

//go:generate counterfeiter -o fakes/fake_govc_client.go $GOPATH/src/bosh-esxi-cpi/govc/govc.go GovcClient
type GovcClient interface {
	ImportOvf(string, string) (string, error)
	CloneVM(string, string) (string, error)
	UpdateVMIso(string, string) (string, error)
	StartVM(string) (string, error)
	HasVM(string) (bool, error)
	SetVMNetworkAdapter(string, string, string) error
	SetVMResources(string, int, int) error
	CreateEphemeralDisk(string, int) error
	CreateDisk(string, int) error
	AttachDisk(string, string) error
	DetachDisk(string, string) error
	DestroyDisk(string) error
	DestroyVM(string) (string, error)
}

//go:generate counterfeiter -o fakes/fake_govc_runner.go $GOPATH/src/bosh-esxi-cpi/govc/govc.go GovcRunner
type GovcRunner interface {
	CliCommand(string, map[string]string, []string) (string, error)
}

//go:generate counterfeiter -o fakes/fake_govc_config.go $GOPATH/src/bosh-esxi-cpi/govc/govc.go GovcConfig
type GovcConfig interface {
	EsxUrl() string
}
