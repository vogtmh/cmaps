<?php

/*===================================================================
  
  CompanyMaps 8.0 SharedLibs
  Release date 2022-11-14
  Copyright (c) 2016-2022 by MavoDev
  see https://www.mavodev.de for more details
  
==================================================================== */
    # include config file
    include __DIR__ ."/../config_cmaps.php"; 

    # shared function for the substitution of the old mysql function
    function mysqli_result($result, $row, $field = 0) {
      # Adjust the result pointer to that specific row
      $result->data_seek($row);
      # Fetch result array
      $data = $result->fetch_array();
  
      return $data[$field];
      }

    # shared function to create log entries
    function auditlog($EventType, $EventUser, $EventInfo) {
      global $dbServer, $dbName, $dbUser, $dbPass;
      $dbTable = 'auditlog';
      $dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);

      $date = date_create();
      $EventTime = date_format($date, 'Y-m-d H:i:s') . "\n";
      $EventUser = str_replace("\\", "\\\\", $EventUser);

      mysqli_query($dbLink, "INSERT INTO `$dbName`.`$dbTable` (`ID`, `EventTime`, `EventType`, `EventUser`, `EventInfo`) VALUES (NULL, '$EventTime', '$EventType', '$EventUser', '$EventInfo');");
      mysqli_close($dbLink);
    }

    # Permission check for current logged-in user. Returns level of permission (0=none, 1=read, 2=write)
    function permcheck($PermUser, $Permission) {
      global $dbServer, $dbName, $dbUser, $dbPass;
      $userLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
      $userQuery = mysqli_query($userLink, "SELECT * FROM `config_mapadmins`");
      $userNum   = mysqli_num_rows ($userQuery);
      for ($t = 0; $t < $userNum; $t++) {
        $user = mysqli_result($userQuery,$t,1);
        $role = mysqli_result($userQuery,$t,2);
        if ($user == $PermUser) {
          # connect to roletable to get permissions for role
          $roleLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
          $roleDetails = mysqli_query($roleLink, "SELECT * FROM `config_roles`");
          $roleNum   = mysqli_num_rows ($roleDetails);
          for ($r = 0; $r < $roleNum; $r++) {
            $roleID   = mysqli_result($roleDetails,$r,0);
            $roleName = mysqli_result($roleDetails,$r,1);
            if ($role == $roleID) {
              $PermissionField = 'perm_'.$Permission;
              $rolePermissions = mysqli_query($roleLink, "SELECT `$PermissionField` FROM `config_roles` WHERE `ID` = $roleID");
              return $roleName = mysqli_result($rolePermissions,0,0);
              break;
            }
          }
        }    
      }
      mysqli_close($userLink);
      return 0;
    }

  # Output debug messages into browser console
  function debug_to_console( $data ) {
      $output = $data;
      if ( is_array( $output ) )
          $output = implode( ',', $output);
  
      echo "<script>console.log( 'Debug Objects: " . $output . "' );</script>";
  }

    # get departments from DB
    $dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
    $query = mysqli_query($dbLink, "SELECT * FROM `config_department_list` ORDER BY `department-name`");
    $num   = mysqli_num_rows ($query);
    $department_list = array();
    for ($t = 0; $t < $num; $t++) {
      $depname = mysqli_result($query,$t,1);
      array_push($department_list, $depname);
      }
    mysqli_close($dbLink);

    # get general variables from DB
		$dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
    $query = mysqli_query($dbLink, "SELECT * FROM `config_general`");
    $num   = mysqli_num_rows ($query);
    for ($t = 0; $t < $num; $t++) {
      $variable = mysqli_result($query,$t,1);
      $value    = mysqli_result($query,$t,2);
      $$variable = $value;
      }
    mysqli_close($dbLink);
    if ($logo_regular == '') {$logo_regular='images/cmaps-regular.png';}
    if ($logo_hover == '') {$logo_hover='images/cmaps-hover.png';}
    if ($apptitle == '') {$apptitle='CompanyMaps';}

    # get mapadmins from DB
    $dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
    $query = mysqli_query($dbLink, "SELECT * FROM `config_mapadmins`");
    $num   = mysqli_num_rows ($query);
    $mapadmins = array();
    for ($t = 0; $t < $num; $t++) {
      $user = mysqli_result($query,$t,1);
      $role = mysqli_result($query,$t,2);
      array_push($mapadmins, $user);
      }
    mysqli_close($dbLink);

    # get maplist and scales from DB
    $dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
    $query = mysqli_query($dbLink, "SELECT * FROM `config_maplist` order by mapname");
    $num   = mysqli_num_rows ($query);
    $maplist = array();
    for ($t = 0; $t < $num; $t++) {
      $mapname   = mysqli_result($query,$t,1);
      $mapactive = mysqli_result($query,$t,3);
      ${'itemscale_'.$mapname} = mysqli_result($query,$t,2);
      if ($mapactive == 'yes') {
        array_push($maplist, $mapname);
      }
    }

    # create empty tables if missing
    mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`config_robinspaces` ( `ID` INT NOT NULL AUTO_INCREMENT , `spacename` TEXT NOT NULL , `spaceid` INT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");

    mysqli_close($dbLink);
?>