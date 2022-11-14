<?php
session_start(); 
// required headers
header("Access-Control-Allow-Origin: *");
header("Content-Type: application/json; charset=UTF-8");

# CompanyMaps 8.0 Account API
# Release date 2022-11-14
# Copyright (c) 2016-2022 by MavoDev
# see https://www.mavodev.de for more details

# default = SAML

# ldapserver and ldap_ou have to be stored in the config_general database table

# Loading shared functions and config file
include '../../shared.php';

$mode    = '';
$output  = array();
$status  = '';
$message = '';
$data    = array();

if ($_SERVER['REQUEST_METHOD'] == "GET") {
    if (isset($_GET['mode'])) {$mode = $_GET['mode']; } else {$mode='';}
    if (isset($_GET['user'])) {$user = $_GET['user']; } else {$user='';}
    if (isset($_GET['password'])) {$password = $_GET['password']; } else {$password='';}
}

switch ($mode) {

    case 'login':
        # With adauth on, LDAP probe is done to check if user/password are correct. 
        if ($user != "" && $password !="" && ($adauth==1)) { 

            switch ($ldaptype){
            case "LDAP":
                $ldapconn=ldap_connect($ldapserver);
                ldap_set_option($ldapconn, LDAP_OPT_PROTOCOL_VERSION, 3);
                ldap_set_option($ldapconn, LDAP_OPT_REFERRALS, 0); 
                break;
            case "LDAPS":
                $ldapconn=ldap_connect("ldaps://$ldapserver",636);		
                ldap_set_option($ldapconn, LDAP_OPT_PROTOCOL_VERSION, 3);
                ldap_set_option($ldapconn, LDAP_OPT_REFERRALS, 0);  					
                break;
            default:
                $status = 'error';
                $message = "database configuration failure";
                throw new Exception('LDAP Type not set in database');
            }  
            if(filter_var($user, FILTER_VALIDATE_EMAIL)) {
                # Test login for email
                $mail = $user; 

                if(ldap_bind($ldapconn, $ldap_user, $ldap_pass)) {

                    $arr = array('dn', 'samaccountname', 1);
                    $result = ldap_search($ldapconn, $ldap_ou, "(mail=$mail)", $arr);
                    $entries = ldap_get_entries($ldapconn, $result);
                    if ($entries['count'] > 0) {
                    $ldapresult=ldap_bind($ldapconn, $entries[0]['dn'], utf8_decode($password));
                    $samaccountname=$entries[0]['samaccountname'][0];
                    if ($domain != "") {$user = "$domain\\$samaccountname";}
                    }
                    else {
                        auditlog("Access",$mail,"User not found");   
                        $status = 'error';
                        $message = "wrong user/password";
                    }
                }
            }
            else {
                # Test login for username
                if ($domain != "") {
                    $samaccountname = $user;
                    $user = $domain.'\\'.$user;
                }
                $ldapresult=ldap_bind($ldapconn, $user, utf8_decode($password));
            }
            
            # Login failed
            if (!$ldapresult) {
                auditlog("Access",$user,"Login denied (wrong user/pass)");   
                $status = 'error';
                $message = "wrong user/password";
                sleep(3);
            }
            # Login successful, get more details of user
            else {
            $fullname = ucfirst($samaccountname);
            $telephonenumber = '-';
            $mail = '-';

            if(ldap_bind($ldapconn, $ldap_user, $ldap_pass)) {
                $arr = array('dn', 'samaccountname', 'name', 'telephonenumber', 'mail', 1);
                $result = ldap_search($ldapconn, $ldap_ou, "(samaccountname=$samaccountname)", $arr);
                $entries = ldap_get_entries($ldapconn, $result);
                if ($entries['count'] > 0) {
                $ldapresult=ldap_bind($ldapconn, $entries[0]['dn'], utf8_decode($password));
                $samaccountname=$entries[0]['samaccountname'][0];
                if (isset($entries[0]["name"][0])) {$fullname = str_replace("'","\'",$entries[0]["name"][0]);} else {$fullname = $samaccountname;};
                if (isset($entries[0]["telephonenumber"][0])) {$telephonenumber = $entries[0]["telephonenumber"][0];} else {$telephonenumber = '-';};
                if (isset($entries[0]["mail"][0])) {$mail = $entries[0]["mail"][0];} else {$mail = '-';};
                }
            }
            $editmode = 1;
            $_SESSION['username'] = $user;  
            $_SESSION['usershort'] = $samaccountname;   
            $_SESSION['fullname'] = $fullname; 
            $_SESSION['telephonenumber'] = $telephonenumber; 
            $_SESSION['mail'] = $mail; 

            auditlog("Access",$user,"User has logged in");   
            }
            ldap_close($ldapconn);
        }

        # If adauth is off, user logins are database users, so mysql probe is done instead.  
        else if ($user != "" && $password !="" && ($adauth==0)) {
                if (in_array($user, $mapadmins)){
                $dbLink = mysqli_connect($dbServer,$user,$password,$dbName);
                #mysqli_query($dbLink, "SET NAMES 'utf8'", $checkdbhandle);
                
                if (!$dbLink) {
                    auditlog("Access",$user,"Login denied (wrong user/pass)");
                    $status = 'error';
                    $message = "wrong user/password";
                    sleep(3);
                }
                else {
                    $editmode = 1;
                    $_SESSION['username'] = $user;
                    $_SESSION['usershort'] = $user;
                    $_SESSION['fullname'] = $user;  
                    $_SESSION['telephonenumber'] = '-'; 
                    $_SESSION['mail'] = '-'; 
                    auditlog("Access",$user,"User has logged in");
                }
                }
                else {
                auditlog("Access",$user,"Login denied (no admin user)");
                sleep(3);
                }  
        }
        else {
            $status = 'error';
            $message = 'parameters missing';
            break;
        }
        if (isset($_SESSION['username'])) {
            $editmode = 1; 
            $_SESSION['editmode'] = base64_encode($_SESSION['username']);
            $status = 'ok';
            $message = "$user has been logged in";
        }  
        break;

    default: #SAML Login

        switch ($ldaptype){
        case "LDAP":
            $ldapconn=ldap_connect($ldapserver);
            ldap_set_option($ldapconn, LDAP_OPT_PROTOCOL_VERSION, 3);
            ldap_set_option($ldapconn, LDAP_OPT_REFERRALS, 0); 
            break;
        case "LDAPS":
            $ldapconn=ldap_connect("ldaps://$ldapserver",636);	
            ldap_set_option($ldapconn, LDAP_OPT_PROTOCOL_VERSION, 3);
            ldap_set_option($ldapconn, LDAP_OPT_REFERRALS, 0); 	 					
            break;
        default:
            $status = 'error';
            $message = "database configuration failure";
            throw new Exception('LDAP Type not set in database');
        } 

        require_once (dirname(__FILE__) . '/../../../simplesamlphp/lib/_autoload.php');
        $as = new SimpleSAML_Auth_Simple('default-sp');
        $as->requireAuth();

        $attributes = $as->getAttributes();

        # close SAML session
        $session = SimpleSAML_Session::getSessionFromRequest();
        $session->cleanup();
        # switch back to user session
        session_start();

        $mail = $attributes['http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress']['0'];
        $_SESSION['mail'] = $mail;
        $fullname = $attributes['http://schemas.microsoft.com/identity/claims/displayname']['0'];
        $_SESSION['fullname'] = $fullname; 
        
        if(ldap_bind($ldapconn, $ldap_user, $ldap_pass)) {

            $arr = array('dn', 'samaccountname', 'telephonenumber', 1);
            $result = ldap_search($ldapconn, $ldap_ou, "(mail=$mail)", $arr);
            $entries = ldap_get_entries($ldapconn, $result);
            $telephonenumber = '-';
            if ($entries['count'] > 0) {
                $samaccountname=$entries[0]['samaccountname'][0];
                if (isset($entries[0]["telephonenumber"][0])) {$telephonenumber = $entries[0]["telephonenumber"][0];} else {$telephonenumber = '-';};
                if ($domain != "") {$user = "$domain\\$samaccountname";}
            }
            else {
                auditlog("Access",$mail,"User not found");   
                $status = 'error';
                $message = "wrong user/password";
            }
        }
        
        $_SESSION['username'] = $user;  
        $_SESSION['usershort'] = $samaccountname;   
        $_SESSION['telephonenumber'] = $telephonenumber; 
        
        $status = 'ok';
        $message =  "$user has been logged in";
        $data = $attributes;
        auditlog("Access",$user,"User has logged in"); 

        header("Location: ../../index.php");
        break;
    case 'logout':
        
        if (isset($_SESSION['username'])) {
            $user = $_SESSION['username'];
            unset($_SESSION['username']);
            unset($_SESSION['usershort']);
            unset($_SESSION['fullname']);
            unset($_SESSION['telephonenumber']);
            unset($_SESSION['mail']);
            unset($_SESSION['editmode']);
            $status = 'ok';
            $message = "$user has been logged out";
        }
        else {
            $status = 'error';
            $message = 'session not found';
        }
        break;

    #default:
    #    $status = 'error';
    #   $message = 'no mode set';
    #    header("Location: ../../index.php");
}

$output['message'] = $message;
$output['status']  = $status;
$output['data'] = $data;

echo json_encode($output);
?>