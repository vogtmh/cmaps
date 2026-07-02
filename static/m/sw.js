/* CompanyMaps mobile service worker.
   Makes the /m/ app installable and usable offline. The VERSION token is
   substituted by the server (handleMobileServiceWorker) with the current asset
   version, so every deploy produces new bytes — the browser then detects an SW
   update, installs it, and the activate step purges the previous cache. */
'use strict';

var VERSION = '__ASSET_VERSION__';
var CACHE = 'cmaps-m-' + VERSION;

// Core shell + assets precached on install so the app opens offline. Versioned
// URLs match the ?v= query the HTML uses, keeping the SW cache in lockstep with
// the deployed assets.
var CORE = [
  '/m/',
  '/static/m/mobile.css?v=' + VERSION,
  '/static/m/mobile-app.js?v=' + VERSION,
  '/static/tools/jquery.js',
  '/images/noavatar.png',
  '/favicons/android-chrome-192x192.png',
  '/favicons/android-chrome-512x512.png'
];

self.addEventListener('install', function (e) {
  self.skipWaiting();
  e.waitUntil(caches.open(CACHE).then(function (c) {
    // Ignore individual precache failures so one 404 can't abort installation.
    return c.addAll(CORE).catch(function () {});
  }));
});

self.addEventListener('activate', function (e) {
  e.waitUntil(
    caches.keys().then(function (keys) {
      return Promise.all(keys.filter(function (k) {
        return k.indexOf('cmaps-m-') === 0 && k !== CACHE;
      }).map(function (k) { return caches.delete(k); }));
    }).then(function () { return self.clients.claim(); })
  );
});

self.addEventListener('fetch', function (e) {
  var req = e.request;
  if (req.method !== 'GET') { return; }

  var url;
  try { url = new URL(req.url); } catch (err) { return; }
  if (url.origin !== self.location.origin) { return; }

  // App shell / navigations: network-first so live data wins when online, with
  // the cached shell as an offline fallback.
  if (req.mode === 'navigate') {
    e.respondWith(
      fetch(req).catch(function () {
        return caches.match('/m/').then(function (r) { return r || caches.match(req); });
      })
    );
    return;
  }

  // Never intercept the live API (session-specific, must hit the network).
  if (url.pathname.indexOf('/rest/') === 0) { return; }

  // Static assets, map images and avatars: cache-first, then network (and
  // populate the cache for next time).
  e.respondWith(
    caches.match(req).then(function (hit) {
      if (hit) { return hit; }
      return fetch(req).then(function (res) {
        if (res && res.ok && res.type === 'basic') {
          var copy = res.clone();
          caches.open(CACHE).then(function (c) { c.put(req, copy); });
        }
        return res;
      }).catch(function () { return hit; });
    })
  );
});
