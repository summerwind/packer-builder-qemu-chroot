package main

import (
	"github.com/hashicorp/packer/packer/plugin"
	"github.com/summerwind/packer-builder-qemu-chroot/qemu/chroot"
)

func main() {
	server, err := plugin.Server()
	if err != nil {
		panic(err)
	}

	server.RegisterBuilder(chroot.NewBuilder())
	server.Serve()
}
