package server

import "github.com/Shaitrish/37studio/internal/proc"

// showInFolder opens the folder holding p in File Explorer, with p selected.
func showInFolder(p string) error {
	cmd := proc.Command("explorer")
	// Explorer parses /select,"path" itself and misreads the quoting Go gives an argument with spaces in it.
	cmd.SysProcAttr.CmdLine = `explorer /select,"` + p + `"`
	// Explorer exits with 1 even when the window opened, so only failing to start it is an error.
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
