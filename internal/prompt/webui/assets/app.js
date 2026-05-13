"use strict";

(() => {
  const MANUAL_LABEL = "New Prompt / Unsaved";

  const state = {
    mode: "prompt",
    title: "",
    subtitle: "",
    files: [],
    selectedFiles: new Set(),
    saved: [],
    savedByLabel: new Map(),
    loadedName: "",
    loadedContent: "",
    submitting: false,
  };

  const $ = (id) => document.getElementById(id);
  const pageTitle = $("page-title");
  const headerTitle = $("header-title");
  const headerSubtitle = $("header-subtitle");
  const filesPanel = $("files-panel");
  const filesList = $("files-list");
  const filesCount = $("files-count");
  const savedSelect = $("saved-select");
  const promptText = $("prompt-text");
  const nameRow = $("name-row");
  const nameInput = $("prompt-name");
  const helper = $("helper");
  const modal = $("modal");
  const modalTitle = $("modal-title");
  const modalBody = $("modal-body");

  const btn = {
    cancel: $("btn-cancel"),
    useOnce: $("btn-use-once"),
    useSave: $("btn-use-save"),
    use: $("btn-use"),
    updateSave: $("btn-update-save"),
    save: $("btn-save"),
    delete: $("btn-delete"),
    close: $("btn-close"),
    loadFile: $("btn-load-file"),
  };

  function removeAllChildren(node) {
    while (node.firstChild) {
      node.removeChild(node.firstChild);
    }
  }

  function showModal(title, body) {
    modalTitle.textContent = title;
    modalBody.textContent = body;
    if (typeof modal.showModal === "function") {
      modal.showModal();
    } else {
      window.alert(`${title}\n\n${body}`);
    }
  }

  // showConfirmModal is like showModal but invokes onClose when dismissed.
  function showConfirmModal(title, body, onClose) {
    modalTitle.textContent = title;
    modalBody.textContent = body;
    function onceClose() {
      modal.removeEventListener("close", onceClose);
      onClose();
    }
    modal.addEventListener("close", onceClose);
    if (typeof modal.showModal === "function") {
      modal.showModal();
    } else {
      window.alert(`${title}\n\n${body}`);
      onClose();
    }
  }

  function previewOf(content) {
    const trimmed = content.replace(/\s+/g, " ").trim();
    if (trimmed.length <= 60) return trimmed;
    return trimmed.slice(0, 57) + "...";
  }

  function labelFor(prompt) {
    const preview = previewOf(prompt.content);
    return preview ? `${prompt.name} - ${preview}` : prompt.name;
  }

  function renderSavedOptions() {
    removeAllChildren(savedSelect);
    const manual = document.createElement("option");
    manual.value = MANUAL_LABEL;
    manual.textContent = MANUAL_LABEL;
    savedSelect.appendChild(manual);

    state.savedByLabel = new Map();
    for (const p of state.saved) {
      const label = labelFor(p);
      const opt = document.createElement("option");
      opt.value = label;
      opt.textContent = label;
      savedSelect.appendChild(opt);
      state.savedByLabel.set(label, p);
    }
  }

  function renderFiles() {
    if (state.mode !== "capture") {
      filesPanel.classList.add("hidden");
      return;
    }
    filesPanel.classList.remove("hidden");
    removeAllChildren(filesList);
    if (state.files.length === 0) {
      const empty = document.createElement("li");
      empty.className = "px-2 py-2 text-slate-500";
      empty.textContent = "No files discovered in the current directory.";
      filesList.appendChild(empty);
    }
    for (const path of state.files) {
      const li = document.createElement("li");
      li.className =
        "flex items-center gap-2 rounded px-2 py-1 hover:bg-slate-50";
      const cb = document.createElement("input");
      cb.type = "checkbox";
      cb.className = "h-4 w-4";
      cb.checked = state.selectedFiles.has(path);
      cb.addEventListener("change", () => {
        if (cb.checked) state.selectedFiles.add(path);
        else state.selectedFiles.delete(path);
        updateFilesCount();
      });
      const label = document.createElement("span");
      label.className = "font-mono text-xs";
      label.textContent = path;
      li.appendChild(cb);
      li.appendChild(label);
      li.addEventListener("click", (e) => {
        if (e.target === cb) return;
        cb.checked = !cb.checked;
        cb.dispatchEvent(new Event("change"));
      });
      filesList.appendChild(li);
    }
    updateFilesCount();
  }

  function updateFilesCount() {
    filesCount.textContent = `${state.selectedFiles.size} of ${state.files.length} selected`;
  }

  function refreshActions() {
    if (state.mode === "manage") {
      refreshManageActions();
    } else {
      refreshFlowActions();
    }
  }

  function refreshFlowActions() {
    const current = promptText.value.trim();
    const hasLoaded = state.loadedName !== "";
    const edited = current !== state.loadedContent;

    let showUseOnce = false;
    let showUseSave = false;
    let showUse = false;
    let showUpdate = false;
    let showName = false;
    let helperText = "";

    if (!hasLoaded) {
      showUseOnce = true;
      showUseSave = true;
      showName = true;
    } else if (!edited) {
      showUse = true;
      helperText = `Using saved prompt: ${state.loadedName}`;
    } else {
      showUse = true;
      showUpdate = true;
      helperText = `Will update saved prompt: ${state.loadedName}`;
    }

    toggle(btn.useOnce, showUseOnce);
    toggle(btn.useSave, showUseSave);
    toggle(btn.use, showUse);
    toggle(btn.updateSave, showUpdate);
    toggle(btn.save, false);
    toggle(btn.delete, false);
    toggle(btn.close, false);

    if (showName) {
      nameRow.classList.remove("hidden");
    } else {
      nameRow.classList.add("hidden");
      nameInput.value = "";
    }

    if (helperText) {
      helper.classList.remove("hidden");
      helper.textContent = helperText;
    } else {
      helper.classList.add("hidden");
      helper.textContent = "";
    }
  }

  function refreshManageActions() {
    const hasLoaded = state.loadedName !== "";

    toggle(btn.useOnce, false);
    toggle(btn.useSave, false);
    toggle(btn.use, false);
    toggle(btn.updateSave, false);
    toggle(btn.save, true);
    toggle(btn.delete, hasLoaded);
    toggle(btn.close, true);

    // Always show name row in manage mode
    nameRow.classList.remove("hidden");

    let helperText = "";
    if (hasLoaded) {
      helperText = `Managing: ${state.loadedName}`;
    } else {
      helperText = "New Prompt";
    }
    helper.classList.remove("hidden");
    helper.textContent = helperText;
  }

  function toggle(el, visible) {
    if (visible) el.classList.remove("hidden");
    else el.classList.add("hidden");
  }

  function loadSaved(label) {
    if (label === MANUAL_LABEL) {
      state.loadedName = "";
      state.loadedContent = "";
      promptText.value = "";
      nameInput.value = "";
      refreshActions();
      return;
    }
    const prompt = state.savedByLabel.get(label);
    if (!prompt) return;
    state.loadedName = prompt.name;
    state.loadedContent = (prompt.content || "").trim();
    promptText.value = prompt.content || "";
    nameInput.value = prompt.name || "";
    refreshActions();
  }

  async function submit(action) {
    if (state.submitting) return;
    state.submitting = true;
    try {
      const payload = {
        action,
        files: Array.from(state.selectedFiles),
        text: promptText.value,
        name: nameInput.value,
        loadedName: state.loadedName,
      };
      const resp = await window.gtcSubmit(payload);
      if (resp && resp.error) {
        state.submitting = false;
        showModal("Cannot submit", resp.error);
        return;
      }
      if (resp && resp.prompts) {
        // Manage mode: refresh the prompt list and stay open
        state.saved = resp.prompts;
        renderSavedOptions();
        if (action === "delete") {
          loadSaved(MANUAL_LABEL);
        } else if (action === "save" && state.loadedName === "") {
          // Just created a new prompt — try to select it
          const newName = nameInput.value.trim();
          if (newName) {
            for (const [label, p] of state.savedByLabel) {
              if (p.name === newName) {
                savedSelect.value = label;
                loadSaved(label);
                break;
              }
            }
          }
        }
        state.submitting = false;
        return;
      }
      if (resp && resp.confirm) {
        // Show success confirmation; after the user dismisses it, signal Go
        // to close the window.
        showConfirmModal("Saved", resp.confirm, () => {
          try { window.gtcClose(); } catch (_) {}
        });
        return;
      }
      // useOnce / use / close — window closes automatically from the Go side.
    } catch (e) {
      state.submitting = false;
      showModal("Error", String(e));
    }
  }

  function updateHeader() {
    if (state.mode === "manage") {
      pageTitle.textContent = state.title || "Prompt Manager";
      headerTitle.textContent = state.title || "Prompt Manager";
      headerSubtitle.textContent = state.subtitle || "Create, edit, and delete saved prompts.";
    } else {
      pageTitle.textContent = state.title || "Prompt";
      headerTitle.textContent = state.title || "Provide Instructions";
      headerSubtitle.textContent = state.subtitle || "Pick or write the prompt that will be used.";
    }
  }

  async function init() {
    let payload;
    try {
      payload = await window.gtcInit();
    } catch (e) {
      showModal("Initialization failed", String(e));
      return;
    }
    state.mode = payload.mode || "prompt";
    state.title = payload.title || "";
    state.subtitle = payload.subtitle || "";
    state.files = Array.isArray(payload.availableFiles)
      ? payload.availableFiles
      : [];
    state.selectedFiles = new Set(state.files);
    state.saved = Array.isArray(payload.savedPrompts)
      ? payload.savedPrompts
      : [];

    updateHeader();
    renderSavedOptions();
    renderFiles();
    refreshActions();

    savedSelect.addEventListener("change", (e) =>
      loadSaved(e.target.value),
    );
    promptText.addEventListener("input", refreshActions);

    btn.cancel.addEventListener("click", () => {
      try {
        window.gtcCancel();
      } catch (_) {}
    });
    for (const key of ["useOnce", "useSave", "use", "updateSave"]) {
      btn[key].addEventListener("click", () =>
        submit(btn[key].dataset.action),
      );
    }
    for (const key of ["save", "close"]) {
      btn[key].addEventListener("click", () =>
        submit(btn[key].dataset.action),
      );
    }
    btn.delete.addEventListener("click", () => {
      if (!state.loadedName) return;
      if (window.confirm(`Delete prompt "${state.loadedName}"?`)) {
        submit("delete");
      }
    });

    if (btn.loadFile) {
      btn.loadFile.addEventListener("click", () => {
        const input = document.createElement("input");
        input.type = "file";
        input.accept = ".txt";
        input.addEventListener("change", (e) => {
          const file = e.target.files[0];
          if (!file) return;
          const reader = new FileReader();
          reader.onload = () => {
            promptText.value = String(reader.result || "");
            state.loadedName = "";
            state.loadedContent = "";
            nameInput.value = "";
            refreshActions();
          };
          reader.readAsText(file);
        });
        input.click();
      });
    }

    const filesAll = $("files-all");
    const filesNone = $("files-none");
    filesAll.addEventListener("click", () => {
      state.selectedFiles = new Set(state.files);
      renderFiles();
    });
    filesNone.addEventListener("click", () => {
      state.selectedFiles = new Set();
      renderFiles();
    });

    window.addEventListener("keydown", (e) => {
      if (e.key === "Escape") {
        e.preventDefault();
        try {
          window.gtcCancel();
        } catch (_) {}
      }
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
