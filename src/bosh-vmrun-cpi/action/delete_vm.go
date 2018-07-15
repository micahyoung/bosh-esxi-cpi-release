package action

import (
	"fmt"

	"github.com/cppforlife/bosh-cpi-go/apiv1"

	"bosh-vmrun-cpi/driver"
)

type DeleteVMMethod struct {
	driverClient driver.Client
}

func NewDeleteVMMethod(driverClient driver.Client) DeleteVMMethod {
	return DeleteVMMethod{
		driverClient: driverClient,
	}
}

func (c DeleteVMMethod) DeleteVM(vmCid apiv1.VMCID) error {
	vmId := "vm-" + vmCid.AsString()
	err := c.driverClient.DestroyVM(vmId)
	if err != nil {
		fmt.Printf("%+v\n", err)
		return err
	}

	return nil
}
