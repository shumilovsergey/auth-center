async function onTelegramAuth(user) {
  const result = document.getElementById("result");
  result.className = "";
  result.textContent = "Verifying...";

  try {
    const res = await fetch("/auth", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(user),
    });

    const data = await res.json();

    if (res.ok && data.ok) {
      result.className = "success";
      result.textContent =
        `Welcome, ${data.user.first_name}!` +
        (data.user.username ? ` (@${data.user.username})` : "");
    } else {
      result.className = "error";
      result.textContent = "Auth failed: " + (data.error || "unknown error");
    }
  } catch (err) {
    result.className = "error";
    result.textContent = "Network error: " + err.message;
  }
}
