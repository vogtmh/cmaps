<?php
# CompanyMaps 8.1 LDAP API
# Release date 2023-03-20
# Copyright (c) 2016-2022 by MavoDev
# see https://www.mavodev.de for more details

# Allows to sync LDAP users to CompanyMaps

# required headers
header("Access-Control-Allow-Origin: *");
header("Content-Type: application/json; charset=UTF-8");

# Defines if MS Teams messages are sent
$enableMSTeams = true;

# Loading shared functions and config file
include '../../shared.php';

error_reporting(E_ERROR | E_WARNING | E_PARSE);

# Timestamp for current sync, used for changelog
$year = date("Y"); $month = date("m"); $day = date("d"); $hour = date("H"); $minute = date("i");

# connect to DB
$MySqlLink          = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
mysqli_query($MySqlLink,"SET NAMES 'utf8'");
$NamesOLD     = mysqli_query($MySqlLink, "SELECT `givenname`,`surname`,`ipphone` FROM `$ldapTable`;");
$NumOldNames  = mysqli_num_rows ($NamesOLD);

# shared function to create log entries
function ldapChangelog($changedUser, $changedAvatar, $changedType, $changedOld, $changedNew) {
  global $dbServer, $dbName, $dbUser, $dbPass, $year, $month, $day, $hour, $minute;
  $dbTable = 'ldap_changelog';
  $LogentryLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);

  mysqli_query($LogentryLink, "INSERT INTO `$dbName`.`$dbTable` (`ID`, `year`, `month`, `day`, `hour`, `minute`, `name`, `avatar`, `type`, `oldvalue`, `newvalue`) 
  VALUES (NULL, '$year', '$month', '$day', '$hour', '$minute', '$changedUser', '$changedAvatar','$changedType', '$changedOld', '$changedNew');");
  mysqli_close($LogentryLink);
}

$msteamscombined = '';

function notifyTeams($content) {
  global $MSTEAMS_WEBHOOK;

  $message = ["text" => "$content"];

  $c = curl_init($MSTEAMS_WEBHOOK);
  curl_setopt($c, CURLOPT_HTTPHEADER, ['Content-Type: application/json']);
  curl_setopt($c, CURLOPT_RETURNTRANSFER, true);
  curl_setopt($c, CURLOPT_POSTFIELDS, json_encode($message));
  $curlreturn = curl_exec($c);
  curl_close($c);
}

