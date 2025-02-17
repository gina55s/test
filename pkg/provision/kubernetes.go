package provision

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/pkg/errors"
	"github.com/threefoldtech/test/pkg"
	"github.com/threefoldtech/test/pkg/stubs"
)

// KubernetesResult result returned by k3s reservation
type KubernetesResult struct {
	ID string `json:"id"`
	IP string `json:"ip"`
}

// Kubernetes reservation data
type Kubernetes struct {
	// Size of the vm, this defines the amount of vCpu, memory, and the disk size
	// Docs: docs/kubernetes/sizes.md
	Size uint8 `json:"size"`

	// NetworkID of the network namepsace in which to run the VM. The network
	// must be provisioned previously.
	NetworkID pkg.NetID `json:"network_id"`
	// IP of the VM. The IP must be part of the subnet available in the network
	// resource defined by the networkID on this node
	IP net.IP `json:"ip"`

	// ClusterSecret is the hex encoded encrypted cluster secret.
	ClusterSecret string `json:"cluster_secret"`
	// MasterIPs define the URL's for the kubernetes master nodes. If this
	// list is empty, this node is considered to be a master node.
	MasterIPs []net.IP `json:"master_ips"`
	// SSHKeys is a list of ssh keys to add to the VM. Keys can be either
	// a full ssh key, or in the form of `github:${username}`. In case of
	// the later, the VM will retrieve the github keys for this username
	// when it boots.
	SSHKeys []string `json:"ssh_keys"`

	PlainClusterSecret string `json:"-"`
}

const k3osFlistURL = "https://hub.grid.tf/tf-official-apps/k3os.flist"

func kubernetesProvision(ctx context.Context, reservation *Reservation) (interface{}, error) {
	return kubernetesProvisionImpl(ctx, reservation)
}

func kubernetesProvisionImpl(ctx context.Context, reservation *Reservation) (result KubernetesResult, err error) {
	var (
		client = GetZBus(ctx)

		storage = stubs.NewVDiskModuleStub(client)
		network = stubs.NewNetworkerStub(client)
		flist   = stubs.NewFlisterStub(client)
		vm      = stubs.NewVMModuleStub(client)

		config Kubernetes

		needsInstall = true
	)

	if err := json.Unmarshal(reservation.Data, &config); err != nil {
		return result, errors.Wrap(err, "failed to decode reservation schema")
	}

	result.ID = reservation.ID
	result.IP = config.IP.String()

	config.PlainClusterSecret, err = decryptSecret(client, config.ClusterSecret)
	if err != nil {
		return result, errors.Wrap(err, "failed to decrypt namespace password")
	}

	cpu, memory, disk, err := vmSize(config.Size)
	if err != nil {
		return result, errors.Wrap(err, "could not interpret vm size")
	}

	if _, err = vm.Inspect(reservation.ID); err == nil {
		// vm is already running, nothing to do here
		return result, nil
	}

	var imagePath string
	imagePath, err = flist.NamedMount(reservation.ID, k3osFlistURL, "", pkg.ReadOnlyMountOptions)
	if err != nil {
		return result, errors.Wrap(err, "could not mount k3os flist")
	}
	// In case of future errrors in the provisioning make sure we clean up
	defer func() {
		if err != nil {
			_ = flist.Umount(imagePath)
		}
	}()

	var diskPath string
	diskName := fmt.Sprintf("%s-%s", reservation.ID, "vda")
	if storage.Exists(diskName) {
		needsInstall = false
		info, err := storage.Inspect(diskName)
		if err != nil {
			return result, errors.Wrap(err, "could not get path to existing disk")
		}
		diskName = info.Path
	} else {
		diskPath, err = storage.Allocate(diskName, int64(disk))
		if err != nil {
			return result, errors.Wrap(err, "failed to reserve filesystem for vm")
		}
	}
	// clean up the disk anyway, even if it has already been installed.
	defer func() {
		if err != nil {
			_ = storage.Deallocate(diskName)
		}
	}()

	var iface string
	netID := networkID(reservation.User, string(config.NetworkID))
	iface, err = network.SetupTap(netID)
	if err != nil {
		return result, errors.Wrap(err, "could not set up tap device")
	}

	defer func() {
		if err != nil {
			_ = vm.Delete(reservation.ID)
			_ = network.RemoveTap(netID)
		}
	}()

	var netInfo pkg.VMNetworkInfo
	netInfo, err = buildNetworkInfo(ctx, reservation.User, iface, config)
	if err != nil {
		return result, errors.Wrap(err, "could not generate network info")
	}

	if needsInstall {
		if err = kubernetesInstall(ctx, reservation.ID, cpu, memory, diskPath, imagePath, netInfo, config); err != nil {
			return result, errors.Wrap(err, "failed to install k3s")
		}
	}

	err = kubernetesRun(ctx, reservation.ID, cpu, memory, diskPath, imagePath, netInfo, config)
	return result, err
}

