<?php
// required headers
header("Access-Control-Allow-Origin: *");
header("Content-Type: application/json; charset=UTF-8");


# Loading shared functions and config file
include '../../shared.php';

if ($_SERVER['REQUEST_METHOD'] == "POST") {
    $token = $_POST["token"];
    $mode = $_POST['mode'];
    $map = $_POST['map'];  
    $id = $_POST['id'];
    $desktype = $_POST['desktype'];
    $x = $_POST['x']; 
    $y = $_POST['y']; 
    $desknumber = $_POST['desknumber']; 
    $employee = $_POST['employee']; 
    $avatar = $_POST['avatar']; 
    $department = $_POST['department']; 
    $user = $_POST['user']; 
    $itemscale = $_POST['itemscale']; 
    $published = $_POST['published']; 
    $mapflag = $_POST['mapflag']; 
    $timezone = $_POST['timezone']; 
    $address = $_POST['address']; 
    $flagsize = $_POST['flagsize']; 

    if (isset($_POST["uploadMapfile"])) { 
        $map = strtolower($map);
        //Get the file information
        $userfile_name = $_FILES['image']['name'];
        $userfile_tmp = $_FILES['image']['tmp_name'];
        $userfile_size = $_FILES['image']['size'];
        $userfile_type = $_FILES['image']['type'];
        $filename = basename($_FILES['image']['name']);
        $file_ext = strtolower(substr($filename, strrpos($filename, '.') + 1));
        $SaveToMapfile = '../../maps/'.$map.'.png';

        switch($file_ext) {
        case "gif":
            $converted = imagecreatefromgif($userfile_tmp); 
            imagepng($converted, $SaveToMapfile);
            break;
        case "jpeg":
        case "jpg":
            $converted = imagecreatefromjpeg($userfile_tmp); 
            imagepng($converted, $SaveToMapfile);
            break;
        case "png":
            move_uploaded_file($_FILES['image']['tmp_name'], $SaveToMapfile);   
            break;
        }
    }     
}

else if ($_SERVER['REQUEST_METHOD'] == "GET") {
    $token = $_GET['token'];
    $mode = $_GET['mode'];
    $map = $_GET['map'];  
    $id = $_GET['id'];
    $desktype = $_GET['desktype'];
    $x = $_GET['x']; 
    $y = $_GET['y']; 
    $desknumber = $_GET['desknumber']; 
    $employee = $_GET['employee']; 
    $avatar = $_GET['avatar']; 
    $department = $_GET['department']; 
    $user = $_GET['user']; 
    $itemscale = $_GET['itemscale']; 
    $published = $_GET['published']; 
    $mapflag = $_GET['mapflag']; 
    $timezone = $_GET['timezone']; 
    $address = $_GET['address']; 
    $flagsize = $_GET['flagsize']; 
}

$checktoken = strrev(date("Ymd")) + date("Ymd");

