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
  // Only #content is still positioned imperatively here (its zoom depends on the
  // open/closed sidebar state, computed below). Every other scaled chrome element
  // is driven by the `--sc` CSS variable published in scalePages(); the layout
  // itself lives in cmaps.css ("UI scale system"). Do NOT re-add per-element
  // sizing here — change the stylesheet instead.
  var page = $('.page_content');
  
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
  // Single source of truth for the UI scale: publish the factor as the `--sc`
  // CSS custom property on <html>. All scaled chrome (header, notify bar, login
  // icon, left/right button clusters, margins, datepicker, clock, announcement
  // bar) reads it from cmaps.css. See the "UI scale system" block there.
  document.documentElement.style.setProperty('--sc', basePage.scale);
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
  // fixed at bottom:15*scale. Their box rises ~65px (toggle incl. margins / the
  // same-height health icon beside it) above that anchor, so the cluster reaches
  // about (15 + 65) * scale from the viewport bottom; add ~10px padding so the
  // toggle and health icon never touch the palette. Converted into the footer's
  // own CAPPED-zoom space that is /editSidebarScale.
  var footerEl = document.getElementById('editsidebar_footer');
  if (footerEl) {
    var reserveScreen = (15 + 65 + 10) * basePage.scale;
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
  // Apply the same 0.99 safety margin the header uses so the content never fills
  // the viewport width to the very edge. Filling it exactly caused a horizontal
  // scrollbar on tall pages (e.g. the admin panel) where a vertical scrollbar
  // appears and steals a few px of width after the zoom was computed.
  var contentZoom = (mapMaxWidth / basePage.width) * manualscale * 0.99;
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

  document.cookie = "autozoom=" + basePage.scale+'; expires=Fri, 31 Dec 9999 23:59:59 GMT; SameSite=Lax';
  document.cookie = "LeftPos=" + newLeftPos+'; expires=Fri, 31 Dec 9999 23:59:59 GMT; SameSite=Lax';
  autozoom = mapScale;
  }
});