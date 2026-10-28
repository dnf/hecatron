package a

import "testing"

func TestSubs(t *testing.T) {
	t.Log("parent says")
	t.Run("ok", func(t *testing.T) { t.Log("quiet") })
	t.Run("bad", func(t *testing.T) { t.Errorf("bad %d", 1) })
	t.Run("nested", func(t *testing.T) {
		t.Run("deep", func(t *testing.T) { t.Fatal("deep fail") })
	})
}

func TestPass(t *testing.T) { t.Log("hidden") }

func TestSkip(t *testing.T) { t.Skip("not today") }

func TestFail(t *testing.T) { t.Error("plain fail") }
