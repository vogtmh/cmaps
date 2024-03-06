// Helper functions for all users and admins

// Declare global variables
var result_old;
var overviewmaps;
var bookingstatus;
var meetingstatus; 
var printerstatus;
var stickaddresses;
var searchtext = "";
var inMobileMode = false;
var activecalendar = '';
var userdate = '';

function toggleUsermode() {
  if (setting_usermode == 'edit') {
    setting_usermode = 'user';
    $("#usermode_switch").css("background-color", "orange");
  }
  else {
    setting_usermode = 'edit';
    $("#usermode_switch").css("background-color", "#333");
  }
  document.cookie = "setting_usermode=" + setting_usermode+'; expires=Fri, 31 Dec 9999 23:59:59 GMT; SameSite=Lax';
  $("#usermode_switch").html(setting_usermode);
  updateDesks(true);
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

function loginUser(mode) {
  if (mode == 'admin') {
    remoteurl = '../rest/account/index.php'
  }
  else {
    remoteurl = 'rest/account/index.php'
  }
  $("#loginstatus").css("display","none");
  $.ajax({
    type: 'GET',
    url: remoteurl,
    data: $("#LoginForm").serialize(),
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
      $("#loginstatus").css("background-color",color);
      $("#loginstatus").html(logmessage);
      $("#loginstatus").css("display","").delay(1000).fadeOut("slow");
      if (logstatus == 'ok') { location.reload(); }
    }
  });
}

function samlLogin(mode) {
  if (mode == 'admin') {
    remoteurl = '../rest/account/index.php'
  }
  else {
    remoteurl = 'rest/account/index.php'
  }
  $.ajax({
    type: 'GET',
    url: remoteurl,
    data: {mode:"samllogin"},
    contentType: false,
    cache: false,
    //processData:false,
    beforeSend: function(){
    },
    error:function(){
      console.log('error');
    },
    success: function(logindata){
      var logstatus = logindata.status;
      var logmessage = logindata.message;
      if (logstatus == 'ok') { location.reload(); }
    }
  });
}

function logoutUser(mode) {
  if (mode == 'admin') {
    remoteurl = '../rest/account/index.php'
  }
  else {
    remoteurl = 'rest/account/index.php'
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
          location.href='../index.php'; 
        }
        else {
          location.reload(); 
        }
      }
    }
  });
}

function loginForm (showform,mode) {
    
    switch (showform) {
      case true:

        var logindata =`
          <div id="logindata" style="position:absolute; left:25%; top:25%; width:50%; height:50%;">
            <form id="LoginForm" name="Login" method="post" autocomplete="off">
              <input type="hidden" name="mode" value="login">
              <div style="width:30%; float:left;display:inline;">Username</div>
              <input type="text" name="user" value="" size="10" autofocus style="width:70%; float:left; display:inline;"><br />
              <div style="width:30%; float:left;display:inline;">Password</div>
              <input type="password" name="password" value="" size="10" style="width:70%; float:left; display:inline;"><br /><br />
              <input type="submit" value="Login" style="width:45%; float: left;" formaction='javascript:loginUser("`+mode+`")'>
            </form>
            <input type="button" value="Cancel" style="width:45%; float:right; background-color:#f00;" onClick="loginForm(false)">
            <div id="loginstatus" style="height:43px; border-top: 8px; width:100%;border-radius:4px;background-color:#00f;float:left;display:none;text-align:center;line-height:43px;"></div>
          </div>
        `;
        var newElement = document.createElement('div');
        newElement.setAttribute('id', 'loginform');
        newElement.innerHTML = logindata;
        document.body.appendChild(newElement);
        $('#loginform').css('position','fixed');
        $('#loginform').css('left','0px');
        $('#loginform').css('top','0px');
        $('#loginform').css('width','100%');
        $('#loginform').css('height','100%');
        $('#loginform').css('background-color','#000');
        $('#loginform').css('opacity','0.85');
        $('#loginform').css('z-index','200');
        $('#logindata').css('transform','scale(1.5)');
        $('#logindata').css('transform-origin','50% 50%');

        // Hide icon
        var icon = '';
        $("#loginicon").html(icon);
        break;
      
      case false:
        // Remove old sticky if exists
        var element = document.getElementById('loginform');
        if (element !== null) {
          element.parentNode.removeChild(element);
        }
        // Output icon
        var icon = '<input type="image" src="images/login-user.png" alt="e" onClick="loginForm(true)" onmouseover=this.src="images/login-user_on.png" onmouseout=this.src="images/login-user.png">';
        $("#loginicon").html(icon);
        updateCounter();
        break;

      default:
        break;
    }
}

