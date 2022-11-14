<?php
  ob_start();
  session_start(); 
  
  # Loading shared functions and config
  include 'shared.php';
?>
<!DOCTYPE HTML>
<!-- ===================================================================
  
  CompanyMaps 8.0 Client
  Release date 2022-11-14
  Copyright (c) 2016-2022 by MavoDev
  see https://www.mavodev.de for more details
  
==================================================================== -->

<html lang="de">
<head>
  <meta name="generator" content="HTML Tidy for Windows (vers 22 March 2008), see www.w3.org">

  <title><?php echo $apptitle?></title>
  <!-- <meta http-equiv="Content-Type" content="text/html; charset=utf-8"> -->
  <meta charset="utf-8">
  <link rel="stylesheet" type="text/css" href="cmaps80.css">
  <link href='https://fonts.googleapis.com/css?family=Roboto' rel='stylesheet' type='text/css'>
  <!-- FAVICONS -->
  <link rel="apple-touch-icon" sizes="57x57" href="favicons/apple-touch-icon-57x57.png">
  <link rel="apple-touch-icon" sizes="60x60" href="favicons/apple-touch-icon-60x60.png">
  <link rel="apple-touch-icon" sizes="72x72" href="favicons/apple-touch-icon-72x72.png">
  <link rel="apple-touch-icon" sizes="76x76" href="favicons/apple-touch-icon-76x76.png">
  <link rel="apple-touch-icon" sizes="114x114" href="favicons/apple-touch-icon-114x114.png">
  <link rel="apple-touch-icon" sizes="120x120" href="favicons/apple-touch-icon-120x120.png">
  <link rel="apple-touch-icon" sizes="144x144" href="favicons/apple-touch-icon-144x144.png">
  <link rel="apple-touch-icon" sizes="152x152" href="favicons/apple-touch-icon-152x152.png">
  <link rel="apple-touch-icon" sizes="180x180" href="favicons/apple-touch-icon-180x180.png">
  <link rel="apple-touch-startup-image" href="favicons/android-chrome-512x512.png">
  <link rel="icon" type="image/png" href="favicons/favicon-32x32.png" sizes="32x32">
  <link rel="icon" type="image/png" href="favicons/android-chrome-192x192.png" sizes="192x192">
  <link rel="icon" type="image/png" href="favicons/favicon-96x96.png" sizes="96x96">
  <link rel="icon" type="image/png" href="favicons/favicon-16x16.png" sizes="16x16">
  <link rel="manifest" href="favicons/manifest.json">
  <link rel="mask-icon" href="favicons/safari-pinned-tab.svg" color="#5BBAD5">
  <link rel="shortcut icon" href="favicons/favicon.ico">
  <meta name="msapplication-TileColor" content="#2d89ef">
  <meta name="msapplication-TileImage" content="favicons/mstile-144x144.png">
  <meta name="msapplication-config" content="favicons/browserconfig.xml">
  <meta name="theme-color" content="#000000">
  <meta name="apple-mobile-web-app-capable" content="yes">
  <meta name="mobile-web-app-capable" content="yes">
  <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=4.0, user-scalable=yes">
  <meta name="apple-mobile-web-app-status-bar-style" content="black-translucent">
  <!-- SCRIPTS -->
  <script src="tools/jquery3.js"></script>
  <script src="tools/jquery-migrate-1.4.1.min.js"></script>
  <script src="user80.js"></script>
  <script src="tools/resize80.js"></script>
  <script src="tools/underscore.js"></script>
</head>


