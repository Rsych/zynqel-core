package policy

import (
	"fmt"
	"os"
	"strconv"
)

const (
	maxMemoryMB       = 2048  // 2 GB — hard ceiling for coding agent sessions
	maxCPUQuota       = 200   // 2 cores — hard ceiling
	maxIdleTimeoutSec = 86400 // 24 hours
	maxMaxSessions    = 1000  // hard ceiling for concurrent sessions
)

// ResourcePolicy defines default resource limits applied to every session container.
type ResourcePolicy struct {
	MemoryMB       int // memory limit in megabytes (default: 512)
	CPUQuota       int // CPU percentage of one core, e.g. 100 = 1 core (default: 100)
	IdleTimeoutSec int // idle timeout in seconds (default: 900 = 15 min, 0 = disabled)
	HardTimeoutSec int // hard timeout in seconds (default: 1800 = 30 min, 0 = disabled)
	MaxSessions    int // max concurrent sessions (default: 10, 0 = unlimited)
}

// DefaultPolicy returns sensible defaults for resource limits.
func DefaultPolicy() ResourcePolicy {
	return ResourcePolicy{
		MemoryMB:       512,
		CPUQuota:       100,
		IdleTimeoutSec: 900,
		HardTimeoutSec: 1800,
		MaxSessions:    10,
	}
}

// PolicyFromEnv reads resource policy from environment variables,
// falling back to defaults for any unset or invalid values.
//
//	ZYNQEL_SESSION_MEMORY_MB — memory limit in MB (default: 512)
//	ZYNQEL_SESSION_CPU_QUOTA — CPU quota as percentage (default: 100)
//	ZYNQEL_IDLE_TIMEOUT      — idle timeout in seconds (default: 900, 0 = disabled)
//	ZYNQEL_HARD_TIMEOUT      — hard timeout in seconds (default: 1800, 0 = disabled)
//	ZYNQEL_MAX_SESSIONS      — max concurrent sessions (default: 10, 0 = unlimited)
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
		if n > maxMemoryMB {
			return p, fmt.Errorf("ZYNQEL_SESSION_MEMORY_MB=%d exceeds max %d", n, maxMemoryMB)
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
		if n > maxCPUQuota {
			return p, fmt.Errorf("ZYNQEL_SESSION_CPU_QUOTA=%d exceeds max %d", n, maxCPUQuota)
		}
		p.CPUQuota = n
	}

	if v := os.Getenv("ZYNQEL_IDLE_TIMEOUT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return p, fmt.Errorf("invalid ZYNQEL_IDLE_TIMEOUT=%q: %w", v, err)
		}
		if n < 0 {
			return p, fmt.Errorf("ZYNQEL_IDLE_TIMEOUT must be non-negative, got %d", n)
		}
		if n > maxIdleTimeoutSec {
			return p, fmt.Errorf("ZYNQEL_IDLE_TIMEOUT=%d exceeds max %d", n, maxIdleTimeoutSec)
		}
		p.IdleTimeoutSec = n
	}

	if v := os.Getenv("ZYNQEL_HARD_TIMEOUT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return p, fmt.Errorf("invalid ZYNQEL_HARD_TIMEOUT=%q: %w", v, err)
		}
		if n < 0 {
			return p, fmt.Errorf("ZYNQEL_HARD_TIMEOUT must be non-negative, got %d", n)
		}
		if n > maxIdleTimeoutSec {
			return p, fmt.Errorf("ZYNQEL_HARD_TIMEOUT=%d exceeds max %d", n, maxIdleTimeoutSec)
		}
		p.HardTimeoutSec = n
	}

	if v := os.Getenv("ZYNQEL_MAX_SESSIONS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return p, fmt.Errorf("invalid ZYNQEL_MAX_SESSIONS=%q: %w", v, err)
		}
		if n < 0 {
			return p, fmt.Errorf("ZYNQEL_MAX_SESSIONS must be non-negative, got %d", n)
		}
		if n > maxMaxSessions {
			return p, fmt.Errorf("ZYNQEL_MAX_SESSIONS=%d exceeds max %d", n, maxMaxSessions)
		}
		p.MaxSessions = n
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
