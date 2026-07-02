// Helper functions required for admins only

function checkHealthStatus() {
  $.ajax({
    url: 'rest/system',
    async: true, 
    type: 'get',
    dataType: 'JSON',
    success: function(result){   
      var healtherrors = result.consistency_ldap + result.consistency_desks
      if (healtherrors == 0) {
        var healthstatus = '<img src="images/dbcheck_ok2.png" style="width:44px;height:44px;" alt="" />'
        document.getElementById('healthstatus').innerHTML= healthstatus
        // Restore the resting (hidden) state so a previously shown red warning
        // disappears once the consistency problem has been resolved.
        $("#healthstatus").css('display','none');
        $("#healthstatus").css('background-color','#333');
      }
      else {
        var healthstatus = '<a href="admin/?tab=dashboard">'
                         + '<img src="images/warning.png" style="width:44px;height:44px;" alt="" />'
                         + '</a>'
        document.getElementById('healthstatus').innerHTML = healthstatus
        $("#healthstatus").css('display','flex');
        $("#healthstatus").css('background-color','#f20000');
      }
      console.log('[HealthStatus] updated');
    }
  })
}

function createDesk(newX,newY) {
  console.log('create new triggered');
  hideSticky();
  doNewItem('hideInputgrid');
  // Create new item
  if (newX > (targetScreenWidth/2)) {
    var editX = Number(newY)+50;
    var editY = Number(newX) - 480;
  }
  else {
    var editX = Number(newY)+50;
    var editY = Number(newX) - 10;
  }

  if (map == "overview") {
    console.log('create new map');
    // Removing sticky first 
    var element = document.getElementById('stickynameplate');
    if (element !== null) {
      element.parentNode.removeChild(element);
    }
    var caption = 'New map';
    var deskClass = 'newmap';
    // Coordinate inputs depend on the overview mode. The preview marker (.newmap)
    // is 66px and anchored by its top-left corner, so subtract half its size to
    // centre it on the click point. The stored coordinate must match where the
    // saved flag is later rendered:
    //   - modern world map: .worldflag is centred on its x/y (translate -50%),
    //     and lat/lon are derived from that same point -> store the click point.
    //   - classic overview: .mapflag is a 30px flag anchored top-left, so store
    //     the click point minus 15 so the flag's centre lands on the click.
    var markerHalf = 33; // half of the 66px preview marker
    var classicHalf = 15; // half of the 30px classic mapflag
    var markerLeft = newX - markerHalf;
    var markerTop = newY - markerHalf;
    var coordFields = '';
    if (typeof setting_worldmap !== 'undefined' && setting_worldmap == 1) {
      var approx = (typeof worldProjection === 'function') ? worldProjection().toLatLon(newX, newY) : { lat: 0, lon: 0 };
      coordFields = '<div class="np-coordwrap">'
                  + '<div class="np-coords">'
                  + '<div class="np-row"><div class="np-label">Latitude *</div><input type="text" class="np-input" id="apimaplat" name="lat" value="' + approx.lat.toFixed(4) + '"></div>'
                  + '<div class="np-row"><div class="np-label">Longitude *</div><input type="text" class="np-input" id="apimaplon" name="lon" value="' + approx.lon.toFixed(4) + '"></div>'
                  + '</div>'
                  + '<input type="button" class="np-geo" value="Get from address" onclick="geocodeNewMap()">'
                  + '</div>'
                  + '<div class="np-row"><div class="np-label"></div><div id="geocodeNewMapMsg" class="np-input np-msg"></div></div>'
                  + '<input type="hidden" id="apimapx" name="x" value="' + newX + '">'
                  + '<input type="hidden" id="apimapy" name="y" value="' + newY + '">';
    } else {
      coordFields = '<div class="np-row"><div class="np-label">x *</div><input type="text" class="np-input" id="apimapx" name="x" value="' + (newX - classicHalf) + '"></div>'
                  + '<div class="np-row"><div class="np-label">y *</div><input type="text" class="np-input" id="apimapy" name="y" value="' + (newY - classicHalf) + '"></div>';
    }
    // create new map instead of item on overview map
    var newdeskitem='';
      newdeskitem +='<div id="newdeskitem" class="' + deskClass + '" style="position:absolute;left:' 
                  + markerLeft + 'px;top:' + markerTop + 'px;border-radius:50%;"></div>'
                  + '<div class="nameplate_edit" style="position:absolute;top:' + editX +'px;left:' + editY + 'px;border-radius:10px;">'
                  + '<div style="position:absolute; top:0px; left:0px; width:100%; font-size:1.5em;line-height:50px; height:50px;'
                  + 'background-color:#666;text-align:center;border-radius:10px 10px 0px 0px;">'+caption+'</div>'
                  + '<div id="formspace">'
                  + '<form class="createItem" style="width:80%; margin-top:60px;margin-left:10%;" enctype="multipart/form-data" action="rest/update/" method="post" onsubmit="return validateNewMap();">'
                  + '<div class="np-row"><div class="np-label">Mapname *</div><input type="text" class="np-input" id="apimapname" name="map"></div>'
                  + '<div class="np-row"><div class="np-label">Itemscale *</div><input type="text" class="np-input" id="apimapitemscale" name="itemscale" value="1"></div>'
                  + '<div class="np-row"><div class="np-label">Published *</div>'
                  + '<select class="np-input" id="apimappublished" name="published">'
                  + '<option value="yes">yes</option> <option value="no">no</option>'
                  + '</select></div>'
                  + '<div class="np-row"><div class="np-label">MapFlag *</div>'
                  + '<div id="mapflags" class="np-input">'
                  + '<select id="selMapflag" class="np-input" name="mapflag">'
                  + '<option value="de">de</option>'
                  + '</select></div></div>';
      newdeskitem+= '<div class="np-row"><div class="np-label">Timezone *</div>'
                  + '<div id="timezones" class="np-input">'
                  + '<select id="selTimezone" class="np-input" name="timezone">'  
                  + '<option value="">-- Select a timezone -- </option>'  
                  + '</select></div></div>';
      newdeskitem+= '<div class="np-row"><div class="np-label">Address</div><input type="text" class="np-input" id="apimapaddress" name="address" placeholder="optional"></div>'
                  + coordFields
                  + '<div class="np-row"><div class="np-label">Floorplan</div><div class="np-input"><input type="file" id="i_file" accept="image/png" name="image" size="30"></div></div>'
                  + '<img src="" width="400" style="display:none;" id="testbild" /><div id="disp_tmp_path"></div>'
                  + '<input type="hidden" name="mode" value="createmap">'
                  + '<input type="hidden" name="token" value="'+token+'">'
                  + '<input type="submit" style="background-color:#0f0" Value="Create item" name="uploadMapfile"></form>'
                  + '<form class="cancelItem" style="width:80%; height: 100%;margin-left:10%;margin-bottom:10px;">'
                  + '<input type="submit" style="background-color:#f00" value="Cancel">'
                  + '</form>'
                  + '</div></div>';
    // Adds sticky to the document
    var p = document.getElementById('content');
    var newElement = document.createElement('div');
    newElement.setAttribute('id', 'stickynameplate');
    newElement.innerHTML = newdeskitem;
    p.appendChild(newElement);

    $('#i_file').change( function(event) {
      tmppath = URL.createObjectURL(event.target.files[0]);
      $("#testbild").fadeIn("fast").attr('src',URL.createObjectURL(event.target.files[0]));
    });

    $.getJSON("tools/timezones.json", function(json) {
      var tzOutput = '<select id="selTimezone" class="np-input" name="timezone">'  
                   + '<option value="">-- Select a timezone -- </option>';
      $.each(json, function( t, timezone ){
        tzOutput+= '<option value="'+timezone+'">'+timezone+'</option>';
      });
      tzOutput += '</select>';
      $("#timezones").html(tzOutput);
    });  
    $.ajax({
      url: 'rest/config?mode=mapflags',
      async: true, 
      type: 'get',
      dataType: 'JSON',
      success: function(result){
        var mfOutput = '<select id="selMapflag" class="np-input" name="mapflag" onchange="switchMapflag()">'  
                   + '<option value="">-- Select a mapflag -- </option>';
        for (var i = 0; i < result.mapflags.length; i++) {
          mfOutput+= '<option value="'+result.mapflags[i]+'">'+result.mapflags[i]+'</option>';
        }
        mfOutput += '</select>';
      $("#mapflags").html(mfOutput);
      }
    });

  }
  else {
    console.log('create new desk');
    var caption = 'New item';
    var deskClass = 'newdesk';
    var newdeskitem ='<div id="newdeskitem" class="' + deskClass + '" style="position:absolute;left:' 
                  + (newX-10) + 'px;top:' + (newY-10) + 'px;border-radius:50%;"></div>'
                  + '<div class="nameplate_edit" style="position:absolute;top:' + editX +'px;left:' + editY + 'px;border-radius:10px;">'
                  + '<div style="position:absolute; top:0px; left:0px; width:100%; font-size:1.5em;line-height:50px; height:50px;'
                  + 'background-color:#666;text-align:center;border-radius:10px 10px 0px 0px;">'+caption+'</div>'
                  + '<form class="createItem" style="width:80%; margin-top:60px;margin-left:10%;">'
                  + '<select id="selDesktype" onchange="addInputfields(' + '666' + ',\'' + deskClass + '\', 3)">'
                  + '<option value="ldap-desk">LDAP synced Desk</option>'
                  + '<option value="blocked">Blocked</option>'
                  + '<option value="exit">Exit</option>'
                  + '<option value="firstaid">First Aid</option>'
                  + '<option value="floor">Floor</option>'
                  + '<option value="food">Food</option>'
                  + '<option value="booking">Booking</option>'
                  + '<option value="hotseat">Hotseat</option>'
                  + '<option value="keycardlock">Keycard Lock</option>'
                  + '<option value="keylock" >Key Lock</option>'
                  + '<option value="meeting">Meeting</option>'
                  + '<option value="printer">Printer</option>'
                  + '<option value="restroom">Restroom</option>'
                  + '<option value="service">Service</option>'
                  + '<option value="local-desk">Non-LDAP Desk</option>'
                  + '</select><div id="inputfields"></div><input type="submit" style="background-color:#0f0" Value="Create item"></form>'
                  + '<form class="cancelItem" style="width:80%; height: 100%;margin-left:10%;margin-bottom:10px;">'
                  + '<input type="submit" style="background-color:#f00" value="Cancel">'
                  + '</form>'
                  + '</div>';
    // Adds sticky to the document
    var p = document.getElementById('content');
    var newElement = document.createElement('div');
    newElement.setAttribute('id', 'stickynameplate');
    newElement.innerHTML = newdeskitem;
    p.appendChild(newElement);

    var manual = {id: 'NULL', map: map, x: newX, y: newY, dsk: '', empl: '', avtr: '', dept: ''};
    addInputfields('newID', deskClass, 2, manual);
    $('.createItem').on('submit', function (e) {
      e.preventDefault();
      itemdesktype = $("#apidesktype").val();
      itemx = $("#apideskx").val();
      itemy = $("#apidesky").val();
      itemdsk = $("#apideskdsk").val();
      itemempl = $("#apideskempl").val();
      itemavtr = $("#apideskavtr").val();
      itemdept = $("#apideskdept").val();
      if (itemdept == "- none -" || itemdept == "") {itemdept = 'NULL';}
      if (itemavtr == "") {itemavtr = 'NULL';}
      $.ajax({
        url: 'rest/update',
        async: true, 
        type: 'get',
        data: {token: token, mode: 'create', map: map, id: 'new', desktype: itemdesktype, x: itemx, y:itemy, desknumber:itemdsk, employee:itemempl, avatar: itemavtr, department: itemdept, user:username},
        dataType: 'JSON',
        success: function(result){
          console.log(result);
          hideSticky();
          updateDesks();
          checkHealthStatus();
        },
        error: function (request, error) {
          alert('Could not create desk. Please check if all attributes have been entered.');
        }
      });
    });
  }
  
  $('.cancelItem').on('submit', function () {
    hideSticky();
  });
  
}

function switchMapflag() {
  var mapValue = $("#selMapflag").val();
  $("#newdeskitem").css("background-image", "url('countryflags/"+mapValue+".svg')");
  $("#newdeskitem").css("background-size", "cover");
  console.log(mapValue);
}

// validateNewMap checks the required fields of the overview "new map" form
// before it is submitted. Coordinates are required as a pair: lat/lon on the
// modern world map, X/Y on the classic overview.
function validateNewMap() {
  var required = [
    ['apimapname', 'Map name'],
    ['apimapitemscale', 'Itemscale'],
    ['apimappublished', 'Published'],
    ['selMapflag', 'Map flag'],
    ['selTimezone', 'Timezone']
  ];
  for (var i = 0; i < required.length; i++) {
    var el = document.getElementById(required[i][0]);
    if (!el || el.value.trim() === '') {
      alert(required[i][1] + ' is required.');
      if (el) { el.focus(); }
      return false;
    }
  }
  if (typeof setting_worldmap !== 'undefined' && setting_worldmap == 1) {
    var lat = document.getElementById('apimaplat');
    var lon = document.getElementById('apimaplon');
    if (!lat || lat.value.trim() === '' || !lon || lon.value.trim() === '') {
      alert('Latitude and longitude are required. Enter them manually or use "Get from address".');
      return false;
    }
  } else {
    var x = document.getElementById('apimapx');
    var y = document.getElementById('apimapy');
    if (!x || x.value.trim() === '' || !y || y.value.trim() === '') {
      alert('X and Y are required.');
      return false;
    }
  }
  return true;
}

