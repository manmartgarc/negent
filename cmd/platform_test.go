package cmd

import "testing"

func TestCheckPlatformSupportRejectsWindows(t *testing.T) {
	prev := currentGOOS
	currentGOOS = "windows"
	t.Cleanup(func() {
		currentGOOS = prev
	})

	err := checkPlatformSupport(nil, nil)
	if err == nil {
		t.Fatal("expected Windows platform check to fail")
	}
	if got := err.Error(); got != "windows is unsupported; negent supports Linux and macOS only" {
		t.Fatalf("unexpected error %q", got)
	}
}

func TestCheckPlatformSupportAllowsSupportedPlatforms(t *testing.T) {
	for _, goos := range []string{"linux", "darwin"} {
		t.Run(goos, func(t *testing.T) {
			prev := currentGOOS
			currentGOOS = goos
			t.Cleanup(func() {
				currentGOOS = prev
			})

			if err := checkPlatformSupport(nil, nil); err != nil {
				t.Fatalf("unexpected error for %s: %v", goos, err)
			}
		})
	}
}
