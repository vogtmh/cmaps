// CompanyMaps 9 AutoResizeJS
// Release date 2023-03-20
// Copyright (c) 2016-2020 by MavoDev
// see https://www.mavodev.de for more details

// This will ensure to resize all elements to the width of the browser

var pageWidth, pageHeight;

var basePage = {
  width: 1600,
  height: 600,
  scale: 1,
  scaleX: 1,
  scaleY: 1
};

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


$(function(){
  // declare all variables used in scalePages function
  var page = $('.page_content');
  var controlcontainer = $('.control_container');
  var controlcontent = $('.control_content');
  var controlback = $('.control_background');
  var notifycontainer = $('.notify_container');
  var notifycontent = $('.notify_content');
  var loginicon = $('.loginicon');
  var buttonsleftanchor = $('.buttons_left_anchor');
  var buttonsleft = $('.buttons_left');
  var buttonsright = $('.buttons_right');
  var datepicker = $('.datepicker');
  var clock = $('.clock');
  //var statsmenu = $('.statsmenu');
  var adressbook_margin = $('.adressbook_margin');
  var announcementbar_margin = $('.announcementbar_margin');
  var announcementbar = $('.announcementbar');
  var announcementbar_body = $('.announcementbar_body');
  
  pageWidth = $('#container').width();
  pageHeight = $('#container').height();
  scalePages();
  // Expose a relayout hook so the search sidebar can re-shrink/re-shift the map.
  window.cmapsRescale = scalePages;
  //using underscore to delay resize method till finished resizing window
  $(window).resize(_.debounce(function () {
  pageWidth = $('#container').width();  
  pageHeight = $('#container').height();  
  scalePages();
  }, 150));

  function manualZoom(zoom) {
  document.cookie = "zoom=" + zoom+'; SameSite=Lax';
  pageWidth = $('#container').width();  
  pageHeight = $('#container').height();  
  scalePages();
  }

  function scalePages() {  
  
  // This condition limits the width to 16:9 = 1.78, so items do not get extra big on ultra widescreen (21:9)
  var setWidth; 
  var maxWidth;
  var maxHeight;    
  maxWidth = pageWidth;
  maxHeight = pageHeight;   
  if (maxWidth/maxHeight > 2) {setWidth = maxHeight*2;}
  else setWidth = maxWidth;

  var CookieZoom = getCookie("zoom");
  if (CookieZoom == "") {CookieZoom = 100;}
  if (CookieZoom > 100) {CookieZoom = 100;}
  if (CookieZoom < 10) {CookieZoom = 10;}
  var manualscale = CookieZoom/100;

  basePage.scale = (setWidth / basePage.width)*0.99;
  setWidth = setWidth*manualscale;
  var newLeftPos = Math.abs(Math.floor((maxWidth-setWidth)/2));
  // Change this DIVs only in Desktop mode
  if (typeof detectMobile === 'function') {
    if( !detectMobile() ) {
      controlcontainer.attr('style', 'position:fixed; left: 0px; top: 0px; height:' + (69*basePage.scale) + 'px;width:100%;background-color:#333333;opacity:1.0;z-index:1; transition: all 300ms ease-in-out !important; display:flex; justify-content:center; align-items:flex-start;');
      controlback.attr('style', 'width:100%; height: 69px; position: fixed; top: 0px; left: 0px;background: #333333;transform-origin:50% 0%;z-index:1; transform:scaleY(' + basePage.scale + ');');
      loginicon.attr('style', 'position:fixed; bottom:10px;left:10px; z-index:3; opacity:0.7;transform:scale(' + basePage.scale + ');transform-origin:0% 100%;');
    }
    else {
      controlcontainer.attr('style', 'position:fixed; left: 0px; top: 0px; height:' + (69*basePage.scale) + 'px;width:100%;background-color:#333333;opacity:1.0;z-index:1; transition: all 300ms ease-in-out !important; display:flex; justify-content:center; align-items:flex-start;display:none;');
      controlback.attr('style', 'width:100%; height: 69px; position: fixed; top: 0px; left: 0px;background: #333333;transform-origin:50% 0%;z-index:1; transform:scaleY(' + basePage.scale + ');display:none;');
      loginicon.attr('style', 'position:fixed; bottom:10px;left:10px; z-index:3; opacity:0.7;transform:scale(' + basePage.scale + ');transform-origin:0% 100%;display:none;');
    }
  }
  else {
    controlcontainer.attr('style', 'position:fixed; left: 0px; top: 0px; height:' + (69*basePage.scale) + 'px;width:100%;background-color:#333333;opacity:1.0;z-index:1; transition: all 300ms ease-in-out !important; display:flex; justify-content:center; align-items:flex-start;');
    controlback.attr('style', 'width:100%; height: 69px; position: fixed; top: 0px; left: 0px;background: #333333;transform-origin:50% 0%;z-index:1; transform:scaleY(' + basePage.scale + ');');
    loginicon.attr('style', 'position:fixed; bottom:10px;left:10px; z-index:3; opacity:0.7;transform:scale(' + basePage.scale + ');transform-origin:0% 100%;');
  }
  controlcontent.attr('style', 'position:relative; top:0px;width:1600px;height:69px; z-index:2;zoom:' + basePage.scale + ';background-color:#333333;');
  notifycontainer.attr('style', 'position:fixed; left: 0px; top:' + (72*basePage.scale) + 'px; height:40px;width:100%;background-color:transparent;z-index:1; pointer-events: none;transition: all 300ms ease-in-out !important; display:flex; justify-content:center;');
  notifycontent.attr('style', 'position:relative; top:0px;width:1600px;height:0px; z-index:2;zoom:' + basePage.scale + ';background-color:transparent;pointer-events: none;');
  // The map content uses CSS `zoom` (not transform:scale) so the browser
  // re-rasterizes desk labels at the final size and fonts stay crisp. `zoom`
  // also scales left/top offsets, so divide them by the zoom factor.
  // The search sidebar (Option B) shrinks & pushes the MAP only (the top
  // header keeps its full-width scale), by removing the sidebar width from
  // the usable map area and re-centering the map in the remaining space.
  // The sidebar itself is scaled (and re-anchored below the header) with the
  // same factor as the header/UI via CSS zoom, so its icons and text track the
  // map's deskball size and it follows window resizes.
  var sidebarEl = document.getElementById('searchsidebar');
  if (sidebarEl) {
    sidebarEl.style.zoom = basePage.scale;
  }
  var sidebarOpen = (typeof searchSidebarWidth === 'number') && searchSidebarWidth > 0;
  var sidebarW = sidebarOpen ? searchSidebarWidth * basePage.scale : 0;
  var mapScale = basePage.scale;
  var mapLeftPos = newLeftPos;
  if (sidebarW > 0) {
    var mapMaxWidth = maxWidth - sidebarW;
    var mapSetWidth = (mapMaxWidth/maxHeight > 2) ? maxHeight*2 : mapMaxWidth;
    mapScale = (mapSetWidth / basePage.width)*0.99;
    mapSetWidth = mapSetWidth*manualscale;
    mapLeftPos = sidebarW + Math.abs(Math.floor((mapMaxWidth - mapSetWidth)/2));
  }
  var contentZoom = mapScale*manualscale;
  page.attr('style', 'zoom:' + contentZoom + ';left:' + (mapLeftPos/contentZoom) + 'px;top:' + ((69*basePage.scale)/contentZoom) + 'px;');
  buttonsleftanchor.attr('style', 'position:fixed; left: 10px; bottom: '+ (25*basePage.scale) + 'px; z-index:5;');
  buttonsleft.attr('style', 'position:relative; height:80px;background: transparent; zoom:' + basePage.scale + ';');
  buttonsright.attr('style', 'position:fixed; right: 10px; bottom: ' + (25*basePage.scale) + 'px; height:auto;width:80px;background: transparent; transform:scale(' + basePage.scale +');transform-origin:100% 100%;');
  
  datepicker.attr('style', 'position:relative;width:180px;height:175px;padding:15px 15px 10px 15px;background-color:#333;border-radius:10px 10px 0px 0px;display:none;zoom:' + basePage.scale + ';z-index:0;pointer-events:auto;');
  clock.attr('style', 'position:relative;width:180px;text-align:center;background-color:#333;border-radius:10px 10px 0px 0px;padding:10px;cursor:pointer;zoom:' + basePage.scale + ';z-index:1;pointer-events:auto;');
  $('.clock').hover(function(){
    $(this).css({ "background-color": "#555" });
  }, function(){
    $(this).css({ "background-color": "" });
  });
  
  //statsmenu.attr('style', 'position: fixed; z-index: 2;top: 45%;margin-top: -250px;left: -' + (620*basePage.scale) + 'px;width: 680px;height: 500px;background-color: transparent;text-align: center;padding: 20px;border-radius:0px 20px 20px 0px;opacity:1.0;transform:scale('+ basePage.scale +');transform-origin:0% 50%;');
  adressbook_margin.attr('style', 'height:' + (69*basePage.scale) + 'px;width:100%;margin-bottom:5px;background:none;');
  announcementbar_margin.attr('style', 'height:' + (69*basePage.scale) + 'px;width:100%;margin-bottom:5px;background:none;');
  announcementbar.attr('style', 'position: fixed;background:#333;right:0px;top:0px;width:590px;height:100%;opacity:0.95;display:none;transform-origin:100% 0%;transform:scaleX(' + (basePage.scale*0.7) + ');');
  announcementbar_body.attr('style', 'overflow-y: scroll;height:'+ (92*1.43/basePage.scale) +'%;width:610px;font-size:20px;transform-origin:100% 0%;transform:scaleY(' + (basePage.scale*0.7) + ');');
  
  document.cookie = "autozoom=" + basePage.scale+'; expires=Fri, 31 Dec 9999 23:59:59 GMT; SameSite=Lax';
  document.cookie = "LeftPos=" + newLeftPos+'; expires=Fri, 31 Dec 9999 23:59:59 GMT; SameSite=Lax';
  autozoom = mapScale;
  if (typeof checkMobile === 'function') {checkMobile()}
  }
});