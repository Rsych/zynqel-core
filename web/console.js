// Zynqel Dev Console — WebSocket + xterm.js terminal

(function () {
  "use strict";

  const selectEl = document.getElementById("session-select");
  const btnCreate = document.getElementById("btn-create");
  const btnKill = document.getElementById("btn-kill");
  const btnRefresh = document.getElementById("btn-refresh");
  const statusEl = document.getElementById("status");

  const term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
    theme: {
      background: "#1a1a2e",
      foreground: "#e0e0e0",
      cursor: "#a0c4ff",
    },
  });
  const fitAddon = new FitAddon.FitAddon();
  term.loadAddon(fitAddon);
  term.open(document.getElementById("terminal-container"));
  fitAddon.fit();

  window.addEventListener("resize", () => fitAddon.fit());

  let ws = null;
  let currentSessionId = null;

  function setStatus(connected, text) {
    const dot = connected ? "connected" : "disconnected";
    statusEl.innerHTML =
      '<span class="dot ' + dot + '"></span>' + (text || dot);
  }

  // --- API helpers ---

  function apiUrl(path) {
    return window.location.origin + path;
  }

  function wsUrl(sessionId) {
    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    return proto + "//" + window.location.host + "/sessions/" + sessionId + "/stream";
  }

  async function fetchSessions() {
    const res = await fetch(apiUrl("/sessions"));
    return res.json();
  }

  async function createSession() {
    const res = await fetch(apiUrl("/sessions"), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ agent: "shell" }),
    });
    return res.json();
  }

  async function killSession(id) {
    await fetch(apiUrl("/sessions/" + id), { method: "DELETE" });
  }

  // --- Session list ---

  async function refreshSessions() {
    const sessions = await fetchSessions();
    selectEl.innerHTML = '<option value="">-- select session --</option>';
    sessions.forEach(function (s) {
      const opt = document.createElement("option");
      opt.value = s.id;
      opt.textContent = s.id.substring(0, 8) + " (" + s.status + ")";
      if (s.id === currentSessionId) opt.selected = true;
      selectEl.appendChild(opt);
    });
  }

  // --- WebSocket connection ---

  function connectWS(sessionId) {
    disconnectWS();
    currentSessionId = sessionId;
    term.clear();
    term.focus();

    ws = new WebSocket(wsUrl(sessionId));

    ws.onopen = function () {
      setStatus(true, "connected");
    };

    ws.onmessage = function (event) {
      var msg;
      try {
        msg = JSON.parse(event.data);
      } catch (e) {
        return;
      }

      switch (msg.type) {
        case "pty.output":
          // Decode base64 → Uint8Array (binary-safe, handles arbitrary bytes)
          var raw = atob(msg.data);
          var buf = new Uint8Array(raw.length);
          for (var i = 0; i < raw.length; i++) buf[i] = raw.charCodeAt(i);
          term.write(buf);
          break;
        case "session.state":
          setStatus(true, msg.data);
          break;
        case "error":
          term.writeln("\r\n\x1B[31mError: " + msg.data + "\x1B[0m");
          break;
      }
    };

    ws.onclose = function () {
      setStatus(false, "disconnected");
      ws = null;
    };

    ws.onerror = function () {
      setStatus(false, "error");
    };
  }

  function disconnectWS() {
    if (ws) {
      ws.close();
      ws = null;
    }
    setStatus(false, "disconnected");
  }

  // --- Terminal input → WebSocket ---

  // Encode string → base64 via TextEncoder (binary-safe for multibyte chars).
  function toBase64(str) {
    var bytes = new TextEncoder().encode(str);
    var binary = "";
    for (var i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]);
    return btoa(binary);
  }

  term.onData(function (data) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({ type: "pty.input", data: toBase64(data) }));
  });

  // --- Button handlers ---

  btnCreate.addEventListener("click", async function () {
    try {
      var sess = await createSession();
      await refreshSessions();
      selectEl.value = sess.id;
      connectWS(sess.id);
    } catch (e) {
      term.writeln("\r\n\x1B[31mFailed to create session: " + e.message + "\x1B[0m");
    }
  });

  btnKill.addEventListener("click", async function () {
    if (!currentSessionId) return;
    try {
      await killSession(currentSessionId);
      disconnectWS();
      currentSessionId = null;
      await refreshSessions();
      term.writeln("\r\n\x1B[33mSession killed.\x1B[0m");
    } catch (e) {
      term.writeln("\r\n\x1B[31mFailed to kill session: " + e.message + "\x1B[0m");
    }
  });

  btnRefresh.addEventListener("click", refreshSessions);

  selectEl.addEventListener("change", function () {
    var id = selectEl.value;
    if (id) {
      connectWS(id);
    } else {
      disconnectWS();
      currentSessionId = null;
    }
  });

  // --- Init ---

  setStatus(false);
  refreshSessions();
  term.writeln("Zynqel Dev Console");
  term.writeln('Click "Create" to start a session.\r\n');
})();
