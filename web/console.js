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

  term.attachCustomKeyEventHandler(function (event) {
    if ((event.ctrlKey && event.shiftKey && event.key === "I") || event.key === "F12") {
      return false;
    }
    if (event.key === "Tab") {
      event.preventDefault();
    }
    return true;
  });

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

  // --- State ---

  let ws = null;
  let wsGeneration = 0; // tracks WS identity to prevent stale handler races
  let currentSessionId = null;
  let currentWorkspaceId = null;

  // --- UI helpers ---

  function setStatus(connected, text) {
    const dot = connected ? "bg-green-500" : "bg-red-500";
    statusEl.innerHTML =
      '<span class="inline-block w-2 h-2 rounded-full ' + dot + ' mr-1 align-middle"></span>' +
      (text || (connected ? "connected" : "disconnected"));
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

  function showWelcome() {
    term.reset();
    term.writeln("");
    term.writeln("  \x1B[32m ____                       _ \x1B[0m");
    term.writeln("  \x1B[32m|_  / _  _ _ _  __ _ ___| |\x1B[0m");
    term.writeln("  \x1B[32m / / | || | ' \\/ _` / -_) |\x1B[0m");
    term.writeln("  \x1B[32m/___| \\_, |_||_\\__, \\___|_|\x1B[0m");
    term.writeln("  \x1B[32m      |__/        |_|      \x1B[0m");
    term.writeln("");
    term.writeln("  Run AI coding agents in your browser.");
    term.writeln("");
    term.writeln("  \x1B[32m+ New\x1B[0m          Create a new workspace");
    term.writeln("  \x1B[34mWorkspaces\x1B[0m     Resume a saved workspace");
    term.writeln("");
    term.writeln("  \x1B[90mWorkspaces persist across sessions and server restarts.\x1B[0m");
    term.writeln("");
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

  // --- New workspace modal ---

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

  mRepo.addEventListener("input", function () {
    const url = mRepo.value.trim();
    if (!url) { mWorkspace.value = ""; return; }
    const match = url.match(/\/([^/]+?)(\.git)?$/);
    if (match) mWorkspace.value = match[1];
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
      term.writeln("\x1B[32m>\x1B[0m Ready!");
      term.writeln("");
      currentSessionId = sess.id;
      showCurrentWorkspace(sess.spec.workspace_id);
      connectWS(sess.id);
    } catch (e) {
      term.writeln("\x1B[31m> Error: " + e.message + "\x1B[0m");
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
          resumeWorkspace(workspace.id, workspace.image, workspace.agent);
        });
        actions.appendChild(openBtn);

        if (isRunning) {
          const stopBtn = document.createElement("button");
          stopBtn.textContent = "Stop";
          stopBtn.className = "bg-red-900/30 hover:bg-red-800/40 border border-red-700/40 rounded px-3 py-1 text-xs text-red-400";
          stopBtn.addEventListener("click", async function () {
            try {
              await killSession(runningMap[workspace.id]);
              await openWorkspaces(); // refresh
            } catch (e) {
              alert("Failed to stop: " + e.message);
            }
          });
          actions.appendChild(stopBtn);
        }

        const deleteBtn = document.createElement("button");
        deleteBtn.textContent = "Delete";
        deleteBtn.className = "bg-neutral-700/50 hover:bg-neutral-600/50 border border-neutral-600 rounded px-3 py-1 text-xs text-neutral-400";
        deleteBtn.addEventListener("click", async function () {
          if (!confirm("Delete workspace '" + workspace.id + "'? All files will be lost.")) return;
          try {
            if (runningMap[workspace.id]) {
              await killSession(runningMap[workspace.id]);
            }
            await apiFetch(apiUrl("/workspaces/" + workspace.id), { method: "DELETE" });
            await openWorkspaces();
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

  async function resumeWorkspace(wsId, image, agent) {
    disconnectWS();
    term.reset();
    term.writeln("\x1B[32m>\x1B[0m Opening workspace \x1B[1m" + wsId + "\x1B[0m...");
    term.writeln("");

    try {
      const config = {
        agent: agent || "shell",
        workspace_id: wsId,
      };
      if (image) config.image = image;
      const res = await apiFetch(apiUrl("/sessions"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(config),
      });
      const sess = await res.json();
      currentSessionId = sess.id;
      showCurrentWorkspace(wsId);
      connectWS(sess.id);
    } catch (e) {
      term.writeln("\x1B[31m> Error: " + e.message + "\x1B[0m");
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

  // --- Base64 helpers ---

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
    const gen = ++wsGeneration; // track this connection's identity
    currentSessionId = sessionId;

    ws = new WebSocket(wsUrl(sessionId));

    ws.onopen = function () {
      if (gen !== wsGeneration) return;
      setStatus(true, "connected");
      fitAddon.fit();
      sendResize();
      term.focus();
      setTimeout(function () {
        if (gen === wsGeneration && ws && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: "pty.input", data: toBase64("\n") }));
        }
      }, 200);
    };

    ws.onmessage = function (event) {
      if (gen !== wsGeneration) return;
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
          if (gen !== wsGeneration) return;
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
      if (gen !== wsGeneration) return; // stale handler — ignore
      setStatus(false, "disconnected");
      ws = null;
    };

    ws.onerror = function () {
      if (gen !== wsGeneration) return;
      setStatus(false, "error");
    };
  }

  function disconnectWS() {
    wsGeneration++; // invalidate any pending handlers
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

  // --- Terminal input ---

  term.onData(function (data) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({ type: "pty.input", data: toBase64(data) }));
  });

  // --- Stop button ---

  btnStop.addEventListener("click", async function () {
    if (!currentSessionId) return;
    term.reset();
    term.writeln("\x1B[33m>\x1B[0m Stopping workspace...");

    try {
      await killSession(currentSessionId);
      disconnectWS();
      currentSessionId = null;
      hideCurrentWorkspace();
      showWelcome();
    } catch (e) {
      term.writeln("\x1B[31m> Failed to stop: " + e.message + "\x1B[0m");
    }
  });

  // --- Init ---

  setStatus(false);
  showWelcome();
})();
