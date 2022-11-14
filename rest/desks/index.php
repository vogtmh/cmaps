<?php

# CompanyMaps 8.0 Desks API
# Release date 2022-11-14
# Copyright (c) 2016-2022 by MavoDev
# see https://www.mavodev.de for more details

// required headers
header("Access-Control-Allow-Origin: *");
header("Content-Type: application/json; charset=UTF-8");

# Loading shared functions and config file
include '../../shared.php';

$map = '';
$search = '';
$ldapdb_arr = array();
$bookingsdb_arr = array();
$bookdata = array();
$currentdate = date('Y-m-d'); 
$userdate = ''; 

if ($_SERVER['REQUEST_METHOD'] == "GET") {
    if(isset($_GET['map'])) {$map = $_GET['map'];}
    if(isset($_GET['search'])) {$search = $_GET['search'];}
    if(isset($_GET['date'])) {$userdate = $_GET['date'];}
    }

$dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
mysqli_set_charset($dbLink, "utf8");
$bookingsTable = 'bookings';


// Initialize array for JSON output
$desks_arr=array();
$desks_arr["desks"]=array();

# Gather vip parsed text first, so it does not need to be collected multiple times
$config_vips = mysqli_query($dbLink, "SELECT `Parsed Text in Job Title`, `Type` FROM `config_vips`;");
$num_vips   = mysqli_num_rows ($config_vips);

