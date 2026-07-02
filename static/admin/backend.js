// Additional functions for the admin panel

// Safe global default. The Desks tab overrides this with the real department
// list via an inline <script> before deskSummary() runs; declaring it here means
// deskSummary() can never throw a ReferenceError even if invoked early.
var departments = {};

// ===================================================================
//  AJAX admin navigation
//  Switch tabs and submit forms without full page reloads. The server
//  returns just the #content fragment (the "admincontent" template) for
//  ?partial=1 requests; we swap it in and let jQuery run the inline init
//  <script> blocks so each tab behaves exactly like a full page load.
// ===================================================================
var adminTimers = [];

// adminSetInterval registers a poller that is cleared on the next tab switch,
// so dashboard/health timers don't stack up as the user navigates.
function adminSetInterval(fn, ms) {
  var id = setInterval(fn, ms);
  adminTimers.push(id);
  return id;
}

function clearAdminTimers() {
  for (var i = 0; i < adminTimers.length; i++) { clearInterval(adminTimers[i]); }
  adminTimers = [];
}

// setActiveAdminTab highlights the given tab pill in the header.
function setActiveAdminTab(tab) {
  $(".control_content .headeritem").css({ "background-color": "", "border-radius": "" });
  $("#tab_dashboard").css("background-color", "transparent");
  $("#tab_" + tab).css({ "background-color": "#505050", "border-radius": "50px" });
}

// loadAdminTab fetches a tab's content fragment and swaps it into #content.
function loadAdminTab(tab, sub, push) {
  if (!tab) { tab = 'dashboard'; }
  clearAdminTimers();
  var q = '?tab=' + encodeURIComponent(tab);
  if (sub) { q += '&sub=' + encodeURIComponent(sub); }
  $('#content').html('<br />\n<div style="margin:20px;color:#aaa;">Loading\u2026</div>');
  $.ajax({
    url: q + '&partial=1',
    type: 'get',
    dataType: 'html',
    success: function (html) {
      $('#content').html('<br />\n' + html);
      setActiveAdminTab(tab === 'saml' ? 'ldap' : tab);
      if (push !== false) { history.pushState({ tab: tab, sub: sub }, '', q); }
    },
    error: function () {
      $('#content').html('<br />\n<div style="margin:20px;color:#f88;">Could not load this tab. <a href="' + q + '">Reload</a></div>');
    }
  });
}

// submitAdminForm posts a form via AJAX and swaps the returned content fragment
// in place, so saving a setting refreshes the tab (lists + status banner)
// without a full page reload.
function submitAdminForm(form) {
  var action = $(form).attr('action') || '';
  var m = action.match(/[?&]tab=([^&]+)/);
  var tab = m ? decodeURIComponent(m[1]) : ($(form).find('input[name=tab]').val() || 'dashboard');
  var subM = action.match(/[?&]sub=([^&]+)/);
  var sub = subM ? decodeURIComponent(subM[1]) : '';
  // Fall back to the subtab currently shown (tracked in the URL) so saving a
  // Sync-tab form keeps the user on their subtab instead of resetting to LDAP.
  if (!sub) {
    try { sub = new URLSearchParams(window.location.search).get('sub') || ''; } catch (e) { sub = ''; }
  }
  var q = '?tab=' + encodeURIComponent(tab);
  if (sub) { q += '&sub=' + encodeURIComponent(sub); }
  $.ajax({
    url: q + '&partial=1',
    type: 'post',
    data: new FormData(form),
    processData: false,
    contentType: false,
    dataType: 'html',
    success: function (html) {
      clearAdminTimers();
      $('#content').html('<br />\n' + html);
      setActiveAdminTab(tab === 'saml' ? 'ldap' : tab);
    },
    error: function () { alert('Could not save. Please try again.'); }
  });
  return false;
}

$(function () {
  // Intercept header tab links.
  $(document).on('click', '.control_content .headeritem a[href^="?tab="]', function (e) {
    e.preventDefault();
    var href = $(this).attr('href');
    var p = new URLSearchParams(href.substring(href.indexOf('?')));
    loadAdminTab(p.get('tab'), p.get('sub'), true);
  });
  // Intercept admin form submissions inside the content area. If an inline
  // onsubmit="return confirm(...)" already cancelled the event (user clicked
  // Cancel), defaultPrevented is true and we leave it alone.
  $(document).on('submit', '#content form', function (e) {
    if (e.isDefaultPrevented()) { return; }
    e.preventDefault();
    return submitAdminForm(this);
  });
  // Back/forward navigation.
  window.addEventListener('popstate', function () {
    var p = new URLSearchParams(window.location.search);
    loadAdminTab(p.get('tab') || 'dashboard', p.get('sub'), false);
  });
});

function submitWhitelist(WLtype, WLtext) {
  console.log('add to whitelist: '+WLtext+', '+WLtype);
  document.getElementById("ignoreHealthType").value = WLtype;
  document.getElementById("ignoreHealthName").value = WLtext;
  // Use trigger() (not the native .submit()) so the delegated AJAX submit
  // handler runs instead of a full page reload.
  $('#updateWhitelist').trigger('submit');
}

