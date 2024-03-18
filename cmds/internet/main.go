package main

import (
	"flag"
	"net"
	"os"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/vishvananda/netlink"

	"github.com/threefoldtech/test/pkg/app"
	"github.com/threefoldtech/test/pkg/environment"
	"github.com/threefoldtech/test/pkg/network/bootstrap"
	"github.com/threefoldtech/test/pkg/network/bridge"
	"github.com/threefoldtech/test/pkg/network/dhcp"
	"github.com/threefoldtech/test/pkg/network/ifaceutil"
	"github.com/threefoldtech/test/pkg/network/options"
	"github.com/threefoldtech/test/pkg/network/types"
	"github.com/threefoldtech/test/pkg/zinit"

	"github.com/threefoldtech/test/pkg/version"
)

func main() {
	app.Initialize()

	var ver bool

	flag.BoolVar(&ver, "v", false, "show version and exit")
	flag.Parse()
	if ver {
		version.ShowAndExit(false)
	}

	if err := ifaceutil.SetLoUp(); err != nil {
		return
	}

	if err := configureZOS(); err != nil {
		log.Error().Err(err).Msg("failed to bootstrap network")
		os.Exit(1)
	}

	// wait for internet connection
	if err := check(); err != nil {
		log.Error().Err(err).Msg("failed to check internet connection")
		os.Exit(1)
	}

	log.Info().Msg("network bootstrapped successfully")
}

func debugZos() error {
	br, err := bridge.Get(types.DefaultBridge)
	if err != nil {
		return errors.Wrap(err, "failed to get default bridge")
	}

	addrs, err := netlink.AddrList(br, netlink.FAMILY_ALL)
	if err != nil {
		return errors.Wrap(err, "failed to list bridge addresses")
	}

	for _, addr := range addrs {
		log.Info().Str("address", addr.String()).Msg("default bridge has address")
	}

	routes, err := netlink.RouteList(br, netlink.FAMILY_ALL)
	if err != nil {
		return errors.Wrap(err, "failed to list bridge routes")
	}

	for _, route := range routes {
		log.Info().Str("route", route.String()).Msg("default bridge has routes")
	}

	return nil
}

func check() error {
	retries := 0
	f := func() error {
		retries += 1

		// print some helpful debugging information to help with debugging issues
		// if internet connection wasn't established
		if err := debugZos(); err != nil {
			log.Error().Err(err).Msg("failed to list default bridge debug information")
		}

		// we only care about possibility of establishing a connection
		// so just establishing a connection then close it is good enough
		for _, address := range []string{"bootstrap.grid.tf:http", "hub.grid.tf:http"} {
			log.Info().Str("trying out address", address).Msg("testing internet connection.")
			con, err := net.Dial("tcp", address)
			if err != nil {
				return errors.Wrapf(err, "failed to reach %s", address)
			}

			if err := con.Close(); err != nil {
				return err
			}
		}

		return nil
	}

	errHandler := func(err error, t time.Duration) {
		if err != nil {
			log.Info().Msg("internet connection is not ready yet")
			if retries%10 == 0 {
				log.Error().Err(err).Msgf("error while trying to test internet connectivity. %d retries attempted", retries)
			}
		}
	}

	return backoff.RetryNotify(f, backoff.NewExponentialBackOff(), errHandler)
}

/*
*
configureZOS bootstraps the private test network (private subnet) it goes as follows:
  - Find a physical interface that can get an IPv4 over DHCP
  - Once interface is found, a bridge called `test` is created, then the interface that was
    found in previous step is attached to the test bridge.
  - Bridge and interface are brought UP then a DHCP daemon is started on the test to get an IP.

In case there is a priv vlan is configured (kernel param vlan:priv=<id>) it is basically the same as
before but with the next twist:
- During probing of the interface, probing done on that vlan
- ZOS is added to vlan as `bridge vlan add vid <id> dev test pvid self untagged`
- link is added to vlan as `bridge vlan add vid <id> dev <link>`
*/
func configureZOS() error {

	env := environment.MustGet()

	f := func() error {
		log.Info().Msg("Start network bootstrap")

		ifaceConfigs, err := bootstrap.AnalyzeLinks(
			bootstrap.RequiresIPv4.WithVlan(env.PrivVlan),
			bootstrap.PhysicalFilter,
			bootstrap.PluggedFilter)
		if err != nil {
			log.Error().Err(err).Msg("failed to gather network interfaces configuration")
			return err
		}

		log.Info().Int("count", len(ifaceConfigs)).Msg("found interfaces with internet access")
		log.Info().Msgf("found interfaces: %+v", ifaceConfigs)
		testChild, err := bootstrap.SelectZOS(ifaceConfigs)
		if err != nil {
			log.Error().Err(err).Msg("failed to select a valid interface for test bridge")
			return err
		}

		log.Info().Str("interface", testChild).Msg("selecting interface")
		br, err := bootstrap.CreateDefaultBridge(types.DefaultBridge, env.PrivVlan)
		if err != nil {
			return err
		}

		time.Sleep(time.Second) // this is dirty

		link, err := netlink.LinkByName(testChild)
		if err != nil {
			return errors.Wrapf(err, "could not get link %s", testChild)
		}

		log.Info().
			Str("device", link.Attrs().Name).
			Str("bridge", br.Name).
			Msg("attach interface to bridge")

		if err := bridge.AttachNicWithMac(link, br); err != nil {
			log.Error().Err(err).
				Str("device", link.Attrs().Name).
				Str("bridge", br.Name).
				Msg("fail to attach device to bridge")
			return err
		}

		if err := options.Set(testChild, options.IPv6Disable(true)); err != nil {
			return errors.Wrapf(err, "failed to disable ip6 on test slave %s", testChild)
		}

		if err := netlink.LinkSetUp(link); err != nil {
			return errors.Wrapf(err, "could not bring %s up", testChild)
		}

		if env.PrivVlan != nil && env.PubVlan != nil {
			// if both priv and pub vlan are configured it means
			// that we can remove the default tagging of vlan 1
			// remove default
			if err := netlink.BridgeVlanDel(link, 1, true, true, false, false); err != nil {
				return errors.Wrapf(err, "failed to delete default vlan on device '%s'", link.Attrs().Name)
			}
		}

		if env.PrivVlan != nil {
			// add new vlan
			if err := netlink.BridgeVlanAdd(link, *env.PrivVlan, false, false, false, false); err != nil {
				return errors.Wrapf(err, "failed to set vlan on device '%s'", link.Attrs().Name)
			}
		}

		if env.PubVlan != nil {
			// add new vlan
			if err := netlink.BridgeVlanAdd(link, *env.PubVlan, false, false, false, false); err != nil {
				return errors.Wrapf(err, "failed to set vlan on device '%s'", link.Attrs().Name)
			}
		}

		dhcpService := dhcp.NewService(types.DefaultBridge, "", zinit.Default())
		if err := dhcpService.DestroyOlderService(); err != nil {
			log.Error().Err(err).Msgf("failed to destory older %s service", dhcpService.Name)
			return err
		}
		// create the new service anyway
		if err := dhcpService.Create(); err != nil {
			log.Error().Err(err).Msgf("failed to create %s service", dhcpService.Name)
			return err
		}

		if err := dhcpService.Start(); err != nil {
			log.Error().Err(err).Msgf("failed to start %s service", dhcpService.Name)
			return err
		}

		return nil
	}

	errHandler := func(err error, t time.Duration) {
		if err != nil {
			log.Error().Err(err).Msg("error while trying to bootstrap network")
		}
	}

	return backoff.RetryNotify(f, backoff.NewExponentialBackOff(), errHandler)
}