<?php  
# INITIALIZING PAGE  

  # Initialize page variables
  $results = 0;
  if (isset($_COOKIE['zoom']) && is_numeric($_COOKIE['zoom'])) {
    $zoom = $_COOKIE['zoom'];
    if ($zoom>100 || $zoom<10) {$zoom=100; setcookie ("zoom", 100, mktime (0, 0, 0, 12, 31, 2030), "/");}
  }

  if (isset($_COOKIE['autozoom'])) {
    $autozoom = $_COOKIE['autozoom'];
  }
  else {
    $autozoom = 1;
  }

  if (isset($_COOKIE['LeftPos'])) {
    $LeftPos = $_COOKIE['LeftPos'];
  }
  else {
    $LeftPos = 0;
  }
  
  
  # Auto-set variables
  $path = $_SERVER['PHP_SELF']; $file = basename($path);
  $tab = '&nbsp;&nbsp;&nbsp;'; 
  $page = $_SERVER['PHP_SELF'];
  $applysettings = 0;
  $editmode = 0;
  $findteam = '';

  # Initialize POST and GET Variables
  if ($_SERVER['REQUEST_METHOD'] == "POST") {
    if (isset($_POST['findme'])) {$findme = $_POST['findme'];} else {$findme = '';}
    if (isset($_POST['map'])) {
      $map = $_POST['map'];
    }
    else {
      if (isset($_COOKIE['map'])) {$map = $_COOKIE['map'];} else {$map='';}
    }
    if (isset($_POST['setting_nodescription'])) {$setting_nodescription=$_POST['setting_nodescription'];} else {$setting_nodescription = 0;}
    if (isset($_POST['setting_desknumbers'])) {$setting_desknumbers=$_POST['setting_desknumbers'];} else {$setting_desknumbers = 0;}
    if (isset($_POST['setting_shownames'])) {$setting_shownames=$_POST['setting_shownames'];} else {$setting_shownames = 0;}
    if (isset($_POST['setting_highlightleaders'])) {$setting_highlightleaders=$_POST['setting_highlightleaders'];} else {$setting_highlightleaders = 0;}
    if (isset($_POST['setting_printmode'])) {$setting_printmode=$_POST['setting_printmode'];} else {$setting_printmode = 0;}
    if (isset($_POST['setting_noanimation'])) {$setting_noanimation=$_POST['setting_noanimation'];} else {$setting_noanimation = 0;}
    if (isset($_POST['setting_dailyvisitors'])) {$setting_dailyvisitors=$_POST['setting_dailyvisitors'];} else {$setting_dailyvisitors = 0;}
    if (isset($_POST['setting_saml'])) {$setting_saml=$_POST['setting_saml'];} else {$setting_saml = 0;}
    if (isset($_POST['applysettings'])) {$applysettings=$_POST['applysettings'];} else {$applysettings = 0;}
  }    

  if ($_SERVER['REQUEST_METHOD'] == "GET") {
    if (isset($_GET['map'])) {
      $map = $_GET['map'];
    } 
    else {
      if (isset($_COOKIE['map'])) {$map = $_COOKIE['map'];} else {$map='';}
    } 
    if (isset($_GET['id'])) {$getid = $_GET['id'];} else {$getid = '';}
    if (isset($_GET['glabel'])) {$glabel = $_GET['glabel'];} else {$glabel = '';}
    if (isset($_GET['teamlabel'])) {$teamlabel = $_GET['teamlabel'];} else {$teamlabel = '';}
    if (isset($_GET['showstats'])) {$showstats = $_GET['showstats'];} else {$showstats = '';}
    if (isset($_GET['findme'])) {$findme = $_GET['findme'];} else {$findme = '';}
    if (isset($_GET['findteam'])) {$findteam = $_GET['findteam'];} else {$findteam = '';}
    if (isset($_GET['zoom'])) {
      $zoom = $_GET['zoom'];
      if (is_numeric($zoom)) {
        if ($zoom < 10) {$zoom = 10;} if ($zoom > 100) {$zoom = 100;}
        setcookie ("zoom", $zoom, mktime (0, 0, 0, 12, 31, 2030), "/");
      }
      else {
        $zoom = 100;   
        setcookie ("zoom", 100, mktime (0, 0, 0, 12, 31, 2030), "/");
      }
    }
  }

  echo "<script>console.log('map: $map');</script>";

  # FindTeam Function to search all team members by URL
  if ($findteam != '') {
    $dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
    mysqli_query($dbLink,"SET NAMES 'utf8'");
    $searchteams = mysqli_query($dbLink, "SELECT * FROM `config_teams` WHERE `teamname` = '$findteam'");
    
    $stnum   = mysqli_num_rows ($searchteams);
    if ($stnum == 1) {
      $teamname    = mysqli_result($searchteams,0,1);
      $teammembers = mysqli_result($searchteams,0,2);
      if ($teammembers != "") {
        $findme = $teammembers;
        $teamlabel=$teamname;
      }
    }
  }

  # Automatically set zoomlevel on first usage (= no cookie is set, no parameter is specified)
  if (isset($_COOKIE['zoom'])==0 && $_GET['zoom'] == "") { 
    setcookie ("zoom", 100, mktime (0, 0, 0, 12, 31, 2030), "/");
    $zoom = 100;
  }

  # Apply new settings if requested
  if ($applysettings == 1) {
    setcookie ("setting_nodescription", $setting_nodescription, mktime (0, 0, 0, 12, 31, 2030), "/");
    setcookie ("setting_desknumbers", $setting_desknumbers, mktime (0, 0, 0, 12, 31, 2030), "/"); 
    setcookie ("setting_shownames", $setting_shownames, mktime (0, 0, 0, 12, 31, 2030), "/"); 
    setcookie ("setting_highlightleaders", $setting_highlightleaders, mktime (0, 0, 0, 12, 31, 2030), "/"); 
    setcookie ("setting_printmode", $setting_printmode, mktime (0, 0, 0, 12, 31, 2030), "/");
    setcookie ("setting_noanimation", $setting_noanimation, mktime (0, 0, 0, 12, 31, 2030), "/");
    setcookie ("setting_dailyvisitors", $setting_dailyvisitors, mktime (0, 0, 0, 12, 31, 2030), "/");
    setcookie ("setting_saml", $setting_saml, mktime (0, 0, 0, 12, 31, 2030), "/");
  }

  # Otherwise check if settings cookies are available
  if ($applysettings == 0) {
    if (isset($_COOKIE['setting_nodescription']) && $_COOKIE['setting_nodescription']==1) {$setting_nodescription=1;} else {$setting_nodescription=0;}
    if (isset($_COOKIE['setting_desknumbers']) && $_COOKIE['setting_desknumbers']==1) {$setting_desknumbers=1;} else {$setting_desknumbers=0;}
    if (isset($_COOKIE['setting_shownames']) && $_COOKIE['setting_shownames']==1) {$setting_shownames=1;} else {$setting_shownames=0;}
    if (isset($_COOKIE['setting_highlightleaders']) && $_COOKIE['setting_highlightleaders']==1) {$setting_highlightleaders=1;} else {$setting_highlightleaders=0;}
    if (isset($_COOKIE['setting_printmode']) && $_COOKIE['setting_printmode']==1) {$setting_printmode=1;} else {$setting_printmode=0;}
    if (isset($_COOKIE['setting_noanimation']) && $_COOKIE['setting_noanimation']==1) {$setting_noanimation=1;} else {$setting_noanimation=0;}
    if (isset($_COOKIE['setting_dailyvisitors']) && $_COOKIE['setting_dailyvisitors']==1) {$setting_dailyvisitors=1;} else {$setting_dailyvisitors=0;}
    if (isset($_COOKIE['setting_saml']) && $_COOKIE['setting_saml']==1) {$setting_saml=1;} else {$setting_saml=0;}
  }

  if (isset($_COOKIE['setting_usermode'])) {$setting_usermode=$_COOKIE['setting_usermode'];} else {$setting_usermode='edit';}
  
  # Create body
  # if print mode was enabled, the alternative map is used and no background is displayed
  if ($setting_printmode == 1) {
    echo '<body style="min-height:1920px; width:100%; margin:0px; background:none;transform-origin:50% 0%; overflow-x:hidden;">';
  }
  else {
    echo '<body style="min-height:1920px; width:100%; margin:0px; overflow-x:hidden;">';
  }
  
  # Set view based on map variable. NULL = overview
  if ($map == 'goeppingen2') {$map = 'goeppingen';}
  if (in_array($map, $maplist)){
    $dbTable = 'desks_'.$map;
    $arr_cookie_options = array (
      'expires' => mktime (0, 0, 0, 12, 31, 2030),
      'path' => '/',
      'samesite' => 'lax'
    );
    setcookie ("map", $map, $arr_cookie_options);
  }
  else {
    header("Location: ".$page."?map=".$map_default."");
  }

  ## Items may be scaled seperately for each map
  # This is defined in db table config_maplist for each map

  if (${"itemscale_".$map} != 0) {
    $itemscale = ${"itemscale_".$map};
  }
  else
  {
    $itemscale = 1;
  }
  
  # Check for XSS Attacks. Skips the search if the searchbox contains a bad character.
  function xsscheck($str)
  {
    $blacklist = array ('<','>','[',']','&amp;','&lt;','&gt;','&quot;','&#x27;','&#x2F;');  
    foreach($blacklist as $a) {
      if (stripos($str,$a) !== false) return true;
    }
  return false;
  }
  
  if (xsscheck($findme)) {$findme = "";}

  # If session is still present, enable edit mode
  if (isset($_SESSION['username'])) {
    $editmode = 1; 
    $_SESSION['editmode'] = base64_encode($_SESSION['username']);
  }

  # Check if iPad / iPhone is used, needs some tweaks
  $iOS = (bool) strpos($_SERVER['HTTP_USER_AGENT'],'iPhone') || strpos($_SERVER['HTTP_USER_AGENT'],'iPad');

