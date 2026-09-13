/* auth-miniapp — the Telegram half of an auth-center login.

   The page has exactly two endings. Either the identity checked out, in which
   case it greets the user and offers the way back into the app; or it did not,
   in which case it says so and shows the technical reason underneath, because
   a user who reports "it didn't work" with nothing else is a user nobody can
   help.

   The way back is a button, and the button is what the page falls back to.
   The tap is not there to ask permission — it is there because every automatic
   route out of a mini app can be refused by the client: on the browser-based
   Telegram clients WebApp.openLink() comes down to window.open, and a
   window.open with no user gesture behind it is exactly what popup blockers
   exist to stop. So the button is shown first, always, and only then does the
   page try to take that route itself. Whatever the client does with the
   attempt, the user is already looking at a screen that works.

   What each ending tries, and what is left standing when the try fails:

     - phone (and every unrecognised platform): navigate this webview to the
       app. If the navigation takes, this document is torn down — `pagehide` is
       both the only proof it happened and the last moment we can still run
       anything, so the close() lives there. If nothing moves, the greeting and
       the button stay exactly as they were.

     - desktop: hand the link to the real browser with openLink(), then close.
       The close is armed only for tdesktop and macos, where openLink is a
       native call: on weba/webk/web it is window.open, a blocked popup looks
       from here exactly like a successful one, and closing on that guess would
       leave somebody with neither a tab nor a mini app. Those clients keep the
       button instead.

     - QR: close, and nothing else. The browser waiting for this login is on
       another machine, so there is no link that would help here and no button
       worth showing. A close() that the client ignores leaves the name on
       screen, which is the right thing to be left with.

   Closing after a jump is deliberate and it is the one part here that trades
   safety for tidiness: on a phone the close() lands on the page just navigated
   to. Both live behind AUTO_* constants below so either can be switched off
   without touching the flow.

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

// The desktop clients that are a real application rather than a browser tab.
// There openLink() is a native call the client either performs or refuses out
// loud, so a close() after it is safe. On weba/webk/web the same call is a
// window.open a blocker can swallow in silence — see the header.
const NATIVE_DESKTOP = ['tdesktop', 'macos'];

// Longest name the frame takes on one line. The card is 380px wide with 28px
// padding and the frame another 18px, and the font is 13px monospace — about 33
// characters fit. Anything longer wraps and pushes the button around, so it is
// cut instead: a name here is an identity check, not a document.
//
// It used to be 24, back when the frame also carried a "привет, " in front of
// the name. The caption moved out to a line of its own and gave the name the
// full width.
const NAME_MAX = 32;

// How long the card is left alone before the page tries to finish by itself.
// Long enough for the greeting to paint and be read as an answer, short enough
// that nobody has started reaching for the button yet.
const AUTO_DELAY_MS = 450;

// How long after openLink() the desktop close waits. The client needs a moment
// to raise its "open this link?" prompt, and closing out from under that prompt
// cancels the very thing we just asked for.
const AUTO_CLOSE_MS = 1200;

// A pagehide this long after the automatic jump is taken as that jump landing.
// Later than this and it is the user leaving on their own — by the button, or
// by backgrounding Telegram — and their exit is not ours to tidy up after.
const AUTO_PAGEHIDE_MS = 3000;

// How long the name stays up on the QR ending before the page closes itself.
// That ending has no button and no destination, so this card is the entire
// receipt for the login on this device — long enough to read a name, which is
// the only place the user is told WHICH account just signed in, and short
// enough that nobody starts looking for the close button.
const QR_CLOSE_MS = 1600;

const el = (id) => document.getElementById(id);

let tg = null;

// ── the endings ───────────────────────────────────────────────────────────

// clip keeps the card one line tall whatever Telegram reports as a name.
function clip(text) {
  return text.length > NAME_MAX ? text.slice(0, NAME_MAX - 1) + '…' : text;
}

// signedIn names the user and offers one way onward. The button carries a
// one-time code of this page's own, so the app signs the user in on arrival —
// this webview does not necessarily share cookies with the browser tab that
// started the login.
//
// The name is the whole point of this screen: it is the only place anybody is
// told WHICH account just signed in, and on a phone with two Telegram accounts
// that is not a rhetorical question. Hence a caption that says what the frame
// below it means, rather than a greeting that buries the answer in a sentence.
function signedIn(data) {
  const who = data.name || (data.username ? '@' + data.username : `id ${data.user_id}`);

  el('label').hidden = false;
  const status = el('status');
  status.classList.add('name');
  status.textContent = clip(who);

  // No code means there is nowhere on this device to send anyone: either the
  // session carried no redirect, or the user got here by scanning the QR and
  // the browser waiting for this login is on another machine entirely. That
  // browser is already polling and will finish without us, so a button here
  // would lead to the wrong device — the page shows who signed in and gets out
  // of the way instead. A close the client ignores costs nothing: the card
  // keeps the name up, which is all this ending was going to leave behind.
  if (!data.redirect || !data.code) {
    setTimeout(closeApp, QR_CLOSE_MS);
    return;
  }

  const sep = data.redirect.includes('?') ? '&' : '?';
  const back = `${data.redirect}${sep}code=${data.code}`;

  // The button goes up before anything is attempted, so a refused attempt
  // changes nothing about what the user is looking at.
  const action = el('action');
  action.textContent = 'ДАЛЕЕ';
  action.hidden = false;
  action.addEventListener('click', () => goBack(back));

  setTimeout(() => autoOnward(back), AUTO_DELAY_MS);
}

// autoOnward does by itself what the button does when tapped, and closes the
// mini app if it can tell the jump landed. It is the same goBack routing —
// deliberately not goBack() itself, because each route needs a different proof
// that it worked, and that proof is the whole point of this function.
function autoOnward(url) {
  const platform = tg?.platform;

  if (DESKTOP_PLATFORMS.includes(platform)) {
    if (!openExternally(url)) {
      console.warn('[auth-miniapp] auto openLink refused — button stands');
      return;
    }
    if (NATIVE_DESKTOP.includes(platform)) setTimeout(closeApp, AUTO_CLOSE_MS);
    return;
  }

  // A phone stays inside Telegram, so the jump replaces this document. Arm the
  // proof before triggering it: once the navigation commits there is no later.
  const startedAt = Date.now();
  addEventListener('pagehide', () => {
    if (Date.now() - startedAt < AUTO_PAGEHIDE_MS) closeApp();
  }, { once: true });

  window.location.href = url;
}

// openExternally reports whether the client took the link at all. A throw is
// the only refusal it can report — on the browser-based clients a blocked
// window.open returns quietly, which is why NATIVE_DESKTOP exists.
function openExternally(url) {
  if (!tg?.openLink) return false;
  try {
    tg.openLink(url);
    return true;
  } catch (e) {
    console.warn('[auth-miniapp] openLink threw', e);
    return false;
  }
}

function closeApp() {
  if (tg?.close) tg.close();
}

// goBack sends the user onward by the route that suits the client they are on.
// It runs inside the button's click handler, which is the whole reason the
// button exists — see the header.
//
// On a phone, navigating this webview keeps them inside Telegram, where they
// already are and where the app they came from is a collapsed tab. Handing that
// user to WebApp.openLink() would throw them out into Safari or Chrome — the
// method is documented to open an EXTERNAL browser, and it does.
//
// On a desktop client that same external browser is exactly right: the user
// already has the app open in a real browser window, and keeping them in
// Telegram's built-in view leaves them with two copies in two places.
//
// Neither path closes the mini app. A tap is the user steering, and they can
// see where they landed; the automatic attempt in autoOnward is the one that
// tidies up after itself, because there nobody watched it happen. Which also
// means a user who reaches this function is a user the automatic route already
// failed for — leaving their window open is the whole reason the button is
// still on screen.
function goBack(url) {
  const desktop = DESKTOP_PLATFORMS.includes(tg?.platform);
  console.log('[auth-miniapp] platform', tg?.platform, desktop ? '→ browser' : '→ stay in telegram');

  // A refused openLink falls through rather than dead-ending the tap: inside
  // Telegram's own view is a worse place to land than the browser, but it is a
  // place, and the tap has to lead somewhere.
  if (desktop && openExternally(url)) return;
  window.location.href = url;
}

// failed says the plain thing, then the useful thing. reason is whatever the
// server or the network actually reported; it is not translated, because its
// audience is whoever reads the screenshot afterwards.
//
// Nothing here happens on a timer: a failure the page cleared away by itself is
// a failure nobody can screenshot.
function failed(reason) {
  const status = el('status');
  status.className = 'card-title error';
  status.textContent = 'Ошибка. Попробуйте снова';

  if (reason) {
    const detail = el('detail');
    detail.textContent = reason;
    detail.hidden = false;
  }

  // Back to the start of this page, which is the start of the login: the page
  // re-reads initData and asks again. That covers the failures worth retrying —
  // a network blip, auth-center not answering in time — and costs nothing on
  // the ones it cannot fix, which say so again with the reason still attached.
  const action = el('action');
  action.textContent = 'НАЗАД';
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
