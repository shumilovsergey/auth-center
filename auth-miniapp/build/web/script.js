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
       anything, so the close() lives there. Here the button waits a second
       behind a spinner: the page is leaving, and a button that flashes up and
       vanishes reads as a missed chance to press it. Whoever is still on this
       card after MOBILE_ACTION_MS gets the button, because the jump can be
       refused and a spinner that never resolves is worse than a button.

     - desktop: nothing automatic at all. openLink() is what hands the link to
       the real browser, and on the browser-based clients that call is a
       window.open — a blocked one looks from here exactly like a successful
       one, so there is no attempt worth making and nothing safe to close on.
       The route out stays the button, with the user's gesture behind it. What
       the tap does do is close the mini app a moment later: Telegram resumes a
       mini app that was left open rather than starting it fresh, so a finished
       login would come back the next time somebody opens the app.

     - QR: close, and nothing else. The browser waiting for this login is on
       another machine, so there is no link that would help here and no button
       worth showing. A close() that the client ignores leaves the name on
       screen, which is the right thing to be left with.

   Closing after the jump is deliberate and it is the one part here that trades
   safety for tidiness: the close() lands on the page just navigated to. It
   lives behind the AUTO_* constants below and can be switched off without
   touching the flow.

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

// How long the phone gets to leave on its own before the button appears
// anyway. If the jump lands, nobody ever sees the button — the document is
// gone well before this fires. If it does not, the wait cost the user a second
// and left them exactly where the desktop user already is.
const MOBILE_ACTION_MS = 1000;

// How long the desktop close waits after the tap handed the link over. The
// close itself is not tidiness — see goBack — but it must not land on the
// "open this link?" prompt some clients raise, because closing the mini app out
// from under that prompt cancels the very link we just gave them.
const DESKTOP_CLOSE_MS = 1500;

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

  const action = el('action');
  action.textContent = 'ДАЛЕЕ';
  action.addEventListener('click', () => goBack(back));

  // Desktop attempts nothing by itself, so the button is the route and goes up
  // at once — see goBack and the header.
  if (DESKTOP_PLATFORMS.includes(tg?.platform)) {
    action.hidden = false;
    return;
  }

  // A phone is about to navigate itself, and a button that appears only to
  // vanish half a second later reads as a thing the user missed their chance
  // to press. A spinner says the true thing instead — the page is going
  // somewhere — and stands in the button's own box, so the swap moves nothing.
  const pending = el('pending');
  pending.hidden = false;

  setTimeout(() => autoOnward(back), AUTO_DELAY_MS);

  // The button still comes, because the jump can be refused and a spinner that
  // never resolves is worse than the button it replaced.
  setTimeout(() => {
    pending.hidden = true;
    action.hidden = false;
  }, MOBILE_ACTION_MS);
}

// autoOnward does by itself what the button does when tapped. Only the phone
// branch of signedIn calls it: desktop is left alone, because the way out
// there is openLink(), which on the browser-based clients is a window.open
// nobody can verify — see the header.
function autoOnward(url) {
  // A phone stays inside Telegram, so the jump replaces this document. Arm the
  // proof before triggering it: once the navigation commits there is no later.
  const startedAt = Date.now();
  addEventListener('pagehide', () => {
    if (Date.now() - startedAt < AUTO_PAGEHIDE_MS) closeApp();
  }, { once: true });

  window.location.href = url;
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
// On desktop this function is the only route there is: nothing is attempted
// before the tap. It closes the mini app afterwards — not to tidy up, but
// because Telegram would otherwise resume this same instance on the next open.
// The phone path does not close here; the jump replaces this document, and
// autoOnward already closes from pagehide when it was the one that jumped.
function goBack(url) {
  const desktop = DESKTOP_PLATFORMS.includes(tg?.platform);
  console.log('[auth-miniapp] platform', tg?.platform, desktop ? '→ browser' : '→ stay in telegram');

  if (desktop && tg?.openLink) {
    tg.openLink(url);

    // And then close, which on desktop is not about tidiness. A mini app left
    // open is not neutral there: Telegram resumes THIS instance the next time
    // the app is opened instead of starting a new one, so a finished login
    // would come back in place of the new one somebody just asked for. The tap
    // is where this page's job ends, and the close is what makes that stick.
    //
    // Only this branch closes. The fallback below navigates the webview itself,
    // and a close() would take down the page just navigated to.
    setTimeout(closeApp, DESKTOP_CLOSE_MS);
    return;
  }
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
