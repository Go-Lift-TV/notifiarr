// Remove an instance from a list of applications.
// Example: Click delete button on one of the Radarrs.
// Works for Snapshot and Download Clients too.
function removeInstance(name, index)
{
    // service table has to be handled specially.
    serviceTable.row($('#'+ name +'-'+ index)).remove();
    $('#'+ name +'-'+ index).fadeOut(1000);
    setTimeout(function() {
        serviceTable.draw();
        $('#'+ name +'-'+ index).remove();
        // if all instances are deleted, show the "no instances" item.
        if (!$('.'+ name).length) {
            $('#'+ name +'-none').show();
        }
        // mark this instance as deleted (to bring up save changes button).
        $('.'+ name + index +'-deleted').val(true);

        //-- FIX THE INDEXING
        reindexList(name.split('-')[0]);

        // bring up the save changes button.
        findPendingChanges();
    },1000);
}




function testInstance(from, instanceType, index)
{
    from.css({'pointer-events':'none'}).find('i').toggleClass('fa-cog fa-spin fa-check-double');
    let fields = '';
    from.closest('tr').find('.client-parameter').each(function() {
        const id = $(this).attr('id')
        if (id !== undefined) {
            fields += '&' + $(this).serialize();
        }
    });

    $.ajax({
        type: 'POST',
        url: URLBase+'checkInstance/'+instanceType+"/"+index,
        data: fields,
        success: function (data){
            from.css({'pointer-events':'auto'}).find('i').toggleClass('fa-cog fa-spin fa-check-double');
            toast(instanceType+' Check Successful', data, 'success');
        },
        error: function (response, status, error) {
            from.css({'pointer-events':'auto'}).find('i').toggleClass('fa-cog fa-spin fa-check-double');
            if (response.responseText === undefined) {
                toast('Web Server Error',
                    'Notifiarr client appears to be down! Hard refresh recommended.', 'error', 30000);
            } else {
                toast(instanceType+' Check Error', response.responseText, 'error', 15000);
            }
        }
    });
}

// The Go app only accepts indexes on lists starting at 0 with no gaps.
// It looks better on screen to see incremental numbers.
// This procedure fixes the numbering on each row when an item is deleted.
function reindexList(group)
{
    let counter = 0;
    let currentIndex = 0;
    let currentApp = '';


    // Loop each form element with this data group and with a non-empty id.
    $('[data-group='+ group+'][id]').each(function(index) {
        const rowIndex = $(this).data("index");
        const itemApp = $(this).data("app");

        if (itemApp !== currentApp) {
            // We get into here when the app (in the group) changes, and on the very first element.
            counter = 0;
            currentIndex = 0;
        } else if (rowIndex !== currentIndex) {
            // We get into here on any new element in the same app.
            counter++;
        }

        //-- CHANGE id ATTR ON LABEL
        $('#'+ itemApp +'IndexLabel'+ rowIndex).attr('id', itemApp +'IndexLabel'+ counter).html(counter + 1);

        //-- CHANGE data-label ON INPUT FIELDS
        $(this).data('label', $(this).data('label').replace((rowIndex + 1), (counter + 1)));

        //-- CHANGE id AND name ATTR ON INPUT FIELDS
        $(this).attr('id', $(this).attr('id').replace(rowIndex, counter));
        $(this).attr('name', $(this).attr('id')); // these are the same.

        //-- SET CURRENT VARIABLES
        $(this).data("index", counter)
        currentIndex = rowIndex;
        currentApp = itemApp;
    });
}

