package upgrade

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/blang/semver"
	"github.com/pkg/errors"

	"github.com/threefoldtech/test/pkg/zinit"

	"github.com/rs/zerolog/log"

	"github.com/threefoldtech/test/pkg"
)

type hookType string

const (
	provisionModuleName = "provisiond"

	// those values must match the values
	// in the bootstrap process. (bootstrap.sh)

	nameFile = "/tmp/flist.name"
	infoFile = "/tmp/flist.info"
)

var (
	// ErrRestartNeeded is returned if upgraded requires a restart
	ErrRestartNeeded = fmt.Errorf("restart needed")

	// services that can't be uninstalled with normal procedure
	protected = []string{"identityd", "redis"}
)

// BootMethod defines the node boot method
type BootMethod string

const (
	// BootMethodFList booted from an flist
	BootMethodFList BootMethod = "flist"

	// BootMethodOther booted with other methods
	BootMethodOther BootMethod = "other"
)

// DetectBootMethod tries to detect the boot method
// of the node
func DetectBootMethod() BootMethod {
	log.Info().Msg("detecting boot method")
	_, err := os.Stat(nameFile)
	if err != nil {
		log.Warn().Err(err).Msg("no flist file found")
		return BootMethodOther
	}

	// NOTE: we can add a check to see if the flist
	// in the file is valid, but this means we need
	// to do a call to the hub, hence the detection
	// can be affected by the network state, or the
	// hub state. So we return immediately
	return BootMethodFList
}

// Upgrader is the component that is responsible
// to keep 0-OS up to date
type Upgrader struct {
	FLister      pkg.Flister
	Zinit        *zinit.Client
	NoSelfUpdate bool
	hub          hubClient
}

// Name always return name of the boot flist. If name file
// does not exist, an empty string is returned
func (u *Upgrader) Name() string {
	data, _ := ioutil.ReadFile(nameFile)
	return strings.TrimSpace(string(data))
}

// currentInfo returns current flist information
func (u *Upgrader) currentInfo() (flistInfo, error) {
	name := u.Name()
	if len(name) == 0 {
		return flistInfo{}, fmt.Errorf("flist name is not known")
	}

	return loadInfo(name, infoFile)
}

// Version always returns curent version of flist
func (u *Upgrader) Version() (semver.Version, error) {
	info, err := u.currentInfo()
	if err != nil {
		return semver.Version{}, errors.Wrap(err, "failed to load flist info")
	}

	return info.Version()
}

// Upgrade is the method that does a full upgrade flow
// first check if a new version is available
// if yes, applies the upgrade
// on a successfully update, upgrade WILL NOT RETURN
// instead the upgraded daemon will be completely stopped
func (u *Upgrader) Upgrade() error {
	info, err := u.hub.Info(u.Name())
	if err != nil {
		return errors.Wrap(err, "failed to get remote flist info")
	}

	current, err := u.Version()
	if err != nil {
		log.Error().Err(err).Msg("failed to detect current version. Update to latest anyway")
	}

	latest, err := info.Version()
	if err != nil {
		return errors.Wrap(err, "failed to parse latest version")
	}

	if latest.GT(current) {
		return u.applyUpgrade(latest, info)
	}

	return nil
}

func (u Upgrader) stopMultiple(timeout time.Duration, service ...string) error {
	services := make(map[string]struct{})
	for _, name := range service {
		log.Info().Str("service", name).Msg("stopping service")
		if err := u.Zinit.Stop(name); err != nil {
			log.Debug().Str("service", name).Msg("service undefined")
			continue
		}

		services[name] = struct{}{}
	}

	deadline := time.After(timeout)

	for len(services) > 0 {
		var stopped []string
		for service := range services {
			status, err := u.Zinit.Status(service)
			if err != nil {
				return err
			}

			if status.Target != zinit.ServiceTargetDown {
				// it means some other entity (another client or command line)
				// has set the service back to up. I think we should immediately return
				// with an error instead.
				return fmt.Errorf("expected service '%s' target should be DOWN. found UP", service)
			}

			if status.State.Exited() {
				stopped = append(stopped, service)
			}
		}

		for _, stop := range stopped {
			if _, ok := services[stop]; ok {
				log.Debug().Str("service", stop).Msg("service stopped")
				delete(services, stop)
			}
		}

		if len(services) == 0 {
			break
		}

		select {
		case <-deadline:
			for service := range services {
				log.Warn().Str("service", service).Msg("service didn't stop in time. use SIGKILL")
				u.Zinit.Kill(service, syscall.SIGKILL)
			}
		case <-time.After(1 * time.Second):
		}
	}

	return nil
}

// upgradeSelf will try to check if the flist has
// an upgraded binary with different revision. If yes
// it will copy the new binary and ask for a restart.
// next time this method is called, it will match the flist
// revision, and hence will continue updating all the other daemons
func (u *Upgrader) upgradeSelf(root string) error {
	if u.NoSelfUpdate {
		log.Debug().Msg("skipping self upgrade")
		return nil
	}

	current := currentRevision()
	log.Debug().Str("revision", current).Msg("current revision")

	bin := filepath.Join(root, currentBinPath())

	if !exists(bin) {
		// no bin for update daemon in the flist.
		log.Debug().Str("bin", bin).Msg("binary file does not exist")
		return nil
	}

	// the timeout here is set to 1 min because
	// this most probably will trigger a download
	// of the binary over 0-fs, hence we need to
	// give it enough time to download the file
	// on slow network (i am looking at u Egypt)
	new, err := revisionOf(bin, 2*time.Minute)
	if err != nil {
		return errors.Wrap(err, "failed to check new update daemon revision number")
	}

	log.Debug().Str("revision", new).Msg("new revision")

	// nothing to be done here.
	if current == new {
		return nil
	}

	if err := copyFile(currentBinPath(), bin); err != nil {
		return err
	}

	log.Debug().Msg("revisions are differnet, self upgrade is needed")
	return ErrRestartNeeded
}

