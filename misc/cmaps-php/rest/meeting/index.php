<?php

# CompanyMaps 8.1 Meeting API
# Release date 2023-03-20
# Copyright (c) 2016-2022 by MavoDev
# see https://www.mavodev.de for more details

// required headers
header("Access-Control-Allow-Origin: *");
header("Content-Type: application/json; charset=UTF-8");

# Loading shared functions and config file
include '../../shared.php';

if (!empty($argv[1])) {
    parse_str($argv[1], $_GET);
  }

$dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
mysqli_set_charset($dbLink, "utf8");

// Check if the script is run in the browser or the CLI
if (php_sapi_name() == "cli") {
    $isbrowser = false;
    error_reporting(E_ERROR | E_WARNING | E_PARSE);
    // Connect to caching database
    $dbTable = 'meetingstatus';
    
}
else {
    $isbrowser = true;
    // Create array for output
    $meeting_arr=array();
    $meeting_arr["rooms"]=array();
}

$map = htmlspecialchars($_GET['map'], ENT_QUOTES);  
$usecache = htmlspecialchars($_GET['usecache'], ENT_QUOTES);

// Using the cached version for faster checks
if ($usecache == true) {
    $dbTable = 'meetingstatus';
    if ($map != '') {
        $cached  = mysqli_query($dbLink, "SELECT * FROM `$dbTable` WHERE `map` LIKE '$map'");
    }
    else {
        $cached  = mysqli_query($dbLink, "SELECT * FROM `$dbTable`");
    }
    $numcache = mysqli_num_rows ($cached);
    for ($i = 0; $i < $numcache; $i++) {
        $roomitem=array(
            "map"          => mysqli_result($cached,$i,1),
            "name"         => mysqli_result($cached,$i,2),
            "availability" => mysqli_result($cached,$i,3),
            "now_title"    => mysqli_result($cached,$i,4),
            "now_start"    => mysqli_result($cached,$i,5),
            "now_end"      => mysqli_result($cached,$i,6),
            "now_tz"       => mysqli_result($cached,$i,7),
            "next_title"   => mysqli_result($cached,$i,8),
            "next_start"   => mysqli_result($cached,$i,9),
            "next_end"     => mysqli_result($cached,$i,10),
            "next_tz"      => mysqli_result($cached,$i,11),
            "deskid"       => mysqli_result($cached,$i,12),
        );
        array_push($meeting_arr["rooms"], $roomitem);
    }
    echo json_encode($meeting_arr);
    exit();
}

// Function to simplify API calls
function callAPI($method, $url, $data){
    global $robintoken;
    $curl = curl_init();
 
    switch ($method){
       case "POST":
          curl_setopt($curl, CURLOPT_POST, 1);
          if ($data)
             curl_setopt($curl, CURLOPT_POSTFIELDS, $data);
          break;
       case "PUT":
          curl_setopt($curl, CURLOPT_CUSTOMREQUEST, "PUT");
          if ($data)
             curl_setopt($curl, CURLOPT_POSTFIELDS, $data);			 					
          break;
       default:
          if ($data)
             $url = sprintf("%s?%s", $url, http_build_query($data));
    }
 
    // OPTIONS:
    curl_setopt($curl, CURLOPT_URL, $url);
    curl_setopt($curl, CURLOPT_HTTPHEADER, array(
       'Authorization: Access-Token '.$robintoken,
       'Content-Type: application/json',
    ));
    curl_setopt($curl, CURLOPT_RETURNTRANSFER, 1);
    curl_setopt($curl, CURLOPT_HTTPAUTH, CURLAUTH_BASIC);
 
    // EXECUTE:
    $result = curl_exec($curl);
    if(!$result){die("Connection Failure");}
    curl_close($curl);
    return $result;
}

