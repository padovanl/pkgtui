package pkg

import (
	"reflect"
	"testing"
)

func TestMaybeSudo(t *testing.T) {
	restore := geteuid
	defer func() { geteuid = restore }()

	geteuid = func() int { return 1000 } // regular user
	if got := MaybeSudo([]string{"apt-get", "install", "-y", "curl"}); !reflect.DeepEqual(got, []string{"sudo", "apt-get", "install", "-y", "curl"}) {
		t.Errorf("MaybeSudo() as non-root = %v, want sudo prefix", got)
	}

	geteuid = func() int { return 0 } // root, e.g. inside a container
	if got := MaybeSudo([]string{"apt-get", "install", "-y", "curl"}); !reflect.DeepEqual(got, []string{"apt-get", "install", "-y", "curl"}) {
		t.Errorf("MaybeSudo() as root = %v, want no sudo prefix", got)
	}
}
