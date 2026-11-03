//go:build !windows

package main

// attachConsole has nothing to do outside Windows: a program there prints to the terminal that started it.
func attachConsole() {}
