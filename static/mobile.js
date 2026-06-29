function addMobileInterface() {
  inMobileMode = true
  $('.control_background').hide()
  $('#controlpanel').hide()
  $('.adressbook_margin').hide()
  $('.announceplate').hide()
  $('#loginicon').hide()
  $('#notifypanel').css('top','10px');
  updateMobileSearchbutton()
  updateMobileTeamlist()
  updateMobileMaplist()
  // disable default search button to listen on Enter key
  document.getElementById("searchtext").onkeyup = null;
}

function updateMobileSearchbutton() {
  // Removes remaining infoboxes from the document
  var element = document.getElementById('search_mobile');
  if (element !== null) {
    // do nothing if exists
  }
  else {
    var search_mobile =''
      + '<div id="mobile_searchbutton" style="position:fixed; left: 50%; margin-left: -3.2rem; bottom: 35px; width:7rem; height:3rem; border-radius: 1.5rem;'
      + 'background: #0000FF;color:#fff; font-size: 20px; line-height:3rem;text-align:center;'
      + 'z-index:250; opacity:0.7;'
      + 'background-image: url(images/find-open.png);background-size: contain;'
      + 'background-repeat: no-repeat; background-position:center;"'
      + ' onClick=\"showMobileSearch(true)\">'
      + '</div>';
      var newElement = document.createElement('div');
      newElement.setAttribute('id', 'search_mobile');
      newElement.innerHTML = search_mobile;
      document.body.appendChild(newElement);
  }
}

function showMobileSearch(showform) {
    
  // Remove old form if exists
  var element = document.getElementById('mobilesearchform');
  if (element !== null) {
    element.parentNode.removeChild(element);
    $('#mobile_searchbutton').css('background-color','#0000FF');
    $('#mobile_searchbutton').css('background-image','url(images/find-open.png)');
  }
  else {
    if (showform == false) {return}
    $( "#addressbook" ).hide();
    $( "#maps_mobile" ).hide();
    var searchdata =''
      + '<div id="searchdata" style="position:absolute; left:25%; top:25%; width:50%; height:50%;">'
      + '  <input type="text" id="mobileSearchtext" name="mobileSearchtext" value="" size="10" autofocus style="font-size:16px;width:80%; display:inline;"><br />'
      + '  <input type="button" value="Search" style="font-size:16px;width:80%;" onClick="startMobileSearch()">'
      + '</div>';
      var newElement = document.createElement('div');
      newElement.setAttribute('id', 'mobilesearchform');
      newElement.innerHTML = searchdata;
      document.body.appendChild(newElement);
      $('#mobilesearchform').css('position','fixed');
      $('#mobilesearchform').css('left','0px');
      $('#mobilesearchform').css('top','0px');
      $('#mobilesearchform').css('width','100%');
      $('#mobilesearchform').css('height','100%');
      $('#mobilesearchform').css('background-color','#000');
      $('#mobilesearchform').css('opacity','0.85');
      $('#mobilesearchform').css('z-index','200');
      $('#mobilesearchform').css('font-size','16px');
      $('#mobilesearchform').css('text-align','center');
      $('#searchdata').css('transform','scale(1.5)');
      $('#searchdata').css('transform-origin','50% 50%');
      // Map Enter key to start search
      $('#mobileSearchtext').keyup(function(event){
        if(event.keyCode == 13){
          startMobileSearch()
        }
      });
      $('#mobile_searchbutton').css('background-color','#FF0000');
      $('#mobile_searchbutton').css('background-image','url(images/find-close.png)');
      // clears search results by running an empty search
      $("#search_button").click()
  }
  
}

function startMobileSearch() {
  if ($('#mobileSearchtext').val() != '') {
    mobilesearchtext = $('#mobileSearchtext').val()
    document.getElementById("searchtext").value = mobilesearchtext
    showMobileSearch(false)
    $("#search_button").click()
  }
  else {
    showMobileSearch(false)
    $("#search_button").click()
  }
}

