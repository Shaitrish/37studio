package remote

import "testing"

// Pasted URLs can't point the server at its own network: loopback, the Docker bridge, cloud metadata.
func TestPublic(t *testing.T) {
	for _, h := range []string{"localhost", "127.0.0.1", "::1", "10.0.0.5", "172.17.0.1", "192.168.1.1", "169.254.169.254", "0.0.0.0", "fd00::1"} {
		if Public(h) == nil {
			t.Errorf("%s allowed", h)
		}
	}
	if err := Public("8.8.8.8"); err != nil {
		t.Errorf("8.8.8.8: %v", err)
	}
}
