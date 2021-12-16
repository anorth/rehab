package fetch

import (
	"bytes"
	"fmt"
	"os/exec"
)

func Exec(workingDir string, cmd string, args ...string) (stdout []byte, err error) {
	//if !filepath.IsAbs(workingDir) {
	//	if workingDir, err = filepath.Abs(workingDir); err != nil {
	//		return nil, nil, err
	//	}
	//}

	var stdoutBuffer bytes.Buffer
	execCmd := exec.Command(cmd, args...)
	execCmd.Dir = workingDir
	execCmd.Stdout = &stdoutBuffer

	invocation := append([]string{execCmd.Path}, execCmd.Args...)
	//_, _ = fmt.Fprintf(os.Stderr, strings.Join(invocation, " "))
	//_, _ = fmt.Fprintf(os.Stderr, "\n")
	err = execCmd.Run()
	if err != nil {
		return stdoutBuffer.Bytes(), fmt.Errorf("failed to run '%s: %s", invocation, err)
	}
	return stdoutBuffer.Bytes(), nil
}