// geocodeNewMap fills the lat/lon fields of the overview "new map" form from the
// entered address using the Geoapify integration. If no API key is configured it
// shows a hint instead of failing silently.
function geocodeNewMap() {
  var msg = document.getElementById('geocodeNewMapMsg');
  if (typeof setting_geoapify_configured === 'undefined' || setting_geoapify_configured != 1) {
    msg.style.color = '#fc6';
    msg.innerHTML = 'Geoapify geocoding is not set up. Ask an administrator to add an API key under Admin \u2192 Sync \u2192 Geocoding, or enter latitude/longitude manually.';
    return;
  }
  var addr = document.getElementById('apimapaddress').value.trim();
  if (addr === '') {
    msg.style.color = '#fc6';
    msg.innerHTML = 'Enter an address first.';
    return;
  }
  msg.style.color = '#ccc';
  msg.innerHTML = 'Looking up\u2026';
  $.ajax({
    url: 'rest/geo/test?address=' + encodeURIComponent(addr),
    async: true,
    type: 'get',
    dataType: 'JSON',
    success: function (d) {
      if (d && d.ok) {
        document.getElementById('apimaplat').value = Number(d.lat).toFixed(4);
        document.getElementById('apimaplon').value = Number(d.lon).toFixed(4);
        msg.style.color = '#8f8';
        msg.innerHTML = 'Found: ' + (d.formatted || (Number(d.lat).toFixed(4) + ', ' + Number(d.lon).toFixed(4)));
      } else {
        msg.style.color = '#f88';
        msg.innerHTML = 'Lookup failed: ' + ((d && d.message) || 'unknown error');
      }
    },
    error: function () {
      msg.style.color = '#f88';
      msg.innerHTML = 'Lookup failed (request error).';
    }
  });
}

// ---------------------------------------------------------------------------
// Dynamic world-map "Add location" slide-in form
// ---------------------------------------------------------------------------

// wmaSlugify turns a free-form name (e.g. a city) into a lowercase map identifier.
function wmaSlugify(name) {
  return (name || '')
    .toString()
    .normalize('NFD').replace(/[\u0300-\u036f]/g, '') // strip accents
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '');
}

// openWorldMapAdd populates the selects and slides the panel in from the right.
function openWorldMapAdd() {
  var panel = document.getElementById('worldmap_add_panel');
  if (!panel) { return; }

  // Reset fields.
  document.getElementById('wma_address').value = '';
  document.getElementById('wma_name').value = '';
  document.getElementById('wma_file').value = '';
  document.getElementById('wma_lat').value = '';
  document.getElementById('wma_lon').value = '';
  document.getElementById('wma_itemscale').value = '1';
  document.getElementById('wma_published').value = 'yes';
  document.getElementById('wma_geo_msg').innerHTML = '';
  document.getElementById('wma_geo_msg').style.color = '#ccc';
  var flag = document.getElementById('wma_flag');
  flag.style.display = 'none';
  flag.style.backgroundImage = '';

  // Collapse advanced unless geocoding is unavailable (then coords need manual entry).
  var advConfigured = (typeof setting_geoapify_configured !== 'undefined' && setting_geoapify_configured == 1);
  setWorldMapAdvanced(!advConfigured);
  if (!advConfigured) {
    var msg = document.getElementById('wma_geo_msg');
    msg.style.color = '#fc6';
    msg.innerHTML = 'Geocoding is not configured. Enter latitude/longitude manually under Advanced.';
  }

  // Timezone select (default to the server region until an address sets it).
  $.getJSON('tools/timezones.json', function (json) {
    var sel = document.getElementById('wma_timezone');
    var html = '<option value="">-- select a timezone --</option>';
    $.each(json, function (i, tz) {
      html += '<option value="' + tz + '">' + tz + '</option>';
    });
    sel.innerHTML = html;
    if (typeof region !== 'undefined' && region) { sel.value = region; }
  });

  // Country flag select.
  $.ajax({
    url: 'rest/config?mode=mapflags', async: true, type: 'get', dataType: 'JSON',
    success: function (result) {
      var sel = document.getElementById('wma_mapflag');
      var html = '<option value="none">none</option>';
      var flags = (result && result.mapflags) || [];
      for (var i = 0; i < flags.length; i++) {
        html += '<option value="' + flags[i] + '">' + flags[i] + '</option>';
      }
      sel.innerHTML = html;
    }
  });

  // Anchor the grow-out animation on the + button so the panel appears to
  // expand from it. The panel is position:fixed at right:16/bottom:16, so its
  // final (unscaled) box can be derived from the viewport and its layout size,
  // independent of the current scale(0.2) transform.
  var addBtn = document.getElementById('inputgrid') || document.querySelector('.worldmap-add-btn');
  if (addBtn) {
    var b = addBtn.getBoundingClientRect();
    var panelRight = window.innerWidth - 16;
    var panelBottom = window.innerHeight - 16;
    var panelLeft = panelRight - panel.offsetWidth;
    var panelTop = panelBottom - panel.offsetHeight;
    var ox = (b.left + b.width / 2) - panelLeft;
    var oy = (b.top + b.height / 2) - panelTop;
    panel.style.transformOrigin = ox + 'px ' + oy + 'px';
  }

  // Defer adding .open so the transition runs from the off-screen position.
  requestAnimationFrame(function () { panel.classList.add('open'); });
}

function closeWorldMapAdd() {
  var panel = document.getElementById('worldmap_add_panel');
  if (panel) { panel.classList.remove('open'); }
}

function setWorldMapAdvanced(show) {
  var box = document.getElementById('wma_advanced');
  var toggle = document.getElementById('wma_advanced_toggle');
  if (!box || !toggle) { return; }
  box.style.display = show ? 'block' : 'none';
  toggle.classList.toggle('open', show);
  toggle.innerHTML = show ? 'Advanced \u25BE' : 'Advanced \u25B8';
}

function toggleWorldMapAdvanced() {
  var box = document.getElementById('wma_advanced');
  setWorldMapAdvanced(box.style.display === 'none');
}

// updateWorldMapFlagPreview reflects the chosen country flag next to the name.
function updateWorldMapFlagPreview() {
  var cc = document.getElementById('wma_mapflag').value;
  var flag = document.getElementById('wma_flag');
  if (cc && cc !== 'none' && cc !== '') {
    flag.style.backgroundImage = 'url(countryflags/' + cc + '.svg)';
    flag.style.display = 'inline-block';
  } else {
    flag.style.backgroundImage = '';
    flag.style.display = 'none';
  }
}

// geocodeWorldMapAdd resolves the entered address to coordinates and auto-fills
// lat/lon, the country flag, the timezone and a suggested map name.
function geocodeWorldMapAdd() {
  var addr = document.getElementById('wma_address').value.trim();
  var msg = document.getElementById('wma_geo_msg');
  if (addr === '') { return; }
  if (typeof setting_geoapify_configured === 'undefined' || setting_geoapify_configured != 1) {
    msg.style.color = '#fc6';
    msg.innerHTML = 'Geocoding is not configured. Enter latitude/longitude manually under Advanced.';
    setWorldMapAdvanced(true);
    return;
  }
  msg.style.color = '#ccc';
  msg.innerHTML = 'Looking up\u2026';
  $.ajax({
    url: 'rest/geo/test?address=' + encodeURIComponent(addr),
    async: true, type: 'get', dataType: 'JSON',
    success: function (d) {
      if (d && d.ok) {
        document.getElementById('wma_lat').value = Number(d.lat).toFixed(4);
        document.getElementById('wma_lon').value = Number(d.lon).toFixed(4);
        // Country flag, if we have a matching flag option.
        if (d.country) {
          var mf = document.getElementById('wma_mapflag');
          var found = false;
          for (var i = 0; i < mf.options.length; i++) {
            if (mf.options[i].value === d.country) { mf.value = d.country; found = true; break; }
          }
          if (found) { updateWorldMapFlagPreview(); }
        }
        // Timezone, if returned and present in the select.
        if (d.timezone) {
          var tz = document.getElementById('wma_timezone');
          for (var j = 0; j < tz.options.length; j++) {
            if (tz.options[j].value === d.timezone) { tz.value = d.timezone; break; }
          }
        }
        // Suggest a map name from the city when the user has not typed one.
        var nameField = document.getElementById('wma_name');
        if (nameField.value.trim() === '' && d.city) {
          nameField.value = wmaSlugify(d.city);
        }
        msg.style.color = '#8f8';
        msg.innerHTML = 'Found: ' + (d.formatted || (Number(d.lat).toFixed(4) + ', ' + Number(d.lon).toFixed(4)));
      } else {
        msg.style.color = '#f88';
        msg.innerHTML = 'Lookup failed: ' + ((d && d.message) || 'unknown error');
      }
    },
    error: function () {
      msg.style.color = '#f88';
      msg.innerHTML = 'Lookup failed (request error).';
    }
  });
}

// submitWorldMapAdd validates and posts the new map to rest/update (mode=createmap).
function submitWorldMapAdd() {
  var msg = document.getElementById('wma_geo_msg');
  var name = wmaSlugify(document.getElementById('wma_name').value);
  var lat = document.getElementById('wma_lat').value.trim();
  var lon = document.getElementById('wma_lon').value.trim();
  var itemscale = document.getElementById('wma_itemscale').value.trim() || '1';
  var published = document.getElementById('wma_published').value;
  var mapflag = document.getElementById('wma_mapflag').value || 'none';
  var timezone = document.getElementById('wma_timezone').value;
  var address = document.getElementById('wma_address').value.trim();

  if (name === '') {
    msg.style.color = '#f88';
    msg.innerHTML = 'Please enter a map name.';
    return;
  }
  if (lat === '' || lon === '') {
    msg.style.color = '#f88';
    msg.innerHTML = 'Coordinates are missing. Enter an address to look them up, or set them under Advanced.';
    setWorldMapAdvanced(true);
    return;
  }
  if (timezone === '') {
    msg.style.color = '#f88';
    msg.innerHTML = 'Please choose a timezone under Advanced.';
    setWorldMapAdvanced(true);
    return;
  }

  var fd = new FormData();
  fd.append('mode', 'createmap');
  fd.append('token', token);
  fd.append('map', name);
  fd.append('itemscale', itemscale);
  fd.append('published', published);
  fd.append('mapflag', mapflag);
  fd.append('timezone', timezone);
  fd.append('address', address);
  fd.append('lat', lat);
  fd.append('lon', lon);
  var fileInput = document.getElementById('wma_file');
  if (fileInput.files && fileInput.files.length > 0) {
    fd.append('image', fileInput.files[0]);
  }

  var btn = document.getElementById('wma_create_btn');
  btn.disabled = true;
  msg.style.color = '#ccc';
  msg.innerHTML = 'Creating\u2026';

  fetch('rest/update/', { method: 'POST', body: fd, credentials: 'same-origin' })
    .then(function (resp) {
      if (resp.ok || resp.redirected || resp.status === 303) {
        window.location.href = '/?map=overview';
        return null;
      }
      if (resp.status === 409) { throw new Error('That map name is already in use.'); }
      return resp.text().then(function (t) { throw new Error(t || ('HTTP ' + resp.status)); });
    })
    .catch(function (err) {
      btn.disabled = false;
      msg.style.color = '#f88';
      msg.innerHTML = 'Could not create map: ' + err.message;
    });
}

// --- Property-form field builders (shared by the sidebar add/edit form) ---
// Each row keeps the same element id/name the create/update submit handlers
// expect; only the layout/order changes (essential fields first).
function sbTextRow(label, id, name, value) {
  var v = (value === undefined || value === null) ? '' : value;
  return '<div class="sbrow"><div class="sblabel">' + label + '</div>'
       + '<input type="text" class="sbinput" id="' + id + '" name="' + name + '" value="' + v + '"></div>';
}
function sbHidden(id, name, value) {
  var v = (value === undefined || value === null) ? '' : value;
  return '<input type="hidden" id="' + id + '" name="' + name + '" value="' + v + '">';
}
function sbDeptRow(selectedDept) {
  var s = '<div class="sbrow"><div class="sblabel">Department</div>'
        + '<select id="apideskdept" name="formdept" class="sbinput">';
  $.each(departments, function (x, department) {
    s += '<option value="' + department + '"' + (department == selectedDept ? ' selected' : '') + '>' + department + '</option>';
  });
  s += '</select></div>';
  return s;
}
// Reserved desknumber keywords (POI markers) are blanked when converting an
// item to a desk so the editor is not asked to keep a placeholder name.
function sbReservedDsk(d) {
  switch (d) {
    case "Exit": case "FirstAid": case "Floor": case "Food":
    case "KeycardLock": case "KeyLock": case "Meeting":
    case "Printer": case "Restroom": case "Service":
      return true;
  }
  return false;
}
function sbAdvanced(advHtml) {
  if (!advHtml) { return ''; }
  return '<div class="sbadv_divider">Advanced</div>' + advHtml;
}

