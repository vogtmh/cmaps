// Additional functions for the admin panel

function submitWhitelist(WLtype, WLtext) {
  console.log('add to whitelist: '+WLtext+', '+WLtype);
  document.getElementById("ignoreHealthType").value = WLtype;
  document.getElementById("ignoreHealthName").value = WLtext;
  document.getElementById('updateWhitelist').submit();
}

function updateHealthDetails() {
  $.ajax({
    url: '../rest/system?healthdetails=1',
    async: true, 
    type: 'get',
    dataType: 'JSON',
    beforeSend: function() {
      // Create two containers for LDAP and Desks
      var healthdetails = ''
      + '<div id="healthldap" style="width:780px; height:auto; float:left; margin-left:20px;">'
      + '<img src="../images/spinner.png" style="margin-left:262px;" />'
      + '</div>'
      + '<div id="healthdesks" style="width:780px; height:auto; float:right; margin-right:10px;">'
      + '<img src="../images/spinner.png" style="margin-left:262px;" />'
      + '</div>'

      var checkdiv = document.getElementById('healthdetails')
      if (checkdiv === null) {
        var root = document.getElementById('content')
        var newElement = document.createElement('div')
        newElement.setAttribute('id', 'healthdetails')
        newElement.innerHTML = healthdetails
        root.appendChild(newElement)  
      }
    },
    success: function(result){
      
      // Output errors in LDAP assignment on left tile
      var color='green'
      var ldaparray = result.health.ldap
      var percentage = ldaparray.length
      if (percentage >=30 ) {color='red';}
      else if (percentage >= 1) {color='orange';}
      else {color='green';}
      var healthldap = ''
      + '<div style="width:750px; height:150px; float:left; margin-left:20px; background:'+color+'; opacity:0.7; text-align:center;line-height:150px;">'
      + '<span style="display: inline-block; vertical-align: middle; line-height: normal;">'
      + '<h1>LDAP errors</h1><h2>'+percentage+'</h2>'
      + '</span>'
      + '</div>'
      for(var i = 0; i <ldaparray.length; i++) {
        healthldap += ''
        + '<div style="width:750px; height:70px; float:left; margin-left:20px; margin-top:5px; background:'+color+'; opacity:0.7; text-align:center;line-height:70px;">'
        + '<span style="float:left; vertical-align: middle; line-height: normal; width:650px; height:70px;">'
        + '<h2>'+ldaparray[i].desk+' assigned to '+ldaparray[i].count+' people: '+ldaparray[i].name+'</h2>'
        + '</span>'
        + '<a href="javascript:{}" onclick="submitWhitelist(\'ldap\',\''+ldaparray[i].desk+'\')">'
        + '<span style="float:left; background-color: #505050; vertical-align: middle; line-height: normal; width:100px; height:70px;">'
        + '<h2>ignore</h2>'
        + '</span>'
        + '</a>'
        + '</div>'
      }
      document.getElementById('healthldap').innerHTML= healthldap;
      
      // Output errors in desk database on right tile
      var color='green'
      var deskarray = result.health.desks
      var percentage = deskarray.length
      if (percentage >=5 ) {color='red';}
      else if (percentage >= 1) {color='orange';}
      else {color='green';}
      healthdesks = ''
      + '<div style="width:750px; height:150px; float:right; margin-right:10px; background:'+color+'; opacity:0.7; text-align:center;line-height:150px;">'
      + '<span style="display: inline-block; vertical-align: middle; line-height: normal;">'
      + '<h1>Desk errors</h1><h2>'+percentage+'</h2>'
      + '</span>'
      + '</div>'
      for(var i = 0; i <deskarray.length; i++) {
        healthdesks += ''
        + '<div style="width:750px; height:70px; float:left; margin-left:20px; margin-top:5px;background:'+color+'; opacity:0.7; text-align:center;line-height:70px;">'
        + '<span style="float:left; vertical-align: middle; line-height: normal; width:650px; height:70px;">'
        + '<h2>'+deskarray[i].desk+' exists '+deskarray[i].count+' times on map '+deskarray[i].map+'</h2>'
        + '</span>'
        + '<a href="javascript:{}" onclick="submitWhitelist(\'desks\',\''+deskarray[i].desk+'\')">'
        + '<span style="float:left; background-color: #505050; vertical-align: middle; line-height: normal; width:100px; height:70px;">'
        + '<h2>ignore</h2>'
        + '</span>'
        + '</a>'
        + '</div>'
      }
      document.getElementById('healthdesks').innerHTML= healthdesks;
      
      console.log('[HealthDetails] updated');
    }
  })
}

