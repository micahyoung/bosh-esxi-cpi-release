package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	cpiconfig "bosh-vmrun-cpi/config"
	"bosh-vmrun-cpi/driver"
)

var _ = Describe("Driver", func() {
	var client driver.Client
	var esxNetworkName = os.Getenv("VCENTER_NETWORK_NAME")
	var vmId = "vm-virtualmachine"
	var stemcellId = "cs-stemcell"

	BeforeEach(func() {
		logger := boshlog.NewLogger(boshlog.LevelDebug)
		fs := boshsys.NewOsFileSystem(logger)
		cpiConfig, err := cpiconfig.NewConfigFromPath(CpiConfigPath, fs)
		Expect(err).ToNot(HaveOccurred())

		config := driver.NewConfig(cpiConfig)
		vmrunRunner := driver.NewVmrunRunner(config.VmrunPath(), logger)
		ovftoolRunner := driver.NewOvftoolRunner(config.OvftoolPath(), logger)
		vmxBuilder := driver.NewVmxBuilder(logger)
		client = driver.NewClient(vmrunRunner, ovftoolRunner, vmxBuilder, config, logger)
	})

	AfterEach(func() {
		//if client.HasVM(vmId) {
		//_, err := client.DestroyVM(vmId)
		//Expect(err).ToNot(HaveOccurred())
		//}
	})

	Describe("full lifecycle", func() {
		It("runs the commands", func() {
			var success bool
			var found bool
			var err error
			var vmInfo driver.VMInfo

			ovfPath := "../test/fixtures/test.ovf"
			success, err = client.ImportOvf(ovfPath, stemcellId)
			Expect(err).ToNot(HaveOccurred())
			Expect(success).To(Equal(true))

			found = client.HasVM(vmId)
			Expect(found).To(Equal(false))

			_, err = client.CloneVM(stemcellId, vmId)
			Expect(err).ToNot(HaveOccurred())

			found = client.HasVM(vmId)
			Expect(found).To(Equal(true))

			vmInfo, err = client.GetVMInfo(vmId)
			Expect(err).ToNot(HaveOccurred())
			Expect(vmInfo.Name).To(Equal(vmId))

			err = client.SetVMNetworkAdapter(vmId, esxNetworkName, "00:50:56:3F:00:00")
			Expect(err).ToNot(HaveOccurred())

			vmInfo, err = client.GetVMInfo(vmId)
			Expect(err).ToNot(HaveOccurred())
			Expect(vmInfo.NICs[0].Network).To(Equal(esxNetworkName))
			Expect(vmInfo.NICs[0].MAC).To(Equal("00:50:56:3F:00:00"))

			err = client.SetVMResources(vmId, 2, 1024)
			Expect(err).ToNot(HaveOccurred())

			vmInfo, err = client.GetVMInfo(vmId)
			Expect(err).ToNot(HaveOccurred())
			Expect(vmInfo.CPUs).To(Equal(2))
			Expect(vmInfo.RAM).To(Equal(1024))

			_, err = client.StartVM(vmId)
			Expect(err).ToNot(HaveOccurred())

			//err = client.CreateEphemeralDisk(vmId, 2048)
			//Expect(err).ToNot(HaveOccurred())

			//err = client.CreateDisk("disk-1", 3096)
			//Expect(err).ToNot(HaveOccurred())

			//err = client.AttachDisk(vmId, "disk-1")
			//Expect(err).ToNot(HaveOccurred())

			//envIsoPath := "../test/fixtures/env.iso"
			//result, err = client.UpdateVMIso(vmId, envIsoPath)
			//Expect(err).ToNot(HaveOccurred())

			//result, err = client.StartVM(vmId)
			//Expect(err).ToNot(HaveOccurred())
			//Expect(result).To(Equal("success"))

			//time.Sleep(1 * time.Second)

			//err = client.DetachDisk(vmId, "disk-1")
			//Expect(err).ToNot(HaveOccurred())

			//result, err = client.DestroyVM(vmId)
			//Expect(err).ToNot(HaveOccurred())
			//Expect(result).To(Equal(""))

			//time.Sleep(1 * time.Second)

			//found = client.HasVM(vmId)
			//Expect(found).To(Equal(false))

			//err = client.DestroyDisk("disk-1")
			//Expect(err).ToNot(HaveOccurred())

			//result, err = client.DestroyVM(stemcellId)
			//Expect(err).ToNot(HaveOccurred())
			//Expect(result).To(Equal(""))
		})
	})

	Describe("partial state", func() {
		It("destroys unstarted vms", func() {
			vmId := "vm-virtualmachine"
			var success bool
			var result string
			var err error

			result, err = client.DestroyVM(vmId)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(""))

			ovfPath := "../test/fixtures/test.ovf"
			success, err = client.ImportOvf(ovfPath, vmId)
			Expect(err).ToNot(HaveOccurred())
			Expect(success).To(Equal(true))

			envIsoPath := "../test/fixtures/env.iso"
			result, err = client.UpdateVMIso(vmId, envIsoPath)
			Expect(err).ToNot(HaveOccurred())

			result, err = client.DestroyVM(vmId)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(""))
		})
	})

	Describe("empty state", func() {
		It("does not fail with nonexistant vms", func() {
			vmId := "doesnt-exist"
			result, err := client.DestroyVM(vmId)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(""))

			found := client.HasVM(vmId)
			Expect(found).To(Equal(false))
		})
	})
})
