<!-- ===================================================================
        
        CompanyMaps MapUploader v20170623
        Copyright (c) 2016-2017 by MavoDev
        see https://www.mavodev.de for more details
        
==================================================================== -->

<?php ini_set("memory_limit", "200000000"); // for large images so that we do not get "Allowed memory exhausted"?>
<?php

# Loading shared functions and config file
include '../shared.php';

session_start();
if ($_SESSION['editmode'] <> '' && $_SESSION['editmode'] == base64_encode($_SESSION['username'])) {print 'auth successful<br />'; } else {print 'no auth done';}

// upload the file
if ((isset($_POST["submitted_form"])) && ($_POST["submitted_form"] == "image_upload_form")) {
	
	// file needs to be jpg,gif,bmp,x-png and 10 MB max
	if (($_FILES["image_upload_box"]["type"] == "image/jpeg" || $_FILES["image_upload_box"]["type"] == "image/pjpeg" || $_FILES["image_upload_box"]["type"] == "image/gif" || $_FILES["image_upload_box"]["type"] == "image/x-png") && ($_FILES["image_upload_box"]["size"] < 10000000))
	{
		
  
		// some settings
		$max_upload_width = $targetScreenWidth;

		//$max_upload_width = $targetWidth;	  
		
		// if uploaded image was JPG/JPEG
		if($_FILES["image_upload_box"]["type"] == "image/jpeg" || $_FILES["image_upload_box"]["type"] == "image/pjpeg"){	
			$image_source = imagecreatefromjpeg($_FILES["image_upload_box"]["tmp_name"]);
		}		
		// if uploaded image was GIF
		if($_FILES["image_upload_box"]["type"] == "image/gif"){	
			$image_source = imagecreatefromgif($_FILES["image_upload_box"]["tmp_name"]);
		}	
		// BMP doesn't seem to be supported so remove it form above image type test (reject bmps)	
		// if uploaded image was BMP
		if($_FILES["image_upload_box"]["type"] == "image/bmp"){	
			$image_source = imagecreatefromwbmp($_FILES["image_upload_box"]["tmp_name"]);
		}			
		// if uploaded image was PNG
		if($_FILES["image_upload_box"]["type"] == "image/x-png"){
			$image_source = imagecreatefrompng($_FILES["image_upload_box"]["tmp_name"]);
		}
		

		$remote_file = "image_files/".$_FILES["image_upload_box"]["name"];
		imagejpeg($image_source,$remote_file,100);
		chmod($remote_file,0644);
	
	

		// get width and height of original image
		list($image_width, $image_height) = getimagesize($remote_file);
	
		if($image_width<>$max_upload_width){
			$proportions = $image_width/$image_height;
			
			$new_width = $max_upload_width;
			$new_height = round($max_upload_width/$proportions);		
			
			
			$new_image = imagecreatetruecolor($new_width , $new_height);
			$image_source = imagecreatefromjpeg($remote_file);
			
			imagecopyresampled($new_image, $image_source, 0, 0, 0, 0, $new_width, $new_height, $image_width, $image_height);
			imagejpeg($new_image,$remote_file,100);
			
			imagedestroy($new_image);
		}
		
		imagedestroy($image_source);
		
		
		header("Location: mapupload.php?upload_message=image uploaded&upload_message_type=success&show_image=".$_FILES["image_upload_box"]["name"]);
		exit;
	}
	else{
		header("Location: mapupload.php?upload_message=make sure the file is jpg, gif or png and that is smaller than 10MB&upload_message_type=error");
		exit;
	}
}
?>

<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
<title>CompanyMaps - Map Upload</title>
<style type="text/css">
<!--
body,td,th {
	font-family: Arial, Helvetica, sans-serif;
	color: #333333;
	font-size: 12px;
}

.upload_message_success {
	padding:4px;
	background-color:#009900;
	border:1px solid #006600;
	color:#FFFFFF;
	margin-top:10px;
	margin-bottom:10px;
}

.upload_message_error {
	padding:4px;
	background-color:#CE0000;
	border:1px solid #990000;
	color:#FFFFFF;
	margin-top:10px;
	margin-bottom:10px;
}

-->
</style></head>

<body>

<h1 style="margin-bottom: 0px">Submit an image</h1>


        <?php if(isset($_REQUEST['upload_message'])){?>
            <div class="upload_message_<?php echo $_REQUEST['upload_message_type'];?>">
            <?php echo htmlentities($_REQUEST['upload_message']);?>
            </div>
		<?php }?>


<form action="mapupload.php" method="post" enctype="multipart/form-data" name="image_upload_form" id="image_upload_form" style="margin-bottom:0px;">
<label>Image file, maximum 10MB. it can be jpg, gif,  png:</label><br />
          <input name="image_upload_box" type="file" id="image_upload_box" size="40" />
          <input type="submit" name="submit" value="Upload map" />     
      <br />
<input name="submitted_form" type="hidden" id="submitted_form" value="image_upload_form" />
</form>




<?php if(isset($_REQUEST['show_image']) and $_REQUEST['show_image']!=''){?>
<p><img src="image_files/<?php echo $_REQUEST['show_image'];?>" /></p>
<?php }?>



</body>
</html>