# Run an automated full import when started from CLI
if (php_sapi_name() == "cli") {
  echo "Running from CLI..\n";

  # Get LDAP configuration from database
  $LdapconfigTable = 'config_ldap';

  $LdapconfigDetails  = mysqli_query($MySqlLink, "SELECT * FROM $LdapconfigTable");
  $LdapconfigNum      = mysqli_num_rows ($LdapconfigDetails);   

  # Count old entries in LDAP-Mirror (complete cache of all LDAP sources)
  $OldEntries     = mysqli_query($MySqlLink, "SELECT * FROM `$ldapTable`;");
  $NumOldEntries  = mysqli_num_rows ($OldEntries);

  # Import Loop Stage 1-4 for every single AD connection
  for ($z = 0; $z < $LdapconfigNum; $z++) {
    $ldap_id          = mysqli_result($LdapconfigDetails,$z,0);
    $ldap_description = mysqli_result($LdapconfigDetails,$z,1);
    $ldap_server      = mysqli_result($LdapconfigDetails,$z,2);
    $ldap_type        = mysqli_result($LdapconfigDetails,$z,3);
    $ldap_ou          = mysqli_result($LdapconfigDetails,$z,4);
    $ldap_user        = mysqli_result($LdapconfigDetails,$z,5);
    $ldap_pass        = mysqli_result($LdapconfigDetails,$z,6);
    $ldap_lastsync    = mysqli_result($LdapconfigDetails,$z,7);
    
    $ldapcache    = 'ldapcache_'.$ldap_id;
    echo "$ldap_id $ldap_description --> $ldapcache ..\n";


  # STAGE 1: CONNECT TO LDAP CACHE

    $MySqlLink = mysqli_connect($dbServer,$dbUser,$dbPass, $dbName);
    mysqli_query($MySqlLink,"SET NAMES 'utf8'");
    if (!$MySqlLink) {
      throw new Exception('Cannot connect to MySQL database');
    }
    else {
      # clear old cache table
      mysqli_query($MySqlLink, "DELETE FROM `$ldapcache`;");
    }  


  # STAGE 2: CONNECT TO AD
    switch ($ldap_type){
      case "LDAP":
        $ldapconn=ldap_connect($ldap_server) or die("not connected");
        break;
      case "LDAPS":
        $ldapconn=ldap_connect("ldaps://$ldap_server",636) or die("not connected");		 					
        break;
      default:
        throw new Exception('LDAP Type not set in database');
    }
    
    ldap_set_option ($ldapconn, LDAP_OPT_TIMELIMIT,1);
    ldap_set_option ($ldapconn, LDAP_OPT_NETWORK_TIMEOUT,1);
    ldap_set_option($ldapconn, LDAP_OPT_PROTOCOL_VERSION, 3);

    if (!$ldapconn) {
      throw new Exception('Cannot connect to LDAP server');
    }

  # STAGE 3: QUERY OU ON LDAP SERVER

    if ($ldapconn && $MySqlLink) {

      $ldapresult=ldap_bind($ldapconn, $ldap_user, $ldap_pass) or die ("Error trying to bind: ".ldap_error($ldapconn));
      ldap_set_option ($ldapconn, LDAP_OPT_TIMELIMIT,15);

      if (!$ldapresult) {
        throw new Exception('Cannot query data from LDAP server');
      }

  # STAGE 4: IMPORT DATA into temporary cache and send notifications
      
      # Divide the AD query into 26 queries (=26*1000 max results)
      foreach(range('A','Z') as $letter) {
        # Search for AD objects that have a value in attribute "Office"
        $filter="(&(physicaldeliveryofficename=*)(givenname=$letter*))";
        $searchresult=ldap_search($ldapconn, $ldap_ou, $filter);

        $info = ldap_get_entries($ldapconn, $searchresult);
        $total = $info["count"];
        $ProgressDelay = 9;

        for ($i=0; $i<$info["count"]; $i++) {

          $givenname = $info[$i]["givenname"][0];
          $surname = str_replace("'","\'",$info[$i]["sn"][0]);
          $telephonenumber = $info[$i]["telephonenumber"][0];
          $mail = $info[$i]["mail"][0];
          $mail = str_replace("'","\'",$mail);
          $physicaldeliveryofficename = $info[$i]["physicaldeliveryofficename"][0];
          $ipphone = $info[$i]["samaccountname"][0];  
          #$mailparts = explode('@', $mail); # avatarname is now the first part of the mail address
          #$ipphone = strtolower($mailparts[0]);
          $description = str_replace("'","\'",$info[$i]["title"][0]);
          $department = $info[$i]["department"][0];
          $mobile = $info[$i]["mobile"][0];
          $fullname = $givenname.' '.$surname;


          if ($physicaldeliveryofficename != "" && $physicaldeliveryofficename != "-" && $givenname != "" && $surname != "" && $mail != "") {

            # Scan current user for changes and report them to the changelog
            $ldapcachescan = mysqli_query($MySqlLink, "SELECT * FROM `$ldapTable` WHERE `ipphone` = '$ipphone'");
            $num_ldapcachescan   = mysqli_num_rows ($ldapcachescan);
            # Get Variables of mysql Result
            $oldgivenname                  = mysqli_result($ldapcachescan,0,1);
            $oldsurname                    = mysqli_result($ldapcachescan,0,2);
            $oldtelephonenumber            = mysqli_result($ldapcachescan,0,3);
            $oldmail                       = mysqli_result($ldapcachescan,0,4);
            $oldphysicaldeliveryofficename = mysqli_result($ldapcachescan,0,5);
            $oldipphone                    = mysqli_result($ldapcachescan,0,6);
            $olddescription                = mysqli_result($ldapcachescan,0,7);
            $oldfullname=$oldgivenname.' '.$oldsurname;
            # Send changes to changelog and MS Teams
            $msteamsmessage = '';
            if ($oldipphone == '') {
              ldapChangelog($fullname, $ipphone, 'Employee', 'none', stripslashes($description));
              $msteamsmessage = 'New employee in Maps: '.$fullname.' - '.stripslashes($description);
              $outputText="$fullname : $description $telephonenumber $mail $physicaldeliveryofficename $ipphone $department $mobile \n";
              echo $outputText;
            }
            if ($olddescription != '' && $olddescription != stripslashes($description)) {
              ldapChangelog($fullname, $ipphone, 'Title', $olddescription, stripslashes($description));
              $msteamsmessage = 'New job title: '.$fullname.' - '.$olddescription.' --> '.stripslashes($description);
            }
            if ($olddescription != '' && $oldsurname != stripslashes($surname)) {
              ldapChangelog($fullname, $ipphone, 'Name', $oldsurname, stripslashes($surname));
            }

            # Prepare MS Teams Notification   
            if ($msteamsmessage != '') {
              $msteamscombined .= $msteamsmessage."   \n";
            }   
            
            # MySQL Dump    
            $physicaldeliveryofficename = str_replace(' ', '', $physicaldeliveryofficename);
            if (stripos($physicaldeliveryofficename, '|') !== false) {
              $deskplaces = explode('|', $physicaldeliveryofficename);
              for ($t = 0; $t < count($deskplaces); $t++) {
                mysqli_query($MySqlLink, "INSERT INTO `$dbName`.`$ldapcache` (`ID`, `givenname`, `surname`, `telephonenumber`, `mail`, `physicaldeliveryofficename`, `ipphone`, `description`, `department`, `mobile`) 
                VALUES (NULL, '$givenname', '$surname', '$telephonenumber', '$mail', '$deskplaces[$t]', '$ipphone', '$description', '$department', '$mobile');");  
              }
              
            }
            else {
              mysqli_query($MySqlLink, "INSERT INTO `$dbName`.`$ldapcache` (`ID`, `givenname`, `surname`, `telephonenumber`, `mail`, `physicaldeliveryofficename`, `ipphone`, `description`, `department`, `mobile`) 
              VALUES (NULL, '$givenname', '$surname', '$telephonenumber', '$mail', '$physicaldeliveryofficename', '$ipphone', '$description', '$department', '$mobile');");
            }   
          }       
        }

      } # end of one letter
      ldap_close($ldapconn); 
    } # end of current AD server
  } # end of all AD servers


  # STAGE 5: COMBINE ALL CACHES INTO LDAP MIRROR TABLE

  for ($y = 0; $y < $LdapconfigNum; $y++) {
    $ldap_id          = mysqli_result($LdapconfigDetails,$y,0); 
    $ldap_description = mysqli_result($LdapconfigDetails,$y,1);
    $ldapcache    = 'ldapcache_'.$ldap_id;
    # Copy databases one by one into Ldap Mirror - ID is not mentioned to prevent duplicate errors
    mysqli_query($MySqlLink, "Insert Into `$ldapTable` 
    (`givenname`, `surname`, `telephonenumber`, `mail`, `physicaldeliveryofficename`, `ipphone`, `description`, `department`, `mobile`) 
    Select 
    `givenname`, `surname`, `telephonenumber`, `mail`, `physicaldeliveryofficename`, `ipphone`, `description`, `department`, `mobile` 
    From `$ldapcache`;"); 

    $date = date_create();
    $EventTime = date_format($date, 'Y-m-d H:i:s') . "\n";
    mysqli_query($MySqlLink, "UPDATE `config_ldap` SET `LastSync`='$EventTime' WHERE `ID`='$ldap_id';");
    auditlog ('LDAP', 'System', $ldap_description.' has been synced.');
  }

  # Delete old entries in MySQL database
  mysqli_query($MySqlLink, "DELETE FROM `$ldapTable` LIMIT $NumOldEntries;");

  # Reset primary key (ID) to prevent IDs getting too big
  mysqli_query($MySqlLink, "ALTER TABLE `$ldapTable` DROP `ID`;");
  mysqli_query($MySqlLink, "ALTER TABLE `$ldapTable` ADD `ID` BIGINT( 200 ) NOT NULL AUTO_INCREMENT FIRST ,ADD PRIMARY KEY (`ID`);");

  # Send MS Teams Notification
  if ($msteamscombined != '' && $enableMSTeams) {
    notifyTeams($msteamscombined);
  }
  
  # Check for obsolete pictures
  $MySqlLink = mysqli_connect($dbServer,$dbUser,$dbPass, $dbName);
  $UserIDs = array();
  $UserIDtable = mysqli_query($MySqlLink, "SELECT `ipphone` FROM `ldap-mirror`");
  $NumIDs  = mysqli_num_rows ($UserIDtable);

  for ($z = 0; $z < $NumIDs; $z++) {
    $user_id = mysqli_result($UserIDtable,$z,0).'.jpg'; 
    $UserIDs[]=$user_id;
  }

  $files = scandir('../../avatarcache');
/*
  foreach ($files as $file) {
      if ($file != '.' && $file != '..' && $file != 'meetingrooms') {
          if (in_array($file, $UserIDs)) {
              #echo 'item INT001171 found';
              #echo '+';
          }
          else {
              $file_pointer = "../../avatarcache/$file";
              if (!unlink($file_pointer)) {
                  echo ("$file_pointer cannot be deleted due to an error \n");
              }
              else {
                  echo ("$file_pointer has been deleted \n");
              }
          }
      }
  }*/
  
  ## check for obsolete users

  # Get LDAP configuration from database
  $LdapconfigTable = 'config_ldap';

  $MySqlLink          = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
  mysqli_query($MySqlLink,"SET NAMES 'utf8'");
  $LdapconfigDetails  = mysqli_query($MySqlLink, "SELECT * FROM $LdapconfigTable");
  $LdapconfigNum      = mysqli_num_rows ($LdapconfigDetails);   

  $ldap_server      = mysqli_result($LdapconfigDetails,0,2);
  $ldap_type        = mysqli_result($LdapconfigDetails,0,3);
  $ldap_ou          = mysqli_result($LdapconfigDetails,0,4);
  $ldap_user        = mysqli_result($LdapconfigDetails,0,5);
  $ldap_pass        = mysqli_result($LdapconfigDetails,0,6);
  # CONNECT TO AD
    switch ($ldap_type){
      case "LDAP":
        $ldapconnUser=ldap_connect($ldap_server) or die("not connected");
        break;
      case "LDAPS":
        $ldapconnUser=ldap_connect("ldaps://$ldap_server",636) or die("not connected");		 					
        break;
      default:
        throw new Exception('LDAP Type not set in database');
    }
    
    ldap_set_option ($ldapconnUser, LDAP_OPT_TIMELIMIT,1);
    ldap_set_option ($ldapconnUser, LDAP_OPT_NETWORK_TIMEOUT,1);
    ldap_set_option($ldapconnUser, LDAP_OPT_PROTOCOL_VERSION, 3);

    if (!$ldapconnUser) {
      throw new Exception('Cannot connect to LDAP server');
    }

    $ldapresultUser=ldap_bind($ldapconnUser, $ldap_user, $ldap_pass) or die ("Error trying to bind: ".ldap_error($ldapconnUser));
    ldap_set_option ($ldapconnUser, LDAP_OPT_TIMELIMIT,15);

    if (!$ldapresultUser) {
      throw new Exception('Cannot query data from LDAP server');
    }
    else {
      foreach ($files as $file) {
        if ($file != '.' && $file != '..' && $file != 'README.MD' && $file != 'meetingrooms') {
          $filename=pathinfo($file, PATHINFO_FILENAME);
          $filter="(&(samaccountname=$filename))";
          $searchresult=ldap_search($ldapconnUser, $ldap_ou, $filter);
          $info = ldap_get_entries($ldapconnUser, $searchresult);
          $total = $info["count"];
          if ($total != 1) {
            $file_pointer = "../../avatarcache/$file";
            echo "User $filename not found in AD, deleting avatar .. \n";
            if (!unlink($file_pointer)) {
                echo ("$file_pointer cannot be deleted due to an error \n");
            }
            else {
                echo ("$file_pointer has been deleted \n");
            }
          }
        }
      }

    }
    ldap_close($ldapconnUser);
}

# Check for credentials and parameters on web access
else {
  if ($_SERVER['REQUEST_METHOD'] == "GET") {
    $token = htmlspecialchars($_GET['token'], ENT_QUOTES);
    $ldapid = htmlspecialchars($_GET['ldapid'], ENT_QUOTES); 
    $user = htmlspecialchars($_GET['user'], ENT_QUOTES);
  }
  
  $checktoken = strrev(date("Ymd")) + date("Ymd");

  if ($token != $checktoken) {
    throw new Exception('Authorization failed');
  } 
  else {
    # Check LDAP ID
    $LdapconfigTable = 'config_ldap';
    $MySqlLink   = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
    mysqli_query($MySqlLink,"SET NAMES 'utf8'");
    $LdapconfigDetails  = mysqli_query($MySqlLink, "SELECT * FROM $LdapconfigTable WHERE ID='$ldapid'");
    $LdapconfigNum    = mysqli_num_rows ($LdapconfigDetails);
  
    $ldap_description = mysqli_result($LdapconfigDetails,0,1);
  
    if ($ldap_description=='') {
      throw new Exception('Unknown LDAP ID');
    }

    # Import Loop Stage 1-4 for every single AD connection
    $ldap_id          = mysqli_result($LdapconfigDetails,0,0);
    $ldap_description = mysqli_result($LdapconfigDetails,0,1);
    $ldap_server      = mysqli_result($LdapconfigDetails,0,2);
    $ldap_type        = mysqli_result($LdapconfigDetails,0,3);
    $ldap_ou          = mysqli_result($LdapconfigDetails,0,4);
    $ldap_user        = mysqli_result($LdapconfigDetails,0,5);
    $ldap_pass        = mysqli_result($LdapconfigDetails,0,6);
    $ldap_lastsync    = mysqli_result($LdapconfigDetails,0,7);
    
    $ldapcache    = 'ldapcache_'.$ldap_id;

    # STAGE 1: CONNECT TO LDAP CACHE

    $MySqlLink = mysqli_connect($dbServer,$dbUser,$dbPass, $dbName);
    mysqli_query($MySqlLink,"SET NAMES 'utf8'");
    if (!$MySqlLink) {
      throw new Exception('Cannot connect to MySQL database');
    }
    else {
      # clear old cache table
      mysqli_query($MySqlLink, "DELETE FROM `$ldapcache`;");
    }  

    # STAGE 2: CONNECT TO AD

    switch ($ldap_type){
      case "LDAP":
        $ldapconn=ldap_connect($ldap_server) or die("not connected");
        break;
      case "LDAPS":
        $ldapconn=ldap_connect("ldaps://$ldap_server",636) or die("not connected");		 					
        break;
      default:
        throw new Exception('LDAP Type not set in database');
    }

    ldap_set_option ($ldapconn, LDAP_OPT_TIMELIMIT,1);
    ldap_set_option ($ldapconn, LDAP_OPT_NETWORK_TIMEOUT,1);
    ldap_set_option($ldapconn, LDAP_OPT_PROTOCOL_VERSION, 3);

    if (!$ldapconn) {
      throw new Exception('Cannot connect to LDAP server');
    }

    # STAGE 3: QUERY OU ON LDAP SERVER

    if ($ldapconn && $MySqlLink) {

      $ldapresult=ldap_bind($ldapconn, $ldap_user, $ldap_pass) or die ("Error trying to bind: ".ldap_error($ldapconn));
      ldap_set_option ($ldapconn, LDAP_OPT_TIMELIMIT,15);

      if (!$ldapresult) {
        throw new Exception('Cannot query data from LDAP server');
      }

    # STAGE 4: IMPORT DATA into temporary cache and send notifications
        
    # Divide the AD query into 26 queries (=26*1000 max results)
      foreach(range('A','Z') as $letter) {
        # Search for AD objects that have a value in attribute "Office"
        $filter="(&(physicaldeliveryofficename=*)(givenname=$letter*))";
        $searchresult=ldap_search($ldapconn, $ldap_ou, $filter);

        $info = ldap_get_entries($ldapconn, $searchresult);
        $total = $info["count"];
        $ProgressDelay = 9;

        for ($i=0; $i<$info["count"]; $i++) {

          $givenname = $info[$i]["givenname"][0];
          $surname = str_replace("'","\'",$info[$i]["sn"][0]);
          $telephonenumber = $info[$i]["telephonenumber"][0];
          $mail = $info[$i]["mail"][0];
          $mail = str_replace("'","\'",$mail);
          $physicaldeliveryofficename = $info[$i]["physicaldeliveryofficename"][0];
          $ipphone = $info[$i]["samaccountname"][0];  
          #$mailparts = explode('@', $mail); # avatarname is now the first part of the mail address
          #$ipphone = strtolower($mailparts[0]);
          $description = str_replace("'","\'",$info[$i]["title"][0]);
          $department = $info[$i]["department"][0];
          $mobile = $info[$i]["mobile"][0];
          $fullname = $givenname.' '.$surname;

          if ($physicaldeliveryofficename != "" && $physicaldeliveryofficename != "-" && $givenname != "" && $surname != "" && $mail != "") {

            # Scan current user for changes and report them to the changelog
            $ldapcachescan = mysqli_query($MySqlLink, "SELECT * FROM `$ldapTable` WHERE `ipphone` = '$ipphone'");
            $num_ldapcachescan   = mysqli_num_rows ($ldapcachescan);
            # Get Variables of mysql Result
            $oldgivenname                  = mysqli_result($ldapcachescan,0,1);
            $oldsurname                    = mysqli_result($ldapcachescan,0,2);
            $oldtelephonenumber            = mysqli_result($ldapcachescan,0,3);
            $oldmail                       = mysqli_result($ldapcachescan,0,4);
            $oldphysicaldeliveryofficename = mysqli_result($ldapcachescan,0,5);
            $oldipphone                    = mysqli_result($ldapcachescan,0,6);
            $olddescription                = mysqli_result($ldapcachescan,0,7);
            $oldfullname=$oldgivenname.' '.$oldsurname;
            # Send changes to changelog and MS Teams
            $msteamsmessage = '';
            if ($olddescription == '' && $description != '') {
              ldapChangelog($fullname, $ipphone, 'Employee', 'none', stripslashes($description));
              $msteamsmessage = 'New employee in Maps: '.$fullname.' - '.stripslashes($description);
            }
            if ($olddescription != '' && $olddescription != stripslashes($description)) {
              ldapChangelog($fullname, $ipphone, 'Title', $olddescription, stripslashes($description));
              $msteamsmessage = 'New job title: '.$fullname.' - '.$olddescription.' --> '.stripslashes($description);
            }
            if ($olddescription != '' && $oldsurname != stripslashes($surname)) {
              ldapChangelog($fullname, $ipphone, 'Name', $oldsurname, stripslashes($surname));
            }

            # Prepare MS Teams Notification   
            if ($msteamsmessage != '') {
              $msteamscombined .= $msteamsmessage."   \n";
            }   
            
            # MySQL Dump    
            $physicaldeliveryofficename = str_replace(' ', '', $physicaldeliveryofficename);
            if (stripos($physicaldeliveryofficename, '|') !== false) {
              $deskplaces = explode('|', $physicaldeliveryofficename);
              for ($t = 0; $t < count($deskplaces); $t++) {
                mysqli_query($MySqlLink, "INSERT INTO `$dbName`.`$ldapcache` (`ID`, `givenname`, `surname`, `telephonenumber`, `mail`, `physicaldeliveryofficename`, `ipphone`, `description`, `department`, `mobile`) 
                VALUES (NULL, '$givenname', '$surname', '$telephonenumber', '$mail', '$deskplaces[$t]', '$ipphone', '$description', '$department', '$mobile');");  
              }
              
            }
            else {
              mysqli_query($MySqlLink, "INSERT INTO `$dbName`.`$ldapcache` (`ID`, `givenname`, `surname`, `telephonenumber`, `mail`, `physicaldeliveryofficename`, `ipphone`, `description`, `department`, `mobile`) 
              VALUES (NULL, '$givenname', '$surname', '$telephonenumber', '$mail', '$physicaldeliveryofficename', '$ipphone', '$description', '$department', '$mobile');");
            }   
          }       
        }

      } # end of one letter
      ldap_close($ldapconn); 
    } # end of current AD server

    # STAGE 5: COMBINE ALL CACHES INTO LDAP MIRROR TABLE

    # Count old entries in LDAP-Mirror for Stage 5
    $LdapconfigAll     = mysqli_query($MySqlLink, "SELECT * FROM $LdapconfigTable");
    $LdapconfigAllNum  = mysqli_num_rows ($LdapconfigAll);   

    $OldEntries     = mysqli_query($MySqlLink, "SELECT * FROM `$ldapTable`;");
    $NumOldEntries  = mysqli_num_rows ($OldEntries);

    auditlog ("LDAP", $user, "$ldap_description has been synced.");

    for ($y = 0; $y < $LdapconfigAllNum; $y++) {
      $ldap_id          = mysqli_result($LdapconfigAll,$y,0); 
      $ldap_description = mysqli_result($LdapconfigAll,$y,1);
      $ldapcache    = 'ldapcache_'.$ldap_id;
      # Copy databases one by one into Ldap Mirror - ID is not mentioned to prevent duplicate errors
      mysqli_query($MySqlLink, "Insert Into `$ldapTable` 
      (`givenname`, `surname`, `telephonenumber`, `mail`, `physicaldeliveryofficename`, `ipphone`, `description`, `department`, `mobile`) 
      Select 
      `givenname`, `surname`, `telephonenumber`, `mail`, `physicaldeliveryofficename`, `ipphone`, `description`, `department`, `mobile` 
      From `$ldapcache`;"); 

      $date = date_create();
      $EventTime = date_format($date, 'Y-m-d H:i:s') . "\n";
      mysqli_query($MySqlLink, "UPDATE `config_ldap` SET `LastSync`='$EventTime' WHERE `ID`='$ldap_id';");
    }

    # Delete old entries in MySQL database
    mysqli_query($MySqlLink, "DELETE FROM `$ldapTable` LIMIT $NumOldEntries;");

    # Reset primary key (ID) to prevent IDs getting too big
    mysqli_query($MySqlLink, "ALTER TABLE `$ldapTable` DROP `ID`;");
    mysqli_query($MySqlLink, "ALTER TABLE `$ldapTable` ADD `ID` BIGINT( 200 ) NOT NULL AUTO_INCREMENT FIRST ,ADD PRIMARY KEY (`ID`);");

    # Send MS Teams Notification   
    if ($msteamscombined != '' && $enableMSTeams) {
      notifyTeams($msteamscombined);
    }   
  
    # Initialize array for JSON output
    $ldap_arr=array();
    $ldap_arr["ldap"]=array();

    # Output values
    $ldap_item=array(
        "status"     => "LDAP ID $ldap_description",
        "info"    => "success with user $user, $LdapconfigAllNum syncs merged, $NumOldEntries old entries removed.",
        "data"   => 'no more data - all good',
    );
    array_push($ldap_arr["ldap"], $ldap_item);
  
    # Send output to client
    ob_start('ob_gzhandler');
    echo json_encode($ldap_arr); 
  }
}

# Comparison of old and new names
$MySqlLink          = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
mysqli_query($MySqlLink,"SET NAMES 'utf8'");
$NamesNEW     = mysqli_query($MySqlLink, "SELECT `givenname`,`surname`,`ipphone` FROM `$ldapTable`;");
$NumNewNames  = mysqli_num_rows ($NamesNEW);
for ($o = 0; $o < $NumOldNames; $o++) {
  $oldgivenname                  = mysqli_result($NamesOLD,$o,0);
  $oldsurname                    = mysqli_result($NamesOLD,$o,1);
  $oldipphone                    = mysqli_result($NamesOLD,$o,2);
  $missing = true;
  for ($n = 0; $n < $NumNewNames; $n++) {
    $newgivenname                  = mysqli_result($NamesNEW,$n,0);
    $newsurname                    = mysqli_result($NamesNEW,$n,1);
    $newipphone                    = mysqli_result($NamesNEW,$n,2);
    if ($oldipphone == $newipphone) {
      $missing = false;
      break;
    }
  }
  if ($missing) {
    echo "$newgivenname $newsurname left Maps \n";
  }
  else {
    echo "$newgivenname $newsurname still in Maps \n";
  }
}

?>