function listLocations() {
    global $robinOrganisation;
    $get_locations = callAPI('GET', 'https://api.robinpowered.com/v1.0/organizations/'.$robinOrganisation.'/locations?page=1&per_page=200', false);
    $locations = json_decode($get_locations, true);
    $locationcount = count($locations['data']);

    echo "$locationcount locations found: \n";
    for ($l = 0; $l < $locationcount; $l++) {
            $location = $locations['data'][$l];
            $city =  $location['name'];
            $address = $location['address'];
            $id = $location['id'];
            echo "$id - $city - $address \n";
    }
}

// Translates locations into Robin IDs - hardcoded
function getLocation($location) {
    global $dbLink, $dbName;
    $dbTable = 'config_robinspaces';

    $locationcheck = mysqli_query($dbLink,"SELECT `spaceid` FROM `config_robinspaces` WHERE `spacename` LIKE '$location'");
    $locationcount = mysqli_num_rows ($locationcheck);
    if ($locationcount == 1) {
        $spaceid = mysqli_result($locationcheck,0,0);
        return $spaceid;
    }
}

// Gets all rooms for a location
function getRooms($locationid, $locationname) {
    $get_data = callAPI('GET', 'https://api.robinpowered.com/v1.0/locations/'.$locationid.'/spaces?page=1&per_page=200', false);
    $response = json_decode($get_data, true);
    //$errors = $response['response']['errors'];

    for ($i = 0; $i < count($response['data']); $i++) {
        $counter = $response['data'][$i];
        getRoomDetails($counter['id'], $counter['name'], $locationname);
    }
}

