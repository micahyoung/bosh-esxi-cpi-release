package driver_test

import (
	"io/ioutil"
	"os"

	"github.com/hooklift/govmx"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakelogger "github.com/cloudfoundry/bosh-utils/logger/loggerfakes"

	"bosh-vmrun-cpi/driver"
)

var _ = Describe("VmxBuilder", func() {
	var logger *fakelogger.FakeLogger
	var builder driver.VmxBuilder
	var vmxPath string

	BeforeEach(func() {
		var err error

		vmxBytes, err := ioutil.ReadFile("../test/fixtures/test.vmx")
		Expect(err).ToNot(HaveOccurred())

		vmxFile, err := ioutil.TempFile("", "")
		Expect(err).ToNot(HaveOccurred())

		err = ioutil.WriteFile(vmxFile.Name(), vmxBytes, 0644)
		Expect(err).ToNot(HaveOccurred())

		vmxPath = vmxFile.Name()
		logger = &fakelogger.FakeLogger{}
		builder = driver.NewVmxBuilder(logger)
	})

	AfterEach(func() {
		os.Remove(vmxPath)
	})

	Describe("VMInfo", func() {
		It("reads the fixture", func() {
			vmInfo, err := builder.VMInfo(vmxPath)

			Expect(err).ToNot(HaveOccurred())
			Expect(vmInfo.Name).To(Equal("vm-virtualmachine"))
			Expect(vmInfo.NICs).To(BeEmpty())
		})
	})

	Describe("InitHardware", func() {
		It("configs basic hardware settings", func() {
			err := builder.InitHardware(vmxPath)
			Expect(err).ToNot(HaveOccurred())

			vmxVm, err := builder.GetVmx(vmxPath)
			Expect(err).ToNot(HaveOccurred())

			Expect(vmxVm.VHVEnable).To(BeTrue())
			Expect(vmxVm.Tools.SyncTime).To(BeTrue())
		})
	})

	Describe("AddNetworkInterface", func() {
		It("add a NIC", func() {
			err := builder.AddNetworkInterface("fooNetwork", "00:11:22:33:44:55", vmxPath)
			Expect(err).ToNot(HaveOccurred())

			vmxVm, err := builder.GetVmx(vmxPath)
			Expect(err).ToNot(HaveOccurred())

			Expect(vmxVm.Ethernet[0].VNetwork).To(Equal("fooNetwork"))
			Expect(vmxVm.Ethernet[0].Address).To(Equal("00:11:22:33:44:55"))
			Expect(vmxVm.Ethernet[0].AddressType).To(Equal(vmx.EthernetAddressType("static")))
			Expect(vmxVm.Ethernet[0].VirtualDev).To(Equal("vmxnet3"))
			Expect(vmxVm.Ethernet[0].Present).To(BeTrue())
		})
	})

	Describe("SetVMResources", func() {
		It("sets cpu and mem", func() {
			err := builder.SetVMResources(2, 4096, vmxPath)
			Expect(err).ToNot(HaveOccurred())

			vmxVm, err := builder.GetVmx(vmxPath)
			Expect(err).ToNot(HaveOccurred())

			Expect(vmxVm.NumvCPUs).To(Equal(uint(2)))
			Expect(vmxVm.Memsize).To(Equal(uint(4096)))
		})
	})
})