if ($token == $checktoken) {
    // Initialize DB
    $dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
    mysqli_set_charset($dbLink, "utf8");
    // Initialize array for JSON output
    $update_arr=array();
    $update_arr["update"]=array();
    if ($department == '- none -' || $department == 'NULL') {$department = '';}

    switch ($mode) {
        case "create":
            if ($map != '' && $x != '' && $y != '' && $desknumber != '' && ($employee != '' || $desktype == 'localdesk')) {
                $dbTable = 'desks_'.$map;
                // Create database entry
                $createsql = mysqli_query($dbLink, "INSERT INTO `$dbName`.`$dbTable` (`ID`, `desktype`, `x`, `y`, `desknumber`, `employee`, `avatar`, `department`) 
                VALUES (NULL, '$desktype', '$x', '$y', '$desknumber', '$employee', '$avatar', '$department');");
                // Check if database entry exists
                $details = mysqli_query($dbLink, "SELECT * FROM `$dbTable` WHERE `x` = '$x' AND `y` = '$y' AND `desknumber` = '$desknumber'");
                $num   = mysqli_num_rows ($details);  

                $returnid   = mysqli_result($details,0,0);
                $returndsk  = mysqli_result($details,0,3);
                $returnempl = mysqli_result($details,0,4);
                auditlog("Desks",$user,"ID ".$returnid." created: Dsk=".$returndsk." Empl=".$returnempl);
            }
            else {
                $d = " / ";
                auditlog("Desks","System","Missing parameters: ".$map.$d.$mode.$d.$x.$d.$y.$d.$desknumber);
                throw new Exception('Parameters missing');
            }
            break;
            

        case "createmap":
        if ($map != "" && $itemscale != "" && $published != "" && $mapflag != "" && $flagsize != "" && $timezone != "" && $address != "" && $x != "" && $y != "") {
                $dbTable = 'config_maplist';
                $rootdir = str_replace("/rest/update", "", __DIR__);
                // Check if mapname is already taken
                $CheckRows = mysqli_query($dbLink, "SELECT * FROM $dbTable WHERE `mapname`= '$newMapName'");
                $CheckRowsNum = mysqli_num_rows ($CheckRows);
                if ($CheckRowsNum != 0) {
                    throw new Exception('Mapname already in use');
                } 
                else {
                    $mapfile = $rootdir.'/maps/'.$map.'.png';
                    if (file_exists($mapfile)) {
                        // new map database gets created
                        $mapdatabase = 'desks_'.$map;
                        mysqli_query($dbLink, "CREATE TABLE `$dbName`.`$mapdatabase` 
                        ( `ID` INT NOT NULL AUTO_INCREMENT , `desktype` TEXT NOT NULL , `x` INT NOT NULL , `y` INT NOT NULL , `desknumber` TEXT NOT NULL , `employee` TEXT NOT NULL , 
                        `avatar` TEXT NOT NULL , `department` TEXT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB;");
                        // map is registered to the maplist database
                        mysqli_query($dbLink, 
                        "INSERT INTO `config_maplist`(`ID`, `mapname`, `itemscale`, `published`, `country`, `flagsize`, `timezone`, `address`, `mapX`, `mapY`) 
                                              VALUES (NULL,'$map','$itemscale','$published','$mapflag','$flagsize','$timezone','$address','$x','$y');");

                        auditlog("Maps",$_SESSION['username'],"Map has been created (".$map.", ".$mapdatabase.", ".$mapfile.")");


                        header('Location: ../../index.php?map=overview');
                        exit();
                    }
                    else {
                        $returnid   = 'mapfile_required: '.$rootdir.'/maps/'.$map.'.png';
                        $returndsk  = ''.$message;
                        $returnempl=array(
                            "map"     => $map,
                            "itemscale"    => $itemscale,
                            "published"   => $published,
                            "mapflag"   => $mapflag,
                            "flagsize"   => $flagsize,
                            "timezone"   => $timezone,
                            "address"   => $address,
                            "x"   => $x,
                            "y"   => $y,
                        );
                    }
                }
                
                auditlog("Desks",$user,"ID ".$returnid." created: Dsk=".$returndsk." Empl=".$returnempl);
            }
            else {
                $d = " / ";
                auditlog("Desks","System","Missing parameters: ".$map.$d.$mode.$d.$x.$d.$y.$d.$itemscale.$d.$published.$d.mapflag.$d.$flagsize.$d.$timezone.$d.$address);
                throw new Exception('Parameters missing');
            }
            break;
        
        case "update":
            if ($map != '' && $id != '' && $x != '' && $y != '' && $desknumber != '' && ($employee != '' || $desktype == 'localdesk')) {
                $dbTable = 'desks_'.$map;
                // Update database entry
                
                $updatesql = mysqli_query($dbLink, "UPDATE `$dbName`.`$dbTable` 
                SET `desktype` = '$desktype', `x` = '$x', `y` = '$y', `desknumber` = '$desknumber', `employee` = '$employee', `avatar` = '$avatar', `department` = '$department' 
                WHERE `$dbTable`.`ID` = $id;");
                // Check if database entry exists
                $details = mysqli_query($dbLink, "SELECT * FROM `$dbTable` WHERE `x` = '$x' AND `y` = '$y' AND `desknumber` = '$desknumber'");
                $num   = mysqli_num_rows ($details);  

                $returnid   = mysqli_result($details,0,0);
                $returndsk  = mysqli_result($details,0,3);
                $returnempl = mysqli_result($details,0,4);
                
                auditlog("Desks",$user,"ID ".$returnid." updated: Dsk=".$returndsk." Empl=".$returnempl);
            }
            else {
                throw new Exception('Parameters missing');
            }
            break;

        case "delete":
            if ($map != '' && $id != '') {
                $dbTable = 'desks_'.$map;
                // Delete database entry
                mysqli_query($dbLink, "DELETE FROM $dbTable WHERE ID = '$id'"); 

                $returnid = $id; 
                $returndsk = 'deleted';
                $returnempl = 'deleted';
                auditlog("Desks",$user,"ID ".$returnid." deleted");
            }
            else {
                throw new Exception('Parameters missing');
            }
            break;
    }

    // Output values
    $update_item=array(
        "status"     => $returnid,
        "info"    => $returndsk,
        "data"   => $returnempl,
    );
    array_push($update_arr["update"], $update_item);

    // Send output to client
    ob_start('ob_gzhandler');
    echo json_encode($update_arr);
}

else {
    auditlog("Desks",$user,"Update try without authorizing, Token:".$token);
    throw new Exception('Not authorized');
}
?>