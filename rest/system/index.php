<?php

# CompanyMaps 8.1 System API
# Release date 2023-03-20
# Copyright (c) 2016-2022 by MavoDev
# see https://www.mavodev.de for more details

# required headers
header("Access-Control-Allow-Origin: *");
header("Content-Type: application/json; charset=UTF-8");

# Loading shared functions and config file
include '../../shared.php';

$healthdetails=0;

if ($_SERVER['REQUEST_METHOD'] == "GET") {
  if (isset($_GET['healthdetails'])) {
    $healthdetails = htmlspecialchars($_GET['healthdetails'], ENT_QUOTES);
    $healtharray=array();
    $healtharray["ldap"]=array();
    $healtharray["desks"]=array();
  } 
}

$dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);

$sysload = sys_getloadavg ();
$sysload = number_format($sysload[0],2);

$free = shell_exec ('free');
$free =(string)trim($free);
$free_arr = explode("\n", $free);
$mem = explode (" ", $free_arr[1]);
$mem = array_filter ($mem);
$mem = array_merge ($mem);
$memory_usage = $mem[2]/$mem[1]*100;
$memory_usage = number_format(round($memory_usage, 2) ,2);

$DiskFree = disk_free_space ("/var/www");
$DiskTotal = disk_total_space ("/var/www");
$DiskUsedAbsolute = $DiskTotal - $DiskFree;
$DiskUsed = number_format(round(($DiskUsedAbsolute / $DiskTotal) * 100, 2) ,2);

/* Gets individual core information */
function GetCoreInformation() {
  $data = file('/proc/stat');
  $cores = array();
  foreach( $data as $line ) {
  if( preg_match('/^cpu[0-9]/', $line) )
  {
    $info = explode(' ', $line );
    $cores[] = array(
    'user' => $info[1],
    'nice' => $info[2],
    'sys' => $info[3],
    'idle' => $info[4]
    );
  }
  }
  return $cores;
}
/* compares two information snapshots and returns the cpu percentage */
function GetCpuPercentages($stat1, $stat2) {
  if( count($stat1) !== count($stat2) ) {
  return;
  }
  $allcpuload = 0;
  $cpus = array();
  for( $i = 0, $l = count($stat1); $i < $l; $i++) {
  $dif = array();
  $dif['user'] = $stat2[$i]['user'] - $stat1[$i]['user'];
  $dif['nice'] = $stat2[$i]['nice'] - $stat1[$i]['nice'];
  $dif['sys'] = $stat2[$i]['sys'] - $stat1[$i]['sys'];
  $dif['idle'] = $stat2[$i]['idle'] - $stat1[$i]['idle'];
  $allcpuload += ($dif['idle']*2);
  }
  $avg_cpuload = 100 - ($allcpuload / count($stat1));
  return $avg_cpuload;
}
/* get core information (snapshot) */
$stat1 = GetCoreInformation();
/* sleep on server for 0.3 seconds */
usleep(500000);
/* take second snapshot */
$stat2 = GetCoreInformation();
/* get the cpu percentage based off two snapshots */
$cpuload = GetCpuPercentages($stat1, $stat2);

# CHECK CONSISTENCY

# Consistency check of ldap-mirror (if existent) - check if a desk is occupied by a maximum of 4 persons
$consistencyresults = '';
$ldap_errors = 0;
$whitelist_ldap = mysqli_query($dbLink, "SELECT `text` FROM `health_whitelist` WHERE `type` = 'ldap'");
$numWLldap = mysqli_num_rows ($whitelist_ldap);
$whitelistLdap = array();
for ($w = 0; $w < $numWLldap; $w++) {
  array_push($whitelistLdap,mysqli_result($whitelist_ldap,$w,0));
}

$mirrorcheck  = mysqli_query($dbLink, "SELECT * FROM `$ldapTable`");
$mirrorresult = mysqli_num_rows ($mirrorcheck);
$ldaparray = array();
for ($i = 0; $i < $mirrorresult; $i++) {
  $scandesk    = mysqli_result($mirrorcheck,$i,5);
  if (in_array($scandesk, $whitelistLdap)) {
    $tablescan_results = 1;
  }
  else {
    $tablescan     = mysqli_query($dbLink, "SELECT * FROM `$ldapTable` WHERE `physicaldeliveryofficename` = '$scandesk'");
    $tablescan_results   = mysqli_num_rows ($tablescan);
  }
  if ($tablescan_results > 4) {
    $employee = mysqli_result($mirrorcheck,$i,1).' '.mysqli_result($mirrorcheck,$i,2);
    $ldaperror = array (
      "desk"  => "$scandesk",
      "count" => $tablescan_results,
      "name"  => $employee,
    );
    array_push($ldaparray, $ldaperror);
  }   
}
if (count($ldaparray) > 0) {
  array_multisort($ldaparray);
  $healtharray["ldap"] = $ldaparray;
  $ldap_errors = count($ldaparray); 
}

# Consistency check of desks database - check if all desks have unique names
$desk_errors = 0;

$whitelist_desks = mysqli_query($dbLink, "SELECT `text` FROM `health_whitelist` WHERE `type` = 'desks'");
$numWLdesks = mysqli_num_rows ($whitelist_desks);
$whitelistDesks = array();
for ($w = 0; $w < $numWLdesks; $w++) {
  array_push($whitelistDesks,mysqli_result($whitelist_desks,$w,0));
}

$dbTable = 'config_maplist';
$config_maplist = mysqli_query($dbLink, "SELECT * FROM $dbTable ORDER BY `mapname` ASC;");
$num   = mysqli_num_rows ($config_maplist);  
$deskarray = array();
    
for ($m = 0; $m < $num; $m++) {
  $mapname = mysqli_result($config_maplist,$m,1);
  $deskcheck  = mysqli_query($dbLink,"SELECT * FROM `desks_$mapname`");
  if ($deskcheck != false) {
    $deskresult = mysqli_num_rows ($deskcheck);
    for ($i = 0; $i < $deskresult; $i++) {
      $scandesk    = mysqli_result($deskcheck,$i,4);
      if (in_array($scandesk, $whitelistDesks)) {
        $tablescan_results = 1;
      }
      else {
        $tablescan = mysqli_query($dbLink, "SELECT * FROM `desks_$mapname` WHERE `desknumber` = '$scandesk'");
        $tablescan_results = mysqli_num_rows ($tablescan);
      }

      if ($tablescan_results > 1) {
        $desk_errors = $desk_errors + $tablescan_results; 
        $deskerror = array (
          "desk"  => strval($scandesk),
          "count" => $tablescan_results,
          "map"   => $mapname
        );
        array_push($deskarray, $deskerror);
      }  
    } 
  }
     
}
array_multisort($deskarray);
$healtharray["desks"] = $deskarray;

switch($healthdetails) {
  case 0:
    $system_arr=array(
      "cpuload" => "$cpuload",
      "memoryused" => $memory_usage,
      "diskused" => $DiskUsed,
      "consistency_ldap" => $ldap_errors,
      "consistency_desks" => $desk_errors,

    );
    break;
  
  case 1:
    $system_arr=array(
      "cpuload" => "$cpuload",
      "memoryused" => $memory_usage,
      "diskused" => $DiskUsed,
      "consistency_ldap" => $ldap_errors,
      "consistency_desks" => $desk_errors,
      "health" => $healtharray,
      "ignoredLdap" => $whitelistLdap,
      "ignoredDesks" => $whitelistDesks
    );
    break;
  
  default:
    break;
}

echo json_encode($system_arr);
?>