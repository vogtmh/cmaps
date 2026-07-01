// Helper functions for all users and admins

// Declare global variables
var result_old;
var result_old_str;
var deskHandlersBound = false;
var overviewmaps;
var bookingstatus;
var meetingstatus; 
var printerstatus;
var stickaddresses;
var searchtext = "";
var activecalendar = '';
var userdate = '';

// Search results sidebar (Google-Maps-style). Width 0 = closed.
var SEARCH_SIDEBAR_WIDTH = 340;
var searchSidebarWidth = 0;
var searchLocalResults = [];
var searchGlobalResults = [];
var searchSelectedId = null;
// Id of the desk currently spotlighted by the search fog (null = no fog).
var searchFogId = null;

// Edit palette sidebar (editors only; opens on the right in edit mode). Width
// 0 = closed. The palette itself is built/controlled in admin.js; this global
// lives here so user.js (applyUsermodeUI) and resize.js (map shrink) can see it
// even on the non-editor build where admin.js is absent.
var editSidebarWidth = 0;

// Floor markers are locked to a fixed vertical rail at the right edge of the map
// (only their Y matters: they are vertical navigation anchors). FLOOR_RAIL_X is
// the locked centre X in the 1600px content space; FLOOR_TAB_HALFH is half the
// tab height in that same pre-zoom space. The rail sits flush against the right
// edge of the 1600px content (just in front of the edit sidebar).
var FLOOR_RAIL_X = 1597;
var FLOOR_TAB_HALFH = 13;

function toggleUsermode() {
  if (setting_usermode == 'edit') {
    setting_usermode = 'user';
  }
  else {
    setting_usermode = 'edit';
  }
  document.cookie = "setting_usermode=" + setting_usermode+'; expires=Fri, 31 Dec 9999 23:59:59 GMT; SameSite=Lax';
  applyUsermodeUI();
  updateDesks(true);
}

// Reflects the current edit/user mode in the toggle switch and hides the
// editor-only "add" (plus) button when in user mode, so a logged-in editor
// gets a user-like experience.
function applyUsermodeUI() {
  var editing = (typeof token !== 'undefined' && setting_usermode == 'edit');
  $("#usermode_switch").toggleClass('on', editing);
  // Reflect edit mode on <body> so editor-only affordances (e.g. floor marker
  // tabs) can be shown/hidden purely via CSS. Only true editors ever enter edit
  // mode; end users (no token) must never see these affordances.
  document.body.classList.toggle('editmode', editing);
  if (editing) {
    $("#inputgrid").show();
    $("#adminpanel_link").show();
  } else {
    $("#inputgrid").hide();
    $("#adminpanel_link").hide();
  }
  // Show/hide the editor drag-and-drop palette (defined in admin.js).
  if (typeof applyEditSidebar === 'function') { applyEditSidebar(); }
  // Re-apply the duplicate-desk health highlight (only shows in edit mode).
  flagHealthDesks();
}

// flagHealthDesks marks the desks reported by the consistency check (duplicate
// desk names on this map) with a pulsing ring so an editor can find and fix
// them. The set is recomputed from the CURRENT desk data on every call so the
// ring updates live as desks are added/removed/renamed in edit mode (adding a
// second desk with the same name starts the glow; deleting one stops it). The
// ring is only ever visible to editors in edit mode (gated via body.editmode in
// CSS). Re-run after every desk re-render and on every edit-mode toggle. The
// server still injects `healthFlaggedDesks` for the first paint; it is used as a
// fallback only when live desk data is not yet available.
function flagHealthDesks() {
  var prev = document.querySelectorAll('#deskitems .deskball.health_flag');
  for (var i = 0; i < prev.length; i++) { prev[i].classList.remove('health_flag'); }

  var flagged = computeDuplicateDeskIds();

  for (var j = 0; j < flagged.length; j++) {
    var el = document.getElementById(flagged[j]);
    if (el && el.classList.contains('deskball')) { el.classList.add('health_flag'); }
  }
}

// computeDuplicateDeskIds returns the IDs of desks on the current map that share
// a (non-whitelisted) desk name with at least one other desk. Mirrors the
// server's duplicateDeskGroups() so the in-map highlight always agrees with the
// health report. Falls back to the server-injected list before live desk data
// exists (first paint) and is empty on the overview map.
function computeDuplicateDeskIds() {
  if (typeof mapname !== 'undefined' && mapname === 'overview') { return []; }
  // No live desk data yet (first paint): use the server-injected set.
  if (!result_old || !result_old.desks || !result_old.desks.length) {
    return (typeof healthFlaggedDesks !== 'undefined' && healthFlaggedDesks) ? healthFlaggedDesks : [];
  }
  var wl = (typeof healthWhitelist !== 'undefined' && healthWhitelist) ? healthWhitelist : [];
  // Collapse to one entry per physical desk ID first. A single stored desk that
  // is shared by several people is expanded server-side into multiple
  // "shareddesk" rows that all carry the SAME id and desk name; without this
  // de-duplication they would look like a name clash and glow even though the
  // server (which works on the raw, unexpanded desks) never flags them. Grouping
  // unique IDs by name then mirrors the server's duplicateDeskGroups() exactly.
  var nameById = {};
  var desks = result_old.desks;
  for (var i = 0; i < desks.length; i++) {
    var d = desks[i];
    if (d.id === undefined || d.id === null) { continue; }
    var name = d.dsk;
    if (name === undefined || name === null) { continue; }
    if (wl.indexOf(name) !== -1) { continue; }
    if (!(d.id in nameById)) { nameById[d.id] = name; }
  }
  var byName = {};
  for (var id in nameById) {
    (byName[nameById[id]] = byName[nameById[id]] || []).push(id);
  }
  var ids = [];
  for (var key in byName) {
    if (byName[key].length >= 2) {
      for (var k = 0; k < byName[key].length; k++) { ids.push(byName[key][k]); }
    }
  }
  return ids;
}

function timezoneDate() {
  let tz_datetime_str = new Date().toLocaleString("en-US", { timeZone: region });
  let date_tz = new Date(tz_datetime_str);
  let year = date_tz.getFullYear();
  let month = ("0" + (date_tz.getMonth() + 1)).slice(-2);
  let date = ("0" + date_tz.getDate()).slice(-2);
  let timezoneDate = year + "-" + month + "-" + date;
  return timezoneDate;
}

function formatDate(date) {
  var d = new Date(date),
      month = '' + (d.getMonth() + 1),
      day = '' + d.getDate(),
      year = d.getFullYear();
  if (month.length < 2) 
      month = '0' + month;
  if (day.length < 2) 
      day = '0' + day;
  return [year, month, day].join('-');
}

function getMondayOfCurrentWeek() {
  const today = new Date(timezoneDate());
  const first = today.getDate() - today.getDay() + 1;
  const monday = new Date(today.setDate(first));
  return formatDate(monday);
}

function updateCounter() {

  var checkcounter = document.getElementById('visitorcounter');
  if (checkcounter == null) {
    var p = document.getElementById('buttons_right');
    var newElement = document.createElement('div');
    newElement.setAttribute('id', 'visitorcounter');
    newElement.setAttribute('style', 'float:right; margin:5px; width:45px;height:15px; background:#333333; border-radius:15px; border: white 5px solid; padding:11px;opacity:0.8; text-align:center; line-height:15px;');
    newElement.innerHTML = '';
    p.appendChild(newElement);
  }

  $.ajax({
    
    // fetch data from stats API
    url: 'rest/stats/?interval=day&limit=1',
    async: true, 
    type: 'get',
    dataType: 'JSON',
    success: function(result){
      //console.log(result)
      document.getElementById('visitorcounter').innerHTML=result[0]['count']
      console.log('[Stats] Updated data for daily visitor counter.')
    },
    error: function()
    {
      console.log('[Stats] Could not get data for daily visitor counter from database.')
    }
  });

}

function addStat () {
  $.ajax({
    url: 'rest/stats',
    async: true, 
    type: 'get',
    dataType: 'JSON',
    success: function(result){
      // console.log('Succesfully added stat via stats API')
    },
    error: function (result) {
      console.log('Could not access stats API')
    }
  });
}

function logoutUser(mode) {
  if (mode == 'admin') {
    remoteurl = '../rest/account/'
  }
  else {
    remoteurl = 'rest/account/'
  }
  $.ajax({
    type: 'GET',
    url: remoteurl,
    data: {mode:'logout'},
    contentType: false,
    cache: false,
    //processData:false,
    beforeSend: function(){
      senddata = $("#Login").serialize();
    },
    error:function(){
      console.log('error');
    },
    success: function(logindata){
      var logstatus = logindata.status;
      var logmessage = logindata.message;
      if (logstatus == 'error') {
        var color='#D82626'
      }
      else if (logstatus == 'ok') {
        var color='#35842E'
      }
      if (logstatus == 'ok') { 
        if (mode == 'admin') { 
          location.href='/'; 
        }
        else {
          location.reload(); 
        }
      }
    }
  });
}

// Backwards-compatible shim: the login UI is now the unified overlay in
// login.js (cmapsLogin). loginForm(true) opens it, loginForm(false) closes it.
function loginForm (showform) {
  if (showform === false) {
    if (typeof cmapsCloseLogin === 'function') { cmapsCloseLogin(); }
    return;
  }
  if (typeof cmapsLogin === 'function') { cmapsLogin(); }
}

function showMapplate (mapname) {
  var mapinfo = overviewmaps.find(o => Object.entries(o).find(([k, value]) => k === 'mapname' && value === mapname) !== undefined);

  // Classic mapflags are 30px and anchored by their top-left corner, so their
  // centre sits at x+15 / y+15. Offset the plate to that centre, minus 100.
  var plateX = Number(mapinfo.x)+15-100;
  var plateY = Number(mapinfo.y)+15-100;

  if (plateY < 0) {
    var y_nameplate = 0;
  }
  else {
    var y_nameplate = plateY; 
  }

  copylink_full = 'https://'+window.location.hostname+root+'?map='+mapinfo.mapname;
  copylink_full = encodeURI(copylink_full);

  var maplabel = mapinfo.displayname ? mapinfo.displayname : ucWords(mapinfo.mapname);

  if (plateX > (targetScreenWidth/2)) {
    var outputmapplate= '<div class="leftmapplate" style="top:'+y_nameplate+'px;left:'+(plateX-400)+'px">'
                      + '<div class="leftmapplate_goto" onclick=hideMapplate()><img src="images/close3.png" style="width:70px;margin-top:60px;margin-right:20px;"></div>'
                      + '<div class="leftmapplate_top">'+maplabel+'</div>'
                      + '<div class="leftmapplate_textbox" >'+mapinfo.address+'</div>'
                      + '<img class="leftmapplate_avatar" src="countryflags/'+mapinfo.country+'.svg" style="background:rgba(0, 200, 200,1.0)" />'
                      + '<a href="'+root+'?map='+mapinfo.mapname+'">'
                      + '<img class="leftmapplate_close" src="images/openlink.png" />'
                      + '</a>'
                      + '<img class="leftmapplate_copy" src="images/copy.png" onclick="copyToClipboard(\''+copylink_full+'\')" />'
                      + '</div>';
    }
  else {
    var outputmapplate= '<div class="rightmapplate" style="top:'+y_nameplate+'px;left:'+plateX+'px">'
                      + '<div class="rightmapplate_goto" onclick=hideMapplate()><img src="images/close3.png" style="width:70px;margin-top:60px;margin-left:20px;"></div>'
                      + '<div class="rightmapplate_top">'+maplabel+'</div>'
                      + '<div class="rightmapplate_textbox" >'+mapinfo.address+'</div>'
                      + '<img class="rightmapplate_avatar" src="countryflags/'+mapinfo.country+'.svg" style="background:rgba(0, 200, 200,1.0)" />'
                      + '<a href="'+root+'?map='+mapinfo.mapname+'">'
                      + '<img class="rightmapplate_close" src="images/openlink.png" />'
                      + '</a>'
                      + '<img class="rightmapplate_copy" src="images/copy.png" onclick="copyToClipboard(\''+copylink_full+'\')" />'
                      + '</div>';
    }

  // Remove old mapplate if exists
  var element = document.getElementById('mapplate');
  if (element !== null) {
    element.parentNode.removeChild(element);
  }

  // Adds mapplate to the document
  var p = document.getElementById('mapOverview');
  var newElement = document.createElement('div');
  newElement.setAttribute('id', 'mapplate');
  newElement.innerHTML = outputmapplate;
  p.appendChild(newElement);
}

function hideMapplate () {
  // Remove old mapplate if exists
  var element = document.getElementById('mapplate');
  if (element !== null) {
    element.parentNode.removeChild(element);
  }
}

function hideNameplate () {
    // Removes an element from the document
    var element = document.getElementById('nameplate');
    if (element !== null) {
      element.parentNode.removeChild(element);
    }
  }

function hideSticky () {
    // Removes an element from the document
    var element = document.getElementById('stickynameplate');
    if (element !== null) {
      element.parentNode.removeChild(element);
    }
    activecalendar = '';
}

function highlightManagers() {
    var managers = result_old.desks.filter(element => element.color !="");
    var ringWidth = 3; // ring thickness in the ball's own (pre-zoom) px
    $.each( managers, function( t, manager ){
      // Draw the VIP/manager ring as a "fake" deskball placed right behind the
      // real one. We CLONE the actual ball node (shallow) so the ring inherits
      // the ball's EXACT geometry: the same inline left/top/zoom AND the same
      // CSS class (hence identical width/height/border-radius/zoom). No size
      // math is done by hand, so there is nothing to drift under autozoom.
      // We then strip its fill, give it a coloured border, and nudge it out by
      // the border width so the ring sits concentrically around the real ball.
      var ball = document.getElementById(manager.id);
      if (!ball) { return; }
      var existing = document.getElementById('manager' + manager.id);
      if (existing) { existing.remove(); }
      var ring = ball.cloneNode(false); // shallow clone -> no caption children
      ring.id = 'manager' + manager.id;
      // Keep the ball's content box size; let the border grow it outward and
      // shift the origin by the border width to stay centred (content-box).
      ring.style.boxSizing = 'content-box';
      ring.style.left = (parseFloat(ball.style.left) - ringWidth) + 'px';
      ring.style.top  = (parseFloat(ball.style.top)  - ringWidth) + 'px';
      ring.style.border = ringWidth + 'px solid ' + manager.color;
      ring.style.background = 'transparent';      // drop any background-color
      ring.style.backgroundImage = 'none';        // drop dot.png/icon images
      ring.style.zIndex = '98';                   // just below ball (.deskball:99)
      ring.style.pointerEvents = 'none';
      // Insert as a sibling right before the ball so it paints behind it.
      ball.parentNode.insertBefore(ring, ball);
    });
}

function showDesknumbers() {
    var desks = result_old.desks;
    if (setting_shownames == 0) {
      lineheight = 20;
    }
    else {
      lineheight = 10;
    }

    if (setting_printmode == 0) {
      var textcolor = '#fff';
    }
    else {
      var textcolor = '#000';
    }

    $.each( desks, function( x, desk ){
      switch (desk.dsk) {
        case "Exit":
        case "Meeting":
        case "Restroom":
        case "FirstAid":
        case "Firstaid":
        case "Food":
        case "KeycardLock":
        case "KeyLock":
        case "Printer":
        case "Service":
          var desknumber = desk.empl;
          var displayNumber = false;
          break;
        default: 
          var desknumber = desk.dsk;
          var displayNumber = true;
          break;
      }
      desknumber = desknumber.substring(desknumber.indexOf("-") + 1);
      if (desk.dsk != "Floor" && displayNumber == true && desk.desktype != "shareddesk") {
        // Create overlayed label
        var p = document.getElementById('deskitems');
        var newElement = document.createElement('div');
        newElement.setAttribute('id', 'desknumber' + desk.id);
        newElement.innerHTML = desknumber;
        p.appendChild(newElement);
        //newElement.innerHTML = output;
        $('#desknumber' + desk.id).css('position','absolute');
        $('#desknumber' + desk.id).css('left',(desk.x-(20*itemscale*itemscale)) + 'px');
        $('#desknumber' + desk.id).css('top',(desk.y-(10*itemscale)) + 'px');
        $('#desknumber' + desk.id).css('width',(40*itemscale*itemscale)+'px');
        $('#desknumber' + desk.id).css('height',(20*itemscale*itemscale)+'px');
        $('#desknumber' + desk.id).css('text-align','center');
        $('#desknumber' + desk.id).css('font-size',(8*itemscale)+'px');
        $('#desknumber' + desk.id).css('color',textcolor);
        $('#desknumber' + desk.id).css('line-height',(lineheight*itemscale)+'px');
        $('#desknumber' + desk.id).css('background-color','transparent');
        $('#desknumber' + desk.id).css('z-index','9');
      }
    });
}

function showNames() {
    var desks = result_old.desks;
    if (setting_printmode == 0) {
      var textcolor = '#fff';
    }
    else {
      var textcolor = '#000';
    }
    $.each( desks, function( x, desk ){
      if (setting_desknumbers == 0) {
        lineheight = 20;
        divtop = (desk.y-(10*itemscale))
      }
      else {
        lineheight = 10;
        divtop = desk.y
      }
      switch (desk.dsk) {
        case "Exit":
        case "Meeting":
        case "Restroom":
        case "FirstAid":
        case "Firstaid":
        case "Food":
        case "KeycardLock":
        case "KeyLock":
        case "Printer":
        case "Service":
          var divname = '';
          var displayNumber = false;
          break;
        default: 
          var divname = desk.fname;
          var displayNumber = true;
          break;
      }
      if (desk.dsk != "Floor" && displayNumber == true && desk.desktype != "shareddesk") {
        // Create overlayed label
        var p = document.getElementById('deskitems');
        var newElement = document.createElement('div');
        newElement.setAttribute('id', 'name' + desk.id);
        newElement.innerHTML = divname;
        p.appendChild(newElement);
        //newElement.innerHTML = output;
        $('#name' + desk.id).css('position','absolute');
        $('#name' + desk.id).css('left',(desk.x-(20*itemscale*itemscale)) + 'px');
        $('#name' + desk.id).css('top',divtop + 'px');
        $('#name' + desk.id).css('width',(40*itemscale*itemscale)+'px');
        $('#name' + desk.id).css('height',(20*itemscale*itemscale)+'px');
        $('#name' + desk.id).css('text-align','center');
        $('#name' + desk.id).css('font-size',(8*itemscale)+'px');
        $('#name' + desk.id).css('color',textcolor);
        $('#name' + desk.id).css('line-height',(lineheight*itemscale)+'px');
        $('#name' + desk.id).css('background-color','transparent');
        $('#name' + desk.id).css('z-index','9');
      }
    });
}