// addInstance is used to build the table rows.
function addInstance(section, app)
{
     //-- DO NOT RELY ON 'index' FOR DIRECT IMPLEMENTATION, USE IT TO SORT AND RE-INDEX DURING THE SAVE
    const index = $('.'+ section +'-'+ app).length;
    const instance = index+1;
    const prefix = $('#'+ section +'-'+ app +'-addbutton').data('prefix');  // storage location like Apps or Snapshot.
    const names = $('#'+ section +'-'+ app +'-addbutton').data('names'); // list of thinges like "Name" and "URL"
    const sslname = $('#'+ section +'-'+ app +'-addbutton').data('sslname'); // name for ValidSSL input.
    // This just sets the first few lines of the row (the action buttons). The more-dynamic bits get added below, in a for loop.
    let row = '<tr class="newRow bk-success '+ section +'-'+ app +'" id="'+ section +'-'+ app +'-'+ instance +'">'+
    '<td style="white-space:nowrap;">'+
        '<div class="btn-group" role="group" style="display:flex;">'+
            '<button onclick="removeInstance(\''+ section +'-'+ app +'\', '+ instance +')" type="button" class="delete-item-button btn btn-danger btn-sm" style="font-size:18px;width:35px;"><i class="fa fa-minus"></i></button>'+
            '<button id="'+ app +'IndexLabel'+ index +'" class="btn btn-sm" style="font-size:18px;width:35px;pointer-events:none;">'+ instance +'</button>'+
            '<button onClick="testInstance($(this), \''+app+'\', \''+index+'\')" type="button" style="font-size:18px;" class="btn btn-success btn-sm"><i class="fas fa-check-double"></i></button>'+
        '</div>'+
    '</td>';

    // Timeout and Interval must have valid go durations, or the parser errors.
    // Host or URL are set with a value, and without an original value to make the save button appear.
    let colspan = 1;
    for (const name of names) {
        let nameval = ""
        let nameori = ""
        let extra = ""
        let itype = "<input type=\"text\""

        // Blank items in the name list add a colspan to the next item.
        if (name == "") {
            colspan++;
            continue;
        }

        switch (name) {
            case "Config.Password":
            case "Config.Pass":
            case "Config.APIKey":
            case "Pass":
            case "Password":
            case "APIKey":
                extra = '<div style="width:35px; max-width:35px;" class="input-group-addon input-sm" onClick="togglePassword(\''+ prefix +'.'+ app +'.'+ index +'.'+ name + '\', $(this).find(\'i\'));"><i class="fas fa-low-vision secret-input"></i></div>';
                itype = "<input type=\"password\"";
                break;
            case "Config.Interval":
            case "Interval":
                nameori = "5m";
                itype = "<select type=\"select\""
                extra = '<option value="-1s">Disabled</option>'+
                '<option value="45s">45 seconds</option>'+
                '<option value="1m">1 minute</option>'+
                '<option value="1m15s">1 min 15 sec</option>'+
                '<option value="1m30s">1 min 30 sec</option>'+
                '<option value="2m">2 minutes</option>'+
                '<option value="2m30s">2 min 30 sec</option>'+
                '<option value="3m">3 minutes</option>'+
                '<option value="4m">4 minutes</option>'+
                '<option selected value="5m">5 minutes</option>'+
                '<option value="10m">10 minutes</option>'+
                '<option value="15m">15 minutes</option>'+
                '<option value="30m">30 minutes</option>'+
                '<option value="45m">45 minutes</option>'+
                '<option value="60m">60 minutes</option>'+
                '</select>';
             break;
             case "Config.Timeout":
             case "extraConfig.Timeout":
             case "Timeout":
                nameori = "1m";
                itype = "<select type=\"select\""
                extra = '<option value="-1s">Disabled</option><option value="0s">No Timeout</option><option selected value="1s">1 second</option>';
                for (i = 2 ; i < 60 ; i++) {
                    extra += '<option value="'+i+'s">'+i+' seconds</option>';
                }
                extra += '<option selected value="1m">1 minute</option>';
                for (i = 1 ; i < 60 ; i++) {
                    extra += '<option value="1m'+i+'s">1 min '+i+' sec</option>';
                }
                extra += '</select>';
             break;
             case "Config.URL":
             case "URL":
                itype = '<input type="text" onChange="showhttps($(this).val(), \'#'+app+index+'SSL\');"';
                extra = '<div style="width:30px; max-width:30px;display:none" id="'+app+index+'SSL" class="input-group-addon input-sm">'+
                        '<input type="checkbox" id="'+ prefix +'.'+ app +'.'+ index +'.'+sslname+'" name="'+ prefix +'.'+ app +'.'+ index +
                            '.'+sslname+'" data-index="'+index+'" data-app="'+app+'" class="client-parameter" data-group="'+section+'" data-label="'+
                            app+' '+instance+' SSL" data-original="false" value="true">'+
                        '</div>';
             case "Host":
                nameval = "changeme";
                nameori = "";
             break;
             case "Deletes":
                itype = "<select type=\"select\""
                extra = '<option value="0">Disabled</option><option selected value="1">1</option><option value="5">5</option><option value="15">15</option></select>';
        }

        row += '<td colspan="'+ colspan +'"><form class="form-inline"><div class="form-group" style="width:100%"><div class="input-group" style="width:100%">';
        row += itype +'" name="'+ prefix +'.'+ app +'.'+ index +'.'+ name +'" '+
        'id="'+ prefix +'.'+ app +'.'+ index +'.'+ name +'" '+
        'name="'+ prefix +'.'+ app +'.'+ index +'.'+ name +'" '+
        'data-app="'+ app +'" data-index="'+ index +'" class="client-parameter form-control input-sm" data-group="'+ section +'" data-label="'+ app +' '+
         instance +' '+name+'" data-original="'+ nameori +'" value="'+ nameval +'">'+ extra;
        row += '</div></div></form></td>';
        colspan = 1;
    }

    // Add this new data row to our table.
    $('#'+ section +'-'+ app +'-container').append(row);

    // Hide the "no instances" item that displays when no instances are configured.
    $('#'+ section +'-'+ app +'-none').hide();

    // Bring up the save changes button.
    findPendingChanges();
}

