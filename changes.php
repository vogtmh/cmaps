<?php
  ob_start();
  session_start();

  # Loading shared functions and config
  include 'shared.php';
?>
<!DOCTYPE HTML>
<!-- ===================================================================

  CompanyMaps 8.1 Client
  Release date 2023-03-20
  Copyright (c) 2016-2022 by MavoDev
  see https://www.mavodev.de for more details

==================================================================== -->

<html lang="de">
<head>
  <meta name="generator" content="HTML Tidy for Windows (vers 22 March 2008), see www.w3.org">

  <title><?php echo $apptitle?></title>
  <!-- <meta http-equiv="Content-Type" content="text/html; charset=utf-8"> -->
  <meta charset="utf-8">
  <link rel="stylesheet" type="text/css" href="cmaps80.css">
  <link rel="stylesheet" type="text/css" href="client80.css">
  <!--<link href='https://fonts.googleapis.com/css?family=Roboto' rel='stylesheet' type='text/css'>-->
  <!-- FAVICONS -->
  <link rel="apple-touch-icon" sizes="57x57" href="favicons/apple-touch-icon-57x57.png">
  <link rel="apple-touch-icon" sizes="60x60" href="favicons/apple-touch-icon-60x60.png">
  <link rel="apple-touch-icon" sizes="72x72" href="favicons/apple-touch-icon-72x72.png">
  <link rel="apple-touch-icon" sizes="76x76" href="favicons/apple-touch-icon-76x76.png">
  <link rel="apple-touch-icon" sizes="114x114" href="favicons/apple-touch-icon-114x114.png">
  <link rel="apple-touch-icon" sizes="120x120" href="favicons/apple-touch-icon-120x120.png">
  <link rel="apple-touch-icon" sizes="144x144" href="favicons/apple-touch-icon-144x144.png">
  <link rel="apple-touch-icon" sizes="152x152" href="favicons/apple-touch-icon-152x152.png">
  <link rel="apple-touch-icon" sizes="180x180" href="favicons/apple-touch-icon-180x180.png">
  <link rel="apple-touch-startup-image" href="favicons/android-chrome-512x512.png">
  <link rel="icon" type="image/png" href="favicons/favicon-32x32.png" sizes="32x32">
  <link rel="icon" type="image/png" href="favicons/android-chrome-192x192.png" sizes="192x192">
  <link rel="icon" type="image/png" href="favicons/favicon-96x96.png" sizes="96x96">
  <link rel="icon" type="image/png" href="favicons/favicon-16x16.png" sizes="16x16">
  <link rel="manifest" href="favicons/manifest.json">
  <link rel="mask-icon" href="favicons/safari-pinned-tab.svg" color="#5BBAD5">
  <link rel="shortcut icon" href="favicons/favicon.ico">
  <meta name="msapplication-TileColor" content="#2d89ef">
  <meta name="msapplication-TileImage" content="favicons/mstile-144x144.png">
  <meta name="msapplication-config" content="favicons/browserconfig.xml">
  <meta name="theme-color" content="#000000">
  <meta name="apple-mobile-web-app-capable" content="yes">
  <meta name="mobile-web-app-capable" content="yes">
  <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=4.0, user-scalable=yes">
  <meta name="apple-mobile-web-app-status-bar-style" content="black-translucent">
  <!-- SCRIPTS -->
  <script src="tools/jquery3.js"></script>
  <script src="tools/jquery-migrate-1.4.1.min.js"></script>
  <script src="changes.js"></script>
</head>

<div id="header" style="position:fixed; top:0px; left:0px; width:100%; height: 60px; padding:10px; line-height:60px; z-index:5000; background-color:black;">
  <label for="cars">Limit results:</label>

  <select name="limit" id="limit" style="width: 100px; " onchange="updateChangesOverview();">
    <option value="50">50</option>
    <option value="100">100</option>
    <option value="200">200</option>
    <option value="500">500</option>
    <option value="2000">2000</option>
    <option value="5000">5000</option>
    <option value="10000">10000</option>
  </select>

  <img src="images/activity_on.gif" id="activity" style="height:30px; float:right; margin-top: 15px; margin-right:20px; display: none;">
</div>

<div id="announcementbar_body" style="width:100%; height: auto; margin-top: 80px;"></div>

<script> updateChangesOverview() </script>
