// ── tile selection ────────────────────────────────────────────────────────

let activeMethod = null;

function selectMethod(method) {
  const prev = activeMethod;

  document.querySelectorAll('.tile').forEach(t => t.classList.remove('active'));
  document.querySelectorAll('.section').forEach(s => s.classList.remove('open'));

  if (prev === method) {
    activeMethod = null;
    return;
  }

  activeMethod = method;
  document.getElementById(`tile-${method}`).classList.add('active');
  document.getElementById(`section-${method}`).classList.add('open');

  if (method === 'tg') startSession();

  if (method === 'google') {
    const r = window.REDIRECT_URL || '';
    const href = '/google/login' + (r ? '?redirect=' + encodeURIComponent(r) : '');
    document.getElementById('google-btn').href = href;
  }
}

// ── shared ────────────────────────────────────────────────────────────────

function showResult(type, text) {
  const el = document.getElementById('result');
  el.className = 'result ' + type;
  el.textContent = text;
}

function lockAll() {
  document.querySelectorAll('.tile').forEach(t => t.disabled = true);
}

function navigateWithCode(redirectUrl, code) {
  const sep = redirectUrl.includes('?') ? '&' : '?';
  window.location.href = `${redirectUrl}${sep}code=${code}`;
}

// ── telegram QR ───────────────────────────────────────────────────────────

let pollInterval = null;
let pollTimeout  = null;

async function startSession() {
  clearInterval(pollInterval);
  clearTimeout(pollTimeout);

  // reset state
  document.getElementById('qr-area').classList.remove('hidden');
  document.getElementById('tg-refresh').style.display = 'none';
  document.getElementById('result').className = 'result';

  const { token, qr, url } = await fetch('/qr-session', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ redirect: window.REDIRECT_URL || '' }),
  }).then(r => r.json());

  document.getElementById('qr-img').src = `data:image/png;base64,${qr}`;
  document.getElementById('open-btn').href = url;

  pollInterval = setInterval(() => poll(token), 2000);

  // after 20s: collapse QR area, show refresh button
  pollTimeout = setTimeout(() => {
    clearInterval(pollInterval);
    document.getElementById('qr-area').classList.add('hidden');
    document.getElementById('tg-refresh').style.display = 'block';
  }, 20000);
}

async function poll(token) {
  const data = await fetch(`/poll/${token}`).then(r => r.json());

  if (data.status === 'authenticated') {
    clearInterval(pollInterval);
    clearTimeout(pollTimeout);

    if (data.redirect && data.code) {
      navigateWithCode(data.redirect, data.code);
      return;
    }

    lockAll();
    const u    = data.user;
    const name = [u.first_name, u.last_name].filter(Boolean).join(' ');
    showResult('success',
      `telegram\n` +
      `${name}${u.username ? ' (@' + u.username + ')' : ''}\n` +
      `id: ${u.id}`
    );
  }

  if (data.status === 'expired') {
    clearInterval(pollInterval);
    document.getElementById('qr-area').classList.add('hidden');
    document.getElementById('tg-refresh').style.display = 'block';
  }
}


// ── solana ────────────────────────────────────────────────────────────────

function getProvider() {
  if (window.phantom?.solana?.isPhantom) return { provider: window.phantom.solana, name: 'Phantom' };
  if (window.solflare?.isSolflare)        return { provider: window.solflare,       name: 'Solflare' };
  if (window.solana)                      return { provider: window.solana,          name: 'Solana Wallet' };
  return null;
}

function uint8ToBase64(bytes) {
  let bin = '';
  for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
  return btoa(bin);
}

async function connectWallet() {
  const btn      = document.getElementById('solana-btn');
  const noWallet = document.getElementById('no-wallet');
  const wallet   = getProvider();

  noWallet.classList.remove('visible');
  btn.classList.remove('invalid');

  if (!wallet) {
    btn.classList.add('invalid');
    noWallet.classList.add('visible');
    return;
  }

  btn.disabled    = true;
  btn.textContent = 'connecting...';

  try {
    await wallet.provider.connect();
    const publicKey = wallet.provider.publicKey.toBase58();

    btn.textContent = 'signing...';

    const { nonce, error: nonceErr } = await fetch('/solana/nonce', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ public_key: publicKey }),
    }).then(r => r.json());
    if (nonceErr) throw new Error(nonceErr);

    const signed = await wallet.provider.signMessage(new TextEncoder().encode(nonce), 'utf8');

    const data = await fetch('/solana/auth', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        public_key: publicKey,
        signature:  uint8ToBase64(new Uint8Array(signed.signature)),
        nonce,
        redirect:   window.REDIRECT_URL || '',
      }),
    }).then(r => r.json());

    if (data.ok) {
      if (data.redirect && data.code) {
        navigateWithCode(data.redirect, data.code);
        return;
      }

      lockAll();
      showResult('success',
        `solana (${wallet.name})\n${data.public_key}`
      );
      btn.textContent = 'connected';
    } else {
      throw new Error(data.error || 'auth failed');
    }
  } catch (err) {
    showResult('error', err.message);
    btn.disabled    = false;
    btn.textContent = 'connect wallet';
  }
}