function ucWords (word) {
    word = word.toLowerCase().replace(/^[\u00C0-\u1FFF\u2C00-\uD7FF\w]|\s[\u00C0-\u1FFF\u2C00-\uD7FF\w]/g, function(letter) {
        return letter.toUpperCase();
    });
    return word;
}

function imageExist(url)
{
var img = new Image();
img.src = url;
return img.height != 0;
}

// avatarUrl returns the avatar image URL for a desk occupant. When the server
// reports no cached avatar (hasavatar !== true), everyone resolves to the same
// shared placeholder URL, which the browser downloads/caches exactly once
// instead of requesting a unique (missing) image per person.
function avatarUrl(avtr, hasavatar) {
  if (hasavatar !== true || !avtr) {
    return 'images/noavatar.png';
  }
  return 'avatarcache/' + avtr + '.jpg';
}

function getCookie(cname) {
  var name = cname + "=";
  var decodedCookie = decodeURIComponent(document.cookie);
  var ca = decodedCookie.split(";");
  for(var i = 0; i <ca.length; i++) {
    var c = ca[i];
    while (c.charAt(0) == " ") {
    c = c.substring(1);
    }
    if (c.indexOf(name) == 0) {
    return c.substring(name.length, c.length);
    }
  }
  return "";
  }

function statsPanel() {
    var output = '<table border="0" style="width:560px; margin-left:30px;">'
                + '<tr>'
                + '<td style="font-weight: bold;color:lightgrey;text-align:left">'+ucWords(map)+'</td>'
                + '<td style="width:130px"></td>'
                + '<td style="width:130px"></td><td style="width:130px"></td>'
                + '</tr>'
                + '<tr>'
                + '<td style="font-weight: bold;color:grey;text-align:left">Department</td>'
                + '<td style="font-weight: bold;color:lightblue;width:130px">Total desks</td>'
                + '<td style="font-weight: bold;color:orange;width:130px">In use</td>'
                + '<td style="font-weight: bold;color:green;width:130px">Free</td>'
                + '</tr>'
                +  '<tr><td>&nbsp;</td></tr>';
    // Output departments one by one
    $.each( departments, function( x, department ){
        var all = result_old.desks.filter(element => element.dept == department);
        var total1 = all.filter(element => element.desktype == 'addesk');
        var total2 = all.filter(element => element.desktype == 'blocked');
        var total3 = all.filter(element => element.desktype == 'hotseat');
        var totalcount = Object.keys(total1).length + Object.keys(total2).length + Object.keys(total3).length;
        var used1 = total1.filter(element => element.fname != '');
        var usedcount = Object.keys(used1).length + Object.keys(total2).length + Object.keys(total3).length;
        var freecount = totalcount - usedcount;
        output+='<tr>'
            + '<td style="color:grey;text-align:left">'+department+'</td>'
            + '<td style="color:lightblue;">'+totalcount+'</td>'
            + '<td style="color:orange;">'+usedcount+'</td>'
            + '<td style="color:green;">'+freecount+'</td>'
            + '</tr>';
    });
    var all = result_old.desks;
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
            + '<td style="color:lightblue; font-weight:bold;">'+totalcount+'</td>'
            + '<td style="color:orange; font-weight:bold;">'+usedcount+'</td>'
            + '<td style="color:green; font-weight:bold;">'+freecount+'</td>'
            + '</tr>';
    $("#statsTable").html(output);
}

function showNameplate (deskid, desktype) {
    attr = result_old.desks.find(o => Object.entries(o).find(([k, value]) => k === 'id' && value === deskid) !== undefined);
    var content = '';
    switch (desktype) {           
      
      case "restroom":
      case "food":
      case "service":
      case "exit":
      case "keycardlock":
      case "keylock":
      case "floor":
      case "blocked":
        var caption = attr.empl;
        var avatar = 'images/' + desktype + '.png';
        var avatarcolor = $('#' + attr.id).css('background-color');
        var hotseat_booking;
        content = attr.title + '<br />'+ attr.mail + '<br />'+ attr.phone + '<br />' + attr.mobil + '<br />'
        break;
      case "booking":
      case "booking_free":
      case "booking_booked":
      case "hotseat":
      case "hotseat_free":
      case "hotseat_booked":
        var caption = attr.empl;
        var avatar = 'images/hotseat.png';
        var avatarcolor = $('#' + attr.id).css('background-color');
        content = 'no bookings for today';
        var deskbookings = bookingstatus.filter(element => element.desk == attr.dsk);
        if (deskbookings.length > 0) {
          for (var i = 0; i < deskbookings.length; i++) {
            var deskbooking = deskbookings[i];
            var bookdate  = deskbooking.date;
            var bookname  = deskbooking.name;
            var bookphone = deskbooking.phone;
            var bookmail  = deskbooking.mail;
            // if user selected a custom date, use that one instead of today
            if (userdate != '') {
              var selectdate = userdate;
            }
            else {
              var selectdate = timezoneDate()
            }
            if (selectdate == bookdate) {
              content = bookname+'<br/>'+bookphone+'<br/>'+bookmail;
              break;
            }
          }
        }
        break;
      case "firstaid":
        var caption = attr.dsk;
        var avatar = 'images/' + desktype + '.png';
        var avatarcolor = $('#' + attr.id).css('background-color');
        content = attr.title + '<br />'+ attr.mail + '<br />'+ attr.phone + '<br />' + attr.mobil + '<br />'
        break;
      case "meeting":
        var caption = attr.empl;
        // Meeting rooms always use their type icon (no per-room avatar lookup).
        avatar = 'images/' + desktype + '.png';
        
        var avatarcolor = $('#' + attr.id).css('background-color')
        roomstatus = meetingstatus.filter(element => element.deskid == attr.id);
        console.log(meetingstatus);
        var nowcolor = 'transparent';
        var nowtext = ''; var nexttext = '';
        var nextcolor = 'transparent';
        if (roomstatus != '') {
          switch (roomstatus[0].availability) {
            case "booked":
            case "in_use":
              nowcolor = 'rgba(0, 0, 136)';
              nextcolor = 'rgb(255,160,0)';
              nowtext = roomstatus[0].now_title + '<br />' + roomstatus[0].now_start + ' - ' +roomstatus[0].now_end;
              nexttext = roomstatus[0].next_title + '<br />' + roomstatus[0].next_start + ' - ' +roomstatus[0].next_end;
              break;
            case "available":
              nowcolor = '#008800';
              nextcolor = 'rgb(255,160,0)';
              nowtext = 'Available';
              nexttext = roomstatus[0].next_title + '<br />' + roomstatus[0].next_start + ' - ' +roomstatus[0].next_end;
              break;
          }
        }
        if (attr.x > (targetScreenWidth/2)) {
          var boxstyle = 'top: 51px;width: 490px;height: 147px;border-radius:0px 0px 0px 10px; padding-right:0px;padding-left:0px;';
          content = '<div class="leftmeet_now" style="background-color:'+nowcolor+'"><div class="meettext">'+nowtext+'</div></div>'
                + '<div class="leftmeet_next" style="background-color:'+nextcolor+'"><div class="meettext">'+nexttext+'</div></div>'
        }
        else {
          var boxstyle = 'top: 51px;width: 490px;height: 147px;border-radius:0px 0px 10px 0px; padding-left:0px;padding-right:0px;';
          content = '<div class="rightmeet_now" style="background-color:'+nowcolor+'"><div class="meettext">'+nowtext+'</div></div>'
                + '<div class="rightmeet_next" style="background-color:'+nextcolor+'"><div class="meettext">'+nexttext+'</div></div>'
        }
        break;
      case "printer":
        var caption = attr.empl;
        var avatar = 'images/' + desktype + '.png';
        var avatarcolor = $('#' + attr.id).css('background-color');
        var boxstyle = 'font-size:18px; margin-top:10px;'
        /*
        printers = printerstatus.filter(element => element.printername == attr.empl);
        printer = printers[0];
        if (printer.availability == '1') {
          var status = 'online';
          var statuscolor = '#0f0';
          if (typeof(printer.black) != 'undefined' && typeof(printer.magenta) != 'undefined' && typeof(printer.cyan) != 'undefined' && typeof(printer.yellow) != 'undefined') {
            content += '<div style="width:94px; height:40px;background-color:#000000;float:left;margin-bottom:10px;line-height:40px;text-align:center;border-radius:20px 0px 0px 20px;">'+printer.black+'</div>'
                   + '<div style="width:94px; height:40px;background-color:#a200a2;float:left;margin-bottom:10px;line-height:40px;text-align:center;">'+printer.magenta+'</div>'
                   + '<div style="width:94px; height:40px;background-color:#20b2aa;float:left;margin-bottom:10px;line-height:40px;text-align:center;">'+printer.cyan+'</div>'
                   + '<div style="width:94px; height:40px;background-color:#cccc00;float:left;margin-bottom:10px;line-height:40px;text-align:center;border-radius:0px 20px 20px 0px;">'+printer.yellow+'</div>';
          }
        }
        else {
          var status = 'offline';
          var statuscolor = '#f00';
        }
        content += '<div style="width:376px; height:40px;float:left;line-height:40px;text-align:center;color:'+statuscolor+';font-size:24px;">'+status+'</div>'*/
        break;
      case "free":
        var caption = 'Not in use';
        var avatar = 'images/' + desktype + '.png';
        var avatarcolor = $('#' + attr.id).css('background-color');
        content = attr.title + '<br />'+ attr.mail + '<br />'+ attr.phone + '<br />' + attr.mobil + '<br />'
        break;
      case "occupied":
        var caption = attr.empl;
        var avatar = avatarUrl(attr.avtr, attr.hasavatar);
        var avatarcolor = $('#' + attr.id).css('background-color');
        content = attr.title + '<br />'+ attr.mail + '<br />'+ attr.phone + '<br />' + attr.mobil + '<br />'
        break;
      case "occupiedldap":
        var caption = attr.fname + ' ' + attr.lname;
        var avatar = avatarUrl(attr.avtr, attr.hasavatar);
        var avatarcolor = $('#' + attr.id).css('background-color');
        content = attr.title + '<br />'+ attr.mail + '<br />'+ attr.phone + '<br />' + attr.mobil + '<br />'
        break;
      case "shareddesk":
        var caption = 'Shared Desk';
        var avatar = 'images/free.png';
        var avatarArr = [];
        var avatarcolor = $('#' + attr.id).css('background-color');
        var multiresult = result_old.desks.filter(element => element.id == attr.id);
        for (var m = 0; m < multiresult.length; m++) {
          var textcolor = '#FFFFFF';
          avatarArr.push(multiresult[m].avtr)
          if (searchtext != '') {
            var searchArr = searchtext.split('|');
            for (var s = 0; s < searchArr.length; s++) {
              var namecheck = multiresult[m].fname.toLowerCase()+' '+multiresult[m].lname.toLowerCase()
              var searchcheck = searchArr[s];
              searchcheck = searchcheck.toLowerCase();
              var sresult = namecheck.includes(searchcheck);
              if (sresult) {textcolor='#FF7F00'}
            }  
          }
          content += '<div style="color:'+textcolor+'">'+multiresult[m].fname+' '+multiresult[m].lname+'</div>';
        }
        break;
      default:
        // Admin-defined custom item types: a simple labelled marker.
        if (desktype && desktype.indexOf('custom_') === 0) {
          var ctNp = (typeof customItemTypes !== 'undefined' && customItemTypes[desktype.slice(7)]) ? customItemTypes[desktype.slice(7)] : null;
          var caption = attr.dsk || (ctNp ? ctNp.label : 'Item');
          var avatar = (ctNp && ctNp.icon) ? ctNp.icon : 'images/noavatar.png';
          var avatarcolor = $('#' + attr.id).css('background-color');
          content = (ctNp && ctNp.description) ? ctNp.description : '';
        }
        break;
    }
    if (attr.color != '') {
      var color = attr.color;
    }
    else {
      var color = '#999';
    }
    // Robin live-occupancy overlay: keep the standard grey header (the tinted
    // ball already marks the desk as a Robin reservation) and show a badge.
    var robinBadge = '';
    if (attr.robin == '1') {
      robinBadge = '<span class="robinbadge">Robin</span>';
    }
    avatarcolor = avatarcolor.replace(/[^,]+(?=\))/, '1.0');
    if (attr.y < 100) {
      var y_nameplate = 100;
    }
    else {
      var y_nameplate = attr.y; 
    }
    if (attr.x > (targetScreenWidth/2)) {
      if (desktype !== 'shareddesk') {
        var outputnameplate='<div class="leftnameplate" style="position: absolute; top:' + (y_nameplate-100) +'px;left:' + (attr.x-650) + 'px;">'
                        + '<div class="leftnameplate_top" style="background:' + color
                        + '">' + caption + '</div>'
                        + '<div class="leftnameplate_textbox" style="'+boxstyle+'" id="textbox' + attr.id +'">' + content + '</div>'
                        + '<img src="' + avatar + '" class="leftnameplate_avatar" style="background:' + avatarcolor + '" onerror="this.src=\'images/noavatar.png\'">'
                        + '<div class="leftnameplate_number">' + attr.dsk + '</div>'
                        + robinBadge + '</div>'
                        + '<div id="caption' + attr.id + '" class="caption">' + attr.empl + '</div>'
                        + '</div>';
      }
      else {
        // shared desk has a special avatar
        var outputnameplate='<div class="leftnameplate" style="position: absolute; top:' + (y_nameplate-100) +'px;left:' + (attr.x-650) + 'px;">'
                        + '<div class="leftnameplate_top" style="background:' + color
                        + '">' + caption + '</div>'
                        + '<div class="leftnameplate_textbox" style="'+boxstyle+'" id="textbox' + attr.id +'">' + content + '</div>'
                        + '<div class="leftnameplate_avatar">'
        switch(avatarArr.length) {
          case 2: 
            outputnameplate += ''
            + '<img src="avatarcache/'+avatarArr[0]+'.jpg" style="position:absolute; right:98px; top:0px; width:196px; height:196px" onerror="this.src=\'images/noavatar.png\'"/>'
            + '<img src="avatarcache/'+avatarArr[1]+'.jpg" style="position:absolute; left:98px; top:0px; width:196px; height:196px" onerror="this.src=\'images/noavatar.png\'"/>'
            break;
          case 3:
            outputnameplate += ''
            + '<img src="avatarcache/'+avatarArr[0]+'.jpg" style="position:absolute; right:98px; top:0px; width:196px; height:196px" onerror="this.src=\'images/noavatar.png\'"/>'
            + '<img src="avatarcache/'+avatarArr[1]+'.jpg" style="position:absolute; right:0px; top:0px; width:98px; height:98px" onerror="this.src=\'images/noavatar.png\'"/>'
            + '<img src="avatarcache/'+avatarArr[2]+'.jpg" style="position:absolute; right:0px; bottom:0px; width:98px; height:98px" onerror="this.src=\'images/noavatar.png\'"/>'
            break;
          case 4:
            outputnameplate += ''
            + '<img src="avatarcache/'+avatarArr[0]+'.jpg" style="position:absolute; left:0px; top:0px; width:50%; height:50%" onerror="this.src=\'images/noavatar.png\'"/>'
            + '<img src="avatarcache/'+avatarArr[1]+'.jpg" style="position:absolute; right:0px; top:0px; width:50%; height:50%" onerror="this.src=\'images/noavatar.png\'"/>'
            + '<img src="avatarcache/'+avatarArr[2]+'.jpg" style="position:absolute; left:0px; bottom:0px; width:50%; height:50%" onerror="this.src=\'images/noavatar.png\'"/>'
            + '<img src="avatarcache/'+avatarArr[3]+'.jpg" style="position:absolute; right:0px; bottom:0px; width:50%; height:50%" onerror="this.src=\'images/noavatar.png\'"/>'
            break;
        }

        outputnameplate += '</div>'
                        + '<div class="leftnameplate_number">' + attr.dsk + '</div>'
                        + robinBadge + '</div>'
                        + '<div id="caption' + attr.id + '" class="caption">' + attr.empl + '</div>'
                        + '</div>';
      }
    }
    else {
      if (desktype !== 'shareddesk') {
        var outputnameplate='<div class="rightnameplate" style="position: absolute; top:' + (y_nameplate-100) +'px;left:' + (Number(attr.x) + 50) + 'px;">'
                      + '<div class="rightnameplate_top" style="background:' + color
                      + '">' + caption + '</div>'
                      + '<div class="rightnameplate_textbox" style="'+boxstyle+'" id="textbox' + attr.id +'">' + content + '</div>'
                      + '<img src="' + avatar + '" class="rightnameplate_avatar" style="background:' + avatarcolor + '" onerror="this.src=\'images/noavatar.png\'">'
                      + '<div class="rightnameplate_number">' + attr.dsk + '</div>'
                      + robinBadge + '</div>'
                      + '<div id="caption' + attr.id + '" class="caption">' + attr.empl + '</div>'
                      + '</div>';
      }
      else {
        // shared desk with special avatar
        var outputnameplate='<div class="rightnameplate" style="position: absolute; top:' + (y_nameplate-100) +'px;left:' + (Number(attr.x) + 50) + 'px;">'
                      + '<div class="rightnameplate_top" style="background:' + color
                      + '">' + caption + '</div>'
                      + '<div class="rightnameplate_textbox" style="'+boxstyle+'" id="textbox' + attr.id +'">' + content + '</div>'
                      + '<div class="rightnameplate_avatar">'
        switch(avatarArr.length) {
          case 2: 
          outputnameplate += ''
          + '<img src="avatarcache/'+avatarArr[0]+'.jpg" style="position:absolute; right:98px; top:0px; width:196px; height:196px" onerror="this.src=\'images/noavatar.png\'" />'
          + '<img src="avatarcache/'+avatarArr[1]+'.jpg" style="position:absolute; left:98px; top:0px; width:196px; height:196px" onerror="this.src=\'images/noavatar.png\'" />'
          break;
          case 3:
            outputnameplate += ''
            + '<img src="avatarcache/'+avatarArr[0]+'.jpg" style="position:absolute; right:98px; top:0px; width:196px; height:196px" onerror="this.src=\'images/noavatar.png\'" />'
            + '<img src="avatarcache/'+avatarArr[1]+'.jpg" style="position:absolute; right:0px; top:0px; width:98px; height:98px" onerror="this.src=\'images/noavatar.png\'" />'
            + '<img src="avatarcache/'+avatarArr[2]+'.jpg" style="position:absolute; right:0px; bottom:0px; width:98px; height:98px" onerror="this.src=\'images/noavatar.png\'" />'
            break;
          case 4:
            outputnameplate += ''
            + '<img src="avatarcache/'+avatarArr[0]+'.jpg" style="position:absolute; left:0px; top:0px; width:50%; height:50%" onerror="this.src=\'images/noavatar.png\'" />'
            + '<img src="avatarcache/'+avatarArr[1]+'.jpg" style="position:absolute; right:0px; top:0px; width:50%; height:50%" onerror="this.src=\'images/noavatar.png\'" />'
            + '<img src="avatarcache/'+avatarArr[2]+'.jpg" style="position:absolute; left:0px; bottom:0px; width:50%; height:50%" onerror="this.src=\'images/noavatar.png\'" />'
            + '<img src="avatarcache/'+avatarArr[3]+'.jpg" style="position:absolute; right:0px; bottom:0px; width:50%; height:50%" onerror="this.src=\'images/noavatar.png\'" />'
            break;
        }              
        outputnameplate += '</div>'
                      + '<div class="rightnameplate_number">' + attr.dsk + '</div>'
                      + robinBadge + '</div>'
                      + '<div id="caption' + attr.id + '" class="caption">' + attr.empl + '</div>'
                      + '</div>';
      }
    }
    
    // Adds an element to the document
    var p = document.getElementById('deskitems');
    var newElement = document.createElement('div');
    newElement.setAttribute('id', 'nameplate');
    newElement.innerHTML = outputnameplate;
    p.appendChild(newElement);
}

