package main

import (
	"strings"
	"testing"
)

func TestDessertFixtureCommandIsRegisteredAndRequiresOutput(t *testing.T) {
	root := newRootCmd()
	command, _, err := root.Find([]string{"dessert-fixture"})
	if err != nil || command == root || command.Name() != "dessert-fixture" {
		t.Fatalf("dessert-fixture command not registered: command=%v err=%v", command, err)
	}
	if command.Flags().Lookup("channel") == nil || command.Flags().Lookup("out") == nil {
		t.Fatalf("dessert-fixture flags are incomplete")
	}
	root.SetArgs([]string{"dessert-fixture", t.TempDir()})
	err = root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--out is required") {
		t.Fatalf("missing --out err=%v", err)
	}
}
