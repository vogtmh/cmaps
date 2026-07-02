// CompanyMaps 9 AutoResizeJS
// Release date 2026-07-02
// Copyright (c) 2016-2026 by MavoDev
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
  // Layout lives in cmaps.css, driven by CSS vars published here: `--sc` for the
  // chrome and `--content-sc/left/top/height` for the map body (.page_content).
  // This function only measures viewport/sidebars/map and publishes those vars.
  var page = $('.page_content');
  
  pageWidth = $('#container').width();
  pageHeight = $('#container').height();
  scalePages();
  // Relayout hook for the search sidebar.
  window.cmapsRescale = scalePages;
  // Debounce resize handling until the window stops resizing.
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
  
  // Cap width to 2:1 so items don't get huge on ultra-wide (21:9) displays.
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
  // Single source of truth for the chrome scale (see "UI scale system" in cmaps.css).
  document.documentElement.style.setProperty('--sc', basePage.scale);
  // Search sidebar scales with the same factor as the chrome (CSS zoom keeps its
  // icons/text tracking the map's deskball size).
  var sidebarEl = document.getElementById('searchsidebar');
  if (sidebarEl) {
    sidebarEl.style.zoom = basePage.scale;
  }
  // Cap the edit sidebar scale so it doesn't grow unbounded on wide displays;
  // the reserved map width below uses the same capped factor.
  var SIDEBAR_MAX_SCALE = 1.0;
  var editSidebarScale = Math.min(basePage.scale, SIDEBAR_MAX_SCALE);
  var editEl = document.getElementById('editsidebar');
  if (editEl) {
    editEl.style.zoom = editSidebarScale;
    // Anchor top to the (uncapped) header height by dividing out the capped zoom,
    // so the taller header never overlaps the sidebar on wide screens.
    editEl.style.top = ((69 * basePage.scale) / (editSidebarScale || 1)) + 'px';
  }
  // Reserve space at the bottom of the edit sidebar so the palette clears the
  // floating bottom-right toggle/health icon cluster (~15+65px screen height,
  // +10px padding), converted into the footer's capped-zoom space.
  var footerEl = document.getElementById('editsidebar_footer');
  if (footerEl) {
    var reserveScreen = (15 + 65 + 10) * basePage.scale;
    footerEl.style.height = Math.ceil(reserveScreen / (editSidebarScale || 1)) + 'px';
  }
  var leftW = ((typeof searchSidebarWidth === 'number') && searchSidebarWidth > 0) ? searchSidebarWidth * basePage.scale : 0;
  var rightW = ((typeof editSidebarWidth === 'number') && editSidebarWidth > 0) ? editSidebarWidth * editSidebarScale : 0;
  // The map fills the width between the sidebars (not aspect-capped) and is
  // anchored to the left edge — required because #content uses CSS zoom +
  // left:mapLeftPos/contentZoom, which Safari mis-handles for non-zero offsets
  // (mapLeftPos=0 renders identically everywhere). Slack from manual zoom-out
  // falls on the right.
  var mapMaxWidth = maxWidth - leftW - rightW;
  if (mapMaxWidth < 100) { mapMaxWidth = 100; }
  // 0.99 safety margin avoids a horizontal scrollbar when a vertical one appears.
  var contentZoom = (mapMaxWidth / basePage.width) * manualscale * 0.99;
  var mapLeftPos = leftW;
  var mapScale = contentZoom; // kept for the autozoom var written below
  // Publish the map layout as CSS vars (applied by .page_content in cmaps.css;
  // seeded by the server for first paint). left/top are pre-divided by the zoom.
  page.each(function () {
    this.style.setProperty('--content-sc', contentZoom);
    this.style.setProperty('--content-left', mapLeftPos / contentZoom);
    this.style.setProperty('--content-top', (69 * basePage.scale) / contentZoom);
  });
  // Detail maps get a taller height (map image + 150px screen clearance) so the
  // bottom overlay buttons don't cover the floor plan; others keep the CSS default.
  var detailmapimage = document.getElementById('detailmapimage');
  if (detailmapimage && detailmapimage.offsetHeight) {
    var contentHeight = (detailmapimage.offsetHeight + (150 / contentZoom)) + 'px';
    page.each(function () {
      this.style.setProperty('--content-height', contentHeight);
    });
  }

  document.cookie = "autozoom=" + basePage.scale+'; expires=Fri, 31 Dec 9999 23:59:59 GMT; SameSite=Lax';
  document.cookie = "LeftPos=" + newLeftPos+'; expires=Fri, 31 Dec 9999 23:59:59 GMT; SameSite=Lax';
  autozoom = mapScale;
  }
});