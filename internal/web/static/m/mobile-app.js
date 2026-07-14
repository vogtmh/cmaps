/* CompanyMaps mobile UI — single-page app served under /m/.
   Separate, touch-first front-end. Talks to the same REST API as the desktop
   client but renders an independent, read-only/profile-only interface.

   All asset and data URLs are ROOT-ABSOLUTE because pages live under /m/. */
(function () {
  'use strict';

  var BOOT = window.mobileBootstrap || { loggedIn: false, perms: {} };

  // ---- DOM refs -----------------------------------------------------------
  var $main, $tabbar, $sheet, $backdrop, $toast;

  // ---- Map view state -----------------------------------------------------
  var mapState = {
    maps: [],          // [{mapname, displayname, itemscale, published}]
    current: '',
    itemscale: 1,      // marker scale for the current map
    desks: [],         // raw desk items for current map
    panzoom: null,
    meetingTimer: null,// poll handle for meeting-room availability
    meetingRooms: null,// live room status for the current map
    meetingCache: {},  // map name -> room status list (for global search)
    allDesks: null     // cached all-maps desk list for global search
  };

  // Item to focus (zoom + highlight) after a map (re)loads, set by global search.
  var pendingLocate = null; // {map, id, x, y}

  // Whether we've pushed a history entry for the open map selector, so the
  // Android/browser back button can dismiss it.
  var mapselPushed = false;

  // =========================================================================
  // Small helpers
  // =========================================================================
  function esc(s) {
    return String(s == null ? '' : s)
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
  }

  function get(url) {
    return $.ajax({ url: url, type: 'get', dataType: 'json' });
  }

  function getCookie(name) {
    var m = document.cookie.match('(?:^|; )' + name.replace(/([.$?*|{}()\[\]\\\/\+^])/g, '\\$1') + '=([^;]*)');
    return m ? decodeURIComponent(m[1]) : null;
  }
  function setCookie(name, value) {
    var d = new Date();
    d.setFullYear(d.getFullYear() + 5);
    document.cookie = name + '=' + encodeURIComponent(value) + '; path=/; expires=' + d.toUTCString() + '; SameSite=Lax';
  }

  var toastTimer = null;
  function toast(msg) {
    $toast.text(msg).removeAttr('hidden');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(function () { $toast.attr('hidden', 'hidden'); }, 2200);
  }

  function openSheet(html) {
    $sheet.html('<div class="grabber"></div>' + html).removeAttr('hidden');
    $backdrop.removeAttr('hidden');
  }
  function closeSheet() {
    $sheet.attr('hidden', 'hidden').empty();
    $backdrop.attr('hidden', 'hidden');
  }

  function spinner() { return '<div class="m_spinner">Loading…</div>'; }
  function empty(msg) { return '<div class="m_empty">' + esc(msg) + '</div>'; }
  function errorBox(msg) { return '<div class="m_empty m_error">' + esc(msg) + '</div>'; }

  function avatarImg(url, cls) {
    return '<img class="m_avatar ' + (cls || '') + '" src="' + esc(url) + '" ' +
      'onerror="this.onerror=null;this.src=\'/images/noavatar2.png\';" alt="">';
  }

  // =========================================================================
  // Router
  // =========================================================================
  function currentRoute() {
    var h = (location.hash || '').replace(/^#/, '');
    if (!h) { return { view: 'map', arg: '' }; }
    var parts = h.split('/');
    return { view: parts[0], arg: parts[1] || '' };
  }

  function navigate(view, arg) {
    var h = '#' + view + (arg ? '/' + arg : '');
    if (location.hash === h) { route(); } else { location.hash = h; }
  }

  function route() {
    closeSheet();
    var r = currentRoute();
    // Login is optional: the map and people directory are public (mirroring the
    // desktop site). Only the admin tab needs an authenticated session.
    if (r.view === 'login') { renderLogin(); return; }
    if ((r.view === 'admin' || r.view === 'adminsec') && !hasAdmin()) {
      navigate('map'); return;
    }
    setActiveTab(r.view === 'adminsec' ? 'admin' : r.view);
    switch (r.view) {
      case 'map': renderMap(); break;
      case 'search': renderSearch(); break;
      case 'admin': renderAdmin(); break;
      case 'adminsec': renderAdminSection(r.arg); break;
      case 'settings': renderSettings(); break;
      default: renderMap();
    }
  }

  function setActiveTab(view) {
    $tabbar.find('.m_tab').removeClass('active').each(function () {
      if ($(this).data('view') === view) { $(this).addClass('active'); }
    });
    // The map tab shows a distinct "map menu" icon while the map view is active
    // (tapping it again then opens the full-screen map selector).
    $('#m_tab_map_img').attr('src', view === 'map' ? '/images/mobile-mapmenu.png' : '/images/mobile-map.png');
  }

  // =========================================================================
  // Login view
  // =========================================================================
  function renderLogin() {
    $tabbar.removeAttr('hidden');
    setActiveTab('settings');
    var samlOnly = BOOT.samlEnabled && !BOOT.allowLocalFallback;
    var html = '<div id="m_login">';
    html += '<div class="m_vhead"><button class="m_vback" id="m_login_back" type="button" aria-label="Back">&#8592;</button></div>';
    html += '<div class="brand"><img src="/logos/logo.png" onerror="this.style.display=\'none\'" alt="">' +
      '<h1>' + esc(BOOT.appTitle || 'CompanyMaps') + '</h1></div>';

    if (BOOT.samlEnabled) {
      html += '<button class="m_btn" id="m_saml_btn" type="button">Sign in with SSO</button>';
    }
    html += '<form id="m_login_form" ' + (samlOnly ? 'style="display:none;margin-top:14px;"' : 'style="margin-top:14px;"') + '>' +
      '<div class="m_field"><input class="m_input" type="text" id="m_user" placeholder="Username" autocomplete="username" autocapitalize="none" autocorrect="off"></div>' +
      '<div class="m_field"><input class="m_input" type="password" id="m_pass" placeholder="Password" autocomplete="current-password"></div>' +
      '<div class="m_field m_error" id="m_login_err" style="display:none;"></div>' +
      '<button class="m_btn" type="submit">Log in</button>' +
      '</form>';
    if (samlOnly) {
      html += '<button class="m_btn secondary" id="m_show_local" type="button" style="margin-top:10px;">Use password instead</button>';
    }
    html += '</div>';
    $main.html(html);

    $('#m_login_back').on('click', function () { navigate('settings'); });
    $('#m_saml_btn').on('click', function () {
      location.href = '/auth/saml/login?next=' + encodeURIComponent('/m/');
    });
    $('#m_show_local').on('click', function () {
      $('#m_login_form').show();
      $(this).hide();
    });
    $('#m_login_form').on('submit', function (e) {
      e.preventDefault();
      var user = $('#m_user').val().trim();
      var pass = $('#m_pass').val();
      if (!user || !pass) { return; }
      var $err = $('#m_login_err').hide();
      $.ajax({
        url: '/rest/account/?mode=login',
        type: 'post',
        dataType: 'json',
        data: { user: user, password: pass }
      }).done(function (res) {
        if (res && res.ok) {
          location.href = '/m/';
        } else {
          $err.text((res && res.message) || 'Login failed.').show();
        }
      }).fail(function () {
        $err.text('Login failed. Please try again.').show();
      });
    });
  }

  // =========================================================================
  // Map view
  // =========================================================================
  function renderMap() {
    $tabbar.removeAttr('hidden');
    $main.attr('class', '').html(
      '<div id="m_mapview">' +
        '<div id="m_mapstage">' +
          '<div id="m_maplayer"><img id="m_mapimg" alt=""><div id="m_desklayer"></div></div>' +
        '</div>' +
      '</div>'
    );

    if (mapState.maps.length) {
      if (pendingLocate && mapHasName(pendingLocate.map)) { mapState.current = pendingLocate.map; }
      loadMap(mapState.current);
    } else {
      get('/rest/config?mode=maps').done(function (res) {
        var list = (res && res.maps) || [];
        // Never surface the overview in the mobile app (it has no world-map
        // renderer); keep only published, non-overview maps.
        mapState.maps = list.filter(function (m) { return m.published !== 'no' && m.mapname !== 'overview'; });
        if (!mapState.maps.length) { mapState.maps = list.filter(function (m) { return m.mapname !== 'overview'; }); }
        // Remember the last opened map (cookie), like the desktop client. Fall
        // back to the server default only on the first visit, then to the first
        // available map.
        var remembered = (getCookie('map') || '').toLowerCase();
        var def = remembered || (BOOT.defaultMap || '').toLowerCase();
        if (def && def !== 'overview' && mapHasName(def)) {
          mapState.current = def;
        } else {
          mapState.current = mapState.maps[0] ? mapState.maps[0].mapname : '';
        }
        if (pendingLocate && mapHasName(pendingLocate.map)) { mapState.current = pendingLocate.map; }
        loadMap(mapState.current);
      }).fail(function () {
        $('#m_mapstage').html(errorBox('Could not load maps.'));
      });
    }
  }

  function mapHasName(name) {
    return mapState.maps.some(function (m) { return m.mapname === name; });
  }

  // Full-screen map picker, opened by tapping the Map tab while already on the
  // map view (the on-screen dropdown was removed).
  function openMapSelector() {
    if (!mapState.maps.length) { navigate('map'); return; }
    var active = [], placeholder = [];
    mapState.maps.forEach(function (m) {
      if (m.mapname === 'overview') { return; }
      (m.placeholder ? placeholder : active).push(m);
    });
    function rows(list) {
      var s = '';
      list.forEach(function (m) {
        var label = (m.displayname || cap(m.mapname)).replace(/-nomap/gi, '');
        s += '<button class="m_maprow' + (m.mapname === mapState.current ? ' current' : '') +
          '" type="button" data-map="' + esc(m.mapname) + '">' + esc(label) + '</button>';
      });
      return s;
    }
    var html = '<div class="m_mapsel_panel"><div class="m_mapsel_list">';
    html += rows(active);
    if (placeholder.length) {
      html += '<div class="m_mapsel_sep">Placeholder maps</div>' + rows(placeholder);
    }
    html += '</div></div>';
    var $ov = $('#m_mapselector');
    $ov.html(html).removeAttr('hidden');
    // Force a reflow so the browser registers the pre-animation state before we
    // add .open, then let the spring transition slide it up from the bottom.
    $ov[0].offsetHeight;
    $ov.addClass('open');
    // De-promote the map's composited layer while the overlay is up (Android
    // Chrome otherwise paints the map on top of the fixed overlay).
    $('#m_app').addClass('m_mapsel_open');
    // While the selector is open the map tab shows the "close" glyph.
    $('#m_tab_map_img').attr('src', '/images/mobile-mapmenuclose.png');
    // Push a history entry so the Android back button dismisses the selector
    // (rather than routing to the previous tab while the overlay lingers).
    if (!mapselPushed) {
      mapselPushed = true;
      history.pushState({ mapsel: true }, '');
    }
    $ov.find('.m_maprow').on('click', function () {
      var mp = this.getAttribute('data-map');
      closeMapSelector();
      if (mp !== mapState.current) { pendingLocate = null; loadMap(mp); }
    });
  }

  // Visually hide the selector without touching browser history. silent skips
  // the spring slide-down animation.
  function hideMapSelector(silent) {
    var $ov = $('#m_mapselector');
    // Revert the map tab icon back to the "map menu" glyph (only meaningful when
    // we stay on the map view; navigation away resets it via setActiveTab).
    $('#m_tab_map_img').attr('src', '/images/mobile-mapmenu.png');
    $('#m_app').removeClass('m_mapsel_open');
    if (silent || !$ov.hasClass('open')) {
      $ov.removeClass('open').attr('hidden', 'hidden').empty();
      return;
    }
    $ov.removeClass('open');
    window.setTimeout(function () {
      $ov.attr('hidden', 'hidden').empty();
    }, 440);
  }

  // Programmatic close (Map-tab tap, map row select, tab switch). Also unwinds
  // the history entry pushed by openMapSelector so the back stack stays clean.
  function closeMapSelector(silent) {
    if (silent) {
      // Caller is about to navigate() to another view; collapse the pushed
      // entry into the current one to avoid a dangling back step.
      if (mapselPushed) { mapselPushed = false; history.replaceState(null, ''); }
      hideMapSelector(true);
      return;
    }
    if (mapselPushed) {
      // Pop our pushed entry; the popstate handler performs the visual close.
      mapselPushed = false;
      history.back();
      return;
    }
    hideMapSelector(false);
  }

  function loadMap(name) {
    if (!name) { $('#m_mapstage').prepend(errorBox('No map available.')); return; }
    mapState.current = name;
    setCookie('map', name);
    var meta = mapState.maps.filter(function (m) { return m.mapname === name; })[0];
    mapState.itemscale = parseFloat(meta && meta.itemscale) || 1;
    if (!mapState.itemscale) { mapState.itemscale = 1; }
    if (mapState.meetingTimer) { clearInterval(mapState.meetingTimer); mapState.meetingTimer = null; }
    var img = document.getElementById('m_mapimg');
    img.src = '/maps/' + encodeURIComponent(name) + '.png';
    $('#m_desklayer').empty();
    get('/rest/desks?map=' + encodeURIComponent(name)).done(function (res) {
      mapState.desks = (res && res.desks) || [];
      renderDesks();
      updateMeetingStatus(name);
      mapState.meetingTimer = setInterval(function () {
        if (mapState.current === name) { updateMeetingStatus(name); }
      }, 60000);
      if (pendingLocate && pendingLocate.map === name) { setTimeout(maybeLocate, 400); }
    }).fail(function () {
      mapState.desks = [];
      renderDesks();
    });
  }

  // Fetches live meeting-room availability and applies the desktop-style pulse
  // (green = available, blue = busy) to the matching meeting markers.
  function updateMeetingStatus(name) {
    get('/rest/meeting?map=' + encodeURIComponent(name) + '&usecache=true').done(function (res) {
      var rooms = (res && res.rooms) || [];
      mapState.meetingCache[name] = rooms;
      if (mapState.current !== name) { return; }
      mapState.meetingRooms = rooms;
      $('#m_desklayer .m_desk.meeting').removeClass('pulse_available pulse_busy');
      rooms.forEach(function (room) {
        var d = null;
        for (var i = 0; i < mapState.desks.length; i++) {
          var cand = mapState.desks[i];
          if (cand.desktype !== 'meeting') { continue; }
          if ((room.deskid && String(cand.id) === String(room.deskid)) ||
              (room.name && cand.dsk === room.name)) { d = cand; break; }
        }
        if (!d) { return; }
        var el = document.querySelector('#m_desklayer .m_desk[data-id="' + d.id + '"]');
        if (!el) { return; }
        if (room.availability === 'available') { el.classList.add('pulse_available'); }
        else if (room.availability === 'booked' || room.availability === 'in_use') { el.classList.add('pulse_busy'); }
      });
    });
  }

  // deskInfo mirrors the desktop client's updateDesks() type/size resolution so
  // the marker gets the same CSS class (hence colour + icon) and on-map size.
  // Returns {type, half, name} or null for floor markers (handled via picker).
  function deskInfo(d) {
    var t = d.desktype;
    if (t === 'floor') { return null; }
    if (t && t.indexOf('custom_') === 0) { return { type: 'custom', half: 25, name: d.dsk || 'Item' }; }
    switch (t) {
      case 'exit': case 'meeting': case 'restroom':
        return { type: t, half: 25, name: d.empl || cap(t) };
      case 'firstaid': case 'food': case 'keycardlock': case 'keylock': case 'printer': case 'service':
        return { type: t, half: 18, name: d.empl || cap(t) };
      case 'shareddesk':
        return { type: 'shareddesk', half: 10, name: 'Shared Desk' };
    }
    if (!d.empl || (t === 'addesk' && !d.mail)) {
      return { type: 'free', half: 10, name: 'Not in use' };
    }
    switch (t) {
      case 'blocked':
        return { type: 'blocked', half: 10, name: d.dsk || 'Blocked' };
      case 'booking': case 'hotseat':
        if (d.booked == 1) { return { type: t + '_booked', half: 10, name: (d.bookdata && d.bookdata.name) || d.dsk }; }
        return { type: t + '_free', half: 10, name: d.dsk || cap(t) };
      case 'addesk':
        return { type: 'occupiedldap', half: 10, name: ((d.fname || '') + ' ' + (d.lname || '')).trim() || d.empl };
      default:
        return { type: 'occupied', half: 10, name: d.empl };
    }
  }

  function cap(s) { return s ? s.charAt(0).toUpperCase() + s.slice(1) : s; }

  // Builds one absolutely-positioned text label centred horizontally on (x, y)
  // inside the scaled map layer (so it zooms with the map). Used for the
  // optional name / desk-number overlays.
  function deskLabel(text, x, y, fontPx) {
    var el = document.createElement('div');
    el.className = 'm_desklabel';
    el.textContent = text;
    el.style.left = x + 'px';
    el.style.top = y + 'px';
    el.style.fontSize = fontPx + 'px';
    return el;
  }

  function renderDesks() {
    var layer = document.getElementById('m_desklayer');
    layer.innerHTML = '';
    var scale = mapState.itemscale || 1;
    var showNames = getCookie('setting_shownames') === '1';
    var showNums = getCookie('setting_desknumbers') === '1';
    var hlLeaders = getCookie('setting_highlightleaders') === '1';
    var seen = {};
    var frag = document.createDocumentFragment();
    mapState.desks.forEach(function (d) {
      if (d.desktype === 'floor') { return; }
      if (d.id && seen[d.id]) { return; }
      if (d.id) { seen[d.id] = true; }
      var info = deskInfo(d);
      if (!info) { return; }
      var x = parseFloat(d.x) || 0, y = parseFloat(d.y) || 0;
      var sizePx = 2 * info.half * scale;

      // Team-leader ring (desktop "highlight leaders"): desks flagged with a
      // VIP/leader colour get a coloured ring drawn concentrically behind the
      // marker. Rendered as a static element (no animation) to stay clear of
      // the Android compositing issues that plagued animated shadows.
      if (hlLeaders && d.color) {
        var rw = Math.max(2, sizePx * 0.16);
        var ring = document.createElement('div');
        ring.className = 'm_deskring';
        ring.style.left = x + 'px';
        ring.style.top = y + 'px';
        ring.style.width = sizePx + 'px';
        ring.style.height = sizePx + 'px';
        ring.style.borderWidth = rw + 'px';
        ring.style.borderColor = d.color;
        frag.appendChild(ring);
      }

      var dot = document.createElement('div');
      dot.className = 'm_desk ' + info.type;
      dot.style.left = x + 'px';
      dot.style.top = y + 'px';
      dot.style.width = sizePx + 'px';
      dot.style.height = sizePx + 'px';
      dot.setAttribute('data-id', d.id);
      frag.appendChild(dot);

      // Optional name / desk-number labels (desktop "show names" / "show desk
      // numbers"). Skipped for facility icons and shared desks, mirroring the
      // desktop client. Labels live in the scaled map layer so they zoom with
      // the map; sizes are in map (pre-zoom) px like the markers.
      if ((showNames || showNums) && !ITEM_TYPES[info.type] && info.type !== 'shareddesk') {
        var labelY = y + info.half * scale + 1;
        var fontPx = Math.max(6, 8 * scale);
        if (showNames && d.fname) {
          frag.appendChild(deskLabel(d.fname, x, labelY, fontPx));
          labelY += fontPx * 1.15;
        }
        if (showNums && d.dsk) {
          var num = d.dsk.substring(d.dsk.indexOf('-') + 1);
          if (num) { frag.appendChild(deskLabel(num, x, labelY, fontPx)); }
        }
      }
    });
    layer.appendChild(frag);

    setupPanZoom();

    $('#m_desklayer').off('click', '.m_desk').on('click', '.m_desk', function () {
      if (mapState.panzoom && mapState.panzoom.didMove()) { return; }
      showDeskDetail(this.getAttribute('data-id'));
    });
  }

  // Facility desk types that show their own icon instead of a person avatar.
  var ITEM_TYPES = { meeting: 1, restroom: 1, exit: 1, firstaid: 1, food: 1, keycardlock: 1, keylock: 1, printer: 1, service: 1, custom: 1 };

  // Finds the live status entry for a meeting desk (by desk id, then room name).
  function meetingStatusFor(rooms, d) {
    rooms = rooms || [];
    for (var i = 0; i < rooms.length; i++) {
      if (rooms[i].deskid && String(rooms[i].deskid) === String(d.id)) { return rooms[i]; }
    }
    for (var j = 0; j < rooms.length; j++) {
      if (rooms[j].name && d.dsk && rooms[j].name === d.dsk) { return rooms[j]; }
    }
    return null;
  }

  // Renders the desktop-style "Now / Next" availability block for a meeting room.
  function meetingBlock(d) {
    var rs = meetingStatusFor(mapState.meetingRooms, d);
    if (!rs) { return '<div class="m_meet_none">No availability information.</div>'; }
    var busy = (rs.availability === 'booked' || rs.availability === 'in_use');
    var nowVal = busy
      ? (esc(rs.now_title || 'In use') + (rs.now_start ? '<br>' + esc(rs.now_start) + ' \u2013 ' + esc(rs.now_end) : ''))
      : 'Available';
    var nextVal = rs.next_title
      ? (esc(rs.next_title) + (rs.next_start ? '<br>' + esc(rs.next_start) + ' \u2013 ' + esc(rs.next_end) : ''))
      : '\u2014';
    return '<div class="m_meet">' +
      '<div class="m_meet_col ' + (busy ? 'now_busy' : 'now_free') + '"><div class="lbl">Now</div><div class="val">' + nowVal + '</div></div>' +
      '<div class="m_meet_col next"><div class="lbl">Next</div><div class="val">' + nextVal + '</div></div>' +
      '</div>';
  }

  function showDeskDetail(id) {
    var d = null;
    for (var i = 0; i < mapState.desks.length; i++) {
      if (String(mapState.desks[i].id) === String(id)) { d = mapState.desks[i]; break; }
    }
    if (!d) { return; }
    var name, dept = d.dept || '', phone = d.phone || '', mail = d.mail || '', title = d.title || '';
    var avtr = d.avtr || '';
    if (d.desktype === 'addesk') {
      name = ((d.fname || '') + ' ' + (d.lname || '')).trim() || d.empl || 'Unassigned';
    } else if (d.booked == 1 && d.bookdata) {
      name = d.bookdata.name; phone = d.bookdata.phone || phone; mail = d.bookdata.mail || mail;
    } else {
      var v = deskInfo(d);
      name = (v && v.name) || d.empl || d.dsk || 'Desk';
    }
    var avatarUrl = avtr ? '/avatarcache/' + encodeURIComponent(avtr) + '.jpg' : '/images/noavatar2.png';

    var html = '<div class="m_sheet_head">' + avatarImg(avatarUrl, 'lg') +
      '<div><div class="name">' + esc(name) + '</div>' +
      (title ? '<div class="role">' + esc(title) + '</div>' : '') +
      (dept ? '<div class="role">' + esc(dept) + '</div>' : '') + '</div></div>';
    if (d.desktype === 'meeting') { html += meetingBlock(d); }
    if (d.dsk) { html += '<div class="m_detail_row"><span class="ico">&#128205;</span><span>' + esc(d.dsk) + '</span></div>'; }
    if (phone) { html += '<div class="m_detail_row"><span class="ico">&#128222;</span><a href="tel:' + esc(phone) + '">' + esc(phone) + '</a></div>'; }
    if (mail) { html += '<div class="m_detail_row"><span class="ico">&#9993;</span><a href="mailto:' + esc(mail) + '">' + esc(mail) + '</a></div>'; }
    html += '<button class="m_btn secondary" type="button" id="m_sheet_close" style="margin-top:14px;">Close</button>';
    openSheet(html);
    $('#m_sheet_close').on('click', closeSheet);
  }

  // ---- Map person search --------------------------------------------------
  // Focus a result chosen from global search once its map has loaded: zoom in,
  // highlight the marker, and open its detail sheet.
  function maybeLocate() {
    if (!pendingLocate || pendingLocate.map !== mapState.current) { return; }
    var id = pendingLocate.id, x = pendingLocate.x, y = pendingLocate.y;
    pendingLocate = null;
    $('#m_desklayer .m_desk').removeClass('highlight');
    $('#m_desklayer .m_desk[data-id="' + id + '"]').addClass('highlight');
    if (mapState.panzoom) { mapState.panzoom.locate(x, y); }
    setTimeout(function () { showDeskDetail(id); }, 350);
  }

  // ---- Pinch / pan --------------------------------------------------------
  var LAYER_W = 1600; // desk coords live in a 1600px-wide space (see desktop client)
  function setupPanZoom() {
    var stage = document.getElementById('m_mapstage');
    var layer = document.getElementById('m_maplayer');
    var img = document.getElementById('m_mapimg');
    // The stage element persists across map switches; only wire the touch
    // listeners once, otherwise they stack on every loadMap().
    if (stage._mpz) {
      mapState.panzoom = stage._mpz;
      if (img.complete && img.naturalWidth) { stage._mpz.refit(); } else { img.onload = stage._mpz.refit; }
      return;
    }
    var scale = 1, tx = 0, ty = 0, minScale = 0.1, maxScale = 8;
    var start = null, moved = false;

    // Plain 2D transform (no translate3d). Forcing a 3D transform promoted this
    // large map layer to a single GPU texture that exceeded Android Chrome's max
    // texture size, so whole regions failed to render. A 2D transform lets the
    // browser tile-render the layer, which has no such size limit.
    function apply() { layer.style.transform = 'translate(' + tx + 'px,' + ty + 'px) scale(' + scale + ')'; }
    function rect() { return stage.getBoundingClientRect(); }
    function dist(t) { return Math.hypot(t[0].clientX - t[1].clientX, t[0].clientY - t[1].clientY); }

    function fit() {
      var r = rect();
      scale = r.width / LAYER_W;
      minScale = scale * 0.5;
      tx = 0; ty = 12;
      apply();
    }

    if (img.complete && img.naturalWidth) { fit(); } else { img.onload = fit; }

    stage.addEventListener('touchstart', function (e) {
      moved = false;
      if (e.touches.length === 1) {
        start = { mode: 'pan', x: e.touches[0].clientX, y: e.touches[0].clientY, tx: tx, ty: ty };
      } else if (e.touches.length === 2) {
        var r = rect(), mx = (e.touches[0].clientX + e.touches[1].clientX) / 2, my = (e.touches[0].clientY + e.touches[1].clientY) / 2;
        start = { mode: 'pinch', d: dist(e.touches), scale: scale, fx: mx - r.left, fy: my - r.top, tx: tx, ty: ty };
      }
    }, { passive: false });

    stage.addEventListener('touchmove', function (e) {
      if (!start) { return; }
      e.preventDefault();
      // Mark an active gesture so CSS suppresses the meeting pulse rings while the
      // map layer is being transformed (prevents black flicker on Android Chrome).
      stage.classList.add('m_panning');
      if (start.mode === 'pan' && e.touches.length === 1) {
        var dx = e.touches[0].clientX - start.x, dy = e.touches[0].clientY - start.y;
        if (Math.abs(dx) > 6 || Math.abs(dy) > 6) { moved = true; }
        tx = start.tx + dx; ty = start.ty + dy;
      } else if (start.mode === 'pinch' && e.touches.length === 2) {
        moved = true;
        var nd = dist(e.touches);
        var ns = Math.min(maxScale, Math.max(minScale, start.scale * (nd / start.d)));
        var k = ns / start.scale;
        tx = start.fx - k * (start.fx - start.tx);
        ty = start.fy - k * (start.fy - start.ty);
        scale = ns;
      }
      apply();
    }, { passive: false });

    stage.addEventListener('touchend', function (e) {
      if (e.touches.length === 0) { start = null; stage.classList.remove('m_panning'); }
      else if (e.touches.length === 1) { start = { mode: 'pan', x: e.touches[0].clientX, y: e.touches[0].clientY, tx: tx, ty: ty }; }
    }, { passive: false });

    stage.addEventListener('touchcancel', function () {
      start = null;
      stage.classList.remove('m_panning');
    }, { passive: false });

    mapState.panzoom = {
      didMove: function () { return moved; },
      refit: fit,
      centerOn: function (lx, ly) {
        var r = rect();
        if (scale < r.width / LAYER_W) { scale = Math.min(maxScale, r.width / LAYER_W * 1.5); }
        tx = r.width / 2 - lx * scale;
        ty = r.height / 2 - ly * scale;
        apply();
      },
      locate: function (lx, ly) {
        var r = rect();
        scale = Math.min(maxScale, Math.max(r.width / LAYER_W * 2.2, 1));
        tx = r.width / 2 - lx * scale;
        ty = r.height / 2 - ly * scale;
        apply();
      }
    };
    stage._mpz = mapState.panzoom;
  }

  // =========================================================================
  // Global search (all maps)
  // =========================================================================
  // Mirrors the on-map "regular" search (matches occupant + desk/room label)
  // but across every published map at once, sorted alphabetically. Selecting a
  // result opens its map and zooms to the person / item.
  function renderSearch() {
    $tabbar.removeAttr('hidden');
    $main.attr('class', '').html(
      '<div class="m_view m_view_scroll">' +
        '<div class="m_search"><div class="m_searchbox">' +
          '<img class="m_searchbox_icon" src="/images/mobile-find.png" alt="">' +
          '<input class="m_searchinput" id="m_gsearch" type="search" placeholder="Search all maps\u2026" autocapitalize="none" autocorrect="off" enterkeyhint="search">' +
        '</div></div>' +
        '<div id="m_gsearchlist">' + empty('Search for people, desks or meeting rooms\u2026') + '</div>' +
      '</div>'
    );
    var t = null;
    $('#m_gsearch').on('input', function () {
      var q = this.value.trim();
      clearTimeout(t);
      if (q.length < 2) { $('#m_gsearchlist').html(empty('Search for people, desks or meeting rooms\u2026')); return; }
      t = setTimeout(function () { runGlobalSearch(q); }, 150);
    }).on('keydown', function (e) {
      // Enter dismisses the keyboard (results already update live while typing).
      if (e.key === 'Enter' || e.keyCode === 13) { e.preventDefault(); this.blur(); }
    }).focus();
  }

  // /rest/desks with no map returns every published map's items (each has .map).
  // Loaded once and cached for instant client-side filtering while typing.
  function ensureAllDesks(cb) {
    if (mapState.allDesks) { cb(); return; }
    get('/rest/desks').done(function (res) {
      mapState.allDesks = (res && res.desks) || [];
      cb();
    }).fail(function () { mapState.allDesks = []; cb(); });
  }

  // Fetches (and caches) a single map's meeting-room status for the search list.
  function ensureMeeting(map, cb) {
    if (mapState.meetingCache[map]) { cb(mapState.meetingCache[map]); return; }
    get('/rest/meeting?map=' + encodeURIComponent(map) + '&usecache=true').done(function (res) {
      var rooms = (res && res.rooms) || [];
      mapState.meetingCache[map] = rooms;
      cb(rooms);
    }).fail(function () { mapState.meetingCache[map] = []; cb([]); });
  }

  function runGlobalSearch(q) {
    var ql = q.toLowerCase();
    $('#m_gsearchlist').html(spinner());
    ensureAllDesks(function () {
      var seen = {}, results = [];
      mapState.allDesks.forEach(function (d) {
        if (d.desktype === 'floor') { return; }
        // Match the same fields as the desktop search, INCLUDING desktype, so a
        // category query like "meeting" returns every meeting room.
        var hay = ((d.empl || '') + ' ' + (d.fname || '') + ' ' + (d.lname || '') + ' ' +
          (d.dsk || '') + ' ' + (d.desktype || '')).toLowerCase();
        if (hay.indexOf(ql) < 0) { return; }
        var key = d.map + '/' + d.id;
        if (seen[key]) { return; }
        seen[key] = true;
        var info = deskInfo(d);
        d._name = (info && info.name) || d.empl || d.dsk || 'Item';
        results.push(d);
      });
      results.sort(function (a, b) { return a._name.localeCompare(b._name) || a.map.localeCompare(b.map); });
      results = results.slice(0, 200);
      if (!results.length) { $('#m_gsearchlist').html(empty('No matches.')); return; }
      var meetingMaps = {};
      var html = '<div class="m_card" style="padding:0 14px;">';
      results.forEach(function (d, i) {
        var meta = mapState.maps.filter(function (m) { return m.mapname === d.map; })[0];
        var maplabel = (meta && meta.displayname) || cap(d.map);
        var info = deskInfo(d);
        var type = info ? info.type : '';
        var isItem = ITEM_TYPES[type] === 1;
        var icon;
        if (isItem) {
          icon = '<span class="m_desk m_resicon ' + type + '"></span>';
        } else if (d.avtr) {
          icon = avatarImg('/avatarcache/' + encodeURIComponent(d.avtr) + '.jpg');
        } else if (d.fname || d.lname || d.empl) {
          icon = avatarImg('/images/noavatar2.png');
        } else {
          icon = '<span class="m_desk m_resicon ' + type + '"></span>';
        }
        var subHtml = esc(maplabel + (d.dsk && d.dsk !== d._name ? ' \u00b7 ' + d.dsk : ''));
        if (d.desktype === 'meeting') {
          meetingMaps[d.map] = 1;
          subHtml += ' \u00b7 <span class="m_mstat" data-map="' + esc(d.map) + '" data-deskid="' + esc(String(d.id)) + '">\u2026</span>';
        }
        html += '<div class="m_row" data-i="' + i + '">' + icon +
          '<div class="m_row_main"><div class="m_row_title">' + esc(d._name) + '</div>' +
          '<div class="m_row_sub">' + subHtml + '</div></div></div>';
      });
      html += '</div>';
      $('#m_gsearchlist').html(html);
      $('#m_gsearchlist .m_row').on('click', function () {
        openSearchResult(results[parseInt(this.getAttribute('data-i'), 10)]);
      });
      // Fill in live meeting-room status per map (like the desktop search).
      Object.keys(meetingMaps).forEach(function (mp) {
        ensureMeeting(mp, function (rooms) {
          $('#m_gsearchlist .m_mstat').each(function () {
            var el = $(this);
            if (el.attr('data-map') !== mp) { return; }
            var rs = null;
            for (var k = 0; k < rooms.length; k++) {
              if (rooms[k].deskid && String(rooms[k].deskid) === el.attr('data-deskid')) { rs = rooms[k]; break; }
            }
            if (!rs) { el.text('no information'); return; }
            var busy = (rs.availability === 'booked' || rs.availability === 'in_use');
            el.text(busy ? (rs.now_title || 'In use') : 'Free')
              .addClass(busy ? 'm_mstat_busy' : 'm_mstat_free');
          });
        });
      });
    });
  }

  function openSearchResult(d) {
    if (!d) { return; }
    pendingLocate = { map: d.map, id: d.id, x: parseFloat(d.x) || 0, y: parseFloat(d.y) || 0 };
    navigate('map');
  }

  // Directory-user detail sheet (used by the admin Users section).
  function showPersonDetail(u) {
    var name = (u.givenname + ' ' + u.surname).trim();
    var av = u.samaccountname ? '/avatarcache/' + encodeURIComponent(u.samaccountname) + '.jpg' : '/images/noavatar2.png';
    var html = '<div class="m_sheet_head">' + avatarImg(av, 'lg') +
      '<div><div class="name">' + esc(name) + '</div>' +
      (u.title ? '<div class="role">' + esc(u.title) + '</div>' : '') + '</div></div>';
    if (u.desk) { html += '<div class="m_detail_row"><span class="ico">&#128205;</span><span>' + esc(u.desk) + '</span></div>'; }
    if (u.phone) { html += '<div class="m_detail_row"><span class="ico">&#128222;</span><a href="tel:' + esc(u.phone) + '">' + esc(u.phone) + '</a></div>'; }
    if (u.mail) { html += '<div class="m_detail_row"><span class="ico">&#9993;</span><a href="mailto:' + esc(u.mail) + '">' + esc(u.mail) + '</a></div>'; }
    html += '<button class="m_btn secondary" type="button" id="m_sheet_close" style="margin-top:14px;">Close</button>';
    openSheet(html);
    $('#m_sheet_close').on('click', closeSheet);
  }

  // =========================================================================
  // Admin (read-only)
  // =========================================================================
  function adminSections() {
    var p = BOOT.perms || {};
    var s = [];
    if (p.health >= 1) { s.push({ key: 'health', label: 'System health', icon: '&#10084;' }); }
    if (p.stats >= 1) { s.push({ key: 'stats', label: 'Statistics', icon: '&#128202;' }); }
    if (p.auditlog >= 1) { s.push({ key: 'audit', label: 'Audit log', icon: '&#128221;' }); }
    if (p.users >= 1) { s.push({ key: 'users', label: 'Users', icon: '&#128100;' }); }
    if (p.teams >= 1) { s.push({ key: 'teams', label: 'Teams', icon: '&#128101;' }); }
    if (p.maps >= 1) { s.push({ key: 'maps', label: 'Maps', icon: '&#128506;' }); }
    return s;
  }

  function hasAdmin() { return adminSections().length > 0; }

  function renderAdmin() {
    $tabbar.removeAttr('hidden');
    var secs = adminSections();
    if (!secs.length) { $main.html('<div class="m_view m_view_scroll">' + empty('You have no admin permissions.') + '</div>'); return; }
    var html = '<div class="m_view m_view_scroll"><div class="m_card m_adminmenu" style="padding:0 14px;">';
    secs.forEach(function (s) {
      html += '<div class="m_row" data-key="' + s.key + '"><span style="font-size:20px;width:26px;text-align:center;">' + s.icon + '</span>' +
        '<div class="m_row_main"><div class="m_row_title">' + s.label + '</div></div><span class="chev">&#8250;</span></div>';
    });
    html += '</div></div>';
    $main.html(html);
    $main.find('.m_row').on('click', function () { navigate('adminsec', this.getAttribute('data-key')); });
  }

  function renderAdminSection(key) {
    $tabbar.removeAttr('hidden');
    var labels = { health: 'System health', stats: 'Statistics', audit: 'Audit log', users: 'Users', teams: 'Teams', maps: 'Maps' };
    var title = labels[key] || 'Admin';
    $main.attr('class', '').html('<div class="m_view m_view_scroll">' +
      '<div class="m_vhead"><button class="m_vback" id="m_adminback" type="button" aria-label="Back">&#8592;</button><span class="m_vtitle">' + esc(title) + '</span></div>' +
      '<div id="m_adminbody">' + spinner() + '</div></div>');
    $('#m_adminback').on('click', function () { navigate('admin'); });
    switch (key) {
      case 'health': adminHealth(); break;
      case 'stats': adminStats(); break;
      case 'audit': adminAudit(); break;
      case 'users': adminUsers(); break;
      case 'teams': adminTeams(); break;
      case 'maps': adminMaps(); break;
      default: $('#m_adminbody').html(empty('Unknown section.'));
    }
  }

  function adminHealth() {
    get('/rest/system?healthdetails=1').done(function (s) {
      var ldap = s.consistency_ldap || 0, desks = s.consistency_desks || 0;
      var html = '<div class="m_card">' +
        statRow('CPU load', (s.cpuload || '0') + ' %') +
        statRow('Memory used', (s.memoryused || '0') + ' %') +
        statRow('Disk used', (s.diskused || '0') + ' %') + '</div>';
      html += '<div class="m_section_title">Consistency</div><div class="m_card">' +
        '<div class="m_row"><div class="m_row_main"><div class="m_row_title">Shared LDAP desks</div></div>' +
        '<span class="m_chip ' + (ldap ? 'warn' : 'ok') + '">' + ldap + '</span></div>' +
        '<div class="m_row"><div class="m_row_main"><div class="m_row_title">Duplicate desks</div></div>' +
        '<span class="m_chip ' + (desks ? 'warn' : 'ok') + '">' + desks + '</span></div></div>';

      var det = (s.health) || {};
      if (det.desks && det.desks.length) {
        html += '<div class="m_section_title">Duplicate desk details</div><div class="m_card" style="padding:0 14px;">';
        det.desks.forEach(function (g) {
          var nm = g.desk || 'Desk';
          var cnt = g.count != null ? g.count : ((g.members && g.members.length) || '');
          html += '<div class="m_row"><div class="m_row_main"><div class="m_row_title">' + esc(nm) + '</div>' +
            '<div class="m_row_sub">' + esc(g.map || '') + '</div></div><span class="m_chip warn">' + esc(cnt) + '</span></div>';
        });
        html += '</div>';
      }
      $('#m_adminbody').html(html);
    }).fail(function () { $('#m_adminbody').html(errorBox('Could not load health data.')); });
  }

  function statRow(label, val) {
    return '<div class="m_row"><div class="m_row_main"><div class="m_row_title">' + esc(label) + '</div></div><div class="m_value" style="flex:0 0 auto;">' + esc(val) + '</div></div>';
  }

  function adminStats() {
    get('/rest/stats?interval=month&limit=12').done(function (res) {
      var items = Array.isArray(res) ? res : [];
      // /rest/stats returns [{period,count}] newest-first.
      items = items.filter(function (x) { return x && x.period; });
      if (!items.length) { $('#m_adminbody').html(empty('No statistics yet.')); return; }
      items.reverse(); // oldest -> newest for a natural bar order
      var max = 1;
      items.forEach(function (x) { if (x.count > max) { max = x.count; } });
      var html = '<div class="m_section_title">Visits per month</div><div class="m_card">';
      items.forEach(function (x) {
        var pct = Math.round((x.count / max) * 100);
        html += '<div class="m_bar_row"><div class="m_bar_label">' + esc(x.period) + '</div>' +
          '<div class="m_bar_track"><div class="m_bar_fill" style="width:' + pct + '%"></div></div>' +
          '<div class="m_bar_val">' + esc(x.count) + '</div></div>';
      });
      html += '</div>';
      $('#m_adminbody').html(html);
    }).fail(function () { $('#m_adminbody').html(errorBox('Could not load statistics.')); });
  }

  function adminAudit() {
    get('/rest/auditlog?limit=100').done(function (res) {
      var entries = (res && res.entries) || [];
      if (!entries.length) { $('#m_adminbody').html(empty('No audit entries.')); return; }
      var html = '<div class="m_card" style="padding:0 14px;">';
      entries.forEach(function (e) {
        html += '<div class="m_row"><div class="m_row_main">' +
          '<div class="m_row_title">' + esc(e.type) + ' &middot; ' + esc(e.user) + '</div>' +
          '<div class="m_row_sub">' + esc(e.info) + '</div>' +
          '<div class="m_row_sub" style="font-size:11px;">' + esc(e.timestamp) + '</div></div></div>';
      });
      html += '</div>';
      $('#m_adminbody').html(html);
    }).fail(function () { $('#m_adminbody').html(errorBox('Could not load audit log.')); });
  }

  function adminUsers() {
    $('#m_adminbody').html(
      '<div class="m_search"><input class="m_input" id="m_usersearch" type="search" placeholder="Search users…" autocapitalize="none"></div>' +
      '<div id="m_userlist">' + empty('Type to search.') + '</div>'
    );
    var t = null;
    $('#m_usersearch').on('input', function () {
      var q = this.value.trim();
      clearTimeout(t);
      if (q.length < 2) { $('#m_userlist').html(empty('Type to search.')); return; }
      t = setTimeout(function () {
        $('#m_userlist').html(spinner());
        get('/rest/users?search=' + encodeURIComponent(q)).done(function (res) {
          var users = (res && res.users) || [];
          if (!users.length) { $('#m_userlist').html(empty('No users found.')); return; }
          var html = '<div class="m_card" style="padding:0 14px;">';
          users.slice(0, 100).forEach(function (u, i) {
            html += '<div class="m_row" data-i="' + i + '"><div class="m_row_main">' +
              '<div class="m_row_title">' + esc((u.givenname + ' ' + u.surname).trim()) + '</div>' +
              '<div class="m_row_sub">' + esc(u.samaccountname || '') + ' &middot; ' + esc(u.mail || '') + '</div></div></div>';
          });
          html += '</div>';
          $('#m_userlist').html(html);
          $('#m_userlist .m_row').on('click', function () { showPersonDetail(users[parseInt(this.getAttribute('data-i'), 10)]); });
        }).fail(function () { $('#m_userlist').html(errorBox('Search failed.')); });
      }, 250);
    });
  }

  function adminTeams() {
    get('/rest/teams').done(function (res) {
      var teams = (res && res.teams) || [];
      if (!teams.length) { $('#m_adminbody').html(empty('No teams defined.')); return; }
      var html = '<div class="m_card" style="padding:0 14px;">';
      teams.forEach(function (tm) {
        var members = (tm.members || '').split('|').filter(function (x) { return x.trim(); });
        html += '<div class="m_row"><div class="m_row_main"><div class="m_row_title">' + esc(tm.teamname) + '</div>' +
          '<div class="m_row_sub">' + members.length + ' member' + (members.length === 1 ? '' : 's') + '</div></div></div>';
      });
      html += '</div>';
      $('#m_adminbody').html(html);
    }).fail(function () { $('#m_adminbody').html(errorBox('Could not load teams.')); });
  }

  function adminMaps() {
    get('/rest/config?mode=maps').done(function (res) {
      var maps = (res && res.maps) || [];
      if (!maps.length) { $('#m_adminbody').html(empty('No maps defined.')); return; }
      var html = '<div class="m_card" style="padding:0 14px;">';
      maps.forEach(function (m) {
        var pub = m.published === 'no' ? '<span class="m_chip">hidden</span>' : '<span class="m_chip ok">published</span>';
        html += '<div class="m_row"><div class="m_row_main"><div class="m_row_title">' + esc(m.displayname || m.mapname) + '</div>' +
          '<div class="m_row_sub">' + esc(m.mapname) + (m.country ? ' &middot; ' + esc(m.country) : '') + '</div></div>' + pub + '</div>';
      });
      html += '</div>';
      $('#m_adminbody').html(html);
    }).fail(function () { $('#m_adminbody').html(errorBox('Could not load maps.')); });
  }

  // =========================================================================
  // Settings
  // =========================================================================
  function renderSettings() {
    $tabbar.removeAttr('hidden');
    var html = '<div class="m_view m_view_scroll">';

    // Profile (or a sign-in prompt for anonymous visitors)
    if (BOOT.loggedIn) {
      html += '<div class="m_card"><div class="m_sheet_head">' + avatarImg(BOOT.avatarURL, 'lg') +
        '<div><div class="name">' + esc(BOOT.fullname || BOOT.user) + '</div>' +
        (BOOT.user ? '<div class="role">' + esc(BOOT.user) + '</div>' : '') + '</div></div>';
      if (BOOT.mail) { html += '<div class="m_detail_row"><span class="ico">&#9993;</span><a href="mailto:' + esc(BOOT.mail) + '">' + esc(BOOT.mail) + '</a></div>'; }
      if (BOOT.phone) { html += '<div class="m_detail_row"><span class="ico">&#128222;</span><a href="tel:' + esc(BOOT.phone) + '">' + esc(BOOT.phone) + '</a></div>'; }
      html += '</div>';
    } else {
      html += '<div class="m_card"><div class="m_value" style="margin-bottom:12px;">Sign in to access the admin panel and your profile.</div>' +
        '<button class="m_btn" id="m_login_btn" type="button">Log in</button></div>';
    }

    // Display preferences
    html += '<div class="m_section_title">Display preferences</div><div class="m_card">';
    html += toggleRow('setting_shownames', 'Show names on map');
    html += toggleRow('setting_desknumbers', 'Show desk numbers');
    html += toggleRow('setting_highlightleaders', 'Highlight team leaders');
    html += '</div>';

    // Account actions
    html += '<div class="m_section_title">Account</div>' +
      '<button class="m_btn secondary" id="m_fullsite" type="button">Switch to full site</button>';
    if (BOOT.loggedIn) {
      html += '<button class="m_btn danger" id="m_logout" type="button" style="margin-top:10px;">Log out</button>';
    }

    html += '</div>';
    $main.html(html);

    $('#m_login_btn').on('click', function () { navigate('login'); });

    $('#m_fullsite').on('click', function () { location.href = '/?desktop=1'; });
    $('#m_logout').on('click', function () {
      $.ajax({ url: '/rest/account/?mode=logout', type: 'get', dataType: 'json' }).always(function () {
        location.href = '/m/';
      });
    });
  }

  function toggleRow(cookieName, label) {
    var on = getCookie(cookieName) === '1';
    return '<div class="m_toggle"><span>' + esc(label) + '</span>' +
      '<label class="m_switch"><input type="checkbox" data-cookie="' + cookieName + '"' + (on ? ' checked' : '') + '>' +
      '<span class="track"></span></label></div>';
  }

  // =========================================================================
  // Init
  // =========================================================================
  $(function () {
    $main = $('#m_main');
    $tabbar = $('#m_tabbar');
    $sheet = $('#m_sheet');
    $backdrop = $('#m_sheet_backdrop');
    $toast = $('#m_toast');

    // Register the PWA service worker (installability + offline shell). Served
    // from /m/ so it controls the whole mobile scope. Failures are non-fatal.
    if ('serviceWorker' in navigator) {
      navigator.serviceWorker.register('/m/sw.js').catch(function () {});
    }

    // Hide the Admin tab unless the user can view at least one section.
    if (!hasAdmin()) { $tabbar.find('.m_tab[data-view="admin"]').attr('hidden', 'hidden'); }
    else { $tabbar.find('.m_tab[data-view="admin"]').removeAttr('hidden'); }

    // Tab bar navigation. Tapping Map while already on the map toggles the
    // full-screen map selector; otherwise it just shows the current map.
    // Switching to any other tab silently closes the selector.
    $tabbar.find('.m_tab').on('click', function () {
      var view = $(this).data('view');
      if (view === 'map' && currentRoute().view === 'map') {
        if ($('#m_mapselector').hasClass('open')) { closeMapSelector(); }
        else { openMapSelector(); }
        return;
      }
      closeMapSelector(true);
      navigate(view);
    });

    // Backdrop / generic close
    $backdrop.on('click', closeSheet);

    // Display-preference toggles (delegated)
    $(document).on('change', '.m_switch input[data-cookie]', function () {
      setCookie(this.getAttribute('data-cookie'), this.checked ? '1' : '0');
    });

    window.addEventListener('hashchange', route);
    // Back button: if the map selector is open, the popped entry is the one we
    // pushed when opening it, so just dismiss the overlay (stay on the map).
    window.addEventListener('popstate', function () {
      var $ov = $('#m_mapselector');
      if ($ov.hasClass('open') || !$ov.attr('hidden')) {
        mapselPushed = false;
        hideMapSelector(false);
      }
    });
    route();
  });
})();