function updateSystemStats() {
  $.ajax({
    url: '../rest/system',
    async: true, 
    type: 'get',
    dataType: 'JSON',
    beforeSend: function() {
      var element = document.getElementById('systemstats');
      if (element === null) {
        var systemspinner = ''
        + '<div id="spinner" style="width:1600px; height:auto; float:left; margin-left:20px;">'
        + '<img src="../images/spinner.png" style="margin-left:672px;" />'
        + '</div>'

        var checkdiv = document.getElementById('systemspinner')
        if (checkdiv === null) {
          var root = document.getElementById('content')
          var newElement = document.createElement('div')
          newElement.setAttribute('id', 'systemspinner')
          newElement.innerHTML = systemspinner
          root.appendChild(newElement)  
        }
      }
    },
    success: function(result){

      var element = document.getElementById('systemspinner');
      if (element !== null) {
       element.parentNode.removeChild(element);
      }

      var color='green'
      var percentage = result.cpuload
      if (percentage >=95 ) {color='red';}
      else if (percentage >= 85) {color='orange';}
      else {color='green';}
      var systemstats = ''
      + '<div id="cpuload" style="width:300px; height:300px; float:left; margin-left:10px; margin-right:15px; background:'+color+'; opacity:0.7; text-align:center;line-height:300px;">'
      + '<span style="display: inline-block; vertical-align: middle; line-height: normal;">'
      + '<h1>CPU Load</h1><h2>'+percentage+'</h2>'
      + '</span>'
      + '</div>'

      var color='green'
      var percentage = result.memoryused
      if (percentage >=95 ) {color='red';}
      else if (percentage >= 85) {color='orange';}
      else {color='green';}
      systemstats += ''
      + '<div id="memoryused" style="width:300px; height:300px; float:left; margin-right:15px; background:'+color+'; opacity:0.7; text-align:center;line-height:300px;">'
      + '<span style="display: inline-block; vertical-align: middle; line-height: normal;">'
      + '<h1>Memory used</h1><h2>'+percentage+'</h2>'
      + '</span>'
      + '</div>'

      var color='green'
      var percentage = result.diskused
      if (percentage >=95 ) {color='red';}
      else if (percentage >= 85) {color='orange';}
      else {color='green';}
      systemstats += ''
      + '<div id="diskused" style="width:300px; height:300px; float:left; margin-right:15px; background:'+color+'; opacity:0.7; text-align:center;line-height:300px;">'
      + '<span style="display: inline-block; vertical-align: middle; line-height: normal;">'
      + '<h1>Disk used</h1><h2>'+percentage+'</h2>'
      + '</span>'
      + '</div>'

      var color='green'
      var percentage = result.consistency_ldap
      if (percentage >=30 ) {color='red';}
      else if (percentage >= 1) {color='orange';}
      else {color='green';}
      systemstats += ''
      + '<div id="consistency_ldap" style="width:300px; height:300px; float:left; margin-right:15px; background:'+color+'; opacity:0.7; text-align:center;line-height:300px;">'
      + '<span style="display: inline-block; vertical-align: middle; line-height: normal;">'
      + '<h1>LDAP Consistency Errors</h1><h2>'+percentage+'</h2>'
      + '</span>'
      + '</div>'

      var color='green'
      var percentage = result.consistency_desks
      if (percentage >=5 ) {color='red';}
      else if (percentage >= 1) {color='orange';}
      else {color='green';}
      systemstats += ''
      + '<div id="consistency_desks" style="width:300px; height:300px; float:left; margin-right:15px; background:'+color+'; opacity:0.7; text-align:center;line-height:300px;">'
      + '<span style="display: inline-block; vertical-align: middle; line-height: normal;">'
      + '<h1>Desks Consistency Errors</h1><h2>'+percentage+'</h2>'
      + '</span>'
      + '</div>'

      var element = document.getElementById('systemstats');
      if (element !== null) {
       element.parentNode.removeChild(element);
      }

      var p = document.getElementById('content')
      var newElement = document.createElement('div')
      newElement.setAttribute('id', 'systemstats')
      newElement.innerHTML = systemstats
      p.appendChild(newElement)

      console.log('[SystemStats] updated');
    }
  })
}