func kubernetesInstall(ctx context.Context, name string, cpu uint8, memory uint64, diskPath string, imagePath string, networkInfo pkg.VMNetworkInfo, cfg Kubernetes) error {
	vm := stubs.NewVMModuleStub(GetZBus(ctx))

	cmdline := fmt.Sprintf("console=ttyS0 reboot=k panic=1 k3os.mode=install k3os.install.silent k3os.install.device=/dev/vda k3os.token=%s", cfg.PlainClusterSecret)
	// if there is no server url configured, the node is set up as a master, therefore
	// this will cause nodes with an empty master list to be implicitly treated as
	// a master node
	for _, ip := range cfg.MasterIPs {
		var ipstring string
		if ip.To4() != nil {
			ipstring = ip.String()
		} else if ip.To16() != nil {
			ipstring = fmt.Sprintf("[%s]", ip.String())
		} else {
			return errors.New("invalid master IP")
		}
		cmdline = fmt.Sprintf("%s k3os.server_url=https://%s:6443", cmdline, ipstring)
	}
	for _, key := range cfg.SSHKeys {
		cmdline = fmt.Sprintf("%s ssh_authorized_keys=\"%s\"", cmdline, key)
	}

	disks := make([]pkg.VMDisk, 2)
	// install disk
	disks[0] = pkg.VMDisk{Path: diskPath, ReadOnly: false, Root: false}
	// install ISO
	disks[1] = pkg.VMDisk{Path: imagePath + "/k3os-amd64.iso", ReadOnly: true, Root: false}

	installVM := pkg.VM{
		Name:        name,
		CPU:         cpu,
		Memory:      int64(memory),
		Network:     networkInfo,
		KernelImage: imagePath + "/k3os-vmlinux",
		InitrdImage: imagePath + "/k3os-initrd-amd64",
		KernelArgs:  cmdline,
		Disks:       disks,
	}

	if err := vm.Run(installVM); err != nil {
		return errors.Wrap(err, "could not run vm")
	}

	deadline, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()
	for {
		if !vm.Exists(name) {
			// install is done
			break
		}
		select {
		case <-time.After(time.Second * 3):
			// retry after 3 secs
		case <-deadline.Done():
			return errors.New("failed to install vm in 5 minutes")
		}
	}

	// Delete the VM, the disk will be installed now
	return vm.Delete(name)
}

func kubernetesRun(ctx context.Context, name string, cpu uint8, memory uint64, diskPath string, imagePath string, networkInfo pkg.VMNetworkInfo, cfg Kubernetes) error {
	vm := stubs.NewVMModuleStub(GetZBus(ctx))

	disks := make([]pkg.VMDisk, 1)
	// installed disk
	disks[0] = pkg.VMDisk{Path: diskPath, ReadOnly: false, Root: false}

	kubevm := pkg.VM{
		Name:        name,
		CPU:         cpu,
		Memory:      int64(memory),
		Network:     networkInfo,
		KernelImage: imagePath + "/k3os-vmlinux",
		InitrdImage: imagePath + "/k3os-initrd-amd64",
		KernelArgs:  "console=ttyS0 reboot=k panic=1",
		Disks:       disks,
	}

	return vm.Run(kubevm)
}

func kubernetesDecomission(ctx context.Context, reservation *Reservation) error {
	var (
		client = GetZBus(ctx)

		storage = stubs.NewVDiskModuleStub(client)
		network = stubs.NewNetworkerStub(client)
		flist   = stubs.NewFlisterStub(client)
		vm      = stubs.NewVMModuleStub(client)

		cfg Kubernetes
	)

	if err := json.Unmarshal(reservation.Data, &cfg); err != nil {
		return errors.Wrap(err, "failed to decode reservation schema")
	}

	if _, err := vm.Inspect(reservation.ID); err == nil {
		if err := vm.Delete(reservation.ID); err != nil {
			return errors.Wrapf(err, "failed to delete vm %s", reservation.ID)
		}
	}

	netID := networkID(reservation.User, string(cfg.NetworkID))
	if err := network.RemoveTap(netID); err != nil {
		return errors.Wrap(err, "could not clean up tap device")
	}

	if err := storage.Deallocate(fmt.Sprintf("%s-%s", reservation.ID, "vda")); err != nil {
		return errors.Wrap(err, "could not remove vDisk")
	}

	if err := flist.NamedUmount(reservation.ID); err != nil {
		return errors.Wrap(err, "could not unmount flist")
	}

	return nil
}

func buildNetworkInfo(ctx context.Context, userID string, iface string, cfg Kubernetes) (pkg.VMNetworkInfo, error) {
	network := stubs.NewNetworkerStub(GetZBus(ctx))

	netID := networkID(userID, string(cfg.NetworkID))
	subnet, err := network.GetSubnet(netID)
	if err != nil {
		return pkg.VMNetworkInfo{}, errors.Wrapf(err, "could not get network resource subnet")
	}

	if !subnet.Contains(cfg.IP) {
		return pkg.VMNetworkInfo{}, fmt.Errorf("IP %s is not part of local nr subnet %s", cfg.IP.String(), subnet.String())
	}

	addrCIDR := net.IPNet{
		IP:   cfg.IP,
		Mask: subnet.Mask,
	}

	gw, err := network.GetDefaultGwIP(netID)
	if err != nil {
		return pkg.VMNetworkInfo{}, errors.Wrapf(err, "could not get network resource default gateway")
	}

	networkInfo := pkg.VMNetworkInfo{
		Tap:         iface,
		MAC:         "", // rely on static IP configuration so we don't care here
		AddressCIDR: addrCIDR,
		GatewayIP:   net.IP(gw),
		Nameservers: []net.IP{net.ParseIP("8.8.8.8"), net.ParseIP("8.8.4.4")},
	}

	return networkInfo, nil
}

// returns the vCpu's, memory, disksize for a vm size
// memory and disk size is expressed in MiB
func vmSize(size uint8) (uint8, uint64, uint64, error) {
	switch size {
	case 1:
		// 1 vCpu, 2 GiB RAM, 50 GB disk
		return 1, 2 * 1024, 50 * 1024, nil
	case 2:
		// 2 vCpu, 4 GiB RAM, 100 GB disk
		return 2, 4 * 1024, 100 * 1024, nil
	}

	return 0, 0, 0, fmt.Errorf("unsupported vm size %d, only size 1 and 2 are supported", size)
}
