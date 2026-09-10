package config

import "testing"

func TestSendTransportAPIPercent(t *testing.T) {
	cases := map[string]int{"": 50, "abc": 50, "0": 0, "100": 100, "-5": 0, "250": 100, " 30 ": 30}
	for raw, want := range cases {
		t.Setenv("SEND_TRANSPORT_API_PERCENT", raw)
		if got := SendTransportAPIPercent(); got != want {
			t.Errorf("%q = %d, want %d", raw, got, want)
		}
	}
}

func TestWorkerDeployment(t *testing.T) {
	cases := map[string]string{"": "unknown", "cloud_run_worker": "cloud_run_worker", " Cloud-VM ": "cloud_vm", "!!": "unknown"}
	for raw, want := range cases {
		t.Setenv("WORKER_DEPLOYMENT", raw)
		if got := WorkerDeployment(); got != want {
			t.Errorf("%q = %q, want %q", raw, got, want)
		}
	}
}
