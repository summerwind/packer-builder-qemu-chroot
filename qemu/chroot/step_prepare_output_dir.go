package chroot

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepPrepareOutputDir struct {
	success bool
}

func (s *StepPrepareOutputDir) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packersdk.Ui)

	if _, err := os.Stat(config.OutputDir); err == nil {
		if !config.PackerForce {
			err := fmt.Errorf("Output directory already exists: %s", config.OutputDir)
			return halt(state, err)
		}

		ui.Say("Deleting previous output directory...")
		os.RemoveAll(config.OutputDir)
	}

	ui.Say(fmt.Sprintf("Creating output directory %s...", config.OutputDir))
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		err := fmt.Errorf("Error creating output directory: %s", config.OutputDir)
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
		ui := state.Get("ui").(packersdk.Ui)

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
