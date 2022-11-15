<?php
session_start();

# Loading shared functions and config file
include '../shared.php';
?>
<!DOCTYPE HTML>
<!-- ===================================================================
    
  CompanyMaps 8.0 AdminPanel
  Release date 2022-11-14
  Copyright (c) 2016-2022 by MavoDev
  see https://www.mavodev.de for more details
    
==================================================================== -->

<html lang="de"> 
<head>
  <meta name="generator" content="HTML Tidy for Windows (vers 22 March 2008), see www.w3.org">

  <title><?php echo $apptitle ?></title>
  <meta http-equiv="Content-Type" content="text/html; charset=us-ascii">
  <link rel="stylesheet" type="text/css" href="../cmaps80.css">
  <link rel="stylesheet" type="text/css" href="admin80.css">

  <link rel="apple-touch-icon" sizes="57x57" href="../../favicons/apple-touch-icon-57x57.png">
  <link rel="apple-touch-icon" sizes="60x60" href="../favicons/apple-touch-icon-60x60.png">
  <link rel="apple-touch-icon" sizes="72x72" href="../favicons/apple-touch-icon-72x72.png">
  <link rel="apple-touch-icon" sizes="76x76" href="../favicons/apple-touch-icon-76x76.png">
  <link rel="apple-touch-icon" sizes="114x114" href="../favicons/apple-touch-icon-114x114.png">
  <link rel="apple-touch-icon" sizes="120x120" href="../favicons/apple-touch-icon-120x120.png">
  <link rel="apple-touch-icon" sizes="144x144" href="../favicons/apple-touch-icon-144x144.png">
  <link rel="apple-touch-icon" sizes="152x152" href="../favicons/apple-touch-icon-152x152.png">
  <link rel="apple-touch-icon" sizes="180x180" href="../favicons/apple-touch-icon-180x180.png">
  <link rel="icon" type="image/png" href="../favicons/favicon-32x32.png" sizes="32x32">
  <link rel="icon" type="image/png" href="../favicons/android-chrome-192x192.png" sizes="192x192">
  <link rel="icon" type="image/png" href="../favicons/favicon-96x96.png" sizes="96x96">
  <link rel="icon" type="image/png" href="../favicons/favicon-16x16.png" sizes="16x16">
  <link rel="manifest" href="../favicons/manifest.json">
  <link rel="mask-icon" href="../favicons/safari-pinned-tab.svg" color="#5BBAD5">
  <link rel="shortcut icon" href="../favicons/favicon.ico">
  <meta name="msapplication-TileColor" content="#2d89ef">
  <meta name="msapplication-TileImage" content="../favicons/mstile-144x144.png">
  <meta name="msapplication-config" content="../favicons/browserconfig.xml">
  <meta name="theme-color" content="#000000">
  <meta name="apple-mobile-web-app-capable" content="yes">
  <meta name="mobile-web-app-capable" content="yes">
  <meta name="viewport" content="width=device-width">
  <meta name="apple-mobile-web-app-status-bar-style" content="black-translucent">
  
  <script src="../tools/Chart.min.js"></script>
  <script src="../tools/jquery3.js"></script>
  <script src="../tools/resize80.js"></script>
  <script src="../tools/underscore.js"></script>
  <script src="backend80.js"></script>
  <script src="../user80.js"></script>
  <!--<link href='https://fonts.googleapis.com/css?family=Roboto' rel='stylesheet' type='text/css'>-->
</head>

<body>

<?php   
    
