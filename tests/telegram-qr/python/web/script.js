let pollInterval = null;
let pollTimeout = null;
let currentToken = null;

const POLL_LIMIT = 20000; // 20 seconds

async function startSession() {
  clearInterval(pollInterval);
  clearTimeout(pollTimeout);

  const res = await fetch("/qr-session", { method: "POST" });
  const { token, qr, ttl, url } = await res.json();
  currentToken = token;

  const img = document.getElementById("qr-img");
  img.src = `data:image/png;base64,${qr}`;
  img.className = "";

  const btn = document.getElementById("open-btn");
  btn.href = url;
  btn.className = "open-btn";

  const result = document.getElementById("result");
  result.className = "hidden";
  result.innerHTML = "";

  pollInterval = setInterval(() => poll(token), 2000);

  // silently stop polling after 20s and offer a new QR
  pollTimeout = setTimeout(() => {
    clearInterval(pollInterval);
    document.getElementById("open-btn").className = "open-btn hidden";
    const result = document.getElementById("result");
    result.className = "error";
    result.innerHTML =
      `Not authenticated yet.<br><br>` +
      `<button class="refresh-btn" onclick="startSession()">Generate New QR</button>`;
  }, POLL_LIMIT);
}

async function poll(token) {
  const res = await fetch(`/poll/${token}`);
  const data = await res.json();

  if (data.status === "authenticated") {
    clearInterval(pollInterval);
    clearTimeout(pollTimeout);

    const u = data.user;
    const name = [u.first_name, u.last_name].filter(Boolean).join(" ");

    const result = document.getElementById("result");
    result.className = "success";
    result.innerHTML =
      `Welcome, <strong>${name}</strong>!` +
      (u.username ? `<br>@${u.username}` : "") +
      `<br><br><strong>Telegram ID:</strong> ${u.id}`;

    document.getElementById("qr-img").className = "expired";
    document.getElementById("open-btn").className = "open-btn hidden";
  }

  if (data.status === "expired") {
    clearInterval(pollInterval);

    const result = document.getElementById("result");
    result.className = "error";
    result.innerHTML =
      `QR code expired.<br><br>` +
      `<button class="refresh-btn" onclick="startSession()">Generate New QR</button>`;

    document.getElementById("qr-img").className = "expired";
    document.getElementById("open-btn").className = "open-btn hidden";
  }
}

// start on page load
startSession();