function addWatchFiles()
{
    const index = $('.files-WatchFiles').length;
    const instance = index+1;
    const names = $('#files-WatchFiles-addbutton').data('names'); // list of thinges like "Name" and "URL"
    // This just sets the first few lines of the row (the action buttons). The more-dynamic bits get added below, in a for loop.
    let row = '<tr class="newRow bk-success files-WatchFiles" id="files-WatchFiles-'+ instance +'">'+
    '<td style="white-space:nowrap;">'+
        '<div class="btn-group" role="group" style="display:flex;">'+
            '<button onclick="removeInstance(\'files-WatchFiles\', '+ instance +')" type="button" class="delete-item-button btn btn-danger btn-sm" style="font-size:18px;width:35px;"><i class="fa fa-trash-alt"></i></button>'+
            '<button id="filesIndexLabel'+ index +'" class="btn btn-sm" style="font-size:18px;width:35px;pointer-events:none;">'+ instance +'</button>'+
        '</div>'+
    '</td>';

    // Timeout and Interval must have valid go durations, or the parser errors.
    // Host or URL are set with a value, and without an original value to make the save button appear.
    for (const name of names) {
        let nameval = ""
        if (name == "Path") {
            // On windows the sample path is c:\something.
            nameval = $('#files-WatchFiles-addbutton').data('samplepath');
        }

        row += '<td><form class="form-inline"><div class="form-group" style="width:100%"><div class="input-group" style="width:100%">';
        let extra = '';

        switch (name) {
             case "Poll":
             case "Pipe":
             case "MustExist":
             case "LogMatch":
                row += '<select id="WatchFiles.'+ index +'.'+ name +'" name="WatchFiles.'+ index +'.'+ name +'" data-index="'+ index +'" data-app="WatchFiles" '+
                    'class="client-parameter form-control input-sm" data-group="files" data-label="Files '+ instance +' '+ name +'" data-original="false" value="false">'+
                    '<option value="true">Enabled</option>'+
                    '<option selected value="false">Disabled</option></select>';
                break;
            case "Path":
                extra = '<div onClick="browseFiles(\'#WatchFiles\\\\.'+index+'\\\\.Path\');" style="max-width:35px;width:35px;cursor:pointer;font-size:16px;" class="input-group-addon input-sm"><a class="help-icon fas fa-folder-open"></a></div>';
             default:
                row += '<input type="text" name="WatchFiles.'+ index +'.'+ name +'" '+
                    'id="WatchFiles.'+ index +'.'+ name +'" '+
                    'name="WatchFiles.'+ index +'.'+ name +'" '+
                    'data-app="WatchFiles" data-index="'+ index +'" class="client-parameter form-control input-sm" data-group="files" data-label="Files '+
                    instance +' '+name+'" data-original="" value="'+ nameval +'">'+extra;
                break;
        }

        row += '</div></div></form></td>';
    }

    // Add this new data row to our table.
    $('#files-WatchFiles-container').append(row);

    // Hide the "no instances" item that displays when no instances are configured.
    $('#files-WatchFiles-none').hide();

    // Bring up the save changes button.
    findPendingChanges();
}