function addInputfields(deskid, desktype, override, manual) {

  // New items overwrite all automatic settings
  if (override == 3) {
    var input = {id: 'NULL', map: map, x: $("#apideskx").val(), y: $("#apidesky").val(), dsk: $("#apideskdsk").val(), empl: $("#apideskempl").val(), avtr: $("#apideskavtr").val(), dept: $("#apideskdept").val()};
    var selected = $("#selDesktype").val();
    // "ldap-mirror" is the hidden technical employee marker used by directory
    // desks, not a real description. Don't let it carry over into another type's
    // Description field when the editor switches the draft's type.
    if (input.empl === 'ldap-mirror') { input.empl = ''; }
  }
  else if (override == 2) {
    var input = manual;
    var selected = desktype;
    $("#selDesktype").val(desktype);

  }
  else {
    attr = result_old.desks.find(o => Object.entries(o).find(([k, value]) => k === 'id' && value === deskid) !== undefined);
    var input = attr;
    // A Robin-occupied desk arrives as a synthetic "occupied" overlay whose
    // desktype/empl/avatar reflect the LIVE occupant, not the stored item. Edit
    // the real underlying configuration instead so we never overwrite it with
    // transient Robin values. Desknumber/department/coordinates are already the
    // stored config, so only type/empl/avatar need restoring.
    if (input && input.configtype) {
      var cfgMap = { addesk: 'ldap-desk', localdesk: 'local-desk' };
      desktype = cfgMap[input.configtype] || input.configtype;
      input = { id: input.id, map: input.map, x: input.x, y: input.y, dsk: input.dsk,
                empl: (input.configempl || ''), avtr: (input.configavtr || ''), dept: input.dept };
    }
    if (desktype == "occupiedldap") {desktype="ldap-desk";}
    if (desktype == "occupied") {desktype="local-desk";}
    if (desktype == "free") {
      if (input.desktype == 'addesk') {desktype = "ldap-desk"}
      else {desktype="local-desk";}
    }
    if (desktype == "hotseat_free" || desktype == "hotseat_booked") {
      desktype = "hotseat";
    }
    if (desktype == "booking_free" || desktype == "booking_booked") {
      desktype = "booking";
    }
    
    var selected = $("#selDesktype").val();
    if (typeof selected === 'undefined' || override == 1) {
      var selected = desktype;
      if (desktype == 'shareddesk') {desktype = 'ldap-desk'}
      $("#selDesktype").val(desktype);
      //console.log('desktype: '+desktype);
    }
  }

  // Build the property form: essential (semantic) fields first, then an
  // "Advanced" block with coordinates + avatar, then hidden inputs. All element
  // ids/names are preserved so the create/update submit handlers work unchanged.
  var ess = ''; // essential fields (top)
  var adv = ''; // advanced fields (below the divider)
  var hid = ''; // hidden inputs
  switch (selected) {
    case "exit":
    case "firstaid":
    case "food":
    case "keycardlock":
    case "keylock":
    case "printer":
    case "restroom":
    case "service":
        // These POI markers have no meaningful desknumber: it is a fixed reserved
        // keyword derived from the type, so we keep it as a hidden field instead
        // of asking the editor for it. Only the description + position matter.
        ess += sbTextRow('Description', 'apideskempl', 'formempl', input.empl);
        adv += sbTextRow('x', 'apideskx', 'formx', input.x);
        adv += sbTextRow('y', 'apidesky', 'formy', input.y);
        hid += sbHidden('apideskdsk', 'formdsk', type2keyword(selected));
        hid += sbHidden('apideskavtr', 'formavtr', selected);
        hid += sbHidden('apideskdept', 'formdept', '- none -');
        hid += sbHidden('apidesktype', 'formdesktype', selected);
        break;
    case "floor":
        // Floor X is locked to the rail; only Y + label are editable.
        ess += sbTextRow('Label', 'apideskempl', 'formempl', input.empl);
        adv += sbTextRow('y', 'apidesky', 'formy', input.y);
        hid += sbHidden('apideskx', 'formx', FLOOR_RAIL_X);
        hid += sbHidden('apideskdsk', 'formdsk', 'Floor');
        hid += sbHidden('apideskavtr', 'formavtr', selected);
        hid += sbHidden('apideskdept', 'formdept', '- none -');
        hid += sbHidden('apidesktype', 'formdesktype', selected);
        break;
    case "meeting":
        ess += sbTextRow('Desknumber', 'apideskdsk', 'formdsk', input.dsk);
        ess += sbTextRow('Description', 'apideskempl', 'formempl', input.empl);
        adv += sbTextRow('x', 'apideskx', 'formx', input.x);
        adv += sbTextRow('y', 'apidesky', 'formy', input.y);
        // Avatar (room preview image) is unused for meeting rooms, so keep it as a
        // hidden field (preserving any stored value) instead of showing it.
        hid += sbHidden('apideskavtr', 'formavtr', input.avtr);
        hid += sbHidden('apideskdept', 'formdept', '- none -');
        hid += sbHidden('apidesktype', 'formdesktype', selected);
        break;
    case "ldap-desk":
    case "shareddesk":
        ess += sbTextRow('Desknumber', 'apideskdsk', 'formdsk', sbReservedDsk(input.dsk) ? '' : input.dsk);
        ess += sbDeptRow(input.dept);
        adv += sbTextRow('x', 'apideskx', 'formx', input.x);
        adv += sbTextRow('y', 'apidesky', 'formy', input.y);
        hid += sbHidden('apidesktype', 'formdesktype', 'addesk');
        hid += '<input type="hidden" id="apideskempl" name="apideskempl" value="ldap-mirror">';
        hid += sbHidden('apideskavtr', 'formavtr', input.avtr);
        break;
    case "blocked":
    case "booking":
    case "hotseat":
        ess += sbTextRow('Desknumber', 'apideskdsk', 'formdsk', sbReservedDsk(input.dsk) ? '' : input.dsk);
        ess += sbTextRow('Description', 'apideskempl', 'formempl', input.empl);
        ess += sbDeptRow(input.dept);
        adv += sbTextRow('x', 'apideskx', 'formx', input.x);
        adv += sbTextRow('y', 'apidesky', 'formy', input.y);
        hid += sbHidden('apidesktype', 'formdesktype', selected);
        hid += sbHidden('apideskavtr', 'formavtr', selected);
        break;
    case "local-desk":
        ess += sbTextRow('Desknumber', 'apideskdsk', 'formdsk', input.dsk);
        ess += sbTextRow('Description', 'apideskempl', 'formempl', input.empl);
        ess += sbDeptRow(input.dept);
        adv += sbTextRow('x', 'apideskx', 'formx', input.x);
        adv += sbTextRow('y', 'apidesky', 'formy', input.y);
        adv += sbTextRow('Avatar', 'apideskavtr', 'formavtr', input.avtr);
        hid += sbHidden('apidesktype', 'formdesktype', 'localdesk');
        break;
    case "newdesk":
        ess += sbTextRow('Desknumber', 'apideskdsk', 'formdsk', '');
        ess += sbTextRow('Description', 'apideskempl', 'formempl', input.empl);
        ess += sbDeptRow(input.dept);
        adv += sbTextRow('x', 'apideskx', 'formx', input.x);
        adv += sbTextRow('y', 'apidesky', 'formy', input.y);
        hid += sbHidden('apideskavtr', 'formavtr', input.avtr);
        hid += sbHidden('apidesktype', 'formdesktype', 'addesk');
        break;
  }
  $("#inputfields").html(ess + sbAdvanced(adv) + hid);
}

function type2keyword (word) {
  switch (word) {
    case "meeting":
    case "restroom":
    case "printer":
    case "food":
    case "floor":
    case "service":
    case "exit":
      return ucWords(word);;
      break;
    case "firstaid":
      return "FirstAid";
      break;
    case "keycardlock":
      return "KeycardLock";
      break;
    case "keylock":
      return "KeyLock";
      break;
    case "blocked":
      return "Blocked";
      break;
    case "hotseat":
      return "HotSeat";
      break;
  }
    
}

function getClickPosition(e) {
  var xPosition = e.pageX;
  var yPosition = e.pageY;
  var CookieAutozoom = getCookie("autozoom");
  var CookieZoom = getCookie("zoom")/100;
  var CookieLeftPos = getCookie("LeftPos");

  var xOutput = (xPosition-CookieLeftPos)/CookieAutozoom/CookieZoom;
  var xWidth = targetScreenWidth*CookieAutozoom*CookieZoom;

  var yMargin = 69*CookieAutozoom;
  var yOutput = (yPosition-yMargin)/CookieAutozoom/CookieZoom;

  if($(e.target).is('.inputgridbutton')){
    e.preventDefault();
    return;
  }
    var newX=parseInt(xOutput);
    var newY=parseInt(yOutput);
    createDesk(newX,newY);
} 

function doNewItem(action) {
  switch (action) {
    case "showInputgrid":
      // On the dynamic world-map overview, creating a map is address-driven via a
      // slide-in form instead of the classic click-to-place flow.
      if (map == "overview" && typeof setting_worldmap !== 'undefined' && setting_worldmap == 1) {
        openWorldMapAdd();
        return false;
      }
      var addButton = '<input class="inputgridbutton" type="image" src="images/add_on.png" style="width:80px; height:80px;" onClick="return doNewItem(\'hideInputgrid\')" onmouseover=this.src="images/add.png" onmouseout=this.src="images/add_on.png">';
      document.body.classList.add("addmode");
      document.body.addEventListener("click", getClickPosition, false);
      $('#inputgrid').html(addButton);
      $('#newitem_container').hide();
      $('#newbox').hide();
      break;

    case "hideInputgrid":
      var addButton = '<input class="inputgridbutton" type="image" src="images/add.png" style="width:80px; height:80px;" onClick="return doNewItem(\'showInputgrid\')" onmouseover=this.src="images/add_on.png" onmouseout=this.src="images/add.png">';
        document.body.classList.remove("addmode");
      document.body.removeEventListener("click", getClickPosition, false);
      $('#inputgrid').html(addButton);
      $('#newitem_container').hide();
      $('#newbox').hide();
      break;
  }
}; 

function toggleStats() {
  if (document.getElementById("statsmenu").style.left != "-10px") {document.getElementById("statsmenu").style.left = "-10px"; }
  else {document.getElementById("statsmenu").style.left = "-"+(620*autozoom)+"px";}
}

function addTimezoneSelection() {
  var tzStrings = $.getJSON("tools/timezones.json", function(json) {
    console.log(json); 
  });
}

//Make the DIV element draggagle:
function dragElement(elmnt, deskType) {

  var dragItem="16";
  var startItemX;
  var startItemY;
  var startJsX;
  var startJsY;
  var diffX;
  var diffY;
  var lastDragClientX;
  var lastDragClientY;
  var dragStarted;

  // Minimum cursor travel (in screen pixels) before a press is treated as a
  // drag rather than a click. Without this, the tiniest jitter while clicking an
  // item nudges it and triggers a "move" (DB update) instead of selecting it.
  var DRAG_THRESHOLD = 5;

  if (document.getElementById(elmnt.id)) {
    document.getElementById(elmnt.id).onmousedown = dragMouseDown;
  } else {
    elmnt.onmousedown = dragMouseDown;
  }

  function dragMouseDown(e) {
    e = e || window.event;
    e.preventDefault();
    var elementId = (e.target || e.srcElement).id;
    // get the mouse cursor position at startup:    
    dragItem= elementId;
    startItemX = parseInt($('#'+elementId).css("left"));
    startItemY = parseInt($('#'+elementId).css("top"));
    startJsX = e.clientX;
    startJsY = e.clientY;
    dragStarted = false;
    document.onmouseup = closeDragElement;
    // call a function whenever the cursor moves:
    document.onmousemove = elementDrag;
  }

  function elementDrag(e) {
    e = e || window.event;
    e.preventDefault();
    var elementId = (e.target || e.srcElement).id;

    // Ignore sub-threshold jitter so a click that wiggles slightly still selects
    // the item instead of moving it. Once the cursor crosses the threshold the
    // press becomes a real drag for the rest of the gesture.
    if (!dragStarted) {
      if (Math.abs(e.clientX - startJsX) < DRAG_THRESHOLD &&
          Math.abs(e.clientY - startJsY) < DRAG_THRESHOLD) {
        return;
      }
      dragStarted = true;
    }

    hideNameplate(1);
    
    // calc page scaling. #content now uses CSS zoom (not transform), and each
    // desk ball additionally uses zoom:itemscale, so the cursor delta in screen
    // pixels must be divided by both to get the desk ball's own layout delta.
    var pageScale = parseFloat(window.getComputedStyle(document.getElementById('content')).zoom) || 1;

    // calculate the new cursor position:
    diffX = (e.clientX-startJsX)/(pageScale*itemscale);
    diffY = (e.clientY-startJsY)/(pageScale*itemscale);
    diffItemX = parseInt($('#'+elementId).css("left"))-startItemX;
    diffItemY = parseInt($('#'+elementId).css("top"))-startItemY;

    dragItem= elmnt.id;
    // set the element's new position:
    if (deskType === 'floor') {
      // Floor markers are rail-locked: only their vertical position changes.
      elmnt.style.top = (startItemY + diffY) + "px";
    } else {
      elmnt.style.left = (startItemX + diffX) + "px";
      elmnt.style.top = (startItemY + diffY) + "px";
    }

    // Track the cursor and highlight the trash drop zone when hovering it.
    lastDragClientX = e.clientX;
    lastDragClientY = e.clientY;
    var trashEl = document.getElementById('editsidebar_trash');
    if (trashEl) {
      if (pointOverTrash(e.clientX, e.clientY)) { trashEl.classList.add('dragover'); }
      else { trashEl.classList.remove('dragover'); }
    }

  }

  function closeDragElement(e) {
    /* stop moving when mouse button is released:*/
    document.onmouseup = null;
    document.onmousemove = null;
    var trashEl = document.getElementById('editsidebar_trash');
    if (trashEl) { trashEl.classList.remove('dragover'); }
    // If the item was released over the trash zone, delete it instead of moving.
    var cx = (e && typeof e.clientX === 'number') ? e.clientX : lastDragClientX;
    var cy = (e && typeof e.clientY === 'number') ? e.clientY : lastDragClientY;
    if (typeof cx === 'number' && pointOverTrash(cx, cy)) {
      deleteDeskById(elmnt.id);
      return;
    }
    var x = parseInt($('#'+dragItem).css("left"));
    var y = parseInt($('#'+dragItem).css("top"));
    if (x == startItemX && y == startItemY) {
      //console.log('Item has been clicked');
      //console.log('"'+elmnt.id+'","'+deskType+'"');
      showSticky(elmnt.id, deskType);
    }
    else {
      //console.log('Item has been moved');
      attr = result_old.desks.find(o => Object.entries(o).find(([k, value]) => k === 'id' && value === elmnt.id) !== undefined);
      itemid = attr.id;
      itemdesktype = attr.desktype;
      itemx = parseInt(attr.x)+Math.round(diffItemX*itemscale);
      itemy = parseInt(attr.y)+Math.round(diffItemY*itemscale);
      // Floor markers are locked to the rail X regardless of horizontal drag.
      if (deskType === 'floor') { itemx = FLOOR_RAIL_X; }
      itemdsk = attr.dsk;
      itemempl = attr.empl;
      itemavtr = attr.avtr;
      itemdept = attr.dept;
      if (itemavtr == '') {itemavtr = '-'}
      if (itemdept == '') {itemdept = '- none -'}

      $.ajax({
        url: 'rest/update',
        async: true, 
        type: 'get',
        data: {token: token, mode: 'update', map: mapname, id: itemid, desktype: itemdesktype, x: itemx, y:itemy, desknumber:itemdsk, employee:itemempl, avatar: itemavtr, department: itemdept, user: username},
        dataType: 'JSON',
        success: function(result){
          updateDesks();
          checkHealthStatus();
        },
        error: function (result) {
          alert('Could not update desk. Please check if all attributes have been entered.');
        }
      });
    }
  }
}

// ---------------------------------------------------------------------------
// Edit palette sidebar (drag-and-drop item placement)
// ---------------------------------------------------------------------------
// A right-side sidebar (mirrors the left search sidebar) that holds a catalog
// of map items grouped by category. Each tile shows a live preview + a short
// description so editors know what each item is for. It opens automatically in
// edit mode and replaces the old "+" button / click-to-place flow.

var EDIT_SIDEBAR_WIDTH = 340;