if ($map != '') {
    
    # Go to Map Table 
    $mapname = $map;
    $dbTable = 'desks_'.$map;
    $details = mysqli_query($dbLink, "SELECT * FROM `$dbTable`");
    $num   = mysqli_num_rows ($details);  

    # get timezone
    $mapTable = 'config_maplist';
    $maps     = mysqli_query($dbLink, "SELECT `mapname`,`timezone` FROM `$dbName`.`$mapTable` WHERE `mapname`='$mapname';");
    $mapcount = mysqli_num_rows ($maps);
    if ($mapcount == 1) {
        $mapname   = mysqli_result($maps,0,0);
        $timezone  = mysqli_result($maps,0,1);
        $mapdate = new DateTime();
        $mapdate->setTimezone(new DateTimeZone($timezone));
        $mapdatestring = $mapdate->format('Y-m-d');
        if ($userdate != '') {
            $currentdate = $userdate;
        } 
        else {
            $currentdate = $mapdatestring;
        }
    }

    for ($i = 0; $i < $num; $i++) {
        $id           = mysqli_result($details,$i,0);
        $desktype     = mysqli_result($details,$i,1);
        $x            = mysqli_result($details,$i,2);
        $y            = mysqli_result($details,$i,3);
        $desknumber   = mysqli_result($details,$i,4);
        $employee     = mysqli_result($details,$i,5);
        $avatar       = mysqli_result($details,$i,6);
        $department   = mysqli_result($details,$i,7);
        $givenname    = '';
        $surname      = '';
        $phone        = '';
        $mail         = '';
        $title        = '';
        $mobile       = '';
        $circle_color = '';
        $parsed       = '';
        $fullname     = '';
        $booked       = 0;
        $bookingsmatch=array();
        $bookingcount = 0;
        $bookdata     = array();

        # check if desk is booked
        if (!$bookingsdb_arr) {
            $bookingsdb      = mysqli_query($dbLink, "SELECT * FROM `$bookingsTable`");
            $bookingsdb_arr = mysqli_fetch_all($bookingsdb, MYSQLI_ASSOC);
        }
        # array_filter for faster speeds
        $bookingsmatch = array_filter($bookingsdb_arr, function ($booking) use ($mapname, $desknumber, $currentdate) {
            return ($booking['date'] == $currentdate && $booking['map'] == $mapname && $booking['desk'] == $desknumber);
        });
        $bookingcount  = count($bookingsmatch);

        if ($bookingcount > 0) {
            $booked = '1';
            foreach($bookingsmatch as $bookingentry) {
                $bookdata = array("name"=>$bookingentry['fullname'], "phone"=>$bookingentry['phone'], "mail"=>$bookingentry['mail']);
            }
        }
        else {
            $booked = '0';
            $bookdata = array();
        }

        // Lookup additional data from LDAP mirror database
        if ($desktype == 'addesk') {
            if (!$ldapdb_arr) {
                $ldapTable = 'ldap-mirror';
                $ldapdb      = mysqli_query($dbLink, "SELECT * FROM `$ldapTable`");

                $ldapdb_arr = mysqli_fetch_all($ldapdb);
            }
                
            # array_filter for faster speeds
            $ldapmatch = array_filter($ldapdb_arr, function ($var) use ($desknumber) {
                return ($var['5'] == $desknumber);
            });
            $numldapdb  = count($ldapmatch);

            if ($numldapdb >0) {
                $l = 0;
                foreach($ldapmatch as $ldapuser) {

                    # allow a maximum of 4 people per desk
                    if( $l > 3 ) break;
                    $givenname = $ldapuser['1'];
                    $surname   = $ldapuser['2'];
                    $phone     = $ldapuser['3'];
                    $mail      = $ldapuser['4'];
                    $avatar    = $ldapuser['6'];
                    $title     = $ldapuser['7'];
                    $mobile    = $ldapuser['9'];;
                    $fullname  = $givenname.' '.$surname;

                    # Check if it's a shared desk
                    if ($numldapdb > 1) {$desktype='shareddesk';} 
                    
                    # check if a colored border has to be applied
                    for ($v = 0; $v < $num_vips; $v++) {
                        $parsedtext = mysqli_result($config_vips,$v,0);
                        $vip_type = mysqli_result($config_vips,$v,1);
                        
                        switch ($vip_type) {
                
                        case "TeamManager":
                            if (stripos($title, $parsedtext) !== false) {$circle_color='#00CC00';}  // Team Managers are green
                            break;

                        case "Director":
                            if (stripos($title, $parsedtext) !== false) {$circle_color='#00bbff';$parsed = $parsedtext.' / '.$title;} // Directors have blue
                            break;
                        
                        case "VP":
                            if (stripos($title, $parsedtext) !== false) {$circle_color='#800080';} // VPs show up in purple
                            break;

                        case "Board":
                            if ($title == $parsedtext)  {$circle_color='#ffa500';} // really important persons are orange
                            break;
                        }
                    }

                    // Output values
                    $desk_item=array(
                        "map"         => $mapname,
                        "id"          => $id,
                        "desktype"    => $desktype,
                        "x"           => $x,
                        "y"           => $y,
                        "dsk"         => $desknumber,
                        "empl"        => $employee,
                        "avtr"        => $avatar,
                        "dept"        => $department,
                        "fname"       => $givenname,
                        "lname"       => $surname,
                        "phone"       => $phone,
                        "mail"        => $mail,
                        "title"       => $title,
                        "mobil"       => $mobile,
                        "color"       => $circle_color,
                        "parsed"      => $parsed,
                        "booked"      => $booked,
                    );
                    // output search results only if specified
                    if ($search != '') {
                        $searcharr = explode('|', $search);
                        for ($t = 0; $t < count($searcharr); $t++) {
                            $combined = $desktype.','.$desknumber.','.$employee.','.$fullname;
                            if (stripos($combined, $searcharr[$t]) !== false) {
                                array_push($desks_arr["desks"], $desk_item);
                            }
                        }
                    }
                    // output all results if no search parameter given
                    else {
                        array_push($desks_arr["desks"], $desk_item);
                    }
                    
                    $l++;
                    if ($l > 3) break;
                }
            }
            // No matches found in LDAP Mirror - output empty desk
            if ($numldapdb == 0) {
                // Output values
                $desk_item=array(
                    "map"      => $mapname,
                    "id"       => $id,
                    "desktype" => $desktype,
                    "x"        => $x,
                    "y"        => $y,
                    "dsk"      => $desknumber,
                    "empl"     => $employee,
                    "avtr"     => $avatar,
                    "dept"     => $department,
                    "fname"    => $givenname,
                    "lname"    => $surname,
                    "phone"    => $phone,
                    "mail"     => $mail,
                    "title"    => $title,
                    "mobil"    => $mobile,
                    "color"    => $circle_color,
                    "parsed"   => $parsed,
                    "booked"   => $booked,
                );
                // output search results only if specified
                if ($search != '') {
                    $searcharr = explode('|', $search);
                    for ($t = 0; $t < count($searcharr); $t++) {
                        $combined = $desktype.','.$desknumber.','.$employee.','.$fullname;
                        if (stripos($combined, $searcharr[$t]) !== false) {
                            array_push($desks_arr["desks"], $desk_item);
                        }
                    }
                }
                // output all results if no search parameter given
                else {
                    array_push($desks_arr["desks"], $desk_item);
                }
            }
        }
        else 
        {
            // Output values
            $desk_item=array(
                "map"      => $mapname,
                "id"       => $id,
                "desktype" => $desktype,
                "x"        => $x,
                "y"        => $y,
                "dsk"      => $desknumber,
                "empl"     => $employee,
                "avtr"     => $avatar,
                "dept"     => $department,
                "fname"    => $givenname,
                "lname"    => $surname,
                "phone"    => $phone,
                "mail"     => $mail,
                "title"    => $title,
                "mobil"    => $mobile,
                "color"    => $circle_color,
                "parsed"   => $parsed,
                "booked"   => $booked,
                "bookdata" => $bookdata
            );
            // output search results only if specified
            if ($search != '') {
                $searcharr = explode('|', $search);
                for ($t = 0; $t < count($searcharr); $t++) {
                    $combined = $desktype.','.$desknumber.','.$employee.','.$fullname;
                    if ($booked == '1') {$combined .= ','.$bookdata['name'];}
                    if (stripos($combined, $searcharr[$t]) !== false) {
                        array_push($desks_arr["desks"], $desk_item);
                    }
                }
            }
            // output all results if no search parameter given
            else {
                array_push($desks_arr["desks"], $desk_item);
            }
        }
    }
}
else {

    # Get all map tables and query all of them
    $dbTable = 'config_maplist';
    $mapsdb      = mysqli_query($dbLink, "SELECT * FROM `$dbTable` ORDER BY `mapname` ASC");
    $nummapsdb   = mysqli_num_rows ($mapsdb);  
    for ($n = 0; $n < $nummapsdb; $n++) {
        $mapname = mysqli_result($mapsdb,$n,1);
        $published = mysqli_result($mapsdb,$n,3);
        if ($published == 'no') {continue;}
        // Query every map table
        if ($mapname != 'overview') {
          $desksTable = 'desks_'.$mapname;
          $details = mysqli_query($dbLink, "SELECT * FROM `$desksTable`");
          $num   = mysqli_num_rows ($details);
        }
        else {
          $num = 0;
        }  

        # get bookings for each map
        $bookingsdb      = mysqli_query($dbLink, "SELECT * FROM `$bookingsTable`");
        $bookingsdb_arr = mysqli_fetch_all($bookingsdb, MYSQLI_ASSOC);
        
        for ($i = 0; $i < $num; $i++) {
            $id           = mysqli_result($details,$i,0);
            $desktype     = mysqli_result($details,$i,1);
            $x            = mysqli_result($details,$i,2);
            $y            = mysqli_result($details,$i,3);
            $desknumber   = mysqli_result($details,$i,4);
            $employee     = mysqli_result($details,$i,5);
            $avatar       = mysqli_result($details,$i,6);
            $department   = mysqli_result($details,$i,7);
            $givenname    = '';
            $surname      = '';
            $phone        = '';
            $mail         = '';
            $title        = '';
            $mobile       = '';
            $circle_color = '';
            $writeDesktype= '';
            $parsed       = '';
            $fullname     = '';
            $booked       = '0';
            $combined     = '';

            # Get desktype for database (preparation for v4)
            
            switch($desknumber) {
                case "Restroom":
                case "Food":
                case "Service":
                case "Exit":
                case "KeycardLock":
                case "KeyLock":
                case "Floor":
                case "Blocked":
                case "Hotseat":
                case "FirstAid":
                case "Meeting":
                case "Printer":
                  $writeDesktype = strtolower($desknumber);
                  break;
            }

            # array_filter for faster speeds
            $bookingsmatch = array_filter($bookingsdb_arr, function ($booking) use ($mapname, $desknumber, $currentdate) {
                return ($booking['date'] == $currentdate && $booking['map'] == $mapname && $booking['desk'] == $desknumber);
            });
            $bookingcount  = count($bookingsmatch);

            if ($bookingcount > 0) {
                $booked = '1';
                foreach($bookingsmatch as $bookingentry) {
                    $bookdata = array("name"=>$bookingentry['fullname'], "phone"=>$bookingentry['phone'], "mail"=>$bookingentry['mail']);
                }
            }
            else {
                $booked = '0';
                $bookdata = array();
            }

            # Lookup additional data from LDAP mirror database
            if ($desktype == 'addesk') {
                if (!$ldapdb_arr) {
                    $ldapTable = 'ldap-mirror';
                    $ldapdb      = mysqli_query($dbLink, "SELECT * FROM `$ldapTable`");

                    $ldapdb_arr = mysqli_fetch_all($ldapdb);
                }
                    
                # array_filter for faster speeds
                $ldapmatch = array_filter($ldapdb_arr, function ($var) use ($desknumber) {
                    return ($var['5'] == $desknumber);
                });
                $numldapdb  = count($ldapmatch);

                if ($numldapdb >0) {
                    $l = 0;
                    foreach($ldapmatch as $ldapuser) {

                    # allow a maximum of 4 people per desk
                    if( $l > 3 ) break;
                    $givenname = $ldapuser['1'];
                    $surname   = $ldapuser['2'];
                    $phone     = $ldapuser['3'];
                    $mail      = $ldapuser['4'];
                    $avatar    = $ldapuser['6'];
                    $title     = $ldapuser['7'];
                    $mobile    = $ldapuser['9'];;
                    $fullname  = $givenname.' '.$surname;

                    # Check if it's a shared desk
                    if ($numldapdb > 1) {$desktype='shareddesk';} 

                    # check if a colored border has to be applied
                    for ($v = 0; $v < $num_vips; $v++) {
                        $parsedtext = mysqli_result($config_vips,$v,0);
                        $vip_type   = mysqli_result($config_vips,$v,1);
                        
                        switch ($vip_type) {
                
                        case "TeamManager":
                            if (stripos($title, $parsedtext) !== false) {$circle_color='#00CC00';}  // Team Managers are green
                            break;

                        case "Director":
                            if (stripos($title, $parsedtext) !== false) {$circle_color='#00bbff';} // Directors have blue
                            break;
                        
                        case "VP":
                            if (stripos($title, $parsedtext) !== false) {$circle_color='#800080';} // VPs show up in purple
                            break;

                        case "Board":
                            if ($title == $parsedtext)  {$circle_color='#ffa500';} // really important persons are orange
                            break;
                        }
                    }

                    // Output values
                    $desk_item=array(
                        "map"      => $mapname,
                        "id"       => $id,
                        "desktype" => $desktype,
                        "x"        => $x,
                        "y"        => $y,
                        "dsk"      => $desknumber,
                        "empl"     => $employee,
                        "avtr"     => $avatar,
                        "dept"     => $department,
                        "fname"    => $givenname,
                        "lname"    => $surname,
                        "phone"    => $phone,
                        "mail"     => $mail,
                        "title"    => $title,
                        "mobil"    => $mobile,
                        "color"    => $circle_color,
                        "parsed"   => $parsed,
                        "booked"   => $booked,
                    );
                    // output search results only if specified
                    if ($search != '') {
                        $searcharr = explode('|', $search);
                        for ($t = 0; $t < count($searcharr); $t++) {
                            $combined = $desktype.','.$desknumber.','.$employee.','.$fullname;
                            if (stripos($combined, $searcharr[$t]) !== false) {
                                array_push($desks_arr["desks"], $desk_item);
                            }
                        }
                    }
                    // output all results if no search parameter given
                    else {
                        array_push($desks_arr["desks"], $desk_item);
                    }

                    $l++;
                    if ($l > 3) break;
                    }
                }

                // No matches found in LDAP Mirror - output empty desk
                else if ($numldapdb == 0) {
                    // Output values
                    $desk_item=array(
                        "map"      => $mapname,
                        "id"       => $id,
                        "desktype" => $desktype,
                        "x"        => $x,
                        "y"        => $y,
                        "dsk"      => $desknumber,
                        "empl"     => $employee,
                        "avtr"     => $avatar,
                        "dept"     => $department,
                        "fname"    => $givenname,
                        "lname"    => $surname,
                        "phone"    => $phone,
                        "mail"     => $mail,
                        "title"    => $title,
                        "mobil"    => $mobile,
                        "color"    => $circle_color,
                        "parsed"   => $parsed,
                        "booked"   => $booked
                    );
                    // output search results only if specified
                    if ($search != '') {
                        $searcharr = explode('|', $search);
                        for ($t = 0; $t < count($searcharr); $t++) {
                            $combined = $desktype.','.$desknumber.','.$employee.','.$fullname;
                            if (stripos($combined, $searcharr[$t]) !== false) {
                                array_push($desks_arr["desks"], $desk_item);
                            }
                        }
                    }
                    // output all results if no search parameter given
                    else {
                        array_push($desks_arr["desks"], $desk_item);
                    }
                }
                $writeDesktype = 'addesk';
            }
            // Output non-ad desks
            else {

                $desk_item=array(
                    "map"      => $mapname,
                    "id"       => $id,
                    "desktype" => $desktype,
                    "x"        => $x,
                    "y"        => $y,
                    "dsk"      => $desknumber,
                    "empl"     => $employee,
                    "avtr"     => $avatar,
                    "dept"     => $department,
                    "fname"    => $givenname,
                    "lname"    => $surname,
                    "phone"    => $phone,
                    "mail"     => $mail,
                    "title"    => $title,
                    "mobil"    => $mobile,
                    "color"    => $circle_color,
                    "booked"   => $booked,
                    "bookdata" => $bookdata
                );
                // filter for search results only if specified
                if ($search != '') {
                    $searcharr = explode('|', $search);
                    for ($t = 0; $t < count($searcharr); $t++) {
                        $combined .= $desktype.','.$desknumber.','.$employee.','.$fullname;
                        if ($booked == '1') {$combined .= ','.$bookdata['name'];}
                        if (stripos($combined, $searcharr[$t]) !== false) {
                            array_push($desks_arr["desks"], $desk_item);
                        }
                    }
                }
                // output all results if no search parameter given
                else {
                    array_push($desks_arr["desks"], $desk_item);
                }
                $desks_arr["status"] = "ok";
            }

        }
    }
    
}
ob_start('ob_gzhandler');
echo json_encode($desks_arr);
?>