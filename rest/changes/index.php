<?php
// required headers
header("Access-Control-Allow-Origin: *");
header("Content-Type: application/json; charset=UTF-8");

# Loading shared functions and config file
include '../../shared.php';

if ($_SERVER['REQUEST_METHOD'] == "GET") {
    $maxresults = htmlspecialchars($_GET['maxresults'], ENT_QUOTES);  
    }

$dbTable = 'ldap_changelog';
$dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
mysqli_set_charset($dbLink, "utf8");

$ldapchangelog = mysqli_query($dbLink, "SELECT * FROM $dbTable ORDER BY `ID` DESC;");
$num   = mysqli_num_rows ($ldapchangelog);  

$changes_arr=array();
$changes_arr["changes"]=array();
$resultsdone = 0;
if ($maxresults == '') {$maxresults = 99999999;}

for ($i = 0; $i < $num; $i++) {
    if ($resultsdone >= $maxresults) {break;}
    $id       = mysqli_result($ldapchangelog,$i,0);
    $fullname = mysqli_result($ldapchangelog,$i,6);
    $avatar   = mysqli_result($ldapchangelog,$i,7);
    $type     = mysqli_result($ldapchangelog,$i,8);
    $oldvalue = mysqli_result($ldapchangelog,$i,9);
    $newvalue = mysqli_result($ldapchangelog,$i,10);
    
    if ($type == 'Title' || $type == "Employee") {
        $product_item=array(
            "fullname" => $fullname,
            "avatar" => $avatar,
            "type" => $type,
            "oldvalue" => $oldvalue,
            "newvalue" => $newvalue,
            "id" => $id
        );
        array_push($changes_arr["changes"], $product_item);
        $resultsdone++;
    }
}

echo json_encode($changes_arr);
?>