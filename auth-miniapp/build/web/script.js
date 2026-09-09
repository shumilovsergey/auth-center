/* auth-miniapp — the Telegram half of an auth-center login.

   The page has exactly two endings. Either the identity checked out, in which
   case it greets the user and offers the way back into the app; or it did not,
   in which case it says so and shows the technical reason underneath, because
   a user who reports "it didn't work" with nothing else is a user nobody can
   help.

   Two rules carried over from the PoC this grew out of:
     - the SDK is loaded with a timeout, never as a blocking <script>, because
       telegram.org is not reachable from every network and a blank page
       teaches nobody anything;
     - nothing is reported only to the console — the Telegram in-app browser
       has no console a phone user can open. */

const SDK_SOURCES = ['/telegram-web-app.js', 'https://telegram.org/js/telegram-web-app.js'];
const SDK_TIMEOUT = 5000;
const AUTH_TIMEOUT = 10000;

// Telegram clients that are a desktop app or already a browser tab. Everything
// else — phones, and anything unrecognised — is treated as "stay inside
// Telegram", which is the safe half: a link opened in place always works,
// whereas handing a phone user to an external browser strands them outside the
// app they came from.
//
// The split is desktop vs everything, not desktop vs mobile, on purpose: an
// unknown platform is far more likely to be a new phone client than a new
// desktop one, and guessing wrong that way costs nothing.
const DESKTOP_PLATFORMS = ['tdesktop', 'macos', 'weba', 'webk', 'web'];

// Longest greeting the card takes on one line. The card is 380px wide with 28px
// padding, and the font is 13px monospace — about 37 characters fit, of which
// "привет, " spends 8. Anything longer wraps and pushes the button around, so
// it is cut instead: a name is an identity check, not a document.
const NAME_MAX = 24;

const el = (id) => document.getElementById(id);

let tg = null;

// ── the two endings ───────────────────────────────────────────────────────

// signedIn greets the user and offers one way onward. The button carries a
// one-time code of this page's own, so the app signs the user in on arrival —
// this webview does not necessarily share cookies with the browser tab that
// started the login.
// clip keeps the card one line tall whatever Telegram reports as a name.
function clip(text) {
  return text.length > NAME_MAX ? text.slice(0, NAME_MAX - 1) + '…' : text;
}

function signedIn(data) {
  const who = data.name || (data.username ? '@' + data.username : `id ${data.user_id}`);
  el('status').textContent = `привет, ${clip(who)}`;

  // No code means no way back worth offering: either the session carried no
  // redirect, or the user got here by scanning the QR and the browser waiting
  // for this login is on another machine entirely.
  if (!data.redirect || !data.code) return;

  const sep = data.redirect.includes('?') ? '&' : '?';
  const back = `${data.redirect}${sep}code=${data.code}`;

  const action = el('action');
  action.textContent = 'НАЗАД';
  action.hidden = false;
  action.addEventListener('click', () => goBack(back));
}

// goBack sends the user onward by the route that suits the client they are on.
//
// On a phone, navigating this webview keeps them inside Telegram, where they
// already are and where the app they came from is a collapsed tab. Handing that
// user to WebApp.openLink() would throw them out into Safari or Chrome — the
// method is documented to open an EXTERNAL browser, and it does.
//
// On a desktop client that same external browser is exactly right: the user
// already has the app open in a real browser window, and keeping them in
// Telegram's built-in view leaves them with two copies in two places.
function goBack(url) {
  const desktop = DESKTOP_PLATFORMS.includes(tg?.platform);
  console.log('[auth-miniapp] platform', tg?.platform, desktop ? '→ browser' : '→ stay in telegram');

  if (desktop && tg?.openLink) {
    tg.openLink(url);
    return;
  }
  window.location.href = url;
}

// failed says the plain thing, then the useful thing. reason is whatever the
// server or the network actually reported; it is not translated, because its
// audience is whoever reads the screenshot afterwards.
function failed(reason) {
  const status = el('status');
  status.className = 'card-title error';
  status.textContent = 'не удалось войти';

  if (reason) {
    const detail = el('detail');
    detail.textContent = reason;
    detail.hidden = false;
  }

  const action = el('action');
  action.textContent = 'ЕЩЁ РАЗ';
  action.hidden = false;
  action.addEventListener('click', () => location.reload());
}

// ── loading the SDK ───────────────────────────────────────────────────────

function loadScript(src, budget) {
  return new Promise((resolve) => {
    const s = document.createElement('script');
    let settled = false;
    const done = (how) => {
      if (settled) return;
      settled = true;
      resolve(how);
    };
    const timer = setTimeout(() => done('timeout'), budget);
    s.onload = () => { clearTimeout(timer); done('loaded'); };
    s.onerror = () => { clearTimeout(timer); done('error'); };
    s.src = src;
    document.head.appendChild(s);
  });
}

async function loadSDK() {
  for (const src of SDK_SOURCES) {
    const how = await loadScript(src, SDK_TIMEOUT);
    // onload also fires for a response that is not the SDK at all — a 404 body
    // or a captive portal page — so the global is what decides.
    if (how === 'loaded' && window.Telegram?.WebApp) return true;
  }
  return false;
}

// ── the flow ──────────────────────────────────────────────────────────────

async function postAuth(initData) {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), AUTH_TIMEOUT);
  try {
    const r = await fetch('/api/auth', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ init_data: initData }),
      signal: ctrl.signal,
    });
    return await r.json();
  } catch (e) {
    return { ok: false, error: e.name === 'AbortError'
      ? `сервер не ответил за ${AUTH_TIMEOUT / 1000} с`
      : String(e) };
  } finally {
    clearTimeout(timer);
  }
}

async function run() {
  if (!await loadSDK()) {
    failed('telegram-web-app.js не загрузился — страница должна открываться внутри Telegram');
    return;
  }

  tg = window.Telegram.WebApp;
  tg.ready();
  tg.expand();

  const initData = tg.initData || '';
  if (!initData) {
    failed('нет initData — страница открыта не по ссылке входа');
    return;
  }

  const data = await postAuth(initData);

  if (!data.ok) {
    console.warn('[auth-miniapp]', data.error);
    failed(data.error);
    return;
  }

  console.log('[auth-miniapp] signed in as', data.user_id);
  signedIn(data);
}

run();
