package chroot

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hashicorp/packer/packer"
	"github.com/mitchellh/multistep"
)

type StepPrepareOutputDir struct {
	success bool
}

func (s *StepPrepareOutputDir) Run(state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packer.Ui)

	if _, err := os.Stat(config.OutputDir); err == nil {
		if !config.PackerForce {
			err := fmt.Errorf("Output directory already exists: %s", config.OutputDir)
			return halt(state, err)
		}

		ui.Say("Deleting previous output directory...")
		os.RemoveAll(config.OutputDir)
	}

	ui.Say("Creating output directory...")
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return halt(state, err)
	}

	s.success = true

	return multistep.ActionContinue
}

func (s *StepPrepareOutputDir) Cleanup(state multistep.StateBag) {
	if !s.success {
		return
	}

	_, cancelled := state.GetOk(multistep.StateCancelled)
	_, halted := state.GetOk(multistep.StateHalted)

	if cancelled || halted {
		config := state.Get("config").(*Config)
		ui := state.Get("ui").(packer.Ui)

		ui.Say("Deleting output directory...")
		for i := 0; i < 5; i++ {
			err := os.RemoveAll(config.OutputDir)
			if err == nil {
				break
			}

			log.Printf("Error removing output dir: %s", err)
			time.Sleep(2 * time.Second)
		}
	}
}