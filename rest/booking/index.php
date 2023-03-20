<?php
session_start(); 
// required headers
header("Access-Control-Allow-Origin: *");
header("Content-Type: application/json; charset=UTF-8");

# CompanyMaps 8.1 Booking API
# Release date 2023-03-20
# Copyright (c) 2016-2022 by MavoDev
# see https://www.mavodev.de for more details

$username = '';
$fullname = '';
$telephonenumber = '';
$mail = '';

# Loading shared functions and config file
include '../../shared.php';

# set variables
if (isset($_SESSION['usershort'])) {
    $bookuser = $_SESSION['usershort'];
} 
else {
    $bookuser = '';
}
if (isset($_SESSION['fullname'])) {
    $bookfullname = $_SESSION['fullname'];
    if ($bookfullname == '') {
        $bookfullname = $bookuser;
    }
} 
else {
    $bookfullname = '';
}
if (isset($_SESSION['telephonenumber'])) {$bookphone = $_SESSION['telephonenumber'];} else {$bookphone = '';};
if (isset($_SESSION['mail'])) {$bookmail = $_SESSION['mail'];} else {$bookmail = '';};
$currentdate = date('Y-m-d');
$dbTable = 'bookings';
$mapTable = 'config_maplist';
$data = array();
$debug = array();
$mode = '';
$bookdate = '';
$bookmap = '';
$bookdesk = '';

# connect to database 
$dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
mysqli_set_charset($dbLink, "utf8");

# make sure table exists
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`$dbTable` 
( `ID` BIGINT NOT NULL AUTO_INCREMENT , `date` TEXT NOT NULL , `map` TEXT NOT NULL , `desk` TEXT NOT NULL , 
`user` TEXT NOT NULL , `fullname` TEXT NOT NULL , `phone` TEXT NOT NULL , `mail` TEXT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB;");

# assign provided parameters
if ($_SERVER['REQUEST_METHOD'] == "GET") {
    if (isset($_GET['mode'])) {$mode = htmlspecialchars($_GET['mode'], ENT_QUOTES); } 
    if (isset($_GET['bookdate'])) {$bookdate = htmlspecialchars($_GET['bookdate'], ENT_QUOTES); } 
    if (isset($_GET['bookmap'])) {$bookmap = htmlspecialchars($_GET['bookmap'], ENT_QUOTES); } 
    if (isset($_GET['bookdesk'])) {$bookdesk = htmlspecialchars($_GET['bookdesk'], ENT_QUOTES); } 
}

## cleanup database
$maps     = mysqli_query($dbLink, "SELECT `mapname`,`timezone` FROM `$dbName`.`$mapTable`;");
$mapcount = mysqli_num_rows ($maps);
for ($i=0; $i<$mapcount; $i++) {
    $mapname   = mysqli_result($maps,$i,0);
    $timezone  = mysqli_result($maps,$i,1);
    $mapdate = new DateTime();
    $mapdate->setTimezone(new DateTimeZone($timezone));
    $mapdatestring = $mapdate->format('Y-m-d');
    if ($mode == 'book' && $bookmap == $mapname) {
        $currentdate = $mapdatestring;
    }
    mysqli_query($dbLink, "DELETE FROM `$dbName`.`$dbTable` WHERE `map`='$mapname' AND `date` < '$mapdatestring';");
}

switch ($mode) {
    # book a new desk
    case 'book':
        if ($bookuser != '' && $bookdate != '' && $bookmap != '' && $bookdesk != '') {
        
            # check if requested desk is available
            $checkavailability = mysqli_query($dbLink, "SELECT * FROM `$dbTable` WHERE `date`='$bookdate' AND `map`='$bookmap' AND `desk`='$bookdesk';");
            $num   = mysqli_num_rows ($checkavailability);
            if ($num > 0) {
                $status  = "error";
                $message = "Already booked.";
                break;
            }
            
            # check if date is not from the past
            if ($bookdate >= $currentdate) {
                # add new booking
                mysqli_query($dbLink, "INSERT INTO `$dbTable` (`ID`, `date`, `map`, `desk`, `user`, `fullname`, `phone`, `mail`) 
                VALUES (NULL, '$bookdate', '$bookmap', '$bookdesk', '$bookuser', '$bookfullname', '$bookphone', '$bookmail');;");
                # count total bookings
                $details = mysqli_query($dbLink, "SELECT * FROM $dbTable WHERE `user` = '$bookuser'");
                $bookcount   = mysqli_num_rows ($details);
                # return string
                $status  = "ok";
                $message = "$bookdesk booked";
            }
            else {
                $status  = "error";
                $message = "Date in the past";
            }
        }
        else {
            $status  = "error";
            $message = "missing data";
            $debug['bookuser']=$bookuser;
            $debug['bookdate']=$bookdate;
            $debug['bookmap']=$bookmap;
            $debug['bookdesk']=$bookdesk;
        }
        break;
    # cancel existing booking
    case 'remove':
        if ($bookuser != '' && $bookdate != '' && $bookmap != '' && $bookdesk != '') {
            # delete booking from database
            mysqli_query($dbLink, "DELETE FROM `$dbTable` WHERE `user`='$bookuser' AND `date`='$bookdate' AND `map`='$bookmap' AND `desk`='$bookdesk';");
            # count total bookings
            $details = mysqli_query($dbLink, "SELECT * FROM $dbTable WHERE `user` = '$bookuser'");
            $bookcount   = mysqli_num_rows ($details);
            # return string
            $status  = "ok";
            $message = "Booking cancelled: $bookdate $bookmap $bookdesk $bookuser - $bookcount bookings in total";     
        }
        else {
            $status  = "error";
            $message = "not logged in or no desk provided";
        }
        break;
    # list existing bookings
    case 'list':
        #if ($bookuser != '' && $bookmap != '') {
        #    $details = mysqli_query($dbLink, "SELECT * FROM $dbTable WHERE `user`='$bookuser' AND `map`='$bookmap' ORDER BY `date` ASC;");
        #}
        if ($bookmap != '') {
            $details = mysqli_query($dbLink, "SELECT * FROM $dbTable WHERE `map`='$bookmap' ORDER BY `date` ASC;");
        }
        else if ($bookuser != '') {
            $details = mysqli_query($dbLink, "SELECT * FROM $dbTable WHERE `user`='$bookuser' ORDER BY `date` ASC;");
        }
        else {
            $details = mysqli_query($dbLink, "SELECT * FROM $dbTable ORDER BY `date` ASC");
            $message = "no user or map provided";
        }
        $num   = mysqli_num_rows ($details);
        for ($i=0; $i<$num; $i++) {
            $ID    = mysqli_result($details,$i,0);
            $date  = mysqli_result($details,$i,1);
            $map   = mysqli_result($details,$i,2);
            $desk  = mysqli_result($details,$i,3);
            $user  = mysqli_result($details,$i,4);
            $name  = mysqli_result($details,$i,5);
            $phone = mysqli_result($details,$i,6);
            $mail  = mysqli_result($details,$i,7);
            $booking = array ('date'=>$date, 'map'=>$map, 'desk'=>$desk, 'user'=>$user, 'name'=>$name, 'phone'=>$phone, 'mail'=>$mail);
            $data[$i]=$booking;
        }
        $status  = "ok";
        $message = "$i bookings found";
        break;
    default:
        $status  = "error";
        $message = 'no mode selected';
        break;
}

$output = array();
$output['status'] = $status;
$output['message'] = $message;
$output['date'] = $currentdate;
$output['data'] = $data;
$output['debug'] = $debug;
ob_start('ob_gzhandler');
echo json_encode($output);
?>