package fetch

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
)

// Executes a command in a specified working directory.
func Exec(workingDir string, cmd string, args ...string) (stdout []byte, err error) {
	if !filepath.IsAbs(workingDir) {
		if workingDir, err = filepath.Abs(workingDir); err != nil {
			return nil, err
		}
	}

	var stdoutBuffer bytes.Buffer
	execCmd := exec.Command(cmd, args...)
	execCmd.Dir = workingDir
	execCmd.Stdout = &stdoutBuffer
	// Stderr is left to the console.

	invocation := append([]string{execCmd.Path}, execCmd.Args...)
	err = execCmd.Run()
	if err != nil {
		return stdoutBuffer.Bytes(), fmt.Errorf("failed to run '%s: %s", invocation, err)
	}
	return stdoutBuffer.Bytes(), nil
}
