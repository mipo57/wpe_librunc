package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	"github.com/opencontainers/runc/libcontainer/intelrdt"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux"
)

func validateProcessSpec(spec *specs.Process) error {
	if spec.Cwd == "" {
		return fmt.Errorf("Cwd property must not be empty")
	}
	if !filepath.IsAbs(spec.Cwd) {
		return fmt.Errorf("Cwd must be an absolute path")
	}
	if len(spec.Args) == 0 {
		return fmt.Errorf("args must not be empty")
	}
	if spec.SelinuxLabel != "" && !selinux.GetEnabled() {
		return fmt.Errorf("selinux label is specified in config, but selinux is disabled or not supported")
	}
	return nil
}

func loadSpecs(cPath string) (spec *specs.Spec, err error) {
	cf, err := os.Open(cPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("JSON specification file %s not found", cPath)
		}
		return nil, err
	}
	defer cf.Close()

	if err = json.NewDecoder(cf).Decode(&spec); err != nil {
		return nil, err
	}

	return spec, validateProcessSpec(spec.Process)
}

func revisePidFile(pidFile string) (string, error) {
	if pidFile == "" {
		return "", nil
	}

	return filepath.Abs(pidFile)
}

// loadFactory returns the configured factory instance for execing containers.
func loadFactory(root string, rootless bool, systemd_cgroup bool, criu: string) (libcontainer.Factory, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	// We default to cgroupfs, and can only use systemd if the system is a
	// systemd box.
	cgroupManager := libcontainer.Cgroupfs

	if rootless {
		cgroupManager = libcontainer.RootlessCgroupfs
	}
	if systemd_cgroup {
		if systemd.UseSystemd() {
			cgroupManager = libcontainer.SystemdCgroups
		} else {
			return nil, fmt.Errorf("systemd cgroup flag passed, but systemd support for managing cgroups is not available")
		}
	}

	intelRdtManager := libcontainer.IntelRdtFs
	if !intelrdt.IsCatEnabled() && !intelrdt.IsMbaEnabled() {
		intelRdtManager = nil
	}

	// We resolve the paths for {newuidmap,newgidmap} from the context of runc,
	// to avoid doing a path lookup in the nsexec context. TODO: The binary
	// names are not currently configurable.
	newuidmap, err := exec.LookPath("newuidmap")
	if err != nil {
		newuidmap = ""
	}
	newgidmap, err := exec.LookPath("newgidmap")
	if err != nil {
		newgidmap = ""
	}

	return libcontainer.New(abs, cgroupManager, intelRdtManager,
		libcontainer.CriuPath(criu),
		libcontainer.NewuidmapPath(newuidmap),
		libcontainer.NewgidmapPath(newgidmap))
}

func Create(name string, cPath string, rootless bool) error {
	spec, err := loadSpecs(cPath)
	if err != nil {
		return err
	}

	config, err := specconv.CreateLibcontainerConfig(&specconv.CreateOpts{
		CgroupName:       name,
		UseSystemdCgroup: false,
		NoPivotRoot:      false,
		NoNewKeyring:     false,
		Spec:             spec,
		RootlessEUID:     os.Geteuid() != 0,
		RootlessCgroups:  rootless,
	})

	if err != nil {
		return err
	}

	factory, err := loadFactory(context)
	if err != nil {
		return err
	}
	return factory.Create(id, config)

	return nil
}

func main() {
	loadSpecs("")
}