// Catalog of placeable items. `color`/`icon`/`square` mirror the on-map look
// (see .deskball and the per-type classes in cmaps.css) so each tile previews
// exactly what will be placed.
var EDIT_PALETTE = [
  { section: 'Desks', items: [
    { type: 'ldap-desk',  label: 'Directory desk', desc: 'Seat linked to a directory user; the assignee fills in automatically.', color: 'rgba(180,180,180,0.85)' },
    { type: 'local-desk', label: 'Custom desk',    desc: 'Manually managed seat with your own name, avatar and department.',     color: 'rgba(0,0,255,0.5)' },
    { type: 'hotseat',    label: 'Hot seat',       desc: 'Flexible first-come desk, shown in red.',                               color: 'rgba(208,19,23,0.8)' },
    { type: 'booking',    label: 'Bookable desk',  desc: 'Reservable desk (green); users can book it for a day.',                color: 'rgba(61,173,30,0.8)' },
    { type: 'blocked',    label: 'Blocked',        desc: 'Marks an unavailable or out-of-service spot.',                         color: 'rgba(180,180,180,0.85)' }
  ]},
  { section: 'Desk clusters', items: [
    { cluster: true, count: 4, cols: 2, rows: 2, diagonal: false, label: '4 desks', desc: 'A tidy 2\u00d72 block of four desks.', color: 'rgba(0,0,255,0.5)' },
    { cluster: true, count: 6, cols: 3, rows: 2, diagonal: false, label: '6 desks', desc: 'A tidy 3\u00d72 block of six desks.', color: 'rgba(0,0,255,0.5)' }
  ]},
  { section: 'Rooms & areas', items: [
    { type: 'meeting', label: 'Meeting room', desc: 'Conference room with live availability.', color: 'rgba(137,26,183,0.8)', icon: 'meeting.png' },
    { type: 'restroom', label: 'Restroom',     desc: 'Toilets / washroom.', color: 'rgba(78,81,100,0.8)', icon: 'restroom.png' }
  ]},
  { section: 'Points of interest', items: [
    { type: 'floor',       label: 'Floor', desc: 'Navigation marker on the right-hand rail; jumps to a floor or section. Only its vertical position matters.', color: '#d017a8b3', square: true, icon: 'floor2.png' },
    { type: 'exit',        label: 'Exit',          desc: 'Emergency exit marker.',         color: 'rgba(84,185,72,0.8)',  icon: 'exit.png' },
    { type: 'food',        label: 'Food & drink',  desc: 'Kitchen, canteen or coffee point.', color: 'rgba(215,125,40,0.8)', icon: 'food.png' },
    { type: 'printer',     label: 'Printer',       desc: 'Printer or copier station.',     color: 'rgba(50,50,50,0.8)',   icon: 'printer.png' },
    { type: 'firstaid',    label: 'First aid',     desc: 'First-aid kit or station.',      color: 'rgba(220,50,50,0.8)',  icon: 'firstaid.png' },
    { type: 'service',     label: 'Service point', desc: 'Help desk or service point.',    color: 'rgba(70,190,190,0.8)', icon: 'service.png' },
    { type: 'keycardlock', label: 'Keycard door',  desc: 'Door secured by a keycard.',     color: 'rgba(240,220,0,0.85)', icon: 'keycardlock.png' },
    { type: 'keylock',     label: 'Key door',      desc: 'Door secured by a physical key.', color: 'rgba(240,220,0,0.85)', icon: 'keylock.png' }
  ]}
];

// Quick lookup of a catalog entry by its desktype.
var EDIT_PALETTE_BY_TYPE = (function () {
  var m = {};
  EDIT_PALETTE.forEach(function (sec) {
    sec.items.forEach(function (it) { m[it.type] = it; });
  });
  return m;
})();

// Desk types selectable for cluster placement (mirrors the "Desks" section).
// `type` is the palette key chosen in the dropdown; clusterDeskFields() maps it
// to the stored desktype plus the matching employee/avatar/department defaults.
var CLUSTER_DESK_OPTIONS = [
  { type: 'ldap-desk',  label: 'Directory desk' },
  { type: 'local-desk', label: 'Custom desk' },
  { type: 'hotseat',    label: 'Hot seat' },
  { type: 'booking',    label: 'Bookable desk' },
  { type: 'blocked',    label: 'Blocked' }
];

// Resolve the stored desk fields for a cluster palette type. Mirrors what the
// single-desk editor form submits per type so batch-created cluster desks are
// indistinguishable from individually placed ones.
function clusterDeskFields(paletteType) {
  switch (paletteType) {
    case 'local-desk': return { desktype: 'localdesk', employee: '',            avatar: '-',       department: '' };
    case 'hotseat':    return { desktype: 'hotseat',   employee: 'HotSeat',     avatar: 'hotseat', department: '' };
    case 'booking':    return { desktype: 'booking',   employee: 'Booking',     avatar: 'booking', department: '' };
    case 'blocked':    return { desktype: 'blocked',   employee: 'Blocked',     avatar: 'blocked', department: '' };
    case 'ldap-desk':
    default:           return { desktype: 'addesk',    employee: 'ldap-mirror', avatar: '-',       department: '' };
  }
}

// Persisted cluster desk-type choice (cookie). Defaults to Directory desk.
function getClusterDeskType() {
  var v = (typeof getCookie === 'function') ? getCookie('cluster_desktype') : '';
  return v || 'ldap-desk';
}
function setClusterDeskType(v) {
  document.cookie = 'cluster_desktype=' + v + '; expires=Fri, 31 Dec 9999 23:59:59 GMT; SameSite=Lax';
}

// On-map colour of the currently selected cluster desk type, so cluster previews
// and the drag ghost match what will actually be placed.
function clusterDeskColor() {
  var it = EDIT_PALETTE_BY_TYPE[getClusterDeskType()];
  return (it && it.color) || 'rgba(0,0,255,0.5)';
}

// Repaint the cluster preview dots in the sidebar to the selected type's colour.
function refreshClusterPreviews() {
  var color = clusterDeskColor();
  var dots = document.querySelectorAll('.editsidebar_clusterdot');
  for (var i = 0; i < dots.length; i++) { dots[i].style.background = color; }
}

// Admin-defined custom item types (injected by index.html as `customItemTypes`,
// keyed by id). Each becomes a draggable palette tile under "Custom items" and
// is stored on desks with desktype "custom_<id>".
function customTypeMap() {
  return (typeof customItemTypes !== 'undefined' && customItemTypes) ? customItemTypes : {};
}

// Half the on-map CSS box size (content space) for a custom type's named size,
// mirroring CustomItemType.Halfsize() in db.go and updateDesks() in user.js.
function customHalfsizeForSize(size) {
  switch (size) {
    case 'small': return 18;
    case 'large': return 40;
    default: return 25;
  }
}

// The custom type definition for a desktype like "custom_plant", or null.
function customTypeDef(type) {
  if (!type || type.indexOf('custom_') !== 0) { return null; }
  var id = type.slice(7);
  var m = customTypeMap();
  return m[id] || null;
}

// Build palette tile descriptors for every configured custom item type.
function customPaletteItems() {
  var m = customTypeMap();
  var items = [];
  Object.keys(m).forEach(function (id) {
    var t = m[id];
    items.push({
      custom: true,
      type: 'custom_' + id,
      label: t.label || id,
      desc: t.description || 'Custom item',
      color: t.color || '#0979D8',
      iconURL: t.icon || '',
      size: t.size || 'medium'
    });
  });
  items.sort(function (a, b) { return a.label.toLowerCase() < b.label.toLowerCase() ? -1 : 1; });
  return items;
}

// Half the on-map CSS box size (in pre-zoom 1600px space) for a palette type,
// matching the per-type halfsize used by updateDesks() in user.js. The rendered
// on-screen diameter of an item is 2*halfsize * itemscale * contentZoom.
function editItemHalfsize(type) {
  var custom = customTypeDef(type);
  if (custom) { return customHalfsizeForSize(custom.size); }
  switch (type) {
    case 'meeting':
    case 'exit':
    case 'restroom':
      return 25;
    case 'firstaid':
    case 'food':
    case 'keycardlock':
    case 'keylock':
    case 'printer':
    case 'service':
      return 18;
    case 'floor':
      return 13; // floor rail tab (half tab height; X is rail-locked)
    default:
      return 10; // desks (ldap/local/hotseat/booking/blocked)
  }
}

// Build the inline style for an item's preview swatch (circle, or square for
// floor/zone), tinted and icon'd to match how it renders on the map.
function editPaletteIconStyle(item) {
  var s = 'background-color:' + item.color + ';';
  s += item.square ? 'border-radius:6px;' : 'border-radius:50%;';
  if (item.iconURL) {
    s += 'background-image:url("' + item.iconURL + '");background-size:cover;';
  } else if (item.icon) {
    s += 'background-image:url("images/' + item.icon + '");background-size:cover;';
  }
  return s;
}

// Render the palette tiles into the sidebar (once).
function renderEditPalette() {
  var inner = document.getElementById('editsidebar_inner');
  if (!inner) { return; }
  inner.innerHTML = '';

  // Top row: the trash drop zone (left) and the auto-align tool (right), 50:50.
  var tools = document.createElement('div');
  tools.className = 'editsidebar_tools';

  var trash = document.createElement('div');
  trash.id = 'editsidebar_trash';
  trash.className = 'edit_sidebar_trash';
  trash.title = 'Drag an item here to delete it';
  trash.innerHTML = '<div class="edit_sidebar_trash_icon"></div>'
                  + '<div class="edit_sidebar_trash_label">Delete</div>';
  tools.appendChild(trash);

  var alignBtn = document.createElement('button');
  alignBtn.type = 'button';
  alignBtn.id = 'editsidebar_alignbtn';
  alignBtn.className = 'editsidebar_toolbtn';
  alignBtn.title = 'Drag a box around a group of desks to tidy them into evenly aligned rows and columns (preview before applying).';
  alignBtn.innerHTML = '<span class="editsidebar_toolbtn_icon"></span>'
                     + '<span class="editsidebar_toolbtn_label">Auto-align</span>';
  alignBtn.addEventListener('click', function () { startAutoAlign('box'); });
  tools.appendChild(alignBtn);

  var lassoBtn = document.createElement('button');
  lassoBtn.type = 'button';
  lassoBtn.id = 'editsidebar_lassobtn';
  lassoBtn.className = 'editsidebar_toolbtn';
  lassoBtn.title = 'Draw a freeform loop around desks to tidy them into evenly aligned rows and columns (preview before applying).';
  lassoBtn.innerHTML = '<span class="editsidebar_toolbtn_icon editsidebar_toolbtn_icon_lasso"></span>'
                     + '<span class="editsidebar_toolbtn_label">Lasso align</span>';
  lassoBtn.addEventListener('click', function () { startAutoAlign('lasso'); });
  tools.appendChild(lassoBtn);

  inner.appendChild(tools);

  EDIT_PALETTE.forEach(function (sec) {
    var h = document.createElement('div');
    h.className = 'editsidebar_section';
    h.textContent = sec.section;
    inner.appendChild(h);

    // Desk clusters can be created as any desk type; offer a dropdown (default
    // Directory desk) whose choice is remembered in a cookie.
    if (sec.section === 'Desk clusters') {
      var typeRow = document.createElement('div');
      typeRow.className = 'editsidebar_clustertype';
      var typeLbl = document.createElement('label');
      typeLbl.setAttribute('for', 'cluster_desktype_sel');
      typeLbl.textContent = 'Desk type';
      var typeSel = document.createElement('select');
      typeSel.id = 'cluster_desktype_sel';
      CLUSTER_DESK_OPTIONS.forEach(function (opt) {
        var o = document.createElement('option');
        o.value = opt.type;
        o.textContent = opt.label;
        typeSel.appendChild(o);
      });
      typeSel.value = getClusterDeskType();
      typeSel.addEventListener('change', function () {
        setClusterDeskType(typeSel.value);
        refreshClusterPreviews();
      });
      typeRow.appendChild(typeLbl);
      typeRow.appendChild(typeSel);
      inner.appendChild(typeRow);
    }

    var grid = document.createElement('div');
    grid.className = 'editsidebar_grid';
    sec.items.forEach(function (item) {
      var tile = document.createElement('div');
      tile.className = 'editsidebar_tile';
      tile.setAttribute('data-type', item.type || ('cluster' + item.count + (item.diagonal ? 'd' : '')));
      tile.setAttribute('title', item.label + ' \u2014 ' + item.desc);

      var icon = document.createElement('div');
      icon.className = 'editsidebar_tile_icon';
      if (item.cluster) {
        icon.classList.add('editsidebar_tile_icon_cluster');
        renderClusterPreviewInto(icon, item);
      } else {
        icon.setAttribute('style', editPaletteIconStyle(item));
      }
      tile.appendChild(icon);

      var name = document.createElement('div');
      name.className = 'editsidebar_tile_name';
      name.textContent = item.label;
      tile.appendChild(name);

      var desc = document.createElement('div');
      desc.className = 'editsidebar_tile_desc';
      desc.textContent = item.desc;
      tile.appendChild(desc);

      // Start a drag-to-place gesture on pointer down.
      tile.addEventListener('pointerdown', function (ev) {
        ev.preventDefault();
        startPaletteDrag(item, ev);
      });

      grid.appendChild(tile);
    });
    inner.appendChild(grid);
  });

  renderCustomPaletteSection(inner);
}

// Append a "Custom items" palette section built from the admin-defined custom
// item types. Drag-to-place creates a marker immediately (no editor form).
function renderCustomPaletteSection(inner) {
  var items = customPaletteItems();
  if (!items.length) { return; }

  var h = document.createElement('div');
  h.className = 'editsidebar_section';
  h.textContent = 'Custom items';
  inner.appendChild(h);

  var grid = document.createElement('div');
  grid.className = 'editsidebar_grid';
  items.forEach(function (item) {
    var tile = document.createElement('div');
    tile.className = 'editsidebar_tile';
    tile.setAttribute('data-type', item.type);
    tile.setAttribute('title', item.label + ' \u2014 ' + item.desc);

    var icon = document.createElement('div');
    icon.className = 'editsidebar_tile_icon';
    icon.setAttribute('style', editPaletteIconStyle(item));
    tile.appendChild(icon);

    var name = document.createElement('div');
    name.className = 'editsidebar_tile_name';
    name.textContent = item.label;
    tile.appendChild(name);

    var desc = document.createElement('div');
    desc.className = 'editsidebar_tile_desc';
    desc.textContent = item.desc;
    tile.appendChild(desc);

    tile.addEventListener('pointerdown', function (ev) {
      ev.preventDefault();
      startPaletteDrag(item, ev);
    });

    grid.appendChild(tile);
  });
  inner.appendChild(grid);
}