function calendarSelection(selection) {
  $('.calendarday_free').css("background-color", "");
  $('.calendarday_booked').css("background-color", "");
  $('.calendarday_past').css("background-color", "");
  $('#bookdate').val(selection);
  $('#'+selection).css("background-color", "#FF7F00");
}

function updateCalendar(deskid) {
  attr = result_old.desks.find(o => Object.entries(o).find(([k, value]) => k === 'id' && value === deskid) !== undefined);
  usershort = username.replace(domain, "");
  var calendardata = `
    <div id="calendar`+deskid+`" class='calendar'>
    <div class='calendar_label'>Mo</div>
    <div class='calendar_label'>Tu</div>
    <div class='calendar_label'>We</div>
    <div class='calendar_label'>Th</div>
    <div class='calendar_label'>Fr</div>
  `
  var startDate = getMondayOfCurrentWeek();
  var today = new Date(timezoneDate());
  for (let i = 0; i < 28; i++) { 
    var start = new Date(startDate);
    var date = new Date();
    date.setTime(start.getTime() + (i*24*60*60*1000));

    var outputnumber = date.toISOString().substring(8, 10);
    var datestring = date.toISOString().substring(0, 10);
    if (date < today) {
      calendardata += '<div class="calendarday_past">'+outputnumber+'</div>';
    }
    else if (date.getDay() == 0 || date.getDay() == 6) {
      // do nothing for Saturday and Sunday
    }
    else {
      // check for each day if the desk has been booked
      bookingdetails = bookingstatus.filter(element => {
        return element.desk === attr.dsk && element.date === datestring
      });
      checkbooking = bookingdetails.length;
      
      if (checkbooking == 1) {
        // check if current user has booked the meeting
        checkuserbooking = bookingstatus.filter(element => {
          return element.desk === attr.dsk && element.date === datestring && element.user === usershort
        }).length;
        if (checkuserbooking == 1) {
          // yellow color
          calendardata += '<div class="calendarday_userbooked" id="'+datestring+'" title="Booked by you">'+outputnumber+'</div>';
        }
        else {
          // red color
          calendardata += '<div class="calendarday_booked" id="'+datestring+'" title="Booked by '+ bookingdetails[0].name +'">'+outputnumber+'</div>';
        }
      }
      else {
        calendardata += '<div class="calendarday_free" id="'+datestring+'" onclick=calendarSelection("'+datestring+'")>'+outputnumber+'</div>';
      }
    }
    
  }
  calendardata += "</div>";
  return calendardata;
}

function toggleDatepicker() {
  if ($("#theDate").is(":visible") == false) {
    var calendardata = `
    <div id="calendarDatepicker" class='calendar'>
    <div class='calendar_label'>Mo</div>
    <div class='calendar_label'>Tu</div>
    <div class='calendar_label'>We</div>
    <div class='calendar_label'>Th</div>
    <div class='calendar_label'>Fr</div>
    `
    var startDate = getMondayOfCurrentWeek();
    var today = new Date(timezoneDate());
    for (let i = 0; i < 28; i++) { 
      var start = new Date(startDate);
      var date = new Date();
      date.setTime(start.getTime() + (i*24*60*60*1000));

      var outputnumber = date.toISOString().substring(8, 10);
      var datestring = date.toISOString().substring(0, 10);
      if (date < today) {
        calendardata += '<div class="calendarday_past">'+outputnumber+'</div>';
      }
      else if (date.getDay() == 0 || date.getDay() == 6) {
        // do nothing for Saturday and Sunday
      }
      else if (datestring == userdate || (userdate == '' && datestring == timezoneDate() )) {
        calendardata += '<div class="calendarday_userdate" id="'+datestring+'" onclick=setUserdate("'+datestring+'")>'+outputnumber+'</div>';
      }
      else {
        calendardata += '<div class="calendarday_neutral" id="'+datestring+'" onclick=setUserdate("'+datestring+'")>'+outputnumber+'</div>';
      }
      
    }
    calendardata += "</div>";
    calendardata += "<div class='datepicker_separator'></div>";
    $("#theDate").html(calendardata);
  }
  $("#theDate").toggle();
}

function showSticky (deskid, desktype, caption) {
    attr = result_old.desks.find(o => Object.entries(o).find(([k, value]) => k === 'id' && value === deskid) !== undefined);
    if (typeof username !== 'undefined') {usershort = username.replace(domain, "");};
    var content = '';
    switch (desktype) {       
      case "restroom":
      case "food":
      case "service":
      case "exit":
      case "keycardlock":
      case "keylock":
      case "floor":
      case "blocked":
        var caption = attr.empl;
        var copylink = attr.empl;
        var avatar = 'images/' + desktype + '.png';
        var avatarcolor = $('#' + attr.id).css('background-color');
        content = attr.title + '<br />'+ attr.mail + '<br />'+ attr.phone + '<br />' + attr.mobil + '<br />'
        break;
      case "hotseat":
      case "hotseat_free":
      case "hotseat_booked":
      case "booking_free":
      case "booking_booked":
        var caption = attr.empl;
        var avatar = 'images/hotseat.png';
        var avatarcolor = $('#' + attr.id).css('background-color');
        if (typeof username !== 'undefined') {
          var calendar = updateCalendar(deskid)
          activecalendar = attr.id;
          var calendarform = `
            <div id="calendarform" style="width:170px;margin-left:5px; margin-right:5px;float:left;">
              <form id="Book" method="post" autocomplete="off" style="width:170px; float:left;">
                <input id="mode" type="hidden" name="mode" value="book">
                <input id="bookmap" type="hidden" name="bookmap" value="`+map+`">
                <input id="bookdesk" type="hidden" name="bookdesk" value="`+attr.dsk+`">
                <input id="bookdate" name="bookdate" type="hidden" style="height:39px;border-radius:5px;width:100px;">
                <input type="button" value="Book" style="width:160px;height:45px;margin-left:10px;margin-top:0px;float:left;" onclick="bookDesk()"><br/>
              </form>
              <div id="bookstatus" style="width: 160px;background-color:#35842e;float:left;text-align:center;margin-left:5px;height:35px;line-height:35px;border-radius:25px;padding:5px;display:none;"></div>
            </div>
          `
          if (attr.x > (targetScreenWidth/2)) {
            content = calendarform + calendar;
          }
          else {
            content = calendar + calendarform;
          }
        }
        else {
          content = 'Please <a href="rest/account" style="color:orange">login</a> to book a desk or check the booking status';
        }
        break;
      case "firstaid":
        var caption = attr.dsk;
        var copylink = attr.dsk;
        var avatar = 'images/' + desktype + '.png';
        var avatarcolor = $('#' + attr.id).css('background-color');
        content = attr.title + '<br />'+ attr.mail + '<br />'+ attr.phone + '<br />' + attr.mobil + '<br />'
        break;
      case "meeting":
        var caption = attr.empl;
        var copylink = attr.empl;
        // Meeting rooms always use their type icon (no per-room avatar lookup).
        var avatar = 'images/' + desktype + '.png';
        roomstatus = meetingstatus.filter(element => element.deskid == attr.id);
        var nowcolor = 'transparent';
        var nowtext = ''; var nexttext = '';
        var nextcolor = 'transparent';
        //console.log(roomstatus);
        if (roomstatus != '') {
          switch (roomstatus[0].availability) {
            case "booked":
            case "in_use":
              nowcolor = 'rgba(0, 0, 136)';
              nextcolor = 'rgb(255,160,0)';
              nowtext = roomstatus[0].now_title + '<br />' + roomstatus[0].now_start + ' - ' +roomstatus[0].now_end;
              nexttext = roomstatus[0].next_title + '<br />' + roomstatus[0].next_start + ' - ' +roomstatus[0].next_end;
              break;
            case "available":
              nowcolor = '#008800';
              nextcolor = 'rgb(255,160,0)';
              nowtext = 'Available';
              nexttext = roomstatus[0].next_title + '<br />' + roomstatus[0].next_start + ' - ' +roomstatus[0].next_end;
              break;
          }
        }
        if (attr.x > (targetScreenWidth/2)) {
          var boxstyle = 'top: 51px;width: 490px;height: 147px;border-radius:0px 0px 0px 10px; padding-right:0px;padding-left:0px;';
          content = '<div class="leftmeet_now" style="background-color:'+nowcolor+'"><div class="meettext">'+nowtext+'</div></div>'
                + '<div class="leftmeet_next" style="background-color:'+nextcolor+'"><div class="meettext">'+nexttext+'</div></div>'
        }
        else {
          var boxstyle = 'top: 51px;width: 490px;height: 147px;border-radius:0px 0px 10px 0px; padding-left:0px;padding-right:0px;';
          content = '<div class="rightmeet_now" style="background-color:'+nowcolor+'"><div class="meettext">'+nowtext+'</div></div>'
                + '<div class="rightmeet_next" style="background-color:'+nextcolor+'"><div class="meettext">'+nexttext+'</div></div>'
        }
        var avatarcolor = $('#' + attr.id).css('background-color');
        break;
      case "printer":
        var caption = attr.empl;
        var copylink = attr.empl;
        var avatar = 'images/' + desktype + '.png';
        var avatarcolor = $('#' + attr.id).css('background-color');
        var boxstyle = 'font-size:18px; margin-top:10px;'
        /*
        var link = attr.empl+'/';
        printers = printerstatus.filter(element => element.printername == attr.empl);
        printer = printers[0];
        if (printer.availability == '1') {
          var status = 'online';
          var statuscolor = '#0f0';
          if (typeof(printer.black) != 'undefined' && typeof(printer.magenta) != 'undefined' && typeof(printer.cyan) != 'undefined' && typeof(printer.yellow) != 'undefined') {
            content += '<div style="width:94px; height:40px;background-color:#000000;float:left;margin-bottom:10px;line-height:40px;text-align:center;border-radius:20px 0px 0px 20px;">'+printer.black+'</div>'
                   + '<div style="width:94px; height:40px;background-color:#a200a2;float:left;margin-bottom:10px;line-height:40px;text-align:center;">'+printer.magenta+'</div>'
                   + '<div style="width:94px; height:40px;background-color:#20b2aa;float:left;margin-bottom:10px;line-height:40px;text-align:center;">'+printer.cyan+'</div>'
                   + '<div style="width:94px; height:40px;background-color:#cccc00;float:left;margin-bottom:10px;line-height:40px;text-align:center;border-radius:0px 20px 20px 0px;">'+printer.yellow+'</div>';
          }
        }
        else {
          var status = 'offline';
          var statuscolor = '#f00';
        }
        content += '<div style="width:256px; height:40px;float:left;line-height:40px;text-align:center;color:'+statuscolor+';font-size:24px;">'+status+'</div>'
        content += '<a target="_blank" href="http://'+link+'"><div style="width:120px; height:40px;float:left;line-height:40px;text-align:center;border-radius:10px;background-color:#4169E1;">details</div></a>'*/
        break;
      case "free":
        var caption = 'Not in use';
        var copylink = attr.dsk;
        var avatar = 'images/' + desktype + '.png';
        var avatarcolor = $('#' + attr.id).css('background-color');
        content = attr.title + '<br />'+ attr.mail + '<br />'+ attr.phone + '<br />' + attr.mobil + '<br />'
        break;
      case "occupied":
        var caption = attr.empl;
        var copylink = attr.empl;
        var avatar = avatarUrl(attr.avtr, attr.hasavatar);
        var avatarcolor = $('#' + attr.id).css('background-color');
        content = attr.title + '<br />'+ attr.mail + '<br />'+ attr.phone + '<br />' + attr.mobil + '<br />'
        break;
      case "occupiedldap":
        var caption = attr.fname + ' ' + attr.lname;
        var copylink = attr.fname + ' ' + attr.lname;
        var avatar = avatarUrl(attr.avtr, attr.hasavatar);
        var avatarcolor = $('#' + attr.id).css('background-color');
        content = attr.title + '<br />'+ attr.mail + '<br />'+ attr.phone + '<br />' + attr.mobil + '<br />'
        break;
      case "shareddesk":
        var caption = attr.fname + ' ' + attr.lname;
        var copylink = attr.dsk;
        var avatar = avatarUrl(attr.avtr, attr.hasavatar);
        var avatarcolor = $('#' + attr.id).css('background-color');
        var multiresult = result_old.desks.filter(element => element.id == attr.id);
        content += '<div style="width:105px;height:100%;float:left;">';
        for (var m = 0; m < multiresult.length; m++) {
          var bgcolor = '#0564C8';
          // Check if search has been used and colorize names
          if (searchtext != '') {
            var searchArr = searchtext.split('|');
            for (var s = 0; s < searchArr.length; s++) {
              var namecheck = multiresult[m].fname.toLowerCase()+' '+multiresult[m].lname.toLowerCase()
              var searchcheck = searchArr[s];
              searchcheck = searchcheck.toLowerCase();
              var sresult = namecheck.includes(searchcheck);
              if (sresult) {bgcolor='#FF7F00'}
            }  
          }
          
          // Output namebuttons
          if (m == 0) {
            // Highlight first entry
            content += '<div class="shareddeskname" id="shared'+m+'" style="background: '+bgcolor+'; border:1px solid white;" onclick="showSharedSelection('
          }
          else { 
            content += '<div class="shareddeskname" id="shared'+m+'" style="background: '+bgcolor+'; border:1px solid transparent;" onclick="showSharedSelection('
          }
          content += '\''+multiresult[m].fname+'\','
          content += '\''+multiresult[m].lname+'\','
          content += '\''+multiresult[m].title+'\','
          content += '\''+multiresult[m].mail+'\','
          content += '\''+multiresult[m].phone+'\','
          content += '\''+multiresult[m].mobil+'\','
          content += '\''+multiresult[m].avtr+'\','
          content += '\''+multiresult[m].color+'\','
          content += '\'shared'+m+'\''
          content += ')" onmouseover="showSharedSelection('
          content += '\''+multiresult[m].fname+'\','
          content += '\''+multiresult[m].lname+'\','
          content += '\''+multiresult[m].title+'\','
          content += '\''+multiresult[m].mail+'\','
          content += '\''+multiresult[m].phone+'\','
          content += '\''+multiresult[m].mobil+'\','
          content += '\''+multiresult[m].avtr+'\','
          content += '\''+multiresult[m].color+'\','
          content += '\'shared'+m+'\''
          content += ')">'+multiresult[m].fname+'</div>';  
        }
        content += '</div><div id="stickytext" style="margin-left:10px;width:260px;height:100%;float:left;">'+attr.title + '<br />'+ attr.mail + '<br />'+ attr.phone + '<br />' + attr.mobil + '<br />'+'</div>';
        break;
      default:
        // Admin-defined custom item types: a simple labelled sticky.
        if (desktype && desktype.indexOf('custom_') === 0) {
          var ctSt = (typeof customItemTypes !== 'undefined' && customItemTypes[desktype.slice(7)]) ? customItemTypes[desktype.slice(7)] : null;
          var caption = attr.dsk || (ctSt ? ctSt.label : 'Item');
          var copylink = caption;
          var avatar = (ctSt && ctSt.icon) ? ctSt.icon : 'images/noavatar.png';
          var avatarcolor = $('#' + attr.id).css('background-color');
          content = (ctSt && ctSt.description) ? ctSt.description : '';
        }
        break;
    }
    if (attr.color != '') {
      var color = attr.color;
    }
    else {
      var color = '#999';
    }
    // Robin live-occupancy overlay: keep the standard grey header and show a
    // badge next to the copy icon instead of tinting the whole plate.
    var robinBadge = '';
    if (attr.robin == '1') {
      robinBadge = '<span class="robinbadge">Robin</span>';
    }
    avatarcolor = avatarcolor.replace(/[^,]+(?=\))/, '1.0');
    deskidstring = "'" + deskid + "'";
    desktypestring = "'" + desktype + "'";
    copylink_full = 'https://'+window.location.hostname+root+'?map='+map+'&findme='+copylink;
    copylink_full = encodeURI(copylink_full);
    //console.log(copylink_full);
    // Footer action buttons: copy link, an optional report button (only when a
    // report URL is configured in the admin panel) and the Robin badge. They
    // share one flex row so the layout reflows cleanly when buttons are absent.
    var copyBtn = '<img class="nameplate_action" src="images/copy.png" title="Copy link" onclick="copyToClipboard(\''+copylink_full+'\')" />';
    var reportBtn = '';
    if (typeof reportURL !== 'undefined' && reportURL) {
      reportBtn = '<img class="nameplate_action" src="images/report.png" title="Report" onclick="openReportURL()" />';
    }
    if (attr.y < 100) {
      var y_nameplate = 100;
    }
    else {
      var y_nameplate = attr.y; 
    }
    if (attr.x > (targetScreenWidth/2)) {
      var outputnameplate='<div class="leftnameplate" style="position: absolute; z-index: 102; top:' + (y_nameplate-100) +'px;left:' + (attr.x-650) + 'px;">'
                          + '<div class="leftnameplate_top" id="stickytitle" style="background:' + color
                          + '">' + caption + '</div>'
                          + '<div class="leftnameplate_textbox" style="'+boxstyle+'" id="textbox' + attr.id +'">' + content +'</div>'
                          + '<img src="' + avatar + '" class="leftnameplate_avatar" id="stickyavatar" style="background:' + avatarcolor + '" onerror="this.src=\'images/noavatar.png\'">'
                          + '<img src="images/close2.png" class="leftnameplate_close" onclick=hideSticky() />'
                          + '<div class="leftnameplate_number">' + attr.dsk + '</div>'
                          + '<div class="nameplate_actions">' + copyBtn + reportBtn + robinBadge + '</div>'
                          + '</div>'
                          + '<div id="caption' + attr.id + '" class="caption">' + attr.empl + '</div>'
                          + '</div>'
                          + '<div style="position:absolute; left:' + (attr.x-5) +'px; top:' + (attr.y-5) + 'px; width: 10px; height: 10px; border-radius:50%;background-color: black;z-index:101;"></div>'
                          + '<div id="line" style="position:absolute; left:' + (attr.x-150) +'px; top:' + (attr.y-1) + 'px; width: 150px; height: 2px; background-color: black;z-index:101;"></div>';
      }
      else {
      var outputnameplate='<div class="rightnameplate" style="position: absolute; z-index: 102; top:' + (y_nameplate-100) +'px;left:' + (Number(attr.x) + 50) + 'px;">'
                          + '<div class="rightnameplate_top" id="stickytitle" style="background:' + color
                          + '">' + caption + '</div>'
                          + '<div class="rightnameplate_textbox" style="'+boxstyle+'" id="textbox' + attr.id +'">' + content +'</div>'
                          + '<img src="' + avatar + '" class="rightnameplate_avatar" id="stickyavatar" style="background:' + avatarcolor + '" onerror="this.src=\'images/noavatar.png\'">'
                          + '<img src="images/close2.png" class="rightnameplate_close" onclick=hideSticky() />'
                          + '<div class="rightnameplate_number">' + attr.dsk + '</div>'
                          + '<div class="nameplate_actions">' + copyBtn + reportBtn + robinBadge + '</div>'
                          + '</div>'
                          + '<div id="caption' + attr.id + '" class="caption">' + attr.empl + '</div>'
                          + '</div>'
                          + '<div style="position:absolute; left:' + (attr.x-5) +'px; top:' + (attr.y-5) + 'px; width: 10px; height: 10px; border-radius:50%;background-color: black;z-index:101;"></div>'
                          + '<div style="position:absolute; left:' + attr.x +'px; top:' + (attr.y-1) + 'px; width: 150px; height: 2px; background-color: black;z-index:101;"></div>';
      }
      if (typeof token !== 'undefined' && setting_usermode == 'edit') {
        if (attr.x > (targetScreenWidth/2)) { 
          outputnameplate+='<div class="nameplate_edit" style="top:' + (Number(y_nameplate)+99) +'px;left:' + (attr.x-640) + 'px;">'
        }
        else {
          outputnameplate+='<div class="nameplate_edit" style="top:' + (Number(y_nameplate)+99) +'px;left:' + (Number(attr.x) + 150) + 'px;">'
        }
        outputnameplate+='<form class="updateItem" style="width:80%; height: 100%;margin-left:10%;">'
                          + '<select id="selDesktype" onchange="addInputfields(' + deskidstring + ',' + desktypestring + ')">'
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
                          + '</select><div id="inputfields"></div><input type="submit" Value="Apply changes"></form>'
                          + '<form class="deleteItem" style="width:80%; height: 100%;margin-left:10%;margin-bottom:10px;">'
                          + '<input type="submit" style="background-color:#f00" value="Delete item">'
                          + '<input type="hidden" name="apimap" value="' + attr.map +'">'
                          + '<input type="hidden" id="apideskid" name="apideskid" value="'+ attr.id +'">'
                          + '</form>'
                          + '</div>';
      }
    // Remove old sticky if exists
    var element = document.getElementById('stickynameplate');
    if (element !== null) {
      element.parentNode.removeChild(element);
    }

    // Adds sticky to the document
    var p = document.getElementById('deskitems');
    var newElement = document.createElement('div');
    newElement.setAttribute('id', 'stickynameplate');
    newElement.innerHTML = outputnameplate;
    p.appendChild(newElement);
    if (typeof token !== 'undefined' && setting_usermode == 'edit') {
      addInputfields(deskid, desktype, 1);
    }

    $('.updateItem').on('submit', function (e) {
      e.preventDefault();
      mapname = map;
      itemid = $("#apideskid").val();
      itemdesktype = $("#apidesktype").val();
      itemx = $("#apideskx").val();
      itemy = $("#apidesky").val();
      itemdsk = $("#apideskdsk").val();
      itemempl = $("#apideskempl").val();
      itemavtr = $("#apideskavtr").val();
      itemdept = $("#apideskdept").val();
      if (itemdept == "- none -") {itemdept = 'NULL';}
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
          alert('Could not update desk');
        }
      });
      hideSticky();
    });
    $('.deleteItem').on('submit', function (e) {
      e.preventDefault();
      mapname = map;
      itemid = $("#apideskid").val();
      $.ajax({
        url: 'rest/update',
        async: true, 
        type: 'get',
        data: {token: token, mode: 'delete', map: mapname, id: itemid, user: username},
        dataType: 'JSON',
        success: function(result){
          updateDesks();
        },
        error: function (result) {
          alert('Could not delete desk');
          console.log(result);
        }    
      });
      hideSticky();
    });
}

