// Additional functions for the admin panel

// Safe global default. The Desks tab overrides this with the real department
// list via an inline <script> before deskSummary() runs; declaring it here means
// deskSummary() can never throw a ReferenceError even if invoked early.
var departments = {};

function submitWhitelist(WLtype, WLtext) {
  console.log('add to whitelist: '+WLtext+', '+WLtype);
  document.getElementById("ignoreHealthType").value = WLtype;
  document.getElementById("ignoreHealthName").value = WLtext;
  document.getElementById('updateWhitelist').submit();
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
      
      // Output errors in LDAP assignment on left tile
      var color='green'
      var ldaparray = result.health.ldap
      var percentage = ldaparray.length
      if (percentage >=30 ) {color='red';}
      else if (percentage >= 1) {color='orange';}
      else {color='green';}
      var healthldap = ''
      + '<div style="width:750px; height:150px; float:left; margin-left:20px; background:'+color+'; opacity:0.7; text-align:center;line-height:150px;">'
      + '<span style="display: inline-block; vertical-align: middle; line-height: normal;">'
      + '<h1>LDAP errors</h1><h2>'+percentage+'</h2>'
      + '</span>'
      + '</div>'
      for(var i = 0; i <ldaparray.length; i++) {
        healthldap += ''
        + '<div style="width:750px; height:70px; float:left; margin-left:20px; margin-top:5px; background:'+color+'; opacity:0.7; text-align:center;line-height:70px;">'
        + '<span style="float:left; vertical-align: middle; line-height: normal; width:650px; height:70px;">'
        + '<h2>'+ldaparray[i].desk+' assigned to '+ldaparray[i].count+' people: '+ldaparray[i].name+'</h2>'
        + '</span>'
        + '<a href="javascript:{}" onclick="submitWhitelist(\'ldap\',\''+ldaparray[i].desk+'\')">'
        + '<span style="float:left; background-color: #505050; vertical-align: middle; line-height: normal; width:100px; height:70px;">'
        + '<h2>ignore</h2>'
        + '</span>'
        + '</a>'
        + '</div>'
      }
      document.getElementById('healthldap').innerHTML= healthldap;
      
      // Output errors in desk database on right tile
      var color='green'
      var deskarray = result.health.desks
      var percentage = deskarray.length
      if (percentage >=5 ) {color='red';}
      else if (percentage >= 1) {color='orange';}
      else {color='green';}
      healthdesks = ''
      + '<div style="width:750px; height:150px; float:right; margin-right:10px; background:'+color+'; opacity:0.7; text-align:center;line-height:150px;">'
      + '<span style="display: inline-block; vertical-align: middle; line-height: normal;">'
      + '<h1>Desk errors</h1><h2>'+percentage+'</h2>'
      + '</span>'
      + '</div>'
      for(var i = 0; i <deskarray.length; i++) {
        healthdesks += ''
        + '<div style="width:750px; height:70px; float:left; margin-left:20px; margin-top:5px;background:'+color+'; opacity:0.7; text-align:center;line-height:70px;">'
        + '<span style="float:left; vertical-align: middle; line-height: normal; width:650px; height:70px;">'
        + '<h2>'+deskarray[i].desk+' exists '+deskarray[i].count+' times on map '+deskarray[i].map+'</h2>'
        + '</span>'
        + '<a href="javascript:{}" onclick="submitWhitelist(\'desks\',\''+deskarray[i].desk+'\')">'
        + '<span style="float:left; background-color: #505050; vertical-align: middle; line-height: normal; width:100px; height:70px;">'
        + '<h2>ignore</h2>'
        + '</span>'
        + '</a>'
        + '</div>'
      }
      document.getElementById('healthdesks').innerHTML= healthdesks;
      
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
        + '<div id="spinner" style="width:1600px; height:auto; float:left; margin-left:20px;">'
        + '<img src="../images/spinner.png" style="margin-left:672px;" />'
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

function showSyncSub(name) {
  var subs = ['ldap', 'saml', 'robin'];
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

function showRobinTest() {
  var body = document.getElementById('robinDebugBody');
  body.textContent = 'Running meeting sync...';
  document.getElementById('robinDebugOverlay').style.display = 'block';
  $.ajax({
    url: '../rest/robin/test',
    async: true,
    type: 'get',
    dataType: 'JSON',
    success: function(d) {
      var lines = (d && d.log) || [];
      body.textContent = lines.length ? lines.join('\n') : 'No output returned.';
    },
    error: function() {
      body.textContent = 'Failed to run meeting sync (forbidden or server error).';
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
    }
  });
  // Status badge.
  $.ajax({
    url: '../rest/saml/status', type: 'get', dataType: 'JSON',
    success: function(st) {
      var cls = st.enabled ? (st.configured ? 'sync-badge-ok' : 'sync-badge-warn') : 'sync-badge-off';
      var text = st.enabled ? (st.configured ? 'SAML enabled and configured' : 'SAML enabled but incomplete (missing Login URL or certificate)') : 'SAML disabled';
      $('#samlStatusBar').html('<span class="sync-badge '+cls+'" style="font-size:13px;padding:5px 12px;">'+esc(text)+'</span>');
    }
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
            reloadBtn.onclick = function() { window.location.href = '?tab=ldap&sub=' + subTab; };
          }
        }
      },
      error: function() { clearInterval(timer); }
    });
  }, 800);
}

function startRobinSync() {
  startSync('robin', '../rest/robin/sync', '../rest/robin/progress', 'robin');
}

function startLdapSync() {
  startSync('ldap', '../rest/ldap/sync', '../rest/ldap/progress', 'ldap');
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
  var input = row ? row.querySelector('input[type=text]') : null;
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
      if (res.updated > 0) setTimeout(function () { location.reload(); }, 1200);
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