# PERFORM OPERATIONS ON PAGE-LOAD

  $dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
  mysqli_query($dbLink,"SET NAMES 'utf8'");

# CREATE PAGE CONTENT

  echo '<div id="container"> </div>';
  ## Page Header
  # Includes all controls with a description next to it 

  # Header of Page - scales vertically with the page
  echo '<div class="control_background" style="width:100%; height: 69px; position: fixed; top: 0px; left: 0px;background: #333333; z-index:1; transform:scaleY('.$autozoom.')"> </div>';
  echo '<div id="controlpanel" class="control_container" style="position:fixed; left: 0px; top: 0px; height:69px;width:100%;background-color:#333333;opacity:1.0;z-index:1;transform:scale('.$autozoom.');transform-origin:50% 0%;">';
  # Center header box with controls - scales horizontally with the page
  echo '<div class="control_content" id="control_content" style="position: absolute; left:50%; margin-left:-'.($targetScreenWidth/2).'px; top:0px;width:'.$targetScreenWidth.'px;height:69px; z-index:2;transform-origin:50% 0%;background-color:#333333">';
  # Page Logo
  echo '     
      <div class="headeritem" id="logocontrol" style="width:200px; background: transparent;">
        <a href="'.$page.'?map='.$map_default.'"><img src="'.$logo_regular.'" height="65" alt="" onmouseover=this.src="'.$logo_hover.'" onmouseout=this.src="'.$logo_regular.'" style="cursor:pointer;"/></a>
      </div>';
  # Search box
  echo ' 
       <div class="headeritem" id="searchcontrol" style="width:370px;">';
        if ($setting_nodescription == 0) {echo '<span class="headeritem_text">Search</span>';}
  echo '      <span style="width:100px;line-height:65px;float:left; display:inline;">
          <div style="width:320px;">';
          # disable autofocus for iOS devices
          if ($iOS == true) {
            echo '<input type="text" name="findme" id="searchtext" value="'.$findme.'" size="10" style="width:150px; margin-right:5px; ">';
          }
          else {
            echo '<input type="text" name="findme" id="searchtext" value="'.$findme.'" size="10" autofocus style="width:150px; margin-right:5px;">';
          }  
  echo '        <input type="hidden" name="map" value="'.$map.'">';
          # Colorize find button if search was done
          if ($findme != "")	{ 
            echo '<input type="button" id="search_button" value="Find" style="width:120px; ">';
          }
          else { 
            echo '<input type="button" id="search_button" value="Find" style="width:120px;background-color: #0979D8; ">';
          }
  echo '        </div>
        </span>
      </div>';

  echo '    <script>
      $(\'#search_button\').click(function() {  
        searchDesks()
      });   
      </script>
  ';
  # Zoom controls
  echo '
      <div class="headeritem" id="zoomcontrol" style="width:160px;">';
        if ($setting_nodescription == 0) {echo '<span class="headeritem_text">Zoom</span>';}
  echo '      <span class="headeritem_picture"><a href="'.$page.'?map='.$map.'&zoom='.($zoom+10).'"><img src="images/zoom_in_blue.png" width="45" alt="" /></a></span>
        <span class="headeritem_picture"><a href="'.$page.'?map='.$map.'&zoom='.($zoom-10).'"><img src="images/zoom_out_blue.png" width="45" alt="" /></a></span>  
      </div>';
  # Location selector
  echo '
      <div class="headeritem" id="locationcontrol" style="width:240px;">';
        if ($setting_nodescription == 0) {echo '<span class="headeritem_text">Location</span>';}
  echo '
        <a style="cursor:pointer" id="toggle_maps">
        <div class="headeritem_maps" style="margin-top:10px; margin-bottom:0px;">'.ucfirst($map).'</div>
        </a>';      
  echo '      
      </div>';

  # Floor selector
    echo '
        <div class="headeritem" id="floorcontrol" style="width:260px;">';
          # Get list of floors in current map
          if ($map != "overview") {
          $floorquery = mysqli_query($dbLink, "SELECT * FROM `$dbTable` WHERE `desktype` = 'floor'");
          $floornum   = mysqli_num_rows($floorquery);
          } else {
            $floornum = 0;
          }
          # description
          if ($setting_nodescription == 0 && $floornum != 0) {echo '<span class="headeritem_text">Floor</span>';}
    echo '      <span style="width:200px;line-height:65px;float:left; display:inline;">';
          # Create buttons for the floors
          for ($i = 0; $i < $floornum; $i++) {
            $floorid  = mysqli_result($floorquery,$i,0);
            $employee = mysqli_result($floorquery,$i,5);
            echo '
            <a href="#'.$floorid.'" style="text-decoration: none;">
            <div class="headeritem_floors">'.$employee.'</div>
            </a>';
          }   
            
    echo '      </span>
        </div>';  


  # Settings and help buttons
  echo '
      <div class="headeritem" id="settingscontrol" style="width:210px; float:right;">';
        if ($setting_nodescription == 0) {echo '<span class="headeritem_text">Extras</span>';}
  echo '  <span class="headeritem_picture" style="float:right;">
            <a style="cursor:pointer" id="toggle_addressbook"><img id="addressbook_img" src="images/addressbook.png" width="45" alt="" /></a>
          </span>
          <span class="headeritem_picture" style="float:right;">
            <a style="cursor:pointer" id="toggle_announcementbar"><img id="announce_img" src="images/announce.png" width="45" alt="" /></a>
          </span>
          <span class="headeritem_picture" style="float:right;">
            <a style="cursor:pointer" id="toggle_settings"><img src="images/settings.png" width="45" alt="" /></a>
          </span>';   
  
  echo '    </div>';
  /* Settings panel - to cover some advanced options*/

    echo '
      <div class="headeritem" id="settingspanel" style="width:500px;height:auto;float:right;background-color:#333333; display:none; border-radius: 0px 0px 10px 10px;">
        <form name="Settings" action="'.$page.'?map='.$map.'" method="post">';
    echo ' 
        <div class="settingsitem">
          <a href="docs/index.htm" target="_blank"><span class="settingsitem_linkbutton">Help</span></a>
          <a href="docs/index.htm#content_changelog" target="_blank"><span class="settingsitem_linkbutton" style="float:right">Changelog</span></a>
        </div>
    ';
    echo '    
        <div class="settingsitem">
          <span class="settingsitem_text">Disable button description - Cleaner look</span>
          <div class="settingsitem_option">
            <div class="switch">';
              if ($setting_nodescription == 1) {
                echo '<input id="cmn-toggle-1" class="cmn-toggle cmn-toggle-round-flat" type="checkbox" name="setting_nodescription" value="1" checked>';
              }
              else {
                echo '<input id="cmn-toggle-1" class="cmn-toggle cmn-toggle-round-flat" type="checkbox" name="setting_nodescription" value="1">';
              }
    echo '          <label for="cmn-toggle-1"></label>
            </div> 
          </div>
        </div>';
    echo '    
        <div class="settingsitem">
          <span class="settingsitem_text">Enable Print mode - black on white</span>
          <div class="settingsitem_option">
            <div class="switch">';
              if ($setting_printmode == 1) {
                echo '<input id="cmn-toggle-2" class="cmn-toggle cmn-toggle-round-flat" type="checkbox" name="setting_printmode" value="1" checked>';
              }
              else {
                echo '<input id="cmn-toggle-2" class="cmn-toggle cmn-toggle-round-flat" type="checkbox" name="setting_printmode" value="1">';
              }
    echo '          <label for="cmn-toggle-2"></label>
            </div> 
          </div>
        </div>';
    echo '
        <div class="settingsitem">
          <span class="settingsitem_text">Show all desk numbers</span>
          <div class="settingsitem_option">
            <div class="switch">';
              if ($setting_desknumbers == 1) {
                echo '<input id="cmn-toggle-3" class="cmn-toggle cmn-toggle-round-flat" type="checkbox" name="setting_desknumbers" value="1" checked>';
              }
              else {
                echo '<input id="cmn-toggle-3" class="cmn-toggle cmn-toggle-round-flat" type="checkbox" name="setting_desknumbers" value="1">';
              }
    echo '          <label for="cmn-toggle-3"></label>
            </div> 
          </div>
        </div>
        <div class="settingsitem">
        <span class="settingsitem_text">Show all names</span>
        <div class="settingsitem_option">
          <div class="switch">';
            if ($setting_shownames == 1) {
              echo '<input id="cmn-toggle-4" class="cmn-toggle cmn-toggle-round-flat" type="checkbox" name="setting_shownames" value="1" checked>';
            }
            else {
              echo '<input id="cmn-toggle-4" class="cmn-toggle cmn-toggle-round-flat" type="checkbox" name="setting_shownames" value="1">';
            }
  echo '          <label for="cmn-toggle-4"></label>
          </div> 
        </div>
      </div>
        <div class="settingsitem">
          <span class="settingsitem_text">Highlight managers</span>
          <div class="settingsitem_option">
            <div class="switch">';
              if ($setting_highlightleaders == 1) {
                echo '<input id="cmn-toggle-5" class="cmn-toggle cmn-toggle-round-flat" type="checkbox" name="setting_highlightleaders" value="1" checked>';
              }
              else {
                echo '<input id="cmn-toggle-5" class="cmn-toggle cmn-toggle-round-flat" type="checkbox" name="setting_highlightleaders" value="1">';
              }
    echo '          <label for="cmn-toggle-5"></label>
            </div> 
          </div>
          
        </div>'; 
    echo '
        <div class="settingsitem">
          <span class="settingsitem_text">Disable animations</span>
          <div class="settingsitem_option">
            <div class="switch">';
              if ($setting_noanimation == 1) {
                echo '<input id="cmn-toggle-9" class="cmn-toggle cmn-toggle-round-flat" type="checkbox" name="setting_noanimation" value="1" checked>';
              }
              else {
                echo '<input id="cmn-toggle-9" class="cmn-toggle cmn-toggle-round-flat" type="checkbox" name="setting_noanimation" value="1">';
              }
    echo '          <label for="cmn-toggle-9"></label>
            </div> 
          </div>  
        </div>';  
    echo '
        <div class="settingsitem">
          <span class="settingsitem_text">Show daily visitors</span>
          <div class="settingsitem_option">
            <div class="switch">';
              if ($setting_dailyvisitors == 1) {
                echo '<input id="cmn-toggle-10" class="cmn-toggle cmn-toggle-round-flat" type="checkbox" name="setting_dailyvisitors" value="1" checked>';
              }
              else {
                echo '<input id="cmn-toggle-10" class="cmn-toggle cmn-toggle-round-flat" type="checkbox" name="setting_dailyvisitors" value="1">';
              }
    echo '          <label for="cmn-toggle-10"></label>
            </div> 
          </div>  
        </div>'; 
    echo '
        <div class="settingsitem">
          <span class="settingsitem_text">Disable SAML Auth</span>
          <div class="settingsitem_option">
            <div class="switch">';
              if ($setting_saml == 1) {
                echo '<input id="cmn-toggle-11" class="cmn-toggle cmn-toggle-round-flat" type="checkbox" name="setting_saml" value="1" checked>';
              }
              else {
                echo '<input id="cmn-toggle-11" class="cmn-toggle cmn-toggle-round-flat" type="checkbox" name="setting_saml" value="1">';
              }
    echo '          <label for="cmn-toggle-11"></label>
            </div> 
          </div>  
        </div>'; 
    echo '    
        <input type="hidden" name="applysettings" value="1">
        <input type="submit" value="Apply settings" style="font-size:1.2em; margin-top:20px;margin-left:25%;width:50%">
        </form>
      </div>
      ';

  /* Location panel - to cover some advanced options*/

  echo '<div class="headeritem" id="mapspanel" style="width:160px;height:auto;float:left;background-color:#333333;margin-top:0px;';
  if ($setting_nodescription == 1) {echo 'left:755px; ';} else {echo 'left:827px; ';}
  echo 'padding-bottom: 5px; border-radius: 0px 0px 10px 10px; display:none;">';
  # Create buttons for the maps
  for ($s = 0; $s < count($maplist); $s++) {
    if ($maplist[$s] != $map) {
      echo '
        <a href="'.$page.'?map='.$maplist[$s].'" style="text-decoration: none;">
        <div class="headeritem_maps">'.ucfirst($maplist[$s]).'</div>
        </a>';
    }
  }

  echo '</div>';    
   
  
  # Close Header DIVs
  echo '
     </div>
     </div>';

  # Notification Area   
  echo '<div id="notifypanel" class="notify_container" style="position:fixed; left: 0px; top: '.(72*$autozoom).'px; height:0px;width:100%;background-color:transparent;z-index:1;transform:scale('.$autozoom.');transform-origin:50% 0%;pointer-events: none;">';
  # Center header box with controls - scales horizontally with the page
  echo '<div id="notifycontent" class="notify_content" style="position: absolute; left:50%; margin-left:-'.($targetScreenWidth/2).'px; top:0px;width:'.$targetScreenWidth.'px;height:0px; z-index:2;transform-origin:50% 0%;background-color:transparent;pointer-events: none;">';
    echo '<span style="position:relative; width:454px;height:0px;display:inline;float:left;line-height: 40px;">&nbsp;</span>';
  echo '</div></div>';
  
  # Create Content Area
  echo '
  <div id="content" class="page_content" style="transform:scale('.$zoom/100*$autozoom.');left:'.$LeftPos.'px;top:'.(69*$autozoom).'px;">';
  # Creates an overview map if no map or map=overview was specified

  if ($map == "overview") {
    echo '  <div id="overviewmap" style="text-align:center;position:absolute;width:'.$targetScreenWidth.'px;">';
    if ($setting_printmode == 1) {
      echo '<img src="maps/'.$map.'.png" style="opacity:0.7;position:absolute; top:20px; left:0px; width: '.$targetScreenWidth.'px; -webkit-filter: invert(1);
      filter: invert(1);" alt="" onclick="hideMapplate();" />';
    }
    else {
      echo '<img src="maps/'.$map.'.png" style="opacity:0.7;position:absolute; top:20px; left:0px; width: '.$targetScreenWidth.'px;" alt="" onclick="hideMapplate();" />';
    }
    echo '</div>';
  }
  # Otherwise display regional page  
  else {
    if ($setting_printmode == 1) {
      echo '<img src="maps/'.$map.'.png" style="opacity:0.7;position:absolute; left:0px; width: '.$targetScreenWidth.'px; -webkit-filter: invert(1);
      filter: invert(1);" alt="" />';
    }
    else {
      echo '<img src="maps/'.$map.'.png" style="opacity:0.7;position:absolute; left:0px; width: '.$targetScreenWidth.'px;" alt="" />';
    }
    echo '<div id="deskitems"></div>';
    echo '<div id="meetingitems"></div>';
  }

  # Show group label
  echo '  <div id= "group_label" style="position: absolute; color: #FF7F00; text-align:center; width: 200px; height: 70px; left:0px;top:0px;visibility:hidden; font-size:50px; font-weight: bold; z-index:100;background-color: rgba(0, 0, 0, 0.3);"> 
    </div>';
  
  # Show group borders
  echo '  <div id= "group_border" style="position: absolute; border-radius: 25px;border: 8px solid #FF7F00; background-color: rgba(255, 127, 0, 0.2); padding: 20px; width: 200px; height: 150px; left:0px;top:0px;visibility:hidden;"> 
    </div>';
  
  # Prepare PHP variables for JavaScript usage
  
  if (isset($_SESSION['username'])) {
    if (permcheck($_SESSION['username'], 'desks') > 1) {
      echo '<script src="admin80.js"></script>';
    }
    else {
      echo '<script src="tools/empty.js"></script>';
    }
  }
  ?>

  <script>

  <?php 
  # Check some variables to display extra content
  echo 'var targetScreenWidth = '.$targetScreenWidth.';';
  echo 'var map = "'.$map.'";';
  echo 'var autozoom = "'.$autozoom.'";';
  echo 'var zoom = "'.($zoom/100).'";';
  echo 'var itemscale = '.$itemscale.';';
  echo 'var departments = '.json_encode($department_list, JSON_FORCE_OBJECT).';';
  echo 'var root = "'.$page.'";';
  echo 'var logo_regular = "'.$logo_regular.'";';
  echo 'var logo_hover = "'.$logo_hover.'";';
  echo 'var setting_usermode = "'.$setting_usermode.'";';
  echo 'var teamsContact = "'.$teamsContact.'";';
  echo 'var teamsChannel = "'.$teamsChannel.'";';
  echo 'var domain = "'.$domain.'";';

  if (isset($_SESSION['username'])) {
    echo 'username = "'.$_SESSION['username'].'";';
    $fullname = $_SESSION['fullname'];
    $telephonenumber = $_SESSION['telephonenumber'] ;
    $mail = $_SESSION['mail'];
    if (permcheck($_SESSION['username'], 'desks') > 1) {
      $token = (strrev(date("Ymd")) + date("Ymd"));
      echo 'token = '.$token.';username = "'.$_SESSION['username'].'"; console.log("Token: " + token);';
    }
  }
  
  if ($setting_highlightleaders == 1) {
    echo 'var setting_highlightleaders = 1;';
  }
  else {
    echo 'var setting_highlightleaders = 0;';
  }

  if ($setting_shownames == 1) {
    echo 'var setting_shownames = 1;';
  }
  else {
    echo 'var setting_shownames = 0;';
  }

  if ($setting_desknumbers == 1) {
    echo 'var setting_desknumbers = 1;';
  }
  else {
    echo 'var setting_desknumbers = 0;';
  }
  if ($setting_printmode == 1) {
    echo 'var setting_printmode = 1;';
  }
  else {
    echo 'var setting_printmode = 0;';
  }
  if ($setting_noanimation == 1) {
    echo 'var setting_noanimation = 1;';
  }
  else {
    echo 'var setting_noanimation = 0;';
  }
  # Find timezone on mapconfig table
  mysqli_query($dbLink,"SET NAMES 'utf8'");
  $map_timezone = mysqli_query($dbLink, "SELECT * FROM `config_maplist` WHERE `mapname` LIKE '$map';");
  $num_tz   = mysqli_num_rows ($map_timezone);

  if ($num_tz > 0) {
    $region = mysqli_result($map_timezone,0,6);
    echo "var region = '$region';";
  }
  else {
    $region = "Europe/Berlin";
  }
  
  $tz=timezone_open($region);
  $GMT=date_create("now",timezone_open("Europe/London"));
  echo 'var tzOffset = "'.(timezone_offset_get($tz,$GMT)/3600).'";';
  ?>

  console.log("Timezone set to: "+region);
  var clockID;
  var d = new Date();  
  //get the timezone offset from local time in minutes
  var tzDifference = tzOffset * 60 + d.getTimezoneOffset();
  //convert the offset to milliseconds, add to targetTime, and make a new Date
  var offset = tzDifference * 60 * 1000;

  if (map == "overview") {
    updateOverview();
  }
  else {
    StartClock();
    updateDesks();
    updateBookings();
    //getPrinterStatus();
    setInterval(getMeetingStatus, 60000);
    //setInterval(getPrinterStatus, 300000);
    setInterval(updateDesks, 300000);
    setInterval(updateBookings, 60000);
  }
  
  checkMobile();
  updateChangeTracker(); 
  setInterval(updateChangeTracker, 300000);
  setInterval(updateTeams, 300000);
  addStat();
  </script>