function syncLDAP(ldap_id, adminuser) {
  var button_div = 'syncbutton'+ldap_id
  console.log('Sync started for LDAP connection #'+ldap_id)
  $("#"+button_div).css("background-color","#404040");
  document.getElementById(button_div).value = "Syncing..."
  $.ajax({
    url: '../rest/ldap/?token='+token+'&ldapid='+ldap_id+'&user='+adminuser,
    async: true, 
    type: 'get',
    dataType: 'JSON',
    success: function(result){
      console.log('[LDAP] updated')
      console.log(result)
      $("#"+button_div).css("background-color","rgba(0, 100, 0, 1.0)");
      document.getElementById(button_div).value = "Success"
    },
    error: function() {
      console.log('[LDAP] update failed');
      $("#"+button_div).css("background-color","rgba(150, 0, 0, 1.0)");
      document.getElementById(button_div).value = "Error"
    }
  })
}

function showCharts(interval, divname) {

  // Check if canvas already exists or create one
  var checkcanvas = document.getElementById(divname);
  if (checkcanvas == null) {
    var p = document.getElementById('content');
    var newElement = document.createElement('div');
    newElement.setAttribute('id', divname+'_container');
    newElement.setAttribute('style', 'background:rgba(60,60,60,0.5);border-radius:5px;padding:15px;opacity:1.0;width:1560px;margin-left:10px;height:250px;margin-bottom:20px;');
    newElement.innerHTML = '<canvas id="'+divname+'" width="1500" height="230"></canvas>';
    p.appendChild(newElement);
  }

  $.ajax({
    
    // fetch data from stats API
    url: '../rest/stats/index.php?interval='+interval,
    async: true, 
    type: 'get',
    dataType: 'JSON',
    success: function(result){

      console.log('result for '+interval+':')
      console.log(result)
      // Copy items to comma-separated string for the chart
      var outlabels = result.reverse().map( function(item){ return item.period; });
      var outcount = result.map( function(item){ return item.count; });
      console.log('outlabels for '+interval+':')
      console.log(outlabels)
      console.log('outcount for '+interval+':')
      console.log(outcount)

      // Prepare chart
      var chartData = {
        labels : outlabels,
        datasets : [{
            borderColor: 'rgba(90, 190, 90,1.0)',
            backgroundColor:'rgba(90,190,90,0.5)',
            fill: true,
            tension: 0.4,
            pointRadius:5,
            pointHitRadius:10,
            data : outcount
        }]
      }
      var chartOptions = {
        scales: {
          x: {
            ticks: {
              color: 'rgba(255,255,255,1.0)'
            },
            grid: {
              color: 'rgba(255,255,255,0.5)'
            }
          },
          y: {
            ticks: {
              color: 'rgba(255,255,255,1.0)'
            },
            grid: {
              color: 'rgba(255,255,255,0.5)'
            }
          },
        },
        plugins: {
          legend: {
              display: false,
          }
        }
      }
      
      // Draw chart
      var ctx = document.getElementById(divname);
      new Chart(ctx, {type: 'line', data: chartData, options: chartOptions})

      console.log('Stats: Graph output for '+divname+' completed')
    },
    error: function()
    {
      console.log('Stats: Could not get data for '+divname+' from database.')
    }
  });
  
}