// Check Admin-Login
if ($_SESSION['editmode'] <> '' && $_SESSION['editmode'] == base64_encode($_SESSION['username'])) {
  function charttile ($chartname, $chartvalue, $chartmax) {
    $percentage = $chartvalue / $chartmax*100;
    if ($percentage >=95 ) {$color='red';}
    else if ($percentage >= 85) {$color='orange';}
    else {$color='green';}
    echo '  <div id="'.$chartname.'" style="width:300px; height:300px; float:left; margin-right:20px; background:'.$color.'; opacity:0.7; text-align:center;line-height:300px;">
          <span style="display: inline-block; vertical-align: middle; line-height: normal;">
          <h1>'.$chartname.'</h1><h2>'.$chartvalue.'</h2>
          </span>
        </div>';
  }

  // Auto-set variables
  $path = $_SERVER['PHP_SELF']; $file = basename($path);
  $tab = '&nbsp;&nbsp;&nbsp;'; 
  $page = $_SERVER['PHP_SELF'];
  $rootdir = str_replace("/admin", "", __DIR__);
  if ($_SERVER['REQUEST_METHOD'] == "GET") {
    if (isset($_GET['tab'])) {$activetab = $_GET['tab'];} else {$activetab = '';}  
  }
  if ($_SERVER['REQUEST_METHOD'] == "POST") {
    $activetab            = $_POST['tab'];  
    $ignoreHealthName     = $_POST['ignoreHealthName'];
    $ignoreHealthType     = $_POST['ignoreHealthType'];
    $newadminuser         = $_POST['newadminuser'];
    $newadminrole         = $_POST['newadminrole'];
    $deleteID             = $_POST['deleteID'];
    $deleteUser           = $_POST['deleteUser'];
    $deleteRole           = $_POST['deleteRole'];
    $deleteTeam           = $_POST['deleteTeam'];
    $deleteMembers        = $_POST['deleteMembers'];
    $createTeam           = $_POST['newTeam'];
    $createMembers        = $_POST['newMembers'];
    $newLdapDescription   = $_POST['newLdapDescription'];
    $newLdapServer        = $_POST['newLdapServer'];
    $newLdapType         = $_POST['newLdapType'];
    $newLdapOU            = $_POST['newLdapOU'];
    $newLdapUser          = $_POST['newLdapUser'];
    $newLdapPass          = $_POST['newLdapPass'];
    $deleteLdapID         = $_POST['deleteLdapID'];
    $deleteLdapDescription= $_POST['deleteLdapDescription'];
    $deleteLdapServer     = $_POST['deleteLdapServer'];
    $deleteLdapOU         = $_POST['deleteLdapOU'];
    $newMapName           = strtolower($_POST['newMapName']);
    $newMapItemscale      = $_POST['newMapItemscale'];
    $newMapPublished      = $_POST['newMapPublished'];
    $newMapCountry        = strtolower($_POST['newMapCountry']);
    $newMapFlagsize       = $_POST['newMapFlagsize'];
    $newMapTimezone       = $_POST['newMapTimezone'];
    $newMapAddress        = $_POST['newMapAddress'];
    $newMapX              = $_POST['newMapX'];
    $newMapY              = $_POST['newMapY'];
    $finishMap            = $_POST['finishMap'];
    $deleteMapID          = $_POST['deleteMapID'];
    $deleteMapname        = $_POST['deleteMapname'];
    $toggleMapID          = $_POST['toggleMapID'];
    $toggleMapname        = $_POST['toggleMapname'];
    $toggleMapStatus      = $_POST['toggleMapStatus'];
    $auditlogEventType    = $_POST['auditlogEventType'];

    if (isset($_POST["uploadMapfile"])) { 
      //Get the file information
      $userfile_name = $_FILES['image']['name'];
      $userfile_tmp = $_FILES['image']['tmp_name'];
      $userfile_size = $_FILES['image']['size'];
      $userfile_type = $_FILES['image']['type'];
      $filename = basename($_FILES['image']['name']);
      $file_ext = strtolower(substr($filename, strrpos($filename, '.') + 1));
      $SaveToMapfile = '../maps/'.$newMapName.'.png';
      $SaveToPrintfile = '../maps/'.$newMapName.'-print.png';

      switch($file_ext) {
        case "gif":
          $converted = imagecreatefromgif($userfile_tmp); 
          imagepng($converted, $SaveToMapfile);
          /*
          $filterimage = imagecreatefrompng($SaveToMapfile);
          imagealphablending($filterimage, false);
          imagesavealpha($filterimage, true);
          imagefilter($filterimage, IMG_FILTER_NEGATE);
          imagepng($filterimage, $SaveToPrintfile);
          imagedestroy($filterimage); 
          */
          break;
        case "jpeg":
        case "jpg":
          $converted = imagecreatefromjpeg($userfile_tmp); 
          imagepng($converted, $SaveToMapfile);
          /*
          $filterimage = imagecreatefrompng($SaveToMapfile);
          imagealphablending($filterimage, false);
          imagesavealpha($filterimage, true);
          imagefilter($filterimage, IMG_FILTER_NEGATE);
          imagepng($filterimage, $SaveToPrintfile);
          imagedestroy($filterimage); 
          */
          break;
        case "png":
          move_uploaded_file($_FILES['image']['tmp_name'], $SaveToMapfile);   
          /*
          $filterimage = imagecreatefrompng($SaveToMapfile);
          imagealphablending($filterimage, false);
          imagesavealpha($filterimage, true);
          imagefilter($filterimage, IMG_FILTER_NEGATE);
          imagepng($filterimage, $SaveToPrintfile);
          imagedestroy($filterimage);   
          */   
          break;
      }
    }
        
  }

  # Check if only allowed tabs are used.
  if ($activetab=="") {$activetab="dashboard";}
  if (permcheck($_SESSION['username'], $activetab)==0) {
    $activetab = 'dashboard';
  }

  if (isset($_COOKIE['browserid'])) {
    $mybrowserid = $_COOKIE['browserid']; 
  }
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


  // Output Header

  echo '<div id="container"> </div>';
  /* Page Header
  // Includes all controls with a description next to it 
  */

  // Header of Page - scales vertically with the page
  echo '<div class="control_background" style="width:100%; height: 69px; position: fixed; top: 0px; left: 0px;background: #333333; transform:scaleY('.$autozoom.')"> </div>';
  echo '<div id="controlpanel" class="control_container" style="position:fixed; left: 0px; top: 0px; height:69px;width:100%;background-color:#333333;opacity:1.0;z-index:1;transform:scale('.$autozoom.');transform-origin:50% 0%;">';
  // Center header box with controls - scales horizontally with the page
  echo '<div class="control_content" style="position: absolute; left:50%; margin-left:-'.($targetScreenWidth/2).'px; top:0px;width:'.$targetScreenWidth.'px;height:69px; z-index:2;transform-origin:50% 0%;background-color:#333333">';
    // DASHBOARD tab with logo
    echo '         
            <div class="headeritem" id="tab_dashboard" style="width:200px; background: transparent; margin-left:0px;">
                <span class="headeritem_picture" style="opacity:1.0";><a href="'.$page.'?tab=dashboard"><img src="../images/logo.png" width="45" alt="" onmouseover=this.src="../images/dashboard.png" onmouseout=this.src="../images/logo.png" /></a></span> 
                <span class="headeritem_text">Dashboard</span>
            </div>';
    // HEALTH tab
    if (permcheck($_SESSION['username'], 'health') > 0) {
      echo '  <div class="headeritem" id="tab_health" style="width:120px;">
          <span class="headeritem_picture"><a href="'.$page.'?tab=health"><img src="../images/health.png" width="45" alt="" /></a></span>
          <span class="headeritem_text">Health</span>
        </div>';
    }
    // CONFIG tab
    if (permcheck($_SESSION['username'], 'config') > 0) {
      echo '  <div class="headeritem" id="tab_config" style="width:120px;">
          <span class="headeritem_picture"><a href="'.$page.'?tab=config"><img src="../images/settings.png" width="45" alt="" /></a></span>
          <span class="headeritem_text">Config</span>
        </div>';
    }
    // LDAP tab
    if (permcheck($_SESSION['username'], 'ldap') > 0) {
      echo '  <div class="headeritem" id="tab_ldap" style="width:120px;">
            <span class="headeritem_picture"><a href="'.$page.'?tab=ldap"><img src="../images/sync.png" width="45" alt="" /></a></span>
            <span class="headeritem_text">LDAP</span>
          </div>';
    }
    // MAPS tab
    if (permcheck($_SESSION['username'], 'maps') > 0) {
      echo '  <div class="headeritem" id="tab_maps" style="width:120px;">
            <span class="headeritem_picture"><a href="'.$page.'?tab=maps"><img src="../images/map.png" width="45" alt="" /></a></span>
            <span class="headeritem_text">Maps</span>
          </div>';
    }
    // DESKS tab
    if (permcheck($_SESSION['username'], 'desks') > 0) {
      echo '  <div class="headeritem" id="tab_desks" style="width:120px;">
            <span class="headeritem_picture"><a href="'.$page.'?tab=desks"><img src="../images/desks.png" width="45" alt="" /></a></span>
            <span class="headeritem_text">Desks</span>
          </div>';
    }
    // USERS tab
    if (permcheck($_SESSION['username'], 'users') > 0) {
      echo '  <div class="headeritem" id="tab_users" style="width:120px;">
            <span class="headeritem_picture"><a href="'.$page.'?tab=users"><img src="../images/adminusers.png" width="45" alt="" /></a></span>
            <span class="headeritem_text">Users</span>
          </div>';
    }
    // TEAMS tab
    if (permcheck($_SESSION['username'], 'teams') > 0) {
      echo '  <div class="headeritem" id="tab_teams" style="width:120px;">
            <span class="headeritem_picture"><a href="'.$page.'?tab=teams"><img src="../images/teams.png" width="45" alt="" /></a></span>
            <span class="headeritem_text">Teams</span>
          </div>';  
    }
    // STATS tab
    if (permcheck($_SESSION['username'], 'stats') > 0) {
      echo '  <div class="headeritem" id="tab_stats" style="width:120px;">
            <span class="headeritem_picture"><a href="'.$page.'?tab=stats"><img src="../images/advancedstats.png" width="45" alt="" /></a></span>
            <span class="headeritem_text">Stats</span>
          </div>';
    }
    // AUDITLOG tab
    if (permcheck($_SESSION['username'], 'auditlog') > 0) {
      echo '  <div class="headeritem" id="tab_auditlog" style="width:130px;">
            <span class="headeritem_picture"><a href="'.$page.'?tab=auditlog"><img src="../images/auditlog.png" width="45" alt="" /></a></span>
            <span class="headeritem_text">AuditLog</span>
          </div>';  
    }
    // LOGOUT button
    echo '  <div class="headeritem" style="width:150px;float:right;">
          <span class="headeritem_picture" style="float:right; cursor:pointer;";>
          <img src="../images/logout2.png" width="45" alt="" onClick=logoutUser("admin") />
          </span>
          <span class="headeritem_picture" style="float:right";>
          <a href="../index.php" title="go back to the map"><img src="../images/globe.png" width="45" alt="" /></a>
          </span>
          <!--<span class="headeritem_text" style="float:right;">'.$_SESSION['username'].'</span>-->
        </div>';
  //Close Header DIVs
  echo '
       </div>
       </div>';

  # Highlight active header
  if ($activetab != '') {
    ?><script>
    $("#tab_<?php echo $activetab ?>").css("background-color", "#505050");
    $("#tab_<?php echo $activetab ?>").css("border-radius", "50px");
    </script><?php
  }
  else {
    ?><script>
    $("#tab_dashboard").css("background-color", "#505050");
    $("#tab_dashboard").css("border-radius", "50px");
    </script><?php
  }
  // END OF HEADER

  // CONTENT DIV start
  echo '<div id="content" class="page_content" style="position:absolute; top:'.(69*$autozoom).'px; left:'.$LeftPos.'px; width:'.$targetScreenWidth.'px; transform:scale('.$zoom/100*$autozoom.');"> <br />';  

  switch ($activetab) {

  case "dashboard":
    ?>
    <script>
      updateSystemStats();
      setInterval(updateSystemStats, 15000);
    </script>
    <?php

    break;

  case "health":
    # Add items to whitelist if requested
    if ($ignoreHealthName != "" && $ignoreHealthType != "") {
      $dbTable = 'health_whitelist';
      $dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
      mysqli_query($dbLink, "INSERT INTO `$dbName`.`$dbTable` (`ID`, `type`, `text`) 
      VALUES (NULL, '$ignoreHealthType', '$ignoreHealthName');");
      mysqli_close($dbLink);
    }
    ?>
    <form id="updateWhitelist" name="updateWhitelist" action="<?php echo $page ?>" method="post">
      <input type="hidden" name="tab" value="health"> 
      <input type="hidden" id="ignoreHealthName" name="ignoreHealthName" value="a"> 
      <input type="hidden" id="ignoreHealthType" name="ignoreHealthType" value="a">  
      <input type="submit" value="No" style="display:none;">
    </form>
    <script>
      updateHealthDetails();
      setInterval(updateHealthDetails, 60000);
    </script>
    <?php
    break;

  case "config":
    echo "<h2 style='margin-left:20px'>Configured base variables</h2><br />";
    $dbTable = 'config_general';
      
    $dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
    $details = mysqli_query($dbLink, "SELECT * FROM $dbTable");
    $num   = mysqli_num_rows ($details);   

    echo "<table style='margin-left: 20px'>";
    echo '<tr style="font-weight:bold"><td width="270">Variable</td><td width="300">Value</td></tr>';
    echo '<tr><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td></tr>';
    
    for ($i = 0; $i < $num; $i++) {
      $id       = mysqli_result($details,$i,0);
      $variable = mysqli_result($details,$i,1);
      $value    = mysqli_result($details,$i,2);
      echo '<tr><td>'.$variable.'</td><td>'.$value.'</td></tr>';
    }

    echo "</table>";
    echo "<br/>";
    echo "<h2 style='margin-left:20px'>Configured VIP desks</h2><br />";
    $dbTable = 'config_vips';
      
    $dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
    $details = mysqli_query($dbLink, "SELECT * FROM $dbTable");
    $num   = mysqli_num_rows ($details);   

    echo "<table style='margin-left: 20px'>";
    echo '<tr style="font-weight:bold"><td width="270">Parsed Text from Job Title</td><td width="150">Type</td><td width="300">Description</td></tr>';
    echo '<tr><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td></tr>';
    
    for ($i = 0; $i < $num; $i++) {
      $id          = mysqli_result($details,$i,0);
      $variable    = mysqli_result($details,$i,1);
      $value       = mysqli_result($details,$i,2);
      $description = mysqli_result($details,$i,3);
      echo '<tr><td>'.$variable.'</td><td>'.$value.'</td><td>'.$description.'</td></tr>';
    }

    echo "</table>";
    break;

  // LDAP TAB start
  case "ldap": 
    $dbTable = 'config_ldap';
    $dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);

    //Add new LDAP connection first if requested
    if ($newLdapDescription != "" && $newLdapServer != "" && $newLdapType != "" && $newLdapOU != "" && $newLdapUser != "" && $newLdapPass != "") {
      $newLdapUser = str_replace("\\", "\\\\", $newLdapUser);
      mysqli_query($dbLink, "
      INSERT INTO `$dbName`.`$dbTable` (`ID`, `description`, `server`, `type`, `OU`, `LdapUser`, `LdapPass`, `LastSync`) 
      VALUES (NULL, '$newLdapDescription', '$newLdapServer', '$newLdapType', '$newLdapOU', '$newLdapUser', '$newLdapPass', 'never');");
      $NewLdapCache = 'ldapcache_'.mysqli_insert_id ($dbLink);
      mysqli_query($dbLink, "Create Table `$NewLdapCache` Like `$ldapTable`;");
      auditlog("LDAP",$_SESSION['username'],"New LDAP sync has been created (".$newLdapDescription.", ".$NewLdapCache.")");
    }

    //Remove LDAP connection first if requested
    if ($deleteLdapID != "" && $deleteLdapDescription != ""&& $deleteLdapServer != "" && $deleteLdapOU != "") {
      mysqli_query($dbLink, "DELETE FROM $dbTable WHERE `ID` = '$deleteLdapID' AND `description` = '$deleteLdapDescription' AND `server` = '$deleteLdapServer' AND `OU` = '$deleteLdapOU'");
      $deletedTable = 'ldapcache_'.$deleteLdapID;
      mysqli_query($dbLink, "DROP TABLE $deletedTable");
      auditlog("LDAP",$_SESSION['username'],"LDAP sync has been removed (".$deleteUser.", ".$deletedTable.")");
    }

    $details = mysqli_query($dbLink, "SELECT * FROM $dbTable");
    $num   = mysqli_num_rows ($details);   
    
    echo "<table style='margin-left:20px;'>";
    echo '<tr style="font-weight:bold;">
        <td width="200">Description</td>
        <td width="180">Server</td>
        <td width="110">Type</td>
        <td width="400">OU</td>
        <td width="230">LdapUser</td>
        <td width="150">LdapPass</td>
        <td width="150">Last Sync</td>
        <td width="150"></td>
        <td width="100">&nbsp;</td>
        </tr>';
    echo '<tr><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td></tr>';

    if (permcheck($_SESSION['username'], 'desks') > 1) {
      $token = (strrev(date("Ymd")) + date("Ymd"));
      echo '<script>token = '.$token.';username = "'.$_SESSION['username'].'";
            console.log("Token: " + token);</script>';
    }

    for ($i = 0; $i < $num; $i++) {
      $ldap_id          = mysqli_result($details,$i,0);
      $ldap_description = mysqli_result($details,$i,1);
      $ldap_server      = mysqli_result($details,$i,2);
      $ldap_ldaps       = mysqli_result($details,$i,3);
      $ldap_ou          = mysqli_result($details,$i,4);
      $ldap_user        = mysqli_result($details,$i,5);
      $ldap_pass        = mysqli_result($details,$i,6);
      $ldap_lastsync    = mysqli_result($details,$i,7);
      echo '<tr>
          <td>'.$ldap_description.'</td>
          <td>'.$ldap_server.'</td>
          <td>'.$ldap_ldaps.'</td>
          <td>'.$ldap_ou.'</td>
          <td>'.$ldap_user.'</td>
          <td>*****</td>
          <td>'.$ldap_lastsync.'</td>
          <td><input type="button" id="syncbutton'.$ldap_id.'" value="Sync now" style="background-color:#00f; width:90%;" onClick="syncLDAP(\''.$ldap_id.'\',\''.addslashes($_SESSION['username']).'\')"></td>
          <td>';
          if (permcheck($_SESSION['username'], 'ldap') > 1) {
            echo '  <form name="DeleteLDAP" action="'.$page.'?tab='.$activetab.'" method="post">
                <input type="hidden" name="tab" value="ldap"> 
                <input type="hidden" name="deleteLdapID" value="'.$ldap_id.'">   
                <input type="hidden" name="deleteLdapDescription" value="'.$ldap_description.'"> 
                <input type="hidden" name="deleteLdapServer" value="'.$ldap_server.'">  
                <input type="hidden" name="deleteLdapOU" value="'.$ldap_ou.'">  
                <input type="submit" value="Delete">
                </form>';
          }
          else {
            echo '&nbsp;';
          }
      echo '  </td>
        </tr>';
    }

    // Include the creation of new admin accounts for superadmin users only
    if (permcheck($_SESSION['username'], 'ldap') > 1) {
      echo '<tr><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td></tr>';      
      echo '<form name="AddLdap" action="'.$page.'?tab='.$activetab.'" method="post"><tr>
          <input type="hidden" name="tab" value="ldap">
          <td><input type="text" name="newLdapDescription" style="width:180px;"></td>
          <td><input type="text" name="newLdapServer" style="width:160px;"></td>
          <td>
            <select name="newLdapType" style="width:90px">
              <option value="LDAPS" selected>LDAPS</option>
              <option value="LDAP">LDAP</option>
            </select>
          </td>
          <td><input type="text" name="newLdapOU" style="width:380px;"></td>
          <td><input type="text" name="newLdapUser" style="width:210px;"></td>
          <td><input type="text" name="newLdapPass" style="width:130px;"></td>
          <td>&nbsp;</td>
          <td>&nbsp;</td>
          <td><input type="submit" value="Create"></td>
        </tr></form>';
    }

    echo "</table><br /><br />";
    break;
  // LDAP TAB end

  case "maps":
    
    $dbTable = 'config_maplist';
    $dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
    
    if ($newMapName != "" && $newMapItemscale != "" && $newMapPublished != "" && $newMapCountry != "" && $newMapFlagsize != "" && $newMapTimezone != "" && $newMapAddress != "" && $newMapX != "" && $newMapY != "") {
      $mapfile = $rootdir.'/maps/'.$newMapName.'.png'; 
      
      
      // Check if image has been uploaded and proceed to finish the creation process
      if ($finishMap == "Finish" && file_exists($mapfile)) {
        // new map database gets created
        $mapdatabase = 'desks_'.$newMapName;
        mysqli_query($dbLink, "CREATE TABLE `$dbName`.`$mapdatabase` 
        ( `ID` INT NOT NULL AUTO_INCREMENT , `desktype` TEXT NOT NULL , `x` INT NOT NULL , `y` INT NOT NULL , `desknumber` TEXT NOT NULL , `employee` TEXT NOT NULL , 
        `avatar` TEXT NOT NULL , `department` TEXT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB;");
        // map is registered to the maplist database
        mysqli_query($dbLink, 
        "INSERT INTO `config_maplist`(`ID`, `mapname`, `itemscale`, `published`, `country`, `flagsize`, `timezone`, `address`, `mapX`, `mapY`) 
        VALUES (NULL,'$newMapName','$newMapItemscale','$newMapPublished','$newMapCountry','$newMapFlagsize','$newMapTimezone','$newMapAddress','$newMapX','$newMapY');");

        auditlog("Maps",$_SESSION['username'],"Map has been created (".$newMapName.", ".$mapdatabase.", ".$mapfile.")");


        echo '  <script type="text/javascript">
              window.location=window.location;
            </script>';
        break;
      }

      echo "<h2>Create new map</h2><br />";
      // Check to see if mapname already exists to prevent damage
      $CheckRows = mysqli_query($dbLink, "SELECT * FROM $dbTable WHERE `mapname`= '$newMapName'");
      $CheckRowsNum = mysqli_num_rows ($CheckRows);
      if ($CheckRowsNum != 0) {echo 'Error: Name already exists! Please choose a unique name.';} 
      else {
        if (file_exists($mapfile)) {
          echo '  Okay, we got everything we need for the new map. Please click on finish to complete to process. <br /><br />
              
              <form name="CreateMap" action="'.$page.'?tab='.$activetab.'" method="post">
              <input type="hidden" name="tab" value="maps">
              <input type="hidden" name="newMapName" value="'.$newMapName.'">
              <input type="hidden" name="newMapItemscale" value="'.$newMapItemscale.'">
              <input type="hidden" name="newMapPublished" value="'.$newMapPublished.'">
              <input type="hidden" name="newMapCountry" value="'.$newMapCountry.'">
              <input type="hidden" name="newMapFlagsize" value="'.$newMapFlagsize.'">
              <input type="hidden" name="newMapTimezone" value="'.$newMapTimezone.'">
              <input type="hidden" name="newMapAddress" value="'.$newMapAddress.'">
              <input type="hidden" name="newMapX" value="'.$newMapX.'">
              <input type="hidden" name="newMapY" value="'.$newMapY.'">
              <input type="submit" name="finishMap" value="Finish" style="width:200px" />
              </form>';
          break;
        } 
        
        echo '  Select a mapfile to upload. Optimal results are achieved by using a PNG file which is 1600 pixels width.<br />File will be saved as "'.$mapfile.'"<br /><br />
            <form name="photo" enctype="multipart/form-data" action="'.$page.'?tab='.$activetab.'" method="post">
            Mapfile <input type="file" accept="image/png" name="image" size="30" /> 
            <input type="hidden" name="tab" value="maps">
            <input type="hidden" name="newMapName" value="'.$newMapName.'">
            <input type="hidden" name="newMapItemscale" value="'.$newMapItemscale.'">
            <input type="hidden" name="newMapPublished" value="'.$newMapPublished.'">
            <input type="hidden" name="newMapCountry" value="'.$newMapCountry.'">
            <input type="hidden" name="newMapFlagsize" value="'.$newMapFlagsize.'">
            <input type="hidden" name="newMapTimezone" value="'.$newMapTimezone.'">
            <input type="hidden" name="newMapAddress" value="'.$newMapAddress.'">
            <input type="hidden" name="newMapX" value="'.$newMapX.'">
            <input type="hidden" name="newMapY" value="'.$newMapY.'">
            <input type="submit" name="uploadMapfile" value="Upload" style="width:200px" />';
        break;
      }
    }

    // Delete map first if requested
    if ($deleteMapID != "" && $deleteMapname != "") {
      // Remove reference in config_maplist
      mysqli_query($dbLink, "DELETE FROM $dbTable WHERE `ID` = '$deleteMapID' AND `mapname` = '$deleteMapname'");

      // Remove database for map
      $deleteTable = 'desks_'.$deleteMapname;
      mysqli_query($dbLink, "DROP TABLE $deleteTable");

      // Remove mapfile image
      $deleteFilename = $rootdir.'/maps'.'/'.$deleteMapname.'.png';
      unlink($deleteFilename);
      //unlink($deletePrintFilename);
      
      auditlog("Maps",$_SESSION['username'],"Map has been deleted (".$deleteMapname.", ".$deleteTable.", ".$deleteFilename.")");
    }

    if ($toggleMapID != "" && $toggleMapname != "") {
      mysqli_query($dbLink, "UPDATE $dbTable SET Published='$toggleMapStatus' WHERE `ID` = '$toggleMapID' AND `mapname` = '$toggleMapname'");
    }

    #echo "<h2 style='margin-left:10px'>Overview of available maps</h2><br />";
    echo '<table style="margin-left:10px">';
    echo '<tr style="font-weight:bold;">
        <td width="200">Mapname</td>
        <td width="100">Itemscale</td>
        <td width="100">Published</td>
        <td width="130">Country</td>
        <td width="100">Flagsize</td>
        <td width="150">Timezone</td>
        <td width="200">Address</td>
        <td width="100">mapX</td>
        <td width="100">mapY</td>
        <td width="80" align="center">Mapfile</td>
        <td width="80" align="center">Database</td>
        <td width="80" align="center">Countryflag</td>
        <td width="100">&nbsp;</td>
        </tr>';
    echo '<tr><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td></tr>';

    $details = mysqli_query($dbLink, "SELECT * FROM $dbTable");
    $num   = mysqli_num_rows ($details);  

    for ($i = 0; $i < $num; $i++) {
      $id        = mysqli_result($details,$i,0);
      $mapname   = mysqli_result($details,$i,1);
      $itemscale = mysqli_result($details,$i,2);
      $published = mysqli_result($details,$i,3);
      $country   = mysqli_result($details,$i,4);
      $flagsize  = mysqli_result($details,$i,5);
      $timezone  = mysqli_result($details,$i,6);
      $address   = mysqli_result($details,$i,7);
      $mapX      = mysqli_result($details,$i,8);
      $mapY      = mysqli_result($details,$i,9);
      echo '<tr>
          <td>'.$mapname.'</td>
          <td>'.$itemscale.'</td>';
      if ($published == 'yes') {
        echo '<td>
                <form name="ToggleMap" action="'.$page.'?tab='.$activetab.'" method="post">
                  <input type="hidden" name="tab" value="maps"> 
                  <input type="hidden" name="toggleMapID" value="'.$id.'">  
                  <input type="hidden" name="toggleMapname" value="'.$mapname.'">  
                  <input type="hidden" name="toggleMapStatus" value="no"> 
                  <input type="submit" value="Yes" style="width:80px; background: rgba(0, 200, 0, 0.7);">
                </form>
              </td>';
      }   
      else {
        echo '<td>
                <form name="ToggleMap" action="'.$page.'?tab='.$activetab.'" method="post">
                  <input type="hidden" name="tab" value="maps"> 
                  <input type="hidden" name="toggleMapID" value="'.$id.'">  
                  <input type="hidden" name="toggleMapname" value="'.$mapname.'">  
                  <input type="hidden" name="toggleMapStatus" value="yes">
                  <input type="submit" value="No" style="width:80px; background: rgba(230, 0, 0, 0.7);">
                </form>
              </td>';
      }
      echo '<td>'.$country.'</td>
          <td>'.$flagsize.'</td>
          <td>'.$timezone.'</td>
          <td>'.$address.'</td>
          <td>'.$mapX.'</td>
          <td>'.$mapY.'</td>
          <td align="center">';
          $mapfile = $rootdir.'/maps'.'/'.$mapname.'.png';
          if (file_exists($mapfile)) {
            echo '<div style="opacity: 1.0;border-radius: 50%;width:16px; height:16px; background: rgba(0, 200, 0, 0.7);" title="/maps/'.$mapname.'.png"></div>';
          } 
          else {
            echo '<div style="opacity: 1.0;border-radius: 50%;width:16px; height:16px; background: rgba(230, 0, 0, 0.7);" title="/maps/'.$mapname.'.png"></div>';
          }
      echo '  </td>';

      echo '  <td align="center">';
          $checktable = 'desks_'.$mapname;
          $checktable_query = mysqli_query($dbLink, "SELECT * FROM $checktable");
          $checknum   = mysqli_num_rows ($checktable_query);
          if ($mapname == 'overview') {$checknum = 1;}
          if ($checknum != 0) {
            echo '<div style="opacity: 1.0;border-radius: 50%;width:16px; height:16px; background: rgba(0, 200, 0, 0.7);" title="desks_'.$mapname.'"></div>';  
          } 
          else {
            echo '<div style="opacity: 1.0;border-radius: 50%;width:16px; height:16px; background: rgba(230, 0, 0, 0.7);" title="desks_'.$mapname.'"></div>';  
          }
      echo '  </td>';
       echo ' <td align="center">';
          $flagfile = $rootdir.'/countryflags'.'/'.$country.'.svg';
          if (file_exists($flagfile) || $country == 'none') {
            echo '<div style="opacity: 1.0;border-radius: 50%;width:16px; height:16px; background: rgba(0, 200, 0, 0.7);" title="/countryflags/'.$country.'.svg"></div>';
          } 
          else {
            echo '<div style="opacity: 1.0;border-radius: 50%;width:16px; height:16px; background: rgba(230, 0, 0, 0.7);" title="/countryflags/'.$country.'.svg"></div>';
          }
      echo '  </td>';
          if (permcheck($_SESSION['username'], 'maps') > 1) {
          echo '  <td><form name="DeleteMap" action="'.$page.'?tab='.$activetab.'" method="post">
                <input type="hidden" name="tab" value="maps"> 
                <input type="hidden" name="deleteMapID" value="'.$id.'">  
                <input type="hidden" name="deleteMapname" value="'.$mapname.'">  
              <input type="submit" value="Delete">
              </form></td>';
          }
          else {
            echo '<td>&nbsp;</td>';
          }
      echo'</tr>';
    }

    
    // Include the creation of new maps for superadmin users only
      if (permcheck($_SESSION['username'], 'maps') > 1) {
      
      // All countryflags are already included in the countryflags directory
      $countryflag_files = scandir('../countryflags');
      
      echo '<tr><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td></tr>';      
      echo '<form name="AddMap" action="'.$page.'?tab='.$activetab.'" method="post"><tr>
          <input type="hidden" name="tab" value="maps">
          <td><input type="text" name="newMapName" style="width:180px;"></td>
          <td><input type="text" name="newMapItemscale" value="1" style="width:100px;"></td>
          <td><select name="newMapPublished" style="width:80px">
            <option value="yes" selected>Yes</option>
            <option value="no">No</option>
            </select>
          </td>
          <td><select name="newMapCountry" style="width:110px">';
      // to prevent faulty countryflags values, the directory is scanned and a dropdown is used instead of a text field
      for ($c = 2; $c < count($countryflag_files); $c++) {
        $countryparts = explode('.', $countryflag_files[$c]);
        $countrytag = strtolower($countryparts[0]);
        echo '<option value="'.$countrytag.'">'.$countrytag.'</option>';
      }
          
      echo '</select></td>
            <td><input type="text" name="newMapFlagsize" style="width:100px;"></td>
            <td style="width:150px">';
      $regions = array(
        'Africa' => DateTimeZone::AFRICA,
        'America' => DateTimeZone::AMERICA,
        'Antarctica' => DateTimeZone::ANTARCTICA,
        'Aisa' => DateTimeZone::ASIA,
        'Atlantic' => DateTimeZone::ATLANTIC,
        'Europe' => DateTimeZone::EUROPE,
        'Indian' => DateTimeZone::INDIAN,
        'Pacific' => DateTimeZone::PACIFIC
    );
    $timezones = array();
    foreach ($regions as $name => $mask)
    {
        $zones = DateTimeZone::listIdentifiers($mask);
        foreach($zones as $timezone)
        {
        // Lets sample the time there right now
        $time = new DateTime(NULL, new DateTimeZone($timezone));
        // Us dumb Americans can't handle millitary time
        $ampm = $time->format('H') > 12 ? ' ('. $time->format('g:i a'). ')' : '';
        // Remove region name and add a sample time
        $timezones[$name][$timezone] = substr($timezone, strlen($name) + 1) . ' - ' . $time->format('H:i') . $ampm;
      }
    }
    // View
    print '<select name="newMapTimezone">';
    foreach($timezones as $region => $list)
    {
      print '<optgroup label="' . $region . '">' . "\n";
      foreach($list as $timezone => $name)
      {
        print '<option value="' . $timezone . '">' . $name . '</option>' . "\n";
      }
      print '<optgroup>' . "\n";
    }
    print '</select>';
      echo '</td>
          <td><input type="text" name="newMapAddress" style="width:200px;"></td>
          <td><input type="text" name="newMapX" style="width:80px;"></td>
          <td><input type="text" name="newMapY" style="width:80px;"></td>
          <td><input type="submit" value="Next"></td>
          <td>&nbsp;</td>
          <td>&nbsp;</td>
        </tr></form>';
    }

    echo "</table>";

    break;

  case "desks":
    echo '<script>var departments = '.json_encode($department_list, JSON_FORCE_OBJECT).';</script>';
    $dbTable = 'config_maplist';
    $dbLink  = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
    $details = mysqli_query($dbLink, "SELECT `mapname` FROM `config_maplist` 
    WHERE `published`='yes' 
    AND `mapname` NOT LIKE '%-nomap%' 
    AND `mapname` NOT LIKE 'overview' 
    ORDER BY `mapname` ASC");
    $num     = mysqli_num_rows ($details);   
    for ($i = 0; $i < $num; $i++) {
      $mapname   = mysqli_result($details,$i,0);
      echo "<div id='$mapname' class='deskstats'><img src='../images/spinner.png' style='margin-left:117px' /></div>";
      echo "<script>deskSummary('$mapname')</script>";
    }
    
    break;

  case "users":
    $dbTable = 'config_mapadmins';
    $dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);

    // Input new user before query is done
    if ($newadminuser != "" && $newadminrole != "") {
      $newadminuser = str_replace("\\", "\\\\", $newadminuser);
      mysqli_query($dbLink, "INSERT INTO `$dbName`.`$dbTable` (`ID`, `user`, `role`) VALUES (NULL, '$newadminuser', '$newadminrole');");
      auditlog("Users",$_SESSION['username'],"New admin has been created (".$newadminuser.", ".$newadminrole.")");
    }

    // Delete user before query is done
    if ($deleteID != "" && $deleteUser != "" && $deleteRole != "") {
      $deleteUser = str_replace("\\", "\\\\", $deleteUser);
      mysqli_query($dbLink, "DELETE FROM $dbTable WHERE `ID` = '$deleteID' AND `user` = '$deleteUser' AND `role` = '$deleteRole'");
      auditlog("Users",$_SESSION['username'],"Admin has been removed (".$deleteUser.", ".$deleteRole.")");
    }


    // Query all users
    $details = mysqli_query($dbLink, "SELECT * FROM $dbTable");
    $num   = mysqli_num_rows ($details);   

    echo "<table style='margin-left: 20px'>";
    echo '<tr style="font-weight:bold"><td width="350">Username</td><td width="200">Role</td><td width="210">&nbsp;</td></tr>';
    echo '<tr><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td></tr>';
    
    $roleTable = 'config_roles';
    $roleLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
    $roleDetails = mysqli_query($roleLink, "SELECT * FROM $roleTable");
    $roleNum   = mysqli_num_rows ($roleDetails);

    for ($i = 0; $i < $num; $i++) {
      $id     = mysqli_result($details,$i,0);
      $user   = mysqli_result($details,$i,1);
      $role   = mysqli_result($details,$i,2);
      $roleEcho = $role;
      for ($r = 0; $r < $roleNum; $r++) {
        if ($role == mysqli_result($roleDetails,$r,0)) {
          $roleEcho = mysqli_result($roleDetails,$r,1);
          break;
        }
      }
      echo '<tr><td>'.$user.'</td><td>'.$roleEcho.'</td><td>';
      if (permcheck($_SESSION['username'], 'users') > 1) {
        echo '
          <form name="DeleteAdmin" action="'.$page.'?tab='.$activetab.'" method="post">
            <input type="hidden" name="tab" value="users"> 
            <input type="hidden" name="deleteID" value="'.$id.'">  
            <input type="hidden" name="deleteUser" value="'.$user.'">  
            <input type="hidden" name="deleteRole" value="'.$role.'">  
            <input type="submit" value="Delete">
          </form>';
      }
      else {
        echo '&nbsp;';
      }
      echo '</td></tr>';
    }

    // Include the creation of new admin accounts for superadmin users only
    if (permcheck($_SESSION['username'], 'users') > 1) {
      echo '<tr><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td></tr>';
      echo '<form name="AddAdmin" action="'.$page.'?tab='.$activetab.'" method="post"><tr>
          <input type="hidden" name="tab" value="users">
          <td><input type="text" name="newadminuser" style="width:320px;"></td>
          <td>
            <select name="newadminrole" style="width:180px">';
            for ($r = 0; $r < $roleNum; $r++) {
              $roleID   = mysqli_result($roleDetails,$r,0);
              $roleName = mysqli_result($roleDetails,$r,1);
              echo '<option value="'.$roleID.'" selected>'.$roleName.'</option>';
            }
    echo '  </select>
          </td>
          <td><input type="submit" value="Create"></td>
        </tr></form>';
    }
    
    echo "</table>";

    break;

    case "teams":
    $dbTable = 'config_teams';
    $dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
    mysqli_set_charset($dbLink, "utf8");

    // Input new team before query is done
    if ($createTeam != "" && $createMembers != "") {
      $createTeam = str_replace("\\", "\\\\", $createTeam);
      mysqli_query($dbLink, "INSERT INTO `$dbName`.`$dbTable` (`ID`, `teamname`, `teammembers`) VALUES (NULL, '$createTeam', '$createMembers');");
      auditlog("Teams",$_SESSION['username'],"New team has been created (".$createTeam.", ".$createMembers.")");
    }

    // Delete team before query is done
    if ($deleteID != "" && $deleteTeam != "" && $deleteMembers != "") {
      $deleteTeam = str_replace("\\", "\\\\", $deleteTeam);
      mysqli_query($dbLink, "DELETE FROM $dbTable WHERE `ID` = '$deleteID' AND `teamname` = '$deleteTeam' AND `teammembers` = '$deleteMembers'");
      auditlog("Teams",$_SESSION['username'],"Team has been removed (".$deleteTeam.", ".$deleteMembers.")");
    }

    // Query all users
    $details = mysqli_query($dbLink, "SELECT * FROM $dbTable");
    $num   = mysqli_num_rows ($details);   

    echo "<table style='margin-left:20px;'>";
    echo '<tr style="font-weight:bold"><td width="350">Name</td><td width="850">Members</td><td width="150">&nbsp;</td></tr>';
    echo '<tr><td>&nbsp;</td><td>&nbsp;</td></tr>';
    
    for ($i = 0; $i < $num; $i++) {
      $id          = mysqli_result($details,$i,0);
      $teamname    = mysqli_result($details,$i,1);
      $teammembers = mysqli_result($details,$i,2);
      echo '<tr><td>'.$teamname.'</td><td>'.$teammembers.'</td><td>';
      if (permcheck($_SESSION['username'], 'teams') > 1) {
        echo '
          <form name="DeleteTeam" action="'.$page.'?tab='.$activetab.'" method="post">
            <input type="hidden" name="tab" value="teams"> 
            <input type="hidden" name="deleteID" value="'.$id.'">  
            <input type="hidden" name="deleteTeam" value="'.$teamname.'">  
            <input type="hidden" name="deleteMembers" value="'.$teammembers.'">  
            <input type="submit" value="Delete">
          </form>';
      }
      else {
        echo '&nbsp;';
      }
      echo '</td></tr>';
    }

    // All Admins are allowed to create teams
    echo '<tr><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td></tr>';
    echo '<form name="NewTeam" action="'.$page.'?tab='.$activetab.'" method="post"><tr>
        <input type="hidden" name="tab" value="teams">
        <td><input type="text" name="newTeam" style="width:320px;"></td>
        <td><input type="text" name="newMembers" style="width:820px;"></td>
        <td><input type="submit" value="Create"></td>
      </tr></form>';
    
    echo "</table>";

    break;  

  // STATS TAB start
  case "stats":
  echo str_repeat(' ',1024*64); // This is for the buffer achieve the minimum size in order to flush data
  flush(); // Send output to browser immediately

  ?>
  <h2 style='margin-left:10px'>Daily chart</h2>
  <script>showCharts('day', 'daycanvas')</script>

  <h2 style='margin-left:10px'>Monthly chart</h2>
  <script>showCharts('month', 'monthcanvas')</script>

  <h2 style='margin-left:10px'>Yearly chart</h2>
  <script>showCharts('year', 'yearcanvas')</script>
  <?php
  
  break;
  // STATS TAB end

  case "auditlog":
    echo "<h2>AuditLog of the last changes</h2><br />";
    $dbTable = 'auditlog';     
    $dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
    if ($auditlogEventType != '') {
      $details = mysqli_query($dbLink, "SELECT * FROM $dbTable WHERE EventType='$auditlogEventType' ORDER BY id desc");
    }
    else {
      $details = mysqli_query($dbLink, "SELECT * FROM $dbTable ORDER BY id desc");
    }
    $num   = mysqli_num_rows ($details);   

    function AddFilterButton ($filtername) {
      global $page, $activetab, $auditlogEventType;
      $displayname = $filtername;
      if ($filtername == 'All') {$filtername = '';}
      echo '
        <div style="float: left; margin-right:10px;">
        <form name="FilterAccess" action="'.$page.'?tab='.$activetab.'" method="post">
          <input type="hidden" name="tab" value="auditlog"> 
          <input type="hidden" name="auditlogEventType" value="'.$filtername.'">';  
          if ($auditlogEventType == $filtername) {
            echo '<input type="submit" value="'.$displayname.'" style="width:80px; background: rgba(0, 200, 0, 0.7);">';
          }
          else {
            echo '<input type="submit" value="'.$displayname.'" style="width:80px; background: rgba(0, 0, 200, 0.7);">';
          }       
      echo '   
        </form>
        </div>';
    }
    
    AddFilterButton ('All');
    AddFilterButton ('Access');
    AddFilterButton ('Users');
    AddFilterButton ('Desks');
    AddFilterButton ('Teams');
    AddFilterButton ('LDAP');

    echo '<br style="clear:both" /><table>';
    echo '<tr style="font-weight:bold;">
        <td width="70">ID</td>
        <td width="200">EventTime</td>
        <td width="200">EventType</td>
        <td width="250">EventUser</td>
        <td width="880">EventInfo</td>
        </tr>';
    echo '<tr><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td><td>&nbsp;</td></tr>';
    
    // Restrict output to 200 entries to prevent excessive load
    if ($num >= 201) {$num = 200;}

    for ($i = 0; $i < $num; $i++) {
      $EventID  = mysqli_result($details,$i,0);
      $EventTime  = mysqli_result($details,$i,1);
      $EventType  = mysqli_result($details,$i,2);
      $EventUser  = mysqli_result($details,$i,3);
      $EventInfo  = mysqli_result($details,$i,4);
      echo '<tr>
          <td>'.$EventID.'</td>
          <td>'.$EventTime.'</td>
          <td>'.$EventType.'</td>
          <td>'.$EventUser.'</td>
          <td>'.$EventInfo.'</td>
        </tr>';
    }

    echo "</table><br /><br />";   
    //auditlog("Access", $_SESSION['username'], "User has accessed the AuditLog");
    break;
  }


  echo '</div>';
  // CONTENT DIV end

} 

else {
  echo "<script>loginForm(true, 'admin')</script>";
}

?>
</body>
</html>
