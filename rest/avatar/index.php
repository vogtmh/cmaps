<?php
  session_start();
  $unixtimestamp = time();
  if (!isset($_POST['mode'])) {
    echo 'Mode not set.';
    exit;
  }
  $mode=$_POST['mode'];
  if (!isset($_POST['mode'])) {
    echo 'Not logged in.';
    exit;
  }
  $userfull=$_SESSION['username'];
  $userarray=explode("\\",$userfull);
  $userid=$userarray[1];

  $moveDirTMP = "../../tmp/";
  $targetDirTMP = "tmp/";
  $moveDir = "../../avatarcache/";
  $targetDir = "avatarcache/";
  $moveFilePath = $moveDir . $userid . '.jpg';
  $moveFilePathTMP = $moveDirTMP . $userid . '.jpg';
  $targetFilePath = $targetDir . $userid . '.jpg';
  $targetFilePathTMP = $targetDirTMP . $userid . '.jpg';

#########################################################################################################
# CONSTANTS																								#
# You can alter the options below																		#
#########################################################################################################
$temp_dir = "tmp";
$temp_path = "${temp_dir}/";
$upload_dir = "upload_pic"; 				// The directory for the images to be saved in
$upload_path = $upload_dir."/";				// The path to where the image will be saved
$large_image_prefix = "resize_"; 			// The prefix name to large image
$thumb_image_prefix = "thumbnail_";			// The prefix name to the thumb image
#$large_image_name = $username.$_SESSION['random_key'];     // New name of the large image (append the timestamp to the filename)
$thumb_image_name = $username;     // New name of the thumbnail image (append the timestamp to the filename)
$max_file = "10"; 							// Maximum file size in MB
$max_width = "600";							// Max width allowed for the large image
$max_height = "600";
$thumb_width = "400";						// Width of thumbnail image
$thumb_height = "400";						// Height of thumbnail image
// Only one of these image types should be allowed for upload
$allowed_image_types = array('image/pjpeg'=>"jpg",'image/jpeg'=>"jpg",'image/jpg'=>"jpg",'image/png'=>"png",'image/x-png'=>"png",'image/gif'=>"gif");
$allowed_image_ext = array_unique($allowed_image_types); // do not change this
$image_ext = "";	// initialise variable, do not change this.
foreach ($allowed_image_ext as $mime_type => $ext) {
    $image_ext.= strtoupper($ext)." ";
}

##########################################################################################################
# IMAGE FUNCTIONS																						 #
# You do not need to alter these functions																 #
##########################################################################################################
function resizeImage($image,$width,$height,$scale) {
	list($imagewidth, $imageheight, $imageType) = getimagesize($image);
	$imageType = image_type_to_mime_type($imageType);
	switch($imageType) {
		case "image/gif":
			$source=imagecreatefromgif($image); 
			break;
	    case "image/pjpeg":
		case "image/jpeg":
		case "image/jpg":
			$source=imagecreatefromjpeg($image); 
			break;
	    case "image/png":
		case "image/x-png":
			$source=imagecreatefrompng($image); 
			break;
  	}
	// Check if picture has rotation tag in EXIF data
	// Rotate picture if needed

	$exif = exif_read_data($image);
	if(!empty($exif['Orientation'])) {
	switch($exif['Orientation']) {
	case 8:
		$source = imagerotate($source,90,0);
		$w1 = $width;
		$h1 = $height;
		$height = $w1;
		$width = $h1;
		break;
	case 3:
		$source = imagerotate($source,180,0);
		break;
	case 6:
		$source = imagerotate($source,-90,0);
		$w1 = $width;
		$h1 = $height;
		$height = $w1;
		$width = $h1;
		break;
	} 
	}


	//-------------------------
	$newImageWidth = ceil($width * $scale);
	$newImageHeight = ceil($height * $scale);
	$newImage = imagecreatetruecolor($newImageWidth,$newImageHeight);
	
	imagecopyresampled($newImage,$source,0,0,0,0,$newImageWidth,$newImageHeight,$width,$height);

	switch($imageType) {
		case "image/gif":
	  		imagejpeg($newImage,$image); 
			break;
      	case "image/pjpeg":
		case "image/jpeg":
		case "image/jpg":
	  		imagejpeg($newImage,$image,90); 
			break;
		case "image/png":
		case "image/x-png":
			imagejpeg($newImage,$image,90);
			break;
    }
	
	chmod($image, 0777);
	return $image;
}

