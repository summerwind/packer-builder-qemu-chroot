//go:generate packer-sdc mapstructure-to-hcl2 -type Config
// see more at https://www.packer.io/guides/hcl/component-object-spec

package chroot

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/chroot"
	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
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
	Compression    bool       `mapstructure:"compression"`
	DevicePath     string     `mapstructure:"device_path"`
	MountPath      string     `mapstructure:"mount_path"`
	MountPartition int        `mapstructure:"mount_partition"`
	MountOptions   []string   `mapstructure:"mount_options"`
	ChrootMounts   [][]string `mapstructure:"chroot_mounts"`
	CopyFiles      []string   `mapstructure:"copy_files"`
	CommandWrapper string     `mapstructure:"command_wrapper"`

	ctx interpolate.Context
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

	state := new(multistep.BasicStateBag)
	state.Put("config", &b.config)
	state.Put("hook", hook)
	state.Put("ui", ui)
	state.Put("command_wrapper", NewCommandWrapper(b.config))

	steps := []multistep.Step{
		&StepPrepareOutputDir{},
		&StepPrepareImage{},
		&StepPrepareDevice{},
		&StepConnectImage{},
		&StepMountDevice{},
		&chroot.StepMountExtra{},
		&chroot.StepCopyFiles{},
		&chroot.StepChrootProvision{},
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
