/* ==========================================================
   tcpdns — dashboard controller
   Vanilla JS, zero dependencies
   ========================================================== */
(function () {
  "use strict";

  /* ---------- DOM helpers ---------- */
  var $ = function (s) { return document.querySelector(s); };
  var $$ = function (s) { return document.querySelectorAll(s); };

  /* ---------- State ---------- */
  var toastTimeout = null;
  var connectTimeout = null;
  var state = {
    clientRunning: false,
    proxyRunning: false,
    transitioning: false
  };

  /* ==========================================================
     API
     ========================================================== */
  function api(method, path, body) {
    var opts = { method: method, headers: {} };
    if (body !== undefined) {
      opts.headers["Content-Type"] = "application/json";
      opts.body = JSON.stringify(body);
    }
    return fetch(path, opts).then(function (res) {
      if (!res.ok) throw new Error(res.status + " " + res.statusText);
      return res.json();
    });
  }

  /* ==========================================================
     Toast
     ========================================================== */
  function toast(message, type) {
    type = type || "success";
    var el = $("#toast");
    el.textContent = message;
    el.className = "toast toast-" + type + " visible";
    clearTimeout(toastTimeout);
    toastTimeout = setTimeout(function () {
      el.classList.remove("visible");
    }, 3000);
  }

  /* ==========================================================
     Tab Navigation
     ========================================================== */
  function switchTab(tab) {
    $$(".nav-btn").forEach(function (btn) {
      btn.classList.toggle("active", btn.dataset.tab === tab);
    });
    $$(".tab-content").forEach(function (el) {
      el.classList.toggle("hidden", el.id !== "tab-" + tab);
    });
    if (tab === "settings") loadSettings();
    if (tab === "logs") fetchLogs();
  }

  /* ==========================================================
     Status Polling
     ========================================================== */
  function fetchStatus() {
    api("GET", "/api/status")
      .then(updateDashboard)
      .catch(function () {
        setRingState("error", "Connection Error", "Cannot reach the tcpdns service");
        setConnectButton(false, false);
      });
  }

  function updateDashboard(data) {
    var client = data.client || {};
    var proxy = data.proxy || {};
    var config = data.config || {};
    var system = data.system || {};

    state.clientRunning = !!client.running;
    state.proxyRunning = !!proxy.running;

    /* --- Connection ring --- */
    if (state.transitioning) {
      if (state.clientRunning) {
        state.transitioning = false;
        clearTimeout(connectTimeout);
        setRingState("connected", "Connected", config.domain || "DNS tunnel active");
        toast("Connected successfully");
      }
      /* else keep showing connecting spinner */
    } else if (state.clientRunning) {
      setRingState("connected", "Connected", config.domain || "DNS tunnel active");
    } else {
      setRingState("disconnected", "Disconnected", "Click connect to start the DNS tunnel");
    }

    /* --- Connect / Disconnect button --- */
    setConnectButton(state.clientRunning, state.transitioning);

    /* --- Tunnel status --- */
    var tunnelInd = $("#tunnel-indicator");
    var tunnelStatus = $("#tunnel-status");
    var tunnelDomain = $("#tunnel-domain");
    tunnelInd.className = "indicator" + (client.running ? " active" : "");
    tunnelStatus.textContent = client.running ? "Running" : "Stopped";
    tunnelDomain.textContent = config.domain || "-";

    /* --- Proxy status --- */
    var proxyInd = $("#proxy-indicator");
    var proxyStatusEl = $("#proxy-status");
    var proxyAddr = $("#proxy-addr");
    var proxyBtn = $("#btn-proxy");
    proxyInd.className = "indicator" + (proxy.running ? " active" : "");
    proxyStatusEl.textContent = proxy.running ? "Running" : "Stopped";
    proxyAddr.textContent = proxy.detail || config.proxy_addr || "-";
    proxyBtn.textContent = proxy.running ? "Stop Proxy" : "Start Proxy";

    /* --- System info --- */
    $("#sys-platform").textContent =
      (system.os || "-") + "/" + (system.arch || "-");
    setText("#sys-iodine", system.iodine_client ? "Installed" : "Not found", system.iodine_client);
    setText("#sys-ssh", system.ssh ? "Available" : "Not found", system.ssh);
    setText("#sys-internet", system.internet ? "Online" : "Offline", system.internet);
  }

  /* --- Helpers --- */
  function setRingState(cls, title, detail) {
    var ring = $("#status-ring");
    ring.className = "status-ring " + cls;
    $("#connection-status").textContent = title;
    $("#connection-detail").textContent = detail;
  }

  function setConnectButton(running, transitioning) {
    var btn = $("#btn-connect");
    if (running) {
      btn.textContent = "Disconnect";
      btn.className = "btn btn-danger btn-large";
      btn.disabled = false;
    } else if (transitioning) {
      btn.textContent = "Connecting\u2026";
      btn.className = "btn btn-large";
      btn.disabled = true;
    } else {
      btn.textContent = "Connect";
      btn.className = "btn btn-connect btn-large";
      btn.disabled = false;
    }
  }

  function setText(sel, text, ok) {
    var el = $(sel);
    el.textContent = text;
    el.style.color = ok ? "var(--green)" : "var(--red)";
  }

  /* ==========================================================
     Connect / Disconnect
     ========================================================== */
  function handleConnect() {
    if (state.transitioning) return;

    if (state.clientRunning) {
      /* --- Disconnect --- */
      var btn = $("#btn-connect");
      btn.textContent = "Disconnecting\u2026";
      btn.disabled = true;
      api("POST", "/api/client/disconnect")
        .then(function () {
          state.clientRunning = false;
          setRingState("disconnected", "Disconnected", "Tunnel closed");
          setConnectButton(false, false);
          toast("Disconnected");
        })
        .catch(function (err) {
          toast("Failed to disconnect: " + err.message, "error");
          btn.disabled = false;
        });
    } else {
      /* --- Connect --- */
      state.transitioning = true;
      setRingState("connecting", "Connecting\u2026", "Establishing DNS tunnel");
      setConnectButton(false, true);

      /* Safety timeout — clear transitioning after 30 s */
      clearTimeout(connectTimeout);
      connectTimeout = setTimeout(function () {
        if (state.transitioning) {
          state.transitioning = false;
          setRingState("error", "Connection Timed Out", "The tunnel did not connect in time");
          setConnectButton(false, false);
          toast("Connection timed out", "error");
        }
      }, 30000);

      api("POST", "/api/client/connect")
        .then(function () {
          /* status poll will pick up the running state */
        })
        .catch(function (err) {
          state.transitioning = false;
          clearTimeout(connectTimeout);
          setRingState("error", "Connection Failed", err.message);
          setConnectButton(false, false);
          toast("Connection failed: " + err.message, "error");
        });
    }
  }

  /* ==========================================================
     Proxy Start / Stop
     ========================================================== */
  function handleProxy() {
    var btn = $("#btn-proxy");
    btn.disabled = true;

    var endpoint = state.proxyRunning ? "/api/proxy/stop" : "/api/proxy/start";
    var successMsg = state.proxyRunning ? "Proxy stopped" : "Proxy started";
    var failMsg = state.proxyRunning ? "Failed to stop proxy" : "Failed to start proxy";

    api("POST", endpoint)
      .then(function () {
        toast(successMsg);
        fetchStatus();
      })
      .catch(function (err) {
        toast(failMsg + ": " + err.message, "error");
      })
      .then(function () {
        btn.disabled = false;
      });
  }

  /* ==========================================================
     Settings
     ========================================================== */
  function loadSettings() {
    api("GET", "/api/config")
      .then(function (cfg) {
        var s = cfg.server || {};
        var c = cfg.client || {};
        var p = cfg.proxy || {};
        var a = cfg.advanced || {};

        $("#cfg-domain").value = s.domain || c.server_domain || "";
        $("#cfg-password").value = s.password || c.password || "";
        $("#cfg-tunnel-ip").value = s.tunnel_ip || "";
        $("#cfg-proxy-listen").value = p.listen || "";
        $("#cfg-ssh-user").value = p.ssh_user || "";
        $("#cfg-ssh-host").value = p.ssh_host || "";
        $("#cfg-record-type").value = a.record_type || "auto";
        $("#cfg-encoding").value = a.encoding || "auto";
      })
      .catch(function () {
        toast("Failed to load settings", "error");
      });
  }

  function saveSettings(e) {
    e.preventDefault();
    var domain = $("#cfg-domain").value;
    var password = $("#cfg-password").value;
    var tunnelIp = $("#cfg-tunnel-ip").value;
    var proxyListen = $("#cfg-proxy-listen").value;
    var sshUser = $("#cfg-ssh-user").value;
    var sshHost = $("#cfg-ssh-host").value;
    var recordType = $("#cfg-record-type").value;
    var encoding = $("#cfg-encoding").value;

    var config = {
      server: { domain: domain, password: password, tunnel_ip: tunnelIp },
      client: { server_domain: domain, password: password },
      proxy: { listen: proxyListen, ssh_user: sshUser, ssh_host: sshHost },
      advanced: { record_type: recordType, encoding: encoding }
    };

    api("POST", "/api/config", config)
      .then(function () { toast("Settings saved"); })
      .catch(function (err) { toast("Failed to save: " + err.message, "error"); });
  }

  function generatePassword() {
    var arr = new Uint8Array(16);
    crypto.getRandomValues(arr);
    var hex = "";
    for (var i = 0; i < arr.length; i++) {
      hex += ("0" + arr[i].toString(16)).slice(-2);
    }
    $("#cfg-password").value = hex;
    /* reveal so user can see the generated value */
    $("#cfg-password").type = "text";
    $("#btn-show-pass").textContent = "Hide";
  }

  function togglePassword() {
    var input = $("#cfg-password");
    var btn = $("#btn-show-pass");
    if (input.type === "password") {
      input.type = "text";
      btn.textContent = "Hide";
    } else {
      input.type = "password";
      btn.textContent = "Show";
    }
  }

  /* ==========================================================
     Diagnostics
     ========================================================== */
  function runDiagnostics() {
    var container = $("#diag-results");
    container.innerHTML = '<p class="empty-state">Running checks\u2026</p>';
    var btn = $("#btn-run-diag");
    btn.disabled = true;

    api("GET", "/api/diagnose")
      .then(function (checks) {
        if (!checks || !checks.length) {
          container.innerHTML = '<p class="empty-state">No checks returned.</p>';
          return;
        }
        container.innerHTML = "";
        checks.forEach(function (check) {
          var row = document.createElement("div");
          row.className = "diag-check diag-" + (check.status || "info");

          var badge = document.createElement("span");
          badge.className = "diag-badge";
          badge.textContent = (check.status || "info").toUpperCase();

          var name = document.createElement("span");
          name.className = "diag-name";
          name.textContent = check.name || "";

          var detail = document.createElement("span");
          detail.className = "diag-detail";
          detail.textContent = check.detail || "";

          row.appendChild(badge);
          row.appendChild(name);
          row.appendChild(detail);
          container.appendChild(row);
        });
      })
      .catch(function (err) {
        container.innerHTML =
          '<p class="empty-state">Diagnostics failed: ' + escapeHtml(err.message) + "</p>";
        toast("Diagnostics failed", "error");
      })
      .then(function () {
        btn.disabled = false;
      });
  }

  /* ==========================================================
     Logs
     ========================================================== */
  function fetchLogs() {
    api("GET", "/api/logs")
      .then(function (entries) {
        var container = $("#log-container");
        if (!entries || !entries.length) {
          container.innerHTML = '<p class="empty-state">No log entries yet.</p>';
          return;
        }
        container.innerHTML = "";
        entries.forEach(function (entry) {
          var row = document.createElement("div");
          row.className = "log-entry log-" + (entry.level || "info");

          var time = document.createElement("span");
          time.className = "log-time";
          time.textContent = entry.time || "";

          var level = document.createElement("span");
          level.className = "log-level";
          level.textContent = (entry.level || "info").toUpperCase();

          var msg = document.createElement("span");
          msg.className = "log-message";
          msg.textContent = entry.message || "";

          row.appendChild(time);
          row.appendChild(level);
          row.appendChild(msg);
          container.appendChild(row);
        });
        container.scrollTop = container.scrollHeight;
      })
      .catch(function () {
        /* silent — logs are non-critical */
      });
  }

  /* ==========================================================
     Utilities
     ========================================================== */
  function escapeHtml(str) {
    var div = document.createElement("div");
    div.appendChild(document.createTextNode(str));
    return div.innerHTML;
  }

  /* ==========================================================
     Init
     ========================================================== */
  function init() {
    /* Prepare toast — switch from display:none to opacity-based hiding */
    $("#toast").classList.remove("hidden");

    /* Tab navigation */
    $$(".nav-btn").forEach(function (btn) {
      btn.addEventListener("click", function () {
        switchTab(btn.dataset.tab);
      });
    });

    /* Connect / Disconnect */
    $("#btn-connect").addEventListener("click", handleConnect);

    /* Proxy */
    $("#btn-proxy").addEventListener("click", handleProxy);

    /* Settings */
    $("#settings-form").addEventListener("submit", saveSettings);
    $("#btn-gen-pass").addEventListener("click", generatePassword);
    $("#btn-show-pass").addEventListener("click", togglePassword);

    /* Diagnostics */
    $("#btn-run-diag").addEventListener("click", runDiagnostics);

    /* Logs */
    $("#btn-refresh-logs").addEventListener("click", fetchLogs);

    /* Initial fetch + auto-refresh every 2 s */
    fetchStatus();
    setInterval(fetchStatus, 2000);
  }

  document.addEventListener("DOMContentLoaded", init);
})();
