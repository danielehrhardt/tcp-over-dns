package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/danielehrhardt/tcp-over-dns/internal/config"
	"github.com/danielehrhardt/tcp-over-dns/internal/platform"
	"github.com/danielehrhardt/tcp-over-dns/internal/proxy"
	"github.com/danielehrhardt/tcp-over-dns/internal/tunnel"
	"github.com/danielehrhardt/tcp-over-dns/internal/ui"
)

//go:embed static/*
var staticFiles embed.FS

// Server manages the web UI HTTP server.
type Server struct {
	mu          sync.Mutex
	port        int
	tunnel      *tunnel.Tunnel
	proxyMgr    *proxy.Proxy
	logs        []LogEntry
	maxLogs     int
	subscribers []chan LogEntry
	subMu       sync.Mutex
}

// LogEntry represents a log message for the UI.
type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// StatusResponse is the JSON response for /api/status.
type StatusResponse struct {
	Client ComponentStatus `json:"client"`
	Server ComponentStatus `json:"server"`
	Proxy  ComponentStatus `json:"proxy"`
	Config ConfigSummary   `json:"config"`
	System SystemInfo      `json:"system"`
}

// ComponentStatus represents the status of a component.
type ComponentStatus struct {
	Running bool   `json:"running"`
	Status  string `json:"status"`
	PID     int    `json:"pid,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// ConfigSummary is a safe subset of config to show in UI.
type ConfigSummary struct {
	Domain    string `json:"domain"`
	TunnelIP  string `json:"tunnel_ip"`
	ProxyAddr string `json:"proxy_addr"`
	HasConfig bool   `json:"has_config"`
}

// SystemInfo has platform details.
type SystemInfo struct {
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	IodineClient bool   `json:"iodine_client"`
	IodineServer bool   `json:"iodine_server"`
	SSH          bool   `json:"ssh"`
	Internet     bool   `json:"internet"`
}

// DiagnosticCheck represents a single diagnostic result.
type DiagnosticCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "pass", "fail", "warn", "info"
	Detail string `json:"detail"`
}

// NewServer creates a new web UI server.
func NewServer(port int) *Server {
	return &Server{
		port:     port,
		tunnel:   tunnel.NewTunnel(),
		proxyMgr: proxy.NewProxy(),
		maxLogs:  500,
	}
}

// Start begins serving the web UI.
func Start(port int, openBrowser bool) error {
	s := NewServer(port)

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/client/connect", s.handleClientConnect)
	mux.HandleFunc("/api/client/disconnect", s.handleClientDisconnect)
	mux.HandleFunc("/api/proxy/start", s.handleProxyStart)
	mux.HandleFunc("/api/proxy/stop", s.handleProxyStop)
	mux.HandleFunc("/api/diagnose", s.handleDiagnose)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/logs", s.handleLogs)

	// Static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("failed to load static files: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	url := fmt.Sprintf("http://%s", addr)
	ui.Success("Web UI running at %s", url)

	if openBrowser {
		go func() {
			time.Sleep(500 * time.Millisecond)
			openURL(url)
		}()
	}

	s.addLog("info", "tcpdns UI started on %s", addr)

	return http.Serve(listener, mux)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	cfg, _ := config.Load()

	clientRunning, clientPid, _ := tunnel.ClientStatus()
	serverRunning, serverPid, _ := tunnel.ServerStatus()
	proxyRunning, proxyPid, _ := proxy.ProxyStatus()

	resp := StatusResponse{
		Client: ComponentStatus{
			Running: clientRunning,
			Status:  boolStatus(clientRunning),
			PID:     clientPid,
		},
		Server: ComponentStatus{
			Running: serverRunning,
			Status:  boolStatus(serverRunning),
			PID:     serverPid,
		},
		Proxy: ComponentStatus{
			Running: proxyRunning,
			Status:  boolStatus(proxyRunning),
			PID:     proxyPid,
		},
		System: SystemInfo{
			OS:           runtime.GOOS,
			Arch:         runtime.GOARCH,
			IodineClient: platform.IodineInstalled(),
			IodineServer: platform.IodinedInstalled(),
			SSH:          platform.SSHInstalled(),
			Internet:     checkInternet(),
		},
	}

	if cfg != nil {
		resp.Config = ConfigSummary{
			Domain:    cfg.Client.ServerDomain,
			TunnelIP:  cfg.Server.TunnelIP,
			ProxyAddr: cfg.Proxy.Listen,
			HasConfig: config.Exists(),
		}
		if resp.Proxy.Running {
			resp.Proxy.Detail = cfg.Proxy.Listen
		}
	} else {
		resp.Config.HasConfig = false
	}

	writeJSON(w, resp)
}

func (s *Server) handleClientConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		writeError(w, "Failed to load config: %v", err)
		return
	}

	// Apply any overrides from request body
	var req struct {
		Domain   string `json:"domain"`
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Domain != "" {
		cfg.Client.ServerDomain = req.Domain
	}
	if req.Password != "" {
		cfg.Client.Password = req.Password
	}

	if cfg.Client.ServerDomain == "" || cfg.Client.ServerDomain == "t.example.com" {
		writeError(w, "No domain configured. Go to Settings first.")
		return
	}
	if cfg.Client.Password == "" {
		writeError(w, "No password configured. Go to Settings first.")
		return
	}

	s.addLog("info", "Connecting to %s...", cfg.Client.ServerDomain)

	go func() {
		if err := s.tunnel.Connect(cfg); err != nil {
			s.addLog("error", "Connection failed: %v", err)
		} else {
			s.addLog("info", "Connected successfully!")
		}
	}()

	writeJSON(w, map[string]string{"status": "connecting"})
}

func (s *Server) handleClientDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	s.addLog("info", "Disconnecting tunnel...")
	if err := s.tunnel.Disconnect(); err != nil {
		s.addLog("error", "Disconnect error: %v", err)
		writeError(w, "Disconnect failed: %v", err)
		return
	}

	s.addLog("info", "Tunnel disconnected")
	writeJSON(w, map[string]string{"status": "disconnected"})
}

func (s *Server) handleProxyStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		writeError(w, "Failed to load config: %v", err)
		return
	}

	s.addLog("info", "Starting SOCKS5 proxy on %s...", cfg.Proxy.Listen)

	go func() {
		if err := s.proxyMgr.Start(cfg); err != nil {
			s.addLog("error", "Proxy failed: %v", err)
		} else {
			s.addLog("info", "Proxy started on %s", cfg.Proxy.Listen)
		}
	}()

	writeJSON(w, map[string]string{"status": "starting"})
}

func (s *Server) handleProxyStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	s.addLog("info", "Stopping proxy...")
	if err := s.proxyMgr.Stop(); err != nil {
		writeError(w, "Stop failed: %v", err)
		return
	}

	s.addLog("info", "Proxy stopped")
	writeJSON(w, map[string]string{"status": "stopped"})
}

func (s *Server) handleDiagnose(w http.ResponseWriter, r *http.Request) {
	checks := []DiagnosticCheck{}

	// Platform
	info := platform.Detect()
	checks = append(checks, DiagnosticCheck{
		Name:   "Platform",
		Status: "info",
		Detail: fmt.Sprintf("%s/%s, pkg: %s", info.OS, info.Arch, info.PackageManager),
	})

	// iodine
	if platform.IodineInstalled() {
		checks = append(checks, DiagnosticCheck{Name: "iodine client", Status: "pass", Detail: "Installed"})
	} else {
		checks = append(checks, DiagnosticCheck{Name: "iodine client", Status: "fail", Detail: "Not installed. Run: brew install iodine"})
	}

	// Config
	if config.Exists() {
		checks = append(checks, DiagnosticCheck{Name: "Configuration", Status: "pass", Detail: "Config file found"})
	} else {
		checks = append(checks, DiagnosticCheck{Name: "Configuration", Status: "fail", Detail: "No config. Use Settings to configure."})
	}

	// Port 53
	inUse, proc := platform.CheckPort53()
	if inUse {
		checks = append(checks, DiagnosticCheck{Name: "Port 53", Status: "warn", Detail: "In use: " + truncateStr(proc, 60)})
	} else {
		checks = append(checks, DiagnosticCheck{Name: "Port 53", Status: "pass", Detail: "Available"})
	}

	// Tunnel
	clientRunning, _, _ := tunnel.ClientStatus()
	if clientRunning {
		checks = append(checks, DiagnosticCheck{Name: "Tunnel client", Status: "pass", Detail: "Running"})
	} else {
		checks = append(checks, DiagnosticCheck{Name: "Tunnel client", Status: "info", Detail: "Not connected"})
	}

	// Proxy
	proxyRunning, _, _ := proxy.ProxyStatus()
	if proxyRunning {
		checks = append(checks, DiagnosticCheck{Name: "SOCKS5 proxy", Status: "pass", Detail: "Running"})
	} else {
		checks = append(checks, DiagnosticCheck{Name: "SOCKS5 proxy", Status: "info", Detail: "Not running"})
	}

	// SSH
	if platform.SSHInstalled() {
		checks = append(checks, DiagnosticCheck{Name: "SSH client", Status: "pass", Detail: "Available"})
	} else {
		checks = append(checks, DiagnosticCheck{Name: "SSH client", Status: "fail", Detail: "Not found"})
	}

	// Internet
	if checkInternet() {
		checks = append(checks, DiagnosticCheck{Name: "Internet", Status: "pass", Detail: "Connected"})
	} else {
		checks = append(checks, DiagnosticCheck{Name: "Internet", Status: "warn", Detail: "No direct internet (captive portal?)"})
	}

	// DNS check
	cfg, _ := config.Load()
	if cfg != nil && cfg.Client.ServerDomain != "" && cfg.Client.ServerDomain != "t.example.com" {
		nsRecords, err := net.LookupNS(cfg.Client.ServerDomain)
		if err == nil && len(nsRecords) > 0 {
			checks = append(checks, DiagnosticCheck{Name: "DNS delegation", Status: "pass", Detail: fmt.Sprintf("NS -> %s", nsRecords[0].Host)})
		} else {
			checks = append(checks, DiagnosticCheck{Name: "DNS delegation", Status: "fail", Detail: fmt.Sprintf("No NS record for %s", cfg.Client.ServerDomain)})
		}
	}

	writeJSON(w, checks)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := config.Load()
		if err != nil {
			writeError(w, "Failed to load config: %v", err)
			return
		}
		writeJSON(w, cfg)

	case http.MethodPost:
		var cfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeError(w, "Invalid config: %v", err)
			return
		}
		if err := config.Save(&cfg); err != nil {
			writeError(w, "Failed to save: %v", err)
			return
		}
		s.addLog("info", "Configuration saved")
		writeJSON(w, map[string]string{"status": "saved"})

	default:
		http.Error(w, "GET or POST required", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	logs := make([]LogEntry, len(s.logs))
	copy(logs, s.logs)
	s.mu.Unlock()
	writeJSON(w, logs)
}

func (s *Server) addLog(level, format string, args ...interface{}) {
	entry := LogEntry{
		Time:    time.Now().Format("15:04:05"),
		Level:   level,
		Message: fmt.Sprintf(format, args...),
	}

	s.mu.Lock()
	s.logs = append(s.logs, entry)
	if len(s.logs) > s.maxLogs {
		s.logs = s.logs[len(s.logs)-s.maxLogs:]
	}
	s.mu.Unlock()
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, format string, args ...interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": fmt.Sprintf(format, args...),
	})
}

func boolStatus(b bool) string {
	if b {
		return "running"
	}
	return "stopped"
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func checkInternet() bool {
	conn, err := net.DialTimeout("tcp", "1.1.1.1:443", 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	}
	if cmd != nil {
		_ = cmd.Start()
	}
}

// Shutdown gracefully shuts down the server.
func Shutdown(ctx context.Context, srv *http.Server) error {
	return srv.Shutdown(ctx)
}

// GeneratePassword exposes password generation to the UI.
func GeneratePassword(w http.ResponseWriter, r *http.Request) {
	pass, err := config.GeneratePassword()
	if err != nil {
		writeError(w, "Failed: %v", err)
		return
	}
	writeJSON(w, map[string]string{"password": pass})
}

func init() {
	// Ensure embed directive is not removed by compiler
	_ = strings.NewReader("")
}
