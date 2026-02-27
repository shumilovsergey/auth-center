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

const IS_MOBILE  = /Android|iPhone|iPad/i.test(navigator.userAgent);
const IS_ANDROID = /Android/i.test(navigator.userAgent);

// base58 encode — needed for Wallet Standard (returns raw Uint8Array public keys)
const B58 = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';
function toBase58(bytes) {
  let n = 0n;
  for (const b of bytes) n = (n << 8n) | BigInt(b);
  let s = '';
  while (n > 0n) { s = B58[Number(n % 58n)] + s; n /= 58n; }
  for (const b of bytes) { if (b !== 0) break; s = '1' + s; }
  return s;
}

function uint8ToBase64(bytes) {
  let bin = '';
  for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
  return btoa(bin);
}

async function fetchNonce(publicKey) {
  return fetch('/solana/nonce', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ public_key: publicKey }),
  }).then(r => r.json());
}

// ── path 1: injected provider (desktop extension / wallet in-app browser) ─

function getInjectedProvider() {
  if (window.phantom?.solana?.isPhantom) return { wallet: window.phantom.solana, name: 'Phantom' };
  if (window.solflare?.isSolflare)        return { wallet: window.solflare,       name: 'Solflare' };
  if (window.solana)                      return { wallet: window.solana,          name: 'Solana Wallet' };
  return null;
}

async function signWithInjected(injected, btn) {
  await injected.wallet.connect();
  const publicKey = injected.wallet.publicKey.toBase58();

  btn.textContent = 'signing...';
  const { nonce, error } = await fetchNonce(publicKey);
  if (error) throw new Error(error);

  const signed = await injected.wallet.signMessage(new TextEncoder().encode(nonce), 'utf8');
  return {
    publicKey,
    signature:  uint8ToBase64(new Uint8Array(signed.signature)),
    nonce,
    walletName: injected.name,
  };
}

// ── path 2: wallet standard (broader wallet support, same in-app browsers) ─

async function findStandardWallet() {
  try {
    const { getWallets } = await import('https://esm.sh/@wallet-standard/app@1');
    const { get } = getWallets();
    return get().find(w =>
      w.chains.some(c => c.startsWith('solana:')) &&
      'standard:connect'   in w.features &&
      'solana:signMessage' in w.features
    ) || null;
  } catch {
    return null;
  }
}

async function signWithStandard(wallet, btn) {
  await wallet.features['standard:connect'].connect();
  const account = wallet.accounts[0];
  if (!account) throw new Error('wallet connected but no account');
  const publicKey = toBase58(account.publicKey);

  btn.textContent = 'signing...';
  const { nonce, error } = await fetchNonce(publicKey);
  if (error) throw new Error(error);

  const [result] = await wallet.features['solana:signMessage'].signMessage({
    account,
    message: new TextEncoder().encode(nonce),
  });
  return {
    publicKey,
    signature:  uint8ToBase64(result.signature),
    nonce,
    walletName: wallet.name,
  };
}

// ── path 3: mobile wallet adapter (android — saga 2 / seed vault / phantom) ─
//
// MWA connects via a local WebSocket to the wallet app using Android intents.
// Works in any Android browser (Chrome, Firefox, etc.) without an extension.
//
// Signed payload format: message_bytes || signature_bytes
// → signature = last 64 bytes of the returned Uint8Array

async function signWithMWA(btn) {
  const { transact } = await import('https://esm.sh/@solana-mobile/mobile-wallet-adapter-protocol@2');

  return await transact(async (wallet) => {
    btn.textContent = 'opening wallet...';

    const authResult = await wallet.authorize({
      chain: 'solana:mainnet',
      identity: {
        name: 'Auth Center',
        uri:  window.location.origin,
        icon: `${window.location.origin}/favicon.svg`,
      },
    });

    const publicKey = authResult.accounts[0].address; // base58

    btn.textContent = 'signing...';
    const { nonce, error } = await fetchNonce(publicKey);
    if (error) throw new Error(error);

    const signedPayloads = await wallet.signMessages({
      addresses: [publicKey],
      payloads:  [new TextEncoder().encode(nonce)],
    });

    // signed payload = message_bytes || signature (last 64 bytes)
    const signedBytes = signedPayloads[0];
    const sigBytes    = signedBytes.slice(-64);

    return {
      publicKey,
      signature:  uint8ToBase64(sigBytes),
      nonce,
      walletName: 'MWA Wallet',
    };
  });
}

// ── path 4: no wallet on mobile — open page inside Phantom's in-app browser ─

function showPhantomLink() {
  const url  = window.location.href;
  const link = `https://phantom.app/ul/browse/${encodeURIComponent(url)}?ref=${encodeURIComponent(window.location.origin)}`;
  const el   = document.getElementById('phantom-link');
  el.href    = link;
  el.style.display = 'inline-block';
}

// ── connect wallet ────────────────────────────────────────────────────────

async function connectWallet() {
  const btn       = document.getElementById('solana-btn');
  const noWallet  = document.getElementById('no-wallet');
  const phantomEl = document.getElementById('phantom-link');

  noWallet.classList.remove('visible');
  phantomEl.style.display = 'none';
  btn.classList.remove('invalid');
  btn.disabled    = true;
  btn.textContent = 'connecting...';

  try {
    let result = null;

    // 1. injected provider (desktop extension / wallet in-app browser)
    const injected = getInjectedProvider();
    if (injected) {
      result = await signWithInjected(injected, btn);
    }

    // 2. wallet standard
    if (!result) {
      btn.textContent = 'looking for wallet...';
      const stdWallet = await findStandardWallet();
      if (stdWallet) result = await signWithStandard(stdWallet, btn);
    }

    // 3. MWA — android only (saga 2, seed vault, phantom android)
    if (!result && IS_ANDROID) {
      try {
        result = await signWithMWA(btn);
      } catch {
        // MWA not available or user cancelled — fall through to deep link
      }
    }

    // 4. no wallet found
    if (!result) {
      btn.classList.add('invalid');
      btn.disabled    = false;
      btn.textContent = 'connect wallet';
      IS_MOBILE ? showPhantomLink() : noWallet.classList.add('visible');
      return;
    }

    // submit
    const data = await fetch('/solana/auth', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        public_key: result.publicKey,
        signature:  result.signature,
        nonce:      result.nonce,
        redirect:   window.REDIRECT_URL || '',
      }),
    }).then(r => r.json());

    if (data.ok) {
      if (data.redirect && data.code) {
        navigateWithCode(data.redirect, data.code);
        return;
      }
      lockAll();
      showResult('success', `solana (${result.walletName})\n${data.public_key}`);
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