function openEditSidebar() {
  if (typeof map !== 'undefined' && map === 'overview') { return; }
  var sb = document.getElementById('editsidebar');
  if (!sb) { return; }
  if (!sb.getAttribute('data-built')) {
    renderEditPalette();
    sb.setAttribute('data-built', '1');
  }
  if (editSidebarWidth === EDIT_SIDEBAR_WIDTH) { return; }
  editSidebarWidth = EDIT_SIDEBAR_WIDTH;
  sb.classList.add('open');
  if (typeof window.cmapsRescale === 'function') { window.cmapsRescale(); }
}

function closeEditSidebar() {
  var sb = document.getElementById('editsidebar');
  if (sb) { sb.classList.remove('open'); }
  if (typeof closeSidebarForm === 'function') { closeSidebarForm(); }
  if (editSidebarWidth === 0) { return; }
  editSidebarWidth = 0;
  if (typeof window.cmapsRescale === 'function') { window.cmapsRescale(); }
}

// Show the palette only while editing a detail map (not on the overview).
// Called from applyUsermodeUI (user.js) and on initial load.
function applyEditSidebar() {
  if (setting_usermode === 'edit' && (typeof map === 'undefined' || map !== 'overview')) {
    openEditSidebar();
  } else {
    closeEditSidebar();
  }
}

$(function () {
  // Open the palette on load if the page starts in edit mode. Runs after
  // resize.js has installed window.cmapsRescale (admin.js loads later).
  applyEditSidebar();
  // Escape cancels an active draft / closes the sidebar editor.
  document.addEventListener('keydown', function (ev) {
    if (ev.key === 'Escape' && (draftState || selectedDeskId)) {
      if (typeof hideSticky === 'function') { hideSticky(); }
    }
  });
});

// ---------------------------------------------------------------------------
// Drag a palette item onto the map (pointer-based; zoom-correct on drop)
// ---------------------------------------------------------------------------
// We deliberately do NOT use the HTML5 drag-and-drop API: the map uses a
// fractional CSS `zoom` on #content plus a per-item `zoom:itemscale`, which make
// native drop coordinates and ghost images unreliable. Instead a floating ghost
// follows the cursor in plain screen space, and the screen->map conversion runs
// once on drop via screenToMap().

var paletteDrag = null; // { item, ghost } while a drag is in progress

// Convert a viewport (clientX/clientY) point to the map's internal coordinate
// space (pre-zoom, 1600px-wide). A desk stored at (x,y) renders with its centre
// exactly at internal (x,y), so the returned point is what we store. Reading the
// live bounding rect + computed zoom of #content makes this correct at any
// autozoom/manual-zoom and regardless of whether a sidebar is open.
function screenToMap(clientX, clientY) {
  var content = document.getElementById('content');
  if (!content) { return null; }
  var rect = content.getBoundingClientRect();
  var z = parseFloat(window.getComputedStyle(content).zoom) || 1;
  return {
    x: Math.round((clientX - rect.left) / z),
    y: Math.round((clientY - rect.top) / z)
  };
}

// True when a viewport point lies over the editable map content (not over the
// header, the sidebars or outside the window).
function pointOverMap(clientX, clientY) {
  var content = document.getElementById('content');
  if (!content) { return false; }
  var el = document.elementFromPoint(clientX, clientY);
  return !!(el && content.contains(el));
}

// True when a viewport point lies over the trash drop zone at the bottom of the
// edit sidebar (only meaningful while the sidebar is open).
function pointOverTrash(clientX, clientY) {
  if (editSidebarWidth === 0) { return false; }
  var trash = document.getElementById('editsidebar_trash');
  if (!trash) { return false; }
  var r = trash.getBoundingClientRect();
  return clientX >= r.left && clientX <= r.right && clientY >= r.top && clientY <= r.bottom;
}

// Delete a map item (desk/floor/poi) by id. Used by the trash drop zone.
function deleteDeskById(id) {
  $.ajax({
    url: 'rest/update',
    async: true,
    type: 'get',
    data: { token: token, mode: 'delete', map: mapname, id: id, user: username },
    dataType: 'JSON',
    success: function () { updateDesks(); checkHealthStatus(); },
    error: function () { alert('Could not delete item.'); }
  });
}

function startPaletteDrag(item, ev) {
  // Cancel any half-finished drag first.
  endPaletteDrag();

  // Clusters use a dedicated multi-dot ghost; single items use a tinted swatch.
  if (item.cluster) {
    startClusterDrag(item, ev);
    return;
  }

  var ghost = document.createElement('div');
  ghost.className = 'editsidebar_dragghost';
  ghost.setAttribute('style', editPaletteIconStyle(item));
  // Size the ghost to the item's real on-screen size so the cursor preview
  // matches what will be placed: diameter = 2*halfsize * itemscale * contentZoom.
  var content = document.getElementById('content');
  var z = content ? (parseFloat(window.getComputedStyle(content).zoom) || 1) : 1;
  var scale = parseFloat(typeof itemscale !== 'undefined' ? itemscale : 1) || 1;
  if (item.type === 'floor') {
    // Floor markers snap to the rail; preview as a small tab and reveal the rail.
    ghost.style.width = (60 * scale * z) + 'px';
    ghost.style.height = (2 * editItemHalfsize('floor') * scale * z) + 'px';
    ghost.style.borderRadius = '5px';
    if (typeof showFloorRailForDrag === 'function') { showFloorRailForDrag(); }
  } else {
    var size = 2 * editItemHalfsize(item.type) * scale * z;
    ghost.style.width = size + 'px';
    ghost.style.height = size + 'px';
  }
  ghost.style.left = ev.clientX + 'px';
  ghost.style.top = ev.clientY + 'px';
  document.body.appendChild(ghost);

  paletteDrag = { item: item, ghost: ghost };
  document.addEventListener('pointermove', onPaletteDragMove, true);
  document.addEventListener('pointerup', onPaletteDragUp, true);
}

// Screen X of the floor rail (so the floor drag ghost can snap to it).
function floorRailScreenX() {
  var rail = document.getElementById('floorrail');
  if (rail) {
    var r = rail.getBoundingClientRect();
    return r.left + r.width / 2;
  }
  // Fall back to converting the rail's content-space X to screen space.
  var content = document.getElementById('content');
  if (!content) { return null; }
  var cr = content.getBoundingClientRect();
  var z = parseFloat(window.getComputedStyle(content).zoom) || 1;
  return cr.left + FLOOR_RAIL_X * z;
}

function onPaletteDragMove(ev) {
  if (!paletteDrag) { return; }
  var clientX = ev.clientX;
  if (paletteDrag.item.type === 'floor') {
    // Lock the floor ghost horizontally onto the rail; only Y tracks the cursor.
    var railX = floorRailScreenX();
    if (railX !== null) { clientX = railX; }
  }
  paletteDrag.ghost.style.left = clientX + 'px';
  paletteDrag.ghost.style.top = ev.clientY + 'px';
  // Dim the ghost when it is not over a valid drop area.
  paletteDrag.ghost.style.opacity = pointOverMap(ev.clientX, ev.clientY) ? '0.95' : '0.4';
}

function onPaletteDragUp(ev) {
  if (!paletteDrag) { return; }
  var item = paletteDrag.item;
  var over = pointOverMap(ev.clientX, ev.clientY);
  var pt = over ? screenToMap(ev.clientX, ev.clientY) : null;
  endPaletteDrag();
  if (over && pt) {
    if (item.cluster) {
      placeCluster(item, pt.x, pt.y);
      return;
    }
    if (item.custom) {
      placeCustomItem(item, pt.x, pt.y);
      return;
    }
    // Floor markers ignore the dropped X and lock to the rail.
    var px = (item.type === 'floor') ? FLOOR_RAIL_X : pt.x;
    placeItem(item.type, px, pt.y);
  }
}

function endPaletteDrag() {
  document.removeEventListener('pointermove', onPaletteDragMove, true);
  document.removeEventListener('pointerup', onPaletteDragUp, true);
  if (paletteDrag && paletteDrag.ghost && paletteDrag.ghost.parentNode) {
    paletteDrag.ghost.parentNode.removeChild(paletteDrag.ghost);
  }
  if (typeof endFloorRailDrag === 'function') { endFloorRailDrag(); }
  paletteDrag = null;
}

// Migrate any pre-existing floor records whose X is not the rail X to the rail
// (their X used to be free; floors are now rail-locked). Runs only for editors,
// once per id; after each successful update the next poll sees the corrected X.
var floorMigrating = {};
function migrateFloorsToRail() {
  if (typeof token === 'undefined') { return; }
  if (!result_old || !result_old.desks) { return; }
  result_old.desks.forEach(function (d) {
    if (d.desktype !== 'floor') { return; }
    if (parseInt(d.x, 10) === FLOOR_RAIL_X) { return; }
    if (floorMigrating[d.id]) { return; }
    floorMigrating[d.id] = true;
    $.ajax({
      url: 'rest/update',
      type: 'get',
      data: { token: token, mode: 'update', map: mapname, id: d.id, desktype: 'floor',
        x: FLOOR_RAIL_X, y: d.y, desknumber: 'Floor', employee: d.empl,
        avatar: (d.avtr || '-'), department: '- none -', user: username },
      dataType: 'JSON',
      success: function () { delete floorMigrating[d.id]; },
      error: function () { delete floorMigrating[d.id]; }
    });
  });
}

// Drop a new item onto the map: instead of the old "fill form then place"
// flow, we spawn a live DRAFT marker at (x,y) that can be dragged into position
// while the property form is shown in the sidebar. Nothing is written to the DB
// until the editor clicks Save (Back/Escape discards the draft).
function placeItem(type, x, y) {
  startDraftPlacement(type, x, y);
}

// ---------------------------------------------------------------------------
// Sidebar property form: add (draft) + edit, shown in #editsidebar_form in
// place of the palette. Coordinates are two-way bound to a preview on the map.
// ---------------------------------------------------------------------------

var draftState = null;     // { type, x, y } while placing a new item
var selectedDeskId = null; // id of the desk currently open in the sidebar editor

// The desk-type <select> used by both the add and edit forms. `onchange` differs
// (new drafts re-read the DOM via override 3; edits re-read the stored desk).
function deskTypeSelectHtml(onchange) {
  return '<select id="selDesktype" class="sbinput" onchange="' + onchange + '">'
    + '<option value="ldap-desk">LDAP synced Desk</option>'
    + '<option value="blocked">Blocked</option>'
    + '<option value="exit">Exit</option>'
    + '<option value="firstaid">First Aid</option>'
    + '<option value="floor">Floor</option>'
    + '<option value="food">Food</option>'
    + '<option value="booking">Booking</option>'
    + '<option value="hotseat">Hotseat</option>'
    + '<option value="keycardlock">Keycard Lock</option>'
    + '<option value="keylock">Key Lock</option>'
    + '<option value="meeting">Meeting</option>'
    + '<option value="printer">Printer</option>'
    + '<option value="restroom">Restroom</option>'
    + '<option value="service">Service</option>'
    + '<option value="local-desk">Non-LDAP Desk</option>'
    + '</select>';
}

// Render the sidebar form shell (header + type select + #inputfields + actions)
// and switch the sidebar from the palette view to the form view.
function buildSidebarForm(opts) {
  var host = document.getElementById('editsidebar_form');
  var inner = document.getElementById('editsidebar_inner');
  var footer = document.getElementById('editsidebar_footer');
  if (!host) { return; }
  var deleteForm = '';
  if (opts.mode === 'edit') {
    deleteForm = '<form class="sidebarDelete">'
               + '<input type="hidden" id="apideskid" name="apideskid" value="' + opts.deskid + '">'
               + '<input type="submit" class="sbbtn sbbtn_delete" value="Delete item">'
               + '</form>';
  }
  host.innerHTML =
      '<div class="editsidebar_form_head">'
    + '<button type="button" class="editsidebar_back" onclick="hideSticky()">&#8592; Back</button>'
    + '<span class="editsidebar_form_title">' + opts.title + '</span>'
    + '</div>'
    + '<form class="sidebarItem">'
    + '<div class="sbrow"><div class="sblabel">Type</div>' + opts.typeSelect + '</div>'
    + '<div id="inputfields"></div>'
    + '<div class="editsidebar_form_actions">'
    + '<input type="submit" class="sbbtn sbbtn_save" value="Save">'
    + '</div>'
    + '</form>'
    + deleteForm;
  if (inner) { inner.style.display = 'none'; }
  if (footer) { footer.style.display = 'none'; }
  host.style.display = 'flex';
}

// Restore the palette view and clear any active draft/selection.
function closeSidebarForm() {
  clearDraft();
  if (selectedDeskId) { highlightSelected(selectedDeskId, false); selectedDeskId = null; }
  var host = document.getElementById('editsidebar_form');
  var inner = document.getElementById('editsidebar_inner');
  var footer = document.getElementById('editsidebar_footer');
  if (host) { host.style.display = 'none'; host.innerHTML = ''; }
  if (inner) { inner.style.display = ''; }
  if (footer) { footer.style.display = ''; }
}

function clearDraft() {
  var m = document.getElementById('draftitem');
  if (m && m.parentNode) { m.parentNode.removeChild(m); }
  draftState = null;
}

function highlightSelected(id, on) {
  var el = document.getElementById(id);
  if (!el) { return; }
  if (on) { el.classList.add('deskselected'); }
  else { el.classList.remove('deskselected'); }
}

// --- New item (draft) -------------------------------------------------------

function startDraftPlacement(type, x, y) {
  hideSticky(); // clear any nameplate/overlay + previous draft/form
  if (type === 'floor') { x = FLOOR_RAIL_X; }
  draftState = { type: type, x: x, y: y };

  // Spawn the draggable draft marker in content space (matches how real desks
  // are positioned: centre at (x,y), diameter = 2*halfsize*itemscale).
  var content = document.getElementById('content');
  var marker = document.createElement('div');
  marker.id = 'draftitem';
  marker.style.position = 'absolute';
  marker.style.zIndex = '95';
  marker.style.cursor = 'move';
  content.appendChild(marker);
  restyleDraftMarker(type);
  attachDraftDrag(marker, type);

  // Build the create form and render the type-specific fields with the drop coords.
  buildSidebarForm({ mode: 'create', title: 'New item', typeSelect: deskTypeSelectHtml('onDraftTypeChange()') });
  $("#selDesktype").val(type);
  addInputfields(0, type, 2, { id: 'NULL', map: mapname, x: x, y: y, dsk: '', empl: '', avtr: '', dept: '' });
  bindCoordPreview();
  wireSidebarCreateSubmit();
  openEditSidebar();
}

