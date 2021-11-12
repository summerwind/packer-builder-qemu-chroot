package chroot

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepPrepareImage struct {
	imagePath string
}

func (s *StepPrepareImage) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packersdk.Ui)

	sourcePath, err := filepath.Abs(config.SourceImage)
	if err != nil {
		err := fmt.Errorf("Error checking source image: %s", err)
		return halt(state, err)
	}

	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		err := fmt.Errorf("Source image not found: %s", sourcePath)
		return halt(state, err)
	}

	log.Printf("Source image path: %s", sourcePath)
	ui.Say("Copying source image...")

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		err := fmt.Errorf("Error opening source image file: %s", err)
		return halt(state, err)
	}

	imagePath := filepath.Join(config.OutputDir, config.ImageName)
	imageFile, err := os.OpenFile(imagePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		err := fmt.Errorf("Error opening image file: %s", err)
		return halt(state, err)
	}

	_, err = io.Copy(imageFile, sourceFile)
	if err != nil {
		err := fmt.Errorf("Error copying source image file: %s", err)
		return halt(state, err)
	}

	err = imageFile.Sync()
	if err != nil {
		err := fmt.Errorf("Error syncing image file: %s", err)
		return halt(state, err)
	}

	s.imagePath = imageFile.Name()
	state.Put("image_path", imageFile.Name())

	return multistep.ActionContinue
}

func (s *StepPrepareImage) Cleanup(state multistep.StateBag) {}
