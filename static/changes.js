var root="index.php/"

// using the changes API to update the announcementbar
function updateChangesOverview() {
  let limit = $("#limit").val()
  $("#activity").show();
  $.ajax({
    url: 'rest/changes/?maxresults=' + limit,
    async: true,
    type: 'get',
    dataType: 'JSON',
    success: function(result){
        var outputstring = '';

        for (var i = 0; i < result.changes.length; i++) {
          var counter = result.changes[i];
          if (counter.type=='Title') {
            outputstring+='<a href="'+root+'?findme='+counter.fullname+'">'
                          +'<div class="announceplate" style="float:left">'
                          +'<div class="announceavatar" style="background-image: url(avatarcache/'+ counter.avatar + '.jpg), url(images/noavatar.png);"></div>'
                          +'<div class="announcetextbox">'
                          +'<div class="announcetext">'
                          +counter.fullname + '<br />' + counter.newvalue + '<br />'
                          +'<span style="text-decoration: line-through; color:#c0c0c0;">'+ counter.oldvalue + '</span>'
                          +'</div></div>'
                          +'<div class="announcedate" style="background-color:#393a3c;">' + counter.timestamp + '</div>'
                          +'<div class="announcetype" style="background-color:#0000CC;">Title</div>'
                          +'</div>'
                          +'</a>';
          }
          if (counter.type=='Employee') {
            outputstring+='<a href="'+root+'?findme='+counter.fullname+'">'
                          +'<div class="announceplate" style="float:left">'
                          +'<div class="announceavatar" style="background-image: url(avatarcache/'+ counter.avatar + '.jpg), url(images/noavatar.png);"></div>'
                          +'<div class="announcetextbox">'
                          +'<div class="announcetext">'
                          +counter.fullname + '<br />' + counter.newvalue + '<br />'
                          +'</div></div>'
                          +'<div class="announcedate" style="background-color:#393a3c;">' + counter.timestamp + '</div>'
                          +'<div class="announcetype" style="background-color:#00CC00;">New</div>'
                          +'</div>';
                          +'</a>';
          }

        }
        announceLive = result.changes[0].id;
        $("#announcementbar_body").html(outputstring);
        $("#activity").hide();
    }
  });
}