// Re-render the fields when the editor changes a draft's type, keeping the
// coordinates typed so far and re-styling the preview marker.
function onDraftTypeChange() {
  var t = $("#selDesktype").val();
  if (draftState) { draftState.type = t; }
  addInputfields(0, 'newdesk', 3); // override 3 reads current #apideskx/#apidesky + #selDesktype
  restyleDraftMarker(t);
  bindCoordPreview();
}

// Size/tint the draft marker to the item type (WYSIWYG preview) and place it at
// the current draft centre.
function restyleDraftMarker(type) {
  var marker = document.getElementById('draftitem');
  if (!marker || !draftState) { return; }
  var scale = parseFloat(typeof itemscale !== 'undefined' ? itemscale : 1) || 1;
  var size = 2 * editItemHalfsize(type) * scale;
  var pitem = EDIT_PALETTE_BY_TYPE[type];
  var cx = draftState.x, cy = draftState.y;
  if (type === 'floor') { cx = FLOOR_RAIL_X; draftState.x = cx; }
  marker.style.width = size + 'px';
  marker.style.height = size + 'px';
  marker.style.left = (cx - size / 2) + 'px';
  marker.style.top = (cy - size / 2) + 'px';
  marker.style.backgroundImage = '';
  if (pitem) {
    marker.style.backgroundColor = pitem.color;
    if (pitem.icon) {
      marker.style.backgroundImage = 'url("images/' + pitem.icon + '")';
      marker.style.backgroundSize = 'cover';
    }
    marker.style.borderRadius = pitem.square ? '3px' : '50%';
  } else {
    marker.style.backgroundColor = 'rgba(74,163,255,0.85)';
    marker.style.borderRadius = '50%';
  }
}

// Dedicated drag handler for the draft marker (kept separate from dragElement so
// it never persists to the DB - it only moves the preview + syncs the coords).
function attachDraftDrag(marker, type) {
  marker.onmousedown = function (e) {
    e = e || window.event;
    e.preventDefault();
    var startLeft = parseFloat(marker.style.left) || 0;
    var startTop = parseFloat(marker.style.top) || 0;
    var startX = e.clientX, startY = e.clientY;
    var pageScale = parseFloat(window.getComputedStyle(document.getElementById('content')).zoom) || 1;
    document.onmousemove = function (ev) {
      ev = ev || window.event;
      ev.preventDefault();
      var dx = (ev.clientX - startX) / pageScale;
      var dy = (ev.clientY - startY) / pageScale;
      var newLeft = (type === 'floor') ? startLeft : (startLeft + dx);
      var newTop = startTop + dy;
      marker.style.left = newLeft + 'px';
      marker.style.top = newTop + 'px';
      var half = (parseFloat(marker.style.width) || 0) / 2;
      var cx = (type === 'floor') ? FLOOR_RAIL_X : Math.round(newLeft + half);
      var cy = Math.round(newTop + half);
      if (draftState) { draftState.x = cx; draftState.y = cy; }
      var xf = document.getElementById('apideskx'); if (xf) { xf.value = cx; }
      var yf = document.getElementById('apidesky'); if (yf) { yf.value = cy; }
    };
    document.onmouseup = function () {
      document.onmousemove = null;
      document.onmouseup = null;
    };
  };
}

// Typing into the x/y fields moves the draft preview marker.
function bindCoordPreview() {
  var xf = document.getElementById('apideskx');
  var yf = document.getElementById('apidesky');
  var marker = document.getElementById('draftitem');
  if (!marker) { return; }
  function apply() {
    var w = parseFloat(marker.style.width) || 0;
    var h = parseFloat(marker.style.height) || 0;
    var cx = xf ? parseInt(xf.value, 10) : NaN;
    var cy = yf ? parseInt(yf.value, 10) : NaN;
    if (draftState && draftState.type === 'floor') { cx = FLOOR_RAIL_X; }
    if (!isFinite(cx) && draftState) { cx = draftState.x; }
    if (!isFinite(cy) && draftState) { cy = draftState.y; }
    if (draftState) { draftState.x = cx; draftState.y = cy; }
    marker.style.left = (cx - w / 2) + 'px';
    marker.style.top = (cy - h / 2) + 'px';
  }
  if (xf) { xf.addEventListener('input', apply); }
  if (yf) { yf.addEventListener('input', apply); }
}

function wireSidebarCreateSubmit() {
  $('.sidebarItem').off('submit').on('submit', function (e) {
    e.preventDefault();
    var itemdesktype = $("#apidesktype").val();
    var itemx = $("#apideskx").val();
    var itemy = $("#apidesky").val();
    var itemdsk = $("#apideskdsk").val();
    var itemempl = $("#apideskempl").val();
    var itemavtr = $("#apideskavtr").val();
    var itemdept = $("#apideskdept").val();
    if (itemdept == "- none -" || itemdept == "" || typeof itemdept === 'undefined') { itemdept = 'NULL'; }
    if (itemavtr == "" || typeof itemavtr === 'undefined') { itemavtr = 'NULL'; }
    $.ajax({
      url: 'rest/update',
      async: true,
      type: 'get',
      data: { token: token, mode: 'create', map: mapname, id: 'new', desktype: itemdesktype, x: itemx, y: itemy, desknumber: itemdsk, employee: itemempl, avatar: itemavtr, department: itemdept, user: username },
      dataType: 'JSON',
      success: function () { hideSticky(); updateDesks(); checkHealthStatus(); },
      error: function () { alert('Could not create item. Please check if all attributes have been entered.'); }
    });
  });
}

// --- Edit existing item -----------------------------------------------------

// Open the sidebar editor for an existing desk. Called from showSticky (user.js)
// in edit mode, so the read-only nameplate stays on the map for reference.
function openSidebarEdit(deskid, desktype) {
  clearDraft();
  if (selectedDeskId && selectedDeskId !== deskid) { highlightSelected(selectedDeskId, false); }
  selectedDeskId = deskid;
  var onchange = "addInputfields('" + deskid + "','" + desktype + "')";
  buildSidebarForm({ mode: 'edit', title: 'Edit item', typeSelect: deskTypeSelectHtml(onchange), deskid: deskid });
  addInputfields(deskid, desktype, 1); // override 1: pick the mapped type + set the dropdown
  bindEditCoordPreview(deskid);
  wireSidebarEditSubmit(deskid);
  highlightSelected(deskid, true);
  openEditSidebar();
}

// Typing into x/y previews the move by repositioning the real desk element; the
// change is only persisted on Save (a redraw restores it otherwise).
function bindEditCoordPreview(deskid) {
  var el = document.getElementById(deskid);
  var xf = document.getElementById('apideskx');
  var yf = document.getElementById('apidesky');
  if (!el) { return; }
  var scale = parseFloat(typeof itemscale !== 'undefined' ? itemscale : 1) || 1;
  function apply() {
    var half = (parseFloat(el.style.width) || 0) / 2;
    var cx = xf ? parseInt(xf.value, 10) : NaN;
    var cy = yf ? parseInt(yf.value, 10) : NaN;
    if (isFinite(cx)) { el.style.left = (cx / scale - half) + 'px'; }
    if (isFinite(cy)) { el.style.top = (cy / scale - half) + 'px'; }
  }
  if (xf) { xf.addEventListener('input', apply); }
  if (yf) { yf.addEventListener('input', apply); }
}

function wireSidebarEditSubmit(deskid) {
  $('.sidebarItem').off('submit').on('submit', function (e) {
    e.preventDefault();
    var itemid = $("#apideskid").val() || deskid;
    var itemdesktype = $("#apidesktype").val();
    var itemx = $("#apideskx").val();
    var itemy = $("#apidesky").val();
    var itemdsk = $("#apideskdsk").val();
    var itemempl = $("#apideskempl").val();
    var itemavtr = $("#apideskavtr").val();
    var itemdept = $("#apideskdept").val();
    if (itemdept == "- none -") { itemdept = 'NULL'; }
    $.ajax({
      url: 'rest/update',
      async: true,
      type: 'get',
      data: { token: token, mode: 'update', map: mapname, id: itemid, desktype: itemdesktype, x: itemx, y: itemy, desknumber: itemdsk, employee: itemempl, avatar: itemavtr, department: itemdept, user: username },
      dataType: 'JSON',
      success: function () { hideSticky(); updateDesks(); checkHealthStatus(); },
      error: function () { alert('Could not update item'); }
    });
  });
  $('.sidebarDelete').off('submit').on('submit', function (e) {
    e.preventDefault();
    var itemid = $("#apideskid").val() || deskid;
    $.ajax({
      url: 'rest/update',
      async: true,
      type: 'get',
      data: { token: token, mode: 'delete', map: mapname, id: itemid, user: username },
      dataType: 'JSON',
      success: function () { hideSticky(); updateDesks(); checkHealthStatus(); },
      error: function () { alert('Could not delete item'); }
    });
  });
}


// ---------------------------------------------------------------------------
// Cluster placement: drop a pre-aligned block of several desks at once. The
// cluster is purely a placement TEMPLATE (no schema change) - it creates N
// INDEPENDENT custom-desk records that can each be moved/edited afterwards.
// ---------------------------------------------------------------------------

// Centre-to-centre spacing (in the map's 1600px content space) between desks in
// a cluster. When the map already has desk groups, match their typical spacing
// so a new cluster blends in; otherwise fall back to a fixed itemscale-based gap.
function clusterSpacing() {
  var scale = parseFloat(typeof itemscale !== 'undefined' ? itemscale : 1) || 1;
  var learned = existingDeskSpacing();
  if (learned) { return learned; }
  return 50 * scale;
}

// Learn the usual centre-to-centre spacing of existing desk groups on the map
// (median nearest-neighbour distance among round, free-placed desks, in content
// space). Returns null when there aren't enough desks to infer a spacing, so the
// caller keeps its default. Clamped to a sane range to avoid odd outliers.
function existingDeskSpacing() {
  var src = (result_old && result_old.desks) ? result_old.desks : [];
  var desks = src
    .filter(function (d) { return AUTOALIGN_TYPES[d.desktype]; })
    .map(function (d) { return { x: parseInt(d.x, 10), y: parseInt(d.y, 10) }; })
    .filter(function (d) { return isFinite(d.x) && isFinite(d.y); });
  if (desks.length < 4) { return null; }
  var dists = [];
  desks.forEach(function (d) {
    var best = Infinity;
    desks.forEach(function (e) {
      if (e === d) { return; }
      var dd = Math.hypot(d.x - e.x, d.y - e.y);
      if (dd < best) { best = dd; }
    });
    if (isFinite(best) && best > 0) { dists.push(best); }
  });
  if (dists.length < 3) { return null; }
  dists.sort(function (a, b) { return a - b; });
  var med = dists[Math.floor(dists.length / 2)];
  if (!isFinite(med) || med <= 0) { return null; }
  return Math.max(30, Math.min(200, med));
}

// Relative (dx,dy) offsets of each desk in a cluster around the drop point, in
// content space. Straight clusters are an even grid; diagonal clusters are the
// same grid rotated 45deg (centres only - desks stay circles, no rotation).
function clusterOffsets(item) {
  var spacing = clusterSpacing();
  var cols = item.cols, rows = item.rows;
  var offs = [];
  for (var r = 0; r < rows; r++) {
    for (var c = 0; c < cols; c++) {
      offs.push({
        dx: (c - (cols - 1) / 2) * spacing,
        dy: (r - (rows - 1) / 2) * spacing
      });
    }
  }
  if (item.diagonal) {
    var k = Math.SQRT1_2; // cos45 == sin45
    offs = offs.map(function (o) {
      return { dx: o.dx * k - o.dy * k, dy: o.dx * k + o.dy * k };
    });
  }
  return offs;
}

// Render a miniature multi-dot preview of a cluster into a tile/ghost element.
function renderClusterPreviewInto(el, item) {
  el.innerHTML = '';
  var offs = clusterOffsets(item);
  var maxAbs = 1;
  offs.forEach(function (o) {
    maxAbs = Math.max(maxAbs, Math.abs(o.dx), Math.abs(o.dy));
  });
  var box = 44, dot = 9, fit = (box / 2 - dot / 2 - 2) / maxAbs;
  var color = clusterDeskColor();
  offs.forEach(function (o) {
    var d = document.createElement('span');
    d.className = 'editsidebar_clusterdot';
    d.style.background = color;
    d.style.width = dot + 'px';
    d.style.height = dot + 'px';
    d.style.left = (box / 2 + o.dx * fit - dot / 2) + 'px';
    d.style.top = (box / 2 + o.dy * fit - dot / 2) + 'px';
    el.appendChild(d);
  });
}

// Start dragging a cluster: a floating multi-dot ghost (sized to the real
// on-screen footprint) follows the cursor; on drop placeCluster() runs.
function startClusterDrag(item, ev) {
  var content = document.getElementById('content');
  var z = content ? (parseFloat(window.getComputedStyle(content).zoom) || 1) : 1;
  var scale = parseFloat(typeof itemscale !== 'undefined' ? itemscale : 1) || 1;
  var offs = clusterOffsets(item);
  var dotSize = 2 * editItemHalfsize('local-desk') * scale * z; // 20*scale*z
  var color = clusterDeskColor();

  var ghost = document.createElement('div');
  ghost.className = 'editsidebar_dragghost editsidebar_dragghost_cluster';
  offs.forEach(function (o) {
    var d = document.createElement('div');
    d.style.position = 'absolute';
    d.style.width = dotSize + 'px';
    d.style.height = dotSize + 'px';
    d.style.borderRadius = '50%';
    d.style.background = color;
    // Centre the cluster on the cursor: container origin sits at the cursor and
    // each dot is offset around it (content offset * live content zoom).
    d.style.left = (o.dx * z - dotSize / 2) + 'px';
    d.style.top = (o.dy * z - dotSize / 2) + 'px';
    ghost.appendChild(d);
  });
  ghost.style.left = ev.clientX + 'px';
  ghost.style.top = ev.clientY + 'px';
  document.body.appendChild(ghost);

  paletteDrag = { item: item, ghost: ghost };
  document.addEventListener('pointermove', onPaletteDragMove, true);
  document.addEventListener('pointerup', onPaletteDragUp, true);
}

