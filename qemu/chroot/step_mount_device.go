package chroot

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/packer/packer"
	"github.com/hashicorp/packer/template/interpolate"
	"github.com/mitchellh/multistep"
)

type mountPathData struct {
	Device string
}

type StepMountDevice struct {
	mountPath string
}

func (s *StepMountDevice) Run(state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packer.Ui)
	device := state.Get("device").(string)
	cmdWrapper := state.Get("command_wrapper").(CommandWrapper)

	ctx := config.ctx
	ctx.Data = &mountPathData{Device: filepath.Base(device)}

	mountPath, err := interpolate.Render(config.MountPath, &ctx)
	if err != nil {
		err := fmt.Errorf("Error preparing mount directory: %s", err)
		return halt(state, err)
	}

	mountPath, err = filepath.Abs(mountPath)
	if err != nil {
		err := fmt.Errorf("Error preparing mount directory: %s", err)
		return halt(state, err)
	}

	log.Printf("Mount path: %s", mountPath)

	if err := os.MkdirAll(mountPath, 0755); err != nil {
		err := fmt.Errorf("Error creating mount directory: %s", err)
		return halt(state, err)
	}

	ui.Say("Mounting device...")

	opts := ""
	if len(config.MountOptions) > 0 {
		opts = "-o " + strings.Join(config.MountOptions, " -o ")
	}

	cmd := fmt.Sprintf("mount %s %sp%d %s", opts, device, config.MountPartition, mountPath)
	cmd, err = cmdWrapper(cmd)
	if err != nil {
		err := fmt.Errorf("Error creating mount command: %s", err)
		return halt(state, err)
	}

	log.Printf("Mount command: %s", cmd)

	shell := NewShellCommand(cmd)
	shell.Stderr = new(bytes.Buffer)
	if err := shell.Run(); err != nil {
		err := fmt.Errorf("Error mounting device: %s\n%s", err, shell.Stderr)
		return halt(state, err)
	}

	s.mountPath = mountPath
	state.Put("mount_path", mountPath)
	state.Put("mount_device_cleanup", s)

	return multistep.ActionContinue
}

func (s *StepMountDevice) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)
	if err := s.CleanupFunc(state); err != nil {
		ui.Error(err.Error())
	}
}

func (s *StepMountDevice) CleanupFunc(state multistep.StateBag) error {
	ui := state.Get("ui").(packer.Ui)
	cmdWrapper := state.Get("command_wrapper").(CommandWrapper)

	if s.mountPath == "" {
		return nil
	}

	ui.Say("Unmounting device...")
	cmd, err := cmdWrapper(fmt.Sprintf("umount %s", s.mountPath))
	if err != nil {
		return fmt.Errorf("Error creating unmount command: %s", err)
	}

	shell := NewShellCommand(cmd)
	shell.Stderr = new(bytes.Buffer)
	if err := shell.Run(); err != nil {
		return fmt.Errorf("Error unmounting device: %s\n%s", err, shell.Stderr)
	}

	s.mountPath = ""

	return nil
}
