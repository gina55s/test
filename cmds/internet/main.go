package main

import (
	"flag"
	"os"
	"os/exec"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/vishvananda/netlink"

	"github.com/threefoldtech/test/pkg/app"
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

func check() error {
	f := func() error {
		cmd := exec.Command("wget", "google.com", "-O", "/dev/null", "-T", "5")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return cmd.Run()
	}

	errHandler := func(err error, t time.Duration) {
		if err != nil {
			log.Error().Err(err).Msg("error while trying to test internet connectivity")
		}
	}

	return backoff.RetryNotify(f, backoff.NewExponentialBackOff(), errHandler)
}

func configureZOS() error {
	f := func() error {
		log.Info().Msg("Start network bootstrap")

		ifaceConfigs, err := bootstrap.AnalyzeLinks(
			bootstrap.RequiresIPv4,
			bootstrap.PhysicalFilter,
			bootstrap.PluggedFilter)
		if err != nil {
			log.Error().Err(err).Msg("failed to gather network interfaces configuration")
			return err
		}

		testChild, err := bootstrap.SelectZOS(ifaceConfigs)
		if err != nil {
			log.Error().Err(err).Msg("failed to select a valid interface for test bridge")
			return err
		}

		br, err := bootstrap.CreateDefaultBridge(types.DefaultBridge)
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
