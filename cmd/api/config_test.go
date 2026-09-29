package main

import "testing"

func TestMissingConfig(t *testing.T) {
	t.Setenv("MYSQL_PASSWORD", "")
	if _, e := loadConfig(); e == nil {
		t.Fatal("missing environment accepted")
	}
}
