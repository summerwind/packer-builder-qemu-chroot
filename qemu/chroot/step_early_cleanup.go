package chroot

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/packer/helper/multistep"
)

type StepEarlyCleanup struct{}

func (s *StepEarlyCleanup) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	keys := []string{
		"copy_files_cleanup",
		"mount_extra_cleanup",
		"mount_device_cleanup",
		"connect_image_cleanup",
	}

	for _, key := range keys {
		log.Printf("Running cleanup: %s", key)
		c := state.Get(key).(Cleaner)

		if err := c.CleanupFunc(state); err != nil {
			err := fmt.Errorf("Error cleaning up: %s", err)
			return halt(state, err)
		}
	}

	return multistep.ActionContinue
}

func (s *StepEarlyCleanup) Cleanup(state multistep.StateBag) {}