function addCommand()
{
    const index = $('.commands-Commands').length;
    const instance = index+1;
    const names = $('#commands-Commands-addbutton').data('names'); // list of thinges like "Name" and "URL"
    // This just sets the first few lines of the row (the action buttons). The more-dynamic bits get added below, in a for loop.
    let row = '<tr class="newRow bk-success commands-Commands" id="commands-Commands-'+ instance +'">'+
    '<td style="white-space:nowrap;">'+
        '<div class="btn-group" role="group" style="display:flex;">'+
            '<button onclick="removeInstance(\'commands-Commands\', '+ instance +')" type="button" class="delete-item-button btn btn-danger btn-sm" style="font-size:18px;width:35px;"><i class="fa fa-trash-alt"></i></button>'+
            '<button id="filesIndexLabel'+ index +'" class="btn btn-sm btn-dark" style="font-size:18px;width:40px;pointer-events:none;"><i class="text-success fa fa-exclamation-triangle"></i></button>'+
            '<button onClick="testInstance($(this), \'Commands\', \''+ index +'\')" type="button" class="btn btn-success btn-sm checkInstanceBtn" style="font-size:18px;"><i class="fas fa-check-double"></i></button>'+
        '</div>'+
    '</td>';

    // Timeout and Interval must have valid go durations, or the parser errors.
    // Host or URL are set with a value, and without an original value to make the save button appear.
    for (const name of names) { 
        row += '<td><form class="form-inline"><div class="form-group" style="width:100%"><div class="input-group" style="width:100%">';
        let extra = '';

        switch (name) {
            case "Log":
                row += '<select id="Commands.'+ index +'.'+ name +'" name="Commands.'+ index +'.'+ name +'" data-index="'+ index +'" data-app="Commands" '+
                'class="client-parameter form-control input-sm" data-group="commands" data-label="Command '+ instance +' '+ name +'" data-original="true" value="true">'+
                '<option selected value="true">Enabled</option>'+
                '<option value="false">Disabled</option></select>';
                break;
            case "Shell":
            case "Notify":
                row += '<select id="Commands.'+ index +'.'+ name +'" name="Commands.'+ index +'.'+ name +'" data-index="'+ index +'" data-app="Commands" '+
                    'class="client-parameter form-control input-sm" data-group="commands" data-label="Command '+ instance +' '+ name +'" data-original="false" value="false">'+
                    '<option value="true">Enabled</option>'+
                    '<option selected value="false">Disabled</option></select>';
                break;
            case "Timeout":
                row += '<select type="select" name="Commands.'+ index +'.'+ name +'" '+
                    'id="Commands.'+ index +'.'+ name +'" '+
                    'name="Commands.'+ index +'.'+ name +'" '+
                    'data-app="Commands" data-index="'+ index +'" class="client-parameter form-control input-sm" data-group="commands" data-label="Command '+
                    instance +' '+name+'" data-original="20s" value="20s"><option value="0s">No Timeout</option><option value="1s">1 second</option>'+
                    '<option value="2s">2 seconds</option><option value="3s">3 seconds</option><option value="4s">4 seconds</option>'+
                    '<option value="5s">5 seconds</option><option value="6s">6 seconds</option><option value="7s">7 seconds</option>'+
                    '<option value="8s">8 seconds</option><option value="9s">9 seconds</option><option value="10s">10 seconds</option>'+
                    '<option value="15s">15 seconds</option><option value="20s">20 seconds</option><option value="25s">25 seconds</option>'+
                    '<option value="30s">30 seconds</option><option value="45s">45 seconds</option><option value="1m">1 minute</option>'+
                    '<option value="1m15s">1 min 15 sec</option><option value="1m30s">1 min 30 sec</option><option value="1m45s">1 min 45 sec</option>'+
                    '<option value="2m">2 minutes</option><option value="2m30s">2 min 30 sec</option><option value="3m">3 minutes</option>'+
                    '<option value="3m">4 minutes</option><option value="3m">5 minutes</option><option value="3m">6 minutes</option>'+
                    '<option value="3m">7 minutes</option></select>';
                    break;
            case "Command":
                extra = '<div onClick="browseFiles(\'#Commands\\\\.'+index+'\\\\.Command\');" style="max-width:35px;width:35px;cursor:pointer;font-size:16px;" class="input-group-addon input-sm"><a class="help-icon fas fa-folder-open"></a></div>';
            default:
                row += '<input type="text" name="Commands.'+ index +'.'+ name +'" '+
                    'id="Commands.'+ index +'.'+ name +'" '+
                    'name="Commands.'+ index +'.'+ name +'" '+
                    'data-app="Commands" data-index="'+ index +'" class="client-parameter form-control input-sm" data-group="commands" data-label="Command '+
                    instance +' '+name+'" data-original="">'+extra;
                break;
        }

        row += '</div></div></form></td>';
    }

    // Add this new data row to our table.
    $('#commands-Commands-container').append(row);

    // Hide the "no instances" item that displays when no instances are configured.
    $('#commands-Commands-none').hide();

    // Bring up the save changes button.
    findPendingChanges();
}

