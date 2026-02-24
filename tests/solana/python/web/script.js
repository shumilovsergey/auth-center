function getProvider() {
  if (window.phantom?.solana?.isPhantom) {
    return { provider: window.phantom.solana, name: "Phantom" };
  }
  if (window.solflare?.isSolflare) {
    return { provider: window.solflare, name: "Solflare" };
  }
  if (window.solana) {
    return { provider: window.solana, name: "Solana Wallet" };
  }
  return null;
}

function uint8ToBase64(bytes) {
  let binary = "";
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

function showResult(type, html) {
  const el = document.getElementById("result");
  el.className = type;
  el.innerHTML = html;
}

async function connectWallet() {
  const btn = document.getElementById("connect-btn");
  const wallet = getProvider();

  if (!wallet) {
    showResult("error", "No Solana wallet found.<br>Install <a href='https://phantom.app' target='_blank' style='color:#9945ff'>Phantom</a> or <a href='https://solflare.com' target='_blank' style='color:#9945ff'>Solflare</a>.");
    return;
  }

  btn.disabled = true;
  btn.textContent = "Connecting...";

  try {
    // 1. connect wallet
    await wallet.provider.connect();
    const publicKey = wallet.provider.publicKey.toBase58();

    btn.textContent = "Signing...";

    // 2. get nonce from server
    const nonceRes = await fetch("/nonce", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ public_key: publicKey }),
    });
    const { nonce, error: nonceErr } = await nonceRes.json();
    if (nonceErr) throw new Error(nonceErr);

    // 3. sign nonce with wallet
    const encoded = new TextEncoder().encode(nonce);
    const signed = await wallet.provider.signMessage(encoded, "utf8");

    // 4. send to server for verification
    const authRes = await fetch("/auth", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        public_key: publicKey,
        signature: uint8ToBase64(new Uint8Array(signed.signature)),
        nonce: nonce,
      }),
    });
    const data = await authRes.json();

    if (authRes.ok && data.ok) {
      showResult(
        "success",
        `Authenticated via ${wallet.name}<br><br>` +
        `<strong>Wallet address (public key):</strong><br>${data.public_key}`
      );
      btn.textContent = "Connected";
    } else {
      throw new Error(data.error || "auth failed");
    }
  } catch (err) {
    showResult("error", "Error: " + err.message);
    btn.disabled = false;
    btn.textContent = "Connect Wallet";
  }
}
