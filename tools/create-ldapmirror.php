<?php
/**********************
 * This file creates a sample ldap-mirror to test LDAP features without LDAP server
 * Copy this file in the cmaps root and run it
 */

# Loading shared functions and config file
include '../shared.php';

$firstnames = array ("Max", "Alexander", "Michael", "Matthias", "Tom", "Simon", "Hermann", "Stefan", "Marcel", "Marc",
                    "Leon", "Brad", "Jonas", "Peter", "Tobias", "Pascal", "Moritz", "Jan", "Samuel", "Kevin",
                    "Fabian", "Sebastian", "Luis", "Daniel", "Adrian", "Lars", "Dominik", "Oliver", "Thomas", "Timo");

$lastnames =  array ("Müller", "Maier", "Schuster", "Maurer", "Wamsler", "Schmidt", "Schneider", "Fischer", "Wagner", "Becker",
                    "Schulz", "Hoffmann", "Schäfer", "Koch", "Bauer", "Richter", "Klein", "Wolf", "Neumann", "Schwarz",
                    "Lange", "Krause", "Walter", "Kaiser", "Keller", "Baumann", "Franke", "Stein", "Jäger", "Schulte");

$titles    =  array ("Software Developer", "Manager", "Technician", "IT Administrator", "Engineer", "Build Engineer", "Talent Acquisition", "Counsellor", "External", "Analyst",
                     "Scrum Master", "Office Management", "Product Owner", "HR Professional", "Security Specialist", "IT Operations", "Web Developer", "Communications Manager", "Marketing Specialist", "Social Media Manager",
                     "Support Engineer", "Trainer", "Project Manager", "Content Creator", "Creative Designer", "Treasury Specialist", "Accountant", "Financial Analyst", "Legal Consultant", "Big Data Engineer");

$MySqlLink   = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);

// Clear database
mysqli_query($MySqlLink, "DELETE FROM `ldap-mirror`;");

for ($i = 1; $i < 41; $i++) {

   // Create random users
   $counter   = sprintf("%02d", $i);
   $firstname = $firstnames[array_rand($firstnames)];
   $lastname  = $lastnames[array_rand($lastnames)];
   $phone     = "07173 0000";
   $mail      = $firstname.'.'.$lastname.'@mavodev.de';
   $pdon      = 'Demo-'.$counter;
   $ipphone   = $counter;
   $title     = $titles[array_rand($titles)];

   // Output generated users
   echo $firstname.' '.$lastname.' | '.$phone.' | '.$mail.' | '.$pdon.' | '.$title.'<br />';

   // Add user to database
   mysqli_query($MySqlLink,
   "INSERT INTO `ldap-mirror` (`ID`, `givenname`, `surname`, `telephonenumber`, `mail`, `physicaldeliveryofficename`, `ipphone`, `description`, `department`, `mobile`) 
   VALUES (NULL, '$firstname', '$lastname', '$phone', '$mail', '$pdon', '$ipphone', '$title', '', '');");
}
?>