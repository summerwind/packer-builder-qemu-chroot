package chroot

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

type StepConnectImage struct {
	device string
}

func (s *StepConnectImage) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	device := state.Get("device").(string)
	imagePath := state.Get("image_path").(string)
	cmdWrapper := state.Get("command_wrapper").(CommandWrapper)

	ui.Say("Connecting source image as a network block device...")

	cmd, err := cmdWrapper(fmt.Sprintf("qemu-nbd -c %s %s", device, imagePath))
	if err != nil {
		err := fmt.Errorf("Error creating connect command: %s", err)
		return halt(state, err)
	}

	log.Printf("Target image path: %s", imagePath)
	log.Printf("Connect command: %s", cmd)

	shell := NewShellCommand(cmd)
	shell.Stderr = new(bytes.Buffer)
	if err := shell.Run(); err != nil {
		err := fmt.Errorf("Error connecting to the source image: %s\n%s", err, shell.Stderr)
		return halt(state, err)
	}

	// Wait for the device to be connected.
	time.Sleep(1 * time.Second)

	s.device = device
	state.Put("connect_image_cleanup", s)

	return multistep.ActionContinue
}

func (s *StepConnectImage) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)
	if err := s.CleanupFunc(state); err != nil {
		ui.Error(err.Error())
	}
}

func (s *StepConnectImage) CleanupFunc(state multistep.StateBag) error {
	ui := state.Get("ui").(packer.Ui)
	cmdWrapper := state.Get("command_wrapper").(CommandWrapper)

	if s.device == "" {
		return nil
	}

	ui.Say("Disconnecting the source image...")
	cmd, err := cmdWrapper(fmt.Sprintf("qemu-nbd -d %s", s.device))
	if err != nil {
		return fmt.Errorf("Error creating disconnect command: %s", err)
	}

	shell := NewShellCommand(cmd)
	shell.Stderr = new(bytes.Buffer)
	if err := shell.Run(); err != nil {
		return fmt.Errorf("Error disconnecting from source image: %s\n%s", err, shell.Stderr)
	}

	s.device = ""

	return nil
}
