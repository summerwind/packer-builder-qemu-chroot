package chroot

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepCompressImage struct{}

func (s *StepCompressImage) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)

	if !config.Compression {
		return multistep.ActionContinue
	}

	ui := state.Get("ui").(packersdk.Ui)
	imagePath := state.Get("image_path").(string)
	wrappedCommand := state.Get("wrappedCommand").(common.CommandWrapper)
	stderr := new(bytes.Buffer)

	ui.Say("Compressing image...")
	tmpPath := imagePath + ".tmp"

	cmd := fmt.Sprintf("qemu-img convert -c -O qcow2 %s %s", imagePath, tmpPath)
	cmd, err := wrappedCommand(cmd)
	if err != nil {
		err := fmt.Errorf("Error creating compression command: %s", err)
		return halt(state, err)
	}

	log.Printf("Compression command: %s", cmd)

	shell := common.ShellCommand(cmd)
	shell.Stderr = stderr
	if err := shell.Run(); err != nil {
		err := fmt.Errorf("Error compressing image: %s\n%s", err, stderr.String())
		return halt(state, err)
	}

	if err := os.Rename(tmpPath, imagePath); err != nil {
		err := fmt.Errorf("Error renaming image: %s", err)
		return halt(state, err)
	}

	return multistep.ActionContinue
}

func (s *StepCompressImage) Cleanup(state multistep.StateBag) {}
