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

  async function apiFetch(url, opts) {
    const res = await fetch(url, opts);
    if (!res.ok) {
      const body = await res.text();
      throw new Error(res.status + ": " + body);
    }
    return res;
  }

  async function fetchSessions() {
    const res = await apiFetch(apiUrl("/sessions"));
    return res.json();
  }

  async function createSession() {
    const res = await apiFetch(apiUrl("/sessions"), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ agent: "shell" }),
    });
    return res.json();
  }

  async function killSession(id) {
    await apiFetch(apiUrl("/sessions/" + id), { method: "DELETE" });
  }

  // --- Session list ---

  async function refreshSessions() {
    try {
      const sessions = await fetchSessions();
      selectEl.innerHTML = '<option value="">-- select session --</option>';
      sessions.forEach(function (s) {
        const opt = document.createElement("option");
        opt.value = s.id;
        opt.textContent = s.id.substring(0, 8) + " (" + s.status + ")";
        if (s.id === currentSessionId) opt.selected = true;
        selectEl.appendChild(opt);
      });
    } catch (e) {
      term.writeln("\r\n\x1B[31mFailed to list sessions: " + e.message + "\x1B[0m");
    }
  }

  // --- Base64 helpers (binary-safe) ---

  function decodeBase64(b64) {
    try {
      const raw = atob(b64);
      const buf = new Uint8Array(raw.length);
      for (let i = 0; i < raw.length; i++) buf[i] = raw.charCodeAt(i);
      return buf;
    } catch (e) {
      return null;
    }
  }

  function toBase64(str) {
    const bytes = new TextEncoder().encode(str);
    let binary = "";
    for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]);
    return btoa(binary);
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
      let msg;
      try {
        msg = JSON.parse(event.data);
      } catch (e) {
        return;
      }

      switch (msg.type) {
        case "pty.output": {
          const buf = decodeBase64(msg.data);
          if (buf) term.write(buf);
          break;
        }
        case "session.state":
          setStatus(true, msg.data);
          if (msg.data === "stopped") clearAllPrompts();
          break;
        case "intercept.event":
          showPrompt(msg.data);
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
    clearAllPrompts();
    setStatus(false, "disconnected");
  }

  // --- Intercept prompt UI ---

  const promptOverlay = document.getElementById("prompt-overlay");

  function showPrompt(prompt) {
    const card = document.createElement("div");
    card.className = "prompt-card";

    const text = document.createElement("div");
    text.className = "prompt-text";
    text.textContent = prompt.text || "Confirm?";
    card.appendChild(text);

    const buttons = document.createElement("div");
    buttons.className = "prompt-buttons";

    (prompt.options || ["Yes", "No"]).forEach(function (option) {
      const btn = document.createElement("button");
      btn.textContent = option;
      btn.className = option === "Yes" ? "yes" : "no";
      if (prompt.default === option) btn.textContent += " *";
      btn.addEventListener("click", function () {
        sendInterceptResponse(prompt.id, option);
        card.remove();
      });
      buttons.appendChild(btn);
    });

    card.appendChild(buttons);
    promptOverlay.appendChild(card);
  }

  function sendInterceptResponse(eventId, option) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({
      type: "intercept.response",
      data: { id: eventId, option: option },
    }));
  }

  function clearAllPrompts() {
    promptOverlay.innerHTML = "";
  }

  // --- Terminal input -> WebSocket ---

  term.onData(function (data) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({ type: "pty.input", data: toBase64(data) }));
  });

  // --- Button handlers ---

  btnCreate.addEventListener("click", async function () {
    try {
      const sess = await createSession();
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
    const id = selectEl.value;
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
