<?php
// required headers
header("Access-Control-Allow-Origin: *");
header("Content-Type: application/json; charset=UTF-8");

# Loading shared functions and config file
include '../../shared.php';

$dbTable = 'config_teams';
$dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
mysqli_set_charset($dbLink, "utf8");

$details = mysqli_query($dbLink, "SELECT * FROM $dbTable ORDER BY `teamname` ASC;");
$num   = mysqli_num_rows ($details);  

$teams_arr=array();
$teams_arr["teams"]=array();

for ($i = 0; $i < $num; $i++) {
    $teamname    = mysqli_result($details,$i,1);
    $teammembers = mysqli_result($details,$i,2);
    
    $product_item=array(
        "teamname" => $teamname,
        "members" => $teammembers
    );
    array_push($teams_arr["teams"], $product_item);
}

echo json_encode($teams_arr);
?>