// Place a cluster: create N independent custom desks in one batch round-trip.
function placeCluster(item, cx, cy) {
  var offs = clusterOffsets(item);
  var f = clusterDeskFields(getClusterDeskType());
  var ops = offs.map(function (o) {
    return {
      op: 'create', desktype: f.desktype,
      x: Math.round(cx + o.dx), y: Math.round(cy + o.dy),
      desknumber: 'Desk', employee: f.employee, avatar: f.avatar, department: f.department
    };
  });
  $.ajax({
    url: 'rest/update',
    type: 'post',
    data: { token: token, mode: 'batch', map: mapname, user: username, ops: JSON.stringify(ops) },
    dataType: 'JSON',
    success: function () { updateDesks(); checkHealthStatus(); },
    error: function () { alert('Could not place the desk cluster.'); }
  });
}

// Drop-to-place a custom item marker: creates one desk record with desktype
// "custom_<id>" and the type's label as its name. No editor form is shown;
// editors can move it by dragging or remove it via the trash zone afterwards.
function placeCustomItem(item, x, y) {
  $.ajax({
    url: 'rest/update',
    type: 'get',
    data: {
      token: token, mode: 'create', map: mapname, user: username,
      desktype: item.type, x: Math.round(x), y: Math.round(y),
      desknumber: item.label, employee: '', avatar: '-', department: '- none -'
    },
    dataType: 'JSON',
    success: function () { updateDesks(); checkHealthStatus(); },
    error: function () { alert('Could not place the custom item.'); }
  });
}

// ---------------------------------------------------------------------------
// Auto-align: tidy nearby desks into shared row/column baselines. Detects
// proximity clusters, snaps each desk to its row-band average Y and column-band
// average X, previews the result (ghosts), and saves on confirm via mode=batch.
// ---------------------------------------------------------------------------

// Display desktypes (as returned by /rest/desks) that are round, free-placed
// desks eligible for auto-align. Floors (rail-locked), meeting rooms and points
// of interest are intentionally excluded.
var AUTOALIGN_TYPES = {
  occupied: 1, addesk: 1, localdesk: 1, hotseat: 1, booking: 1, blocked: 1, shareddesk: 1
};

// Group items by a coordinate (x or y) into bands whose members are within
// `tol` of the band's anchor (first member). Anchoring (rather than comparing to
// the previous element) caps each band's width at `tol` and prevents a dense run
// of desks from chaining distinct rows/columns into one. Returns a map
// id -> the band's average coordinate (rounded).
function autoAlignBands(items, key, tol) {
  var sorted = items.slice().sort(function (a, b) { return a[key] - b[key]; });
  var bands = [], cur = [], anchor = 0;
  sorted.forEach(function (it) {
    if (cur.length === 0) { cur = [it]; anchor = it[key]; return; }
    if (it[key] - anchor <= tol) { cur.push(it); }
    else { bands.push(cur); cur = [it]; anchor = it[key]; }
  });
  if (cur.length) { bands.push(cur); }
  var out = {};
  bands.forEach(function (band) {
    var sum = 0;
    band.forEach(function (it) { sum += it[key]; });
    var avg = Math.round(sum / band.length);
    band.forEach(function (it) { out[it.id] = avg; });
  });
  return out;
}

// Median nearest-neighbour distance among the desks (in content space). Used to
// adapt the clustering/banding tolerances to whatever spacing the map actually
// uses, instead of a fixed guess that over-merges dense floors.
function autoAlignMedianSpacing(desks) {
  if (desks.length < 2) { return 60; }
  var dists = [];
  desks.forEach(function (d) {
    var best = Infinity;
    desks.forEach(function (e) {
      if (e === d) { return; }
      var dd = Math.hypot(d.x - e.x, d.y - e.y);
      if (dd < best) { best = dd; }
    });
    if (isFinite(best)) { dists.push(best); }
  });
  if (!dists.length) { return 60; }
  dists.sort(function (a, b) { return a - b; });
  var med = dists[Math.floor(dists.length / 2)] || 60;
  return Math.max(30, Math.min(200, med));
}

// Estimate the dominant grid orientation of a desk set (in radians). Desks are
// often laid out on a grid that is rotated to follow an angled wall or aisle, so
// snapping to horizontal/vertical would fight the layout. We take each desk's
// nearest-neighbour direction, fold it into a quarter turn (grid axes are
// perpendicular, so directions 90 deg apart are equivalent) via a 4x circular
// mean, and return the rotation that makes the grid axis-aligned. Near-zero
// results snap to 0 so ordinary axis-aligned grids are left exactly as before.
function autoAlignDominantAngle(desks) {
  if (desks.length < 2) { return 0; }
  var sumSin = 0, sumCos = 0, n = 0;
  desks.forEach(function (d) {
    var best = Infinity, bx = 0, by = 0;
    desks.forEach(function (e) {
      if (e === d) { return; }
      var dd = Math.hypot(d.x - e.x, d.y - e.y);
      if (dd > 0 && dd < best) { best = dd; bx = e.x - d.x; by = e.y - d.y; }
    });
    if (!isFinite(best)) { return; }
    var ang = Math.atan2(by, bx);
    sumSin += Math.sin(4 * ang);
    sumCos += Math.cos(4 * ang);
    n++;
  });
  if (n === 0 || (sumSin === 0 && sumCos === 0)) { return 0; }
  var theta = Math.atan2(sumSin, sumCos) / 4; // -45 deg .. 45 deg
  if (Math.abs(theta) < 0.05) { return 0; }   // ~2.9 deg -> treat as axis-aligned
  return theta;
}

// Compute the list of desk moves needed to tidy a set of desks. When `contains`
// (a function taking a desk {id,x,y} in content space and returning a boolean)
// is given, only desks it accepts are considered — this lets the caller restrict
// alignment to a dragged box or a freeform lasso. Returns an array of
// { id, oldX, oldY, newX, newY } for desks that actually move.
function autoAlignPlan(contains) {
  var src = (result_old && result_old.desks) ? result_old.desks : [];
  // Map every desk to a lightweight {id,x,y,raw} probe so the selection
  // predicate can be applied uniformly.
  var allProbes = src.map(function (d) {
    return { id: parseInt(d.id, 10), x: parseInt(d.x, 10), y: parseInt(d.y, 10), raw: d };
  });
  var inSelection = (typeof contains === 'function')
    ? allProbes.filter(contains)
    : allProbes;
  var alignable = inSelection.filter(function (d) { return AUTOALIGN_TYPES[d.raw.desktype]; });
  // Shared desks carry one record per occupant at the same id/position; collapse
  // them to a single entry per desk id so a desk is planned (and moved) only once.
  var seenId = {};
  var desks = alignable.filter(function (d) {
    if (seenId[d.id]) { return false; }
    seenId[d.id] = true;
    return true;
  });
  if (desks.length < 2) { return []; }

  // Adapt thresholds to the map's real spacing: cluster pod-neighbours but not
  // across aisles, and band rows/columns at well under one desk pitch.
  var med = autoAlignMedianSpacing(desks);
  var prox = med * 1.7; // centre-distance for "same cluster"
  var tol = med * 0.45; // row/column banding tolerance

  // Union-find proximity clustering.
  var parent = desks.map(function (_, i) { return i; });
  function find(i) { while (parent[i] !== i) { parent[i] = parent[parent[i]]; i = parent[i]; } return i; }
  function union(a, b) { parent[find(a)] = find(b); }
  for (var i = 0; i < desks.length; i++) {
    for (var j = i + 1; j < desks.length; j++) {
      var dx = desks[i].x - desks[j].x, dy = desks[i].y - desks[j].y;
      if (Math.sqrt(dx * dx + dy * dy) <= prox) { union(i, j); }
    }
  }
  var groups = {};
  desks.forEach(function (d, i) { var r = find(i); (groups[r] = groups[r] || []).push(d); });

  var moves = [];
  // Per-desk canonical row/column targets for all axis-aligned pods, collected so
  // a second pass can line the pods up with each other (shared row/column lines
  // across the whole selection) instead of each pod being tidy only on its own.
  var axisDesks = [];      // {id, x, y}
  var xCanonById = {};     // id -> pod column centre
  var yCanonById = {};     // id -> pod row centre
  Object.keys(groups).forEach(function (k) {
    var g = groups[k];
    if (g.length < 2) { return; } // lone desks are left alone
    // Detect the grid's rotation. Axis-aligned pods feed the cross-pod pass;
    // angled pods are tidied on their own (rotation makes shared lines ambiguous).
    var theta = autoAlignDominantAngle(g);
    if (theta === 0) {
      var pc = autoAlignPodCanon(g, tol);
      g.forEach(function (d) {
        axisDesks.push({ id: d.id, x: d.x, y: d.y });
        xCanonById[d.id] = pc.x[d.id];
        yCanonById[d.id] = pc.y[d.id];
      });
      return;
    }
    // Rotate the cluster by -theta about its centroid, band the axis-aligned
    // coordinates, then rotate each aligned point back into map space.
    var cx = 0, cy = 0;
    g.forEach(function (d) { cx += d.x; cy += d.y; });
    cx /= g.length; cy /= g.length;
    var cos = Math.cos(theta), sin = Math.sin(theta);
    var rot = g.map(function (d) {
      var ox = d.x - cx, oy = d.y - cy;
      return { id: d.id, x: ox * cos + oy * sin, y: -ox * sin + oy * cos, sx: d.x, sy: d.y };
    });
    var colCanonR = autoAlignBands(rot, 'x', tol);
    var rowCanonR = autoAlignBands(rot, 'y', tol);
    rot.forEach(function (d) {
      var rx = colCanonR[d.id], ry = rowCanonR[d.id];
      var nx = Math.round(cx + rx * cos - ry * sin);
      var ny = Math.round(cy + rx * sin + ry * cos);
      if (nx !== d.sx || ny !== d.sy) {
        moves.push({ id: d.id, oldX: d.sx, oldY: d.sy, newX: nx, newY: ny });
      }
    });
  });

  // Cross-pod pass: snap the per-pod row/column centres that fall within one
  // banding tolerance of each other onto a single shared line. Pods spaced
  // farther apart than `tol` keep their own lines, so distinct rows/columns and
  // pods at genuinely different heights are never merged.
  var xSnap = autoAlignUnifyCanon(xCanonById, tol);
  var ySnap = autoAlignUnifyCanon(yCanonById, tol);
  axisDesks.forEach(function (d) {
    var nx = xSnap[d.id], ny = ySnap[d.id];
    if (nx !== d.x || ny !== d.y) {
      moves.push({ id: d.id, oldX: d.x, oldY: d.y, newX: nx, newY: ny });
    }
  });
  return moves;
}

// Compute per-desk canonical column/row coordinates for one axis-aligned pod.
// A desk that sits alone on both a column band and a row band while being flanked
// by real lines on every side (e.g. a 5th desk centred inside a 2x2 pod) is
// placed at the exact midpoint of its neighbouring lines rather than left on its
// own slightly-off position, so it stays perfectly centred.
function autoAlignPodCanon(g, tol) {
  var colCanon = autoAlignBands(g, 'x', tol);
  var rowCanon = autoAlignBands(g, 'y', tol);
  var colMembers = {}, rowMembers = {};
  g.forEach(function (d) {
    (colMembers[colCanon[d.id]] = colMembers[colCanon[d.id]] || []).push(d.id);
    (rowMembers[rowCanon[d.id]] = rowMembers[rowCanon[d.id]] || []).push(d.id);
  });
  var colLines = Object.keys(colMembers).map(Number).sort(function (a, b) { return a - b; });
  var rowLines = Object.keys(rowMembers).map(Number).sort(function (a, b) { return a - b; });
  function midpoint(lines, v) {
    var below = -Infinity, above = Infinity;
    lines.forEach(function (l) {
      if (l < v && l > below) { below = l; }
      if (l > v && l < above) { above = l; }
    });
    return Math.round((below + above) / 2);
  }
  var xById = {}, yById = {};
  g.forEach(function (d) {
    var cx = colCanon[d.id], cy = rowCanon[d.id];
    var aloneX = colMembers[cx].length === 1;
    var aloneY = rowMembers[cy].length === 1;
    var flankedX = cx > colLines[0] && cx < colLines[colLines.length - 1];
    var flankedY = cy > rowLines[0] && cy < rowLines[rowLines.length - 1];
    if (aloneX && aloneY && flankedX && flankedY) {
      xById[d.id] = midpoint(colLines, cx);
      yById[d.id] = midpoint(rowLines, cy);
    } else {
      xById[d.id] = cx;
      yById[d.id] = cy;
    }
  });
  return { x: xById, y: yById };
}

// Collapse per-pod band centres (an id->value map) onto shared lines: values
// within `tol` of each other are clustered and replaced by the cluster's mean,
// so equivalent rows/columns across pods end up on exactly the same coordinate.
function autoAlignUnifyCanon(canonById, tol) {
  var ids = Object.keys(canonById);
  if (!ids.length) { return {}; }
  var vals = ids.map(function (id) { return canonById[id]; }).sort(function (a, b) { return a - b; });
  var clusters = [], cur = [vals[0]];
  for (var i = 1; i < vals.length; i++) {
    if (vals[i] - cur[0] <= tol) { cur.push(vals[i]); }
    else { clusters.push(cur); cur = [vals[i]]; }
  }
  clusters.push(cur);
  var valToRep = {};
  clusters.forEach(function (c) {
    var sum = 0; c.forEach(function (v) { sum += v; });
    var rep = Math.round(sum / c.length);
    c.forEach(function (v) { valToRep[v] = rep; });
  });
  var out = {};
  ids.forEach(function (id) { out[id] = valToRep[canonById[id]]; });
  return out;
}

// Remove any auto-align preview overlay + confirm bar + in-progress selection.
function cancelAutoAlign() {
  setAutoAlignActive(false);
  endAutoAlignSelection();
  var p = document.getElementById('autoalign_preview');
  if (p && p.parentNode) { p.parentNode.removeChild(p); }
  var bar = document.getElementById('autoalign_bar');
  if (bar && bar.parentNode) { bar.parentNode.removeChild(bar); }
  var sel = document.getElementById('autoalign_select');
  if (sel && sel.parentNode) { sel.parentNode.removeChild(sel); }
  var lasso = document.getElementById('autoalign_lasso');
  if (lasso && lasso.parentNode) { lasso.parentNode.removeChild(lasso); }
}