function updateMobileTeamlist() {
  var element = document.getElementById('teams_mobile');
  if (element !== null) {
    // do nothing if exists
  }
  else {
    var teams_mobile =''
      + '<div id="mobile_teambutton" style="position:fixed; right: 0px; bottom: 0px; width:4rem; height:4rem; border-radius: 2rem 0rem 0rem 0rem;'
      + 'background: #222;color:#fff; font-size: 20px; line-height:4rem;text-align:center;'
      + 'z-index:250; opacity:0.7;'
      + 'background-image: url(images/teams-mobile.png);background-size: cover;" onClick=\"toggleMobileTeamlist()\">'
      + ' '
      + '</div>';
      var newElement = document.createElement('div');
      newElement.setAttribute('id', 'teams_mobile');
      newElement.innerHTML = teams_mobile;
      document.body.appendChild(newElement);
  }
}

function toggleMobileTeamlist() {
  $( "#maps_mobile" ).hide();
  showMobileSearch(false)
  $( "#addressbook" ).toggle("slide");
}

function updateMobileMaplist() {
  var element = document.getElementById('maps_mobile');
  if (element !== null) {
    // do nothing if exists
  }
  else {
    $.ajax({
      url: 'rest/config/?mode=maps',
      async: true, 
      type: 'get',
      dataType: 'JSON',
      success: function(result){
        var allmaps = result.maps

        var maps_mobile =''
        + '<div id="mapbox" style="width:100%; height: 100%; overflow-y:scroll;"><div style="font-size:20px;margin-left:10px;">'
        + '<a href="'+root+'?map=overview">'
        + '<img src="'+logo_regular+'" style="width:40%; display:block;margin-left: auto; margin-right:auto;" alt="logo" />'
        + '</a><table id="maplist" style="width:100%; margin-top:20px; font-size:1.4rem;text-align:center;">'
        + '<tbody>'
        
        for (var i = 0; i < allmaps.length; i++) {
          if (allmaps[i].published == 'yes' && allmaps[i].mapname != 'overview') {
            if (allmaps[i].mapname == map) {
              maps_mobile += '<tr><td><a href="'+root+'?map='+allmaps[i].mapname+'" style="color:#FF7F00">'+ucWords(allmaps[i].mapname)+'</a></td></tr>'
            }
            else {
              maps_mobile += '<tr><td><a href="'+root+'?map='+allmaps[i].mapname+'" style="color:#FFFFFF">'+ucWords(allmaps[i].mapname)+'</a></td></tr>'
            }
          }
        }
        maps_mobile += '</tbody></table>'
                    + '<div style="width:100%; height:6em"></div>' //spacer
                    + '</div></div>';

        var newElement = document.createElement('div');
        newElement.setAttribute('id', 'maps_mobile');
        newElement.style.position = "fixed"
        newElement.style.background = "rgb(51, 51, 51)"
        newElement.style.left = "0px"
        newElement.style.bottom = "0px"
        newElement.style.width = "100%"
        newElement.style.maxWidth = "400px"
        newElement.style.height = "100%"
        newElement.style.opacity = "0.95"
        newElement.style.display = "none"
        newElement.innerHTML = maps_mobile;
        document.body.appendChild(newElement);
      }
    });
  }

  var element = document.getElementById('mapsbutton_mobile');
  if (element !== null) {
    // do nothing if exists
  }
  else {
    var mapsbutton_mobile =''
      + '<div id="mobile_mapbutton" style="position:fixed; left: 0px; bottom: 0px; width:4rem; height:4rem; border-radius: 0rem 2rem 0rem 0rem;'
      + 'background: #222;color:#fff; font-size: 20px; line-height:4rem;text-align:center;'
      + 'z-index:250; opacity:0.7;'
      + 'background-image: url(images/map-mobile.png);background-size: cover;" onClick=\"toggleMobileMaplist()\">'
      + ' '
      + '</div>';
      var newElement = document.createElement('div');
      newElement.setAttribute('id', 'mapsbutton_mobile');
      newElement.innerHTML = mapsbutton_mobile;
      document.body.appendChild(newElement);
  }
}

function toggleMobileMaplist() {
  $( "#addressbook" ).hide();
  showMobileSearch(false)
  $( "#maps_mobile" ).toggle("slide");
}