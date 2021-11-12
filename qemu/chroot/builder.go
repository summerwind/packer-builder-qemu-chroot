//go:generate packer-sdc mapstructure-to-hcl2 -type Config
// see more at https://www.packer.io/guides/hcl/component-object-spec

package chroot

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"

  "golang.org/x/sys/unix"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/chroot"
	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/packerbuilderdata"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

const BuilderId = "summerwind.qemu-chroot"

// Builder represents a builder plugin for Packer.
type Builder struct {
	config Config
	runner multistep.Runner
}

// Config represents a configuration of builder.
type Config struct {
	common.PackerConfig `mapstructure:",squash"`

	SourceImage    string     `mapstructure:"source_image"`
	OutputDir      string     `mapstructure:"output_directory"`
	ImageName      string     `mapstructure:"image_name"`
	// This is a list of devices to mount into the chroot environment. This
	// configuration parameter requires some additional documentation which is
	// in the Chroot Mounts section. Please read that section for more
	// information on how to use this.
	ChrootMounts   [][]string `mapstructure:"chroot_mounts" required:"false"`
	// How to run shell commands. This defaults to `{{.Command}}`. This may be
	// useful to set if you want to set environmental variables or perhaps run
	// it with sudo or so on. This is a configuration template where the
	// .Command variable is replaced with the command to be run. Defaults to
	// `{{.Command}}`.
	CommandWrapper string     `mapstructure:"command_wrapper" required:"false"`
	// Paths to files on the host instance that will be copied into the
	// chroot environment prior to provisioning. Defaults to /etc/resolv.conf
	// so that DNS lookups work. Pass an empty list to skip copying
	// /etc/resolv.conf. You may need to do this if you're building an image
	// that uses systemd.
	CopyFiles      []string   `mapstructure:"copy_files" required:"false"`
  Compression    bool       `mapstructure:"compression"`
	// The path to the device where the root volume of the source AMI will be
	// attached. This defaults to "" (empty string), which forces Packer to
	// find an open device automatically.
	DevicePath     string     `mapstructure:"device_path" required:"false"`
	// Options to supply the mount command when mounting devices. Each option
	// will be prefixed with -o and supplied to the mount command ran by
	// Packer. Because this command is ran in a shell, user discretion is
	// advised. See this manual page for the mount command for valid file
	// system specific options.
	MountOptions   []string   `mapstructure:"mount_options" required:"false"`
	// The partition number containing the / partition. By default this is the
	// first partition of the volume, (for example, xvda1) but you can
	// designate the entire block device by setting "mount_partition": "0" in
	// your config, which will mount xvda instead.
	MountPartition int        `mapstructure:"mount_partition" required:"false"`
	// The path where the volume will be mounted. This is where the chroot
	// environment will be. This defaults to
	// `/mnt/packer-amazon-chroot-volumes/{{.Device}}`. This is a configuration
	// template where the .Device variable is replaced with the name of the
	// device where the volume is attached.
	MountPath      string     `mapstructure:"mount_path" required:"false"`
	ctx interpolate.Context
}

type wrappedCommandTemplate struct {
	Command string
}

func (c *Config) GetContext() interpolate.Context {
	return c.ctx
}

// Cleaner is an interface with a function for cleanup.
type Cleaner interface {
	CleanupFunc(multistep.StateBag) error
}

func (b *Builder) ConfigSpec() hcldec.ObjectSpec { return b.config.FlatMapstructure().HCL2Spec() }