function showMapplate (mapname) {
  var mapinfo = overviewmaps.find(o => Object.entries(o).find(([k, value]) => k === 'mapname' && value === mapname) !== undefined);
  
  var plateX = Number(mapinfo.x)+(Number(mapinfo.flagsize)/2)-100;
  var plateY = Number(mapinfo.y)+(Number(mapinfo.flagsize)/2)-100;

  if (plateY < 0) {
    var y_nameplate = 0;
  }
  else {
    var y_nameplate = plateY; 
  }

  copylink_full = 'https://'+window.location.hostname+root+'?map='+mapinfo.mapname;
  copylink_full = encodeURI(copylink_full);

  if (plateX > (targetScreenWidth/2)) {
    var outputmapplate= '<div class="leftmapplate" style="top:'+y_nameplate+'px;left:'+(plateX-400)+'px">'
                      + '<div class="leftmapplate_goto" onclick=hideMapplate()><img src="images/close3.png" style="width:70px;margin-top:60px;margin-right:20px;"></div>'
                      + '<div class="leftmapplate_top">'+ucWords(mapinfo.mapname)+'</div>'
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
                      + '<div class="rightmapplate_top">'+ucWords(mapinfo.mapname)+'</div>'
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
    $.each( managers, function( t, manager ){
      // Create highlightmarker
      var p = document.getElementById('deskitems');
      var newElement = document.createElement('div');
      newElement.setAttribute('id', 'manager' + manager.id);
      p.appendChild(newElement);
      //newElement.innerHTML = output;
      $('#manager' + manager.id).css('position','absolute');
      $('#manager' + manager.id).css('left',(manager.x-(12*itemscale)) + 'px');
      $('#manager' + manager.id).css('top',(manager.y-(12*itemscale)) + 'px');
      $('#manager' + manager.id).css('width',(18*itemscale)+'px');
      $('#manager' + manager.id).css('height',(18*itemscale)+'px');
      $('#manager' + manager.id).css('border',(3*itemscale)+'px solid '+manager.color);
      $('#manager' + manager.id).css('background-color', 'transparent');
      $('#manager' + manager.id).css('border-radius','50%');
      $('#manager' + manager.id).css('z-index','9');
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
        $('#desknumber' + desk.id).css('left',(desk.x-(20*itemscale)) + 'px');
        $('#desknumber' + desk.id).css('top',(desk.y-(10*itemscale)) + 'px');
        $('#desknumber' + desk.id).css('width',(40*itemscale)+'px');
        $('#desknumber' + desk.id).css('height',(20*itemscale)+'px');
        $('#desknumber' + desk.id).css('text-align','center');
        $('#desknumber' + desk.id).css('font-size','8px');
        $('#desknumber' + desk.id).css('color',textcolor);
        $('#desknumber' + desk.id).css('line-height',lineheight+'px');
        $('#desknumber' + desk.id).css('background-color','transparent');
        $('#desknumber' + desk.id).css('z-index','9');
        $('#desknumber' + desk.id).css('transform','scale('+itemscale+')');
        $('#desknumber' + desk.id).css('transform-origin','50% 0%');
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
        $('#name' + desk.id).css('left',(desk.x-(20*itemscale)) + 'px');
        $('#name' + desk.id).css('top',divtop + 'px');
        $('#name' + desk.id).css('width',(40*itemscale)+'px');
        $('#name' + desk.id).css('height',(20*itemscale)+'px');
        $('#name' + desk.id).css('text-align','center');
        $('#name' + desk.id).css('font-size','8px');
        $('#name' + desk.id).css('color',textcolor);
        $('#name' + desk.id).css('line-height',lineheight+'px');
        $('#name' + desk.id).css('background-color','transparent');
        $('#name' + desk.id).css('z-index','9');
        $('#name' + desk.id).css('transform','scale('+itemscale+')');
        $('#name' + desk.id).css('transform-origin','50% 0%');
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
        if (imageExist('avatarcache/' + attr.avtr + '.jpg')) {
          avatar = 'avatarcache/' + attr.avtr + '.jpg';
        }
        else {
          avatar = 'images/' + desktype + '.png';
        }
        
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
        var avatar = 'avatarcache/' + attr.avtr + '.jpg';
        var avatarcolor = $('#' + attr.id).css('background-color');
        content = attr.title + '<br />'+ attr.mail + '<br />'+ attr.phone + '<br />' + attr.mobil + '<br />'
        break;
      case "occupiedldap":
        var caption = attr.fname + ' ' + attr.lname;
        var avatar = 'avatarcache/' + attr.avtr + '.jpg';
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
    }
    if (attr.color != '') {
      var color = attr.color;
    }
    else {
      var color = '#999';
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
                        + '<div class="leftnameplate_number">' + attr.dsk + '</div></div>'
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
                        + '<div class="leftnameplate_number">' + attr.dsk + '</div></div>'
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
                      + '<div class="rightnameplate_number">' + attr.dsk + '</div></div>'
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
                      + '<div class="rightnameplate_number">' + attr.dsk + '</div></div>'
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
        var avatar; 
        if (imageExist('avatarcache/' + attr.avtr + '.jpg')) {
          avatar = 'avatarcache/' + attr.avtr + '.jpg';
        }
        else {
          avatar = 'images/' + desktype + '.png';
        }
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
        var avatar = 'avatarcache/' + attr.avtr + '.jpg';
        var avatarcolor = $('#' + attr.id).css('background-color');
        content = attr.title + '<br />'+ attr.mail + '<br />'+ attr.phone + '<br />' + attr.mobil + '<br />'
        break;
      case "occupiedldap":
        var caption = attr.fname + ' ' + attr.lname;
        var copylink = attr.fname + ' ' + attr.lname;
        var avatar = 'avatarcache/' + attr.avtr + '.jpg';
        var avatarcolor = $('#' + attr.id).css('background-color');
        content = attr.title + '<br />'+ attr.mail + '<br />'+ attr.phone + '<br />' + attr.mobil + '<br />'
        break;
      case "shareddesk":
        var caption = attr.fname + ' ' + attr.lname;
        var copylink = attr.dsk;
        var avatar = 'avatarcache/' + attr.avtr + '.jpg';
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
    }
    if (attr.color != '') {
      var color = attr.color;
    }
    else {
      var color = '#999';
    }
    avatarcolor = avatarcolor.replace(/[^,]+(?=\))/, '1.0');
    deskidstring = "'" + deskid + "'";
    desktypestring = "'" + desktype + "'";
    copylink_full = 'https://'+window.location.hostname+root+'?map='+map+'&findme='+copylink;
    copylink_full = encodeURI(copylink_full);
    //console.log(copylink_full);
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
                          + '<img class="leftnameplate_copy" src="images/copy.png" onclick="copyToClipboard(\''+copylink_full+'\')" />'
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
                          + '<img class="rightnameplate_copy" src="images/copy.png" onclick="copyToClipboard(\''+copylink_full+'\')" />'
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
        for (var i = 0; i < allmaps.length; i++) {
          var onemap = allmaps[i];
          if (onemap.mapname != 'overview' && onemap.published == 'yes') {
            mapout += '' //<a href="'+root+'?map='+onemap.mapname+'" id="link_'+onemap.mapname+'">
                  + '<div class="mapflag" id="mapflag_'+onemap.mapname+'" style="left: '+onemap.x+'px; top: '+onemap.y+'px;' 
                  + 'width:'+onemap.flagsize+'px; height:'+onemap.flagsize+'px; background-image: url(countryflags/'+onemap.country+'.svg);" '
                  + 'onclick="showMapplate(\''+onemap.mapname+'\')">'
                  + '<div style="position:relative; height:100%; text-align: center;color:white;">'
                  + '<span style="line-height:'+onemap.flagsize+'px;font-size:'+(Number(onemap.flagsize)/100*16)+'px; background: rgba(50,50,50,0.8);">'+ucWords(onemap.mapname)+'</span>'
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
  // Reset search button
  $('#search_button').val('Find')
  $('#searchtext').val('')
  $('#search_button').css('background-color','#0979D8')
}

function searchDesks() {    
  searchtext = $('#searchtext').val()
  searchtext = searchtext.trim();
  resetColors()
  hideSticky()
  // Start search
  if (searchtext) {
    showInfo("searching...")
    if (map != 'overview') {
      searchLocaldesks()
    }
    searchGlobaldesks()
  }
  updateTeams()
}

function searchLocaldesks() {
  var localdesks = result_old.desks;
  var localresults = 0
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
          // count local results
          localresults++
          // output local results
          $('#'+localdesk.id).css('background-color','rgba(255, 127, 0, 1)')
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
  // Display number of results on search button
  $('#search_button').val(localresults+' found')
  $('#search_button').css('background-color','#FF7F00')
  // Colorize mobile search button - if exists
  var element = document.getElementById('mobile_searchbutton');
  if (element !== null) {
    $('#mobile_searchbutton').css('background-color','#FF7F00');
  }
  // scroll to first result
  if (gotoY != 0) {
    window.scrollTo(0, (gotoY-150)*autozoom*zoom)
  }
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
      // Output global search results
      for (var g = 0; g < globalmaps.length; g++) {
        mapoutput = globalmaps[g].charAt(0).toUpperCase() + globalmaps[g].substr(1).toLowerCase()
        document.getElementById("notifycontent").innerHTML += '<a href="'+root+'?map='+globalmaps[g]+'&findme='+searchtext+'"><span class="notifybadge">'+globalresults[g]+' | '+mapoutput+'</span></a>'
      }
    }
  });
}

