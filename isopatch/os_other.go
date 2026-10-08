//go:build !windows

package main

import (
	"os/exec"
	"path/filepath"
)

const canBrowse = false

func browseISO() (string, error) { return "", errNoDialog }

func showInFolder(path string) { _ = exec.Command("xdg-open", filepath.Dir(path)).Start() }

func openBrowser(url string) { _ = exec.Command("xdg-open", url).Start() }
