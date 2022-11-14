<?php
// required headers
header("Access-Control-Allow-Origin: *");
header("Content-Type: application/json; charset=UTF-8");

# Loading shared functions and config file
include '../../shared.php';

if ($_SERVER['REQUEST_METHOD'] == "GET") {
  $getmap = $_GET['map'];  
}

$dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
mysqli_set_charset($dbLink, "utf8");

function hexToStr($hex){
  $string='';
  for ($i=0; $i < strlen($hex)-1; $i+=2){
      $string .= chr(hexdec($hex[$i].$hex[$i+1]));
  }
  return $string;
}

// Check if the script is run in the browser or the CLI
if (php_sapi_name() == "cli") {
  error_reporting(E_ERROR | E_WARNING | E_PARSE);
  $mapList = 'config_maplist';
  $getmaps = mysqli_query($dbLink, "SELECT `mapname` FROM `$mapList`;");
  $nummaps = mysqli_num_rows ($getmaps);

  for ($i = 0; $i < $nummaps; $i++) {
    if (mysqli_result($getmaps,$i,0) != 'overview') {
      $map = mysqli_result($getmaps,$i,0);
      $deskTable = 'desks_'.$map;
      $getprinters = mysqli_query($dbLink, "SELECT `employee` FROM `$deskTable` WHERE `desktype` LIKE 'printer';");
      $numprinters = mysqli_num_rows ($getprinters);

      for ($h = 0; $h < $numprinters; $h++) {
        $printerTable = 'printerstatus';
        $printername = mysqli_result($getprinters,$h,0);
        $checkupdate = mysqli_query($dbLink, "SELECT `ID` FROM `$printerTable` WHERE `printername` LIKE '$printername' AND `map` LIKE '$map';");
        $num = mysqli_num_rows ($checkupdate);

        $community = "public"; 

        $snmpcheck = false;
        $pingresult = '';
        $availability = '0';
        $ausgabe = '';

        // ping printer to check availability
        exec("ping -c 1 ".$printername, $ausgabe); 
        if (stripos($ausgabe[4], 'Name or service not known') !== false) {
          $availability = '0';
        }
        else {
          $x = explode(',', $ausgabe[4]);
          $pingresult = "$x[2]";
          if ($pingresult == ' 0% packet loss') {
            $availability = '1';
            $snmpcheck = snmpget("$printername","$community",".1.3.6.1.2.1.43.11.1.1.9.1.1");
          } 
          else {
            $availability = '0';
          }
        }
        
        // check cadridge status via snmp
        
        if ($availability == '1' && $snmpcheck != false) {
          $data = snmpget("$printername","$community",".1.3.6.1.2.1.43.11.1.1.9.1.1"); 
          $data_name = snmpget("$printername","$community",".1.3.6.1.2.1.43.12.1.1.4.1.1"); 
          if (stripos($data_name, 'Hex-STRING') !== false) {
            $hexstring = str_replace(' ','',$data_name); 
            $hexstring = str_replace('Hex-STRING:','',$hexstring); 
            $data_name = hexToStr ($hexstring);
            $data_name = str_replace(' ','',$data_name); 
          }  
          else if (stripos($data_name, 'STRING') !== false) {
            preg_match('/^[^"]*"([^"]*)"$/', $data_name, $matches);
            $data_name = $matches[1];
          }
          $color1 = intval(str_replace("INTEGER: ", "", $data)); 
          $colorname1 = $data_name;

          $data = snmpget("$printername","$community",".1.3.6.1.2.1.43.11.1.1.9.1.2"); 
          $data_name = snmpget("$printername","$community",".1.3.6.1.2.1.43.12.1.1.4.1.2"); 
          if (stripos($data_name, 'Hex-STRING') !== false) {
            $hexstring = str_replace(' ','',$data_name); 
            $hexstring = str_replace('Hex-STRING:','',$hexstring); 
            $data_name = hexToStr ($hexstring);
            $data_name = str_replace(' ','',$data_name); 
          }  
          else if (stripos($data_name, 'STRING') !== false) {
            preg_match('/^[^"]*"([^"]*)"$/', $data_name, $matches);
            $data_name = $matches[1];
          }
          $color2 = intval(str_replace("INTEGER: ", "", $data)); 
          $colorname2 = $data_name;

          $data = snmpget("$printername","$community",".1.3.6.1.2.1.43.11.1.1.9.1.3"); 
          $data_name = snmpget("$printername","$community",".1.3.6.1.2.1.43.12.1.1.4.1.3"); 
          if (stripos($data_name, 'Hex-STRING') !== false) {
            $hexstring = str_replace(' ','',$data_name); 
            $hexstring = str_replace('Hex-STRING:','',$hexstring); 
            $data_name = hexToStr ($hexstring);
            $data_name = str_replace(' ','',$data_name); 
          }  
          else if (stripos($data_name, 'STRING') !== false) {
            preg_match('/^[^"]*"([^"]*)"$/', $data_name, $matches);
            $data_name = $matches[1];
          }
          $color3 = intval(str_replace("INTEGER: ", "", $data));
          $colorname3 = $data_name;

          $data = snmpget("$printername","$community",".1.3.6.1.2.1.43.11.1.1.9.1.4"); 
          $data_name = snmpget("$printername","$community",".1.3.6.1.2.1.43.12.1.1.4.1.4"); 
          if (stripos($data_name, 'Hex-STRING') !== false) {
            $hexstring = str_replace(' ','',$data_name); 
            $hexstring = str_replace('Hex-STRING:','',$hexstring); 
            $data_name = hexToStr ($hexstring);
            $data_name = str_replace(' ','',$data_name); 
          }  
          else if (stripos($data_name, 'STRING') !== false) {
            preg_match('/^[^"]*"([^"]*)"$/', $data_name, $matches);
            $data_name = $matches[1];
          }
          $color4 = intval(str_replace("INTEGER: ", "", $data)); 
          $colorname4 = $data_name;
        } 
        else {
          $color1 = '0';
          $color2 = '0';
          $color3 = '0';
          $color4 = '0';
          $colorname1 = '0';
          $colorname2 = '0';
          $colorname3 = '0';
          $colorname4 = '0';
        }
        

        if ($num != 0) {
            // Update existing DB entry
            $printerid = mysqli_result($checkupdate,0,0);
            mysqli_query($dbLink, "UPDATE `$dbName`.`$printerTable` SET `map` = '$map', `printername` = '$printername', `availability` = '$availability', `color1` = '$color1', `color2` = '$color2', `color3` = '$color3', `color4` = '$color4' , `colorname1` = '$colorname1', `colorname2` = '$colorname2', `colorname3` = '$colorname3', `colorname4` = '$colorname4'WHERE `ID` = $printerid;");
            echo '[DONE] '.$printername." updated\n";
        }
        else 
        {
            // Create new entry
            $createsql = mysqli_query($dbLink, "INSERT INTO `$dbName`.`$printerTable` 
                (`ID`, `map`, `printername`, `availability`, `color1`, `color2`, `color3`, `color4`, `colorname1`, `colorname2`, `colorname3`, `colorname4`) 
                VALUES (NULL, '$map', '$printername','$availability', '$color1', '$color2', '$color3', '$color4', '$colorname1', '$colorname2', '$colorname3', '$colorname4');");
            echo '[DONE] '.$printername." created\n";
        }
      }
    }
    
  }
  echo '[ALL DONE]'."\n";
    
}
else {
  // Create array for output
  $printer_arr=array();
  $printer_arr["printers"]=array();
  $dbTable = 'printerstatus';
  if ($getmap != '') {
    $printers  = mysqli_query($dbLink, "SELECT * FROM `$dbTable` WHERE `map` LIKE '$getmap'");
    $numcache = mysqli_num_rows ($printers);
    for ($i = 0; $i < $numcache; $i++) {
      $printeritem=array(
          "printername"  => mysqli_result($printers,$i,2),
          "availability" => mysqli_result($printers,$i,3),
          trim(mysqli_result($printers,$i,8))   => mysqli_result($printers,$i,4),
          trim(mysqli_result($printers,$i,9))    => mysqli_result($printers,$i,5),
          trim(mysqli_result($printers,$i,10))      => mysqli_result($printers,$i,6),
          trim(mysqli_result($printers,$i,11))      => mysqli_result($printers,$i,7),
      );
      array_push($printer_arr["printers"], $printeritem);
    }
  }
  else {
    $printers  = mysqli_query($dbLink, "SELECT * FROM `$dbTable`");
    $numcache = mysqli_num_rows ($printers);
    for ($i = 0; $i < $numcache; $i++) {
      $printeritem=array(
          "map"          => mysqli_result($printers,$i,1),
          "printername"  => mysqli_result($printers,$i,2),
          "availability" => mysqli_result($printers,$i,3),
          trim(mysqli_result($printers,$i,8))   => mysqli_result($printers,$i,4),
          trim(mysqli_result($printers,$i,9))    => mysqli_result($printers,$i,5),
          trim(mysqli_result($printers,$i,10))      => mysqli_result($printers,$i,6),
          trim(mysqli_result($printers,$i,11))      => mysqli_result($printers,$i,7),
      );
      array_push($printer_arr["printers"], $printeritem);
    }
  }
  
  echo json_encode($printer_arr);
}
?>