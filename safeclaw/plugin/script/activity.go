package script

import (
	"context"

	"github.com/bitfield/script"
)

// ScriptExecInput contains parameters for executing a shell command.
type ScriptExecInput struct {
	Command string
}

// ScriptFileInput contains parameters for reading a file.
type ScriptFileInput struct {
	Path string
}

// ScriptOutput contains the result of a script operation.
type ScriptOutput struct {
	Data  []byte
	Error string
}

// ScriptExecActivity executes a shell command as a Temporal activity.
func ScriptExecActivity(ctx context.Context, input ScriptExecInput) (*ScriptOutput, error) {
	pipe := script.Exec(input.Command)
	data, err := pipe.Bytes()
	if err != nil {
		return &ScriptOutput{
			Data:  data,
			Error: err.Error(),
		}, nil
	}
	return &ScriptOutput{Data: data}, nil
}

// ScriptFileActivity reads a file as a Temporal activity.
func ScriptFileActivity(ctx context.Context, input ScriptFileInput) (*ScriptOutput, error) {
	pipe := script.File(input.Path)
	data, err := pipe.Bytes()
	if err != nil {
		return &ScriptOutput{
			Data:  data,
			Error: err.Error(),
		}, nil
	}
	return &ScriptOutput{Data: data}, nil
}

// PipeExecInput contains parameters for chaining exec on a pipe.
type PipeExecInput struct {
	PipeData []byte
	Command  string
}

// PipeExecActivity chains an exec command on pipe data.
func PipeExecActivity(ctx context.Context, input PipeExecInput) (*ScriptOutput, error) {
	pipe := script.Echo(string(input.PipeData)).Exec(input.Command)
	data, err := pipe.Bytes()
	if err != nil {
		return &ScriptOutput{
			Data:  data,
			Error: err.Error(),
		}, nil
	}
	return &ScriptOutput{Data: data}, nil
}
