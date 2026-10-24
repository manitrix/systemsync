package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/manitrix/systemsync/internal/ai"
	"github.com/manitrix/systemsync/internal/collectors/containers"
	"github.com/manitrix/systemsync/internal/collectors/network"
	"github.com/manitrix/systemsync/internal/collectors/services"
	"github.com/manitrix/systemsync/internal/config"
	"github.com/manitrix/systemsync/internal/diagnostics"
	"github.com/manitrix/systemsync/internal/remediation"
	"github.com/manitrix/systemsync/internal/report"
	"github.com/manitrix/systemsync/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type Build struct{ Version, Commit, Date string }
type app struct {
	cfg          config.Config
	path, format string
	verbose      bool
	build        Build
	logger       *slog.Logger
}

func New(build Build) *cobra.Command {
	a := &app{build: build}
	a.path, _ = config.Path()
	root := &cobra.Command{Use: "ssync", Short: "SystemSync: evidence-based Linux diagnostics and safe automation", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	root.RunE = func(c *cobra.Command, _ []string) error { return c.Help() }
	root.PersistentFlags().StringVar(&a.path, "config", a.path, "YAML configuration path")
	root.PersistentFlags().StringVar(&a.format, "format", "", "Output format: text or json (overrides config)")
	root.PersistentFlags().BoolVar(&a.verbose, "verbose", false, "Enable structured debug logs on stderr")
	root.PersistentPreRunE = func(c *cobra.Command, _ []string) error {
		var e error
		a.cfg, e = config.Load(a.path)
		if e != nil {
			return e
		}
		if a.format != "" {
			a.cfg.Format = a.format
		}
		if e = a.cfg.Validate(); e != nil {
			return e
		}
		level := slog.LevelWarn
		if a.verbose {
			level = slog.LevelDebug
		}
		a.logger = slog.New(slog.NewTextHandler(c.ErrOrStderr(), &slog.HandlerOptions{Level: level}))
		return nil
	}
	root.AddCommand(&cobra.Command{Use: "version", Short: "Print build and Go runtime information", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		_, e := fmt.Fprintf(c.OutOrStdout(), "ssync %s commit=%s built=%s go=%s platform=%s/%s\n", build.Version, build.Commit, build.Date, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return e
	}})
	root.AddCommand(&cobra.Command{Use: "doctor", Short: "Check platform, tools and local runtime availability", Args: cobra.NoArgs, RunE: a.doctor})
	for _, name := range []string{"status", "diagnose", "report", "explain"} {
		name := name
		var pid int
		var service, input string
		var useAI bool
		cmd := &cobra.Command{Use: name, Short: map[string]string{"status": "Collect a current system health snapshot", "diagnose": "Correlate Linux observations using deterministic diagnostic rules", "report": "Generate a timestamped diagnostic report", "explain": "Explain a collected report; AI is opt-in"}[name], Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
			if pid < 0 {
				return fmt.Errorf("pid must be positive")
			}
			if c.Flags().Changed("pid") && pid == 0 {
				return fmt.Errorf("pid must be positive")
			}
			if service != "" {
				if e := services.ValidateName(service); e != nil {
					return e
				}
			}
			var r types.Report
			if input != "" {
				f, e := os.Open(input)
				if e != nil {
					return e
				}
				defer f.Close()
				if e = json.NewDecoder(io.LimitReader(f, 8*1024*1024)).Decode(&r); e != nil {
					return e
				}
				if r.SchemaVersion != "1" {
					return fmt.Errorf("unsupported report schema")
				}
			} else {
				r = diagnostics.Run(c.Context(), a.cfg, diagnostics.Tasks(a.cfg, pid, service))
			}
			if name == "explain" {
				r.Explanation = offlineExplanation(r)
			}
			if useAI {
				ctx, cancel := context.WithTimeout(c.Context(), a.cfg.Timeout)
				defer cancel()
				text, e := ai.New(a.cfg.AI).Explain(ctx, r)
				if e != nil {
					r.Explanation = "AI unavailable; core diagnostics are preserved: " + e.Error()
				} else {
					r.Explanation = text
				}
			}
			return a.emit(c, r)
		}}
		cmd.Flags().IntVar(&pid, "pid", 0, "Focus process collection on this PID")
		cmd.Flags().StringVar(&service, "service", "", "Inspect an exact systemd unit and its journal")
		cmd.Flags().BoolVar(&useAI, "ai", false, "Request a sanitized explanation from the configured AI provider")
		if name == "explain" {
			cmd.Flags().StringVar(&input, "input", "", "Explain an existing JSON report instead of collecting")
		}
		root.AddCommand(cmd)
	}
	root.AddCommand(a.network(), a.services(), a.containers(), a.configuration(), a.remediate())
	var service string
	logs := &cobra.Command{Use: "logs", Short: "Read recent systemd journal messages for a service", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if service == "" {
			return fmt.Errorf("--service is required")
		}
		return a.single(c, "journal", func(ctx context.Context) (any, error) { return services.New().Logs(ctx, service, a.cfg.LogLimit) })
	}}
	logs.Flags().StringVar(&service, "service", "", "Exact systemd unit name")
	root.AddCommand(logs)
	completion := &cobra.Command{Use: "completion", Short: "Generate shell completion scripts", Args: cobra.NoArgs}
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		shell := shell
		completion.AddCommand(&cobra.Command{Use: shell, Short: "Generate " + shell + " completion", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
			switch shell {
			case "bash":
				return root.GenBashCompletion(c.OutOrStdout())
			case "zsh":
				return root.GenZshCompletion(c.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(c.OutOrStdout(), true)
			default:
				return root.GenPowerShellCompletion(c.OutOrStdout())
			}
		}})
	}
	root.AddCommand(completion)
	return root
}
func offlineExplanation(r types.Report) string {
	if len(r.Findings) == 0 {
		return "No deterministic rule triggered. Check collection errors and repeat measurements before concluding the system is healthy."
	}
	var b strings.Builder
	for _, f := range r.Findings {
		fmt.Fprintf(&b, "%s [%s]: %s\n", f.Title, f.Confidence, f.Explanation)
	}
	return b.String()
}
func (a *app) emit(c *cobra.Command, r types.Report) error {
	diagnostics.DebugReport(a.logger, r)
	return report.Write(c.OutOrStdout(), r, a.cfg.Format)
}
func (a *app) single(c *cobra.Command, name string, f func(context.Context) (any, error)) error {
	r := diagnostics.Observe(c.Context(), a.cfg, name, f)
	if e := a.emit(c, r); e != nil {
		return e
	}
	if r.Observations[0].Error != "" {
		return fmt.Errorf("%s: %s", name, r.Observations[0].Error)
	}
	return nil
}
func (a *app) doctor(c *cobra.Command, _ []string) error {
	return a.single(c, "doctor", func(ctx context.Context) (any, error) {
		m := map[string]any{"platform": runtime.GOOS, "linux_supported": runtime.GOOS == "linux", "core_requires_ai": false, "core_requires_root": false}
		for _, name := range []string{"systemctl", "journalctl", "nft", "iptables"} {
			p, e := exec.LookPath(name)
			if e != nil {
				m[name] = "unavailable"
			} else {
				m[name] = p
			}
		}
		for _, path := range []string{"/proc/stat", "/proc/meminfo", "/proc/net/tcp", "/run/systemd/system"} {
			_, e := os.Stat(path)
			if e != nil {
				m[path] = e.Error()
			} else {
				m[path] = "accessible"
			}
		}
		for _, r := range containers.Available(a.cfg.LogLimit) {
			_, e := r.List(ctx)
			if e != nil {
				m[r.Name()] = e.Error()
			} else {
				m[r.Name()] = "API accessible"
			}
		}
		return m, nil
	})
}
func (a *app) network() *cobra.Command {
	root := &cobra.Command{Use: "network", Short: "Inspect Linux networking in the current network namespace", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		return a.single(c, "network", func(ctx context.Context) (any, error) { return network.Collect(ctx) })
	}}
	for _, name := range []string{"ports", "routes"} {
		name := name
		root.AddCommand(&cobra.Command{Use: name, Short: "Inspect current " + name, Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
			return a.single(c, "network."+name, func(ctx context.Context) (any, error) {
				n, e := network.Collect(ctx)
				if e != nil {
					return nil, e
				}
				if name == "ports" {
					return struct {
						Sockets  []types.Socket `json:"sockets"`
						Warnings []string       `json:"warnings"`
					}{n.Sockets, n.Warnings}, nil
				}
				return struct {
					Routes   []types.Route `json:"routes"`
					Warnings []string      `json:"warnings"`
				}{n.Routes, n.Warnings}, nil
			})
		}})
	}
	root.AddCommand(&cobra.Command{Use: "firewall", Short: "Read nftables JSON rules and detect iptables compatibility", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		return a.single(c, "network.firewall", func(ctx context.Context) (any, error) { return network.Firewall(ctx) })
	}})
	var domain string
	dns := &cobra.Command{Use: "dns", Short: "Show resolver configuration, or resolve one domain with --domain", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if domain == "" {
			return a.single(c, "network.dns", func(ctx context.Context) (any, error) {
				n, e := network.Collect(ctx)
				return struct {
					Resolver string   `json:"resolver"`
					Warnings []string `json:"warnings"`
				}{n.Resolver, n.Warnings}, e
			})
		}
		return a.single(c, "network.dns", func(ctx context.Context) (any, error) { return network.Check(ctx, domain, 0, a.cfg.Timeout), nil })
	}}
	dns.Flags().StringVar(&domain, "domain", "", "Single domain to resolve (no query by default)")
	root.AddCommand(dns)
	var host string
	var port int
	var timeout time.Duration
	check := &cobra.Command{Use: "check", Short: "Perform one bounded DNS and TCP connectivity check", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if host == "" || port < 1 || port > 65535 {
			return fmt.Errorf("--host and --port (1..65535) are required")
		}
		if timeout < 100*time.Millisecond || timeout > time.Minute {
			return fmt.Errorf("timeout must be 100ms..1m")
		}
		return a.single(c, "network.check", func(ctx context.Context) (any, error) { return network.Check(ctx, host, port, timeout), nil })
	}}
	check.Flags().StringVar(&host, "host", "", "Single destination hostname or IP")
	check.Flags().IntVar(&port, "port", 0, "TCP destination port")
	check.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "Total DNS and TCP deadline")
	root.AddCommand(check)
	return root
}
func (a *app) services() *cobra.Command {
	root := &cobra.Command{Use: "services", Short: "List systemd service states", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		return a.single(c, "services", func(ctx context.Context) (any, error) { return services.New().List(ctx) })
	}}
	root.AddCommand(&cobra.Command{Use: "inspect <service>", Short: "Read systemd unit properties without changing state", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		return a.single(c, "services", func(ctx context.Context) (any, error) { return services.New().Inspect(ctx, args[0]) })
	}})
	return root
}
func (a *app) containers() *cobra.Command {
	var selected string
	root := &cobra.Command{Use: "containers", Short: "List containers from local Docker and Podman APIs", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		tasks := []diagnostics.Task{}
		for _, r := range containers.Available(a.cfg.LogLimit) {
			r := r
			if selected != "auto" && r.Name() != selected {
				continue
			}
			tasks = append(tasks, diagnostics.Task{Name: "containers." + r.Name(), Collect: func(ctx context.Context) (any, error) { return r.List(ctx) }})
		}
		if len(tasks) == 0 {
			return fmt.Errorf("runtime must be auto, docker or podman")
		}
		r := diagnostics.Run(c.Context(), a.cfg, tasks)
		return a.emit(c, r)
	}}
	root.PersistentFlags().StringVar(&selected, "runtime", "auto", "Runtime: auto, docker or podman")
	for _, name := range []string{"inspect", "diagnose"} {
		name := name
		root.AddCommand(&cobra.Command{Use: name + " <container>", Short: "Inspect container state, logs, limits and diagnostic findings", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
			if selected != "auto" && selected != "docker" && selected != "podman" {
				return fmt.Errorf("invalid runtime")
			}
			return a.single(c, "containers."+name, func(ctx context.Context) (any, error) {
				var errors []string
				for _, r := range containers.Available(a.cfg.LogLimit) {
					if selected != "auto" && r.Name() != selected {
						continue
					}
					v, e := r.Inspect(ctx, args[0])
					if e == nil {
						return v, nil
					}
					errors = append(errors, e.Error())
				}
				return nil, fmt.Errorf("%s", strings.Join(errors, "; "))
			})
		}})
	}
	return root
}
func (a *app) configuration() *cobra.Command {
	root := &cobra.Command{Use: "config", Short: "Manage secure YAML configuration", Args: cobra.NoArgs}
	root.AddCommand(&cobra.Command{Use: "init", Short: "Create defaults without overwriting an existing file", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if e := config.Init(a.path); e != nil {
			return e
		}
		_, e := fmt.Fprintln(c.OutOrStdout(), a.path)
		return e
	}}, &cobra.Command{Use: "show", Short: "Print effective YAML configuration (API keys are read from environment)", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		b, e := yaml.Marshal(a.cfg)
		if e != nil {
			return e
		}
		_, e = c.OutOrStdout().Write(b)
		return e
	}})
	return root
}
func (a *app) remediate() *cobra.Command {
	var host string
	var approve bool
	cmd := &cobra.Command{Use: "remediate", Short: "Experimental: plan or approve a read-only DNS recheck; no state-changing actions", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		p, e := remediation.DNSPlan(host)
		if e != nil {
			return e
		}
		if !approve {
			return a.single(c, "remediation.plan", func(context.Context) (any, error) { return p, nil })
		}
		return a.single(c, "remediation.result", func(ctx context.Context) (any, error) { return remediation.Execute(ctx, p, true) })
	}}
	cmd.Flags().StringVar(&host, "dns-recheck", "", "Hostname for the allowlisted verification plan")
	cmd.Flags().BoolVar(&approve, "approve", false, "Approve the displayed operation and perform verification")
	return cmd
}
