// Zynqel Dev Console — WebSocket + xterm.js terminal

(function () {
  "use strict";

  var btnNew = document.getElementById("btn-new");
  var btnWorkspaces = document.getElementById("btn-workspaces");
  var btnStop = document.getElementById("btn-stop");
  var currentWsEl = document.getElementById("current-workspace");
  var currentWsName = document.getElementById("current-ws-name");
  var statusEl = document.getElementById("status");

  // Modal elements
  var modalOverlay = document.getElementById("modal-overlay");
  var mAgent = document.getElementById("m-agent");
  var mRepo = document.getElementById("m-repo");
  var mBranch = document.getElementById("m-branch");
  var mWorkspace = document.getElementById("m-workspace");
  var mCancel = document.getElementById("m-cancel");
  var mCreate = document.getElementById("m-create");
  var mError = document.getElementById("m-error");

  var term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    scrollback: 5000,
    fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
    theme: {
      background: "#000000",
      foreground: "#e0e0e0",
      cursor: "#22c55e",
      selectionBackground: "#22c55e33",
    },
  });
  var fitAddon = new FitAddon.FitAddon();
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

  // Escape closes modals.
  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape") {
      if (!modalOverlay.classList.contains("hidden")) { closeModal(); }
      if (!wsOverlay.classList.contains("hidden")) { closeWorkspaces(); }
    }
  });

  var resizeTimer = null;
  function debouncedFit() {
    if (resizeTimer) clearTimeout(resizeTimer);
    resizeTimer = setTimeout(function () {
      fitAddon.fit();
      sendResize();
    }, 100);
  }

  function sendResize() {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({ type: "pty.resize", data: { cols: term.cols, rows: term.rows } }));
  }

  window.addEventListener("resize", debouncedFit);

  // --- State ---

  var ws = null;
  var wsGeneration = 0;
  var currentSessionId = null;
  var currentWorkspaceId = null;

  // --- Friendly error messages ---

  function friendlyError(msg) {
    if (msg.indexOf("connection refused") !== -1) return "Cannot connect to server. Is zynqel-core running?";
    if (msg.indexOf("pull access denied") !== -1) return "Docker image not found. Run: make images";
    if (msg.indexOf("at capacity") !== -1) return "Too many active sessions. Stop one and try again.";
    if (msg.indexOf("No such image") !== -1) return "Docker image not built. Run: make images";
    if (msg.indexOf("could not read Username") !== -1) return "Repository not found or is private. Check the URL.";
    if (msg.indexOf("Repository not found") !== -1) return "Repository not found. Check the URL.";
    // Strip JSON wrapper and binary framing from error messages.
    var jsonMatch = msg.match(/"error":"([^"]+)"/);
    if (jsonMatch) msg = jsonMatch[1];
    // Clean up Docker exec binary framing bytes.
    msg = msg.replace(/\\u[\da-f]{4}/gi, "").replace(/\\n/g, " ").trim();
    return msg;
  }

  // --- UI helpers ---

  function setStatus(connected, text) {
    var dot = connected ? "bg-green-500" : "bg-red-500";
    statusEl.innerHTML = '<span class="inline-block w-2 h-2 rounded-full ' + dot + ' mr-1 align-middle"></span>' + (text || (connected ? "connected" : "disconnected"));
  }

  function showCurrentWorkspace(wsId) {
    currentWorkspaceId = wsId;
    currentWsName.textContent = wsId;
    currentWsEl.classList.remove("hidden");
    document.title = "Zynqel — " + wsId;
  }

  function hideCurrentWorkspace() {
    currentWorkspaceId = null;
    currentWsEl.classList.add("hidden");
    document.title = "Zynqel";
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

  // Animated dots for loading feedback.
  var loadingInterval = null;
  function startLoading(prefix) {
    var dots = 0;
    term.write("\x1B[32m" + prefix + "\x1B[0m");
    loadingInterval = setInterval(function () {
      term.write(".");
      dots++;
      if (dots > 30) { stopLoading(); term.writeln(" (still working)"); }
    }, 500);
  }
  function stopLoading() {
    if (loadingInterval) { clearInterval(loadingInterval); loadingInterval = null; }
  }

  // --- API helpers ---

  function apiUrl(path) { return window.location.origin + path; }

  function wsUrl(sessionId) {
    var proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    return proto + "//" + window.location.host + "/sessions/" + sessionId + "/stream";
  }

  async function apiFetch(url, opts) {
    var res = await fetch(url, opts);
    if (!res.ok) {
      var body = await res.text();
      throw new Error(friendlyError(body));
    }
    return res;
  }

  async function killSession(id) {
    await apiFetch(apiUrl("/sessions/" + id), { method: "DELETE" });
  }

  // --- New workspace modal ---

  var imageConfigs = {
    shell:  { agent: "shell", image: "zynqel-base:latest" },
    claude: { agent: "claude" },
    qwen:   { agent: "shell", image: "zynqel-qwen:latest" },
  };

  function openModal() {
    mRepo.value = "";
    mBranch.value = "";
    mWorkspace.value = "";
    mAgent.value = "shell";
    mError.classList.add("hidden");
    mError.textContent = "";
    mCreate.disabled = false;
    mCreate.textContent = "Create";
    modalOverlay.classList.remove("hidden");
    mAgent.focus();
  }

  function closeModal() {
    modalOverlay.classList.add("hidden");
    term.focus();
  }

  mRepo.addEventListener("input", function () {
    var url = mRepo.value.trim();
    if (!url) { mWorkspace.value = ""; return; }
    // Handle HTTPS, SSH, and shorthand formats.
    var match = url.match(/[/:]([^/:]+?)(\.git)?$/);
    if (match) mWorkspace.value = match[1].toLowerCase();
  });

  async function handleCreate() {
    var selected = mAgent.value || "shell";
    var config = Object.assign({}, imageConfigs[selected] || imageConfigs.shell);

    var repo = mRepo.value.trim();
    var branch = mBranch.value.trim();
    var wsId = mWorkspace.value.trim();

    if (repo) config.repo_url = repo;
    if (branch) config.branch = branch;
    if (wsId) config.workspace_id = wsId;

    // Keep modal open — show loading state.
    mError.classList.add("hidden");
    mCreate.disabled = true;
    mCreate.textContent = "Creating...";

    try {
      var res = await apiFetch(apiUrl("/sessions"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(config),
      });
      var sess = await res.json();

      // Success — close modal and connect.
      closeModal();
      term.reset();
      term.writeln("\x1B[32m>\x1B[0m Workspace ready!");
      term.writeln("");
      currentSessionId = sess.id;
      showCurrentWorkspace(sess.spec.workspace_id);
      connectWS(sess.id);
    } catch (e) {
      // Error — keep modal open, show inline error.
      mError.textContent = e.message;
      mError.classList.remove("hidden");
    } finally {
      mCreate.disabled = false;
      mCreate.textContent = "Create";
    }
  }

  btnNew.addEventListener("click", openModal);
  mCancel.addEventListener("click", closeModal);
  mCreate.addEventListener("click", handleCreate);
  // Don't dismiss modal on overlay click — only Cancel or Escape.

  // --- Workspaces panel ---

  var wsOverlay = document.getElementById("ws-overlay");
  var wsList = document.getElementById("ws-list");
  var wsCloseBtn = document.getElementById("ws-close");

  async function openWorkspaces() {
    wsOverlay.classList.remove("hidden");
    wsList.innerHTML = '<p class="text-neutral-500 text-sm animate-pulse">Loading workspaces...</p>';

    try {
      var res = await apiFetch(apiUrl("/workspaces"));
      var workspaces = await res.json();

      if (workspaces.length === 0) {
        wsList.innerHTML = '<p class="text-neutral-500 text-sm">No saved workspaces. Click + New to create one.</p>';
        return;
      }

      var sessRes = await apiFetch(apiUrl("/sessions"));
      var sessions = await sessRes.json();
      var runningMap = {};
      sessions.forEach(function (s) {
        if (s.spec.workspace_id) runningMap[s.spec.workspace_id] = s.id;
      });

      wsList.innerHTML = "";
      workspaces.forEach(function (workspace) {
        var isRunning = !!runningMap[workspace.id];
        var isCurrent = currentWorkspaceId === workspace.id;
        var row = document.createElement("div");
        row.className = "flex items-center justify-between rounded px-3 py-2 " +
          (isCurrent ? "bg-green-900/20 border border-green-700/30" : "bg-neutral-800 border border-neutral-700");

        var info = document.createElement("div");
        var agentLabel = workspace.agent ? ' <span class="text-[10px] text-neutral-500">' + workspace.agent + '</span>' : '';
        info.className = "flex items-center gap-2";
        info.innerHTML = '<span class="text-sm text-gray-200 font-mono">' + workspace.id + '</span>' +
          agentLabel +
          (isRunning ? '<span class="text-[10px] bg-green-900/50 text-green-400 border border-green-700/50 rounded px-1">running</span>' : '');
        row.appendChild(info);

        var actions = document.createElement("div");
        actions.className = "flex gap-2";

        if (isCurrent) {
          var currentBadge = document.createElement("span");
          currentBadge.textContent = "Current";
          currentBadge.className = "bg-green-900/50 text-green-400 border border-green-700/50 rounded px-3 py-1 text-xs";
          actions.appendChild(currentBadge);
        } else {
          var openBtn = document.createElement("button");
          openBtn.textContent = isRunning ? "Connect" : "Open";
          openBtn.className = "bg-green-900/50 hover:bg-green-800/60 border border-green-700/50 rounded px-3 py-1 text-xs text-green-400";
          openBtn.addEventListener("click", function () {
            closeWorkspaces();
            resumeWorkspace(workspace.id, workspace.image, workspace.agent);
          });
          actions.appendChild(openBtn);
        }

        if (isRunning) {
          var stopBtn = document.createElement("button");
          stopBtn.textContent = "Stop";
          stopBtn.className = "bg-red-900/30 hover:bg-red-800/40 border border-red-700/40 rounded px-3 py-1 text-xs text-red-400";
          stopBtn.addEventListener("click", async function () {
            try {
              await killSession(runningMap[workspace.id]);
              if (currentWorkspaceId === workspace.id) {
                disconnectWS();
                hideCurrentWorkspace();
                showWelcome();
              }
              await openWorkspaces();
            } catch (e) {
              alert("Failed to stop: " + e.message);
            }
          });
          actions.appendChild(stopBtn);
        }

        var deleteBtn = document.createElement("button");
        deleteBtn.textContent = "Delete";
        deleteBtn.className = "bg-neutral-700/50 hover:bg-neutral-600/50 border border-neutral-600 rounded px-3 py-1 text-xs text-neutral-400";
        deleteBtn.addEventListener("click", async function () {
          if (!confirm("Delete workspace '" + workspace.id + "'? All files will be lost.")) return;
          try {
            if (runningMap[workspace.id]) await killSession(runningMap[workspace.id]);
            await apiFetch(apiUrl("/workspaces/" + workspace.id), { method: "DELETE" });
            if (currentWorkspaceId === workspace.id) {
              disconnectWS();
              hideCurrentWorkspace();
              showWelcome();
            }
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
      wsList.innerHTML = '<p class="text-red-400 text-sm">' + friendlyError(e.message) + '</p>';
    }
  }

  async function resumeWorkspace(wsId, image, agent) {
    disconnectWS();
    term.reset();
    startLoading("> Opening " + wsId);

    try {
      var config = { agent: agent || "shell", workspace_id: wsId };
      if (image) config.image = image;
      var res = await apiFetch(apiUrl("/sessions"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(config),
      });
      var sess = await res.json();
      stopLoading();
      term.writeln(" ready!");
      term.writeln("");
      currentSessionId = sess.id;
      showCurrentWorkspace(wsId);
      connectWS(sess.id);
    } catch (e) {
      stopLoading();
      term.writeln("");
      term.writeln("\x1B[31m> " + friendlyError(e.message) + "\x1B[0m");
    }
  }

  function closeWorkspaces() {
    wsOverlay.classList.add("hidden");
    term.focus();
  }

  var wsDeleteAll = document.getElementById("ws-delete-all");

  wsDeleteAll.addEventListener("click", async function () {
    if (!confirm("Delete ALL workspaces? This cannot be undone.")) return;
    try {
      var res = await apiFetch(apiUrl("/workspaces"));
      var workspaces = await res.json();
      // Kill running sessions first.
      var sessRes = await apiFetch(apiUrl("/sessions"));
      var sessions = await sessRes.json();
      for (var i = 0; i < sessions.length; i++) {
        await killSession(sessions[i].id);
      }
      // Delete all volumes.
      for (var j = 0; j < workspaces.length; j++) {
        await apiFetch(apiUrl("/workspaces/" + workspaces[j].id), { method: "DELETE" });
      }
      disconnectWS();
      hideCurrentWorkspace();
      showWelcome();
      await openWorkspaces();
    } catch (e) {
      alert("Failed: " + e.message);
    }
  });

  btnWorkspaces.addEventListener("click", openWorkspaces);
  wsCloseBtn.addEventListener("click", closeWorkspaces);
  // Don't dismiss workspaces panel on overlay click — only Close or Escape.

  // --- Base64 helpers ---

  function decodeBase64(b64) {
    try {
      var raw = atob(b64);
      var buf = new Uint8Array(raw.length);
      for (var i = 0; i < raw.length; i++) buf[i] = raw.charCodeAt(i);
      return buf;
    } catch (e) {
      return null;
    }
  }

  function toBase64(str) {
    var bytes = new TextEncoder().encode(str);
    var binary = "";
    for (var i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]);
    return btoa(binary);
  }

  // --- WebSocket connection ---

  function connectWS(sessionId) {
    disconnectWS();
    var gen = ++wsGeneration;
    currentSessionId = sessionId;

    var connectTimeout = setTimeout(function () {
      if (gen === wsGeneration && ws && ws.readyState === WebSocket.CONNECTING) {
        ws.close();
        term.writeln("\x1B[31m> Connection timed out. Server may be busy.\x1B[0m");
        setStatus(false, "timeout");
      }
    }, 10000);

    ws = new WebSocket(wsUrl(sessionId));

    ws.onopen = function () {
      clearTimeout(connectTimeout);
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
      var msg;
      try { msg = JSON.parse(event.data); } catch (e) { return; }

      switch (msg.type) {
        case "pty.output":
          var buf = decodeBase64(msg.data);
          if (buf) term.write(buf);
          break;
        case "session.state":
          if (gen !== wsGeneration) return;
          setStatus(true, msg.data);
          if (msg.data === "stopped") clearAllPrompts();
          break;
        case "intercept.event":
          showPrompt(msg.data);
          break;
        case "error":
          term.writeln("\r\n\x1B[31m" + msg.data + "\x1B[0m");
          break;
      }
    };

    ws.onclose = function () {
      clearTimeout(connectTimeout);
      if (gen !== wsGeneration) return;
      setStatus(false, "disconnected");
      ws = null;
    };

    ws.onerror = function () {
      clearTimeout(connectTimeout);
      if (gen !== wsGeneration) return;
      setStatus(false, "error");
    };
  }

  function disconnectWS() {
    wsGeneration++;
    if (ws) { ws.close(); ws = null; }
    clearAllPrompts();
    setStatus(false, "disconnected");
  }

  // --- Intercept prompt UI ---

  var promptOverlay = document.getElementById("prompt-overlay");

  function showPrompt(prompt) {
    var card = document.createElement("div");
    card.className = "bg-neutral-900 border border-neutral-700 rounded-lg p-3 max-w-sm shadow-lg";

    var text = document.createElement("div");
    text.className = "text-sm text-gray-300 mb-2";
    text.textContent = prompt.text || "Confirm?";
    card.appendChild(text);

    var buttons = document.createElement("div");
    buttons.className = "flex gap-2";

    (prompt.options || ["Yes", "No"]).forEach(function (option) {
      var btn = document.createElement("button");
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
    ws.send(JSON.stringify({ type: "intercept.response", data: { id: eventId, option: option } }));
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
    disconnectWS();
    hideCurrentWorkspace();
    term.reset();
    startLoading("> Stopping workspace");

    try {
      await killSession(currentSessionId);
      stopLoading();
      term.writeln(" done.");
      term.writeln("");
      currentSessionId = null;
      showWelcome();
    } catch (e) {
      stopLoading();
      term.writeln("");
      term.writeln("\x1B[31m> " + friendlyError(e.message) + "\x1B[0m");
    }
  });

  // --- Init ---

  setStatus(false);
  showWelcome();
})();
