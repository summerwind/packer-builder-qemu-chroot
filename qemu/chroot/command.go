package chroot

import (
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
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