function stopFileWatch(from, index)
{
    from.css({'pointer-events':'none'}).find('i').toggleClass('fa-cog fa-spin fa-stop-circle');

    $.ajax({
        type: 'GET',
        url: URLBase+'stopFileWatch/'+index,
        success: function (data){
            from.css({'pointer-events':'auto'}).find('i').toggleClass('fa-cog fa-spin fa-stop-circle');
            from.hide();
            from.siblings('.checkInstanceBtn').show();
            $('#activeFileCell'+index).toggleClass('bk-brand bk-danger');
            toast('Watcher Stopped!', data, 'success');
        },
        error: function (response, status, error) {
            from.css({'pointer-events':'auto'}).find('i').toggleClass('fa-cog fa-spin fa-stop-circle');

            if (response.responseText === undefined) {
                toast('Web Server Error',
                    'Notifiarr client appears to be down! Hard refresh recommended.', 'error', 30000);
            } else {
                toast('Watcher Error', error+': '+response.responseText, 'error', 15000);
            }
        }
    });
}

function startFileWatch(from, index)
{
    from.css({'pointer-events':'none'}).find('i').toggleClass('fa-cog fa-spin fa-play-circle');

    $.ajax({
        type: 'GET',
        url: URLBase+'startFileWatch/'+index,
        success: function (data){
            from.css({'pointer-events':'auto'}).find('i').toggleClass('fa-cog fa-spin fa-play-circle');
            from.hide();
            $('#activeFileCell'+index).toggleClass('bk-brand bk-danger');
            from.siblings('.checkInstanceBtn').show();

            toast('Watcher Started!', data, 'success');
        },
        error: function (response, status, error) {
            from.css({'pointer-events':'auto'}).find('i').toggleClass('fa-cog fa-spin fa-play-circle');

            if (response.responseText === undefined) {
                toast('Web Server Error',
                    'Notifiarr client appears to be down! Hard refresh recommended.', 'error', 30000);
            } else {
                toast('Watcher Error', error+': '+response.responseText, 'error', 15000);
            }
        }
    });
}