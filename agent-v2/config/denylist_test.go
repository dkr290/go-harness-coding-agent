package config

import (
	"reflect"
	"testing"
)

func TestBashDenylist(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("BASH_DENYLIST", "")
		want := []string{"rm", "sudo", "dd", "mkfs"}
		if got := getBashDenylist(); !reflect.DeepEqual(got, want) {
			t.Errorf("getBashDenylist() = %v, want %v", got, want)
		}
	})

	t.Run("env override replaces default", func(t *testing.T) {
		t.Setenv("BASH_DENYLIST", "curl, wget ,, shutdown")
		want := []string{"curl", "wget", "shutdown"}
		if got := getBashDenylist(); !reflect.DeepEqual(got, want) {
			t.Errorf("getBashDenylist() = %v, want %v", got, want)
		}
	})
}