// healthEsc escapes a string for safe insertion as HTML text/attribute content.
function healthEsc(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

// healthJsArg returns a quoted JS string literal that is also safe inside a
// double-quoted HTML onclick attribute (escapes JS quotes/backslash and HTML
// specials so the value can never break out of the attribute).
function healthJsArg(s) {
  s = String(s == null ? '' : s)
    .replace(/\\/g, '\\\\')
    .replace(/'/g, "\\'")
    .replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;');
  return "'" + s + "'";
}

function updateHealthDetails() {
  $.ajax({
    url: '../rest/system?healthdetails=1',
    async: true, 
    type: 'get',
    dataType: 'JSON',
    beforeSend: function() {
      // Create two containers for LDAP and Desks
      var healthdetails = ''
      + '<div id="healthldap" style="width:780px; height:auto; float:left; margin-left:20px;">'
      + '<img src="../images/spinner.png" style="margin-left:262px;" />'
      + '</div>'
      + '<div id="healthdesks" style="width:780px; height:auto; float:right; margin-right:10px;">'
      + '<img src="../images/spinner.png" style="margin-left:262px;" />'
      + '</div>'

      var checkdiv = document.getElementById('healthdetails')
      if (checkdiv === null) {
        var root = document.getElementById('content')
        var newElement = document.createElement('div')
        newElement.setAttribute('id', 'healthdetails')
        newElement.innerHTML = healthdetails
        root.appendChild(newElement)  
      }
    },
    success: function(result){

      // ---- LDAP consistency (left column) ----
      // One card per over-occupied office; the tile shows the total number of
      // people involved (result.consistency_ldap), not the number of offices.
      var ldaparray = result.health.ldap || [];
      var ldapTotal = result.consistency_ldap;
      var ldapColor = ldapTotal >= 30 ? 'red' : (ldapTotal >= 1 ? 'orange' : 'green');
      var healthldap = ''
        + '<div style="width:750px; margin-left:20px; background:'+ldapColor+'; opacity:0.85; text-align:center; border-radius:8px; padding:24px 0;">'
        + '<h1 style="margin:0;">LDAP errors</h1><h2 style="margin:4px 0 0;">'+ldapTotal+'</h2>'
        + '</div>';
      for (var i = 0; i < ldaparray.length; i++) {
        var lo = ldaparray[i];
        healthldap += ''
          + '<div style="width:750px; margin-left:20px; margin-top:10px; background:#3a3a3a; border-left:5px solid '+ldapColor+'; border-radius:8px; padding:14px 16px; box-sizing:border-box;">'
          + '<div style="font-size:16px; font-weight:bold; color:#eee;">Office &ldquo;'+healthEsc(lo.desk)+'&rdquo; is shared by '+lo.count+' people</div>'
          + '<div style="margin:6px 0 10px; color:#bbb; font-size:13px; line-height:1.4;">More than four people are mapped to the same office in the directory. Fix the office/room attribute for the people below in your directory source, then re-sync &mdash; or ignore this office to accept it.</div>'
          + '<div style="color:#ccc; font-size:13px; margin-bottom:12px;">'+healthEsc((lo.names || []).join(', '))+'</div>'
          + '<a href="javascript:{}" onclick="submitWhitelist(\'ldap\','+healthJsArg(lo.desk)+')" style="display:inline-block; background:#505050; color:#fff; padding:6px 14px; border-radius:6px; text-decoration:none; font-size:13px;">Ignore</a>'
          + '</div>';
      }
      document.getElementById('healthldap').innerHTML = healthldap;

      // ---- Desk consistency (right column) ----
      // One card per duplicated desk name on a map. The tile shows the total
      // number of desks involved (result.consistency_desks), so four desks named
      // "Desk" read as 4 errors in a single card instead of 16 across four rows.
      var deskarray = result.health.desks || [];
      var deskTotal = result.consistency_desks;
      var deskColor = deskTotal >= 5 ? 'red' : (deskTotal >= 1 ? 'orange' : 'green');
      var healthdesks = ''
        + '<div style="width:750px; margin-right:10px; background:'+deskColor+'; opacity:0.85; text-align:center; border-radius:8px; padding:24px 0;">'
        + '<h1 style="margin:0;">Desk errors</h1><h2 style="margin:4px 0 0;">'+deskTotal+'</h2>'
        + '</div>';
      for (var k = 0; k < deskarray.length; k++) {
        var de = deskarray[k];
        var members = de.members || [];
        var memberRows = '';
        for (var m = 0; m < members.length; m++) {
          var who = (members[m].employee && String(members[m].employee).trim()) ? members[m].employee : '(unassigned)';
          var dept = members[m].department ? ' \u2014 ' + members[m].department : '';
          memberRows += '<li style="margin:2px 0;">' + healthEsc(who) + healthEsc(dept) + '</li>';
        }
        healthdesks += ''
          + '<div style="width:750px; margin-right:10px; margin-top:10px; background:#3a3a3a; border-left:5px solid '+deskColor+'; border-radius:8px; padding:14px 16px; box-sizing:border-box;">'
          + '<div style="font-size:16px; font-weight:bold; color:#eee;">&ldquo;'+healthEsc(de.desk)+'&rdquo; is used by '+de.count+' desks on map '+healthEsc(de.map)+'</div>'
          + '<div style="margin:6px 0 10px; color:#bbb; font-size:13px; line-height:1.4;">Desk names must be unique on a map. Open the map in edit mode &mdash; the affected desks are highlighted with a red ring &mdash; and rename each duplicate to a unique number, or ignore this name to accept it.</div>'
          + '<ul style="margin:0 0 12px 18px; color:#ccc; font-size:13px;">'+memberRows+'</ul>'
          + '<a href="../?map='+encodeURIComponent(de.map)+'" style="display:inline-block; background:#0979D8; color:#fff; padding:6px 14px; border-radius:6px; text-decoration:none; font-size:13px; margin-right:8px;">Show on map</a>'
          + '<a href="javascript:{}" onclick="submitWhitelist(\'desks\','+healthJsArg(de.desk)+')" style="display:inline-block; background:#505050; color:#fff; padding:6px 14px; border-radius:6px; text-decoration:none; font-size:13px;">Ignore</a>'
          + '</div>';
      }
      document.getElementById('healthdesks').innerHTML = healthdesks;

      console.log('[HealthDetails] updated');
    }
  })
}

function updateSystemStats() {
  $.ajax({
    url: '../rest/system',
    async: true, 
    type: 'get',
    dataType: 'JSON',
    beforeSend: function() {
      var element = document.getElementById('systemstats');
      if (element === null) {
        var systemspinner = ''
        + '<div id="spinner" style="width:100%; height:auto; box-sizing:border-box; text-align:center;">'
        + '<img src="../images/spinner.png" />'
        + '</div>'

        var checkdiv = document.getElementById('systemspinner')
        if (checkdiv === null) {
          var root = document.getElementById('content')
          var newElement = document.createElement('div')
          newElement.setAttribute('id', 'systemspinner')
          newElement.innerHTML = systemspinner
          root.appendChild(newElement)  
        }
      }
    },
    success: function(result){

      var element = document.getElementById('systemspinner');
      if (element !== null) {
       element.parentNode.removeChild(element);
      }

      var color='green'
      var percentage = result.cpuload
      if (percentage >=95 ) {color='red';}
      else if (percentage >= 85) {color='orange';}
      else {color='green';}
      var systemstats = ''
      + '<div id="cpuload" style="width:300px; height:300px; float:left; margin-left:10px; margin-right:15px; background:'+color+'; opacity:0.7; text-align:center;line-height:300px;">'
      + '<span style="display: inline-block; vertical-align: middle; line-height: normal;">'
      + '<h1>CPU Load</h1><h2>'+percentage+'</h2>'
      + '</span>'
      + '</div>'

      var color='green'
      var percentage = result.memoryused
      if (percentage >=95 ) {color='red';}
      else if (percentage >= 85) {color='orange';}
      else {color='green';}
      systemstats += ''
      + '<div id="memoryused" style="width:300px; height:300px; float:left; margin-right:15px; background:'+color+'; opacity:0.7; text-align:center;line-height:300px;">'
      + '<span style="display: inline-block; vertical-align: middle; line-height: normal;">'
      + '<h1>Memory used</h1><h2>'+percentage+'</h2>'
      + '</span>'
      + '</div>'

      var color='green'
      var percentage = result.diskused
      if (percentage >=95 ) {color='red';}
      else if (percentage >= 85) {color='orange';}
      else {color='green';}
      systemstats += ''
      + '<div id="diskused" style="width:300px; height:300px; float:left; margin-right:15px; background:'+color+'; opacity:0.7; text-align:center;line-height:300px;">'
      + '<span style="display: inline-block; vertical-align: middle; line-height: normal;">'
      + '<h1>Disk used</h1><h2>'+percentage+'</h2>'
      + '</span>'
      + '</div>'

      var color='green'
      var percentage = result.consistency_ldap
      if (percentage >=30 ) {color='red';}
      else if (percentage >= 1) {color='orange';}
      else {color='green';}
      systemstats += ''
      + '<div id="consistency_ldap" style="width:300px; height:300px; float:left; margin-right:15px; background:'+color+'; opacity:0.7; text-align:center;line-height:300px;">'
      + '<span style="display: inline-block; vertical-align: middle; line-height: normal;">'
      + '<h1>LDAP Consistency Errors</h1><h2>'+percentage+'</h2>'
      + '</span>'
      + '</div>'

      var color='green'
      var percentage = result.consistency_desks
      if (percentage >=5 ) {color='red';}
      else if (percentage >= 1) {color='orange';}
      else {color='green';}
      systemstats += ''
      + '<div id="consistency_desks" style="width:300px; height:300px; float:left; margin-right:15px; background:'+color+'; opacity:0.7; text-align:center;line-height:300px;">'
      + '<span style="display: inline-block; vertical-align: middle; line-height: normal;">'
      + '<h1>Desks Consistency Errors</h1><h2>'+percentage+'</h2>'
      + '</span>'
      + '</div>'

      var element = document.getElementById('systemstats');
      if (element !== null) {
       element.parentNode.removeChild(element);
      }

      var p = document.getElementById('content')
      var newElement = document.createElement('div')
      newElement.setAttribute('id', 'systemstats')
      newElement.innerHTML = systemstats
      p.appendChild(newElement)

      console.log('[SystemStats] updated');
    }
  })
}

function syncLDAP(ldap_id, adminuser) {
  var button_div = 'syncbutton'+ldap_id
  console.log('Sync started for LDAP connection #'+ldap_id)
  $("#"+button_div).css("background-color","#404040");
  document.getElementById(button_div).value = "Syncing..."
  $.ajax({
    url: '../rest/ldap/?token='+token+'&ldapid='+ldap_id+'&user='+adminuser,
    async: true, 
    type: 'get',
    dataType: 'JSON',
    success: function(result){
      console.log('[LDAP] updated')
      console.log(result)
      $("#"+button_div).css("background-color","rgba(0, 100, 0, 1.0)");
      document.getElementById(button_div).value = "Success"
    },
    error: function() {
      console.log('[LDAP] update failed');
      $("#"+button_div).css("background-color","rgba(150, 0, 0, 1.0)");
      document.getElementById(button_div).value = "Error"
    }
  })
}

// testLDAP validates a single connection's server + bind credentials without
// running a sync and shows a structured summary in the shared test modal.
function testLDAP(id) {
  var btn = document.getElementById('ldaptestbtn' + id);
  var desc = btn ? (btn.getAttribute('data-desc') || '') : '';
  var title = 'LDAP connection test' + (desc ? ' \u2014 ' + desc : '');
  var body = openTestModal(title, 'Testing ' + (desc || 'connection') + '\u2026');
  if (btn) { btn.disabled = true; }
  $.ajax({
    url: '../rest/ldap/test?token=' + token + '&ldapid=' + encodeURIComponent(id),
    async: true, type: 'get', dataType: 'JSON',
    success: function(d) {
      if (body) body.innerHTML = renderTestChecks(d, 'Connection looks valid', 'Connection has problems');
    },
    error: function() {
      if (body) body.textContent = 'Request failed (forbidden or server error).';
    },
    complete: function() { if (btn) btn.disabled = false; }
  });
}

// toggleLdapEnabled activates/deactivates a connection in place (no full reload)
// and shows/hides its "Sync now" button to match.
function toggleLdapEnabled(id, cb) {
  var on = !!(cb && cb.checked);
  if (cb) cb.disabled = true;
  var body = 'toggleLdapID=' + encodeURIComponent(id) + '&ldapEnabled=' + (on ? '1' : '0');
  fetch('?tab=ldap&partial=1', {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    credentials: 'same-origin',
    body: body
  }).then(function() {
    if (cb) cb.disabled = false;
    var syncBtn = document.getElementById('syncbutton' + id);
    if (syncBtn) {
      syncBtn.disabled = !on;
      syncBtn.title = on ? '' : 'Enable this connection to sync it';
    }
  }).catch(function() {
    if (cb) { cb.disabled = false; cb.checked = !on; }
  });
}

// editLdapSource pre-fills the add/edit form from a row's data attributes and
// switches it into edit mode (the password is left blank to keep the stored one).
function editLdapSource(btn) {
  var d = btn.dataset;
  var set = function(elid, val) { var el = document.getElementById(elid); if (el) el.value = val || ''; };
  set('ldapFormEditID', d.id);
  set('ldapFDescription', d.desc);
  set('ldapFServer', d.server);
  set('ldapFOU', d.ou);
  set('ldapFUser', d.user);
  var typeSel = document.getElementById('ldapFType');
  if (typeSel) typeSel.value = (String(d.type || '').toUpperCase() === 'LDAP') ? 'LDAP' : 'LDAPS';
  var pass = document.getElementById('ldapFPass');
  if (pass) { pass.value = ''; pass.placeholder = 'Leave blank to keep current password'; }
  var submit = document.getElementById('ldapFormSubmit');
  if (submit) submit.textContent = 'Save changes';
  var cancel = document.getElementById('ldapFormCancel');
  if (cancel) cancel.style.display = '';
  var collapse = document.getElementById('ldapAddCollapse');
  if (collapse && !collapse.classList.contains('open')) {
    toggleCollapse('ldapAddCollapse', document.getElementById('ldapAddToggle'));
  }
  if (collapse && collapse.scrollIntoView) collapse.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
}

// cancelEditLdap resets the add/edit form back to "add a connection" mode.
function cancelEditLdap() {
  var form = document.getElementById('ldapForm');
  if (form) form.reset();
  var editId = document.getElementById('ldapFormEditID');
  if (editId) editId.value = '';
  var pass = document.getElementById('ldapFPass');
  if (pass) pass.placeholder = 'Password';
  var submit = document.getElementById('ldapFormSubmit');
  if (submit) submit.textContent = 'Create connection';
  var cancel = document.getElementById('ldapFormCancel');
  if (cancel) cancel.style.display = 'none';
  var collapse = document.getElementById('ldapAddCollapse');
  if (collapse && collapse.classList.contains('open')) {
    toggleCollapse('ldapAddCollapse', document.getElementById('ldapAddToggle'));
  }
}

function showSyncSub(name) {
  var subs = ['ldap', 'entra', 'saml', 'robin', 'geo', 'database'];
  // Fall back to the first available subsection if the requested one is not
  // rendered (e.g. the user lacks the matching permission).
  if (!document.getElementById('syncsub_' + name)) {
    name = null;
    for (var i = 0; i < subs.length; i++) {
      if (document.getElementById('syncsub_' + subs[i])) { name = subs[i]; break; }
    }
  }
  subs.forEach(function(s) {
    var content = document.getElementById('syncsub_' + s);
    var nav = document.getElementById('syncnav_' + s);
    if (content) content.style.display = (s === name) ? 'block' : 'none';
    if (nav) nav.classList.toggle('active', s === name);
  });
  // Lazily load the database browser bucket list the first time it is shown.
  if (name === 'database') { dbInit(); }
  // Persist the active subtab in the URL so a full page reload restores it.
  if (name) {
    try {
      var p = new URLSearchParams(window.location.search);
      if (p.get('sub') !== name) {
        p.set('sub', name);
        history.replaceState(history.state, '', '?' + p.toString());
      }
    } catch (e) { /* history API unavailable: ignore */ }
  }
}

// ---- Read-only database browser (Sync > Database subtab) ----
var dbState = { loaded: false, offset: 0, limit: 50, total: 0 };

function dbEscape(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function dbPretty(val) {
  try { return JSON.stringify(JSON.parse(val), null, 2); }
  catch (e) { return val; }
}

// dbInit loads the bucket list once, the first time the tab is opened.
function dbInit() {
  if (dbState.loaded) { return; }
  dbState.loaded = true;
  var sel = document.getElementById('dbBucketSelect');
  var status = document.getElementById('dbBrowseStatus');
  if (status) { status.textContent = 'Loading buckets\u2026'; }
  $.ajax({
    url: '../rest/db/buckets?token=' + token,
    async: true, type: 'get', dataType: 'JSON',
    success: function(d) {
      var buckets = (d && d.buckets) || [];
      var html = '<option value="">Select a bucket\u2026</option>';
      buckets.forEach(function(b) {
        html += '<option value="' + dbEscape(b.name) + '">' + dbEscape(b.name) + ' (' + b.count + ')</option>';
      });
      sel.innerHTML = html;
      if (status) { status.textContent = buckets.length + ' bucket(s). Choose one to browse.'; }
    },
    error: function() {
      dbState.loaded = false;
      if (status) { status.textContent = 'Failed to load buckets.'; }
    }
  });
}

function dbSelectBucket() {
  dbState.offset = 0;
  dbLoadEntries(true);
}

function dbPage(dir) {
  var next = dbState.offset + dir * dbState.limit;
  if (next < 0) { next = 0; }
  if (next >= dbState.total) { return; }
  dbState.offset = next;
  dbLoadEntries(false);
}

function dbLoadEntries(reset) {
  var sel = document.getElementById('dbBucketSelect');
  var bucket = sel ? sel.value : '';
  var status = document.getElementById('dbBrowseStatus');
  var table = document.getElementById('dbBrowseTable');
  var pager = document.getElementById('dbBrowsePager');
  if (!bucket) {
    if (table) { table.style.display = 'none'; }
    if (pager) { pager.style.display = 'none'; }
    if (status) { status.textContent = 'Select a bucket to browse.'; }
    return;
  }
  if (reset) { dbState.offset = 0; }
  var search = (document.getElementById('dbSearchInput') || {}).value || '';
  if (status) { status.textContent = 'Loading\u2026'; }
  $.ajax({
    url: '../rest/db/entries?token=' + token +
      '&bucket=' + encodeURIComponent(bucket) +
      '&search=' + encodeURIComponent(search) +
      '&offset=' + dbState.offset + '&limit=' + dbState.limit,
    async: true, type: 'get', dataType: 'JSON',
    success: function(d) {
      dbState.total = (d && d.total) || 0;
      var entries = (d && d.entries) || [];
      var body = document.getElementById('dbBrowseBody');
      if (!entries.length) {
        body.innerHTML = '<tr><td colspan="2" class="sync-empty">No matching entries.</td></tr>';
      } else {
        var rows = '';
        entries.forEach(function(e) {
          rows += '<tr><td class="sync-td-min"><code>' + dbEscape(e.key) + '</code></td>' +
            '<td><pre style="margin:0; white-space:pre-wrap; word-break:break-word; max-height:300px; overflow:auto;">' +
            dbEscape(dbPretty(e.value)) + '</pre></td></tr>';
        });
        body.innerHTML = rows;
      }
      if (table) { table.style.display = ''; }
      var from = dbState.total ? dbState.offset + 1 : 0;
      var to = Math.min(dbState.offset + dbState.limit, dbState.total);
      if (status) { status.textContent = 'Showing ' + from + '\u2013' + to + ' of ' + dbState.total; }
      if (pager) {
        pager.style.display = dbState.total > dbState.limit ? 'flex' : 'none';
        var prev = document.getElementById('dbPrevBtn');
        var next = document.getElementById('dbNextBtn');
        var info = document.getElementById('dbPageInfo');
        if (prev) { prev.disabled = dbState.offset <= 0; }
        if (next) { next.disabled = dbState.offset + dbState.limit >= dbState.total; }
        if (info) { info.textContent = from + '\u2013' + to + ' / ' + dbState.total; }
      }
    },
    error: function() {
      if (status) { status.textContent = 'Failed to load entries.'; }
    }
  });
}

function showLdapDebug() {
  var body = document.getElementById('ldapDebugBody');
  body.innerHTML = 'Loading...';
  document.getElementById('ldapDebugOverlay').style.display = 'block';
  $.ajax({
    url: '../rest/ldap/debug?token='+token,
    async: true,
    type: 'get',
    dataType: 'JSON',
    success: function(d) {
      body.innerHTML = renderLdapDebug(d);
    },
    error: function() {
      body.innerHTML = '<span style="color:#a00;">Failed to load debug data (forbidden or server error).</span>';
    }
  })
}

function esc(s) {
  return String(s == null ? '' : s)
    .replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

// renderTestChecks turns a {ok, checks:[{name,status,detail}]} test result into
// the shared SAML/Robin-style summary markup (a header line plus one tinted
// status icon per check).
function renderTestChecks(res, okText, failText) {
  var cls = { ok: 'status-icon-ok', warn: 'status-icon-warn', fail: 'status-icon-fail' };
  var html = '<div style="display:flex; align-items:center; gap:10px; font-weight:700; margin-bottom:12px;">' +
    '<span class="status-icon ' + (res && res.ok ? 'status-icon-ok' : 'status-icon-fail') + '"></span>' +
    ((res && res.ok) ? okText : failText) + '</div>';
  ((res && res.checks) || []).forEach(function(c) {
    html += '<div style="display:flex; gap:10px; padding:6px 0; border-top:1px solid rgba(255,255,255,0.08);">' +
      '<span class="status-icon ' + (cls[c.status] || '') + '" style="margin-top:2px;"></span>' +
      '<span><strong>' + esc(c.name) + '</strong><br><span style="color:#aaa;">' + esc(c.detail) + '</span></span>' +
      '</div>';
  });
  return html;
}

// openTestModal shows the shared connection-test modal with the given title and
// a loading placeholder, returning the body element for the caller to fill in.
function openTestModal(title, loadingText) {
  var t = document.getElementById('testResultTitle');
  var body = document.getElementById('testResultBody');
  var overlay = document.getElementById('testResultOverlay');
  if (t) t.textContent = title || 'Connection test';
  if (body) body.innerHTML = esc(loadingText || 'Testing\u2026');
  if (overlay) overlay.style.display = 'block';
  return body;
}

function showRobinTest() {
  var body = document.getElementById('robinDebugBody');
  var overlay = document.getElementById('robinDebugOverlay');
  if (body) body.innerHTML = 'Checking Robin credentials\u2026';
  if (overlay) overlay.style.display = 'block';
  $.ajax({
    url: '../rest/robin/test',
    async: true,
    type: 'get',
    dataType: 'JSON',
    success: function(res) {
      if (!body) return;
      var cls = { ok: 'status-icon-ok', warn: 'status-icon-warn', fail: 'status-icon-fail' };
      var html = '<div style="display:flex; align-items:center; gap:10px; font-weight:700; margin-bottom:12px;">' +
        '<span class="status-icon ' + (res.ok ? 'status-icon-ok' : 'status-icon-fail') + '"></span>' +
        (res.ok ? 'Robin connection looks valid' : 'Robin connection has problems') + '</div>';
      (res.checks || []).forEach(function(c) {
        html += '<div style="display:flex; gap:10px; padding:6px 0; border-top:1px solid rgba(255,255,255,0.08);">' +
          '<span class="status-icon ' + (cls[c.status] || '') + '" style="margin-top:2px;"></span>' +
          '<span><strong>' + esc(c.name) + '</strong><br><span style="color:#aaa;">' + esc(c.detail) + '</span></span>' +
          '</div>';
      });
      body.innerHTML = html;
    },
    error: function() {
      if (body) body.textContent = 'Failed to run the connection test (forbidden or server error).';
    }
  })
}

function renderLdapDebug(d) {
  if (!d || !d.when) {
    return '<p>No sync has run yet since the server started. Click "Sync now" on a source first.</p>';
  }
  var h = '<p><b>When:</b> ' + esc(d.when) + ' &nbsp; <b>Total mirrored:</b> ' + esc(d.total) + '</p>';
  var srcs = d.sources || [];
  if (srcs.length === 0) {
    h += '<p>No sources were processed (none configured?).</p>';
    return h;
  }
  for (var i = 0; i < srcs.length; i++) {
    var s = srcs[i];
    h += '<div style="border:1px solid #ccc;border-radius:4px;padding:10px;margin-bottom:12px;">';
    h += '<div style="font-weight:bold;font-size:14px;">' + esc(s.description || '(no description)') + '</div>';
    h += '<table style="border:0;margin-top:6px;">';
    h += '<tr><td style="padding-right:14px;">Server</td><td>' + esc(s.server) + ' (' + esc(s.type) + ')</td></tr>';
    h += '<tr><td>OU</td><td>' + esc(s.ou) + '</td></tr>';
    h += '<tr><td>Bind user</td><td>' + esc(s.bind_user) + '</td></tr>';
    h += '<tr><td>Connected</td><td>' + (s.connected ? 'yes' : 'NO') + '</td></tr>';
    h += '<tr><td>Bound</td><td>' + (s.bound ? 'yes' : 'NO') + '</td></tr>';
    h += '<tr><td>Entries found</td><td>' + esc(s.entries_found) + '</td></tr>';
    h += '<tr><td>Mirrored</td><td>' + esc(s.mirrored) + '</td></tr>';
    h += '<tr><td>Skipped</td><td>' + esc(s.skipped) + '</td></tr>';
    h += '</table>';
    if (s.error) {
      h += '<div style="color:#a00;margin-top:6px;"><b>Error:</b> ' + esc(s.error) + '</div>';
    }
    if (s.skip_reasons && Object.keys(s.skip_reasons).length > 0) {
      h += '<div style="margin-top:6px;"><b>Skip reasons:</b><ul style="margin:4px 0;">';
      for (var k in s.skip_reasons) {
        h += '<li>' + esc(k) + ': ' + esc(s.skip_reasons[k]) + '</li>';
      }
      h += '</ul></div>';
    }
    if (s.attribute_names && s.attribute_names.length > 0) {
      h += '<div style="margin-top:6px;"><b>Attributes returned by AD (first entry):</b><br>' +
           esc(s.attribute_names.join(', ')) + '</div>';
    } else if (s.entries_found > 0) {
      h += '<div style="margin-top:6px;color:#a60;">Entries were returned but no attributes were captured.</div>';
    } else {
      h += '<div style="margin-top:6px;color:#a60;">No entries matched the search filter ' +
           '<code>(&amp;(physicaldeliveryofficename=*)(givenname=A*..Z*))</code> under the OU.</div>';
    }
    h += '</div>';
  }
  return h;
}

// --- SAML settings tab ---

var samlEditing = false;
var samlConfigured = false;
var samlCurrent = null;

function loadSamlSettings() {
  // SP info (entity ID, ACS URL, metadata/login URLs).
  $.ajax({
    url: '../rest/saml/spinfo', type: 'get', dataType: 'JSON',
    success: function(sp) {
      $('#sp_entity_id').text(sp.entity_id || '-');
      $('#sp_acs_url').text(sp.acs_url || '-');
      $('#sp_metadata_url').text(sp.metadata_url || '-');
      $('#sp_login_url').text(sp.login_url || '-');
    }
  });
  // Current settings.
  $.ajax({
    url: '../rest/saml/settings', type: 'get', dataType: 'JSON',
    success: function(c) {
      document.getElementById('saml_enabled').checked = !!c.enabled;
      document.getElementById('saml_allow_local').checked = !!c.allow_local_password_fallback;
      $('#saml_nameid').val(c.name_id_format || '');
      $('#saml_tenant').val(c.entra_tenant_id || '');
      $('#saml_entity').val(c.entra_entity_id || '');
      $('#saml_login').val(c.entra_login_url || '');
      $('#saml_cert').val(c.entra_x509_certificate || '');
      $('#saml_app_entity').val(c.app_entity_id || '');
      $('#saml_app_reply').val(c.app_reply_url || '');
      $('#saml_app_logout').val(c.app_logout_url || '');
      $('#saml_attr_sam').val(c.attribute_samaccountname || '');
      $('#saml_attr_given').val(c.attribute_givenname || '');
      $('#saml_attr_sn').val(c.attribute_surname || '');
      $('#saml_attr_full').val(c.attribute_fullname || '');
      $('#saml_attr_mail').val(c.attribute_mail || '');

      // "Anything configured" drives whether we show the summary or placeholder.
      samlCurrent = c;
      samlConfigured = !!(c.entra_login_url || c.entra_x509_certificate || c.entra_entity_id || c.entra_tenant_id);
      document.getElementById('saml_sum_toggle').checked = !!c.enabled;
      $('#saml_sum_toggle_label').text(c.enabled ? 'Enabled' : 'Disabled');
      $('#saml_sum_enabled').text(c.enabled ? 'Enabled' : 'Disabled');
      $('#saml_sum_local').text(c.allow_local_password_fallback ? 'Allowed' : 'Not allowed');
      $('#saml_sum_tenant').text(c.entra_tenant_id || '-');
      $('#saml_sum_entity').text(c.entra_entity_id || '-');
      $('#saml_sum_login').text(c.entra_login_url || '-');
      $('#saml_sum_cert').text(c.entra_x509_certificate ? 'Configured' : 'Not set');

      if (!samlEditing) renderSamlView();
    }
  });
}

// renderSamlView shows the summary card when a config exists, otherwise the
// create placeholder, and always hides the setup form.
function renderSamlView() {
  var setup = document.getElementById('samlSetup');
  var summary = document.getElementById('samlSummaryCard');
  var placeholder = document.getElementById('samlPlaceholder');
  if (setup) setup.style.display = 'none';
  if (samlConfigured) {
    if (summary) summary.style.display = '';
    if (placeholder) placeholder.style.display = 'none';
  } else {
    if (summary) summary.style.display = 'none';
    if (placeholder) placeholder.style.display = '';
  }
}

// samlStartCreate / samlStartEdit reveal the 3-step setup form.
function samlStartCreate() { showSamlSetup(); }
function samlStartEdit() { showSamlSetup(); }
function showSamlSetup() {
  samlEditing = true;
  var summary = document.getElementById('samlSummaryCard');
  var placeholder = document.getElementById('samlPlaceholder');
  var setup = document.getElementById('samlSetup');
  if (summary) summary.style.display = 'none';
  if (placeholder) placeholder.style.display = 'none';
  if (setup) setup.style.display = '';
  $('#saml_save_status').text('');
}

// samlCancelEdit discards unsaved edits and returns to the summary/placeholder.
function samlCancelEdit() {
  samlEditing = false;
  loadSamlSettings();
}

// toggleSamlEnabled flips the enabled flag straight from the summary card,
// persisting the rest of the current configuration unchanged.
function toggleSamlEnabled(cb) {
  if (!samlCurrent) return;
  var payload = $.extend({}, samlCurrent, { enabled: cb.checked });
  $('#saml_sum_toggle_label').text(cb.checked ? 'Enabled' : 'Disabled');
  $.ajax({
    url: '../rest/saml/settings', type: 'PUT',
    contentType: 'application/json', data: JSON.stringify(payload), dataType: 'JSON',
    success: function() { loadSamlSettings(); },
    error: function() {
      cb.checked = !cb.checked;
      $('#saml_sum_toggle_label').text(cb.checked ? 'Enabled' : 'Disabled');
      alert('Failed to update SAML status.');
    }
  });
}

// samlTest runs a server-side pre-flight validation of the configuration and
// shows the results inline (no new browser window / IdP round-trip needed).
function samlTest() {
  var body = document.getElementById('samlDebugBody');
  var overlay = document.getElementById('samlDebugOverlay');
  if (body) body.innerHTML = 'Testing\u2026';
  if (overlay) overlay.style.display = 'block';
  $.ajax({
    url: '../rest/saml/validate', type: 'get', dataType: 'JSON',
    success: function(res) {
      if (!body) return;
      var cls = { ok: 'status-icon-ok', warn: 'status-icon-warn', fail: 'status-icon-fail' };
      var html = '<div style="display:flex; align-items:center; gap:10px; font-weight:700; margin-bottom:12px;">' +
        '<span class="status-icon ' + (res.ok ? 'status-icon-ok' : 'status-icon-fail') + '"></span>' +
        (res.ok ? 'Configuration looks valid' : 'Configuration has problems') + '</div>';
      (res.checks || []).forEach(function(c) {
        html += '<div style="display:flex; gap:10px; padding:6px 0; border-top:1px solid rgba(255,255,255,0.08);">' +
          '<span class="status-icon ' + (cls[c.status] || '') + '" style="margin-top:2px;"></span>' +
          '<span><strong>' + esc(c.name) + '</strong><br><span style="color:#aaa;">' + esc(c.detail) + '</span></span>' +
          '</div>';
      });
      body.innerHTML = html;
    },
    error: function() {
      if (body) body.textContent = 'Validation request failed (forbidden or server error).';
    }
  });
}

// samlDelete clears the SAML configuration (keeps local login as a fallback so
// nobody gets locked out).
function samlDelete() {
  if (!confirm('Delete the SAML configuration? Single sign-on will be turned off.')) return;
  $.ajax({
    url: '../rest/saml/settings', type: 'PUT',
    contentType: 'application/json',
    data: JSON.stringify({ enabled: false, allow_local_password_fallback: true }),
    dataType: 'JSON',
    success: function() { samlEditing = false; loadSamlSettings(); },
    error: function() { alert('Failed to delete SAML configuration.'); }
  });
}


function saveSamlSettings() {
  var payload = {
    enabled: document.getElementById('saml_enabled').checked,
    allow_local_password_fallback: document.getElementById('saml_allow_local').checked,
    name_id_format: $('#saml_nameid').val(),
    entra_tenant_id: $('#saml_tenant').val(),
    entra_entity_id: $('#saml_entity').val(),
    entra_login_url: $('#saml_login').val(),
    entra_x509_certificate: $('#saml_cert').val(),
    app_entity_id: $('#saml_app_entity').val(),
    app_reply_url: $('#saml_app_reply').val(),
    app_logout_url: $('#saml_app_logout').val(),
    attribute_samaccountname: $('#saml_attr_sam').val(),
    attribute_givenname: $('#saml_attr_given').val(),
    attribute_surname: $('#saml_attr_sn').val(),
    attribute_fullname: $('#saml_attr_full').val(),
    attribute_mail: $('#saml_attr_mail').val()
  };
  $('#saml_save_status').css('color', '#aaa').text('Saving...');
  $.ajax({
    url: '../rest/saml/settings', type: 'PUT',
    contentType: 'application/json', data: JSON.stringify(payload), dataType: 'JSON',
    success: function() {
      $('#saml_save_status').css('color', '#2ecc71').text('Saved.');
      samlEditing = false;
      loadSamlSettings();
    },
    error: function() {
      $('#saml_save_status').css('color', '#e74c3c').text('Save failed.');
    }
  });
}

function importSamlMetadata() {
  var url = $('#saml_metadata_import_url').val();
  if (!url) { $('#saml_import_status').css('color', '#e74c3c').text('Enter a metadata URL first.'); return; }
  $('#saml_import_status').css('color', '#aaa').text('Importing...');
  $.ajax({
    url: '../rest/saml/import-metadata', type: 'POST',
    contentType: 'application/json', data: JSON.stringify({url: url}), dataType: 'JSON',
    success: function(res) {
      if (res.error) {
        $('#saml_import_status').css('color', '#e74c3c').text('Error: ' + res.error);
        return;
      }
      $('#saml_import_status').css('color', '#2ecc71').text('Imported. Review and click Save.');
      loadSamlSettings();
    },
    error: function() {
      $('#saml_import_status').css('color', '#e74c3c').text('Import request failed.');
    }
  });
}

function showSamlDebug() {
  var body = document.getElementById('samlDebugBody');
  body.textContent = 'Loading...';
  document.getElementById('samlDebugOverlay').style.display = 'block';
  $.ajax({
    url: '../rest/saml/debug', type: 'get', dataType: 'JSON',
    success: function(d) {
      body.textContent = JSON.stringify(d, null, 2);
    },
    error: function() {
      body.textContent = 'Failed to load debug data (forbidden or server error).';
    }
  });
}

// ── Collapsible (one-time config) sections ──────────────────
function toggleCollapse(id, btn) {
  var el = document.getElementById(id);
  if (!el) return;
  var open = el.classList.toggle('open');
  if (btn) btn.classList.toggle('open', open);
}

// ── Background sync with progress bar + live log ────────────
function renderSyncProgress(prefix, snap) {
  var wrap = document.getElementById(prefix + 'Progress');
  var fill = document.getElementById(prefix + 'ProgFill');
  var stage = document.getElementById(prefix + 'ProgStage');
  var count = document.getElementById(prefix + 'ProgCount');
  var logEl = document.getElementById(prefix + 'Log');

  if (wrap) wrap.classList.add('show');
  if (stage) stage.textContent = snap.stage || (snap.done ? 'Done' : 'Working…');

  if (fill) {
    if (snap.total > 0) {
      fill.classList.remove('indeterminate');
      var pct = Math.round((snap.cur / snap.total) * 100);
      if (pct > 100) pct = 100;
      fill.style.width = pct + '%';
      if (count) count.textContent = snap.cur + ' / ' + snap.total;
    } else {
      fill.classList.add('indeterminate');
      if (count) count.textContent = '';
    }
  }

  if (logEl) {
    var lines = snap.log || [];
    logEl.style.display = lines.length ? 'block' : 'none';
    logEl.textContent = lines.join('\n');
    logEl.scrollTop = logEl.scrollHeight;
  }
}

function startSync(prefix, startUrl, progressUrl, subTab) {
  var btn = document.getElementById(prefix + 'SyncBtn');
  if (btn) { btn.disabled = true; btn.textContent = 'Syncing…'; }
  var logEl = document.getElementById(prefix + 'Log');
  if (logEl) { logEl.textContent = ''; logEl.style.display = 'block'; }

  $.ajax({
    url: startUrl, type: 'POST', dataType: 'JSON',
    complete: function() { pollSync(prefix, progressUrl, subTab); }
  });
}

function pollSync(prefix, progressUrl, subTab) {
  var timer = setInterval(function() {
    $.ajax({
      url: progressUrl, type: 'GET', dataType: 'JSON',
      success: function(snap) {
        renderSyncProgress(prefix, snap);
        if (!snap.running && snap.done) {
          clearInterval(timer);
          var fill = document.getElementById(prefix + 'ProgFill');
          if (fill) {
            fill.classList.remove('indeterminate');
            fill.style.width = '100%';
            if (snap.error) fill.style.background = 'var(--sy-danger)';
          }
          var stage = document.getElementById(prefix + 'ProgStage');
          if (stage) stage.textContent = snap.error ? ('Error: ' + snap.error) : (snap.summary || 'Done');
          var btn = document.getElementById(prefix + 'SyncBtn');
          if (btn) { btn.disabled = false; btn.textContent = 'Run sync now'; }
          // Offer to refresh the structured "last sync" view without wiping the
          // log the user just ran the sync to read.
          var reloadBtn = document.getElementById(prefix + 'ReloadBtn');
          if (reloadBtn) {
            reloadBtn.style.display = 'inline-flex';
            reloadBtn.onclick = function() { loadAdminTab('ldap', subTab, true); };
          }
        }
      },
      error: function() { clearInterval(timer); }
    });
  }, 800);
}

function startRobinSync() {
  var btn = document.getElementById('robinSyncBtn');
  if (btn) { btn.disabled = true; btn.textContent = 'Syncing\u2026'; }
  $.ajax({
    url: '../rest/robin/sync', type: 'POST', dataType: 'JSON',
    complete: function () { pollRobinSync(); }
  });
}

// toggleRobinEnabled switches the whole Robin integration on/off. The saved
// token and options are kept; disabling only stops syncing and map overlays.
function toggleRobinEnabled(cb) {
  var enabled = cb.checked;
  $('#robinEnabledLabel').text(enabled ? 'Enabled' : 'Disabled');
  var btn = document.getElementById('robinSyncBtn');
  if (btn) { btn.disabled = !enabled; btn.title = enabled ? '' : 'Robin integration is disabled'; }
  $.ajax({
    url: '../rest/robin/enabled?enabled=' + (enabled ? '1' : '0'), type: 'POST', dataType: 'JSON',
    error: function () {
      cb.checked = !enabled;
      $('#robinEnabledLabel').text(cb.checked ? 'Enabled' : 'Disabled');
      if (btn) { btn.disabled = !cb.checked; btn.title = cb.checked ? '' : 'Robin integration is disabled'; }
      alert('Failed to update Robin status.');
    }
  });
}

// toggleGeoEnabled switches the geocoding integration on/off. The saved API key
// is kept; disabling only blocks manual geocode syncs/tests.
function toggleGeoEnabled(cb) {
  var enabled = cb.checked;
  $('#geoEnabledLabel').text(enabled ? 'Enabled' : 'Disabled');
  var btn = document.getElementById('geoSyncBtn');
  if (btn) { btn.disabled = !enabled; btn.title = enabled ? '' : 'Geocoding integration is disabled'; }
  $.ajax({
    url: '../rest/geo/enabled?enabled=' + (enabled ? '1' : '0'), type: 'POST', dataType: 'JSON',
    error: function () {
      cb.checked = !enabled;
      $('#geoEnabledLabel').text(cb.checked ? 'Enabled' : 'Disabled');
      if (btn) { btn.disabled = !cb.checked; btn.title = cb.checked ? '' : 'Geocoding integration is disabled'; }
      alert('Failed to update geocoding status.');
    }
  });
}

// deleteRobinConfig clears the saved Robin token and organisation id.
function deleteRobinConfig() {
  if (!confirm('Delete the Robin access token and organisation id? This cannot be undone.')) return;
  $.ajax({
    url: '../rest/robin/delete', type: 'POST', dataType: 'JSON',
    success: function () { window.location.reload(); },
    error: function () { alert('Failed to delete Robin configuration.'); }
  });
}

// deleteGeoConfig clears the saved Geoapify API key.
function deleteGeoConfig() {
  if (!confirm('Delete the Geoapify API key? This cannot be undone.')) return;
  $.ajax({
    url: '../rest/geo/delete', type: 'POST', dataType: 'JSON',
    success: function () { window.location.reload(); },
    error: function () { alert('Failed to delete geocoding configuration.'); }
  });
}

// pollRobinSync drives the Robin sync progress bar, then auto-refreshes the
// Sync tab so the rooms/desk-reservation results below reflect the latest data
// (no manual "view updated results" step).
function pollRobinSync() {
  var timer = setInterval(function () {
    $.ajax({
      url: '../rest/robin/progress', type: 'GET', dataType: 'JSON',
      success: function (snap) {
        renderSyncProgress('robin', snap);
        if (!snap.running && snap.done) {
          clearInterval(timer);
          loadAdminTab('ldap', 'robin', false);
        }
      },
      error: function () { clearInterval(timer); }
    });
  }, 800);
}

// showRobinResultTab toggles the "Meeting rooms" / "Desk reservations" panels in
// the Last sync card.
function showRobinResultTab(name) {
  var tabs = ['rooms', 'people', 'check'];
  tabs.forEach(function (t) {
    var panel = document.getElementById('robinRes_' + t);
    var nav = document.getElementById('robinResNav_' + t);
    if (panel) panel.style.display = (t === name) ? 'block' : 'none';
    if (nav) nav.classList.toggle('active', t === name);
  });
}

// updateRobinDeskModeDesc shows the description of the selected "Show Robin desk
// reservations" option to the right of the dropdown.
function updateRobinDeskModeDesc() {
  var descs = {
    off: 'Robin desk occupancy is neither synced nor shown on the map.',
    sync: 'Keeps the occupancy cache fresh without overlaying anything, so you can switch to a display mode later without waiting for a sync.',
    unused: 'Overlays only desks that have no assigned employee and no native booking.',
    all: 'Overlays every desk that is occupied in Robin right now, on top of the native assignments.',
    allclear: 'Like \u201CAll desks\u201D, but also hides a person\u2019s AD-assigned desk when Robin has them booked on a different desk, so they never appear twice on the same map.'
  };
  var sel = document.getElementById('robinDeskModeSelect');
  var el = document.getElementById('robinDeskModeDesc');
  if (sel && el) { el.textContent = descs[sel.value] || ''; }
}

function startLdapSync() {
  startSync('ldap', '../rest/ldap/sync', '../rest/ldap/progress', 'ldap');
}

// ── EntraID (Microsoft Graph) ────────────────────────────────
function startEntraSync() {
  startSync('entra', '../rest/entra/sync', '../rest/entra/progress', 'entra');
}

// testEntra validates one EntraID connection's credentials against Microsoft
// Graph without running a full sync and shows a structured summary in the shared
// test modal.
function testEntra(id) {
  var btn = document.getElementById('entratestbtn' + id);
  var desc = btn ? (btn.getAttribute('data-desc') || '') : '';
  var title = 'EntraID connection test' + (desc ? ' \u2014 ' + desc : '');
  var body = openTestModal(title, 'Testing ' + (desc || 'connection') + '\u2026');
  if (btn) btn.disabled = true;
  $.ajax({
    url: '../rest/entra/test?token=' + token + '&entraid=' + encodeURIComponent(id),
    type: 'GET', dataType: 'JSON',
    success: function (d) {
      if (body) body.innerHTML = renderTestChecks(d, 'Connection looks valid', 'Connection has problems');
    },
    error: function () {
      if (body) body.textContent = 'Request failed (forbidden or server error).';
    },
    complete: function () { if (btn) btn.disabled = false; }
  });
}

// syncEntra synchronously syncs a single EntraID connection (the per-connection
// "Sync now" button), mirroring syncLDAP.
function syncEntra(id) {
  var btn = document.getElementById('entrasyncbtn' + id);
  var out = document.getElementById('entraTestResult');
  var desc = btn ? (btn.getAttribute('data-desc') || '') : '';
  if (btn) { btn.disabled = true; btn.textContent = 'Syncing\u2026'; }
  if (out) { out.style.color = ''; out.textContent = 'Syncing\u2026'; }
  $.ajax({
    url: '../rest/entra/syncone?token=' + token + '&entraid=' + encodeURIComponent(id),
    type: 'GET', dataType: 'JSON',
    success: function (result) {
      if (btn) { btn.disabled = false; btn.textContent = 'Sync now'; }
      if (out) {
        out.style.color = 'var(--sy-ok)';
        out.textContent = 'Synced ' + (result && result.count != null ? result.count + ' placement(s).' : 'successfully.');
      }
    },
    error: function (xhr) {
      if (btn) { btn.disabled = false; btn.textContent = 'Sync now'; }
      if (out) {
        out.style.color = 'var(--sy-danger)';
        out.textContent = 'Sync failed: ' + ((xhr && xhr.responseText) || 'server error');
      }
    }
  });
}

// toggleEntraEnabled flips one EntraID connection on/off in place (no sub-tab
// reload). The scheduler and manual sync skip disabled connections.
function toggleEntraEnabled(id, cb) {
  if (cb) cb.disabled = true;
  var on = !!(cb && cb.checked);
  var body = 'toggleEntraID=' + encodeURIComponent(id) + '&entraEnabled=' + (on ? '1' : '0');
  fetch('?tab=ldap&partial=1', {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    credentials: 'same-origin',
    body: body
  }).then(function () {
    if (cb) cb.disabled = false;
    // Keep the per-connection "Sync now" button in step with the toggle: a
    // disabled connection cannot be synced.
    var syncBtn = document.getElementById('entrasyncbtn' + id);
    if (syncBtn) {
      syncBtn.disabled = !on;
      syncBtn.title = on ? '' : 'Enable this connection to sync it';
    }
  }).catch(function () {
    if (cb) { cb.disabled = false; cb.checked = !on; }
  });
}

// editEntraSource loads a connection's details into the add/edit form so it can
// be updated. Secrets are never sent back to the browser, so the credential
// fields stay blank ("leave blank to keep current").
function editEntraSource(btn) {
  var id = btn.getAttribute('data-id');
  var method = btn.getAttribute('data-method') || 'secret';
  document.getElementById('entraFormEditID').value = id;
  document.getElementById('entraFDescription').value = btn.getAttribute('data-desc') || '';
  document.getElementById('entraFTenant').value = btn.getAttribute('data-tenant') || '';
  document.getElementById('entraFClient').value = btn.getAttribute('data-client') || '';
  var sel = document.getElementById('entraFAuthMethod');
  if (sel) sel.value = (method === 'certificate') ? 'certificate' : 'secret';
  updateEntraAuthMethod();
  var secret = document.getElementById('entraFSecret');
  if (secret) { secret.value = ''; secret.placeholder = 'Leave blank to keep current secret'; }
  var cert = document.getElementById('entraFCert');
  if (cert) { cert.value = ''; cert.placeholder = 'Leave blank to keep current certificate'; }
  var key = document.getElementById('entraFKey');
  if (key) { key.value = ''; key.placeholder = 'Leave blank to keep current key'; }
  document.getElementById('entraFormSubmit').textContent = 'Save changes';
  var cancel = document.getElementById('entraFormCancel');
  if (cancel) cancel.style.display = 'inline-flex';
  var collapse = document.getElementById('entraAddCollapse');
  if (collapse && !collapse.classList.contains('open')) {
    toggleCollapse('entraAddCollapse', document.getElementById('entraAddToggle'));
  }
  var form = document.getElementById('entraForm');
  if (form) form.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
}

// cancelEditEntra resets the add/edit form back to "create" mode.
function cancelEditEntra() {
  var form = document.getElementById('entraForm');
  if (form) form.reset();
  document.getElementById('entraFormEditID').value = '';
  var secret = document.getElementById('entraFSecret');
  if (secret) secret.placeholder = 'Client secret value';
  var cert = document.getElementById('entraFCert');
  if (cert) cert.placeholder = '-----BEGIN CERTIFICATE-----';
  var key = document.getElementById('entraFKey');
  if (key) key.placeholder = '-----BEGIN PRIVATE KEY-----';
  document.getElementById('entraFormSubmit').textContent = 'Create connection';
  var cancel = document.getElementById('entraFormCancel');
  if (cancel) cancel.style.display = 'none';
  updateEntraAuthMethod();
  var collapse = document.getElementById('entraAddCollapse');
  if (collapse && collapse.classList.contains('open')) {
    toggleCollapse('entraAddCollapse', document.getElementById('entraAddToggle'));
  }
}

// updateEntraAuthMethod toggles the secret vs certificate credential fields
// based on the selected authentication method.
function updateEntraAuthMethod() {
  var sel = document.getElementById('entraFAuthMethod');
  var secret = document.getElementById('entraSecretFields');
  var cert = document.getElementById('entraCertFields');
  var isCert = sel && sel.value === 'certificate';
  if (secret) secret.style.display = isCert ? 'none' : '';
  if (cert) cert.style.display = isCert ? '' : 'none';
}

// showEntraCmpTab toggles the LDAP <-> EntraID comparison panels.
function showEntraCmpTab(name) {
  var tabs = ['diff', 'same', 'ldap', 'entra'];
  tabs.forEach(function (t) {
    var panel = document.getElementById('entraCmp_' + t);
    var nav = document.getElementById('entraCmpNav_' + t);
    if (panel) panel.style.display = (t === name) ? 'block' : 'none';
    if (nav) nav.classList.toggle('active', t === name);
  });
}

// ── Geoapify geocoding ───────────────────────────────────────
// Saves the API key, tests it against a single address, and runs a manual
// batch geocode of every location. There is no scheduler — syncing is always
// triggered explicitly here.
function saveGeoapify(ev) {
  if (ev && ev.preventDefault) ev.preventDefault();
  var key = (document.getElementById('geoapifyApiKey') || {}).value || '';
  var statusEl = document.getElementById('geoSaveStatus');
  if (statusEl) statusEl.textContent = 'Saving\u2026';
  var fd = new FormData();
  fd.append('tab', 'ldap');
  fd.append('saveGeoapify', '1');
  fd.append('geoapifyApiKey', key);
  $.ajax({
    url: '?tab=ldap&partial=1', type: 'POST', data: fd,
    processData: false, contentType: false,
    complete: function () {
      if (statusEl) statusEl.textContent = 'Saved.';
      loadAdminTab('ldap', 'geo', false);
    }
  });
  return false;
}

// testGeoapify runs the "Test" button: it geocodes a fixed sample address and
// shows a structured summary in the shared test modal to confirm the key works.
function testGeoapify() {
  var body = openTestModal('Geocoding test', 'Testing Geoapify\u2026');
  $.ajax({
    url: '../rest/geo/test?summary=1',
    type: 'GET', dataType: 'JSON',
    success: function (d) {
      if (body) body.innerHTML = renderTestChecks(d, 'Geocoding looks valid', 'Geocoding has problems');
      if (d) updateGeoUsage(d.usageMonth, d.usageCount);
    },
    error: function () {
      if (body) body.textContent = 'Request failed (forbidden or server error).';
    }
  });
}

// testGeoapifyAddress runs the "Test an address" card: it geocodes a
// user-supplied address and shows the resolved coordinates inline.
function testGeoapifyAddress() {
  var addr = ((document.getElementById('geoTestAddress') || {}).value || '').trim();
  var out = document.getElementById('geoTestResult');
  if (!addr) {
    if (out) { out.style.color = 'var(--sy-danger)'; out.textContent = 'Enter an address to test.'; }
    return;
  }
  if (out) { out.textContent = 'Testing\u2026'; out.style.color = ''; }
  $.ajax({
    url: '../rest/geo/test?address=' + encodeURIComponent(addr),
    type: 'GET', dataType: 'JSON',
    success: function (d) {
      if (!out) return;
      if (d && d.ok) {
        out.style.color = 'var(--sy-ok)';
        out.textContent = 'OK \u2014 ' + (d.formatted || d.address) +
          ' \u2192 lat ' + Number(d.lat).toFixed(5) + ', lon ' + Number(d.lon).toFixed(5);
        updateGeoUsage(d.usageMonth, d.usageCount);
      } else {
        out.style.color = 'var(--sy-danger)';
        out.textContent = 'Failed: ' + ((d && d.message) || 'unknown error');
      }
    },
    error: function () {
      if (out) { out.style.color = 'var(--sy-danger)'; out.textContent = 'Request failed (forbidden or server error).'; }
    }
  });
}

function updateGeoUsage(month, count) {
  if (count === undefined || count === null) return;
  var c = document.getElementById('geoUsageCount');
  var m = document.getElementById('geoUsageMonth');
  if (c) c.textContent = count;
  if (m && month) m.textContent = month;
}

function runGeoapifySync() {
  var btn = document.getElementById('geoSyncBtn');
  if (btn) { btn.disabled = true; btn.textContent = 'Syncing\u2026'; }
  // Hide any results from a previous run while this one builds.
  var summary = document.getElementById('geoSyncSummary');
  var table = document.getElementById('geoSyncTable');
  if (summary) summary.style.display = 'none';
  if (table) table.style.display = 'none';
  $.ajax({
    url: '../rest/geo/sync', type: 'POST', dataType: 'JSON',
    success: function (d) {
      if (!d || !d.ok) {
        alert((d && d.message) || 'Sync failed.');
        if (btn) { btn.disabled = false; btn.textContent = 'Sync all locations now'; }
        return;
      }
      pollGeoapifySync();
    },
    error: function () {
      alert('Sync request failed (forbidden or server error).');
      if (btn) { btn.disabled = false; btn.textContent = 'Sync all locations now'; }
    }
  });
}

function pollGeoapifySync() {
  var timer = setInterval(function () {
    $.ajax({
      url: '../rest/geo/progress', type: 'GET', dataType: 'JSON',
      success: function (snap) {
        renderSyncProgress('geoSync', snap);
        if (!snap.running && snap.done) {
          clearInterval(timer);
          var fill = document.getElementById('geoSyncProgFill');
          var stage = document.getElementById('geoSyncProgStage');
          if (fill) {
            fill.classList.remove('indeterminate');
            fill.style.width = '100%';
            if (snap.error) fill.style.background = 'var(--sy-danger)';
          }
          if (stage) stage.textContent = snap.error ? ('Error: ' + snap.error) : (snap.summary || 'Done');
          var btn = document.getElementById('geoSyncBtn');
          if (btn) { btn.disabled = false; btn.textContent = 'Sync all locations now'; }
          if (!snap.error && snap.result) renderGeoSyncResult(snap.result);
        }
      },
      error: function () {
        clearInterval(timer);
        var btn = document.getElementById('geoSyncBtn');
        if (btn) { btn.disabled = false; btn.textContent = 'Sync all locations now'; }
      }
    });
  }, 800);
}

function renderGeoSyncResult(res) {
  var summary = document.getElementById('geoSyncSummary');
  var table = document.getElementById('geoSyncTable');
  var body = document.getElementById('geoSyncBody');
  if (summary) summary.style.display = 'flex';
  document.getElementById('geoSyncUpdated').textContent = res.updated || 0;
  document.getElementById('geoSyncSkipped').textContent = res.skipped || 0;
  document.getElementById('geoSyncFailed').textContent = res.failed || 0;
  updateGeoUsage(res.usageMonth, res.usageCount);
  if (!body) return;
  body.innerHTML = '';
  var rows = res.results || [];
  for (var i = 0; i < rows.length; i++) {
    var r = rows[i];
    var tr = document.createElement('tr');
    var badge = r.status === 'ok' ? 'sync-badge-ok'
      : (r.status === 'skipped' ? 'sync-badge-warn' : 'sync-badge-danger');
    var latTxt = r.status === 'ok' ? Number(r.lat).toFixed(5) : '';
    var lonTxt = r.status === 'ok' ? Number(r.lon).toFixed(5) : '';
    var msg = r.status === 'ok' ? (r.formatted || '') : (r.message || '');
    tr.innerHTML = '<td></td><td><span class="sync-badge"></span></td><td></td><td></td><td></td>';
    tr.children[1].firstChild.className = 'sync-badge ' + badge;
    tr.children[0].textContent = r.mapname || '';
    tr.children[1].firstChild.textContent = r.status || '';
    tr.children[2].textContent = latTxt;
    tr.children[3].textContent = lonTxt;
    tr.children[4].textContent = msg;
    body.appendChild(tr);
  }
  if (table) table.style.display = '';
}

// ── Backup: export / import ──────────────────────────────────
// Export builds a zip server-side (with a progress bar) and auto-downloads it.
// Import uploads a zip and restores the selected data sets, overwriting them.
function startExport() {
  var btn = document.getElementById('exportSyncBtn');
  if (btn) { btn.disabled = true; btn.textContent = 'Building\u2026'; }
  $.ajax({
    url: '../rest/export/start', type: 'POST', dataType: 'JSON',
    complete: function () { pollExport(); }
  });
}

function pollExport() {
  var timer = setInterval(function () {
    $.ajax({
      url: '../rest/export/progress', type: 'GET', dataType: 'JSON',
      success: function (snap) {
        renderSyncProgress('export', snap);
        if (!snap.running && snap.done) {
          clearInterval(timer);
          var fill = document.getElementById('exportProgFill');
          var stage = document.getElementById('exportProgStage');
          var btn = document.getElementById('exportSyncBtn');
          if (btn) { btn.disabled = false; btn.textContent = 'Create export'; }
          if (snap.error) {
            if (fill) { fill.classList.remove('indeterminate'); fill.style.width = '100%'; fill.style.background = 'var(--sy-danger)'; }
            if (stage) stage.textContent = 'Error: ' + snap.error;
            return;
          }
          if (fill) { fill.classList.remove('indeterminate'); fill.style.width = '100%'; }
          if (stage) stage.textContent = 'Download starting\u2026';
          window.location = '../rest/export/download';
        }
      },
      error: function () { clearInterval(timer); }
    });
  }, 700);
}

function runImport() {
  var fileInput = document.getElementById('importFile');
  var out = document.getElementById('importResult');
  if (!fileInput || !fileInput.files || !fileInput.files.length) {
    if (out) { out.style.color = 'var(--sy-danger)'; out.textContent = 'Choose an export zip first.'; }
    return;
  }
  var groups = document.querySelectorAll('#importGroups .import-group:checked');
  if (!groups.length) {
    if (out) { out.style.color = 'var(--sy-danger)'; out.textContent = 'Select at least one data set.'; }
    return;
  }
  if (!confirm('Importing overwrites the selected data sets with the archive contents. Continue?')) return;
  var fd = new FormData();
  fd.append('archive', fileInput.files[0]);
  for (var i = 0; i < groups.length; i++) { fd.append('group_' + groups[i].value, '1'); }
  var btn = document.getElementById('importBtn');
  if (btn) { btn.disabled = true; btn.textContent = 'Importing\u2026'; }
  if (out) { out.style.color = ''; out.textContent = 'Importing\u2026'; }
  $.ajax({
    url: '../rest/import', type: 'POST', data: fd,
    processData: false, contentType: false, dataType: 'JSON',
    success: function (d) {
      if (!d || !d.ok) {
        if (out) { out.style.color = 'var(--sy-danger)'; out.textContent = (d && d.message) || 'Import failed.'; }
        return;
      }
      var parts = [];
      (d.results || []).forEach(function (r) {
        var detail = r.files ? (r.files + ' file(s)') : (r.records + ' record(s)');
        parts.push(r.label + ': ' + (r.status === 'ok' ? detail : ('failed \u2014 ' + (r.message || ''))));
      });
      if (out) { out.style.color = 'var(--sy-ok)'; out.innerHTML = esc('Import complete.') + '<br>' + parts.map(esc).join('<br>'); }
    },
    error: function () {
      if (out) { out.style.color = 'var(--sy-danger)'; out.textContent = 'Import request failed (forbidden or server error).'; }
    },
    complete: function () {
      if (btn) { btn.disabled = false; btn.textContent = 'Import selected'; }
    }
  });
}

// ── Robin desk-data diagnostic (read-only) ───────────────────
function runRobinDeskTest() {
  var btn = document.getElementById('robinDeskTestBtn');
  if (btn) { btn.disabled = true; btn.textContent = 'Running\u2026'; }
  var logEl = document.getElementById('robinDeskLog');
  if (logEl) { logEl.textContent = ''; logEl.style.display = 'block'; }
  $.ajax({
    url: '../rest/robin/desktest', type: 'POST', dataType: 'JSON',
    complete: function () { pollRobinDeskTest(); }
  });
}

function pollRobinDeskTest() {
  var timer = setInterval(function () {
    $.ajax({
      url: '../rest/robin/desktest/progress', type: 'GET', dataType: 'JSON',
      success: function (snap) {
        renderSyncProgress('robinDesk', snap);
        if (!snap.running && snap.done) {
          clearInterval(timer);
          var fill = document.getElementById('robinDeskProgFill');
          if (fill) {
            fill.classList.remove('indeterminate');
            fill.style.width = '100%';
            if (snap.error) fill.style.background = 'var(--sy-danger)';
          }
          var stage = document.getElementById('robinDeskProgStage');
          if (stage) stage.textContent = snap.error ? ('Error: ' + snap.error) : (snap.summary || 'Done');
          var btn = document.getElementById('robinDeskTestBtn');
          if (btn) { btn.disabled = false; btn.textContent = 'Run desk diagnostic'; }
        }
      },
      error: function () { clearInterval(timer); }
    });
  }, 800);
}

function downloadRobinDeskDump() {
  // Stream the zip via a hidden navigation so the browser triggers a download.
  window.location.href = '../rest/robin/deskdump';
}

// ── Robin strip-pattern suggestions ─────────────────────────
function scanRobinStrip() {
  var btn = document.getElementById('robinSuggestBtn');
  var box = document.getElementById('robinSuggestResult');
  if (btn) { btn.disabled = true; btn.textContent = 'Scanning\u2026'; }
  if (box) box.innerHTML = '';
  $.ajax({
    url: '../rest/robin/suggestions', type: 'POST', dataType: 'JSON',
    complete: function () { pollRobinStrip(); }
  });
}

function pollRobinStrip() {
  var timer = setInterval(function () {
    $.ajax({
      url: '../rest/robin/suggestions/progress', type: 'GET', dataType: 'JSON',
      success: function (snap) {
        renderSyncProgress('robinSuggest', snap);
        if (!snap.running && snap.done) {
          clearInterval(timer);
          var fill = document.getElementById('robinSuggestProgFill');
          if (fill) {
            fill.classList.remove('indeterminate');
            fill.style.width = '100%';
            if (snap.error) fill.style.background = 'var(--sy-danger)';
          }
          var stage = document.getElementById('robinSuggestProgStage');
          if (stage) stage.textContent = snap.error ? ('Error: ' + snap.error) : (snap.summary || 'Done');
          var btn = document.getElementById('robinSuggestBtn');
          if (btn) { btn.disabled = false; btn.textContent = 'Scan for suggestions'; }
          if (!snap.error) renderRobinStripSuggestions(snap.suggestions || []);
        }
      },
      error: function () { clearInterval(timer); }
    });
  }, 800);
}

function renderRobinStripSuggestions(list) {
  var box = document.getElementById('robinSuggestResult');
  if (!box) return;
  list = list || [];
  if (!list.length) {
    box.innerHTML = '<div class="sync-empty">No partial matches found. Every Robin seat already matches a desk (or no extra prefix/suffix was detected).</div>';
    return;
  }
  var html = '';
  for (var i = 0; i < list.length; i++) {
    var s = list[i];
    var label = (s.type === 'prefix') ? 'strip prefix' : 'strip suffix';
    var cnt = (s.count > 1) ? ' <span class="sync-badge sync-badge-off">' + s.count + '\u00d7</span>' : '';
    html += '<div class="robin-suggest-row">'
          + '<span class="robin-suggest-text">Partial match: <b>"' + esc(s.sample) + '"</b>, '
          + label + ' <code>"' + esc(s.pattern) + '"</code>' + cnt + '</span>'
          + '<button type="button" class="sync-btn sync-btn-sm robin-suggest-add" data-type="' + escAttr(s.type) + '" data-pattern="' + escAttr(s.pattern) + '">Add</button>'
          + '</div>';
  }
  box.innerHTML = html;
  var btns = box.querySelectorAll('.robin-suggest-add');
  for (var j = 0; j < btns.length; j++) {
    btns[j].addEventListener('click', function () {
      addRobinStrip(this, this.getAttribute('data-type'), this.getAttribute('data-pattern'));
    });
  }
}

function addRobinStrip(btn, type, pattern) {
  btn.disabled = true; btn.textContent = 'Adding\u2026';
  $.ajax({
    url: '../rest/robin/strip/add', type: 'POST', dataType: 'JSON',
    data: { type: type, pattern: pattern },
    success: function (res) {
      if (res && res.ok) {
        var row = btn.closest('.robin-suggest-row');
        if (row) { row.classList.add('robin-suggest-done'); }
        btn.textContent = res.already ? 'Already set' : 'Added';
        applyStripToForm(type, pattern);
      } else {
        btn.disabled = false; btn.textContent = 'Add';
        alert((res && res.error) ? res.error : 'Could not add pattern.');
      }
    },
    error: function () { btn.disabled = false; btn.textContent = 'Add'; alert('Could not add pattern.'); }
  });
}

// applyStripToForm mirrors a saved strip pattern into the "Robin options" form so
// the prefix/suffix list reflects the change without a page reload.
function applyStripToForm(type, pattern) {
  var listName = (type === 'prefix') ? 'robinStripPrefixList' : 'robinStripSuffixList';
  var enName = (type === 'prefix') ? 'robinStripPrefixEnabled' : 'robinStripSuffixEnabled';
  var ta = document.querySelector('textarea[name="' + listName + '"]');
  if (ta) {
    var lines = ta.value.split('\n').filter(function (l) { return l.trim() !== ''; });
    var exists = false;
    for (var i = 0; i < lines.length; i++) { if (lines[i] === pattern) { exists = true; break; } }
    if (!exists) {
      lines.push(pattern);
      ta.value = lines.join('\n');
    }
  }
  var cb = document.querySelector('input[name="' + enName + '"]');
  if (cb) cb.checked = true;
}

// saveRobinOptions persists the Robin options form in place (no tab re-render),
// since none of its fields drive a server-computed display. Returns false to
// cancel the default submit and stop the generic #content form handler.
function saveRobinOptions(form) {
  var note = document.getElementById('robinOptionsSaved');
  var btn = form.querySelector('button[type="submit"]');
  if (btn) { btn.disabled = true; }
  if (note) { note.textContent = ''; note.classList.remove('error'); }
  $.ajax({
    url: '?tab=ldap&sub=robin&partial=1', type: 'post',
    data: new FormData(form), processData: false, contentType: false,
    success: function () {
      if (note) { note.textContent = 'Saved \u2713'; }
    },
    error: function () {
      if (note) { note.textContent = 'Could not save'; note.classList.add('error'); }
    },
    complete: function () { if (btn) { btn.disabled = false; } }
  });
  return false;
}

// Escape a string for safe use inside a double-quoted HTML attribute.
function escAttr(s) {
  return esc(s).replace(/"/g, '&quot;');
}

// ── Admin add-user directory autocomplete ───────────────────
var _dirSearchTimer = null;
var _dirResults = [];

function searchDirectory(q) {
  // Typing a new name invalidates any previous pick.
  var picked = document.getElementById('newadminuser');
  if (picked) picked.value = '';
  var pickedName = document.getElementById('newadminname');
  if (pickedName) pickedName.value = '';
  var info = document.getElementById('adminUserPicked');
  if (info) info.textContent = '';

  q = (q || '').trim();
  if (q.length < 2) { hideDirectoryResults(); return; }

  if (_dirSearchTimer) clearTimeout(_dirSearchTimer);
  _dirSearchTimer = setTimeout(function() {
    $.ajax({
      url: '../rest/directory/search?q=' + encodeURIComponent(q),
      type: 'GET', dataType: 'JSON',
      success: function(list) { renderDirectoryResults(list || []); },
      error: function() { hideDirectoryResults(); }
    });
  }, 250);
}

function renderDirectoryResults(list) {
  var box = document.getElementById('adminUserResults');
  if (!box) return;
  _dirResults = list;
  if (!list.length) {
    box.innerHTML = '<div class="dir-empty">No matching directory users.</div>';
    box.style.display = 'block';
    return;
  }
  var html = '';
  for (var i = 0; i < list.length; i++) {
    var d = list[i];
    var sub = [d.sam];
    if (d.office) sub.push(d.office);
    if (d.mail) sub.push(d.mail);
    html += '<div class="dir-item" onclick="pickDirectoryUser(' + i + ')">' +
            '<span class="dir-name">' + esc(d.name) + '</span>' +
            '<span class="dir-sub">' + esc(sub.join(' · ')) + '</span></div>';
  }
  box.innerHTML = html;
  box.style.display = 'block';
}

function pickDirectoryUser(i) {
  var d = _dirResults[i];
  if (!d) return;
  document.getElementById('newadminuser').value = d.username || d.sam;
  document.getElementById('newadminname').value = d.name || '';
  var search = document.getElementById('adminUserSearch');
  if (search) search.value = d.name || d.sam;
  var info = document.getElementById('adminUserPicked');
  if (info) info.innerHTML = 'Selected <b>' + esc(d.name) + '</b> (<code>' + esc(d.username || d.sam) + '</code>)';
  hideDirectoryResults();
}

function hideDirectoryResults() {
  var box = document.getElementById('adminUserResults');
  if (box) box.style.display = 'none';
}

// Save a single base variable without reloading the page.
function saveSetting(name, btn) {
  var row = btn.closest('tr');
  var input = row ? row.querySelector('input[type=text], textarea') : null;
  if (!input) return;
  var orig = btn.textContent;
  btn.disabled = true;
  btn.textContent = 'Saving\u2026';
  var body = 'name=' + encodeURIComponent(name) + '&value=' + encodeURIComponent(input.value);
  fetch('../rest/setting', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: body
  }).then(function (r) {
    if (!r.ok) throw new Error('save failed');
    return r.json();
  }).then(function () {
    btn.disabled = false;
    btn.textContent = 'Saved';
    setTimeout(function () { btn.textContent = orig; }, 1200);
  }).catch(function () {
    btn.disabled = false;
    btn.textContent = 'Failed';
    setTimeout(function () { btn.textContent = orig; }, 1500);
  });
}

// Toggle the dynamic world map setting (stored as "1"/"" under "worldmap").
// Enabling first checks that every published location has lat/lon; if some are
// missing it opens the coordinate review dialog instead of enabling immediately.
function saveWorldMap(cb) {
  if (cb.checked) {
    cb.disabled = true;
    fetch('../rest/config?mode=maps', { credentials: 'same-origin' })
      .then(function (r) { return r.json(); })
      .then(function (d) {
        var maps = (d && d.maps) || [];
        var missing = maps.filter(function (m) {
          return String(m.mapname).toLowerCase() !== 'overview'
            && String(m.published).toLowerCase() === 'yes'
            && Number(m.lat) === 0 && Number(m.lon) === 0;
        });
        cb.disabled = false;
        if (missing.length === 0) { persistWorldMap(cb, '1'); return; }
        cb.checked = false; // keep disabled until coordinates are saved
        openWorldCoords(cb, missing);
      })
      .catch(function () { cb.disabled = false; persistWorldMap(cb, '1'); });
    return;
  }
  persistWorldMap(cb, '');
}

// persistWorldMap writes the worldmap setting and updates the inline status text.
function persistWorldMap(cb, value) {
  var status = document.getElementById('worldmapStatus');
  cb.checked = value === '1';
  cb.disabled = true;
  if (status) { status.style.color = ''; status.textContent = 'Saving\u2026'; }
  var body = 'name=worldmap&value=' + encodeURIComponent(value);
  return fetch('../rest/setting', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: body
  }).then(function (r) {
    if (!r.ok) throw new Error('save failed');
    return r.json();
  }).then(function () {
    cb.disabled = false;
    if (status) {
      status.style.color = 'var(--sy-ok)';
      status.textContent = cb.checked ? 'Enabled' : 'Disabled';
      setTimeout(function () { status.textContent = ''; }, 1500);
    }
  }).catch(function () {
    cb.disabled = false;
    cb.checked = !cb.checked;
    if (status) { status.style.color = 'var(--sy-danger)'; status.textContent = 'Failed'; }
    throw new Error('save failed');
  });
}

// ── World map coordinate review dialog (classic -> modern switch) ─────────────
// Holds the pending enable while the dialog is open.
var _worldCoordsPending = null; // { cb, rows:[{mapname,address}], imgW, imgH }

// cleanWorldAddr turns a stored address (with <br>) into a single geocodable line.
function cleanWorldAddr(a) {
  return String(a == null ? '' : a)
    .replace(/<br\s*\/?>/gi, ', ')
    .replace(/\s+/g, ' ')
    .trim();
}

// approxWorldLatLon converts a stored X/Y screen position into lat/lon using the
// exact same projection the dynamic world map renders with (worldProjection in
// user.js, at the internal 1600px screen width). In other words: imagine the
// dynamic world map drawn at targetScreenWidth and read off the lat/lon under
// the pixel where the classic marker sits. It is an approximation (the old
// overview.png is a slightly different projection) and meant to be reviewed and
// edited before saving.
function approxWorldLatLon(x, y) {
  if (typeof worldProjection !== 'function') return null;
  var ll = worldProjection().toLatLon(Number(x), Number(y));
  if (!isFinite(ll.lat) || !isFinite(ll.lon)) return null;
  if (ll.lon < -180) ll.lon = -180; if (ll.lon > 180) ll.lon = 180;
  if (ll.lat < -90) ll.lat = -90; if (ll.lat > 90) ll.lat = 90;
  return { lat: ll.lat, lon: ll.lon };
}

function openWorldCoords(cb, missing) {
  _worldCoordsPending = { cb: cb, rows: missing };
  var hint = document.getElementById('worldcoordsGeoHint');
  var geoActions = document.getElementById('worldcoordsGeoActions');
  var configured = (typeof ADMIN_GEOAPIFY_CONFIGURED !== 'undefined') && ADMIN_GEOAPIFY_CONFIGURED;
  if (hint) hint.style.display = configured ? 'none' : 'block';
  if (geoActions) geoActions.style.display = configured ? 'flex' : 'none';
  var result = document.getElementById('worldcoordsResult');
  if (result) { result.textContent = ''; result.style.color = ''; }
  document.getElementById('worldcoordsOverlay').style.display = 'block';
  renderWorldCoords();
}

function renderWorldCoords() {
  var pend = _worldCoordsPending;
  if (!pend) return;
  var configured = (typeof ADMIN_GEOAPIFY_CONFIGURED !== 'undefined') && ADMIN_GEOAPIFY_CONFIGURED;
  var body = document.getElementById('worldcoordsBody');
  if (!body) return;
  var html = '';
  pend.rows.forEach(function (m, i) {
    var addr = cleanWorldAddr(m.address);
    var lat = '', lon = '', source = '\u2014';
    var approx = approxWorldLatLon(m.x, m.y);
    if (approx) { lat = approx.lat.toFixed(4); lon = approx.lon.toFixed(4); source = '~from X/Y'; }
    var label = m.displayname || m.mapname;
    html += '<tr data-mapname="' + esc(m.mapname) + '">'
      + '<td style="white-space:nowrap;">' + esc(label) + '</td>'
      + '<td>' + (addr ? esc(addr) : '<span style="color:var(--sy-muted);">no address</span>') + '</td>'
      + '<td><input class="sync-input wc-lat" type="text" value="' + esc(lat) + '" style="width:90px;"></td>'
      + '<td><input class="sync-input wc-lon" type="text" value="' + esc(lon) + '" style="width:90px;"></td>'
      + '<td class="wc-source" style="white-space:nowrap;color:var(--sy-muted);">' + esc(source) + '</td>'
      + '<td>' + (configured && addr ? '<button type="button" class="sync-btn sync-btn-sm" onclick="worldCoordsGeocodeRow(' + i + ')">Geocode</button>' : '') + '</td>'
      + '</tr>';
  });
  body.innerHTML = html;
}

function worldCoordsGeocodeRow(i) {
  var pend = _worldCoordsPending;
  if (!pend) return Promise.resolve();
  var m = pend.rows[i];
  var addr = cleanWorldAddr(m.address);
  if (!addr) return Promise.resolve();
  var tr = document.querySelector('#worldcoordsBody tr[data-mapname="' + (window.CSS && CSS.escape ? CSS.escape(m.mapname) : m.mapname) + '"]');
  if (!tr) return Promise.resolve();
  var srcCell = tr.querySelector('.wc-source');
  if (srcCell) srcCell.textContent = 'geocoding\u2026';
  return fetch('../rest/geo/test?address=' + encodeURIComponent(addr), { credentials: 'same-origin' })
    .then(function (r) { return r.json(); })
    .then(function (d) {
      if (d && d.ok) {
        tr.querySelector('.wc-lat').value = Number(d.lat).toFixed(4);
        tr.querySelector('.wc-lon').value = Number(d.lon).toFixed(4);
        if (srcCell) { srcCell.textContent = 'geocoded'; srcCell.style.color = 'var(--sy-ok)'; }
      } else if (srcCell) {
        srcCell.textContent = 'failed'; srcCell.style.color = 'var(--sy-danger)';
      }
    })
    .catch(function () { if (srcCell) { srcCell.textContent = 'failed'; srcCell.style.color = 'var(--sy-danger)'; } });
}

function worldCoordsGeocodeAll() {
  var pend = _worldCoordsPending;
  if (!pend) return;
  var btn = document.getElementById('worldcoordsGeocodeAll');
  if (btn) btn.disabled = true;
  // Geocode rows one at a time to keep API usage predictable.
  var chain = Promise.resolve();
  pend.rows.forEach(function (m, i) {
    if (!cleanWorldAddr(m.address)) return;
    chain = chain.then(function () { return worldCoordsGeocodeRow(i); });
  });
  chain.then(function () { if (btn) btn.disabled = false; });
}

function cancelWorldCoords() {
  document.getElementById('worldcoordsOverlay').style.display = 'none';
  if (_worldCoordsPending && _worldCoordsPending.cb) {
    _worldCoordsPending.cb.checked = false;
    _worldCoordsPending.cb.disabled = false;
  }
  _worldCoordsPending = null;
}

function saveWorldCoords() {
  var pend = _worldCoordsPending;
  if (!pend) return;
  var btn = document.getElementById('worldcoordsSaveBtn');
  var result = document.getElementById('worldcoordsResult');
  var rows = document.querySelectorAll('#worldcoordsBody tr');
  var posts = [];
  rows.forEach(function (tr) {
    var mapname = tr.getAttribute('data-mapname');
    var lat = tr.querySelector('.wc-lat').value.trim();
    var lon = tr.querySelector('.wc-lon').value.trim();
    if (lat === '' || lon === '') return; // skip rows the user left blank
    var b = 'mapname=' + encodeURIComponent(mapname)
      + '&lat=' + encodeURIComponent(lat)
      + '&lon=' + encodeURIComponent(lon);
    posts.push(fetch('../rest/maps/coords', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: b
    }).then(function (r) { if (!r.ok) throw new Error('save failed'); return r.json(); }));
  });
  if (btn) btn.disabled = true;
  if (result) { result.style.color = ''; result.textContent = 'Saving\u2026'; }
  Promise.all(posts).then(function () {
    var cb = pend.cb;
    document.getElementById('worldcoordsOverlay').style.display = 'none';
    _worldCoordsPending = null;
    return persistWorldMap(cb, '1');
  }).then(function () {
    if (btn) btn.disabled = false;
  }).catch(function () {
    if (btn) btn.disabled = false;
    if (result) { result.style.color = 'var(--sy-danger)'; result.textContent = 'Failed to save coordinates.'; }
  });
}

// ── VIP desk border categories (chips) ───────────────────────
var _vipEditable = false;
function loadVips(editable) {
  _vipEditable = !!editable;
  fetch('../rest/vips', { credentials: 'same-origin' })
    .then(function (r) { return r.json(); })
    .then(renderVips)
    .catch(function () {
      var c = document.getElementById('vipCategories');
      if (c) c.textContent = 'Could not load VIP categories.';
    });
}

function renderVips(cats) {
  var c = document.getElementById('vipCategories');
  if (!c) return;
  c.innerHTML = '';
  (cats || []).forEach(function (cat) {
    var card = document.createElement('div');
    card.className = 'vip-card';
    card.style.borderLeftColor = cat.color;

    var head = document.createElement('div');
    head.className = 'vip-card-head';
    var dot = document.createElement('span');
    dot.className = 'vip-dot';
    dot.style.background = cat.color;
    head.appendChild(dot);
    var title = document.createElement('span');
    title.className = 'vip-card-title';
    title.textContent = cat.type;
    head.appendChild(title);
    card.appendChild(head);

    var chips = document.createElement('div');
    chips.className = 'vip-chips';
    (cat.tags || []).forEach(function (tag) {
      var chip = document.createElement('span');
      chip.className = 'vip-chip';
      chip.style.background = cat.color;
      var label = document.createElement('span');
      label.textContent = tag;
      chip.appendChild(label);
      if (_vipEditable) {
        var x = document.createElement('button');
        x.type = 'button';
        x.className = 'vip-chip-x';
        x.innerHTML = '&times;';
        x.title = 'Remove';
        x.onclick = (function (t, tg) { return function () { removeVipTag(t, tg); }; })(cat.type, tag);
        chip.appendChild(x);
      }
      chips.appendChild(chip);
    });
    if (!(cat.tags && cat.tags.length)) {
      var empty = document.createElement('span');
      empty.className = 'vip-empty';
      empty.textContent = 'No tags yet.';
      chips.appendChild(empty);
    }
    card.appendChild(chips);

    if (_vipEditable) {
      var addRow = document.createElement('div');
      addRow.className = 'vip-add';
      var input = document.createElement('input');
      input.type = 'text';
      input.className = 'vip-add-input';
      input.placeholder = 'Add tag\u2026';
      input.onkeydown = (function (t, inp) {
        return function (e) { if (e.key === 'Enter') { e.preventDefault(); addVipTag(t, inp); } };
      })(cat.type, input);
      var btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'sync-btn sync-btn-sm';
      btn.textContent = 'Add';
      btn.onclick = (function (t, inp) { return function () { addVipTag(t, inp); }; })(cat.type, input);
      addRow.appendChild(input);
      addRow.appendChild(btn);
      card.appendChild(addRow);
    }

    c.appendChild(card);
  });
}

function postVip(action, type, tag) {
  var body = 'action=' + encodeURIComponent(action) + '&type=' + encodeURIComponent(type) + '&tag=' + encodeURIComponent(tag);
  return fetch('../rest/vips', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: body
  }).then(function (r) { return r.json(); }).then(renderVips);
}

function addVipTag(type, input) {
  var tag = (input.value || '').trim();
  if (!tag) return;
  postVip('add', type, tag);
}

function removeVipTag(type, tag) {
  postVip('remove', type, tag);
}

// Re-match existing admins to full names from the cached AD directory. Useful
// for accounts created before the directory cache existed.
function matchAdminNames() {
  var btn = document.getElementById('matchNamesBtn');
  var status = document.getElementById('matchNamesStatus');
  if (btn) { btn.disabled = true; btn.textContent = 'Matching\u2026'; }
  if (status) { status.style.display = 'block'; status.textContent = 'Checking directory\u2026'; }
  fetch('../rest/directory/match', { method: 'POST', credentials: 'same-origin' })
    .then(function (r) { return r.json(); })
    .then(function (res) {
      if (status) status.textContent = res.message || 'Done.';
      if (btn) { btn.disabled = false; btn.textContent = 'Match names from directory'; }
      if (res.updated > 0) setTimeout(function () { loadAdminTab('users', null, false); }, 1200);
    })
    .catch(function () {
      if (status) status.textContent = 'Matching failed.';
      if (btn) { btn.disabled = false; btn.textContent = 'Match names from directory'; }
    });
}

// Holds the live Chart.js instances keyed by canvas id so they can be destroyed
// and rebuilt on tab re-entry instead of stacking up on the same canvas.
var statsCharts = {};

// Cumulative CSS zoom of an element and all of its ancestors.
function cumulativeZoom(el) {
  var z = 1;
  for (var e = el; e; e = e.parentElement) {
    var cz = parseFloat(getComputedStyle(e).zoom) || 1;
    z *= cz;
  }
  return z;
}

// The admin body (#content) is shown with CSS `zoom` for autozoom. A non-unity
// zoom on a Chart.js ancestor breaks tooltip/point hit-testing: the browser
// delivers mouse offsets in painted (zoomed) pixels while Chart.js maps them
// using the un-zoomed layout width, so the tooltip lands on the wrong point.
//
// To fix this we measure the real on-screen (painted) size the chart should
// occupy, then neutralise the ancestor zoom on the chart container (zoom = 1/Z)
// and size it explicitly in those painted pixels. The canvas then has a net
// zoom of 1, so layout pixels == painted pixels and hit-testing is exact.
function fitStatsChartContainer(canvas) {
  var container = canvas.parentElement; // .statschart
  // Reset any sizing from a previous run so we measure the natural CSS size.
  container.style.zoom = '';
  container.style.width = '';
  container.style.height = '';
  container.style.maxWidth = '';
  var ancestorZoom = cumulativeZoom(container.parentElement);
  // Real painted dimensions = natural layout size * the ancestor zoom factor.
  var paintedWidth = Math.round(container.clientWidth * ancestorZoom);
  var paintedHeight = Math.round(container.clientHeight * ancestorZoom);
  // Cancel the ancestor zoom and pin the box to its painted pixel size.
  container.style.zoom = String(1 / ancestorZoom);
  container.style.maxWidth = 'none';
  container.style.width = paintedWidth + 'px';
  container.style.height = paintedHeight + 'px';
}

function showCharts(interval, divname) {

  // The stats template provides a <div class="statschart"><canvas></div> for
  // each chart. As a fallback (e.g. older callers) create the same structure.
  // The canvas must NOT carry fixed width/height attributes: Chart.js sizes it
  // to the container (with maintainAspectRatio:false), which keeps point/tooltip
  // hit-areas aligned. Fixed attributes caused the canvas to be drawn small then
  // CSS-stretched, shifting every tooltip away from its data point.
  if (document.getElementById(divname) == null) {
    var content = document.getElementById('content');
    var container = document.createElement('div');
    container.className = 'statschart';
    var canvas = document.createElement('canvas');
    canvas.id = divname;
    container.appendChild(canvas);
    content.appendChild(container);
  }

  $.ajax({

    // fetch data from stats API (newest-first; reverse to chronological order)
    url: '../rest/stats/?interval=' + interval,
    async: true,
    type: 'get',
    dataType: 'JSON',
    success: function (result) {

      var outlabels = result.reverse().map(function (item) { return item.period; });
      var outcount = result.map(function (item) { return item.count; });

      var chartData = {
        labels: outlabels,
        datasets: [{
          borderColor: 'rgba(90,190,90,1.0)',
          backgroundColor: 'rgba(90,190,90,0.5)',
          fill: true,
          tension: 0.4,
          pointRadius: 5,
          pointHitRadius: 10,
          data: outcount
        }]
      };

      var chartOptions = {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        interaction: { mode: 'index', intersect: false },
        scales: {
          x: {
            ticks: { color: 'rgba(255,255,255,1.0)' },
            grid: { color: 'rgba(255,255,255,0.5)' }
          },
          y: {
            beginAtZero: true,
            ticks: { color: 'rgba(255,255,255,1.0)', precision: 0 },
            grid: { color: 'rgba(255,255,255,0.5)' }
          }
        },
        plugins: {
          legend: { display: false }
        }
      };

      // Replace any previous chart on this canvas before drawing a new one.
      if (statsCharts[divname]) { statsCharts[divname].destroy(); }
      var ctx = document.getElementById(divname);
      // Size the container to real painted pixels (cancelling autozoom) so the
      // canvas net zoom is 1 and tooltip hit-testing maps to the right point.
      fitStatsChartContainer(ctx);
      statsCharts[divname] = new Chart(ctx, { type: 'line', data: chartData, options: chartOptions });
    },
    error: function () {
      console.log('Stats: Could not get data for ' + divname + ' from database.');
    }
  });

}

function ucWords (word) {
  word = word.toLowerCase().replace(/^[\u00C0-\u1FFF\u2C00-\uD7FF\w]|\s[\u00C0-\u1FFF\u2C00-\uD7FF\w]/g, function(letter) {
      return letter.toUpperCase();
  });
  return word;
}

function deskSummary(map) {

  $.ajax({
    url: '../rest/desks?map=' + map,
    async: true, 
    type: 'get',
    dataType: 'JSON',
    success: function(result){
      var output  = '<table border="0" style="width:470px; margin-left:30px;">'
                  + '<tr>'
                  + '<td style="font-weight: bold;color:lightgrey;text-align:left">'+ucWords(map)+'</td>'
                  + '<td style="width:130px"></td>'
                  + '<td style="width:130px"></td><td style="width:130px"></td>'
                  + '</tr>'
                  + '<tr>'
                  + '<td style="font-weight: bold;color:grey;text-align:left">Department</td>'
                  + '<td style="font-weight: bold;color:lightblue;width:130px;text-align:center;">Total</td>'
                  + '<td style="font-weight: bold;color:orange;width:130px;text-align:center;">In use</td>'
                  + '<td style="font-weight: bold;color:green;width:130px;text-align:center;">Free</td>'
                  + '</tr>'
                  +  '<tr><td>&nbsp;</td></tr>';
      // Output departments one by one
      $.each( departments, function( x, department ){
          var all = result.desks.filter(element => element.dept == department);
          var total1 = all.filter(element => element.desktype == 'addesk');
          var total2 = all.filter(element => element.desktype == 'blocked');
          var total3 = all.filter(element => element.desktype == 'hotseat');
          var totalcount = Object.keys(total1).length + Object.keys(total2).length + Object.keys(total3).length;
          var used1 = total1.filter(element => element.fname != '');
          var usedcount = Object.keys(used1).length + Object.keys(total2).length + Object.keys(total3).length;
          var freecount = totalcount - usedcount;
          output+='<tr>'
              + '<td style="color:grey;text-align:left">'+department+'</td>'
              + '<td style="color:lightblue;text-align:center;">'+totalcount+'</td>'
              + '<td style="color:orange;text-align:center;">'+usedcount+'</td>'
              + '<td style="color:green;text-align:center;">'+freecount+'</td>'
              + '</tr>';
      });
      var all = result.desks;
      var total1 = all.filter(element => element.desktype == 'addesk');
      var total2 = all.filter(element => element.desktype == 'blocked');
      var total3 = all.filter(element => element.desktype == 'hotseat');
      var total4 = all.filter(element => element.desktype == 'shareddesk');
      var totalcount = Object.keys(total1).length + Object.keys(total2).length + Object.keys(total3).length + Object.keys(total4).length;
      var used1 = all.filter(element => element.fname != '');
      var usedcount = Object.keys(used1).length + Object.keys(total2).length + Object.keys(total3).length + Object.keys(total4).length;
      var freecount = totalcount - usedcount;
      
      output+='<tr>'
              + '<td style="color:grey;text-align:left; font-weight:bold;">Summary</td>'
              + '<td style="color:lightblue; font-weight:bold;text-align:center;">'+totalcount+'</td>'
              + '<td style="color:orange; font-weight:bold;text-align:center;">'+usedcount+'</td>'
              + '<td style="color:green; font-weight:bold;text-align:center;">'+freecount+'</td>'
              + '</tr></table>';

      var statsoutput = document.getElementById(map);
      statsoutput.innerHTML = output;
      statsoutput.style.visibility = 'visible';
      console.log('[Desks] '+ map + ' updated');
    }    
  });

}

// ---------------------------------------------------------------------------
// Audit log: server-paginated, lazily scrolled. The production audit log can
// hold 100k+ rows, so entries are never loaded all at once. /rest/auditlog
// returns a page (offset+limit) filtered server-side by type and free-text;
// scrolling the sentinel into view fetches the next page and appends it.
// ---------------------------------------------------------------------------
var AUDIT_PAGE = 100;
var _auditOffset = 0;
var _auditHasMore = true;
var _auditLoading = false;
var _auditObserver = null;
var _auditDebounce = null;
var _auditGen = 0; // bumped on every filter change; stale responses are ignored

function auditFilterValues() {
  return {
    type: (document.getElementById('auditFilterType') || {}).value || '',
    time: (document.getElementById('auditFilterTime') || {}).value || '',
    user: (document.getElementById('auditFilterUser') || {}).value || '',
    info: (document.getElementById('auditFilterInfo') || {}).value || ''
  };
}

// Called on every keystroke / dropdown change. Debounced so typing doesn't fire
// a request per character; resets the pager and reloads from the top. Bumping
// the generation makes any in-flight request's response be discarded.
function auditFilterChanged() {
  clearTimeout(_auditDebounce);
  _auditDebounce = setTimeout(function () {
    _auditGen++;
    _auditOffset = 0;
    _auditHasMore = true;
    _auditLoading = false;
    var body = document.getElementById('auditBody');
    if (body) { body.innerHTML = ''; }
    loadAuditPage();
  }, 300);
}

function loadAuditPage() {
  if (_auditLoading || !_auditHasMore) { return; }
  _auditLoading = true;
  var gen = _auditGen;
  var f = auditFilterValues();
  $.ajax({
    url: '../rest/auditlog/',
    async: true,
    type: 'get',
    dataType: 'JSON',
    data: { offset: _auditOffset, limit: AUDIT_PAGE, type: f.type, time: f.time, user: f.user, info: f.info },
    success: function (res) {
      if (gen !== _auditGen) { return; } // superseded by a newer filter state
      var rows = (res && res.entries) ? res.entries : [];
      var body = document.getElementById('auditBody');
      if (body) {
        rows.forEach(function (e) {
          var tr = document.createElement('tr');
          var tdTime = document.createElement('td');
          tdTime.style.whiteSpace = 'nowrap';
          tdTime.textContent = e.timestamp || '';
          var tdType = document.createElement('td');
          tdType.textContent = e.type || '';
          var tdUser = document.createElement('td');
          tdUser.textContent = e.user || '';
          var tdInfo = document.createElement('td');
          tdInfo.style.whiteSpace = 'normal';
          tdInfo.textContent = e.info || '';
          tr.appendChild(tdTime);
          tr.appendChild(tdType);
          tr.appendChild(tdUser);
          tr.appendChild(tdInfo);
          body.appendChild(tr);
        });
      }
      _auditOffset += rows.length;
      _auditHasMore = !!(res && res.hasMore);
      _auditLoading = false;
      updateAuditStatus();
      // If the first page(s) didn't fill the viewport, keep loading until the
      // sentinel is pushed out of view or there is nothing more to fetch.
      if (_auditHasMore && auditSentinelVisible()) { loadAuditPage(); }
    },
    error: function () {
      if (gen !== _auditGen) { return; }
      _auditLoading = false;
      var s = document.getElementById('auditStatus');
      if (s) { s.textContent = 'Could not load audit log.'; }
    }
  });
}

function auditSentinelVisible() {
  var s = document.getElementById('auditSentinel');
  if (!s) { return false; }
  var r = s.getBoundingClientRect();
  return r.top < (window.innerHeight || document.documentElement.clientHeight);
}

function updateAuditStatus() {
  var cnt = document.getElementById('auditCount');
  if (cnt) {
    cnt.textContent = _auditOffset + (_auditHasMore ? '+' : '') + (_auditOffset === 1 ? ' entry' : ' entries');
  }
  var s = document.getElementById('auditStatus');
  if (!s) { return; }
  if (_auditOffset === 0) {
    s.textContent = 'No matching entries.';
  } else if (_auditHasMore) {
    s.textContent = 'Showing ' + _auditOffset + ' \u2014 scroll for more';
  } else {
    s.textContent = 'Showing all ' + _auditOffset + ' matching entries';
  }
}

function initAuditLog() {
  _auditGen++;
  _auditOffset = 0;
  _auditHasMore = true;
  _auditLoading = false;
  var body = document.getElementById('auditBody');
  if (body) { body.innerHTML = ''; }
  loadAuditPage();
  var sentinel = document.getElementById('auditSentinel');
  if (sentinel && 'IntersectionObserver' in window) {
    if (_auditObserver) { _auditObserver.disconnect(); }
    _auditObserver = new IntersectionObserver(function (entries) {
      if (entries[0].isIntersecting) { loadAuditPage(); }
    }, { threshold: 0 });
    _auditObserver.observe(sentinel);
  }
}

// --- Superadmin one-time audit-log re-import (legacy MySQL) ---
// (Re-import UI removed; the legacy import was a one-time migration step.)