function CreateThumbnail($image,$thumb,$thumbwidth, $quality = 100)
{

        $source=ImageCreateFromJPEG($image);

        //if(function_exists("exif_read_data")){
                $exif = exif_read_data($image);
                if(!empty($exif['Orientation'])) {
                switch($exif['Orientation']) {
                case 8:
                    $source = imagerotate($source,90,0);
                    break;
                case 3:
                    $source = imagerotate($source,180,0);
                    break;
                case 6:
                    $source = imagerotate($source,-90,0);
                    break;
                } 
                }
        //}
        $info = @getimagesize($image);

        $width = $info[0];

        $w2=ImageSx($im1);
        $h2=ImageSy($im1);
        $w1 = ($thumbwidth <= $info[0]) ? $thumbwidth : $info[0]  ;

        $h1=floor($h2*($w1/$w2));
        $im2=imagecreatetruecolor($w1,$h1);

        imagecopyresampled ($im2,$im1,0,0,0,0,$w1,$h1,$w2,$h2); 
        $path=addslashes($thumb);
        ImageJPEG($im2,$path,$quality);
        ImageDestroy($im1);
        ImageDestroy($im2);
}


//You do not need to alter these functions
function resizeThumbnailImage($thumb_image_name, $image, $width, $height, $start_width, $start_height, $scale){
	list($imagewidth, $imageheight, $imageType) = getimagesize($image);
	$imageType = image_type_to_mime_type($imageType);
	
	$newImageWidth = ceil($width * $scale);
	$newImageHeight = ceil($height * $scale);
	$newImage = imagecreatetruecolor($newImageWidth,$newImageHeight);
	switch($imageType) {
		case "image/gif":
			$source=imagecreatefromgif($image); 
			break;
	    case "image/pjpeg":
		case "image/jpeg":
		case "image/jpg":
			$source=imagecreatefromjpeg($image); 
			break;
	    case "image/png":
		case "image/x-png":
			$source=imagecreatefrompng($image); 
			break;
  	}
	imagecopyresampled($newImage,$source,0,0,$start_width,$start_height,$newImageWidth,$newImageHeight,$width,$height);
	switch($imageType) {
		case "image/gif":
	  		imagegif($newImage,$thumb_image_name); 
			break;
      	case "image/pjpeg":
		case "image/jpeg":
		case "image/jpg":
	  		imagejpeg($newImage,$thumb_image_name,90); 
			break;
		case "image/png":
		case "image/x-png":
			imagejpeg($newImage,$thumb_image_name,90);
			break;
    }
	chmod($thumb_image_name, 0777);
	return $thumb_image_name;
}
//You do not need to alter these functions
function getHeight($image) {
	$size = getimagesize($image);
	$height = $size[1];
	return $height;
}
//You do not need to alter these functions
function getWidth($image) {
	$size = getimagesize($image);
	$width = $size[0];
	return $width;
}

//Create the upload directory with the right permissions if it doesn't exist
if(!is_dir("../../$upload_dir")){
	mkdir("../../$upload_dir", 0777);
	chmod("../../$upload_dir", 0777);
}

if(!is_dir("../../$temp_dir")){
	mkdir("../../$temp_dir", 0777);
	chmod("../../$temp_dir", 0777);
}