function showSharedSelection(fname, lname, title, mail, phone, mobil, avatar, color, sharedindex) {
  // replace title
  var outputtitle = fname+' '+lname;
  document.getElementById('stickytitle').innerHTML = outputtitle;
  if (color == '') {color = '#999'}
  $('#stickytitle').css('background-color',color);
  // replace information 
  var outputinfo = title+'<br/>'+mail+'<br/>'+phone+'<br/>'+mobil;
  document.getElementById('stickytext').innerHTML = outputinfo;
  // replace avatar
  var outputavatar = 'avatarcache/'+avatar+'.jpg';
  document.getElementById("stickyavatar").src=outputavatar;
  // replace indicator
  $('.shareddeskname').css('border','1px solid transparent');
  $('#' + sharedindex).css('border','1px solid white');
}

function getMeetingStatus(force) {
    mapname = map;
    $.ajax({
      url: 'rest/meeting?map=' + mapname + '&usecache=true',
      async: true, 
      type: 'get',
      dataType: 'JSON',
      success: function(result){
        var apirooms = result.rooms;
        var maprooms = result_old.desks.filter(element => element.desktype =="meeting");
        if(JSON.stringify(apirooms) != JSON.stringify(meetingstatus) || force==true) {
          meetingstatus = apirooms;
          console.log('[Meeting] new data - updating map');
          // Iterate API rooms
          $.each( apirooms, function( i, apiroom ){
            // Compare them to Maprooms
            $.each( maprooms, function( t, maproom ){
              if (apiroom.name == maproom.dsk) {
                //console.log ('[Match] '+apiroom.name+': '+apiroom.availability);  
                if (setting_noanimation == 1) {
                  showMeetingStatus(maproom.id, apiroom.availability, false)
                }          
                else {
                  showMeetingStatus(maproom.id, apiroom.availability, true)
                }
              }
            });
          });
        }
        else {
          console.log('[Meeting] up-to-date');
        }
      }
    });
}

function showMeetingStatus(itemid, status, animated) {
  var desk = result_old.desks.filter(element => element.id == itemid);
  var pulse = desk[0];

  switch (status) {
    case "available":
      var color = 'rgba(0, 255, 0, 0.5)';
      var animation = 'green-pulse 2s infinite';
      break;
    case "booked":
    case "in_use":
      var color = 'rgba(0, 187, 255, 0.5)';
      var animation = 'blue-pulse 2s infinite';
      break;
    default:
      var color = 'transparent';
      var animation = 'none';
      break
  }

  switch (animated) {
    case true:
      // Remove old pulse if exists
      var element = document.getElementById('pulse'+pulse.id);
      if (element !== null) {
        element.parentNode.removeChild(element);
      }

      // Adds pulse to the meeting room
      var p = document.getElementById('meetingitems');
      var newElement = document.createElement('div');
      newElement.setAttribute('id', 'pulse'+pulse.id);
      p.appendChild(newElement);
      $('#pulse' + pulse.id).css('position','absolute');
      $('#pulse' + pulse.id).css('left',(pulse.x-(25*itemscale)) + 'px');
      $('#pulse' + pulse.id).css('top',(pulse.y-(25*itemscale)) + 'px');
      $('#pulse' + pulse.id).css('width',(50*itemscale)+'px');
      $('#pulse' + pulse.id).css('height',(50*itemscale)+'px');
      $('#pulse' + pulse.id).css('border-radius','50%');
      $('#pulse' + pulse.id).css('animation',animation);
      break;
    case false:
      $('#meeting'+pulse.id).css('background-color',color);
  }
}

function pulsateTeamResults() {

  if (setting_noanimation == 1) {
    // do nothing
  }          
  else {
    //var animation = 'orange-teampulse 5s infinite';
    var animation = 'orange-teampulse 3s 1';

    // Remove old pulse if exists
    removePulsateTeams()

    // Adds pulse to the team button
    var p = document.getElementById('control_content');
    var newElement = document.createElement('div');
    newElement.setAttribute('id', 'pulseteamresult');
    p.appendChild(newElement);
    $('#pulseteamresult').css('position','absolute');
    $('#pulseteamresult').css('pointer-events','none');
    //$('#pulseteamresult').css('z-index','300');
    $('#pulseteamresult').css('right','5px');
    $('#pulseteamresult').css('top','13px');
    $('#pulseteamresult').css('width','43px');
    $('#pulseteamresult').css('height','43px');
    $('#pulseteamresult').css('border-radius','50%');
    $('#pulseteamresult').css('animation',animation);
    setTimeout(removePulsateTeams,5000);
  }
  
}

function removePulsateTeams() {
  // Remove old pulse if exists
  var element = document.getElementById('pulseteamresult');
  if (element !== null) {
    element.parentNode.removeChild(element);
  }
}

function updateOverview() {
    $.ajax({
      url: 'rest/config?mode=maps',
      async: true, 
      type: 'get',
      dataType: 'JSON',
      success: function(result){
        var allmaps = result.maps;
        var mapout = '';
        var scriptout = '<script>';
        // Projection used to place locations that have lat/lon but no stored
        // X/Y onto the classic background (assumed geographic). See
        // worldProjection for the shared formula.
        var proj = worldProjection();
        for (var i = 0; i < allmaps.length; i++) {
          var onemap = allmaps[i];
          if (onemap.mapname != 'overview' && onemap.published == 'yes') {
            // Use the stored pixel position; if it is missing (0,0) but the
            // location has lat/lon, derive an approximate X/Y from the geo
            // coordinates so the marker still appears.
            var fx = Number(onemap.x), fy = Number(onemap.y);
            if ((!isFinite(fx) || !isFinite(fy) || (fx === 0 && fy === 0))) {
              var flat = Number(onemap.lat), flon = Number(onemap.lon);
              if (isFinite(flat) && isFinite(flon) && (flat !== 0 || flon !== 0)) {
                var fxy = proj.toXY(flat, flon);
                fx = Math.round(fxy.x); fy = Math.round(fxy.y);
              }
            }
            onemap.x = fx;
            onemap.y = fy;
            mapout += '' //<a href="'+root+'?map='+onemap.mapname+'" id="link_'+onemap.mapname+'">
                  + '<div class="mapflag" id="mapflag_'+onemap.mapname+'" style="left: '+fx+'px; top: '+fy+'px;'
                  + 'width:30px; height:30px; background-image: url(countryflags/'+onemap.country+'.svg);" '
                  + 'onclick="showMapplate(\''+onemap.mapname+'\')">'
                  + '<div style="position:relative; height:100%; text-align: center;color:white;">'
                  + '<span style="line-height:30px;font-size:4.8px; background: rgba(50,50,50,0.8);">'+(onemap.displayname ? onemap.displayname : ucWords(onemap.mapname))+'</span>'
                  + '</div>'
                  + '</div>';
                  //+ '<div class="mapflag_results" id="mapresults_'+onemap.mapname+'"></div>  </div>';
          } 
        }
        var p = document.getElementById('content');
        var newElement = document.createElement('div');
        newElement.setAttribute('id', 'mapOverview');
        newElement.innerHTML = mapout;
        p.appendChild(newElement);
        overviewmaps = allmaps;
        if ($('#searchtext').val() != '') {$("#search_button").click()}
      }
    });
}

// Cached Natural Earth country geometry (loaded once per page).
var worldGeo = null;

// worldProjection is the single source of truth for converting between the
// equirectangular world map and screen pixels. Both overview renderers
// (updateWorldMap and updateOverview) and the classic<->modern coordinate
// derivation use it so the map background, the markers and any back/forward
// projected coordinates always stay in agreement.
//   forward:  {lat,lon} -> {x,y}px
//   inverse:  {x,y}px   -> {lat,lon}
// The map is cropped to lon -180..180 and lat 90..-60 (Antarctica removed),
// drawn at targetScreenWidth with a 2.4:1 base aspect ratio, vertically
// exaggerated by vStretch and offset topOffset px down to match index.html.
function worldProjection() {
  var latTop = 90, latBottom = -60, vStretch = 1.3, topOffset = 20;
  var W = targetScreenWidth;
  var H = W * (latTop - latBottom) / 360 * vStretch;
  return {
    latTop: latTop, latBottom: latBottom, topOffset: topOffset, W: W, H: H,
    toXY: function (lat, lon) {
      return {
        x: (lon + 180) / 360 * W,
        y: (latTop - lat) / (latTop - latBottom) * H + topOffset
      };
    },
    toLatLon: function (x, y) {
      return {
        lon: x / W * 360 - 180,
        lat: latTop - (y - topOffset) / H * (latTop - latBottom)
      };
    }
  };
}

// loadWorldGeo fetches the country polygons (GeoJSON) and caches them, then runs
// the callback. The data is the Natural Earth 1:110m admin-0 countries set, a
// small (~250 KB) low-detail vector of every country as [lon,lat] polygons.
function loadWorldGeo(cb) {
  if (worldGeo) { cb(); return; }
  $.ajax({
    url: 'worldmap.geojson',
    async: true,
    type: 'get',
    dataType: 'JSON',
    success: function(result) { worldGeo = result; cb(); },
    error: function() { worldGeo = { features: [] }; cb(); }
  });
}

