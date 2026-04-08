package main

import (
	"flag"
	"testing"
)

var runConfigFlow bool

func TestMain(m *testing.M) {
	flag.BoolVar(&runConfigFlow, "run-config-flow", false, "run config-driven integration flow")
	flag.Parse()
	m.Run()
}

func TestRunSTDTestFromIniConfigOptional(t *testing.T) {
	t.Log("TestRunSTDTestFromIniConfigOptional started")

	if !runConfigFlow {
		t.Log("run-config-flow=false, skip integration execution")
		t.Skip("skip integration flow; pass -run-config-flow to enable")
	}

	t.Log("run-config-flow=true, calling RunSTDTestFromIniConfig")
	if err := RunSTDTestFromIniConfig(); err != nil {
		t.Fatalf("RunSTDTestFromIniConfig() failed: %v", err)
	}
	t.Log("RunSTDTestFromIniConfig finished")
}
