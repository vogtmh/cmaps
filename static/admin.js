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
        var healthstatus = '<img src="images/dbcheck_ok2.png" style="width:100%;height:100%;" alt="" />'
        document.getElementById('healthstatus').innerHTML= healthstatus
      }
      else {
        var healthstatus = '<a href="admin/?tab=health">'
                         + '<img src="images/dbcheck_error2.png" style="width:100%;height:100%;" alt="" />'
                         + '</a>'
        document.getElementById('healthstatus').innerHTML = healthstatus
        $("#healthstatus").show();
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

function addInputfields(deskid, desktype, override, manual) {

  // New items overwrite all automatic settings
  if (override == 3) {
    var input = {id: 'NULL', map: map, x: $("#apideskx").val(), y: $("#apidesky").val(), dsk: $("#apideskdsk").val(), empl: $("#apideskempl").val(), avtr: $("#apideskavtr").val(), dept: $("#apideskdept").val()};
    var selected = $("#selDesktype").val();
  }
  else if (override == 2) {
    var input = manual;
    var selected = desktype;
    $("#selDesktype").val('ldap-desk'); 

  }
  else {
    attr = result_old.desks.find(o => Object.entries(o).find(([k, value]) => k === 'id' && value === deskid) !== undefined);
    var input = attr;
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

  var fields = '';
  switch (selected) {
    case "exit":
    case "firstaid":
    case "food":
    case "keycardlock":
    case "keylock":
    case "printer":
    case "restroom":
    case "service":
        fields+='<div style="width:30%; float:left;display:inline;">x</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskx" name="formx" value="' + input.x + '">';
        fields+='<div style="width:30%; float:left;display:inline;">y</div><input type="text" style="width: 70%; float: left;display:inline;" id="apidesky" name="formy" value="' + input.y + '">';
        fields+='<div style="width:30%; float:left;display:inline;">Desknumber</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskdsk" name="formdsk" value="' + input.dsk + '">';
        fields+='<div style="width:30%; float:left;display:inline;">Description</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskempl" name="formempl" value="' + input.empl + '">';
        fields+='<input type="hidden" id="apideskavtr" name="formavtr" value="' + selected + '">';
        fields+='<input type="hidden" id="apideskdept" name="formdept" value="- none -">';
        fields+='<input type="hidden" id="apidesktype" name="formdesktype" value="' + selected + '">';
        break;
    case "floor":
        fields+='<div style="width:30%; float:left;display:inline;">x</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskx" name="formx" value="' + input.x + '">';
        fields+='<div style="width:30%; float:left;display:inline;">y</div><input type="text" style="width: 70%; float: left;display:inline;" id="apidesky" name="formy" value="' + input.y + '">';
        fields+='<div style="width:30%; float:left;display:inline;">Label</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskempl" name="formempl" value="' + input.empl + '">';
        fields+='<input type="hidden" id="apideskdsk" name="formdsk" value="Floor">';
        fields+='<input type="hidden" id="apideskavtr" name="formavtr" value="' + selected + '">';
        fields+='<input type="hidden" id="apideskdept" name="formdept" value="- none -">';
        fields+='<input type="hidden" id="apidesktype" name="formdesktype" value="' + selected + '">';
        break;
    case "meeting":
        fields+='<div style="width:30%; float:left;display:inline;">x</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskx" name="formx" value="' + input.x + '">';
        fields+='<div style="width:30%; float:left;display:inline;">y</div><input type="text" style="width: 70%; float: left;display:inline;" id="apidesky" name="formy" value="' + input.y + '">';
        fields+='<div style="width:30%; float:left;display:inline;">Desknumber</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskdsk" name="formdsk" value="' + input.dsk + '">';
        fields+='<div style="width:30%; float:left;display:inline;">Description</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskempl" name="formempl" value="' + input.empl + '">';
        fields+='<div style="width:30%; float:left;display:inline;">Avatar</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskavtr" name="formavtr" value="' + input.avtr + '">';
        fields+='<input type="hidden" id="apideskdept" name="formdept" value="- none -">';
        fields+='<input type="hidden" id="apidesktype" name="formdesktype" value="' + selected + '">';
        break;
    case "ldap-desk":
    case "shareddesk":
        fields+='<div style="width:30%; float:left;display:inline;">x</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskx" name="formx" value="' + input.x + '">';
        fields+='<div style="width:30%; float:left;display:inline;">y</div><input type="text" style="width: 70%; float: left;display:inline;" id="apidesky" name="formy" value="' + input.y + '">';
        fields+='<input type="hidden" id="apidesktype" name="formdesktype" value="addesk">';
        switch (input.dsk) {
            case "Exit":
            case "FirstAid":
            case "Floor":
            case "Food":
            case "KeycardLock":
            case "KeyLock":
            case "Meeting":
            case "Printer":
            case "Restroom":
            case "Service":
                fields+='<div style="width:30%; float:left;display:inline;">Desknumber</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskdsk" name="formdsk" value="">';
                break;
            default:
                fields+='<div style="width:30%; float:left;display:inline;">Desknumber</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskdsk" name="formdsk" value="' + input.dsk + '">';
                break;
        }
        fields+='<input type="hidden" id= "apideskempl" name="apideskempl" value="ldap-mirror">';
        fields+='<input type="hidden" id="apideskavtr" name="formavtr" value="' + input.avtr + '">';
        fields+='<div style="width:30%; float:left;display:inline;">Department</div>';
        fields+='<select id="apideskdept" name="formdept" style="width: 70%; float: left;display:inline;">';
        $.each( departments, function( x, department ){
          if (department == input.dept) {
            fields+='<option value = "'+department+'" selected>'+department+'</option>';
          }
          else {
            fields+='<option value = "'+department+'">'+department+'</option>';
          }
        });
        fields+='</select>';
        break;
    case "blocked":
    case "booking":
    case "hotseat":
        fields+='<div style="width:30%; float:left;display:inline;">x</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskx" name="formx" value="' + input.x + '">';
        fields+='<div style="width:30%; float:left;display:inline;">y</div><input type="text" style="width: 70%; float: left;display:inline;" id="apidesky" name="formy" value="' + input.y + '">';
        //fields+='<input type="hidden" id= "apideskempl" name="apideskempl" value="' + type2keyword(selected) + '">';
        fields+='<input type="hidden" id="apidesktype" name="formdesktype" value="' + selected + '">';
        switch (input.dsk) {
            case "Exit":
            case "FirstAid":
            case "Floor":
            case "Food":
            case "KeycardLock":
            case "KeyLock":
            case "Meeting":
            case "Printer":
            case "Restroom":
            case "Service":
                fields+='<div style="width:30%; float:left;display:inline;">Desknumber</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskdsk" name="formdsk" value="">';
                break;
            default:
                fields+='<div style="width:30%; float:left;display:inline;">Desknumber</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskdsk" name="formdsk" value="' + input.dsk + '">';
                break;
        }
        fields+='<div style="width:30%; float:left;display:inline;">Description</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskempl" name="formempl" value="' + input.empl + '">';
        fields+='<input type="hidden" id="apideskavtr" name="formavtr" value="' + selected + '">';
        fields+='<div style="width:30%; float:left;display:inline;">Department</div>';
        fields+='<select id="apideskdept" name="formdept" style="width: 70%; float: left;display:inline;">';
        $.each( departments, function( x, department ){
          if (department == input.dept) {
            fields+='<option value = "'+department+'" selected>'+department+'</option>';
          }
          else {
            fields+='<option value = "'+department+'">'+department+'</option>';
          }
        });
        fields+='</select>';
      break;
    case "local-desk":
      fields+='<div style="width:30%; float:left;display:inline;">x</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskx" name="formx" value="' + input.x + '">';
      fields+='<div style="width:30%; float:left;display:inline;">y</div><input type="text" style="width: 70%; float: left;display:inline;" id="apidesky" name="formy" value="' + input.y + '">';
      fields+='<div style="width:30%; float:left;display:inline;">Desknumber</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskdsk" name="formdsk" value="' + input.dsk + '">';
      fields+='<div style="width:30%; float:left;display:inline;">Description</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskempl" name="formempl" value="' + input.empl + '">';
      fields+='<div style="width:30%; float:left;display:inline;">Avatar</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskavtr" name="formavtr" value="' + input.avtr + '">';
      fields+='<div style="width:30%; float:left;display:inline;">Department</div>';
      fields+='<select id="apideskdept" name="formdept" style="width: 70%; float: left;display:inline;">';
      $.each( departments, function( x, department ){
        if (department == input.dept) {
          fields+='<option value = "'+department+'" selected>'+department+'</option>';
        }
        else {
          fields+='<option value = "'+department+'">'+department+'</option>';
        }
      });
      fields+='</select>';
      fields+='<input type="hidden" id="apidesktype" name="formdesktype" value="localdesk">';
      break;
    case "newdesk":
      fields+='<div style="width:30%; float:left;display:inline;">x</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskx" name="formx" value="' + input.x + '">';
      fields+='<div style="width:30%; float:left;display:inline;">y</div><input type="text" style="width: 70%; float: left;display:inline;" id="apidesky" name="formy" value="' + input.y + '">';
      fields+='<div style="width:30%; float:left;display:inline;">Desknumber</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskdsk" name="formdsk" value="">';
      //fields+='<input type="hidden" id= "apideskempl" name="apideskempl" value="ldap-mirror">';
      fields+='<div style="width:30%; float:left;display:inline;">Description</div><input type="text" style="width: 70%; float: left;display:inline;" id="apideskempl" name="formempl" value="' + input.empl + '">';
      fields+='<input type="hidden" id="apideskavtr" name="formavtr" value="' + input.avtr + '">';
      fields+='<input type="hidden" id="apidesktype" name="formdesktype" value="addesk">';
      fields+='<div style="width:30%; float:left;display:inline;">Department</div>';
      fields+='<select id="apideskdept" name="formdept" style="width: 70%; float: left;display:inline;">';
      $.each( departments, function( x, department ){
          fields+='<option value = "'+department+'">'+department+'</option>';
      });
      fields+='</select>';
      break;
  }
  $("#inputfields").html(fields);
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
      $("body").css("background-image", "url(images/blackprint.png)");
      document.body.addEventListener("click", getClickPosition, false);
      $('#inputgrid').html(addButton);
      $('#newitem_container').hide();
      $('#newbox').hide();
      break;

    case "hideInputgrid":
      var addButton = '<input class="inputgridbutton" type="image" src="images/add.png" style="width:80px; height:80px;" onClick="return doNewItem(\'showInputgrid\')" onmouseover=this.src="images/add_on.png" onmouseout=this.src="images/add.png">';
        $("body").css("background-image", "url(images/blueprint.png)");
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
    document.onmouseup = closeDragElement;
    // call a function whenever the cursor moves:
    document.onmousemove = elementDrag;
  }

  function elementDrag(e) {
    e = e || window.event;
    e.preventDefault();
    var elementId = (e.target || e.srcElement).id;

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
    elmnt.style.left = (startItemX + diffX) + "px";
    elmnt.style.top = (startItemY + diffY) + "px";
    
  }

  function closeDragElement() {
    /* stop moving when mouse button is released:*/
    document.onmouseup = null;
    document.onmousemove = null;
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
        },
        error: function (result) {
          alert('Could not update desk. Please check if all attributes have been entered.');
        }
      });
    }
  }
}