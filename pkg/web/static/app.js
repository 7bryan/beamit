const isHost =
  location.hostname === "localhost" || location.hostname === "127.0.0.1";

const els = {
  dropZone: document.getElementById("drop-zone"),
  fileInput: document.getElementById("file-input"),
  status: document.getElementById("status"),
  modal: document.getElementById("consent-modal"),
  modalText: document.getElementById("consent-text"),
  acceptBtn: document.getElementById("accept-btn"),
  rejectBtn: document.getElementById("reject-btn"),
  modeLabel: document.getElementById("mode-label"),
};

function setStatus(text) {
  els.status.textContent = text;
}

function formatBytes(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 ** 2).toFixed(1)} MB`;
}

function getAutoDeviceName() {
  const ua = navigator.userAgent;
  let os = "Device";

  if (/iPhone/i.test(ua)) os = "iPhone";
  else if (/iPad/i.test(ua)) os = "iPad";
  else if (/Android/i.test(ua)) {
    // Tries to extract actual model name (e.g., "Pixel 7" or "SM-G991B")
    const match = ua.match(/Android[^;]+; ([^;)]+)/);
    os =
      match && match[1] && !match[1].includes("Build")
        ? match[1]
        : "Android Device";
  } else if (/Macintosh/i.test(ua)) os = "Mac";
  else if (/Windows/i.test(ua)) os = "Windows PC";
  else if (/Linux/i.test(ua)) os = "Linux PC";

  return os;
}

function deviceName() {
  return localStorage.getItem("beamit-name") || getAutoDeviceName();
}

async function pollUntilResolved(id) {
  for (let i = 0; i < 60; i++) {
    await new Promise((r) => setTimeout(r, 1000));
    const res = await fetch(`/request-status?id=${id}`);
    if (!res.ok) return null;
    const req = await res.json();
    if (req.Status === "accepted") return req.Token;
    if (req.Status === "rejected") return null;
  }
  return null; // timed out waiting for a response
}

if (isHost) {
  els.modeLabel.textContent = "Sharing from this laptop";

  ["dragover", "drop"].forEach((evt) =>
    els.dropZone.addEventListener(evt, (e) => e.preventDefault()),
  );
  els.dropZone.addEventListener("drop", (e) => {
    const file = e.dataTransfer.files[0];
    if (file) shareFile(file);
  });
  els.fileInput.addEventListener("change", (e) => {
    const file = e.target.files[0];
    if (file) shareFile(file);
  });

  async function shareFile(file) {
    setStatus(`Sharing '${file.name}'...`);
    const form = new FormData();
    form.append("file", file);
    const res = await fetch("/share", { method: "POST", body: form });
    setStatus(res.ok ? `Sharing '${file.name}'` : "Failed to share file");
  }

  let activeRequestId = null;
  setInterval(async () => {
    if (activeRequestId) return;
    const res = await fetch("/pending");
    const pending = await res.json();
    if (pending && pending.length > 0) {
      showConsentModal(pending[0]);
    }
  }, 1500);

  function showConsentModal(req) {
    activeRequestId = req.ID;
    const verb =
      req.Type === "upload" ? "wants to send you" : "wants to download";
    els.modalText.textContent = `${req.FromDevice} ${verb} '${req.FileName}' (${formatBytes(req.FileSize)})`;
    els.modal.classList.remove("hidden");

    const respond = async (accept) => {
      await fetch("/respond", {
        method: "POST",
        body: JSON.stringify({ id: req.ID, accept }),
      });
      els.modal.classList.add("hidden");
      activeRequestId = null;
    };
    els.acceptBtn.onclick = () => respond(true);
    els.rejectBtn.onclick = () => respond(false);
  }
} else {
  els.modeLabel.textContent = "Connected to BeamIt";

  (async function checkShared() {
    const res = await fetch("/manifest");
    if (res.status === 204) {
      setStatus("Nothing is currently being shared.");
      return;
    }
    const manifest = await res.json();
    setStatus(
      `Available: ${manifest.file_name} (${formatBytes(manifest.file_size)})`,
    );
    const btn = document.createElement("button");
    btn.textContent = `Request '${manifest.file_name}'`;
    btn.onclick = requestDownload;
    els.status.appendChild(document.createElement("br"));
    els.status.appendChild(btn);
  })();

  async function requestDownload() {
    setStatus("Requesting — waiting for the laptop to accept...");
    const res = await fetch("/request-download", {
      method: "POST",
      body: JSON.stringify({ from_device: deviceName() }),
    });
    const req = await res.json();
    const token = await pollUntilResolved(req.ID);
    if (token) {
      setStatus("Accepted — downloading...");
      window.location.href = `/download?token=${token}`;
    } else {
      setStatus("Request was declined or timed out.");
    }
  }

  els.dropZone.addEventListener("dragover", (e) => e.preventDefault());
  els.dropZone.addEventListener("drop", (e) => {
    e.preventDefault();
    const file = e.dataTransfer.files[0];
    if (file) requestUpload(file);
  });
  els.fileInput.addEventListener("change", (e) => {
    const file = e.target.files[0];
    if (file) requestUpload(file);
  });

  async function requestUpload(file) {
    setStatus(`Requesting to send '${file.name}'...`);
    const res = await fetch("/request-upload", {
      method: "POST",
      body: JSON.stringify({
        from_device: deviceName(),
        file_name: file.name,
        file_size: file.size,
      }),
    });
    const req = await res.json();
    const token = await pollUntilResolved(req.ID);
    if (!token) {
      setStatus("Request was declined or timed out.");
      return;
    }
    uploadWithProgress(file, token);
  }

  function uploadWithProgress(file, token) {
    const form = new FormData();
    form.append("file", file);
    const xhr = new XMLHttpRequest();
    xhr.open("POST", `/share?token=${token}`);
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        setStatus(`Uploading... ${Math.round((e.loaded / e.total) * 100)}%`);
      }
    };
    xhr.onload = () =>
      setStatus(xhr.status === 200 ? "Sent!" : "Upload failed.");
    xhr.send(form);
  }
}

if ("serviceWorker" in navigator) {
  navigator.serviceWorker.register("/sw.js").catch(() => {});
}
