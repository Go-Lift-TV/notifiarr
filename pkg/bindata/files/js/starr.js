// Remove an instance from a list of applications.
// Example: Click delete button on one of the Radarrs.
// Works for Snapshot and Download Clients too.
function removeInstance(section, app, index)
{
    $('#'+ section +'-'+ app +'-'+ index).remove();
    // redo tooltips since some got nuked.
    setTooltips();
    // if all instances are deleted, show the "no instances" item.
    if (!$('.'+ section +'-'+ app).length) {
        $('#'+ section +'-'+ app +'-none').show();
    }
    // mark this instance as deleted (to bring up save changes button).
    $('.'+ app + index +'-deleted').val(true);
    // bring up the delete button on the next last row-item.
    $('.'+ section +'-'+ app +'-deletebutton').last().show();
    // bring up the save changes button.
    findPendingChanges();
}
// -------------------------------------------------------------------------------------------

function addStarrInstance(app)
{
     //-- DO NOT RELY ON 'index' FOR DIRECT IMPLEMENTATION, USE IT TO SORT AND RE-INDEX DURING THE SAVE
    const index = $('.starr-'+ app).length;
    const instance = index+1;
    const titleCase = titleCaseWord(app);
    const row = '<tr class="starr-'+ app +'" id="starr-'+ app +'-'+ instance +'">'+
                '   <td style="font-size: 22px;">'+instance+
                '       <div class="starr-'+ app +'-deletebutton" style="float: right; display:none;">'+
                '           <button onclick="removeInstance(\'starr\', \''+ app +'\', '+ instance +')" type="button" title="Delete this instance of '+ titleCase +'" class="delete-item-button btn btn-danger btn-sm"><i class="fa fa-minus"></i></button>'+
                '       </div>'+
                '   </td>'+
                '   <td><input type="text" id="Apps.'+ titleCase +'.'+ index +'.Name" class="client-parameter" data-group="starr" data-label="'+ titleCase +' '+ instance +' Name" data-original="" value="" style="width: 100%;"></td>'+
                '   <td><input type="text" id="Apps.'+ titleCase +'.'+ index +'.URL" class="client-parameter" data-group="starr" data-label="'+ titleCase +' '+ instance +' URL" data-original="" value="http://changeme" style="width: 100%;"></td>'+
                '   <td><input type="text" id="Apps.'+ titleCase +'.'+ index +'.APIKey" class="client-parameter" data-group="starr" data-label="'+ titleCase +' '+ instance +' API Key" data-original="" value="" style="width: 100%;"></td>'+
                '   <td><input type="text" id="Apps.'+ titleCase +'.'+ index +'.Username" class="client-parameter" data-group="starr" data-label="'+ titleCase +' '+ instance +' Username" data-original="" value="" style="width: 100%;"></td>'+
                '   <td><input type="text" id="Apps.'+ titleCase +'.'+ index +'.Password" class="client-parameter" data-group="starr" data-label="'+ titleCase +' '+ instance +' Password" data-original="" value="" style="width: 100%;"></td>'+
                '   <td><input type="text" id="Apps.'+ titleCase +'.'+ index +'.Interval" class="client-parameter" data-group="starr" data-label="'+ titleCase +' '+ instance +' Interval" data-original="5m" value="5m" style="width: 100%;"></td>'+
                '   <td><input type="text" id="Apps.'+ titleCase +'.'+ index +'.Timeout" class="client-parameter" data-group="starr" data-label="'+ titleCase +' '+ instance +' Timeout" data-original="1m" value="1m" style="width: 100%;"></td>'+
                '</tr>';
    $('#starr-'+ app +'-container').append(row);
    // hide all delete buttons, and show only the one we just added.
    $('.starr-'+app+'-deletebutton').hide().last().show();
    // redo tooltips since some got added.
    setTooltips();
    // hide the "no instances" item that displays if no instances are configured.
    $('#starr-'+ app +'-none').hide();
    // bring up the save changes button.
    findPendingChanges();
}
// -------------------------------------------------------------------------------------------


// This works like the above, except it grabs a list of names from a data atribute.
// The list is used to build the table rows.
function addOpaqueInstance(section, app)
{
     //-- DO NOT RELY ON 'index' FOR DIRECT IMPLEMENTATION, USE IT TO SORT AND RE-INDEX DURING THE SAVE
    const index = $('.'+ section +'-'+ app).length;
    const instance = index+1;
    const prefix = $('#'+ section +'-'+ app +'-addbutton').data('prefix');  // storage location like Apps or Snapshot.
    const names = $('#'+ section +'-'+ app +'-addbutton').data('names'); // list of thinges like "Name" and "URL"
    let row = '<tr class="'+ section +'-'+ app +'" id="'+ section +'-'+ app +'-'+ instance +'">'+
                '   <td style="font-size: 22px;">'+instance+
                '       <div class="'+ section +'-'+ app +'-deletebutton" style="float: right;">'+
                '           <button onclick="removeInstance(\''+ section +'\', \''+ app +'\', '+ instance +
                            ')" type="button" title="Delete this instance of '+ app +
                            '" class="delete-item-button btn btn-danger btn-sm"><i class="fa fa-minus"></i></button>'+
                '       </div>'+
                '   </td>';

    // Timeout and Interval must have valid go durations, or the parser errors.
    // Host or URL are set without an original value to make the save button appear.
    let colspan = 1
    for (const name of names) {
        let nameval = ""
        let nameori = ""
        if (name == "") {
            colspan++
            continue
        }

        switch (name) {
            case "Config.Interval":
            case "Interval":
             nameval = "5m";
             nameori = "5m";
             break;
             case "Config.Timeout":
             case "Timeout":
             nameval = "1m";
             nameori = "1m";
             break;
             case "Config.URL":
             case "URL":
             case "Host":
             nameval = "changeme";
             nameori = "";
             break;
        }

        row +=  '<td colspan="'+ colspan +'"><input type="text" id="'+ prefix +'.'+ app +'.'+ index +'.'+ name +
        '" class="client-parameter" data-group="'+ section +'" data-label="'+ app +' '+
         instance +' '+name+'" data-original="'+ nameori +'" value="'+ nameval +'" style="width: 100%;"></td>';
         colspan = 1
    }


    $('#'+ section +'-'+ app +'-container').append(row);
    // hide all delete buttons, and show only the one we just added.
    $('.'+ section +'-'+app+'-deletebutton').hide().last().show();
//    $('.'+ section +'-delete-'+app+'-button').hide().last().show();
    // redo tooltips since some got added.
    setTooltips();
    // hide the "no instances" item that displays if no instances are configured.
    $('#'+ section +'-'+ app +'-none').hide();
    // bring up the save changes button.
    findPendingChanges();
}
// -------------------------------------------------------------------------------------------
// -------------------------------------------------------------------------------------------
