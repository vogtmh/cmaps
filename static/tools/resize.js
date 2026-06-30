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
  var buttonsrightanchor = $('.buttons_right_anchor');
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
      loginicon.attr('style', 'position:fixed; bottom:10px;left:10px; z-index:3; opacity:1.0;transform:scale(' + basePage.scale + ');transform-origin:0% 100%;');
    }
    else {
      controlcontainer.attr('style', 'position:fixed; left: 0px; top: 0px; height:' + (69*basePage.scale) + 'px;width:100%;background-color:#333333;opacity:1.0;z-index:1; transition: all 300ms ease-in-out !important; display:flex; justify-content:center; align-items:flex-start;display:none;');
      controlback.attr('style', 'width:100%; height: 69px; position: fixed; top: 0px; left: 0px;background: #333333;transform-origin:50% 0%;z-index:1; transform:scaleY(' + basePage.scale + ');display:none;');
      loginicon.attr('style', 'position:fixed; bottom:10px;left:10px; z-index:3; opacity:1.0;transform:scale(' + basePage.scale + ');transform-origin:0% 100%;display:none;');
    }
  }
  else {
    controlcontainer.attr('style', 'position:fixed; left: 0px; top: 0px; height:' + (69*basePage.scale) + 'px;width:100%;background-color:#333333;opacity:1.0;z-index:1; transition: all 300ms ease-in-out !important; display:flex; justify-content:center; align-items:flex-start;');
    controlback.attr('style', 'width:100%; height: 69px; position: fixed; top: 0px; left: 0px;background: #333333;transform-origin:50% 0%;z-index:1; transform:scaleY(' + basePage.scale + ');');
    loginicon.attr('style', 'position:fixed; bottom:10px;left:10px; z-index:3; opacity:1.0;transform:scale(' + basePage.scale + ');transform-origin:0% 100%;');
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
  // Cap the edit sidebar's scale so it does not grow without bound on very wide
  // (e.g. 21:9) displays where basePage.scale climbs well above 1. The reserved
  // width below uses the same capped factor so the map gap stays in sync.
  var SIDEBAR_MAX_SCALE = 1.0;
  var editSidebarScale = Math.min(basePage.scale, SIDEBAR_MAX_SCALE);
  var editEl = document.getElementById('editsidebar');
  if (editEl) {
    editEl.style.zoom = editSidebarScale;
    // The header's real height is 69*basePage.scale. The sidebar's CSS top is
    // expressed in its own zoomed space (rendered top = cssTop * zoom), and its
    // zoom is CAPPED at 1.0 while the header scale is not. So on wide screens
    // (basePage.scale > 1) a static top:69px would render at only 69px and the
    // taller header would overlap the sidebar. Anchor it to the header height by
    // dividing out the capped zoom so the rendered top always matches.
    editEl.style.top = ((69 * basePage.scale) / (editSidebarScale || 1)) + 'px';
  }
  // Keep the trash drop zone the SAME on-screen size as the bottom-right Edit
  // toggle and clear of it. The toggle lives in #buttons_right (zoom:basePage
  // Reserve empty space at the bottom of the edit sidebar so the palette never
  // renders behind the floating bottom-right edit-mode toggle and health icon.
  // Those live in #buttons_right (zoom:basePage.scale, UNcapped) inside an anchor
  // fixed at bottom:40*scale. Their box rises ~65px (toggle incl. margins / the
  // same-height health icon beside it) above that anchor, so the cluster reaches
  // about (40 + 65) * scale from the viewport bottom; add ~10px padding so the
  // toggle and health icon never touch the palette. Converted into the footer's
  // own CAPPED-zoom space that is /editSidebarScale.
  var footerEl = document.getElementById('editsidebar_footer');
  if (footerEl) {
    var reserveScreen = (40 + 65 + 10) * basePage.scale;
    footerEl.style.height = Math.ceil(reserveScreen / (editSidebarScale || 1)) + 'px';
  }
  var leftW = ((typeof searchSidebarWidth === 'number') && searchSidebarWidth > 0) ? searchSidebarWidth * basePage.scale : 0;
  var rightW = ((typeof editSidebarWidth === 'number') && editSidebarWidth > 0) ? editSidebarWidth * editSidebarScale : 0;
  // The MAP content always fills the full width between the sidebars (or the
  // whole viewport when none are open). Unlike the header UI it is NOT subject to
  // the 2:1 aspect cap (which only keeps header icons from getting huge on 21:9),
  // so it never leaves a centered gap on wide screens. The page body has
  // overflow-x:hidden, so filling the width exactly never adds a horizontal
  // scrollbar.
  //
  // The content is ANCHORED to the left edge (or the left search sidebar), never
  // centered. This is both the desired look (fill the full width up to the
  // sidebar) and a cross-browser requirement: `#content` is positioned with CSS
  // `zoom` plus `left:mapLeftPos/contentZoom`, which relies on the browser
  // scaling the `left` offset back up by the zoom factor. Blink (Chrome) and
  // Gecko (Firefox 126+) do that so the division cancels out, but WebKit
  // (Safari) handles `zoom` + positioned offsets differently, so any non-zero
  // offset leaves a leftover horizontal gap. With no left sidebar open this makes
  // mapLeftPos = 0 -> left:0, which every browser renders identically. Any slack
  // from manual zoom-out (<100%) therefore falls entirely on the right.
  var mapMaxWidth = maxWidth - leftW - rightW;
  if (mapMaxWidth < 100) { mapMaxWidth = 100; }
  var contentZoom = (mapMaxWidth / basePage.width) * manualscale;
  var contentScreenW = basePage.width * contentZoom; // == mapMaxWidth * manualscale
  var mapLeftPos = leftW;
  var mapScale = contentZoom; // kept for the autozoom var written below
  var pageStyle = 'zoom:' + contentZoom + ';left:' + (mapLeftPos/contentZoom) + 'px;top:' + ((69*basePage.scale)/contentZoom) + 'px;';
  // For detail maps, extend the content height to the map image plus 150px
  // (real screen pixels) of clearance so the fixed bottom overlay buttons
  // never cover the lowest part of the floor plan.
  var detailmapimage = document.getElementById('detailmapimage');
  if (detailmapimage && detailmapimage.offsetHeight) {
    pageStyle += 'height:' + (detailmapimage.offsetHeight + (150/contentZoom)) + 'px;';
  }
  page.attr('style', pageStyle);
  buttonsleftanchor.attr('style', 'position:fixed; left: 10px; bottom: '+ (25*basePage.scale) + 'px; z-index:5;');
  buttonsleft.attr('style', 'position:relative; height:80px;background: transparent; zoom:' + basePage.scale + ';');
  buttonsrightanchor.attr('style', 'position:fixed; right: 10px; bottom: '+ (40*basePage.scale) + 'px; z-index:5;');
  buttonsright.attr('style', 'position:relative; display:flex; flex-direction:row; align-items:center; justify-content:flex-end; gap:8px; width:auto; background: transparent; zoom:' + basePage.scale + ';');
  
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