// Render ghost markers at each move's target position (in the map's zoomed
// content space) plus a Confirm/Cancel bar.
function renderAutoAlignPreview(moves) {
  var content = document.getElementById('content');
  if (!content) { return; }
  var scale = parseFloat(typeof itemscale !== 'undefined' ? itemscale : 1) || 1;
  var half = editItemHalfsize('local-desk'); // 10

  var layer = document.createElement('div');
  layer.id = 'autoalign_preview';
  moves.forEach(function (m) {
    var ghost = document.createElement('div');
    ghost.className = 'autoalign_ghost';
    ghost.style.zoom = scale;
    ghost.style.left = (m.newX / scale - half) + 'px';
    ghost.style.top = (m.newY / scale - half) + 'px';
    ghost.style.width = (2 * half) + 'px';
    ghost.style.height = (2 * half) + 'px';
    layer.appendChild(ghost);
  });
  content.appendChild(layer);

  var bar = document.createElement('div');
  bar.id = 'autoalign_bar';
  var msg = document.createElement('span');
  msg.className = 'autoalign_bar_msg';
  msg.textContent = 'Align ' + moves.length + ' desk' + (moves.length === 1 ? '' : 's') + ' into tidy rows & columns?';
  bar.appendChild(msg);
  var apply = document.createElement('button');
  apply.type = 'button';
  apply.className = 'autoalign_apply';
  apply.textContent = 'Apply';
  apply.addEventListener('click', function () { applyAutoAlign(moves); });
  bar.appendChild(apply);
  var cancel = document.createElement('button');
  cancel.type = 'button';
  cancel.className = 'autoalign_cancel';
  cancel.textContent = 'Cancel';
  cancel.addEventListener('click', cancelAutoAlign);
  bar.appendChild(cancel);
  document.body.appendChild(bar);
}

// Save the computed moves in one batch round-trip, then refresh + clear preview.
function applyAutoAlign(moves) {
  var ops = moves.map(function (m) {
    return { op: 'update', id: m.id, x: m.newX, y: m.newY };
  });
  $.ajax({
    url: 'rest/update',
    type: 'post',
    data: { token: token, mode: 'batch', map: mapname, user: username, ops: JSON.stringify(ops) },
    dataType: 'JSON',
    success: function () { cancelAutoAlign(); updateDesks(); },
    error: function () { alert('Could not save the alignment.'); }
  });
}

// Entry point wired to the sidebar align buttons. Instead of aligning the whole
// map, the editor first marks the desks they want tidied — either by dragging a
// box ('box') or drawing a freeform loop ('lasso') — and only those are aligned
// (then previewed + confirmed).
var autoAlignMode = 'box';

function startAutoAlign(mode) {
  mode = (mode === 'lasso') ? 'lasso' : 'box';
  // Toggle: a second click on the engaged tool disables it (same as Cancel).
  var activeBtn = document.getElementById(mode === 'lasso' ? 'editsidebar_lassobtn' : 'editsidebar_alignbtn');
  if (activeBtn && activeBtn.classList.contains('active')) {
    cancelAutoAlign();
    return;
  }
  cancelAutoAlign();
  autoAlignMode = mode;
  setAutoAlignActive(true);
  beginAutoAlignSelection();
}

// Toggle the blue "glow" state on whichever align button is engaged (selecting
// or previewing). Grey when idle, blue when active.
function setAutoAlignActive(on) {
  var boxBtn = document.getElementById('editsidebar_alignbtn');
  var lassoBtn = document.getElementById('editsidebar_lassobtn');
  if (boxBtn) { boxBtn.classList.toggle('active', !!on && autoAlignMode === 'box'); }
  if (lassoBtn) { lassoBtn.classList.toggle('active', !!on && autoAlignMode === 'lasso'); }
}

// In-progress area-selection drag state (null when not selecting).
var autoAlignSelect = null;

// Enter selection mode for the current tool: box drag or freeform lasso.
function beginAutoAlignSelection() {
  if (autoAlignMode === 'lasso') { beginLassoSelection(); return; }
  endAutoAlignSelection();

  var bar = document.createElement('div');
  bar.id = 'autoalign_bar';
  var msg = document.createElement('span');
  msg.className = 'autoalign_bar_msg';
  msg.textContent = 'Drag a box around the desks you want to align.';
  bar.appendChild(msg);
  var cancel = document.createElement('button');
  cancel.type = 'button';
  cancel.className = 'autoalign_cancel';
  cancel.textContent = 'Cancel';
  cancel.addEventListener('click', cancelAutoAlign);
  bar.appendChild(cancel);
  document.body.appendChild(bar);

  autoAlignSelect = { selecting: true, dragging: false, x0: 0, y0: 0, rect: null };
  document.addEventListener('pointerdown', onAutoAlignSelectDown, true);
}

function onAutoAlignSelectDown(ev) {
  if (!autoAlignSelect || !autoAlignSelect.selecting) { return; }
  // Ignore clicks that are not on the map (e.g. the Cancel button or sidebar).
  if (!pointOverMap(ev.clientX, ev.clientY)) { return; }
  // preventDefault suppresses the compatibility mousedown so desk dragging does
  // not start while we draw the selection box.
  ev.preventDefault();
  ev.stopPropagation();
  autoAlignSelect.dragging = true;
  autoAlignSelect.x0 = ev.clientX;
  autoAlignSelect.y0 = ev.clientY;

  var rect = document.createElement('div');
  rect.id = 'autoalign_select';
  rect.style.left = ev.clientX + 'px';
  rect.style.top = ev.clientY + 'px';
  rect.style.width = '0px';
  rect.style.height = '0px';
  document.body.appendChild(rect);
  autoAlignSelect.rect = rect;

  document.addEventListener('pointermove', onAutoAlignSelectMove, true);
  document.addEventListener('pointerup', onAutoAlignSelectUp, true);
}

function onAutoAlignSelectMove(ev) {
  if (!autoAlignSelect || !autoAlignSelect.dragging) { return; }
  var x = Math.min(ev.clientX, autoAlignSelect.x0);
  var y = Math.min(ev.clientY, autoAlignSelect.y0);
  var w = Math.abs(ev.clientX - autoAlignSelect.x0);
  var h = Math.abs(ev.clientY - autoAlignSelect.y0);
  var r = autoAlignSelect.rect;
  if (!r) { return; }
  r.style.left = x + 'px';
  r.style.top = y + 'px';
  r.style.width = w + 'px';
  r.style.height = h + 'px';
}

function onAutoAlignSelectUp(ev) {
  if (!autoAlignSelect || !autoAlignSelect.dragging) { return; }
  var x0 = autoAlignSelect.x0, y0 = autoAlignSelect.y0;
  var x1 = ev.clientX, y1 = ev.clientY;
  // Convert the box corners to the map's content space.
  var p0 = screenToMap(Math.min(x0, x1), Math.min(y0, y1));
  var p1 = screenToMap(Math.max(x0, x1), Math.max(y0, y1));
  endAutoAlignSelection();
  var bar = document.getElementById('autoalign_bar');
  if (bar && bar.parentNode) { bar.parentNode.removeChild(bar); }

  // Ignore an accidental click / tiny box: re-arm the selection.
  if (!p0 || !p1 || (p1.x - p0.x) < 10 || (p1.y - p0.y) < 10) {
    beginAutoAlignSelection();
    return;
  }

  var bounds = { x1: p0.x, y1: p0.y, x2: p1.x, y2: p1.y };
  var moves = autoAlignPlan(function (d) {
    return d.x >= bounds.x1 && d.x <= bounds.x2 && d.y >= bounds.y1 && d.y <= bounds.y2;
  });
  if (!moves.length) {
    cancelAutoAlign();
    showAutoAlignToast('No desks to align here \u2014 try selecting a desk cluster.',
                       (x0 + x1) / 2, (y0 + y1) / 2);
    return;
  }
  renderAutoAlignPreview(moves);
}

// Show a short-lived message centred on the given viewport point that fades out
// on its own after ~2s, so the editor never has to dismiss a dialog when a
// selection contains nothing to align.
function showAutoAlignToast(text, clientX, clientY) {
  var t = document.createElement('div');
  t.className = 'autoalign_toast';
  t.textContent = text;
  t.style.left = clientX + 'px';
  t.style.top = clientY + 'px';
  document.body.appendChild(t);
  setTimeout(function () { t.classList.add('fadeout'); }, 1400);
  setTimeout(function () { if (t.parentNode) { t.parentNode.removeChild(t); } }, 2100);
}

// Tear down the selection listeners + rectangle (leaves any preview alone).
function endAutoAlignSelection() {
  document.removeEventListener('pointerdown', onAutoAlignSelectDown, true);
  document.removeEventListener('pointermove', onAutoAlignSelectMove, true);
  document.removeEventListener('pointerup', onAutoAlignSelectUp, true);
  if (autoAlignSelect && autoAlignSelect.rect && autoAlignSelect.rect.parentNode) {
    autoAlignSelect.rect.parentNode.removeChild(autoAlignSelect.rect);
  }
  autoAlignSelect = null;
  // Lasso teardown (mirrors the box teardown above).
  document.removeEventListener('pointerdown', onLassoDown, true);
  document.removeEventListener('pointermove', onLassoMove, true);
  document.removeEventListener('pointerup', onLassoUp, true);
  if (autoAlignLasso && autoAlignLasso.svg && autoAlignLasso.svg.parentNode) {
    autoAlignLasso.svg.parentNode.removeChild(autoAlignLasso.svg);
  }
  autoAlignLasso = null;
}

// ---------------------------------------------------------------------------
// Lasso (freeform) selection: draw a loop around the desks to align. The path
// is captured in viewport space (an SVG overlay), converted to a map-space
// polygon on release, and desks whose centres fall inside it are aligned.
// ---------------------------------------------------------------------------
var autoAlignLasso = null; // { drawing, pts:[[x,y]...screen], svg, path }

function beginLassoSelection() {
  endAutoAlignSelection();

  var bar = document.createElement('div');
  bar.id = 'autoalign_bar';
  var msg = document.createElement('span');
  msg.className = 'autoalign_bar_msg';
  msg.textContent = 'Draw a loop around the desks you want to align.';
  bar.appendChild(msg);
  var cancel = document.createElement('button');
  cancel.type = 'button';
  cancel.className = 'autoalign_cancel';
  cancel.textContent = 'Cancel';
  cancel.addEventListener('click', cancelAutoAlign);
  bar.appendChild(cancel);
  document.body.appendChild(bar);

  autoAlignLasso = { drawing: false, pts: [], svg: null, path: null };
  document.addEventListener('pointerdown', onLassoDown, true);
}

function onLassoDown(ev) {
  if (!autoAlignLasso) { return; }
  // Ignore clicks that are not on the map (e.g. the Cancel button or sidebar).
  if (!pointOverMap(ev.clientX, ev.clientY)) { return; }
  // preventDefault suppresses the compatibility mousedown so desk dragging /
  // panning does not start while we draw the loop.
  ev.preventDefault();
  ev.stopPropagation();
  autoAlignLasso.drawing = true;
  autoAlignLasso.pts = [[ev.clientX, ev.clientY]];

  var svgNS = 'http://www.w3.org/2000/svg';
  var svg = document.createElementNS(svgNS, 'svg');
  svg.setAttribute('id', 'autoalign_lasso');
  var path = document.createElementNS(svgNS, 'path');
  path.setAttribute('class', 'autoalign_lasso_path');
  svg.appendChild(path);
  document.body.appendChild(svg);
  autoAlignLasso.svg = svg;
  autoAlignLasso.path = path;
  updateLassoPath();

  document.addEventListener('pointermove', onLassoMove, true);
  document.addEventListener('pointerup', onLassoUp, true);
}

function onLassoMove(ev) {
  if (!autoAlignLasso || !autoAlignLasso.drawing) { return; }
  var pts = autoAlignLasso.pts;
  var last = pts[pts.length - 1];
  // Skip near-duplicate points to keep the path light.
  if (!last || Math.hypot(ev.clientX - last[0], ev.clientY - last[1]) >= 3) {
    pts.push([ev.clientX, ev.clientY]);
    updateLassoPath();
  }
}

function updateLassoPath() {
  if (!autoAlignLasso || !autoAlignLasso.path || !autoAlignLasso.pts.length) { return; }
  var d = 'M' + autoAlignLasso.pts.map(function (p) { return p[0] + ',' + p[1]; }).join(' L') + ' Z';
  autoAlignLasso.path.setAttribute('d', d);
}

function onLassoUp(ev) {
  if (!autoAlignLasso || !autoAlignLasso.drawing) { return; }
  var pts = autoAlignLasso.pts.slice();
  var cxScreen = 0, cyScreen = 0;
  pts.forEach(function (p) { cxScreen += p[0]; cyScreen += p[1]; });
  if (pts.length) { cxScreen /= pts.length; cyScreen /= pts.length; }
  endAutoAlignSelection();
  var bar = document.getElementById('autoalign_bar');
  if (bar && bar.parentNode) { bar.parentNode.removeChild(bar); }

  // Too small a scribble to be a loop: re-arm the lasso.
  if (pts.length < 3) {
    beginLassoSelection();
    return;
  }
  // Convert the screen path into a map-space polygon.
  var poly = [];
  pts.forEach(function (p) {
    var m = screenToMap(p[0], p[1]);
    if (m) { poly.push([m.x, m.y]); }
  });
  if (poly.length < 3) {
    cancelAutoAlign();
    showAutoAlignToast('Couldn\u2019t read that loop \u2014 try again.', cxScreen, cyScreen);
    return;
  }
  var moves = autoAlignPlan(function (d) { return pointInPolygon(d.x, d.y, poly); });
  if (!moves.length) {
    cancelAutoAlign();
    showAutoAlignToast('No desks to align in that loop \u2014 encircle a desk cluster.', cxScreen, cyScreen);
    return;
  }
  renderAutoAlignPreview(moves);
}

// Ray-casting point-in-polygon test. `poly` is an array of [x,y] in map space.
function pointInPolygon(x, y, poly) {
  var inside = false;
  for (var i = 0, j = poly.length - 1; i < poly.length; j = i++) {
    var xi = poly[i][0], yi = poly[i][1];
    var xj = poly[j][0], yj = poly[j][1];
    var intersect = ((yi > y) !== (yj > y)) &&
                    (x < (xj - xi) * (y - yi) / (yj - yi) + xi);
    if (intersect) { inside = !inside; }
  }
  return inside;
}