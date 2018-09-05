package chroot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

const (
	devicePrefix string = "nbd"
)

type StepPrepareDevice struct{}

func (s *StepPrepareDevice) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packer.Ui)

	ui.Say("Finding available device...")

	devicePath := config.DevicePath
	if devicePath == "" {
		var err error

		log.Println("Device path not specified, searching for available device...")

		devicePath, err = findAvailableDevice()
		if err != nil {
			err := fmt.Errorf("Error finding available device: %s", err)
			return halt(state, err)
		}
	} else {
		if !isAvailable(devicePath) {
			err := fmt.Errorf("Device is not available: %s", devicePath)
			return halt(state, err)
		}
	}

	log.Printf("Device: %s", devicePath)
	state.Put("device", devicePath)

	return multistep.ActionContinue
}

func (s *StepPrepareDevice) Cleanup(state multistep.StateBag) {}

func findAvailableDevice() (string, error) {
	for i := 0; i < 10; i++ {
		device := fmt.Sprintf("%s%d", devicePrefix, i)

		devicePath := fmt.Sprintf("/dev/%s", device)
		_, err := os.Stat(devicePath)
		if err != nil {
			continue
		}

		if isAvailable(device) {
			return devicePath, nil
		}
	}

	return "", errors.New("available device could not be found")
}

func isAvailable(devicePath string) bool {
	device := filepath.Base(devicePath)
	pidPath := fmt.Sprintf("/sys/block/%s/pid", device)
	_, err := os.Stat(pidPath)
	return (err != nil)
}
