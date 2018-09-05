package chroot

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

type StepMountExtra struct {
	mountPaths []string
}

func (s *StepMountExtra) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packer.Ui)
	mountPath := state.Get("mount_path").(string)
	cmdWrapper := state.Get("command_wrapper").(CommandWrapper)

	s.mountPaths = make([]string, 0, len(config.ChrootMounts))

	ui.Say("Mounting additional paths within the chroot...")
	for _, mountInfo := range config.ChrootMounts {
		p := filepath.Join(mountPath, mountInfo[2])

		if err := os.MkdirAll(p, 0755); err != nil {
			err := fmt.Errorf("Error creating mount directory: %s", err)
			return halt(state, err)
		}

		ui.Message(fmt.Sprintf("Mounting: %s", mountInfo[2]))

		flags := "-t " + mountInfo[0]
		if mountInfo[0] == "bind" {
			flags = "--bind"
		}

		opts := ""
		if len(mountInfo) > 3 {
			opts = "-o " + strings.Join(mountInfo[3:], " -o ")
		}

		cmd := fmt.Sprintf("mount %s %s %s %s", flags, opts, mountInfo[1], p)
		cmd, err := cmdWrapper(cmd)
		if err != nil {
			err := fmt.Errorf("Error creating mount command: %s", err)
			return halt(state, err)
		}

		log.Printf("Mount command: %s", cmd)

		shell := NewShellCommand(cmd)
		shell.Stderr = new(bytes.Buffer)
		if err := shell.Run(); err != nil {
			err := fmt.Errorf("Error mounting path: %s\n%s", err, shell.Stderr)
			return halt(state, err)
		}

		s.mountPaths = append(s.mountPaths, p)
	}

	state.Put("mount_extra_cleanup", s)

	return multistep.ActionContinue
}

func (s *StepMountExtra) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)
	if err := s.CleanupFunc(state); err != nil {
		ui.Error(err.Error())
	}
}

func (s *StepMountExtra) CleanupFunc(state multistep.StateBag) error {
	ui := state.Get("ui").(packer.Ui)
	cmdWrapper := state.Get("command_wrapper").(CommandWrapper)

	if s.mountPaths == nil {
		return nil
	}

	ui.Say("Unmounting additional paths...")

	lastIndex := len(s.mountPaths) - 1
	for i := lastIndex; i >= 0; i-- {
		path := s.mountPaths[i]

		cmd, err := cmdWrapper(fmt.Sprintf("grep %s /proc/mounts", path))
		if err != nil {
			return fmt.Errorf("Error creating grep command: %s", err)
		}

		shell := NewShellCommand(cmd)
		shell.Stderr = new(bytes.Buffer)
		if err := shell.Run(); err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitStatus := 0
				if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
					exitStatus = status.ExitStatus()
				}
				if exitStatus == 1 {
					continue
				}
			}
		}

		cmd, err = cmdWrapper(fmt.Sprintf("umount %s", path))
		if err != nil {
			return fmt.Errorf("Error creating unmount command: %s", err)
		}

		shell = NewShellCommand(cmd)
		shell.Stderr = new(bytes.Buffer)
		if err := shell.Run(); err != nil {
			return fmt.Errorf("Error unmounting path: %s\n%s", err, shell.Stderr)
		}
	}

	s.mountPaths = nil

	return nil
}
