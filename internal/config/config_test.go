package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "config.yaml")
	if e := Init(p); e != nil {
		t.Fatal(e)
	}
	if e := Init(p); e == nil {
		t.Fatal("overwrote config")
	}
	c, e := Load(p)
	if e != nil || c.Timeout != 10*time.Second {
		t.Fatalf("%+v %v", c, e)
	}
	if e = os.WriteFile(p, []byte("timeout: 1ms\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = Load(p); e == nil {
		t.Fatal("accepted invalid timeout")
	}
	c = Default()
	c.Disabled = []string{"typo"}
	if c.Validate() == nil {
		t.Fatal("unknown collector")
	}
	c = Default()
	c.CPUPercent = 101
	if c.Validate() == nil {
		t.Fatal("bad threshold")
	}
}

func TestUnknownConfigField(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	if e := os.WriteFile(p, []byte("timeuot: 1s\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := Load(p); e == nil {
		t.Fatal("ignored misspelled setting")
	}
	if e := os.WriteFile(p, []byte("cpu_percent: .nan\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := Load(p); e == nil {
		t.Fatal("accepted NaN")
	}
}
