// login.js — one page-agnostic login entry point usable from anywhere.
//
// Call cmapsLogin():
//   1. If the user is already logged in, it does nothing.
//   2. If SAML is enabled and local fallback is OFF, it redirects straight to SSO.
//   3. Otherwise it shows a centered overlay form over a blurred backdrop. When
//      SAML and local fallback are both enabled, the SSO button is offered too.
//
// Configuration is read from window.cmapsAuth (all fields optional):
//   { loggedIn, samlEnabled, samlFallback, samlUrl, next, error, loginPage, appTitle }
(function () {
  function cfg() { return window.cmapsAuth || {}; }

  function nextPath() {
    var c = cfg();
    if (c.next) { return c.next; }
    return location.pathname + location.search;
  }

  function samlHref() {
    var c = cfg();
    var url = c.samlUrl || '/auth/saml/login';
    var n = nextPath();
    if (n && n !== '/') { url += '?next=' + encodeURIComponent(n); }
    return url;
  }

  function esc(s) {
    return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }
  function escAttr(s) { return esc(s).replace(/"/g, '&quot;'); }

  function closeOverlay() {
    var el = document.getElementById('cmapsLoginOverlay');
    if (el && el.parentNode) { el.parentNode.removeChild(el); }
  }
  window.cmapsCloseLogin = closeOverlay;

  window.cmapsLogin = function () {
    var c = cfg();
    if (c.loggedIn) { return; }

    // SAML-only: no form, go straight to SSO.
    if (c.samlEnabled && !c.samlFallback) {
      window.location.href = samlHref();
      return;
    }

    if (document.getElementById('cmapsLoginOverlay')) { return; }

    var samlBlock = '';
    if (c.samlEnabled && c.samlFallback) {
      samlBlock =
        '<div class="cmaps-login-or"><span>or</span></div>' +
        '<button type="button" class="cmaps-login-btn cmaps-login-btn-sso" id="cmapsLoginSso">Sign in with SSO</button>';
    }

    var cancelBlock = '';
    if (!c.loginPage) {
      cancelBlock = '<button type="button" class="cmaps-login-cancel" id="cmapsLoginCancel">Cancel</button>';
    }

    var overlay = document.createElement('div');
    overlay.id = 'cmapsLoginOverlay';
    overlay.className = 'cmaps-login-overlay';
    overlay.innerHTML =
      '<div class="cmaps-login-card" role="dialog" aria-modal="true" aria-label="Sign in">' +
        '<img class="cmaps-login-logo" src="' + escAttr(c.logoUrl || '/static/images/cmaps-regular.png') + '" alt="' + escAttr(c.appTitle || 'CompanyMaps') + '">' +
        '<form method="post" action="/login" autocomplete="off">' +
          '<input type="hidden" name="next" value="' + escAttr(nextPath()) + '">' +
          '<input class="cmaps-login-field" type="text" name="username" placeholder="Username" autocomplete="username">' +
          '<input class="cmaps-login-field" type="password" name="password" placeholder="Password" autocomplete="current-password">' +
          '<button class="cmaps-login-btn" type="submit">Sign in</button>' +
        '</form>' +
        samlBlock +
        '<div class="cmaps-login-err">' + (c.error ? esc(c.error) : '') + '</div>' +
        cancelBlock +
      '</div>';

    document.body.appendChild(overlay);

    var sso = document.getElementById('cmapsLoginSso');
    if (sso) { sso.addEventListener('click', function () { window.location.href = samlHref(); }); }

    var cancel = document.getElementById('cmapsLoginCancel');
    if (cancel) { cancel.addEventListener('click', closeOverlay); }

    // On the map (cancelable) allow Esc / backdrop click to dismiss.
    if (!c.loginPage) {
      overlay.addEventListener('mousedown', function (e) { if (e.target === overlay) { closeOverlay(); } });
      document.addEventListener('keydown', function onEsc(e) {
        if (e.key === 'Escape') { closeOverlay(); document.removeEventListener('keydown', onEsc); }
      });
    }

    var first = overlay.querySelector('input[name="username"]');
    if (first) { first.focus(); }
  };
})();
