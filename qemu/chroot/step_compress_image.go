package chroot

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/packer/packer"
	"github.com/mitchellh/multistep"
)

type StepCompressImage struct{}

func (s *StepCompressImage) Run(state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packer.Ui)
	imagePath := state.Get("image_path").(string)
	cmdWrapper := state.Get("command_wrapper").(CommandWrapper)

	if !config.Compression {
		return multistep.ActionContinue
	}

	ui.Say("Compressing image...")
	tmpPath := imagePath + ".tmp"

	cmd := fmt.Sprintf("qemu-img convert -c -O qcow2 %s %s", imagePath, tmpPath)
	cmd, err := cmdWrapper(cmd)
	if err != nil {
		err := fmt.Errorf("Error creating compression command: %s", err)
		return halt(state, err)
	}

	log.Printf("Compression command: %s", cmd)

	shell := NewShellCommand(cmd)
	shell.Stderr = new(bytes.Buffer)
	if err := shell.Run(); err != nil {
		err := fmt.Errorf("Error compressing image: %s\n%s", err, shell.Stderr)
		return halt(state, err)
	}

	if err := os.Rename(tmpPath, imagePath); err != nil {
		err := fmt.Errorf("Error renaming image: %s", err)
		return halt(state, err)
	}

	return multistep.ActionContinue
}

func (s *StepCompressImage) Cleanup(state multistep.StateBag) {}
