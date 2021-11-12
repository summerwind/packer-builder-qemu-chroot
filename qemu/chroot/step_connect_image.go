package chroot

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepConnectImage struct {
	device string
}

func (s *StepConnectImage) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	device := state.Get("device").(string)
	imagePath := state.Get("image_path").(string)
	wrappedCommand := state.Get("wrappedCommand").(common.CommandWrapper)
	stderr := new(bytes.Buffer)

	ui.Say(fmt.Sprintf("Connecting source image as a network block device to %s...", device))

	cmd, err := wrappedCommand(fmt.Sprintf("qemu-nbd -c %s %s", device, imagePath))
	if err != nil {
		err := fmt.Errorf("Error creating connect command: %s", err)
		return halt(state, err)
	}

	log.Printf("Target image path: %s", imagePath)
	log.Printf("Connect command: %s", cmd)

	shell := common.ShellCommand(cmd)
	shell.Stderr = stderr
	if err := shell.Run(); err != nil {
		err := fmt.Errorf("Error connecting to the source image: %s\n%s", err, stderr.String())
		return halt(state, err)
	}

	// Wait for the device to be connected.
	time.Sleep(1 * time.Second)

	s.device = device
	// different step naming in the common library (https://github.com/hashicorp/packer-plugin-sdk/blob/main/chroot/step_early_cleanup.go)
	state.Put("attach_cleanup", s)

	return multistep.ActionContinue
}

func (s *StepConnectImage) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packersdk.Ui)
	if err := s.CleanupFunc(state); err != nil {
		ui.Error(err.Error())
	}
}

func (s *StepConnectImage) CleanupFunc(state multistep.StateBag) error {
	ui := state.Get("ui").(packersdk.Ui)
	wrappedCommand := state.Get("wrappedCommand").(common.CommandWrapper)

	if s.device == "" {
		return nil
	}

	ui.Say(fmt.Sprintf("Disconnecting the source image from %s...", s.device))
	cmd, err := wrappedCommand(fmt.Sprintf("qemu-nbd -d %s", s.device))
	if err != nil {
		return fmt.Errorf("Error creating disconnect command: %s", err)
	}

	shell := common.ShellCommand(cmd)
	shell.Stderr = new(bytes.Buffer)
	if err := shell.Run(); err != nil {
		return fmt.Errorf("Error disconnecting from source image: %s\n%s", err, shell.Stderr)
	}

	s.device = ""

	return nil
}