## Start of API

  if ($mode != '' && $userfull != '') {
    switch ($mode) {
      case 'upload':
        if(!empty($_FILES['images'])){
          // File upload configuration

          $allowTypes = array('jpg','png','jpeg','gif');
          $uploaded_image = '';
          foreach($_FILES['images']['name'] as $key=>$val){
            $image_name = $_FILES['images']['name'][$key];
            $tmp_name = $_FILES['images']['tmp_name'][$key];
            $size = $_FILES['images']['size'][$key];
            $type = $_FILES['images']['type'][$key];
            $error = $_FILES['images']['error'][$key];

            // File upload path
            $fileName = basename($_FILES['images']['name'][$key]);

            // Check whether file type is valid
            $fileType = pathinfo($targetFilePathTMP,PATHINFO_EXTENSION);
            if(in_array($fileType, $allowTypes)){
              // Store images on the server
              if(move_uploaded_file($_FILES['images']['tmp_name'][$key],$moveFilePathTMP)){
                $uploaded_image = $targetFilePathTMP;
                $width = getWidth($moveFilePathTMP);
                $height = getHeight($moveFilePathTMP);
                //Scale the image if it is greater than the width set above
                if ($width > $max_width){
                        $scale = $max_width/$width;
                        $uploaded = resizeImage($moveFilePathTMP,$width,$height,$scale);
                }
                $width = getWidth($moveFilePathTMP);
                $height = getHeight($moveFilePathTMP);
                if ($height > $max_height){
                  $scale = $max_height/$height;
                  $uploaded = resizeImage($moveFilePathTMP,$width,$height,$scale);
                }
              }
            }
          }
          // Generate gallery view of the images
          if($uploaded_image != ''){
              #echo "<img src='${uploaded_image}?time=${unixtimestamp}' style='width:80px; height:80px;' />";
              #if (file_exists($moveFilePathTMP)) {
                $current_large_image_width = getWidth($moveFilePathTMP);
                $current_large_image_height = getHeight($moveFilePathTMP);
                if ($current_large_image_width == $current_large_image_height) {
                  $shorterside=$current_large_image_width;
                  $x1 = 0; $y1 = 0; $x2 = $shorterside; $y2 = $shorterside;
                }
                if ($current_large_image_width < $current_large_image_height) {
                  $shorterside=$current_large_image_width;
                  $x1 = 0; $y1 = round(($current_large_image_height-$current_large_image_width)/2);
                  $x2 = $shorterside; $y2 = ($y1+$shorterside);
                }
                if ($current_large_image_width > $current_large_image_height) {
                  $shorterside=$current_large_image_height;
                  $x1 = round(($current_large_image_width-$current_large_image_height)/2); $y1 = 0;
                  $x2 = ($x1+$shorterside); $y2 = $shorterside;
                }
                
                echo "
                <script src='../../tools/jquery.imgareaselect.min.js'></script>
                <script type='text/javascript'>
                function preview(img, selection) { 
                  console.log(img);
                  console.log(selection);
                  var scaleX = $thumb_width / selection.width; 
                  var scaleY = $thumb_height / selection.height; 
                  
                  $('#thumbnail + div > img').css({ 
                    width: Math.round(scaleX * $current_large_image_width) + 'px', 
                    height: Math.round(scaleY * $current_large_image_height) + 'px',
                    marginLeft: '-' + Math.round(scaleX * selection.x1) + 'px', 
                    marginTop: '-' + Math.round(scaleY * selection.y1) + 'px' 
                  });
                  $('#x1').val(selection.x1);
                  $('#y1').val(selection.y1);
                  $('#x2').val(selection.x2);
                  $('#y2').val(selection.y2);
                  $('#w').val(selection.width);
                  $('#h').val(selection.height);
                } 
      
                $('#thumbnail').imgAreaSelect({ x1 : $x1, y1 : $y1, x2 : $x2, y2: $y2, aspectRatio: '1:1', onSelectChange: preview }); 
      
                </script>
                
                <h2>Select the area for the profile picture</h2>
                <form name='thumbnail' id='resizeForm' enctype='multipart/form-data' style='width:18em;position:relative; float:left;visibility:hidden;display:none;'>
                    <input type='hidden' name='x1' value='' id='x1' />
                    <input type='hidden' name='y1' value='' id='y1' />
                    <input type='hidden' name='x2' value='' id='x2' />
                    <input type='hidden' name='y2' value='' id='y2' />
                    <input type='hidden' name='w' value='' id='w' />
                    <input type='hidden' name='h' value='' id='h' />
                    <input type='hidden' name='mode' value='crop'>
                    <input type='submit' name='upload_thumbnail' value='Create profile picture' id='save_thumb' />
                </form>
                <div style='position:absolute; right: 40px; bottom: 130px; width:80px; height:80px;background: transparent;cursor:pointer;'>
                  <img src='images/avatar-cancel.png' style='width:100%;height:100%;' alt='' onclick=\"cancelAvatar()\" onmouseover=this.src='images/avatar-cancel_on.png' onmouseout=this.src='images/avatar-cancel.png' />
                </div>
                <div style='position:absolute; right: 40px; bottom: 20px; width:80px; height:80px;background: transparent;cursor:pointer;'>
                  <img src='images/avatar-save.png' style='width:100%;height:100%;' alt='' onclick=\"document.getElementById('save_thumb').click();\" onmouseover=this.src='images/avatar-save_on.png' onmouseout=this.src='images/avatar-save.png' />
                </div>
                <br style='clear:both;'/>
                <div style='text-align:center;'>
                  <img src='$targetFilePathTMP?time=$unixtimestamp' style='float: left; margin-right: 10px; z-index:2000;' id='thumbnail' alt='Create Thumbnail' />
                  <div style='visibility:hidden;zindex:1; float:left; position:fixed; top:40%;left:$max_width px; overflow:hidden; width:$thumb_width px; height:$thumb_height px;'>
                    <img src='$targetFilePathTMP?time=$unixtimestamp' style='position: relative;' alt='Thumbnail Preview' />
                  </div>
                  
                </div>

                <script>
                preview('thumb',{
                  'x1': $x1,
                  'y1': $y1,
                  'x2': $x2,
                  'y2': $y2,
                  'width': $current_large_image_width,
                  'height': $current_large_image_height
                })
                function cancelAvatar() {
                  $('#resizeForm')[0].reset();
                  $( '#image_resize' ).remove();
                  $( '.imgareaselect-outer' ).remove();
                  $( '.imgareaselect-selection' ).remove();
                  $( '.imgareaselect-border1' ).remove();
                  $( '.imgareaselect-border2' ).remove();
                }
                $('#resizeForm').on('submit', function(e){
                  e.preventDefault();
                  var x1 = $('#x1').val();
                  var y1 = $('#y1').val();
                  var x2 = $('#x2').val();
                  var y2 = $('#y2').val();
                  var w = $('#w').val();
                  var h = $('#h').val();
                  if(x1=='' || y1=='' || x2=='' || y2=='' || w=='' || h==''){
                    alert('Please select an area in your image');
                  }
                  else {
                    $.ajax({
                      type: 'POST',
                      url: 'rest/avatar/index.php',
                      data: new FormData(this),
                      contentType: false,
                      cache: false,
                      processData:false,
                      beforeSend: function(){
                        //$('#uploadStatus').html('<img src=\"images/uploading.gif\"/>');
                      },
                      error:function(){
                        //$('#uploadStatus').html('<span style='color:#EA4335;'>Deletion failed, please try again.<span>');
                      },
                      success: function(data){
                        $('#resizeForm')[0].reset();
                        $('#avatarbutton').html(data);
                        $( '#image_resize' ).remove();
                        $( '.imgareaselect-outer' ).remove();
                        $( '.imgareaselect-selection' ).remove();
                        $( '.imgareaselect-border1' ).remove();
                        $( '.imgareaselect-border2' ).remove();
                      }
                    });
                  }
                  
                  
                });
                </script>
                ";
              #}
          }
        }
        break;

      case 'delete':
        unlink($moveFilePath);
        echo "<img src='images/noavatar.png' style='width:80px; height:80px;' />";
        break;
      
      case 'crop':
        //Get the new coordinates to crop the image.
        $x1 = $_POST["x1"];
        $y1 = $_POST["y1"];
        $x2 = $_POST["x2"];
        $y2 = $_POST["y2"];
        $w = $_POST["w"];
        $h = $_POST["h"];
        //Scale the image to the thumb_width set above
        $scale = $thumb_width/$w;
        $cropped = resizeThumbnailImage($moveFilePath, $moveFilePathTMP,$w,$h,$x1,$y1,$scale);
        echo "<img src='${targetFilePath}?time=${unixtimestamp}' style='width:80px; height:80px;' />";
        break;

      default:
        echo "mode not found.";
        break;
    }
  }
?>