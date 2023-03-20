<?php

# CompanyMaps 8.0 Stats API
# Release date 2022-11-14
# Copyright (c) 2016-2022 by MavoDev
# see https://www.mavodev.de for more details

# required headers
header("Access-Control-Allow-Origin: *");
header("Content-Type: application/json; charset=UTF-8");

# Loading shared functions and config file
include '../../shared.php';

# Check for parameters
if ($_SERVER['REQUEST_METHOD'] == "GET") {
  if (isset($_GET['interval'])) {$interval = htmlspecialchars($_GET['interval'], ENT_QUOTES);} else {$interval='';}# year / month / day
  if (isset($_GET['limit'])) {$limit = htmlspecialchars($_GET['limit'], ENT_QUOTES);} else {$limit='';}# limit number of returned values
}

# Get current day
$year = date("Y"); $month = date("m"); $day = date("d"); $date = date("Y-m-d");

function schaltjahr($checkyear){
if(($checkyear % 400) == 0 || (($checkyear % 4) == 0 && ($checkyear % 100) != 0))
  return TRUE;
else
  return FALSE;
}

# Connect to database
$dbTable = 'stats';
$dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
mysqli_set_charset($dbLink, "utf8");

# Read stats
if ($interval != '') {

  # Return a maximum of 48 values if not specified
  if ($limit == '') {
    $limit = 48;
  }

  $details = mysqli_query($dbLink, "SELECT * FROM $dbTable ORDER BY `date` ASC LIMIT 1;");
  $num   = mysqli_num_rows ($details);
  if ($num != 0) {
    $firstyear = mysqli_result($details,0,2);
    $firstmonth = mysqli_result($details,0,3);
    $firstday = mysqli_result($details,0,4);
  }
  else {
    echo json_encode(array($date,0));
    exit;
  }

  switch ($interval) {

    case "year":
      $return_years=array();
      $y = $year;
      for ($l = $limit; $l > 0; $l--) {

        # stop if first entry has been reached
        if ($y < $firstyear) {
          break;
        }

        # Count one year
        $details   = mysqli_query($dbLink, "SELECT * FROM $dbTable WHERE `year`='$y';");
        $num   = mysqli_num_rows ($details);
        $yearcount = 0;
        if ($num != 0) {
          for ($i = 0; $i < $num; $i++) {
            $count = mysqli_result($details,$i,5);
            $yearcount = $yearcount + $count;
          }
        }
        # add number of current year
        $stats_item=array(
          "period"     => "$y",
          "count"    => "$yearcount"
        );
        array_push($return_years, $stats_item);

        # go back one year
        $y--;
      }
      # return all year items
      echo json_encode($return_years);
      break;
  
    case "month":
      $return_months=array();
      $y = $year;
      $m = $month;
      
      for ($l = $limit; $l > 0; $l--) {

        # month in two digit numbers
        $formatm = sprintf("%02d", $m);

        # stop if first entry has been reached
        if (($y == $firstyear && $formatm < $firstmonth)||($y < $firstyear)) {
          break;
        }

        # gather all visitors of one month
        $details = mysqli_query($dbLink, "SELECT * FROM $dbTable WHERE `year`='$y' AND `month`='$formatm';");
        $num     = mysqli_num_rows ($details);
        $monthcount = 0;
        if ($num != 0) {
          for ($i = 0; $i < $num; $i++) {
            $count = mysqli_result($details,$i,5);
            $monthcount = $monthcount + $count;
          }
        }

        # add number of current month
        $stats_item=array(
          "period"     => $y.'-'.$formatm,
          "count"    => "$monthcount"
        );
        array_push($return_months, $stats_item);

        # jump to next year if required
        $m--;
        if ($m == 0) {
          $y--;
          $m = 12;
        }
      }
      echo json_encode($return_months);
      break;

    case "day":
      $return_days=array();
      $y = $year;
      $m = $month;
      $d = $day;
      
      for ($l = $limit; $l > 0; $l--) {

        # month in two digit numbers
        $formatm = sprintf("%02d", $m);
        $formatd = sprintf("%02d", $d);

        # stop if first entry has been reached
        if (($y == $firstyear && $formatm == $firstmonth && $formatd < $firstday)||($y < $firstyear && $m < $firstmonth)) {
          break;
        }

        # gather all visitors of one month
        $details = mysqli_query($dbLink, "SELECT * FROM $dbTable WHERE `year`='$y' AND `month`='$formatm' AND `day`='$formatd';");
        $num     = mysqli_num_rows ($details);
        $daycount = 0;
        if ($num != 0) {
          for ($i = 0; $i < $num; $i++) {
            $count = mysqli_result($details,$i,5);
            $daycount = $daycount + $count;
          }
        }
        
        # add number of current day
        $stats_item=array(
          "period"     => $y.'-'.$formatm.'-'.$formatd,
          "count"    => "$daycount"
        );
        array_push($return_days, $stats_item);

        # jump to next year if required
        $d--;
        if ($d == 0) {
          $m--;
          if ($m == 0) {
            $y--;
            $m = 12;
          }
          switch($m) {
            case "2":
              if (schaltjahr($y)) {$d = 29;} else {$d=28;}
              break;
            case '4':
            case '6':
            case '9':
            case '11':
              $d = 30;
              break;
            default: 
              $d = 31;
              break;
          }
        }
      }
      ob_start('ob_gzhandler');
      echo json_encode($return_days);
      break;

    default:
      echo json_encode(array("stats" => "missing or invalid interval requested"));
      break;
  }
}
# Write stats
else {
  $details = mysqli_query($dbLink, "SELECT * FROM $dbTable WHERE `date`='$date';");
  $num     = mysqli_num_rows ($details); 

  if ($num == 0) {
    mysqli_query($dbLink, "INSERT INTO `$dbTable`(`ID`,`date`,`year`,`month`,`day`,`count`) VALUES (NULL,'$date','$year','$month','$day','1');");
    # Return status
    echo json_encode(array("stats added" => "$date ok"));
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
      $status = 'ok';
    } 
    else {
      $status = 'error';
    }
    # Return status
    echo json_encode(array("stats added" => "$date $status"));
  }
}
?>