package chroot

import (
	"os/exec"

	"github.com/hashicorp/packer/template/interpolate"
)

type wrappedCommandData struct {
	Command string
}

type CommandWrapper func(string) (string, error)

func NewCommandWrapper(config Config) CommandWrapper {
	return CommandWrapper(func(command string) (string, error) {
		ctx := config.ctx
		ctx.Data = &wrappedCommandData{Command: command}
		return interpolate.Render(config.CommandWrapper, &ctx)
	})
}

func NewShellCommand(command string) *exec.Cmd {
	return exec.Command("/bin/sh", "-c", command)
}
