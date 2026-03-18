package policy

import (
	"fmt"
	"os"
	"strconv"
)

// ResourcePolicy defines default resource limits applied to every session container.
type ResourcePolicy struct {
	MemoryMB int // memory limit in megabytes (default: 512)
	CPUQuota int // CPU percentage of one core, e.g. 100 = 1 core (default: 100)
}

// DefaultPolicy returns sensible defaults for resource limits.
func DefaultPolicy() ResourcePolicy {
	return ResourcePolicy{
		MemoryMB: 512,
		CPUQuota: 100,
	}
}

// PolicyFromEnv reads resource policy from environment variables,
// falling back to defaults for any unset or invalid values.
//
//	ZYNQEL_SESSION_MEMORY_MB — memory limit in MB (default: 512)
//	ZYNQEL_SESSION_CPU_QUOTA — CPU quota as percentage (default: 100)
func PolicyFromEnv() (ResourcePolicy, error) {
	p := DefaultPolicy()

	if v := os.Getenv("ZYNQEL_SESSION_MEMORY_MB"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return p, fmt.Errorf("invalid ZYNQEL_SESSION_MEMORY_MB=%q: %w", v, err)
		}
		if n <= 0 {
			return p, fmt.Errorf("ZYNQEL_SESSION_MEMORY_MB must be positive, got %d", n)
		}
		p.MemoryMB = n
	}

	if v := os.Getenv("ZYNQEL_SESSION_CPU_QUOTA"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return p, fmt.Errorf("invalid ZYNQEL_SESSION_CPU_QUOTA=%q: %w", v, err)
		}
		if n <= 0 {
			return p, fmt.Errorf("ZYNQEL_SESSION_CPU_QUOTA must be positive, got %d", n)
		}
		p.CPUQuota = n
	}

	return p, nil
}

// MemoryBytes returns the memory limit in bytes.
func (p ResourcePolicy) MemoryBytes() int64 {
	return int64(p.MemoryMB) * 1024 * 1024
}

// NanoCPUs returns the CPU quota in Docker's NanoCPU format.
// 100 (1 core) → 1e9, 50 (half core) → 5e8, 200 (2 cores) → 2e9.
func (p ResourcePolicy) NanoCPUs() int64 {
	return int64(p.CPUQuota) * 1e7
}
