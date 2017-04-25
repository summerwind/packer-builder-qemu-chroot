# QEMU chroot builder for Packer

A builder plugin of Packer to support building QEMU images within chroot.

## Prerequirements

This plugin depends on the following tools:

- Packer
- QEMU Utilities (`qemu-nbd` and `qemu-img`)
- NBD kernel module

## Install

Download the binary from the [Releases](https://github.com/summerwind/packer-builder-qemu-chroot/releases) page and place it in one of the following places:

- The directory where packer is, or the executable directory
- `~/.packer.d/plugins` on Unix systems or `%APPDATA%/packer.d/plugins` on Windows
- The current working directory

## Build plugin

You can build this plugin with the following command. Please install [Glide](https://github.com/Masterminds/glide) before the build.

```
$ make
```

## How Does it Work?

This plugin mounts the specified QCOW2 format image to the file system using the `qemu-nbd` command. Once mounted, a `chroot` command is used to provision the system within the image. After provisioning, the image is unmounted and save it as the QCOW2 format.

Using this process eliminates the need to start the virtual machine, so you can provision the image faster.

## Quick Start

To use this plugin, you need to load the NBD kernel module first. Note that this process must be executed on a Linux.

```
$ sudo modprobe nbd
```

Prepare the following template file.

```
$ vim template.json
```

```
{
  "builders": [
    {
      "type": "qemu-chroot",
      "source_path": "ubuntu-16.04-server-cloudimg-amd64-disk1.img",
      "image_name": "ubuntu-16.04.img",
      "compression": true
    }
  ],
  "provisioners": [
    {
      "type": "shell",
      "inline": [
        "apt update"
      ]
    }
  ] 
}
```

Once you have the template, build it using Packer.

```
$ sudo packer build template.json
```

## Configuration Reference

### Required

- `source_image` (string) - A path to the base image file to use. This file must be in QCOW2 format.

### Optional

- `output_dir` (string) - This is the path to the directory where the resulting image file will be created. By default this is "output-BUILDNAME" where "BUILDNAME" is the name of the builder.
- `image_name` (string) - The name of the resulting image file.
- `compression` (boolean) - Apply compression to the QCOW2 disk file using `qemu-img` convert. Defaults to false.
- `device_path` (string) - The path to the device where the volume of the source image will be attached.
- `mount_path` (string) - The path where the volume will be mounted. This is where the chroot environment will be. This defaults to /mnt/packer-builder-qemu-chroot/{{.Device}}. This is a configuration template where the .Device variable is replaced with the name of the device where the volume is attached.
- `mount_partition` (integer) - The partition number containing the / partition. By default this is the first partition of the volume.
- `mount_options` (array of string) - Options to supply the mount command when mounting devices. Each option will be prefixed with `-o` and supplied to the mount command ran by this plugin.
- `chroot_mounts` (array of array of string) - This is a list of devices to mount into the chroot environment. This configuration parameter requires some additional documentation which is in the "Chroot Mounts" section below. Please read that section for more information on how to use this.
- `copy_files` (array of string) - Paths to files on the running EC2 instance that will be copied into the chroot environment prior to provisioning. Defaults to /etc/resolv.conf so that DNS lookups work. Pass an empty list to skip copying /etc/resolv.conf. You may need to do this if you're building an image that uses systemd.
- `command_wrapper` (string) - How to run shell commands. This defaults to {{.Command}}. This may be useful to set if you want to set environmental variables or perhaps run it with sudo or so on. This is a configuration template where the .Command variable is replaced with the command to be run. Defaults to "{{.Command}}".

### Chroot Mounts

The `chroot_mounts` configuration can be used to mount specific devices within the chroot. By default, the following additional mounts are added into the chroot by this plugin:

- `/proc` (proc)
- `/sys` (sysfs)
- `/dev` (bind to real `/dev`)
- `/dev/pts` (devpts)
- `/proc/sys/fs/binfmt_misc` (binfmt_misc)

These default mounts are usually good enough for anyone and are sane defaults. However, if you want to change or add the mount points, you may using the chroot_mounts configuration. Here is an example configuration which only mounts /prod and /dev:

```
{
  "chroot_mounts": [
    ["proc", "proc", "/proc"],
    ["bind", "/dev", "/dev"]
  ]
}
```

`chroot_mounts` is a list of string arrays with more than three elements. The meaning of each component is as follows in order:

- The filesystem type. If this is "bind", then Packer will properly bind the filesystem to another mount point.
- The source device.
- The mount directory.
- The mount option (This element can be specified multiple times).

## License

Mozilla Public License 2.0

Note that this plugin is implemented by forking [AMI Builder (chroot)](https://www.packer.io/docs/builders/amazon-chroot.html) of Packer.