func (u *Upgrader) uninstall() error {
	current, err := u.currentInfo()
	if err != nil {
		return errors.Wrap(err, "failed to get current flist info")
	}

	files, err := current.Files()
	if err != nil {
		return errors.Wrapf(err, "failed to get list of current installed files for '%s'", current.Absolute())
	}

	//stop all services names
	var names []string
	for _, file := range files {
		dir := filepath.Dir(file.Path)
		if dir != "/etc/zinit" {
			continue
		}

		name := filepath.Base(file.Path)
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}

		name = strings.TrimSuffix(name, ".yaml")
		// skip self and redis
		if isIn(name, protected) {
			continue
		}

		names = append(names, name)
	}

	log.Debug().Strs("services", names).Msg("stopping services")

	if err = u.stopMultiple(20*time.Second, names...); err != nil {
		return errors.Wrapf(err, "failed to stop services")
	}

	// we do a forget so any changes of the zinit config
	// themselves get reflected once monitored again
	for _, name := range names {
		if err := u.Zinit.Forget(name); err != nil {
			log.Error().Err(err).Str("service", name).Msg("error on zinit forget")
		}
	}

	// now delete ALL files, ignore what doesn't delete
	for _, file := range files {
		log.Debug().Str("file", file.Path).Msg("deleting file")
		stat, err := os.Stat(file.Path)
		if err != nil {
			log.Debug().Err(err).Str("file", file.Path).Msg("failed to check file")
			continue
		}
		if stat.IsDir() {
			continue
		}

		if err := os.Remove(file.Path); err != nil {
			log.Error().Err(err).Str("file", file.Path).Msg("failed to remove file")
		}
	}

	return nil
}

func (u *Upgrader) applyUpgrade(version semver.Version, info flistInfo) error {
	log.Info().Str("flist", u.Name()).Str("version", version.String()).Msg("start applying upgrade")

	flistRoot, err := u.FLister.Mount(u.hub.MountURL(u.Name()), u.hub.StorageURL())
	if err != nil {
		return err
	}

	defer func() {
		if err := u.FLister.Umount(flistRoot); err != nil {
			log.Error().Err(err).Msgf("fail to umount flist at %s: %v", flistRoot, err)
		}
	}()

	if err := u.upgradeSelf(flistRoot); err != nil {
		return err
	}

	if err := u.uninstall(); err != nil {
		log.Error().Err(err).Msg("failed to unistall current flist. Upgraded anyway")
	}

	log.Info().Msg("clean up complete, copying new files")
	// once the flist is mounted we can inspect
	// it for all zinit config files.
	files, err := ioutil.ReadDir(filepath.Join(flistRoot, "etc", "zinit"))
	if err != nil {
		return errors.Wrap(err, "invalid flist. no zinit services")
	}

	var names []string
	for _, service := range files {
		name := service.Name()
		if service.IsDir() || !strings.HasSuffix(name, ".yaml") {
			continue
		}

		names = append(names, strings.TrimSuffix(name, ".yaml"))
	}

	log.Debug().Strs("services", names).Msg("new services")

	if err := copyRecursive(flistRoot, "/"); err != nil {
		return err
	}

	log.Debug().Msg("copying files complete")
	// start all services in the flist
	for _, name := range names {
		if err := u.Zinit.Monitor(name); err != nil {
			log.Error().Err(err).Str("service", name).Msg("error on zinit monitor")
		}

		// while we totally do not need to call start after monitor but
		// monitor won't take an action on a monitored service if it's
		// stopped (but not forgoten). So we call start just to be sure
		if err := u.Zinit.Start(name); err != nil {
			log.Error().Err(err).Str("service", name).Msg("error on zinit start")
		}
	}

	if err := info.Commit(infoFile); err != nil {
		return err
	}

	return nil
}

func copyRecursive(source string, destination string, skip ...string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		if isIn(rel, skip) {
			if info.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}

		dest := filepath.Join(destination, rel)
		if info.IsDir() {
			if err := os.MkdirAll(dest, info.Mode()); err != nil {
				return err
			}
		} else {
			// regular file (or other types that we don't handle)
			if err := copyFile(dest, path); err != nil {
				return err
			}
		}

		return nil
	})
}

func isIn(target string, list []string) bool {
	for _, x := range list {
		if target == x {
			return true
		}
	}
	return false
}

func copyFile(dst, src string) error {
	log.Info().Str("source", src).Str("destination", dst).Msg("copy file")

	var (
		isNew  = false
		dstOld string
	)

	if _, err := os.Stat(dst); os.IsNotExist(err) {
		// case where this is a new file
		// we just need to copy from flist to root
		isNew = true
	}

	if !isNew {
		dstOld = dst + ".old"
		if err := os.Rename(dst, dstOld); err != nil {
			return err
		}
	}

	fSrc, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fSrc.Close()

	stat, err := fSrc.Stat()
	if err != nil {
		return err
	}

	fDst, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_SYNC, stat.Mode().Perm())
	if err != nil {
		return err
	}
	defer fDst.Close()

	if _, err = io.Copy(fDst, fSrc); err != nil {
		return err
	}

	if !isNew {
		return os.Remove(dstOld)
	}
	return nil
}
