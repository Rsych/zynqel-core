package policy

import (
	"os"
	"testing"
)

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	if p.MemoryMB != 512 {
		t.Errorf("MemoryMB = %d, want 512", p.MemoryMB)
	}
	if p.CPUQuota != 100 {
		t.Errorf("CPUQuota = %d, want 100", p.CPUQuota)
	}
}

func TestMemoryBytes(t *testing.T) {
	p := ResourcePolicy{MemoryMB: 512}
	want := int64(512 * 1024 * 1024)
	if got := p.MemoryBytes(); got != want {
		t.Errorf("MemoryBytes() = %d, want %d", got, want)
	}
}

func TestNanoCPUs(t *testing.T) {
	tests := []struct {
		quota int
		want  int64
	}{
		{100, 1e9},  // 1 core
		{50, 5e8},   // half core
		{200, 2e9},  // 2 cores
	}
	for _, tt := range tests {
		p := ResourcePolicy{CPUQuota: tt.quota}
		if got := p.NanoCPUs(); got != tt.want {
			t.Errorf("NanoCPUs() with quota %d = %d, want %d", tt.quota, got, tt.want)
		}
	}
}

func TestPolicyFromEnv_Defaults(t *testing.T) {
	os.Unsetenv("ZYNQEL_SESSION_MEMORY_MB")
	os.Unsetenv("ZYNQEL_SESSION_CPU_QUOTA")

	p, err := PolicyFromEnv()
	if err != nil {
		t.Fatalf("PolicyFromEnv: %v", err)
	}
	if p.MemoryMB != 512 || p.CPUQuota != 100 {
		t.Errorf("got %+v, want defaults (512MB, 100%%)", p)
	}
}

func TestPolicyFromEnv_CustomValues(t *testing.T) {
	t.Setenv("ZYNQEL_SESSION_MEMORY_MB", "1024")
	t.Setenv("ZYNQEL_SESSION_CPU_QUOTA", "200")

	p, err := PolicyFromEnv()
	if err != nil {
		t.Fatalf("PolicyFromEnv: %v", err)
	}
	if p.MemoryMB != 1024 {
		t.Errorf("MemoryMB = %d, want 1024", p.MemoryMB)
	}
	if p.CPUQuota != 200 {
		t.Errorf("CPUQuota = %d, want 200", p.CPUQuota)
	}
}

func TestPolicyFromEnv_InvalidMemory(t *testing.T) {
	t.Setenv("ZYNQEL_SESSION_MEMORY_MB", "not-a-number")
	_, err := PolicyFromEnv()
	if err == nil {
		t.Fatal("expected error for invalid ZYNQEL_SESSION_MEMORY_MB")
	}
}

func TestPolicyFromEnv_NegativeMemory(t *testing.T) {
	t.Setenv("ZYNQEL_SESSION_MEMORY_MB", "-1")
	_, err := PolicyFromEnv()
	if err == nil {
		t.Fatal("expected error for negative ZYNQEL_SESSION_MEMORY_MB")
	}
}

func TestPolicyFromEnv_InvalidCPU(t *testing.T) {
	t.Setenv("ZYNQEL_SESSION_CPU_QUOTA", "abc")
	_, err := PolicyFromEnv()
	if err == nil {
		t.Fatal("expected error for invalid ZYNQEL_SESSION_CPU_QUOTA")
	}
}

func TestPolicyFromEnv_NegativeCPU(t *testing.T) {
	t.Setenv("ZYNQEL_SESSION_CPU_QUOTA", "0")
	_, err := PolicyFromEnv()
	if err == nil {
		t.Fatal("expected error for zero ZYNQEL_SESSION_CPU_QUOTA")
	}
}
