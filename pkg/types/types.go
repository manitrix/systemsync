// Package types defines the versioned public diagnostic report schema.
package types

import "time"

type Finding struct {
	ID          string    `json:"id"`
	Category    string    `json:"category"`
	Severity    string    `json:"severity"`
	Confidence  string    `json:"confidence"`
	Title       string    `json:"title"`
	Explanation string    `json:"explanation"`
	Evidence    []string  `json:"evidence"`
	Causes      []string  `json:"possible_causes"`
	Steps       []string  `json:"next_steps"`
	Timestamp   time.Time `json:"timestamp"`
}
type Observation struct {
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data,omitempty"`
	Error     string    `json:"error,omitempty"`
}
type Report struct {
	SchemaVersion string        `json:"schema_version"`
	Timestamp     time.Time     `json:"timestamp"`
	Platform      string        `json:"platform"`
	Observations  []Observation `json:"observations"`
	Findings      []Finding     `json:"findings"`
	Explanation   string        `json:"explanation,omitempty"`
}
type CPU struct{ Busy, Total uint64 }
type System struct {
	OS              string    `json:"os"`
	Kernel          string    `json:"kernel"`
	Uptime          float64   `json:"uptime_seconds"`
	CPUPercent      float64   `json:"cpu_percent"`
	CPUCount        int       `json:"cpu_count"`
	Load            []float64 `json:"load"`
	MemoryTotal     uint64    `json:"memory_total_bytes"`
	MemoryAvailable uint64    `json:"memory_available_bytes"`
	SwapTotal       uint64    `json:"swap_total_bytes"`
	SwapFree        uint64    `json:"swap_free_bytes"`
	Disks           []Disk    `json:"disks"`
	Warnings        []string  `json:"warnings,omitempty"`
}
type Disk struct {
	Mount      string `json:"mount"`
	Total      uint64 `json:"total_bytes"`
	Available  uint64 `json:"available_bytes"`
	Inodes     uint64 `json:"inodes"`
	FreeInodes uint64 `json:"free_inodes"`
}
type Process struct {
	PID        int      `json:"pid"`
	Name       string   `json:"name"`
	State      string   `json:"state"`
	RSS        uint64   `json:"rss_bytes"`
	Threads    int      `json:"threads"`
	FDs        int      `json:"fd_count"`
	CPUTicks   uint64   `json:"cpu_ticks"`
	CPUPercent *float64 `json:"cpu_percent,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}
type Socket struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	State    string `json:"state"`
	Inode    string `json:"inode"`
}
type Interface struct {
	Name      string   `json:"name"`
	State     string   `json:"state"`
	Addresses []string `json:"addresses"`
	MTU       int      `json:"mtu"`
}
type Network struct {
	Interfaces []Interface `json:"interfaces"`
	Sockets    []Socket    `json:"sockets"`
	Routes     []Route     `json:"routes"`
	Resolver   string      `json:"resolver"`
	Namespace  string      `json:"namespace"`
	Warnings   []string    `json:"warnings"`
}
type Route struct {
	Interface   string `json:"interface"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Flags       string `json:"flags"`
	Metric      int    `json:"metric"`
}
type Check struct {
	Host      string        `json:"host"`
	Port      int           `json:"port"`
	Addresses []string      `json:"addresses"`
	Outcome   string        `json:"outcome"`
	Detail    string        `json:"detail,omitempty"`
	Duration  time.Duration `json:"duration_ns"`
}
type Service struct {
	Name       string   `json:"name"`
	Load       string   `json:"load_state"`
	Active     string   `json:"active_state"`
	Sub        string   `json:"sub_state"`
	Result     string   `json:"result"`
	PID        int      `json:"pid"`
	Restarts   int      `json:"restarts"`
	ExecCode   int      `json:"exec_code"`
	ExecStatus int      `json:"exec_status"`
	Logs       []string `json:"logs"`
}
type Container struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Runtime     string   `json:"runtime"`
	Status      string   `json:"status"`
	Health      string   `json:"health"`
	ExitCode    int      `json:"exit_code"`
	OOMKilled   bool     `json:"oom_killed"`
	Restarts    int      `json:"restarts"`
	MemoryLimit uint64   `json:"memory_limit_bytes"`
	MemoryUsage uint64   `json:"memory_usage_bytes,omitempty"`
	CPUPercent  float64  `json:"cpu_percent,omitempty"`
	Ports       any      `json:"ports,omitempty"`
	Networks    any      `json:"networks,omitempty"`
	Mounts      any      `json:"mounts,omitempty"`
	Logs        string   `json:"logs,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}