// Gets rooms details for a room and passes them to the output array
function getRoomDetails($roomid, $roomname, $locationname) {
    global $meeting_arr, $isbrowser;
    global $dbName, $dbTable, $dbLink;

    // check availability
    $get_state = callAPI('GET', 'https://api.robinpowered.com/v1.0/spaces/'.$roomid.'/state', false);
    $state = json_decode($get_state, true);
    //$errors = $response['response']['errors'];
    $statedata = $state['data'];

    // check events
    $after  = gmdate('Y-m-d\TH:i:s\Z', strtotime('-24 hours'));
    $before = gmdate('Y-m-d\TH:i:s\Z', strtotime('+144 hours'));

    $get_events = callAPI('GET', 'https://api.robinpowered.com/v1.0/spaces/'.$roomid.'/events?after='.$after.'&before='.$before.'&page=1&per_page=200', false);
    $events = json_decode($get_events, true);
    //$errors = $response['response']['errors'];
    $eventdata = $events['data'];
    $current = $events['data'][0];

    $now_title = '';
    $now_start = '';
    $now_end = '';
    $now_tz = '';
    // find current event (if any)
    for ($i = 0; $i < count($eventdata); $i++) {
        $event = $events['data'][$i];
        $now = strtotime(date("Y-m-d H:i:s"));
        $start = strtotime($event['start']['date_time']);
        $end = strtotime($event['end']['date_time']);
        if ($start < $now && $now < $end) {
            $start = new DateTime($event['start']['date_time']);
            $end = new DateTime($event['end']['date_time']);
            $now_start = $start->format('g:i A');
            $now_end = $end->format('g:i A');
            $now_tz = $event['end']['time_zone'];
            $now_title = $event['title'];
            if ($now_title == '') {$now_title = 'In use';}
            if (strlen($now_title) > 40) {$now_title = substr ($now_title, 0, 40).'...';}
            break;
        }
    }
    // find next event
    $next_title = '';
    $next_start = '';
    $next_end = '';
    $next_tz = '';
    for ($i = 0; $i < count($eventdata); $i++) {
        $event = $events['data'][$i];
        $now = strtotime(date("Y-m-d H:i:s"));
        $start = strtotime($event['start']['date_time']);
        $end = strtotime($event['end']['date_time']);
        if ($start > $now) {
            $start = new DateTime($event['start']['date_time']);
            $end = new DateTime($event['end']['date_time']);
            $next_start = $start->format('g:i A');
            $next_end = $end->format('g:i A');
            $next_tz = $event['end']['time_zone'];
            $next_title = $event['title'];
            if ($next_title == '') {$next_title = 'Booked for';}
            if (strlen($next_title) > 40) {$next_title = substr ($next_title, 0, 40).'...';}
            break;
        }
    }

    $desktable = 'desks_'.$locationname;
    $deskcheck  = mysqli_query($dbLink,"SELECT ID FROM `$desktable` WHERE `desktype` LIKE 'Meeting' AND `desknumber` LIKE '$roomname'");
    $deskresult = mysqli_num_rows ($deskcheck);
    for ($i = 0; $i < $deskresult; $i++) {
      $deskid    = mysqli_result($deskcheck,0,0);
    }

    if ($isbrowser) {     
        $roomitem=array(
            "map"          => $locationname,
            "name"         => $roomname,
            "availability" => $statedata['availability'],
            "now_title"    => $now_title,
            "now_start"    => $now_start,
            "now_end"      => $now_end,
            "now_tz"       => $now_tz,
            "next_title"   => $next_title,
            "next_start"   => $next_start,
            "next_end"     => $next_end,
            "next_tz"      => $next_tz,
            "deskid"       => $deskid,
        );
        array_push($meeting_arr["rooms"], $roomitem);
    }
    else {
        $availability = $statedata['availability'];
        $checkupdate = mysqli_query($dbLink, "SELECT `ID` FROM `$dbTable` WHERE `room` LIKE '$roomname' AND `map` = '$locationname';");
        $num = mysqli_num_rows ($checkupdate);
        if ($num != 0) {
            // Update existing DB entry
            $tableid = mysqli_result($checkupdate,0,0);
            mysqli_query($dbLink, "UPDATE `$dbName`.`$dbTable` 
            SET `map` = '$locationname', `room` = '$roomname', `availability` = '$availability', 
                `now_title` = '$now_title', `now_start` = '$now_start', `now_end` = '$now_end' , `now_tz` = '$now_tz',
                `next_title` = '$next_title', `next_start` = '$next_start', `next_end` = '$next_end', `next_tz` = '$next_tz', `deskid` = '$deskid'
            WHERE `$dbTable`.`ID` = $tableid;");
            echo "[DONE] $locationname: $roomname updated \n";
        }
        else 
        {
            // Create new entry
            $createsql = mysqli_query($dbLink, "INSERT INTO `$dbName`.`$dbTable` 
                (`ID`, `map`, `room`, `availability`, `now_title`, `now_start`, `now_end`, `now_tz`, `next_title`, `next_start`, `next_end`, `next_tz`, `deskid`) 
                VALUES (NULL, '$locationname', '$roomname', '$availability', '$now_title', '$now_start', '$now_end', '$now_tz', '$next_title', '$next_start', '$next_end', '$next_tz', '$deskid');");
            echo "[DONE] $locationname: $roomname created \n";
        }
    }
}

if ($map != '') {
    if ($map == 'goeppingen') {
        getRooms(getLocation('goeppingenMain'), 'goeppingen'); 
        getRooms(getLocation('goeppingenAux'), 'goeppingen'); 
    }
    else {
        getRooms(getLocation($map), $map);
    }
}
else {
    getRooms(getLocation('adelaide'), 'adelaide');
    getRooms(getLocation('clearwater'), 'clearwater');
    getRooms(getLocation('goeppingenMain'), 'goeppingen');
    getRooms(getLocation('goeppingenAux'), 'goeppingen');
    getRooms(getLocation('berlin'), 'berlin');
    getRooms(getLocation('bremen'), 'bremen');
    getRooms(getLocation('karlsruhe'), 'karlsruhe');
    getRooms(getLocation('stuttgart'), 'stuttgart');
    getRooms(getLocation('yerevan'), 'yerevan');
    getRooms(getLocation('porto'), 'porto');
    listLocations();
}
if ($isbrowser) {
    ob_start('ob_gzhandler');
    echo json_encode($meeting_arr);
}
else {
    echo '[ALL DONE]'."\n";
}
?>
