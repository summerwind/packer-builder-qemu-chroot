package chroot

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/packerbuilderdata"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

type mountPathData struct {
	Device string
}

type StepMountDevice struct {
	MountOptions   []string
	MountPartition string

	mountPath     string
	GeneratedData *packerbuilderdata.GeneratedData
}

func (s *StepMountDevice) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packersdk.Ui)
	device := state.Get("device").(string)
	wrappedCommand := state.Get("wrappedCommand").(common.CommandWrapper)

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

	ui.Say("Mounting the root device...")
	stderr := new(bytes.Buffer)

	opts := ""
	if len(config.MountOptions) > 0 {
		opts = "-o " + strings.Join(config.MountOptions, " -o ")
	}

	mountCommand := fmt.Sprintf("mount %s %sp%d %s", opts, device, config.MountPartition, mountPath)
	mountCommand, err = wrappedCommand(mountCommand)
	if err != nil {
		err := fmt.Errorf("Error creating mount command: %s", err)
		return halt(state, err)
	}
	log.Printf("[DEBUG] (step mount) mount command is %s", mountCommand)

	cmd := common.ShellCommand(mountCommand)
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		err := fmt.Errorf(
			"Error mounting root volume: %s\nStderr: %s", err, stderr.String())
		return halt(state, err)
	}

	// Set the mount path so we remember to unmount it later
	s.mountPath = mountPath
	state.Put("mount_path", mountPath)
	s.GeneratedData.Put("MountPath", s.mountPath)
	state.Put("mount_device_cleanup", s)

	return multistep.ActionContinue
}

func (s *StepMountDevice) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packersdk.Ui)
	if err := s.CleanupFunc(state); err != nil {
		ui.Error(err.Error())
	}
}

func (s *StepMountDevice) CleanupFunc(state multistep.StateBag) error {
	if s.mountPath == "" {
		return nil
	}

	ui := state.Get("ui").(packersdk.Ui)
	wrappedCommand := state.Get("wrappedCommand").(common.CommandWrapper)

	ui.Say("Unmounting the root device...")
	unmountCommand, err := wrappedCommand(fmt.Sprintf("umount %s", s.mountPath))
	if err != nil {
		return fmt.Errorf("Error creating unmount command: %s", err)
	}

	cmd := common.ShellCommand(unmountCommand)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Error unmounting root device: %s", err)
	}

	s.mountPath = ""
	return nil
}
