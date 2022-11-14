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
        var healthstatus = '<a href="admin/index.php?tab=health">'
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
    // create new map instead of item on overview map
    var newdeskitem='';
      newdeskitem +='<div id="newdeskitem" class="' + deskClass + '" style="position:absolute;left:' 
                  + (newX-50) + 'px;top:' + (newY-50) + 'px;border-radius:50%;"></div>'
                  + '<div class="nameplate_edit" style="position:absolute;top:' + editX +'px;left:' + editY + 'px;border-radius:10px;">'
                  + '<div style="position:absolute; top:0px; left:0px; width:100%; font-size:1.5em;line-height:50px; height:50px;'
                  + 'background-color:#666;text-align:center;border-radius:10px 10px 0px 0px;">'+caption+'</div>'
                  + '<div id="formspace">'
                  + '<form class="createItem" style="width:80%; margin-top:60px;margin-left:10%;" enctype="multipart/form-data" action="rest/update/index.php" method="post">'
                  + '<div style="width:30%; float:left;display:inline;">Mapname</div><input type="text" style="width: 70%; float: left;display:inline;" id="apimapname" name="map">'
                  + '<div style="width:30%; float:left;display:inline;">Itemscale</div><input type="text" style="width: 70%; float: left;display:inline;" id="apimapitemscale" name="itemscale" value="1">'
                  + '<div style="width:30%; float:left;display:inline;">Published</div>'
                  + '<select id="apimappublished" style="width: 70%; float: left;display:inline;" name="published">'
                  + '<option value"yes">yes</option> <option value="no">no</option>'
                  + '</select>'
                  + '<div style="width:30%; float:left;display:inline;">MapFlag</div>'
                  + '<div id="mapflags">'
                  + '<select id="selMapflag" style="width: 70%; float: left;display:inline;" name="mapflag">'
                  + '<option value="de">de</option>'
                  + '</select></div>'
                  + '<div style="width:30%; float:left;display:inline;">Flagsize</div>'
                  + '<input type="text" style="width: 70%; float: left;display:inline;" id="apimapflagsize" name="flagsize" value="100" '
                  + 'onchange="$(\'#newdeskitem\').css(\'width\',document.getElementById(\'apimapflagsize\').value+\'px\');'
                  + '$(\'#newdeskitem\').css(\'height\',document.getElementById(\'apimapflagsize\').value+\'px\');">';
      newdeskitem+= '<div style="width:30%; float:left;display:inline;">Timezone</div>'
                  + '<div id="timezones">'
                  + '<select id="selTimezone" style="width: 70%; float: left;display:inline;" name="timezone">'  
                  + '<option value="">-- Select a timezone -- </option>'  
                  + '</select></div>';
      newdeskitem+= '<div style="width:30%; float:left;display:inline;">Address</div><input type="text" style="width: 70%; float: left;display:inline;" id="apimapaddress" name="address" value="-">'
                  + '<div style="width:30%; float:left;display:inline;">x</div><input type="text" style="width: 70%; float: left;display:inline;" id="apimapx" name="x" value="'+(newX-50)+'">'
                  + '<div style="width:30%; float:left;display:inline;">y</div><input type="text" style="width: 70%; float: left;display:inline;" id="apimapy" name="y" value="'+(newY-50)+'">'
                  + '<div style="width:30%; float:left;display:inline;">Floorplan</div>'
                  + '<input type="file" id="i_file" accept="image/png" name="image" size="30"><img src="" width="400" style="display:none;" id="testbild" /><br /><div id="disp_tmp_path"></div>'
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
      var tzOutput = '<select id="selTimezone" style="width: 70%; float: left;display:inline;" name="timezone">'  
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
        var mfOutput = '<select id="selMapflag" style="width: 70%; float: left;display:inline;" name="mapflag" onchange="switchMapflag()">'  
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
    
    // calc page scaling
    pageScale = $('#container').width()/targetScreenWidth;
    var div = $('#content').css('transform');
    var values = div.split('(')[1];
    values = values.split(')')[0];
    values = values.split(',');
    var a = values[0];
    var b = values[1];

    var pageScale = Math.sqrt(a*a + b*b);

    // calculate the new cursor position:
    diffX = (e.clientX-startJsX)/pageScale;
    diffY = (e.clientY-startJsY)/pageScale;
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
      itemx = parseInt(attr.x)+parseInt(diffItemX);
      itemy = parseInt(attr.y)+parseInt(diffItemY);
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