function ucWords (word) {
  word = word.toLowerCase().replace(/^[\u00C0-\u1FFF\u2C00-\uD7FF\w]|\s[\u00C0-\u1FFF\u2C00-\uD7FF\w]/g, function(letter) {
      return letter.toUpperCase();
  });
  return word;
}

function deskSummary(map) {

  $.ajax({
    url: '../rest/desks?map=' + map,
    async: true, 
    type: 'get',
    dataType: 'JSON',
    success: function(result){
      var output  = '<table border="0" style="width:470px; margin-left:30px;">'
                  + '<tr>'
                  + '<td style="font-weight: bold;color:lightgrey;text-align:left">'+ucWords(map)+'</td>'
                  + '<td style="width:130px"></td>'
                  + '<td style="width:130px"></td><td style="width:130px"></td>'
                  + '</tr>'
                  + '<tr>'
                  + '<td style="font-weight: bold;color:grey;text-align:left">Department</td>'
                  + '<td style="font-weight: bold;color:lightblue;width:130px;text-align:center;">Total</td>'
                  + '<td style="font-weight: bold;color:orange;width:130px;text-align:center;">In use</td>'
                  + '<td style="font-weight: bold;color:green;width:130px;text-align:center;">Free</td>'
                  + '</tr>'
                  +  '<tr><td>&nbsp;</td></tr>';
      // Output departments one by one
      $.each( departments, function( x, department ){
          var all = result.desks.filter(element => element.dept == department);
          var total1 = all.filter(element => element.desktype == 'addesk');
          var total2 = all.filter(element => element.desktype == 'blocked');
          var total3 = all.filter(element => element.desktype == 'hotseat');
          var totalcount = Object.keys(total1).length + Object.keys(total2).length + Object.keys(total3).length;
          var used1 = total1.filter(element => element.fname != '');
          var usedcount = Object.keys(used1).length + Object.keys(total2).length + Object.keys(total3).length;
          var freecount = totalcount - usedcount;
          output+='<tr>'
              + '<td style="color:grey;text-align:left">'+department+'</td>'
              + '<td style="color:lightblue;text-align:center;">'+totalcount+'</td>'
              + '<td style="color:orange;text-align:center;">'+usedcount+'</td>'
              + '<td style="color:green;text-align:center;">'+freecount+'</td>'
              + '</tr>';
      });
      var all = result.desks;
      var total1 = all.filter(element => element.desktype == 'addesk');
      var total2 = all.filter(element => element.desktype == 'blocked');
      var total3 = all.filter(element => element.desktype == 'hotseat');
      var total4 = all.filter(element => element.desktype == 'shareddesk');
      var totalcount = Object.keys(total1).length + Object.keys(total2).length + Object.keys(total3).length + Object.keys(total4).length;
      var used1 = all.filter(element => element.fname != '');
      var usedcount = Object.keys(used1).length + Object.keys(total2).length + Object.keys(total3).length + Object.keys(total4).length;
      var freecount = totalcount - usedcount;
      
      output+='<tr>'
              + '<td style="color:grey;text-align:left; font-weight:bold;">Summary</td>'
              + '<td style="color:lightblue; font-weight:bold;text-align:center;">'+totalcount+'</td>'
              + '<td style="color:orange; font-weight:bold;text-align:center;">'+usedcount+'</td>'
              + '<td style="color:green; font-weight:bold;text-align:center;">'+freecount+'</td>'
              + '</tr></table>';

      var statsoutput = document.getElementById(map);
      statsoutput.innerHTML = output;
      statsoutput.style.visibility = 'visible';
      console.log('[Desks] '+ map + ' updated');
    }    
  });

      
}