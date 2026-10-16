//go:build !linux && !darwin && !windows

package workbenchterminal

import "errors"

func launchProcess(cwd string, rows, cols uint16) (process, error) {
	return nil, errors.New("workbench terminal unsupported on this platform")
}
