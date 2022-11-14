<?php
// required headers
header("Access-Control-Allow-Origin: *");
header("Content-Type: application/json; charset=UTF-8");

# Loading shared functions and config file
include '../../shared.php';

if ($_SERVER['REQUEST_METHOD'] == "GET") {
    $mode = $_GET['mode'];  
    $dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
    mysqli_set_charset($dbLink, "utf8");
    }

$config_arr=array();

switch ($mode) {
  case "mapflags":
    $config_arr["mapflags"]=array();
    $countryflag_files = scandir('../../countryflags');

    // to prevent faulty countryflags values, the directory is scanned and a dropdown is used instead of a text field
    for ($c = 2; $c < count($countryflag_files); $c++) {
      $countryparts = explode('.', $countryflag_files[$c]);
      $countrytag = strtolower($countryparts[0]);
      array_push($config_arr["mapflags"], $countrytag);
    }
    break;

  case "maps":
    $config_arr["maps"]=array();
    $dbTable = 'config_maplist';
    $config_maplist = mysqli_query($dbLink, "SELECT * FROM $dbTable ORDER BY `mapname` ASC;");
    $num   = mysqli_num_rows ($config_maplist);  
    
    for ($i = 0; $i < $num; $i++) {
      $id        = mysqli_result($config_maplist,$i,0);
      $mapname   = mysqli_result($config_maplist,$i,1);
      $itemscale = mysqli_result($config_maplist,$i,2);
      $published = mysqli_result($config_maplist,$i,3);
      $country   = mysqli_result($config_maplist,$i,4);
      $flagsize  = mysqli_result($config_maplist,$i,5);
      $timezone  = mysqli_result($config_maplist,$i,6);
      $address   = mysqli_result($config_maplist,$i,7);
      $x         = mysqli_result($config_maplist,$i,8);
      $y         = mysqli_result($config_maplist,$i,9);
      
      $mapitem=array(
        "id"        => $id,
        "mapname"   => $mapname,
        "itemscale" => $itemscale,
        "published" => $published,
        "country"   => $country,
        "flagsize"  => $flagsize,
        "timezone"  => $timezone,
        "address"   => $address,
        "x"         => $x,
        "y"         => $y
      );
      array_push($config_arr["maps"], $mapitem);
    }
    break;
  default:
    $config_arr["error"]=array();
    array_push($config_arr["error"], 'Please specify a mode');
    break;
}

echo json_encode($config_arr);
?>