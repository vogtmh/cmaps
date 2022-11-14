<?php
// required headers
header("Access-Control-Allow-Origin: *");
header("Content-Type: application/json; charset=UTF-8");

# Loading shared functions and config file
include '../../shared.php';

if ($_SERVER['REQUEST_METHOD'] == "GET") {
    $search = $_GET['search'];  
    $titlesearch = $_GET['title'];
    }

$dbTable = 'ldap-mirror';
$dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
mysqli_set_charset($dbLink, "utf8");
if ($search != '' && $titlesearch != '') {
    $details = mysqli_query($dbLink, "SELECT * FROM `$dbTable` 
    WHERE (
        givenname LIKE '%$search%' 
        OR surname LIKE '%$search%' 
        OR physicaldeliveryofficename LIKE '%$search%'
    ) 
    AND (
        description LIKE '%$titlesearch%'
        )
    ");
}
else if ($titlesearch != '') {
    $details = mysqli_query($dbLink, "SELECT * FROM `$dbTable` 
    WHERE description LIKE '%$titlesearch%'");
}
else if ($search != '') {
    $details = mysqli_query($dbLink, "SELECT * FROM `$dbTable` 
    WHERE givenname LIKE '%$search%' 
    OR surname LIKE '%$search%' 
    OR physicaldeliveryofficename LIKE '%$search%'");
}
else if ($search != '') {
    $details = mysqli_query($dbLink, "SELECT * FROM `$dbTable` 
    WHERE givenname LIKE '%$search%' 
    OR surname LIKE '%$search%' 
    OR physicaldeliveryofficename LIKE '%$search%'");
}
else {
    $details = mysqli_query($dbLink, "SELECT * FROM `$dbTable`");
}

$num   = mysqli_num_rows ($details);  

$users_arr=array();
$users_arr["users"]=array();

for ($i = 0; $i < $num; $i++) {
    $givenname      = mysqli_result($details,$i,1);
    $surname        = mysqli_result($details,$i,2);
    $phone          = mysqli_result($details,$i,3);
    $mail           = mysqli_result($details,$i,4);
    $desk           = mysqli_result($details,$i,5);
    $samaccountname = mysqli_result($details,$i,6);
    $title          = mysqli_result($details,$i,7);
    
    $product_item=array(
        "givenname" => $givenname,
        "surname" => $surname,
        "phone" => $phone,
        "mail" => $mail,
        "desk" => $desk,
        "samaccountname" => $samaccountname,
        "title" => $title
    );
    array_push($users_arr["users"], $product_item);
}

echo json_encode($users_arr);
?>