// renderWorldCountries projects every country polygon to screen pixels with the
// same equirectangular projection used for the location markers and returns an
// <svg> string of one <path> per country. Each path carries data-iso/data-name
// so individual countries can be styled or coloured (e.g. on hover) — the whole
// reason for rendering the map from raw data instead of a flat PNG.
function renderWorldCountries(W, H, topOffset, latTop, latBottom) {
  function projX(lon) { return (lon + 180) / 360 * W; }
  function projY(lat) { return (latTop - lat) / (latTop - latBottom) * H + topOffset; }
  function ringPath(ring) {
    var d = '';
    for (var i = 0; i < ring.length; i++) {
      var lon = ring[i][0], lat = ring[i][1];
      // Clamp to the cropped southern edge so the few southern tips that dip
      // just below the visible area don't stretch the shape.
      if (lat < latBottom) { lat = latBottom; }
      d += (i === 0 ? 'M' : 'L') + projX(lon).toFixed(1) + ',' + projY(lat).toFixed(1);
    }
    return d + 'Z';
  }
  var paths = '';
  var maskPaths = '';
  var feats = (worldGeo && worldGeo.features) ? worldGeo.features : [];
  for (var f = 0; f < feats.length; f++) {
    var feat = feats[f];
    var props = feat.properties || {};
    var name = props.NAME || props.name || '';
    // Antarctica is below the cropped southern edge; skip it like the old map.
    if (name === 'Antarctica') { continue; }
    var geom = feat.geometry;
    if (!geom) { continue; }
    var polys = geom.type === 'Polygon' ? [geom.coordinates]
              : geom.type === 'MultiPolygon' ? geom.coordinates : [];
    var d = '';
    for (var p = 0; p < polys.length; p++) {
      for (var r = 0; r < polys[p].length; r++) {
        d += ringPath(polys[p][r]);
      }
    }
    if (!d) { continue; }
    var iso = (props.ISO_A2 || props.iso_a2 || '').toLowerCase();
    paths += '<path class="worldcountry" data-iso="' + iso + '" '
           + 'data-name="' + name.replace(/"/g, '') + '" d="' + d + '"></path>';
    // Same geometry for the border mask: filled white (land kept) and stroked
    // black (border pixels punched out to full transparency).
    maskPaths += '<path d="' + d + '"></path>';
  }
  var svgH = topOffset + H;
  // A mask renders the land white (visible) but strokes every country outline in
  // black, so the shared borders become genuinely transparent (alpha 0) pixels
  // that let whatever is behind the map show through — exactly like the old PNG,
  // while each country stays an individually colourable <path> for hover. The
  // mask only hides border pixels, so changing a country's fill (e.g. on hover)
  // still shows through everywhere except the thin border lines.
  return '<svg class="worldcountries" width="' + W + '" height="' + svgH + '" '
       + 'viewBox="0 0 ' + W + ' ' + svgH + '" '
       + 'style="position:absolute; left:0; top:0;">'
       + '<defs><mask id="worldbordermask" maskUnits="userSpaceOnUse" '
       + 'x="0" y="0" width="' + W + '" height="' + svgH + '">'
       + '<g fill="#fff" stroke="#000" stroke-width="1" stroke-linejoin="round">'
       + maskPaths + '</g></mask></defs>'
       + '<g mask="url(#worldbordermask)">' + paths + '</g></svg>';
}

// updateWorldMap renders the overview as a real world map: each location is
// placed by its geocoded latitude/longitude (equirectangular projection) over
// the world-map background image, instead of by the static MapX/MapY pixels.
// It reuses showMapplate() for the location popup by writing the projected pixel
// position back into each map's x/y.
function updateWorldMap() {
    loadWorldGeo(function() {
    $.ajax({
      url: 'rest/config?mode=maps',
      async: true,
      type: 'get',
      dataType: 'JSON',
      success: function(result){
        var allmaps = result.maps;
        // Projection shared with the country layer, the classic overview and the
        // coordinate derivation (see worldProjection).
        var proj = worldProjection();
        var latTop = proj.latTop, latBottom = proj.latBottom;
        var W = proj.W, H = proj.H, topOffset = proj.topOffset;

        // Render the country layer (vector, from raw GeoJSON) into the overview
        // container, beneath the location markers added further below.
        var omap = document.getElementById('overviewmap');
        if (omap) { omap.innerHTML = renderWorldCountries(W, H, topOffset, latTop, latBottom); }

        // First pass: project every published location to a pixel position (the
        // exact pin spot). Locations missing lat/lon (never geocoded) but with a
        // stored X/Y are back-projected from those pixels, so the modern map can
        // still show them approximately without requiring geo data.
        var pts = [];
        for (var i = 0; i < allmaps.length; i++) {
          var onemap = allmaps[i];
          if (onemap.mapname == 'overview' || onemap.published != 'yes') { continue; }
          var lat = Number(onemap.lat);
          var lon = Number(onemap.lon);
          // Approximate missing coordinates by back-projecting the stored X/Y
          // (only meaningful when the classic background was a geographic map,
          // but harmless otherwise and editable by the admin).
          if ((!isFinite(lat) || !isFinite(lon) || (lat === 0 && lon === 0))) {
            var sx = Number(onemap.x), sy = Number(onemap.y);
            if (isFinite(sx) && isFinite(sy) && (sx !== 0 || sy !== 0)) {
              var ll = proj.toLatLon(sx, sy);
              lat = ll.lat; lon = ll.lon;
            }
          }
          // Skip locations with no usable position at all, or that fall below
          // the cropped southern edge.
          if (!isFinite(lat) || !isFinite(lon) || (lat === 0 && lon === 0) || lat < latBottom) { continue; }
          var pxy = proj.toXY(lat, lon);
          var px = pxy.x;
          var py = pxy.y;
          // The name sits on top of the flag circle; strip the "-nomap" marker
          // suffix some maps use so it never shows in the label.
          var label = (onemap.displayname ? onemap.displayname : ucWords(onemap.mapname)).replace(/-nomap/gi, '');
          // The label is overlaid on the flag, so the footprint is just the
          // circular flag itself.
          var flagSize = 66;
          var w = flagSize;
          var h = flagSize;
          pts.push({ map: onemap, ax: px, ay: py, x: px, y: py, w: w, h: h, label: label });
        }

        // --- Cluster nearby locations -------------------------------------
        // Group locations whose flag circles would actually overlap if drawn on
        // their exact spots (e.g. the European cities). The test uses only the
        // round flag footprint, NOT the (sometimes long) text label, so two
        // locations whose circles are clearly apart stay separate even if their
        // labels are wide (e.g. Shanghai / Tokyo, Mumbai / Singapore). Such
        // singletons keep their flag on the exact spot and never get a pin.
        var clusterPad = 16;
        var clusterReach = flagSize + clusterPad;
        var cluster = new Array(pts.length);
        for (var i = 0; i < pts.length; i++) { cluster[i] = i; }
        function findRoot(x) { while (cluster[x] !== x) { cluster[x] = cluster[cluster[x]]; x = cluster[x]; } return x; }
        for (var a = 0; a < pts.length; a++) {
          for (var b = a + 1; b < pts.length; b++) {
            var cdx = pts[a].ax - pts[b].ax;
            var cdy = pts[a].ay - pts[b].ay;
            // Use the true centre-to-centre distance: two flag circles only
            // overlap when their centres are closer than one flag diameter
            // (plus padding). A bounding-box test would wrongly group diagonal
            // neighbours like Shanghai / Tokyo.
            if (Math.sqrt(cdx * cdx + cdy * cdy) < clusterReach) {
              cluster[findRoot(a)] = findRoot(b);
            }
          }
        }
        var groups = {};
        for (var i = 0; i < pts.length; i++) {
          var r = findRoot(i);
          if (!groups[r]) { groups[r] = []; }
          groups[r].push(i);
        }

        // --- Fan each multi-member cluster into rows -----------------------
        // The cluster's dominant country (the one with the most locations, e.g.
        // Germany) fans into a row above the cluster; every other country
        // (Spain, Portugal, Austria, Greece...) fans into a row below. Each row
        // is ordered west-to-east so leader lines stay in geographic order.
        // Members of a fanned cluster get a pin + leader line; singletons keep
        // their flag on the exact spot with no pin.
        var bubbleGap = 96;  // horizontal spacing between bubbles in a row
        var rowGap = 130;    // vertical distance of a row from the cluster centre
        for (var key in groups) {
          var idx = groups[key];
          if (idx.length < 2) { continue; } // singletons stay on their spot
          // Cluster centre.
          var ccx = 0, ccy = 0;
          for (var j = 0; j < idx.length; j++) {
            ccx += pts[idx[j]].ax; ccy += pts[idx[j]].ay;
            pts[idx[j]].fanned = true;
          }
          ccx /= idx.length; ccy /= idx.length;

          // Find the dominant country in this cluster.
          var counts = {};
          for (var j = 0; j < idx.length; j++) {
            var c = pts[idx[j]].map.country || '';
            counts[c] = (counts[c] || 0) + 1;
          }
          var primary = '', best = -1;
          for (var c in counts) { if (counts[c] > best) { best = counts[c]; primary = c; } }

          // Upper row = dominant country, lower row = everyone else.
          var upper = [], lower = [];
          for (var j = 0; j < idx.length; j++) {
            ((pts[idx[j]].map.country || '') === primary ? upper : lower).push(idx[j]);
          }
          upper.sort(function(p, q) { return pts[p].ax - pts[q].ax; });
          lower.sort(function(p, q) { return pts[p].ax - pts[q].ax; });

          // Place a band of bubbles in an evenly-spaced horizontal row, centred
          // on the cluster. A flat row (rather than a curved arc) keeps every
          // bubble at the same height so the leader lines stay in left-to-right
          // order and don't cross.
          function placeBand(band, dirY) {
            var n = band.length;
            var spread = (n - 1) * bubbleGap;
            var startX = ccx - spread / 2;
            for (var k = 0; k < n; k++) {
              pts[band[k]].x = n === 1 ? ccx : startX + spread * k / (n - 1);
              pts[band[k]].y = ccy + dirY * rowGap;
            }
          }
          placeBand(upper, -1);
          placeBand(lower, 1);

          // Leader lines can still cross when two cities in the same row share a
          // similar longitude but differ a lot in latitude (e.g. Bremen vs
          // Stuttgart). Greedily swap adjacent same-row bubbles whenever it
          // reduces the number of crossings, which removes those cases while
          // keeping the rows broadly west-to-east.
          function segCross(p, q) {
            var a = pts[p], b = pts[q];
            function ccw(ax, ay, bx, by, cx, cy) { return (cy - ay) * (bx - ax) > (by - ay) * (cx - ax); }
            return ccw(a.ax, a.ay, b.ax, b.ay, b.x, b.y) !== ccw(a.x, a.y, b.ax, b.ay, b.x, b.y) &&
                   ccw(a.ax, a.ay, a.x, a.y, b.ax, b.ay) !== ccw(a.ax, a.ay, a.x, a.y, b.x, b.y);
          }
          function clusterCross() {
            var c = 0;
            for (var x = 0; x < idx.length; x++) {
              for (var y = x + 1; y < idx.length; y++) { if (segCross(idx[x], idx[y])) c++; }
            }
            return c;
          }
          function optimizeRow(band) {
            var improved = true, guard = 0;
            while (improved && guard++ < 50) {
              improved = false;
              for (var k = 0; k < band.length - 1; k++) {
                var before = clusterCross();
                var tmp = pts[band[k]].x; pts[band[k]].x = pts[band[k + 1]].x; pts[band[k + 1]].x = tmp;
                if (clusterCross() < before) {
                  var t = band[k]; band[k] = band[k + 1]; band[k + 1] = t; improved = true;
                } else {
                  var t2 = pts[band[k]].x; pts[band[k]].x = pts[band[k + 1]].x; pts[band[k + 1]].x = t2;
                }
              }
            }
          }
          optimizeRow(upper);
          optimizeRow(lower);
        }

        // --- Collision cleanup --------------------------------------------
        // After fanning, relax positions so nothing overlaps. Singletons stay
        // pinned to their exact spot (they act as fixed obstacles); only fanned
        // bubbles move, springing toward their row target while pairwise
        // separation pushes overlaps apart.
        var pad = 12;
        var spring = 0.05;
        var minX = 60, maxX = W - 60;
        var minY = topOffset + 40, maxY = topOffset + H + 200;
        for (var i = 0; i < pts.length; i++) { pts[i].tx = pts[i].x; pts[i].ty = pts[i].y; }
        for (var it = 0; it < 300; it++) {
          for (var i = 0; i < pts.length; i++) {
            if (!pts[i].fanned) { continue; }
            pts[i].x += (pts[i].tx - pts[i].x) * spring;
            pts[i].y += (pts[i].ty - pts[i].y) * spring;
          }
          for (var pass = 0; pass < 6; pass++) {
            for (var a = 0; a < pts.length; a++) {
              for (var b = a + 1; b < pts.length; b++) {
                var pa = pts[a], pb = pts[b];
                if (!pa.fanned && !pb.fanned) { continue; }
                var ddx = pb.x - pa.x, ddy = pb.y - pa.y;
                var ox = (pa.w + pb.w) / 2 + pad - Math.abs(ddx);
                var oy = (pa.h + pb.h) / 2 + pad - Math.abs(ddy);
                if (ox > 0 && oy > 0) {
                  // A fixed (singleton) bubble doesn't move; the fanned one
                  // takes the full push.
                  var wa = pa.fanned ? (pb.fanned ? 0.5 : 1) : 0;
                  var wb = pb.fanned ? (pa.fanned ? 0.5 : 1) : 0;
                  if (ox < oy) {
                    var sgnx = ddx === 0 ? (a < b ? -1 : 1) : (ddx < 0 ? -1 : 1);
                    pa.x -= sgnx * ox * wa; pb.x += sgnx * ox * wb;
                  } else {
                    var sgny = ddy === 0 ? (a < b ? -1 : 1) : (ddy < 0 ? -1 : 1);
                    pa.y -= sgny * oy * wa; pb.y += sgny * oy * wb;
                  }
                }
              }
            }
          }
          // Keep fanned flag footprints clear of every other location's exact
          // pin spot, so a pin is never hidden underneath a neighbouring flag.
          var pinHalf = 9;
          for (var a = 0; a < pts.length; a++) {
            if (!pts[a].fanned) { continue; }
            for (var b = 0; b < pts.length; b++) {
              if (a === b) { continue; }
              var pa = pts[a];
              var adx = pts[b].ax - pa.x, ady = pts[b].ay - pa.y;
              var aox = pa.w / 2 + pinHalf - Math.abs(adx);
              var aoy = pa.h / 2 + pinHalf - Math.abs(ady);
              if (aox > 0 && aoy > 0) {
                if (aox < aoy) {
                  pa.x += (adx < 0 ? 1 : -1) * aox;
                } else {
                  pa.y += (ady < 0 ? 1 : -1) * aoy;
                }
              }
            }
          }
          for (var i = 0; i < pts.length; i++) {
            if (!pts[i].fanned) { continue; }
            if (pts[i].x < minX) pts[i].x = minX;
            if (pts[i].x > maxX) pts[i].x = maxX;
            if (pts[i].y < minY) pts[i].y = minY;
            if (pts[i].y > maxY) pts[i].y = maxY;
          }
        }

        var lines = '';
        var markers = '';
        for (var i = 0; i < pts.length; i++) {
          var pt = pts[i];

          // Round the bubble position to whole pixels. The flag is centred with
          // translate(-50%,-50%), so a fractional left/top would land the
          // overlaid text on a sub-pixel boundary and render it blurry.
          pt.x = Math.round(pt.x);
          pt.y = Math.round(pt.y);

          // Feed the bubble position to showMapplate via x/y.
          pt.map.x = pt.x;
          pt.map.y = pt.y;

          // Only fanned bubbles (members of a dense cluster) are displaced from
          // their true spot, so only they get a small (non-clickable) pin at the
          // exact location plus a leader line. Singletons sit on their spot.
          if (pt.fanned) {
            lines += '<line x1="' + pt.ax + '" y1="' + pt.ay + '" x2="' + pt.x + '" y2="' + pt.y + '" '
                   + 'stroke="#555555" stroke-width="2" />';
            markers += '<div class="worldpin" style="left:' + pt.ax + 'px; top:' + pt.ay + 'px;"></div>';
          }

          var hasFlag = pt.map.country && pt.map.country != 'none' && pt.map.country != '';
          var flagStyle = hasFlag
            ? 'background-image:url(countryflags/' + pt.map.country + '.svg);'
            : 'background-image:none; background-color:#FF7F00;';
          markers += '<div class="worldflag" id="mapflag_' + pt.map.mapname + '" '
                  + 'style="left:' + pt.x + 'px; top:' + pt.y + 'px;" '
                  + 'onclick="showMapplate(\'' + pt.map.mapname + '\')">'
                  + '<div class="worldflag_img" style="' + flagStyle + '"></div>'
                  + '<span class="worldflag_label">' + pt.label + '</span>'
                  + '</div>';
        }

        var svgH = topOffset + H + 200;
        var mapout = '<svg class="worldlines" width="' + W + '" height="' + svgH + '" '
                   + 'style="position:absolute; left:0; top:0; pointer-events:none;">' + lines + '</svg>'
                   + markers;

        var p = document.getElementById('content');
        var newElement = document.createElement('div');
        newElement.setAttribute('id', 'mapOverview');
        newElement.innerHTML = mapout;
        p.appendChild(newElement);
        overviewmaps = allmaps;
        if ($('#searchtext').val() != '') {$("#search_button").click()}
      }
    });
    });
}

function resetColors() {
  $(".free").css("background-color","");
  $(".occupied").css("background-color","");
  $(".occupiedldap").css("background-color","");
  $(".shareddesk").css("background-color","");
  $(".blocked").css("background-color","");
  $(".hotseat").css("background-color","");
  $(".booking_booked").css("background-color","")
  $(".hotseat_booked").css("background-color","")
  $(".meeting").css("background-color","");
  $(".service").css("background-color","");
  $(".printer").css("background-color","");

  $(".food").css("background-color","");
  $(".firstaid").css("background-color","");
  $(".exit").css("background-color","");
  $(".keycardlock").css("background-color","");
  $(".keylock").css("background-color","");
  $(".restroom").css("background-color","");
  $(".shareddeskname").css("background","");

  $(".div_teamfound").css("color","white");
  $(".caption").css("visibility","hidden");
  
  document.getElementById("group_border").style.visibility = "hidden";
  document.getElementById("group_label").style.visibility = "hidden";

  document.getElementById("notifycontent").innerHTML = "<span style=\"position:relative; width:454px;height:40px;display:inline;float:left;line-height: 40px;\">&nbsp;</span>";

  document.getElementById("addressbook_img").src="images/addressbook.png";
  // Reset search box
  $('#searchtext').val('')
  // Clear any search dimming / fog spotlight from a previous search.
  clearSearchDimming();
  hideSearchFog();
}

function searchDesks() {    
  searchtext = $('#searchtext').val()
  searchtext = searchtext.trim();
  resetColors()
  hideSticky()
  // Start search
  if (searchtext) {
    searchLocalResults = []
    searchGlobalResults = []
    searchSelectedId = null
    openSearchSidebar()
    if (map != 'overview') {
      searchLocaldesks()
    }
    searchGlobaldesks()
    renderSearchSidebar()
  }
  else {
    closeSearchSidebar()
  }
  updateTeams()
}

function searchLocaldesks() {
  var localdesks = result_old.desks;
  var gotoY = 0
  $.each( localdesks, function( t, localdesk ){
    var namecheck = (localdesk.fname+' '+localdesk.lname+','+localdesk.desktype+','+localdesk.dsk+','+localdesk.empl).toLowerCase()
    if (searchtext != '') {
      var searchArr = searchtext.split('|');
      for (var s = 0; s < searchArr.length; s++) {
        var searchcheck = searchArr[s];
        searchcheck = searchcheck.toLowerCase();
        var sresult = namecheck.includes(searchcheck);
        if (localdesk.booked == 1) {
          // check for fullnames on booked desks
          namecheck += ','+(localdesk.bookdata.name).toLowerCase()
          sresult = namecheck.includes(searchcheck);
        }
        if (sresult) {
          // output local results
          $('#'+localdesk.id).css('background-color','rgba(255, 127, 0, 1)')
          // collect for the search sidebar (dedupe by desk id)
          if (!searchLocalResults.some(function(x){ return x.id == localdesk.id; })) {
            var sbname = (localdesk.fname+' '+localdesk.lname).trim();
            if (sbname == '' && localdesk.booked == 1 && localdesk.bookdata) {
              sbname = localdesk.bookdata.name;
            }
            // Robin-occupied desks carry the resolved person in empl (no
            // fname/lname), same as the map nameplate — use it as the label.
            if (sbname == '' && localdesk.empl) {
              sbname = localdesk.empl;
            }
            searchLocalResults.push({
              id: localdesk.id,
              name: sbname,
              dsk: localdesk.dsk,
              empl: localdesk.empl,
              title: localdesk.title,
              desktype: localdesk.desktype,
              avtr: localdesk.avtr,
              hasavatar: localdesk.hasavatar,
              x: Number(localdesk.x),
              y: Number(localdesk.y)
            });
          }
          // show labels if no teamsearch has been triggered
          if (typeof teamlabel == 'undefined') {
            $('#caption'+localdesk.id).attr("style", "visibility: visible")
          }
          // check if teamsearch is empty, else suppress labels for this run
          else {
            if (teamlabel == '') {
              $('#caption'+localdesk.id).attr("style", "visibility: visible")
            }
          }
          // get first result to scroll to
          if (gotoY == 0) {
            gotoY = localdesk.y
          }
          else {
            if (Number(localdesk.y) < Number(gotoY)) {
              gotoY = localdesk.y
            }
            else {
              //console.log('no update')
            }
          } 
          continue
        }
      }  
    }
  });
  // Allow suppresion of labels only once
  if (typeof teamlabel !== 'undefined') {
    teamlabel = '';
    window.history.replaceState({}, document.title, root);
  }
  // scroll to first result
  if (gotoY != 0) {
    window.scrollTo(0, (gotoY-150)*autozoom*zoom)
  }
  // Dim non-matching desks so the results stand out on the map.
  applySearchDimming();
}

function searchGlobaldesks() {
  $.ajax({
    url: 'rest/desks?search='+searchtext,
    async: true, 
    type: 'get',
    dataType: 'JSON',
    success: function(result){
      mapname = map;
      var globalmaps = []
      var globalresults = []
      for (var i = 0; i < result.desks.length; i++) {
        var counter = result.desks[i];
        if (counter.map == mapname) {
          // do nothing
        } 
        // count global results too
        else {
          // Check if map is already in array
          var checkarr = $.inArray(counter.map, globalmaps)
          if ( checkarr == '-1') {
            globalmaps.push(counter.map)
            globalresults.push(1)
          }
          else {
            globalresults[checkarr]++
          }
        }
      }    
      // Feed the search sidebar with the other-location results
      searchGlobalResults = []
      for (var h = 0; h < globalmaps.length; h++) {
        searchGlobalResults.push({ map: globalmaps[h], count: globalresults[h] })
      }
      renderSearchSidebar()
    }
  });
}

// --- Search results sidebar (Google-Maps-style) ---

function openSearchSidebar() {
  var sidebar = document.getElementById('searchsidebar');
  if (!sidebar) { return; }
  if (searchSidebarWidth === SEARCH_SIDEBAR_WIDTH) { return; }
  searchSidebarWidth = SEARCH_SIDEBAR_WIDTH;
  sidebar.classList.add('open');
  // scalePages() scales/re-anchors the sidebar and re-shifts the map.
  if (typeof window.cmapsRescale === 'function') { window.cmapsRescale(); }
}

function closeSearchSidebar() {
  var sidebar = document.getElementById('searchsidebar');
  if (sidebar) { sidebar.classList.remove('open'); }
  if (searchSidebarWidth === 0) { return; }
  searchSidebarWidth = 0;
  if (typeof window.cmapsRescale === 'function') { window.cmapsRescale(); }
}

function renderSearchSidebar() {
  var inner = document.getElementById('searchsidebar_inner');
  if (!inner) { return; }
  inner.innerHTML = '';

  // Current map results first
  if (map != 'overview') {
    var lh = document.createElement('div');
    lh.className = 'searchsidebar_section';
    lh.textContent = 'On this map (' + searchLocalResults.length + ')';
    inner.appendChild(lh);
    if (searchLocalResults.length === 0) {
      var none = document.createElement('div');
      none.className = 'searchsidebar_empty';
      none.textContent = 'No matches on this map';
      inner.appendChild(none);
    }
    else {
      // Sort by position on the map: top-to-bottom (y), then left-to-right (x).
      searchLocalResults.sort(function (a, b) {
        if (a.y !== b.y) { return a.y - b.y; }
        return a.x - b.x;
      });
      for (var i = 0; i < searchLocalResults.length; i++) {
        inner.appendChild(buildSidebarLocalRow(searchLocalResults[i]));
      }
    }
  }

  // Other locations
  if (searchGlobalResults.length > 0) {
    var gh = document.createElement('div');
    gh.className = 'searchsidebar_section';
    gh.textContent = 'Other locations';
    inner.appendChild(gh);
    for (var j = 0; j < searchGlobalResults.length; j++) {
      inner.appendChild(buildSidebarGlobalRow(searchGlobalResults[j]));
    }
  }
}

function buildSidebarLocalRow(r) {
  var row = document.createElement('div');
  row.className = 'searchsidebar_row';
  row.setAttribute('data-deskid', r.id);
  row.onclick = function () { selectSearchResult(r.id, row); };
  // Hovering a row previews that result on the map (isolates it), reverting
  // when the cursor leaves. A persistent click selection takes precedence.
  row.onmouseenter = function () { hoverSearchResult(r.id); };
  row.onmouseleave = function () { unhoverSearchResult(); };

  // Desk types that represent a real person (show their avatar). Everything else
  // is a facility/marker that uses its own icon image instead.
  var personTypes = ['addesk', 'localdesk', 'shareddesk', 'occupied', 'occupiedldap', 'hotseat', 'booking', 'booking_booked', 'hotseat_booked'];
  // Background colours for facility icons, matching the on-map item colours.
  var itemBg = {
    meeting: 'rgba(137, 26, 183, 0.7)',
    printer: 'rgba(50,50,50,0.7)',
    firstaid: 'rgba(220,50,50,0.7)',
    restroom: 'rgba(78, 81, 100, 0.7)',
    food: 'rgba(215, 125, 40, 0.7)',
    service: 'rgba(70, 190, 190, 0.7)',
    exit: 'rgba(84, 185, 72, 0.7)',
    keycardlock: 'rgba(240, 220, 0, 0.7)',
    keylock: 'rgba(240, 220, 0, 0.7)',
    floor: '#d017a8b3',
    blocked: 'rgba(180, 180, 180, 0.7)'
  };
  var isPerson = personTypes.indexOf(r.desktype) !== -1;

  // Meeting room live status (current event + busy colour), looked up by desk id.
  // meetingSynced is false when the room has no status entry (not synced).
  var meetingNow = null, meetingBusy = false, meetingSynced = false;
  if (r.desktype === 'meeting') {
    var rs = (meetingstatus || []).filter(function (e) { return e.deskid == r.id; });
    if (rs.length) {
      meetingSynced = true;
      var av = rs[0].availability;
      meetingBusy = (av === 'booked' || av === 'in_use');
      if (meetingBusy && rs[0].now_title) { meetingNow = rs[0].now_title; }
    }
  }

  var img = document.createElement('img');
  img.className = 'searchsidebar_avatar';
  if (isPerson) {
    img.src = avatarUrl(r.avtr, r.hasavatar);
    img.onerror = function () { this.onerror = null; this.src = 'images/noavatar.png'; };
  }
  else {
    // Facility/marker: use its own icon, fitted inside the round badge. The
    // badge always uses the item's default colour, regardless of state.
    img.src = 'images/' + r.desktype + '.png';
    img.style.objectFit = 'contain';
    img.style.padding = '6px';
    img.style.boxSizing = 'border-box';
    if (itemBg[r.desktype]) {
      img.style.background = itemBg[r.desktype];
    }
  }
  row.appendChild(img);

  var txt = document.createElement('div');
  txt.className = 'searchsidebar_text';
  var nm = document.createElement('div');
  nm.className = 'searchsidebar_name';
  nm.textContent = r.name || (r.dsk ? r.dsk : '\u2014');
  var sub = document.createElement('div');
  sub.className = 'searchsidebar_sub';
  // Subtitle:
  //  - Meeting rooms: current event title, or "free" when not in use.
  //  - People (desk types): job title, falling back to the desk name.
  //  - Facilities (printer, ...): description (empl), falling back to the type.
  if (r.desktype === 'meeting') {
    if (!meetingSynced) {
      // Room is not synced: report no information and leave the subtitle uncoloured.
      sub.textContent = 'no information provided';
    } else {
      sub.textContent = meetingNow ? meetingNow : 'free';
      // Colour the subtitle with the meeting-room status colour: the pulse blue
      // when in use, the available green when free.
      sub.style.color = meetingBusy ? 'rgb(0, 187, 255)' : 'rgb(0, 210, 0)';
    }
  }
  else if (isPerson) {
    sub.textContent = (r.title && r.title.trim() !== '') ? r.title : r.dsk;
  }
  else {
    sub.textContent = (r.empl && r.empl.trim() !== '') ? r.empl : r.desktype;
  }
  txt.appendChild(nm);
  txt.appendChild(sub);
  row.appendChild(txt);
  return row;
}

function buildSidebarGlobalRow(g) {
  var row = document.createElement('div');
  row.className = 'searchsidebar_row';
  row.onclick = function () {
    window.location.href = root + '?map=' + encodeURIComponent(g.map) + '&findme=' + encodeURIComponent(searchtext);
  };

  var icon = document.createElement('div');
  icon.className = 'searchsidebar_mapicon';
  icon.textContent = g.count;
  row.appendChild(icon);

  var txt = document.createElement('div');
  txt.className = 'searchsidebar_text';
  var nm = document.createElement('div');
  nm.className = 'searchsidebar_name';
  nm.textContent = g.map.charAt(0).toUpperCase() + g.map.substr(1).toLowerCase();
  var sub = document.createElement('div');
  sub.className = 'searchsidebar_sub';
  sub.textContent = g.count + (g.count == 1 ? ' result' : ' results');
  txt.appendChild(nm);
  txt.appendChild(sub);
  row.appendChild(txt);
  return row;
}

function selectSearchResult(id, row) {
  var inner = document.getElementById('searchsidebar_inner');
  // Toggle: clicking the already-active result restores all matches.
  if (searchSelectedId == id) {
    searchSelectedId = null;
    if (inner) {
      var allrows = inner.querySelectorAll('.searchsidebar_row');
      for (var a = 0; a < allrows.length; a++) { allrows[a].classList.remove('selected'); }
    }
    showAllSearchResultsOnMap();
    hideSearchFog();
    return;
  }
  // Switch selection: highlight just this row in the list.
  searchSelectedId = id;
  if (inner) {
    var rows = inner.querySelectorAll('.searchsidebar_row');
    for (var i = 0; i < rows.length; i++) {
      if (rows[i] === row) { rows[i].classList.add('selected'); }
      else { rows[i].classList.remove('selected'); }
    }
  }
  // On the map: hide every other match, keep only this desk orange + caption.
  isolateDeskOnMap(id);
  jumpToDesk(id);
  showSearchFog(id);
}

// Re-apply the orange highlight + caption to all current local matches.
function showAllSearchResultsOnMap() {
  for (var k = 0; k < searchLocalResults.length; k++) {
    var oid = searchLocalResults[k].id;
    $('#' + oid).css('background-color', 'rgba(255, 127, 0, 1)');
    $('#caption' + oid).attr('style', 'visibility: visible');
  }
  // Dim everything that is not a match so the results stand out.
  applySearchDimming();
}

// Dim all deskballs that are not current search matches (grey + faded), and
// restore matches to full strength. Re-applied after each desk refresh because
// the desk list is re-rendered when its data changes.
function applySearchDimming() {
  var matchIds = {};
  for (var i = 0; i < searchLocalResults.length; i++) { matchIds[searchLocalResults[i].id] = true; }
  if (searchLocalResults.length === 0) { clearSearchDimming(); return; }
  var balls = document.querySelectorAll('#deskitems .deskball');
  for (var b = 0; b < balls.length; b++) {
    if (matchIds[balls[b].id]) { balls[b].classList.remove('searchdim'); }
    else { balls[b].classList.add('searchdim'); }
  }
}

// Remove the search dimming from every desk.
function clearSearchDimming() {
  var balls = document.querySelectorAll('#deskitems .deskball.searchdim');
  for (var b = 0; b < balls.length; b++) { balls[b].classList.remove('searchdim'); }
}

// On the map: keep only the given desk highlighted (orange + caption) and hide
// every other match. Does not change the persistent click selection.
function isolateDeskOnMap(id) {
  for (var k = 0; k < searchLocalResults.length; k++) {
    var oid = searchLocalResults[k].id;
    if (oid != id) {
      $('#' + oid).css('background-color', '');
      $('#caption' + oid).attr('style', 'visibility: hidden');
    }
  }
  $('#' + id).css('background-color', 'rgba(255, 127, 0, 1)');
  $('#caption' + id).attr('style', 'visibility: visible');
}

// Hover preview: isolate the hovered desk without touching the click selection,
// and pan the map to it just like a click does (but without the pulse, so it
// doesn't fire repeatedly while scanning down the result list).
function hoverSearchResult(id) {
  isolateDeskOnMap(id);
  jumpToDesk(id, true);
  showSearchFog(id);
}

// Mouse leaves a row: restore the persistent state — keep the clicked result
// isolated if one is selected, otherwise show all matches again.
function unhoverSearchResult() {
  if (searchSelectedId != null) {
    isolateDeskOnMap(searchSelectedId);
    showSearchFog(searchSelectedId);
  } else {
    showAllSearchResultsOnMap();
    hideSearchFog();
  }
}

// --- Search fog spotlight ---------------------------------------------------
// A dark veil over the map with a single round hole over the focused desk, to
// guide the eye. The veil lives INSIDE the map content layer (#content) so it
// scrolls and zooms together with the desks automatically — no scroll/resize
// tracking needed. It sits above the map image but below the deskballs, so the
// matched desks stay visible. The transparent hole + huge box-shadow form the
// surrounding fog. The opaque header and sidebar paint above #content, so the
// veil never darkens them.
function showSearchFog(id) {
  var desk = result_old && result_old.desks
    ? result_old.desks.filter(function (e) { return e.id == id; })[0]
    : null;
  if (!desk) { hideSearchFog(); return; }
  searchFogId = id;
  var deskitems = document.getElementById('deskitems');
  if (!deskitems) { return; }
  var scale = Number(itemscale);
  if (!(scale > 0)) { scale = 1; }
  var fog = document.getElementById('searchfog');
  if (!fog) {
    fog = document.createElement('div');
    fog.id = 'searchfog';
    fog.className = 'search_fog';
    // Insert as the first child so the veil paints below the deskballs (which
    // come after it in #deskitems) but above the map image behind #deskitems.
    deskitems.insertBefore(fog, deskitems.firstChild);
  }
  // Position the fog hole exactly like a deskball: coordinates are in the
  // pre-zoom space (counter.x / itemscale) and the element itself carries
  // zoom:itemscale, so it shares the desks' coordinate transform and stays
  // aligned through scroll and rescale automatically.
  var rPre = 110; // hole radius in pre-zoom units
  fog.style.zoom = scale;
  fog.style.left = (Number(desk.x) / scale - rPre) + 'px';
  fog.style.top = (Number(desk.y) / scale - rPre) + 'px';
  fog.style.width = (2 * rPre) + 'px';
  fog.style.height = (2 * rPre) + 'px';
  fog.classList.add('visible');
}

function hideSearchFog() {
  searchFogId = null;
  var fog = document.getElementById('searchfog');
  if (fog) { fog.classList.remove('visible'); }
}

function closeSearchAndClear() {
  $('#searchtext').val('');
  searchDesks();
}

function jumpToDesk(id, skipPulse) {
  var el = document.getElementById(id);
  if (!el) { return; }
  // Center the desk in the visible map area (the region right of the sidebar),
  // using on-screen geometry so it works regardless of zoom/scale math.
  var rect = el.getBoundingClientRect();
  var visLeft = searchSidebarWidth;
  var visCenterX = visLeft + (window.innerWidth - visLeft) / 2;
  var visCenterY = window.innerHeight / 2;
  var dx = (rect.left + rect.width / 2) - visCenterX;
  var dy = (rect.top + rect.height / 2) - visCenterY;
  window.scrollBy({ left: dx, top: dy, behavior: 'smooth' });
  if (!skipPulse) { pulseSearchResult(id); }
}

function pulseSearchResult(id) {
  if (setting_noanimation == 1) { return; }
  var desk = result_old.desks.filter(function (e) { return e.id == id; })[0];
  if (!desk) { return; }
  var old = document.getElementById('pulse' + id);
  if (old !== null && old.parentNode) { old.parentNode.removeChild(old); }
  var container = document.getElementById('deskitems') || document.getElementById('content');
  if (!container) { return; }
  var n = document.createElement('div');
  n.setAttribute('id', 'pulse' + id);
  container.appendChild(n);
  $('#pulse' + id).css('position', 'absolute');
  $('#pulse' + id).css('left', (desk.x - (15 * itemscale)) + 'px');
  $('#pulse' + id).css('top', (desk.y - (15 * itemscale)) + 'px');
  $('#pulse' + id).css('width', (30 * itemscale) + 'px');
  $('#pulse' + id).css('height', (30 * itemscale) + 'px');
  $('#pulse' + id).css('border-radius', '50%');
  $('#pulse' + id).css('pointer-events', 'none');
  $('#pulse' + id).css('animation', 'orange-jumppulse 1.2s 3');
  setTimeout(function () {
    var e = document.getElementById('pulse' + id);
    if (e !== null && e.parentNode) { e.parentNode.removeChild(e); }
  }, 4000);
}

// bindDeskHandlers attaches the desk mouse handlers once via event delegation
// on the #deskitems container, rather than emitting an inline <script> with
// per-desk bindings on every refresh. showSticky is suppressed in admin edit
// mode (where dragElement handles interaction instead).
function bindDeskHandlers() {
  if (deskHandlersBound) { return; }
  deskHandlersBound = true;
  $('#deskitems')
    .on('mouseover', '.deskball', function () {
      showNameplate(this.id, this.getAttribute('data-type'));
    })
    .on('mouseout', '.deskball', function () {
      hideNameplate(1);
    })
    .on('mouseup', '.deskball', function () {
      if (typeof token !== 'undefined' && setting_usermode == 'edit') { return; }
      showSticky(this.id, this.getAttribute('data-type'));
    });
}

// using the desks API to create all deskitems
// Half the on-map CSS box size (content space) for a custom item type's named
// size, mirroring CustomItemType.Halfsize() in db.go and editItemHalfsize() in
// admin.js so the marker renders at the same size everywhere.
function customHalfsize(size) {
  switch (size) {
    case 'small': return 18;
    case 'large': return 40;
    default: return 25;
  }
}

function updateDesks(forceRefresh) {    
  mapname = map;
  if (userdate != '') {
    var selectdate = userdate;
  }
  else {
    var selectdate = timezoneDate();
  }
  $.ajax({
    url: 'rest/desks?map=' + mapname+'&date='+selectdate,
    async: true, 
    type: 'get',
    dataType: 'JSON',
    error:function(result){
      console.log('Error: Could not update desks');
    },
    success: function(result){
        var outputdesks = '';
        var deskClass = '';
        var deskType = '';
        var nameplate_caption = '';
        var halfsize = '';
        var seenIds = {};
        var hasFloor = false;
        var floorEntries = [];
        for (var i = 0; i < result.desks.length; i++) {
          var counter = result.desks[i];
          // Check for shared desk - output only one item
          if (counter.id != '' && seenIds[counter.id]) {
            continue;
          }
          if (counter.id != '') { seenIds[counter.id] = true; }
          // Admin-defined custom item types render as a coloured/iconed marker
          // straight from the customItemTypes definition (no per-type CSS class).
          if (counter.desktype && counter.desktype.indexOf('custom_') === 0) {
            var ctDef = (typeof customItemTypes !== 'undefined' && customItemTypes[counter.desktype.slice(7)]) ? customItemTypes[counter.desktype.slice(7)] : null;
            var chalf = customHalfsize(ctDef ? ctDef.size : 'medium');
            var ccolor = ctDef ? ctDef.color : '#0979D8';
            var cicon = (ctDef && ctDef.icon) ? ctDef.icon : '';
            var clabel = counter.dsk || (ctDef ? ctDef.label : 'Item');
            var cstyle = 'position:absolute;left:' + (counter.x/itemscale-chalf) + 'px;top:' + (counter.y/itemscale-chalf)
                       + 'px;width:' + (2*chalf) + 'px;height:' + (2*chalf) + 'px;border-radius:50%;background-color:' + ccolor + ';'
                       + (cicon ? 'background-image:url(\'' + cicon + '\');background-size:cover;background-repeat:no-repeat;background-position:center;' : '')
                       + 'zoom:' + itemscale + ';';
            outputdesks += '<div id="' + counter.id + '" class="deskball" data-type="' + counter.desktype + '" style="' + cstyle + '">'
                        + '<div id="caption' + counter.id + '" class="caption">' + clabel + '</div></div>';
            continue;
          }
          switch (counter.desktype) {
  
          case "exit":
          case "meeting":
          case "restroom":
            //deskType = counter.dsk.toLowerCase();
            deskType = counter.desktype;
            nameplate_caption = counter.empl;
            halfsize = 25;
            break;
            
          case "firstaid":
          case "food":
          case "keycardlock":
          case "keylock":
          case "printer":
          case "service":
            //deskType = counter.dsk.toLowerCase();
            deskType = counter.desktype;
            nameplate_caption = counter.empl;
            halfsize = 18;
            break;

          case "floor":
            //deskType = "floor"
            deskType = counter.desktype;
            nameplate_caption = counter.empl;
            halfsize = 50;
            break;
          
          case "shareddesk":
            deskType = counter.desktype;
            nameplate_caption = 'Shared Desk';
            halfsize = 10;
            break;

          default: 
            if (counter.empl == '' || (counter.desktype == 'addesk' && counter.mail == '')) {
              deskType = "free";
              nameplate_caption = 'Not in use';
              halfsize = 10;
            }
            else {
              halfsize = 10;
              switch (counter.desktype) {
              case "blocked":
                deskType = counter.desktype;
                nameplate_caption = counter.dsk;
                break;
              case "booking":
              case "hotseat":
                if (counter.booked == 1) {
                  deskType = counter.desktype+"_booked";
                  nameplate_caption = counter.bookdata.name;
                }
                else {
                  deskType = counter.desktype+"_free";
                  nameplate_caption = counter.dsk;
                }
                break;
              case "addesk":
                deskType = "occupiedldap";
                nameplate_caption = counter.fname + ' ' + counter.lname;
                break;
              default: 
                deskType = "occupied";
                nameplate_caption = counter.empl;
                break;
              }
            }  
            break;
          }
          deskClass = 'deskball ' + deskType;
          // Robin live-occupancy overlay: tag the ball with a class so the pink
          // tint comes from CSS and survives resetColors() (which only clears
          // inline background-color set by the search highlighter).
          if (counter.robin == '1') {
            deskClass += ' robin';
          }

          switch (deskType) {
            case "floor": 
              // Floor markers are vertical navigation anchors locked to a fixed
              // rail on the right edge. Render an identical right-aligned tab in
              // BOTH edit and view mode so the on-screen position matches the
              // scroll anchor (the old view-mode `y-40` 1px hack made the anchor
              // land lower than where the marker was placed). zoom:itemscale +
              // translate(-100%,-50%) anchors the tab's right edge on the rail X
              // and centres it vertically on the stored Y.
              hasFloor = true;
              floorEntries.push({ id: counter.id, label: nameplate_caption });
              outputdesks+='<div id="' + counter.id + '" class="floor_tab" data-type="floor" style="position:absolute;left:'
                      + (FLOOR_RAIL_X/itemscale) + 'px;top:' + (counter.y/itemscale) + 'px;zoom:' + itemscale + ';transform:translate(-100%,-50%);">'
                      + '<span class="floor_tab_label">' + nameplate_caption + '</span>'
                      + '</div>';
              break;
            case "meeting":
              outputdesks+='<div id="' + counter.id + '" class="' + deskClass + '" data-type="' + deskType + '" style="position:absolute;left:' 
                            + (counter.x/itemscale-halfsize) + 'px;top:' + (counter.y/itemscale-halfsize) + 'px;border-radius:50%;zoom:' + itemscale + ';"'
                            + '>'
                            + '<div id="caption' + counter.id + '" class="caption">' + nameplate_caption + '</div>'
                            + '<div id="meeting' + counter.id + '" class="meeting_indicator" style="position:absolute; left:0px; top:25px;display:none;"'
                            + '>'
                            + '</div>'
                            + '</div>';
              break;
            default: 
                outputdesks+='<div id="' + counter.id + '" class="' + deskClass + '" data-type="' + deskType + '" style="position:absolute;left:' 
                            + (counter.x/itemscale-halfsize) + 'px;top:' + (counter.y/itemscale-halfsize) + 'px;border-radius:50%;zoom:' + itemscale + ';"'
                            + '>'
                            + '<div id="caption' + counter.id + '" class="caption">' + nameplate_caption + '</div></div>';
                break;
              }
        }
        
        if (forceRefresh != true && JSON.stringify(result) == result_old_str) {
          console.log('[Desks] up-to-date');
        }
        else {
          console.log('[Desks] new data - updating map');
          $("#deskitems").html(outputdesks);
          result_old = result;
          result_old_str = JSON.stringify(result);
          // Show/size the floor rail and (in edit mode) make floor tabs draggable.
          updateFloorRail(hasFloor);
          // Refresh the header floor navigation links so newly added/removed/
          // renamed floors appear immediately without a page reload.
          updateFloorLinks(floorEntries);
          // Bind mouse handlers once via event delegation instead of emitting a
          // <script> block per desk. In admin edit mode also (re)attach the
          // drag handlers to the freshly rendered desk elements.
          bindDeskHandlers();
          if (typeof(token) != 'undefined' && setting_usermode == 'edit') {
            var deskEls = document.querySelectorAll('#deskitems .deskball, #deskitems .floor_tab');
            for (var d = 0; d < deskEls.length; d++) {
              dragElement(deskEls[d], deskEls[d].getAttribute('data-type'));
            }
            // One-time conversion of legacy floor records to the rail X.
            if (typeof migrateFloorsToRail === 'function') { migrateFloorsToRail(); }
          }
          getMeetingStatus(true);
          //statsPanel(); 
          //if (searchtext) {searchDesks()}
          if ($('#searchtext').val() != '') {$("#search_button").click()}
          if (setting_highlightleaders == 1) {highlightManagers();}
          if (setting_desknumbers == 1) {showDesknumbers();}
          if (setting_shownames == 1) {showNames();}
          if (setting_printmode == 1) {$('.meeting').css('opacity','0.5');}
          // Re-apply the duplicate-desk health highlight after each re-render.
          flagHealthDesks();
        }
    }    
  });
}

// ---------------------------------------------------------------------------
// Floor rail (right-edge vertical navigation rail)
// ---------------------------------------------------------------------------
// Floor markers are rendered as tabs locked to a vertical rail at the right edge
// of the map. The rail is a thin line drawn as a direct child of #content (so it
// is unaffected by #deskitems being rebuilt each poll). It is shown whenever at
// least one floor marker exists, or while a floor item is being dragged from the
// palette.

// Ensure the rail element exists and is sized to the current map height. Returns
// the rail element (or null on the overview / when there is no content).
function ensureFloorRail() {
  var content = document.getElementById('content');
  if (!content) { return null; }
  var rail = document.getElementById('floorrail');
  if (!rail) {
    rail = document.createElement('div');
    rail.id = 'floorrail';
    rail.className = 'floor_rail';
    content.appendChild(rail);
  }
  rail.style.left = FLOOR_RAIL_X + 'px';
  // In edit mode the rail is drawn all the way to the end of the page (the full
  // content height) so it is always a clear drop target; outside edit mode it
  // spans the map image only.
  var editing = (typeof token !== 'undefined' && setting_usermode == 'edit');
  var img = document.getElementById('detailmapimage');
  var mapH = (img && img.offsetHeight) ? img.offsetHeight : (content.offsetHeight || 2000);
  var h = editing ? Math.max(content.offsetHeight || 0, mapH) : mapH;
  rail.style.height = h + 'px';
  return rail;
}

// Show/hide the rail. The rail is an editor-only affordance: it is only ever
// visible in edit mode (or while dragging a floor item from the palette).
function updateFloorRail(hasFloor) {
  var rail = ensureFloorRail();
  if (!rail) { return; }
  var editing = (typeof token !== 'undefined' && setting_usermode == 'edit');
  if (editing && (hasFloor || rail.classList.contains('dragging'))) {
    rail.classList.add('visible');
  } else if (!rail.classList.contains('dragging')) {
    rail.classList.remove('visible');
  }
}

// Rebuild the header floor navigation links from the current floor markers so
// adding/removing/renaming a floor is reflected immediately (no page reload).
// Mirrors the server-rendered markup in index.html. Labels are inserted as text
// nodes (not innerHTML) to avoid any markup injection from floor names.
function updateFloorLinks(floorEntries) {
  var container = document.getElementById('floorlinks');
  if (!container) { return; }
  container.innerHTML = '';
  for (var i = 0; i < floorEntries.length; i++) {
    var a = document.createElement('a');
    a.href = '#' + floorEntries[i].id;
    a.style.textDecoration = 'none';
    var div = document.createElement('div');
    div.className = 'headeritem_floors';
    div.textContent = floorEntries[i].label;
    a.appendChild(div);
    container.appendChild(a);
  }
  // The "Floor" caption shows only when there are floors and button
  // descriptions are enabled, matching the server-side template condition.
  var label = document.getElementById('floorcontrol_label');
  if (label) {
    var show = floorEntries.length > 0 && !(typeof noDescription !== 'undefined' && noDescription);
    label.style.display = show ? '' : 'none';
  }
}

// Force the rail visible (used while dragging a floor item from the palette).
function showFloorRailForDrag() {
  var rail = ensureFloorRail();
  if (!rail) { return; }
  rail.classList.add('visible');
  rail.classList.add('dragging');
}

// End the drag-forced visibility; hide the rail again unless floors exist.
function endFloorRailDrag() {
  var rail = document.getElementById('floorrail');
  if (!rail) { return; }
  rail.classList.remove('dragging');
  var hasFloor = !!document.querySelector('#deskitems .floor_tab');
  if (!hasFloor) { rail.classList.remove('visible'); }
}

// Smoothly scroll the page so the given floor marker reaches the top edge of the
// map content (just below the fixed header). Computed from live bounding rects so
// it is correct at any zoom; a floor placed at y=0 produces ~0 scroll (no jump).
function scrollToFloor(id) {
  var el = document.getElementById(id);
  var content = document.getElementById('content');
  if (!el || !content) { return; }
  var contentTop = content.getBoundingClientRect().top + window.scrollY;
  var elTop = el.getBoundingClientRect().top + window.scrollY;
  var target = elTop - contentTop;
  if (target < 0) { target = 0; }
  window.scrollTo({ top: target, behavior: 'smooth' });
}

$(function () {
  // Reflect the initial edit/user mode on <body> so CSS-driven editor-only
  // affordances (e.g. floor marker tabs) render correctly on first paint. Only
  // true editors (token defined) ever count as editing.
  document.body.classList.toggle('editmode', typeof token !== 'undefined' && setting_usermode === 'edit');
  // Header floor links (server-rendered as <a href="#id">) navigate via a
  // header-aware smooth scroll instead of the native anchor jump (which hid the
  // target behind the fixed header and used the buggy offset).
  $(document).on('click', '#floorcontrol a[href^="#"]', function (e) {
    e.preventDefault();
    scrollToFloor(this.getAttribute('href').substring(1));
  });
  // In view mode, clicking a floor tab on the rail also navigates. In edit mode
  // the tab is draggable and a click opens its editor (handled by dragElement).
  $(document).on('click', '#deskitems .floor_tab', function () {
    if (setting_usermode !== 'edit') { scrollToFloor(this.id); }
  });
});

function updateTeams() {
  $.ajax({
    url: 'rest/teams',
    async: true, 
    type: 'get',
    dataType: 'JSON',
    success: function(result){
      var teamfound = false;
      var teambox = ''
      + '<div style="font-size:20px;margin-left:10px;">'
      + '<img src="images/teams-banner.png" style="width:40%; display:block;margin-left: auto; margin-right:auto;" alt="logo" />'
      + '<table id="teamlist" style="width:95%">'
      + '<a href="https://teams.microsoft.com/l/chat/0/0?users='+teamsContact+'" target="_blank"><div class="announceplate" style="margin:0px; margin-bottom:10px; margin-top:10px; left:0px; height:30px; width:330px; background-color: #999900;">'
      + '<div class="announcetextbox" style="width:330px;left:0px;height:20px;top:5px;">'
      + '<div class="announcetext" style="width:310px;text-align:center; line-height:20px; font-size:15px;">Your team is missing? Click here to text me</div></div></div></a>'
      
      for (var i = 0; i < result.teams.length; i++) {
        var counter = result.teams[i];
        var teamname = counter.teamname
        var members = counter.members
        var textcolor = '#FFFFFF'
        // Check if it's a search result
        if (searchtext != '') {
          var searchArr = searchtext.split('|');
          for (var s = 0; s < searchArr.length; s++) {
            var namecheck = teamname.toLowerCase()+','+members.toLowerCase()
            var searchcheck = searchArr[s];
            searchcheck = searchcheck.toLowerCase();
            var sresult = namecheck.includes(searchcheck);
            if (sresult) {
              textcolor = '#FF7F00'
              teamfound = true
            }
          }  
        }
        teambox += '<tr><td><a href="'+root+'?findme='+members+'&teamlabel='+teamname+'" style="color:'+textcolor+'">'+teamname+'</a></td></tr>'
        if (teamfound) {
          document.getElementById("addressbook_img").src="images/addressbook_found.png";    
        }
        else {
          document.getElementById("addressbook_img").src="images/addressbook.png";
          removePulsateTeams()
        }
      }
        
      teambox += '<tr><td></td></tr><tr><td></td></tr><tr><td></td></tr>'
              +  '</table><div style="width:100%; height:6em"></div></div>'
      
      var element = document.getElementById('teambox');
      if (element !== null) {
       element.parentNode.removeChild(element);
      }

      var p = document.getElementById('addressbook')
      var newElement = document.createElement('div')
      newElement.setAttribute('id', 'teambox')
      newElement.innerHTML = teambox
      p.appendChild(newElement)

      $('#teambox').css('width','110%');
      $('#teambox').css('height','100%');
      $('#teambox').css('overflow-x','hidden');
      $('#teambox').css('overflow-y','scroll');

      console.log('[Teams] updated');
    }
  })
}

function showInfo(str) {
  // Removes remaining infoboxes from the document
  var element = document.getElementById('infobox');
  if (element !== null) {
    element.parentNode.removeChild(element);
  }

  var infodata =''
        + '<div id="infodata" style="position:fixed; left: 50%; margin-left: -150px; top: 200px; width:300px; height:100px; border-radius: 50px;'
        + 'background: #111;color:#fff; font-size: 20px; line-height:100px;text-align:center;'
        + 'border-style: solid; border-width: 2px; border-color: #0979D8; z-index:250;">'
        + str
        + '</div>';
        var newElement = document.createElement('div');
        newElement.setAttribute('id', 'infobox');
        newElement.setAttribute('pointer-events', 'none')
        newElement.innerHTML = infodata;
        document.body.appendChild(newElement);
        $("#infobox").fadeToggle(2500);
        //var p = document.getElementById('content');
        //p.appendChild(newElement);
}

function copyToClipboard (str) {
  // Create temporary element
  var el = document.createElement('textarea');
  // add string to element
  el.value = str;
  // set element to read-only and move it out of the window
  el.setAttribute('readonly', '');
  el.style = {position: 'absolute', left: '-9999px'};
  document.body.appendChild(el);
  // select text
  el.select();
  // copy text
  document.execCommand('copy');
  // remove temporary element
  document.body.removeChild(el);
  //alert("Link copied to clipboard");
  showInfo('Copied to clipboard');
}

// Open the admin-configured report URL in a new tab. The button that calls this
// is only rendered when reportURL is set, but guard anyway.
function openReportURL() {
  if (typeof reportURL !== 'undefined' && reportURL) {
    window.open(reportURL, '_blank', 'noopener');
  }
}

function setUserdate(selectdate) {
  $("#theDate").hide();
  userdate = selectdate;
  console.log("userdate set to "+userdate);
  UpdateClock();
  updateDesks();
}

function UpdateClock() {
    var tDate = new Date(new Date().getTime()+offset);
    if (userdate == timezoneDate()) {userdate = ''};
    if (userdate != '') {
      var printdate = '<span style="color:rgb(216, 196, 22); font-weight:bold;">' + userdate + '</span>';
    } 
    else {
      var printdate = timezoneDate();
    }
    var in_hours = tDate.getHours();
    var in_minutes=tDate.getMinutes();
    var in_seconds= tDate.getSeconds();
    var APM;

    if(in_minutes < 10)
        in_minutes = '0'+in_minutes;
    if(in_seconds<10)   
        in_seconds = '0'+in_seconds;
    if(in_hours>12) {
      in_hours = in_hours-12;
      APM = 'PM';
    }
    else {
      APM = 'AM';
    } 
    if(in_hours<10) 
        in_hours = '0'+in_hours;
  
   var tab = '&nbsp;&nbsp;&nbsp;';
   document.getElementById('theTime').innerHTML = printdate + tab + in_hours + ':' +in_minutes+' '+APM;
   console.log('[Clock] local time updated to '+printdate + '   ' + in_hours + ':' +in_minutes+' '+APM);
}

function StartClock() {
  setTimeout(UpdateClock, 500);
  clockID = setInterval(UpdateClock, 60000);
}

function KillClock() {
  clearTimeout(clockID);
}

var announceLive;
var changesData = [];
// Lazy-loading state for the full-screen changes modal.
var changesPageSize = 50;   // rows fetched per request
var changesOffset = 0;      // server offset of the next page
var changesHasMore = true;  // whether the server has more rows to send
var changesLoading = false; // a page request is currently in flight
var changesQuery = '';      // active server-side search term
var changesReqToken = 0;    // guards against stale/out-of-order responses
var changesSearchTimer = null; // debounce timer for the search box

// Fill the announcement sidebar with the most recent changes and keep the header
// glow indicator in sync. The full list (24-month-capped server-side) can be
// browsed via the full-screen modal (openChangesModal), reached from the button
// at the top of the sidebar.
function updateChangeTracker() {
  $.ajax({
    url: 'rest/changes/?maxresults=20',
    async: true,
    type: 'get',
    dataType: 'JSON',
    success: function(result){
        // The "Browse all changes" button lives inside the scrolling body so it
        // inherits the same scaleX/scaleY autozoom transforms as the change
        // plates (the body is the only counter-scaled element).
        var outputstring = '<div class="announceplate announceplate-action" style="height:60px;background-color:#0a66c2;cursor:pointer;" onclick="openChangesModal()">'
                          +'<div class="announcetextbox" style="height:60px;width:530px;left:10px;top:0px;">'
                          +'<div class="announcetext" style="width:530px;text-align:center;">'
                          +'<img src="images/maximize.png" style="height:22px;vertical-align:middle;margin-right:10px;" alt="" />Browse all changes'
                          +'</div></div></div>';
        for (var i = 0; i < result.changes.length; i++) {
          var counter = result.changes[i];
          if (counter.type=='Title') {
            outputstring+='<a href="'+root+'?findme='+encodeURIComponent(counter.fullname)+'">'
                          +'<div class="announceplate">'
                          +'<div class="announceavatar" style="background-image: url(avatarcache/'+ counter.avatar + '.jpg), url(images/noavatar.png);"></div>'
                          +'<div class="announcetextbox">'
                          +'<div class="announcetext">'
                          +counter.fullname + '<br />' + counter.newvalue + '<br />'
                          +'<span style="text-decoration: line-through; color:#c0c0c0;">'+ counter.oldvalue + '</span>'
                          +'</div></div>'
                          +'<div class="announcedate" style="background-color:#393a3c;">' + counter.timestamp + '</div>'
                          +'<div class="announcetype" style="background-color:#0000CC;">Title</div>'
                          +'</div>'
                          +'</a>';
          }
          if (counter.type=='Employee') {
            outputstring+='<a href="'+root+'?findme='+encodeURIComponent(counter.fullname)+'">'
                          +'<div class="announceplate">'
                          +'<div class="announceavatar" style="background-image: url(avatarcache/'+ counter.avatar + '.jpg), url(images/noavatar.png);"></div>'
                          +'<div class="announcetextbox">'
                          +'<div class="announcetext">'
                          +counter.fullname + '<br />' + counter.newvalue + '<br />'
                          +'</div></div>'
                          +'<div class="announcedate" style="background-color:#393a3c;">' + counter.timestamp + '</div>'
                          +'<div class="announcetype" style="background-color:#00CC00;">New</div>'
                          +'</div>'
                          +'</a>';
          }
        }
        announceLive = (result.changes && result.changes.length > 0) ? result.changes[0].id : 0;
        $("#announcementbar_body").html(outputstring);
        if (announceLive > announceValue) {
          document.getElementById("announce_img").src = "images/announce_on.png";
          $("#toggle_announcementbar").fadeTo(2000,0.1,"swing", function(){$(this).fadeTo(2000,0.9,"swing");} );
        }
        else {
          document.getElementById("announce_img").src = "images/announce.png";
        }
    }
  });
}

// buildChangeRow creates a single change list entry as a DOM node. User-supplied
// fields (from the LDAP mirror) are inserted via textContent to stay XSS-safe.
function buildChangeRow(c) {
  var a = document.createElement('a');
  a.className = 'changerow';
  a.href = root + '?findme=' + encodeURIComponent(c.fullname || '');

  var av = document.createElement('div');
  av.className = 'changerow-avatar';
  av.style.backgroundImage = 'url(avatarcache/' + (c.avatar || '') + '.jpg), url(images/noavatar.png)';
  a.appendChild(av);

  var main = document.createElement('div');
  main.className = 'changerow-main';
  var name = document.createElement('div');
  name.className = 'changerow-name';
  name.textContent = c.fullname || '';
  main.appendChild(name);
  var val = document.createElement('div');
  val.className = 'changerow-value';
  val.textContent = c.newvalue || '';
  main.appendChild(val);
  if (c.type === 'Title' && c.oldvalue) {
    var old = document.createElement('div');
    old.className = 'changerow-old';
    old.textContent = c.oldvalue;
    main.appendChild(old);
  }
  a.appendChild(main);

  var meta = document.createElement('div');
  meta.className = 'changerow-meta';
  var date = document.createElement('div');
  date.className = 'changerow-date';
  date.textContent = c.timestamp || '';
  meta.appendChild(date);
  var type = document.createElement('span');
  if (c.type === 'Title') {
    type.className = 'changerow-type changerow-type-title';
    type.textContent = 'Title';
  }
  else {
    type.className = 'changerow-type changerow-type-new';
    type.textContent = 'New';
  }
  meta.appendChild(type);
  a.appendChild(meta);

  return a;
}

// renderChanges (re)builds the modal list from changesData. Rows are loaded
// page-by-page from the server (see loadMoreChanges), so this just renders what
// has been fetched so far plus the loading/empty footer state.
function renderChanges() {
  var list = document.getElementById('changesList');
  if (!list) { return; }
  list.innerHTML = '';
  for (var i = 0; i < changesData.length; i++) {
    list.appendChild(buildChangeRow(changesData[i]));
  }
  var status = document.createElement('div');
  status.id = 'changesStatus';
  status.className = 'changes-empty';
  if (changesLoading) {
    status.textContent = 'Loading\u2026';
  }
  else if (changesData.length === 0) {
    status.textContent = changesQuery ? 'No changes match your search.' : 'No changes in the last 24 months.';
  }
  else if (!changesHasMore) {
    status.textContent = 'End of list.';
  }
  list.appendChild(status);
}

// loadMoreChanges fetches the next page of changes from the server and appends
// it to changesData. Guarded so only one request runs at a time.
function loadMoreChanges() {
  if (changesLoading || !changesHasMore) { return; }
  changesLoading = true;
  var reqToken = ++changesReqToken;
  renderChanges();
  $.ajax({
    url: 'rest/changes/?limit=' + changesPageSize + '&offset=' + changesOffset + '&q=' + encodeURIComponent(changesQuery),
    async: true,
    type: 'get',
    dataType: 'JSON',
    success: function(result){
      // Ignore stale responses from a superseded search.
      if (reqToken !== changesReqToken) { return; }
      var page = (result && result.changes) ? result.changes : [];
      changesData = changesData.concat(page);
      changesOffset += page.length;
      changesHasMore = !!(result && result.hasMore);
      changesLoading = false;
      renderChanges();
    },
    error: function(){
      if (reqToken !== changesReqToken) { return; }
      changesLoading = false;
      changesHasMore = false;
      var list = document.getElementById('changesList');
      if (list && changesData.length === 0) {
        list.innerHTML = '<div class="changes-empty">Could not load changes.</div>';
      }
    }
  });
}

// resetChangesList clears the loaded data and starts loading from the top for
// the current search query.
function resetChangesList() {
  changesData = [];
  changesOffset = 0;
  changesHasMore = true;
  changesLoading = false;
  changesReqToken++;
  renderChanges();
  loadMoreChanges();
}

// filterChanges re-runs the search server-side (debounced) so it covers the
// whole 24-month data set, not just the pages loaded so far.
function filterChanges() {
  var search = document.getElementById('changesSearch');
  changesQuery = search ? search.value.trim() : '';
  if (changesSearchTimer) { clearTimeout(changesSearchTimer); }
  changesSearchTimer = setTimeout(resetChangesList, 250);
}

// openChangesModal shows the full-screen modal and lazy-loads the change list
// (server-side 24-month-capped) page by page, then clears the header glow.
function openChangesModal() {
  var modal = document.getElementById('changesModal');
  if (!modal) { return; }
  $("#addressbook").hide();
  $("#settingspanel").hide();
  var search = document.getElementById('changesSearch');
  if (search) { search.value = ''; }
  changesQuery = '';
  modal.style.display = 'flex';
  resetChangesList();
  // Mark the current newest change as seen and stop the icon glowing.
  document.cookie = "announcecookie=" + announceLive + '; expires=Fri, 31 Dec 9999 23:59:59 GMT; SameSite=Lax';
  announceValue = announceLive;
  document.getElementById("announce_img").src = "images/announce.png";
}

function closeChangesModal() {
  var modal = document.getElementById('changesModal');
  if (modal) { modal.style.display = 'none'; }
}

function getPrinterStatus() {
  mapname = map;
  $.ajax({
    url: 'rest/printers?map=' + mapname,
    async: true, 
    type: 'get',
    dataType: 'JSON',
    success: function(result){
      if(JSON.stringify(result.printers) != JSON.stringify(printerstatus)) {
        printerstatus = result.printers;
        console.log('[Printers] new data');
      }
      else {
        console.log('[Printers] up-to-date');
      }  
    }
  });
};

function bookDesk() {
  $("#bookstatus").css("display","none");
  $.ajax({
    type: 'GET',
    url: 'rest/booking/',
    data: $("#Book").serialize(),
    contentType: false,
    cache: false,
    //processData:false,
    beforeSend: function(){
      senddata = $("#Book").serialize();
    },
    error:function(){
      console.log('error');
    },
    success: function(booking){
      var bookstatus = booking.status;
      var bookmessage = booking.message;
      if (bookstatus == 'error') {
        var color='#D82626'
        console.log(booking);
      }
      else if (bookstatus == 'ok') {
        var color='#35842E'
      }
      $("#bookstatus").css("background-color",color);
      $("#bookstatus").html(bookmessage);
      $("#bookstatus").css("display","").delay(1000).fadeOut("slow");
      userBookings()
      updateBookings()
    }
  });
}

function cancelBooking(date,map,desk) {
  $.ajax({
    type: 'GET',
    url: 'rest/booking/',
    data: { mode: "remove", bookdate: date, bookmap: map, bookdesk: desk },
    contentType: false,
    cache: false,
    //processData:false,
    beforeSend: function(){
      //$('#output').html('<img src="spinner.png" style="height:30px" />');
    },
    error:function(){
      //$('#uploadStatus').html('<span style="color:#EA4335;">Avatar upload failed, please try again.<span>');
      console.log('error');
    },
    success: function(data){
      //$('body').append('<div id="image_resize" style="position:fixed;bottom:10px;left:10px;width:750px;height:700px;background-color:rgb(51, 51, 51);border-radius:40px;z-index:2000;padding:10px;"></div>');    
      //$('#output').html(data);
      console.log(data);
      userBookings()
      updateBookings()
    }
  });
}

function userBookings() {
  usershort = username.replace(domain, "");
  $.ajax({
    type: 'GET',
    url: 'rest/booking/',
    data: { mode: "list", bookuser: usershort },
    contentType: false,
    cache: false,
    //processData:false,
    beforeSend: function(){
    },
    error:function(){
      console.log('error');
    },
    success: function(bookings){
      
      if (bookings.data.length > 0) {
        var output = '<table style="width: 100%;text-align: left;margin-left: 10px;">';
        output += '<tr> <th>date</th> <th>map</th> <th>desk</th> <th></th> </tr>';
        for (var i = 0; i < bookings.data.length; i++) {
          var booking = bookings.data[i];
          var bookdate = booking.date;
          var bookmap = booking.map;
          var bookdesk = booking.desk;
          output += '<tr style="height:30px;"><td>'+bookdate+'</td><td>'+bookmap+'</td><td>'+bookdesk+'</td>';
          output += '<td><img src="images/avatar-cancel.png" style="width:20px;cursor:pointer;" onmouseover="this.src=\'images/avatar-cancel_on.png\'"';
          output += 'onmouseout="this.src=\'images/avatar-cancel.png\'" onclick=cancelBooking("'+bookdate+'","'+bookmap+'","'+bookdesk+'")></td></tr>';
        }
        output += '</table>'
      }
      else {
        var output = 'no bookings found';
      }
      
      $('#bookingstable').html(output);
      bookheight=$("#bookingstable").height()
      fullheight=270 + bookheight + 'px';
      $('#personal_menu').css({'height':fullheight});
    }
  });
  console.log('[UserBookings] updated');
}

function updateBookings() {
  $.ajax({
    type: 'GET',
    url: 'rest/booking/',
    data: { mode: "list", bookmap: map },
    contentType: false,
    cache: false,
    //processData:false,
    beforeSend: function(){
    },
    error:function(){
      console.log('error');
    },
    success: function(bookings){
      bookingstatus = bookings.data;
      bookingdate = bookings.date;
      if (activecalendar != '') {
        $('#calendar'+activecalendar).replaceWith(updateCalendar(activecalendar))
      }
      updateDesks();
    }
  });
  console.log('[Bookings] updated');
}

// Lay out the personal-menu box and its three action buttons so they line up
// with the row below (avatar + username + optional admin button), regardless of
// how long the user's name is.
//   - The menu spans from before the avatar to behind the username (or, for
//     admins, behind the admin-panel button).
//   - Logout sits on top of the avatar.
//   - Remove image sits on top of the admin-panel button when present; otherwise
//     it mirrors the logout button's left gap on the right edge.
//   - Upload image sits centered between the logout and remove buttons.
function layoutPersonalMenu() {
  var menu = document.getElementById("personal_menu");
  var row = document.querySelector(".avatarbutton_row");
  if (!menu || !row) { return; }

  var HALF = 40;                 // half of the 80px button width
  var logoutCenter = 60;         // avatar is at left:20, width 80 -> center 60

  var rowRight = row.offsetLeft + row.offsetWidth;   // right edge of the row
  // Extend the menu 20px past the row's right edge to mirror the 20px gap on the
  // left (between the menu edge and the avatar).
  var menuWidth = Math.max(390, rowRight + 20);
  menu.style.width = menuWidth + "px";

  var admin = document.getElementById("adminpanel_button");
  var removeCenter;
  if (admin && admin.offsetParent !== null) {
    // Place "Remove image" on top of the admin-panel button. Convert the admin
    // button's on-screen centre into the menu's (pre-zoom) local coordinates.
    var aRect = admin.getBoundingClientRect();
    var mRect = menu.getBoundingClientRect();
    var zoom = aRect.width / admin.offsetWidth || 1;
    removeCenter = (aRect.left + aRect.width / 2 - mRect.left) / zoom;
  } else {
    // No admin button: mirror the logout button's 20px left gap on the right.
    removeCenter = menuWidth - 60;
  }

  var uploadCenter = (logoutCenter + removeCenter) / 2;

  function place(id, center) {
    var el = document.getElementById(id);
    if (el) { el.style.left = (center - HALF) + "px"; }
  }
  place("pm_logout_label", logoutCenter);
  place("pm_logout_btn", logoutCenter);
  place("pm_upload_label", uploadCenter);
  place("pm_upload_btn", uploadCenter);
  place("pm_remove_label", removeCenter);
  place("pm_remove_btn", removeCenter);

  // Keep the bookings text centered across the (possibly wider) menu.
  var bookings = document.getElementById("bookingstable");
  if (bookings) { bookings.style.width = (menuWidth - 40) + "px"; }
}

function togglePersonalMenu() {
  var x = document.getElementById("personal_menu");
  var admin = document.getElementById("adminpanel_button");
  if (!x.classList.contains("open")) {
    layoutPersonalMenu();
    x.classList.add("open");
    if (admin) { admin.classList.add("menu-open"); }
    userBookings();
  } else {
    x.classList.remove("open");
    if (admin) { admin.classList.remove("menu-open"); }
  }
}

// Document ready function
$(function() {
  $('#searchtext').keyup(function(event){
    if(event.keyCode == 13){
      $("#search_button").click();
    }
  });
  $( "#toggle_maps" ).click(function() {
    $.ajax({
      url: 'rest/config/?mode=maps',
      async: true, 
      type: 'get',
      dataType: 'JSON',
      success: function(result){
        var hoehe=$("#container").height()-65
        // Let each dropdown column (real maps / placeholder maps) wrap into
        // sub-columns when it would be taller than the viewport. The panel
        // width then sizes itself to the columns via flexbox.
        $('#mapspanel .maps-col').css('max-height', hoehe+'px')
        $("#mapspanel").slideToggle("fast");
      }
    });
    
  });
  $( "#toggle_settings" ).click(function() {
    $( "#addressbook" ).hide();
    $( "#announcementbar" ).hide();
    $("#settingspanel").toggle();
  });
  $("#toggle_addressbook").click(function() {
    $( "#announcementbar" ).hide();
    $( "#settingspanel" ).hide();
    $( "#addressbook" ).toggle();
  });
  $("#toggle_announcementbar").click(function() {
    $( "#addressbook" ).hide();
    $( "#settingspanel" ).hide();
    $( "#announcementbar" ).toggle();
    document.cookie = "announcecookie=" + announceValue+'; expires=Fri, 31 Dec 9999 23:59:59 GMT; SameSite=Lax';
    announceValue = announceLive;
    document.getElementById("announce_img").src = "images/announce.png";
  });
  // Dismiss the changes modal on backdrop click or Esc.
  $("#changesModal").click(function(e) {
    if (e.target === this) { closeChangesModal(); }
  });
  // Infinite scroll: load the next page as the user nears the bottom.
  $("#changesList").on("scroll", function() {
    if (this.scrollTop + this.clientHeight >= this.scrollHeight - 200) {
      loadMoreChanges();
    }
  });
  $(document).on("keydown", function(e) {
    if (e.key === "Escape") { closeChangesModal(); }
  });

  // File upload via Ajax
  $("#uploadForm").on('submit', function(e){
    e.preventDefault();
    $.ajax({
      type: 'POST',
      url: 'rest/avatar',
      data: new FormData(this),
      contentType: false,
      cache: false,
      processData:false,
      beforeSend: function(){
        //$('#uploadStatus').html('<img src="images/uploading.gif"/>');
      },
      error:function(){
        //$('#uploadStatus').html('<span style="color:#EA4335;">Avatar upload failed, please try again.<span>');
      },
      success: function(data){
        $('#uploadForm')[0].reset();
        // The Go backend auto-crops to a square and returns JSON; just refresh the thumbnail.
        $('#avatarbutton img').attr('src', 'avatarcache/' + data.userid + '.jpg?time=' + Date.now());
      }
    });
  });
  // File type validation
  $("#avatarInput").change(function(){
    var fileLength = this.files.length;
    var match= ["image/jpeg","image/jpg"];
    var i;
    for(i = 0; i < fileLength; i++){
      var file = this.files[i];
      var imagefile = file.type;
      if(!((imagefile==match[0]) || (imagefile==match[1]) )){
        alert('Please select a valid image file (JPEG/JPG).');
        $("#avatarInput").val('');
        return false;
      }
      else {
        document.getElementById('uploadButton').click()
      }
    }
  });
  $("#deleteForm").on('submit', function(e){
    e.preventDefault();
    $.ajax({
      type: 'POST',
      url: 'rest/avatar',
      data: new FormData(this),
      contentType: false,
      cache: false,
      processData:false,
      beforeSend: function(){
        //$('#uploadStatus').html('<img src="images/uploading.gif"/>');
      },
      error:function(){
        //$('#uploadStatus').html('<span style="color:#EA4335;">Deletion failed, please try again.<span>');
      },
      success: function(data){
        $('#uploadForm')[0].reset();
        //$('#uploadStatus').html('<span style="color:#28A74B;">Avatar has been deleted.<span>');
        $('#avatarbutton img').attr('src', 'images/noavatar.png?time=' + Date.now());
      }
    });
  });

  $("#resizeForm").on('submit', function(e){
    e.preventDefault();
    var x1 = $('#x1').val();
    var y1 = $('#y1').val();
    var x2 = $('#x2').val();
    var y2 = $('#y2').val();
    var w = $('#w').val();
    var h = $('#h').val();
    if(x1=='' || y1=='' || x2=='' || y2=='' || w=='' || h==''){
      alert('Please select an area in your image');
    }
    else {
      $.ajax({
        type: 'POST',
        url: 'rest/avatar/',
        data: new FormData(this),
        contentType: false,
        cache: false,
        processData:false,
        beforeSend: function(){
          //$('#uploadStatus').html('<img src="images/uploading.gif"/>');
        },
        error:function(){
          //$('#uploadStatus').html('<span style="color:#EA4335;">Deletion failed, please try again.<span>');
        },
        success: function(data){
          console.log('success on creating the thumbnail');
          $('#resizeForm')[0].reset();
          $('#avatarbutton').html(data);
          console.log(data);
          $( "#image_resize" ).remove();
        }
      });
    }
    
  });

});