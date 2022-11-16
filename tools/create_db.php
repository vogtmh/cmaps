<?php

# Loading shared functions and config file
include '../shared.php';

# local override for testing
$dbName = 'test1';

$freshDB = false;

function checkDB() {
    global $dbServer,$dbUser,$dbPass,$dbName;
    $checkLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
    if (!$checkLink) {
        return 'false';
    }
    else {
        return 'true';
    }
}

if (checkDB() == 'false') {
    $createLink = mysqli_connect($dbServer,$dbUser,$dbPass);
    mysqli_query($createLink, "CREATE DATABASE `$dbName`;");
    if (checkDB() == 'false') {
        echo "database $dbName is missing and could not be created<br/>";
    }
    else {
        echo "database $dbName was missing and has been created<br/>";
        $freshDB = true;
    }
}
else {
    $check = checkDB();
    echo "database $dbName exists: $check <br/>";
}

$dbLink = mysqli_connect($dbServer,$dbUser,$dbPass,$dbName);
mysqli_query($dbLink,"SET NAMES 'utf8'");

mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`auditlog` ( `ID` BIGINT NOT NULL AUTO_INCREMENT , `EventTime` TEXT NOT NULL , `EventType` TEXT NOT NULL , `EventUser` TEXT NOT NULL , `EventInfo` TEXT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`bookings` ( `ID` BIGINT NOT NULL AUTO_INCREMENT , `date` TEXT NOT NULL , `map` TEXT NOT NULL , `desk` TEXT NOT NULL , `user` TEXT NOT NULL , `fullname` TEXT NOT NULL , `phone` TEXT NOT NULL , `mail` TEXT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`config_department_list` ( `ID` INT NOT NULL AUTO_INCREMENT , `department-name` TEXT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`config_general` ( `ID` INT NOT NULL AUTO_INCREMENT , `variable` TEXT NOT NULL , `value` TEXT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`config_ldap` ( `ID` INT NOT NULL AUTO_INCREMENT , `description` TEXT NOT NULL , `server` TEXT NOT NULL , `type` TEXT NOT NULL , `OU` TEXT NOT NULL , `LdapUser` TEXT NOT NULL , `LdapPass` TEXT NOT NULL , `LastSync` TEXT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`config_mapadmins` ( `ID` INT NOT NULL AUTO_INCREMENT , `user` TEXT NOT NULL , `role` TEXT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`config_maplist` ( `ID` INT NOT NULL AUTO_INCREMENT , `mapname` TEXT NOT NULL , `itemscale` TEXT NOT NULL , `published` TEXT NOT NULL , `country` TEXT NOT NULL , `flagsize` TEXT NOT NULL , `timezone` TEXT NOT NULL , `address` TEXT NOT NULL , `mapX` INT NOT NULL , `mapY` INT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`config_robinspaces` ( `ID` INT NOT NULL AUTO_INCREMENT , `spacename` TEXT NOT NULL , `spaceid` INT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`config_roles` ( `ID` INT NOT NULL AUTO_INCREMENT , `rolename` TEXT NOT NULL , `perm_desks` TINYINT NOT NULL , `perm_dashboard` TINYINT NOT NULL , `perm_config` TINYINT NOT NULL , `perm_ldap` TINYINT NOT NULL , `perm_maps` TINYINT NOT NULL , `perm_users` TINYINT NOT NULL , `perm_teams` TINYINT NOT NULL , `perm_stats` TINYINT NOT NULL , `perm_auditlog` TINYINT NOT NULL , `perm_health` TINYINT NOT NULL , `perm_adminpanel` TINYINT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`config_teams` ( `ID` INT NOT NULL AUTO_INCREMENT , `teamname` TEXT NOT NULL , `teammembers` LONGTEXT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`config_vips` ( `ID` INT NOT NULL AUTO_INCREMENT , `Parsed Text in Job Title` TEXT NOT NULL , `Type` TEXT NOT NULL , `Description` TEXT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`health_whitelist` ( `ID` INT NOT NULL AUTO_INCREMENT , `type` TEXT NOT NULL , `text` TEXT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`ldap-mirror` ( `ID` BIGINT NOT NULL AUTO_INCREMENT , `givenname` MEDIUMTEXT NOT NULL , `surname` MEDIUMTEXT NOT NULL , `telephonenumber` MEDIUMTEXT NOT NULL , `mail` MEDIUMTEXT NOT NULL , `physicaldeliveryofficename` MEDIUMTEXT NOT NULL , `ipphone` MEDIUMTEXT NOT NULL , `description` MEDIUMTEXT NOT NULL , `department` MEDIUMTEXT NOT NULL , `mobile` MEDIUMTEXT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`ldap_changelog` ( `ID` BIGINT NOT NULL AUTO_INCREMENT , `year` SMALLINT NOT NULL , `month` TINYINT NOT NULL , `day` TINYINT NOT NULL , `hour` TINYINT NOT NULL , `minute` TINYINT NOT NULL , `name` TEXT NOT NULL , `avatar` TEXT NOT NULL , `type` TEXT NOT NULL , `oldvalue` TEXT NOT NULL , `newvalue` TEXT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`meetingstatus` ( `ID` INT NOT NULL AUTO_INCREMENT , `map` TEXT NOT NULL , `room` TEXT NOT NULL , `availability` TEXT NOT NULL , `now_title` TEXT NOT NULL , `now_start` TEXT NOT NULL , `now_end` TEXT NOT NULL , `now_tz` TEXT NOT NULL , `next_title` TEXT NOT NULL , `next_start` TEXT NOT NULL , `next_end` TEXT NOT NULL , `next_tz` TEXT NOT NULL , `deskid` TEXT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`printerstatus` ( `ID` INT NOT NULL AUTO_INCREMENT , `map` TEXT NOT NULL , `printername` TEXT NOT NULL , `availability` TEXT NOT NULL , `color1` TEXT NOT NULL , `color2` TEXT NOT NULL , `color3` TEXT NOT NULL , `color4` TEXT NOT NULL , `colorname1` TEXT NOT NULL , `colorname2` TEXT NOT NULL , `colorname3` TEXT NOT NULL , `colorname4` TEXT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");
mysqli_query($dbLink, "CREATE TABLE IF NOT EXISTS `$dbName`.`stats` ( `ID` BIGINT NOT NULL AUTO_INCREMENT , `date` DATE NOT NULL , `year` INT NOT NULL , `month` INT NOT NULL , `day` INT NOT NULL , `count` BIGINT NOT NULL , PRIMARY KEY (`ID`)) ENGINE = InnoDB; ");

if ($freshDB) {

    #config_department_list
    mysqli_query($dbLink, "INSERT INTO `config_department_list` (`ID`, `department-name`) VALUES
    (1, '- none -'),
    (2, 'BusinessDevelopment'),
    (3, 'BusinessIntelligence'),
    (4, 'Corporate'),
    (5, 'Development'),
    (6, 'Finance'),
    (7, 'HumanResources'),
    (8, 'IntDev'),
    (9, 'IT-Administration'),
    (10, 'Marketing'),
    (11, 'OfficeManagement'),
    (12, 'ProductManagement'),
    (13, 'QualityAssurance'),
    (14, 'Sales'),
    (15, 'SalesOperations'),
    (16, 'Support');");

    #config_general
    mysqli_query($dbLink, "INSERT INTO `config_general` (`ID`, `variable`, `value`) VALUES
    (1, 'dbTable', 'desks'),
    (2, 'ldapTable', 'ldap-mirror'),
    (3, 'avatarDir', 'avatarcache/'),
    (4, 'avatarType', 'jpg'),
    (5, 'targetScreenWidth', '1600'),
    (6, 'adauth', '0'),
    (7, 'ldapserver', ''),
    (8, 'domain', ''),
    (9, 'map_default', 'overview'),
    (11, 'logo_regular', 'images/cmaps-regular.png'),
    (12, 'logo_hover', 'images/cmaps-hover.png'),
    (13, 'apptitle', 'CompanyMaps'),
    (14, 'ldaptype', 'LDAPS'),
    (15, 'ldap_user', ''),
    (16, 'ldap_pass', ''),
    (17, 'ldap_ou', 'OU=Users,DC=example,DC=org'),
    (18, 'teamsContact', ''),
    (19, 'teamsChannel', ''),
    (20, 'MSTEAMS_WEBHOOK', ''),
    (21, 'robintoken', ''),
    (22, 'robinOrganisation', '');");

    #config_mapadmins
    mysqli_query($dbLink, "INSERT INTO `config_mapadmins`(`ID`, `user`, `role`) VALUES (NULL,'$dbUser','$dbPass');");

    #config_roles
    mysqli_query($dbLink, "INSERT INTO `config_roles` (`ID`, `rolename`, `perm_desks`, `perm_dashboard`, `perm_config`, `perm_ldap`, `perm_maps`, `perm_users`, `perm_teams`, `perm_stats`, `perm_auditlog`, `perm_health`, `perm_adminpanel`) VALUES
    (1, 'superadmin', 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2),
    (2, 'admin', 2, 1, 0, 1, 0, 1, 2, 1, 0, 1, 2),
    (3, 'groupmanager', 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0),
    (4, 'deskmaintainer', 2, 1, 0, 0, 0, 0, 0, 0, 0, 1, 2),
    (5, 'user', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0);");

    
}

?>