// using the desks API to create all deskitems
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
        for (var i = 0; i < result.desks.length; i++) {
          var counter = result.desks[i];
          // Check for shared desk - output only one item
          if (counter.id != '' && outputdesks.includes('id="'+counter.id+'"')) {
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

          switch (deskType) {
            case "floor": 
              if (typeof token !== 'undefined') {
                outputdesks+='<div id="' + counter.id + '" class="' + deskClass + '" style="position:absolute;left:' 
                        + (counter.x-(halfsize*itemscale)) + 'px;top:' + (counter.y-(halfsize*itemscale)) + 'px;border-radius:50%;transform:scale(' + itemscale + ');transform-origin: 0% 0%;"'
                        + 'onmouseover=showNameplate("' + counter.id + '","' + deskType + '"); onmouseup=showSticky("' + counter.id + '","' + deskType + '"); onmouseout=hideNameplate(1);>'
                        + '<div id="caption' + counter.id + '" class="caption">' + nameplate_caption + '</div></div>';
              }
              else {
                outputdesks+='<div id="' + counter.id + '" class="' + deskClass + '" style="position:absolute; visibility:hidden; left:' 
                        + counter.x + 'px;top:' + (counter.y-40) + 'px;border-radius:50%; width:1px; height:1px;"></div>';
              }
              break;
            case "meeting":
              outputdesks+='<div id="' + counter.id + '" class="' + deskClass + '" style="position:absolute;left:' 
                            + (counter.x-(halfsize*itemscale)) + 'px;top:' + (counter.y-(halfsize*itemscale)) + 'px;border-radius:50%;transform:scale(' + itemscale + ');transform-origin: 0% 0%;"'
                            + '>'
                            + '<div id="caption' + counter.id + '" class="caption">' + nameplate_caption + '</div>'
                            + '<div id="meeting' + counter.id + '" class="meeting_indicator" style="position:absolute; left:0px; top:25px;display:none;"'
                            + '>'
                            + '</div>'
                            + '</div>';
              break;
            default: 
                outputdesks+='<div id="' + counter.id + '" class="' + deskClass + '" style="position:absolute;left:' 
                            + (counter.x-(halfsize*itemscale)) + 'px;top:' + (counter.y-(halfsize*itemscale)) + 'px;border-radius:50%;transform:scale(' + itemscale + ');transform-origin: 0% 0%;"'
                            + '>'
                            + '<div id="caption' + counter.id + '" class="caption">' + nameplate_caption + '</div></div>';
                break;
              }
          // add mouse events based on user/admin mode
          outputdesks += '<script>';
          if (typeof(token) != 'undefined' && setting_usermode == 'edit') {
            // dragNdrop for admins
            outputdesks += 'dragElement(document.getElementById('+counter.id+'),"'+deskType+'");'
            outputdesks += '$("#'+counter.id+'").mouseover(function(){showNameplate("'+counter.id+'", "'+deskType+'");});'
            outputdesks += '$("#'+counter.id+'").mouseout(function(){hideNameplate(1);});'
          }
          else {
            // default actions for users
            outputdesks += '$("#'+counter.id+'").mouseover(function(){showNameplate("'+counter.id+'", "'+deskType+'");});'
            outputdesks += '$("#'+counter.id+'").mouseup(function(){showSticky("'+counter.id+'", "'+deskType+'");});'
            outputdesks += '$("#'+counter.id+'").mouseout(function(){hideNameplate(1);});'
          }
          outputdesks += '</script>';
        }
        
        if (JSON.stringify(result) == JSON.stringify(result_old) && (forceRefresh != true)) {
          console.log('[Desks] up-to-date');
        }
        else {
          console.log('[Desks] new data - updating map');
          $("#deskitems").html(outputdesks);
          result_old = result;
          getMeetingStatus(true);
          //statsPanel(); 
          //if (searchtext) {searchDesks()}
          if ($('#searchtext').val() != '') {$("#search_button").click()}
          if (setting_highlightleaders == 1) {highlightManagers();}
          if (setting_desknumbers == 1) {showDesknumbers();}
          if (setting_shownames == 1) {showNames();}
          if (setting_printmode == 1) {$('.meeting').css('opacity','0.5');}
        }
    }    
  });
}

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
          pulsateTeamResults()
          if (inMobileMode) {$('#mobile_teambutton').css("background-color", "#ff7f00")} 
        }
        else {
          document.getElementById("addressbook_img").src="images/addressbook.png";
          removePulsateTeams()
          if (inMobileMode) {$('#mobile_teambutton').css("background-color", "#222")}
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
      checkMobile()
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

// loads additional mobile scripts if running on mobile
function checkMobile() {
  if( detectMobile() ) {
    if (inMobileMode) {
      // already in mobile mode, do nothing
    } 
    else {
      console.log('switching to mobile mode')
      inMobileMode = true;
      var script = document.createElement('script');
      // wait until mobile script has loaded
      script.onload = function () {
        addMobileInterface()
      };
      script.src = 'mobile80.js';
      document.head.appendChild(script);
    }
  }

}

// checks if running on a mobile device and returns true or false
function detectMobile() {
  var isMobile = {
    Android: function() {
        return navigator.userAgent.match(/Android/i);
    },
    BlackBerry: function() {
        return navigator.userAgent.match(/BlackBerry/i);
    },
    iOS: function() {
        return navigator.userAgent.match(/iPhone|iPad|iPod/i);
    },
    Opera: function() {
        return navigator.userAgent.match(/Opera Mini/i);
    },
    Windows: function() {
        return navigator.userAgent.match(/IEMobile/i) || navigator.userAgent.match(/WPDesktop/i);
    },
    any: function() {
        return (isMobile.Android() || isMobile.BlackBerry() || isMobile.iOS() || isMobile.Opera() || isMobile.Windows());
    }
  };

  if( isMobile.any() ) {
    return true
  }
  else {
    return false
  }
}

var announceLive;
// using the changes API to update the announcementbar
function updateChangeTracker() {    
  $.ajax({
    url: 'rest/changes/?maxresults=20',
    async: true, 
    type: 'get',
    dataType: 'JSON',
    success: function(result){
        var outputstring ='<a href="'+teamsChannel+'" target="_blank"><div class="announceplate" style="height:60px; background-color: #999900;">'
                          +'<div class="announcetextbox" style="height:40px;width:430px;left:10px;top:10px;">'
                          +'<div class="announcetext" style="width:410px;text-align:center;">Join our Teams channel for live notifications</div></div></div></a>';
        for (var i = 0; i < result.changes.length; i++) {
          var counter = result.changes[i];
          if (counter.type=='Title') {
            outputstring+='<a href="'+root+'?findme='+counter.fullname+'">'
                          +'<div class="announceplate">'
                          +'<div class="announceavatar" style="background-image: url(avatarcache/'+ counter.avatar + '.jpg), url(images/noavatar.png);"></div>'
                          +'<div class="announcetextbox">'
                          +'<div class="announcetext">'
                          +counter.timestamp + '<br />'
                          +counter.fullname + '<br />' + counter.newvalue + '<br />'
                          +'<span style="text-decoration: line-through; color:#c0c0c0;">'+ counter.oldvalue + '</span>'
                          +'</div></div>'
                          +'<div class="announcedate" style="background-color:#0000CC;">0000.00.00 00 AM</div>'
                          +'<div class="announcetype" style="background-color:#0000CC;">Title</div>'
                          +'</div>'
                          +'</a>';
          }
          if (counter.type=='Employee') {
            outputstring+='<a href="'+root+'?findme='+counter.fullname+'">'
                          +'<div class="announceplate">'
                          +'<div class="announceavatar" style="background-image: url(avatarcache/'+ counter.avatar + '.jpg), url(images/noavatar.png);"></div>'
                          +'<div class="announcetextbox">'
                          +'<div class="announcetext">'
                          +counter.timestamp + '<br />'
                          +counter.fullname + '<br />' + counter.newvalue + '<br />'
                          +'</div></div>'
                          +'<div class="announcedate" style="background-color:#0000CC;">0000.00.00 00 AM</div>'
                          +'<div class="announcetype" style="background-color:#00CC00;">New</div>'
                          +'</div>';
                          +'</a>';
          }
          
        }
        announceLive = result.changes[0].id;
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
    url: 'rest/booking/index.php',
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
    url: 'rest/booking/index.php',
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
    url: 'rest/booking/index.php',
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
    url: 'rest/booking/index.php',
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

function togglePersonalMenu() {
  var x = document.getElementById("personal_menu");
  if (x.style.visibility === "hidden") {
    x.style.visibility = "visible";
    userBookings();
  } else {
    x.style.visibility = "hidden";
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
        var allmaps = result.maps
        var activemaps = 0
        for (var i = 0; i < allmaps.length; i++) {
          if (allmaps[i].published == 'yes') {activemaps++}
        }
        maplist_height = (((activemaps-1)*55)+5)*autozoom
        if (maplist_height < hoehe) {
          $('#mapspanel').width(160)
        }
        else {
          $('#mapspanel').width(320)
        }
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

  // File upload via Ajax
  $("#uploadForm").on('submit', function(e){
    e.preventDefault();
    $.ajax({
      type: 'POST',
      url: 'rest/avatar/index.php',
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
        //$('#uploadStatus').html('<span style="color:#28A74B;">Avatar uploaded.<span>');
        //$('#avatarbutton').html(data);
        $('body').append('<div id="image_resize" style="position:fixed;bottom:10px;left:10px;width:750px;height:700px;background-color:rgb(51, 51, 51);border-radius:40px;z-index:2000;padding:10px;"></div>');    
        $('#image_resize').html(data);
        
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
      url: 'rest/avatar/index.php',
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
        $('#avatarbutton').html(data);
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
        url: 'rest/avatar/index.php',
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