// Prepare validates given configuration.
func (b *Builder) Prepare(raws ...interface{}) ([]string, []string, error) {
	err := config.Decode(&b.config, &config.DecodeOpts{
		Interpolate:        true,
		InterpolateContext: &b.config.ctx,
	}, raws...)
	if err != nil {
		return nil, nil, err
	}

	if b.config.OutputDir == "" {
		b.config.OutputDir = fmt.Sprintf("output-%s", b.config.PackerBuildName)
	}

	if b.config.ImageName == "" {
		b.config.ImageName = fmt.Sprintf("packer-%s", b.config.PackerBuildName)
	}

	if b.config.MountPath == "" {
		b.config.MountPath = "/mnt/packer-builder-qemu-chroot/{{.Device}}"
	}

	if b.config.MountPartition == 0 {
		b.config.MountPartition = 1
	}

	if b.config.ChrootMounts == nil {
		b.config.ChrootMounts = make([][]string, 0)
	}

	if len(b.config.ChrootMounts) == 0 {
		b.config.ChrootMounts = [][]string{
			{"proc", "proc", "/proc"},
			{"sysfs", "sysfs", "/sys"},
			{"bind", "/dev", "/dev"},
			{"devpts", "devpts", "/dev/pts"},
			{"binfmt_misc", "binfmt_misc", "/proc/sys/fs/binfmt_misc"},
		}
	}

	if b.config.CopyFiles == nil {
		b.config.CopyFiles = []string{"/etc/resolv.conf"}
	}

	if b.config.CommandWrapper == "" {
		b.config.CommandWrapper = "{{.Command}}"
	}

	if b.config.MountPath == "" {
		b.config.MountPath = "/mnt/packer-builder-qemu-chroot/{{.Device}}"
	}

	// Accumulate any errors or warnings
	var errs *packersdk.MultiError
	var warns []string

	if b.config.SourceImage == "" {
		errs = packersdk.MultiErrorAppend(errs, errors.New("source_image is required."))
	}

	for _, mounts := range b.config.ChrootMounts {
		if len(mounts) != 3 {
			errs = packersdk.MultiErrorAppend(
				errs, errors.New("Each chroot_mounts entry should be three elements."))
			break
		}
	}

	if errs != nil && len(errs.Errors) > 0 {
		return nil, warns, errs
	}

	generatedData := []string{
		"ImageName",
		"OutputDir",
		"MountPath",
	}

	return generatedData, warns, nil
}

// Run runs each step of the plugin in order.
func (b *Builder) Run(ctx context.Context, ui packersdk.Ui, hook packersdk.Hook) (packersdk.Artifact, error) {
	if runtime.GOOS != "linux" {
		return nil, errors.New("The packer-qemu-chroot builder only works on Linux environments.")
	}

	_, err := exec.LookPath("qemu-nbd")
	if err != nil {
		return nil, errors.New("qemu-nbd command not found.")
	}

  // reset umask to default otherwise `cp --preserve` is required in some cases
  unix.Umask(0022)

  wrappedCommand := func(command string) (string, error) {
		ictx := b.config.ctx
		ictx.Data = &wrappedCommandTemplate{Command: command}
		return interpolate.Render(b.config.CommandWrapper, &ictx)
	}

	state := new(multistep.BasicStateBag)
	state.Put("config", &b.config)
	state.Put("hook", hook)
	state.Put("ui", ui)
	// state.Put("command_wrapper", NewCommandWrapper(b.config))
  state.Put("wrappedCommand", common.CommandWrapper(wrappedCommand))
	generatedData := &packerbuilderdata.GeneratedData{State: state}

	steps := []multistep.Step{
		&StepPrepareOutputDir{},
		&StepPrepareImage{},
		&StepPrepareDevice{
			GeneratedData: generatedData,
		},
		&StepConnectImage{},
		&StepMountDevice{
			MountOptions:   b.config.MountOptions,
			MountPartition: b.config.MountPartition,
			GeneratedData:  generatedData,
		},
		&chroot.StepMountExtra{
			ChrootMounts: b.config.ChrootMounts,
		},
		&chroot.StepCopyFiles{
			Files: b.config.CopyFiles,
		},
    // TODO: use Communicator & ChrootProvision from SDK/common (https://github.com/hashicorp/packer-plugin-sdk/issues/89)
		// &chroot.StepChrootProvision{},
		&StepChrootProvision{},
		&chroot.StepEarlyCleanup{},
		&StepCompressImage{},
	}

	b.runner = commonsteps.NewRunner(steps, b.config.PackerConfig, ui)
	b.runner.Run(ctx, state)

	if rawErr, ok := state.GetOk("error"); ok {
		return nil, rawErr.(error)
	}

	if _, ok := state.GetOk(multistep.StateCancelled); ok {
		return nil, errors.New("Build was cancelled.")
	}

	if _, ok := state.GetOk(multistep.StateHalted); ok {
		return nil, errors.New("Build was halted.")
	}

	artifact := &Artifact{
		dir: b.config.OutputDir,
		files: []string{
			state.Get("image_path").(string),
		},
	}

	return artifact, nil
}