<?php

echo '</div>'; # content

  # User menu
  if (isset($_SESSION['username'])) {
    echo '<div class="buttons_left" style="position:fixed; left: 10px; bottom: '.(25*$autozoom).'px; height:80px;background: transparent; transform:scale('.$autozoom.');transform-origin:0% 100%;z-index:5;">';
      
      # Background for advanced personal settings
      echo '<div id="personal_menu" style="position:fixed; left: 0px; bottom: 0px; width:350px; height:270px;background-color:#333333; border-radius:40px; visibility:hidden;">';
      
      # Logout button
      echo '<div style="position:fixed; left: 20px; bottom: 210px; width:80px; height:10px;background: transparent;font-size:12px;text-align:center;">Logout</div>';
      echo '<div style="position:fixed; left: 20px; bottom: 120px; width:80px; height:80px;background: transparent;">';
        echo '<img src="images/logout3.png" style="width:100%;height:100%;cursor:pointer;" alt="" onmouseover=this.src="images/logout3_on.png" onmouseout=this.src="images/logout3.png" onClick="logoutUser()"/>';
      echo '</div>';

      # Upload button
      echo '<div style="position:fixed; left: 130px; bottom: 210px; width:80px; height:10px;background: transparent;font-size:12px;text-align:center;">Upload image</div>';
      echo '<div style="position:fixed; left: 130px; bottom: 120px; width:80px; height:80px;background: transparent;cursor:pointer;">';
        echo "<img src='images/avatar-upload.png' style='width:100%;height:100%;' alt='' onclick=\"document.getElementById('avatarInput').click();\" onmouseover=this.src='images/avatar-upload_on.png' onmouseout=this.src='images/avatar-upload.png' />";
      echo '</div>';

      # Remove button
      echo '<div style="position:fixed; left: 240px; bottom: 210px; width:80px; height:10px;background: transparent;font-size:12px;text-align:center;">Remove image</div>';
      echo '<div style="position:fixed; left: 240px; bottom: 120px; width:80px; height:80px;background: transparent;cursor:pointer;">';
        echo "<img src='images/avatar-delete.png' style='width:100%;height:100%;' alt='' onclick=\"document.getElementById('avatarDelete').click();\" onmouseover=this.src='images/avatar-delete_on.png' onmouseout=this.src='images/avatar-delete.png' />";
      echo '</div>';

      # Bookings list
      echo '<div id="bookingstable" style="position:fixed; left: 20px; bottom: 260px; width:300px; height:auto;background: transparent;font-size:12px;text-align:center;">no bookings found</div>';

      # Upload form (hidden)
      echo "
      <form id='uploadForm' enctype='multipart/form-data' style='width:300px;height:40px;visibility:hidden'>
      <input type='file' name='images[]' id='avatarInput' style='display: none;'>
      <input type='button' value='Change avatar' id='browseButton' style='background-color:#0979D8; color:white;;width: 200px;height: 32px;border-radius: 10px;border: none;cursor: pointer;'/>
      <input type='hidden' name='mode' value='upload'>
      <input type='submit' name='submit' id='uploadButton' value='UPLOAD'>
      </form>
      <form id='deleteForm' style='width:300px;height:40px;visibility:hidden'>
      <input type='hidden' name='mode' value='delete'>
      <input type='submit' name='submit' id='avatarDelete' value='Delete avatar'style='background-color:#0979D8; color:white;width: 200px;height: 32px;border-radius: 10px;border: none;cursor: pointer;'>
      </form>";
      
      echo '</div>';
      # User Avatar
      $unixtimestamp = time();
      $userfull=$_SESSION['username'];
      if (strpos($userfull, '\\') !== FALSE) {
        $userarray=explode("\\",$userfull);
        $userid = $userarray[1];
      }
      else {
        $userid = $userfull;
      }
      $avatarfile="avatarcache/${userid}.jpg";
      echo '<div id="avatarbutton" onclick="togglePersonalMenu()">';
      if (file_exists($avatarfile)) {
        echo "<img src='${avatarfile}?time=${unixtimestamp}' style='width:80px; height:80px;'>";
      }
      else {
        echo "<img src='images/noavatar.png' style='width:80px; height:80px;'>";
      }
      echo '</div>';

      # Username
      echo '<div class="avatarbutton_username" onclick="togglePersonalMenu()">';
        echo $_SESSION['username'];
      echo '</div>';
    echo '</div>';
  }

  # Admin menu
  echo '<div id="buttons_right" class="buttons_right" style="position:fixed; right: 10px; bottom: '.(25*$autozoom).'px; height:auto;width:80px;background: transparent; transform:scale('.$autozoom.');transform-origin:100% 100%;">';
  if (isset($_SESSION['username'])) {
    if ($map != "overview" && permcheck($_SESSION['username'], 'desks') > 1){
      
      # Add button
      echo '<div class="inputgrid" id="inputgrid" style="float:right; margin:5px; width:80px; height:80px; background: transparent;">
      <input class="inputgridbutton" type="image" src="images/add.png" style="width:80px; height:80px;" onClick="return doNewItem(\'showInputgrid\')" onmouseover=this.src="images/add_on.png" onmouseout=this.src="images/add.png">
      </div>';
    }

    if ($map == "overview" && permcheck($_SESSION['username'], 'maps') > 1) {
      # Add button
      echo '<div class="inputgrid" id="inputgrid" style="float:right; margin:5px; width:80px; height:80px; background: transparent;">
      <input class="inputgridbutton" type="image" src="images/add.png" style="width:80px; height:80px;" onClick="return doNewItem(\'showInputgrid\')" onmouseover=this.src="images/add_on.png" onmouseout=this.src="images/add.png">
      </div>';
    }

      # Link to Admin Panel
      if (permcheck($_SESSION['username'], 'adminpanel') > 0){
        echo '<a href="./admin/"><div id="adminpanel_button" class="editbutton" style="background-image: url(images/adminpanel.png);"></div></a>';
      }
      # Switch to usermode if desired
      if (permcheck($_SESSION['username'], 'desks') > 0){
        if ($setting_usermode == 'user') {$usermode_color = 'orange';} else {$usermode_color = '#333';}
        echo '<div id="usermode_switch" class="editbutton" style="height:40px;border-radius:10px;width:77px;text-align:center;line-height:40px;cursor:pointer;background-color: '.$usermode_color.'" onclick=toggleUsermode()>'.$setting_usermode.'</div>';
      }
      # DB Integrity check indicator
      if (permcheck($_SESSION['username'], 'desks') > 0){
        ?>
        <div id="healthstatus" class="consistency" style="float:right; margin:5px; width:80px; height:80px;background: transparent;display:none;">
          <img src="images/dbcheck_ok2.png" style="width:100%;height:100%;" alt="" />
        </div>
        <script>
          checkHealthStatus();
          setInterval(checkHealthStatus, 300000);
        </script>   

        <?php
      }
  }
  echo '</div>';

  # Login Icon in lower left corner
  
  if (!isset($_SESSION['username'])) {
    if ($setting_saml == 0) {
      echo '<div class="loginicon" id="loginicon" style="position:fixed; bottom:10px;left:10px; width:50px; z-index:3; transform:scale('.$autozoom.');transform-origin:0% 100%;">
      <input type="image" src="images/login-user.png" alt="e" onclick=location.href="rest/account" onmouseover=this.src="images/login-user_on.png" onmouseout=this.src="images/login-user.png" style="width:50px;">
      </div>';
    }
    else {
      echo '<div class="loginicon" id="loginicon" style="position:fixed; bottom:10px;left:10px; width:50px; z-index:3; transform:scale('.$autozoom.');transform-origin:0% 100%;">
      <input type="image" src="images/login-user.png" alt="e" onClick="loginForm(true)" onmouseover=this.src="images/login-user_on.png" onmouseout=this.src="images/login-user.png">
      </div>';
    } 
  }

  # Enable visitor counter in the lower left corner if enabled in the options
  if ($setting_dailyvisitors == 1) {
    echo '<script> updateCounter(); setInterval(updateCounter, 300000) </script>';
  }

  # Teams Sidebar
  echo '<script>updateTeams()</script>';
  echo '<div id="addressbook" style="position: fixed;background:#333;right:0px;bottom:0px;width: 100%; max-width:400px;height:100%;opacity:0.95;display:none;">';
  echo '<div class="adressbook_margin" style="height:'.(69*$autozoom).'px;width:100%;margin-bottom:5px;background:none;"></div>';
  echo '';
    # Content will be added by updateTeams()
  echo '</div>';

  # Announcement Sidebar

  if (isset($_COOKIE['announcecookie'])) {$announcecookie = $_COOKIE['announcecookie'];} else {$announcecookie = 0;} 
  echo '<script>announceValue = '.$announcecookie.'</script>';
  
  mysqli_query($dbLink,"SET NAMES 'utf8'");
  $ldapchangelog = mysqli_query($dbLink, "SELECT * FROM `ldap_changelog` ORDER BY `ID` DESC; ");
  $ldapchangenum   = mysqli_num_rows ($ldapchangelog);
  ?>

  <div class="datepicker" id="theDate"></div>
  <div class="clock" id="theTime" onclick="toggleDatepicker()"></div>
  
  <?php

  echo '<div id="announcementbar" class="announcementbar" style="position: fixed;background:#333333;right:0px;top:0px;width:490px;height:100%;opacity:0.95;display:none;">';
  echo '<div class="announcementbar_margin" style="height:'.(69*$autozoom).'px;width:100%;margin-bottom:5px;background:none;"></div>';
  echo '<div id="announcementbar_body" class="announcementbar_body" style="height:92%; overflow-y: scroll;width:510px;font-size:20px;">';
  
  echo '</div></div>';

  # Trigger search if findme has a value from URL
  if ($findme != "") {
    echo '<script> teamlabel = "'.$teamlabel.'"; searchtext = "'.$findme.'"</script>';
  }
?>
<?php echo '</body></html>';
ob_end_flush(); ?>