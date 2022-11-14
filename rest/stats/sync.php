<?php
# CompanyMaps 8.0 Stats Sync
# Release date 2022-11-14
# Copyright (c) 2016-2020 by MavoDev
# see https://www.mavodev.de for more details

# Aggregates the old tracking objects into the stats db

# required headers
header("Access-Control-Allow-Origin: *");
header("Content-Type: application/json; charset=UTF-8");

# Loading shared functions and config file
include '../../shared.php';

# Connect to database
$trackingTable = 'tracking_detailed';
$dbTable = 'stats';
$dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
mysqli_set_charset($dbLink, "utf8");

# Drop stats table first
mysqli_query($dbLink, "DELETE FROM `$dbTable`;");

# Sync stats
$tracks   = mysqli_query($dbLink, "SELECT * FROM $trackingTable;");
$numtrack = mysqli_num_rows ($tracks); 

for ($i = 0; $i < $numtrack; $i++) {
  $year  = mysqli_result($tracks,$i,1);
  $month = sprintf("%02d", mysqli_result($tracks,$i,2));
  $day   = sprintf("%02d", mysqli_result($tracks,$i,3));
  $date = $year.'-'.$month.'-'.$day;
  echo "($i / $numtrack) $date";

  # Import into new stats db
  $details = mysqli_query($dbLink, "SELECT * FROM $dbTable WHERE `date`='$date';");
  $num     = mysqli_num_rows ($details); 

  if ($num == 0) {
    mysqli_query($dbLink, "INSERT INTO `$dbTable`(`ID`,`date`,`year`,`month`,`day`,`count`) VALUES (NULL,'$date','$year','$month','$day','1');");
    # Return status
    echo " [ done ] \n";
  }
  else {
    $count = mysqli_result($details,0,5);
    $count++;
    $updatesql = mysqli_query($dbLink, "UPDATE `$dbTable` SET `count` = '$count' WHERE `stats`.`date` = '$date';");

    # check if value was updated
    $details  = mysqli_query($dbLink, "SELECT * FROM $dbTable WHERE `date`='$date';");
    $newcount = mysqli_result($details,0,5);

    # define return string
    if ($count = $newcount) {
      $status = 'done';
    } 
    else {
      $status = 'error';
    }
    # Return status
    echo " [ $status ] \n";
  }
}


?>