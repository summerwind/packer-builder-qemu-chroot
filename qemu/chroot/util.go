package chroot

import (
	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

func halt(state multistep.StateBag, err error) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	ui.Error(err.Error())

	state.Put("error", err)

	return multistep.ActionHalt
}
