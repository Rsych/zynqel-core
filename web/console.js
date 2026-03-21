// Zynqel Dev Console — WebSocket + xterm.js terminal

(function () {
  "use strict";

  const btnNew = document.getElementById("btn-new");
  const btnWorkspaces = document.getElementById("btn-workspaces");
  const btnStop = document.getElementById("btn-stop");
  const currentWsEl = document.getElementById("current-workspace");
  const currentWsName = document.getElementById("current-ws-name");
  const statusEl = document.getElementById("status");

  // Modal elements
  const modalOverlay = document.getElementById("modal-overlay");
  const mAgent = document.getElementById("m-agent");
  const mRepo = document.getElementById("m-repo");
  const mBranch = document.getElementById("m-branch");
  const mWorkspace = document.getElementById("m-workspace");
  const mCancel = document.getElementById("m-cancel");
  const mCreate = document.getElementById("m-create");

  const term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
    theme: {
      background: "#000000",
      foreground: "#e0e0e0",
      cursor: "#22c55e",
      selectionBackground: "#22c55e33",
    },
  });
  const fitAddon = new FitAddon.FitAddon();
  term.loadAddon(fitAddon);
  term.open(document.getElementById("terminal-container"));
  fitAddon.fit();

  // Prevent browser from capturing Tab, etc.
  term.attachCustomKeyEventHandler(function (event) {
    if ((event.ctrlKey && event.shiftKey && event.key === "I") || event.key === "F12") {
      return false;
    }
    if (event.key === "Tab") {
      event.preventDefault();
    }
    return true;
  });

  // Debounced resize.
  let resizeTimer = null;
  function debouncedFit() {
    if (resizeTimer) clearTimeout(resizeTimer);
    resizeTimer = setTimeout(function () {
      fitAddon.fit();
      sendResize();
    }, 100);
  }

  function sendResize() {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({
      type: "pty.resize",
      data: { cols: term.cols, rows: term.rows },
    }));
  }

  window.addEventListener("resize", debouncedFit);

  let ws = null;
  let currentSessionId = null;
  let currentWorkspaceId = null;

  function setStatus(connected, text) {
    const dot = connected ? "bg-green-500" : "bg-red-500";
    statusEl.innerHTML =
      '<span class="inline-block w-2 h-2 rounded-full ' + dot + ' mr-1 align-middle"></span>' + (text || (connected ? "connected" : "disconnected"));
  }

  function showCurrentWorkspace(wsId) {
    currentWorkspaceId = wsId;
    currentWsName.textContent = wsId;
    currentWsEl.classList.remove("hidden");
  }

  function hideCurrentWorkspace() {
    currentWorkspaceId = null;
    currentWsEl.classList.add("hidden");
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

  async function killSession(id) {
    await apiFetch(apiUrl("/sessions/" + id), { method: "DELETE" });
  }

  // --- Modal ---

  const imageConfigs = {
    shell:  { agent: "shell", image: "zynqel-base:latest" },
    claude: { agent: "claude" },
    qwen:   { agent: "shell", image: "zynqel-qwen:latest" },
  };

  function openModal() {
    mRepo.value = "";
    mBranch.value = "";
    mWorkspace.value = "";
    mAgent.value = "shell";
    modalOverlay.classList.remove("hidden");
    mRepo.focus();
  }

  function closeModal() {
    modalOverlay.classList.add("hidden");
    term.focus();
  }

  // Auto-generate workspace ID from repo URL.
  mRepo.addEventListener("input", function () {
    const url = mRepo.value.trim();
    if (!url) { mWorkspace.value = ""; return; }
    const match = url.match(/\/([^/]+?)(\.git)?$/);
    if (match) {
      mWorkspace.value = match[1];
    }
  });

  async function handleCreate() {
    const selected = mAgent.value || "shell";
    const config = Object.assign({}, imageConfigs[selected] || imageConfigs.shell);

    const repo = mRepo.value.trim();
    const branch = mBranch.value.trim();
    const wsId = mWorkspace.value.trim();

    if (repo) config.repo_url = repo;
    if (branch) config.branch = branch;
    if (wsId) config.workspace_id = wsId;

    closeModal();
    term.reset();

    // Show progress in terminal.
    term.writeln("\x1B[32m>\x1B[0m Creating workspace...");
    if (repo) term.writeln("\x1B[32m>\x1B[0m Cloning " + repo + (branch ? " (" + branch + ")" : ""));
    term.writeln("");

    mCreate.disabled = true;
    mCreate.textContent = "Creating...";

    try {
      const res = await apiFetch(apiUrl("/sessions"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(config),
      });
      const sess = await res.json();
      term.writeln("\x1B[32m>\x1B[0m Ready! Connecting...");
      term.writeln("");
      currentSessionId = sess.id;
      showCurrentWorkspace(sess.spec.workspace_id);
      connectWS(sess.id);
    } catch (e) {
      term.writeln("\r\n\x1B[31m> Error: " + e.message + "\x1B[0m");
    } finally {
      mCreate.disabled = false;
      mCreate.textContent = "Create";
    }
  }

  btnNew.addEventListener("click", openModal);
  mCancel.addEventListener("click", closeModal);
  mCreate.addEventListener("click", handleCreate);
  modalOverlay.addEventListener("click", function (e) {
    if (e.target === modalOverlay) closeModal();
  });

  // --- Workspaces panel ---

  const wsOverlay = document.getElementById("ws-overlay");
  const wsList = document.getElementById("ws-list");
  const wsCloseBtn = document.getElementById("ws-close");

  async function openWorkspaces() {
    wsOverlay.classList.remove("hidden");
    wsList.innerHTML = '<p class="text-neutral-500 text-sm">Loading...</p>';

    try {
      const res = await apiFetch(apiUrl("/workspaces"));
      const workspaces = await res.json();

      if (workspaces.length === 0) {
        wsList.innerHTML = '<p class="text-neutral-500 text-sm">No saved workspaces. Click + New to create one.</p>';
        return;
      }

      // Check which workspaces have running sessions.
      const sessRes = await apiFetch(apiUrl("/sessions"));
      const sessions = await sessRes.json();
      const runningMap = {};
      sessions.forEach(function (s) {
        if (s.spec.workspace_id) runningMap[s.spec.workspace_id] = s.id;
      });

      wsList.innerHTML = "";
      workspaces.forEach(function (workspace) {
        const isRunning = !!runningMap[workspace.id];
        const row = document.createElement("div");
        row.className = "flex items-center justify-between bg-neutral-800 border border-neutral-700 rounded px-3 py-2";

        const info = document.createElement("div");
        info.className = "flex items-center gap-2";
        info.innerHTML = '<span class="text-sm text-gray-200 font-mono">' + workspace.id + '</span>' +
          (isRunning ? '<span class="text-[10px] bg-green-900/50 text-green-400 border border-green-700/50 rounded px-1">running</span>' : '');
        row.appendChild(info);

        const actions = document.createElement("div");
        actions.className = "flex gap-2";

        const openBtn = document.createElement("button");
        openBtn.textContent = isRunning ? "Connect" : "Open";
        openBtn.className = "bg-green-900/50 hover:bg-green-800/60 border border-green-700/50 rounded px-3 py-1 text-xs text-green-400";
        openBtn.addEventListener("click", function () {
          closeWorkspaces();
          resumeWorkspace(workspace.id);
        });
        actions.appendChild(openBtn);

        const deleteBtn = document.createElement("button");
        deleteBtn.textContent = "Delete";
        deleteBtn.className = "bg-red-900/30 hover:bg-red-800/40 border border-red-700/40 rounded px-3 py-1 text-xs text-red-400";
        deleteBtn.addEventListener("click", async function () {
          if (!confirm("Delete workspace '" + workspace.id + "'? All files will be lost.")) return;
          try {
            // Kill running session first if any.
            if (runningMap[workspace.id]) {
              await apiFetch(apiUrl("/sessions/" + runningMap[workspace.id]), { method: "DELETE" });
            }
            await apiFetch(apiUrl("/workspaces/" + workspace.id), { method: "DELETE" });
            await openWorkspaces(); // refresh list
          } catch (e) {
            alert("Failed to delete: " + e.message);
          }
        });
        actions.appendChild(deleteBtn);

        row.appendChild(actions);
        wsList.appendChild(row);
      });
    } catch (e) {
      wsList.innerHTML = '<p class="text-red-400 text-sm">Failed to load: ' + e.message + '</p>';
    }
  }

  async function resumeWorkspace(wsId) {
    disconnectWS();
    term.reset();
    term.writeln("\x1B[32m>\x1B[0m Opening workspace " + wsId + "...");
    term.writeln("");

    try {
      // POST /sessions with workspace_id — backend returns existing session if running.
      const res = await apiFetch(apiUrl("/sessions"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ agent: "shell", image: "zynqel-base:latest", workspace_id: wsId }),
      });
      const sess = await res.json();
      currentSessionId = sess.id;
      showCurrentWorkspace(wsId);
      connectWS(sess.id);
    } catch (e) {
      term.writeln("\r\n\x1B[31m> Error: " + e.message + "\x1B[0m");
    }
  }

  function closeWorkspaces() {
    wsOverlay.classList.add("hidden");
    term.focus();
  }

  btnWorkspaces.addEventListener("click", openWorkspaces);
  wsCloseBtn.addEventListener("click", closeWorkspaces);
  wsOverlay.addEventListener("click", function (e) {
    if (e.target === wsOverlay) closeWorkspaces();
  });

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

    ws = new WebSocket(wsUrl(sessionId));

    ws.onopen = function () {
      setStatus(true, "connected");
      fitAddon.fit();
      sendResize();
      term.focus();
      // Trigger a fresh prompt.
      setTimeout(function () {
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: "pty.input", data: toBase64("\n") }));
        }
      }, 200);
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
    card.className = "bg-neutral-900 border border-neutral-700 rounded-lg p-3 max-w-sm shadow-lg";

    const text = document.createElement("div");
    text.className = "text-sm text-gray-300 mb-2";
    text.textContent = prompt.text || "Confirm?";
    card.appendChild(text);

    const buttons = document.createElement("div");
    buttons.className = "flex gap-2";

    (prompt.options || ["Yes", "No"]).forEach(function (option) {
      const btn = document.createElement("button");
      btn.textContent = option;
      btn.className = option === "Yes"
        ? "bg-green-900/50 hover:bg-green-800/60 border border-green-700/50 rounded px-4 py-1 text-sm text-green-400"
        : "bg-red-900/30 hover:bg-red-800/40 border border-red-700/40 rounded px-3 py-1 text-sm text-red-400";
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

  btnStop.addEventListener("click", async function () {
    if (!currentSessionId) return;
    try {
      await killSession(currentSessionId);
      disconnectWS();
      currentSessionId = null;
      hideCurrentWorkspace();
      term.reset();
      term.writeln("\x1B[33m> Workspace stopped.\x1B[0m");
      term.writeln('Click \x1B[32m+ New\x1B[0m or \x1B[34mWorkspaces\x1B[0m to continue.\r\n');
    } catch (e) {
      term.writeln("\r\n\x1B[31m> Failed to stop: " + e.message + "\x1B[0m");
    }
  });

  // --- Init ---

  setStatus(false);
  term.writeln("\x1B[32mZynqel\x1B[0m Terminal");
  term.writeln('Click \x1B[32m+ New\x1B[0m to create a workspace or \x1B[34mWorkspaces\x1B[0m to resume.\r\n');
})();
