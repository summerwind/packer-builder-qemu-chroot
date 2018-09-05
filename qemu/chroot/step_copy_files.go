package chroot

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

type StepCopyFiles struct {
	files []string
}

func (s *StepCopyFiles) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packer.Ui)
	mountPath := state.Get("mount_path").(string)
	cmdWrapper := state.Get("command_wrapper").(CommandWrapper)

	s.files = make([]string, 0, len(config.CopyFiles))

	ui.Say("Copying files to withnin the chroot...")
	for _, srcPath := range config.CopyFiles {
		destPath := filepath.Join(mountPath, srcPath)

		ui.Message(fmt.Sprintf("Copying: %s", srcPath))

		cmd := fmt.Sprintf("cp --remove-destination %s %s", srcPath, destPath)
		cmd, err := cmdWrapper(cmd)
		if err != nil {
			err := fmt.Errorf("Error building copy command: %s", err)
			return halt(state, err)
		}

		log.Printf("Copy command: %s", cmd)

		shell := NewShellCommand(cmd)
		shell.Stderr = new(bytes.Buffer)
		if err := shell.Run(); err != nil {
			err := fmt.Errorf("Error copying file: %s\n%s", err, shell.Stderr)
			return halt(state, err)
		}

		s.files = append(s.files, destPath)
	}

	state.Put("copy_files_cleanup", s)

	return multistep.ActionContinue
}

func (s *StepCopyFiles) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)
	if err := s.CleanupFunc(state); err != nil {
		ui.Error(err.Error())
	}
}

func (s *StepCopyFiles) CleanupFunc(state multistep.StateBag) error {
	cmdWrapper := state.Get("command_wrapper").(CommandWrapper)

	if s.files == nil {
		return nil
	}

	for _, file := range s.files {
		log.Printf("Removing file: %s", file)

		cmd, err := cmdWrapper(fmt.Sprintf("rm -f %s", file))
		if err != nil {
			return err
		}

		shell := NewShellCommand(cmd)
		shell.Stderr = new(bytes.Buffer)
		if err := shell.Run(); err != nil {
			return fmt.Errorf("Error removing file: %s\n%s", err, shell.Stderr)
		}
	}

	s.files = nil

	return nil
}
