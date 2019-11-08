// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

function handleStreamAnnounce(msg) {
    var e = document.createElement('div');
    var stream = msg.Metadata.name;
    e.innerHTML = 'Stream: ' + stream;
    e.id = 'stream ' + stream + ' list element';
    e.classList.add('sidebarlistelement', 'stream');
    sidebar.appendChild(e);

    e.addEventListener('click',
        function() {
            if (ws.readyState != WebSocket.OPEN) {
                return
            }

            if (e.classList.contains('enabledlistelement')) {
                var cmd = {
                    Command: 'stream unsub',
                    Metadata: {
                        stream: stream
                    }
                }
                ws.send(JSON.stringify(cmd));

                e.classList.remove('enabledlistelement');
            } else {
                var cmd = {
                    Command: 'stream sub',
                    Metadata: {
                        stream: stream
                    }
                }
                ws.send(JSON.stringify(cmd));

                e.classList.add('enabledlistelement');
            }
        }
    );
}

function handleStreamClose(msg) {
    var stream = msg.Metadata.name

    handleUnsubscribe({
        Metadata: {
            stream: stream
        }
    })

    sidebar.removeChild(document.getElementById('stream ' + stream + ' list element'));
}

function handleSubscribe(msg) {
    var stream = msg.Metadata.stream;

    makeStreamBox(msg, stream)

    var cmd = {
        Command: 'stream cmd',
        Metadata: {
            stream: stream,
            'stream cmd': 'pub all shows'
        }
    }
    ws.send(JSON.stringify(cmd));

    var cmd = {
        Command: 'stream cmd',
        Metadata: {
            stream: stream,
            'stream cmd': 'list all sources'
        }
    }
    ws.send(JSON.stringify(cmd));

    var cmd = {
        Command: 'stream cmd',
        Metadata: {
            stream: stream,
            'stream cmd': 'pub desc'
        }
    }
    ws.send(JSON.stringify(cmd));
}

function handleUnsubscribe(msg) {
    var stream = msg.Metadata.stream;

    var e = document.getElementById('stream ' + stream);
    if (e != null) {
        shrinkRemoveBox(e);
    }

    var query = '[id^="stream ' + stream + ' show "]';
    var shows = document.querySelectorAll(query);
    var nShows = shows.length;
    for (var i = 0; i < nShows; i++) {
        shrinkRemoveBox(shows[i]);
    }
}

function handleStreamStatus(msg) {
    var stream = msg.Metadata.stream;
    delete msg.Metadata['stream'];

    var statustablediv = document.getElementById('stream ' + stream + ' status table');
    if (statustablediv != null) {
        for (var key in msg.Metadata) {
            var found = false
            var rows = statustablediv.childNodes;
            for (var i = 0; i < rows.length; i++) {
                if (rows[i].childNodes[0].innerHTML === key) {
                    rows[i].childNodes[1].innerHTML = msg.Metadata[key];
                    found = true;
                    break;
                }
            }

            if (!found) {
                var rowdiv = document.createElement('div');
                rowdiv.setAttribute('class', 'statusrow');
                statustablediv.appendChild(rowdiv);
                var keydiv = document.createElement('div');
                keydiv.setAttribute('class', 'statuskey');
                rowdiv.appendChild(keydiv);
                var valuediv = document.createElement('div');
                valuediv.setAttribute('class', 'statusvalue');
                rowdiv.appendChild(valuediv);

                keydiv.innerHTML = key;
                valuediv.innerHTML = msg.Metadata[key];
            }
        }
    }

    var runstart = document.getElementById('stream ' + stream + ' run start');
    if (runstart !== null) {
        if (msg.Metadata.hasOwnProperty('Run Time')) {
            runstart.innerHTML = 'Start Run (' + msg.Metadata['Run Time'] + ')';
        }
    }
}

var dragsource = null;

function handleSourceAnnounce(msg) {
    var stream = msg.Metadata.stream;
    var type = msg.Metadata.type;

    var datsourcediv = null;
    if (type === 'Normal') {
        datadiv = document.getElementById('stream ' + stream + ' data sources');
    } else if (type === 'Advanced') {
        datadiv = document.getElementById('stream ' + stream + ' adv data sources');
    }

    if (datadiv != null) {
        var sourcediv = document.createElement('div');
        sourcediv.classList.add('listelement', 'indented', 'interactive');
        sourcediv.innerHTML = msg.Metadata.source;
        sourcediv.draggable = true;

        var i = 0;
        var elements = datadiv.childNodes;
        for (; i < elements.length; i++) {
            if (elements[i].innerHTML >= sourcediv.innerHTML) {
                break;
            }
        }

        if (i == elements.length) {
            datadiv.appendChild(sourcediv);
        } else {
            if (elements[i].innerHTML != sourcediv.innerHTML) {
                datadiv.insertBefore(sourcediv, elements[i]);
            } else {
                sourcediv = elements[i];
            }
        }

        var compatshows = msg.Metadata['compat shows'].split(',');
        for (var i = 0; i < compatshows.length; i++) {
            compatshows[i] = compatshows[i].trim();
        }
        sourcediv.compatshows = compatshows;
        sourcediv.stream = stream;
        sourcediv.type = type;

        sourcediv.ondragstart = function(event) {
            event.dataTransfer.setData('text', msg.Metadata.source)
            dragsource = sourcediv;
        }
        sourcediv.ondragend = function(event) {
            dragsource = null;
        }
    }
}


function makeStreamBox(msg, stream) {
    var box = document.createElement('div');
    box.classList.add('box', 'hidden');
    box.id = 'stream ' + stream
    box.ondragover = function(event) {
        event.stopPropagation();
    }
    controlpanel.appendChild(box);

    var title = document.createElement('div');
    title.setAttribute('class', 'controltitle');
    box.appendChild(title);
    var titletext = document.createElement('div');
    title.appendChild(titletext);
    titletext.innerHTML = stream;
    var titlebuttons = document.createElement('div');
    titlebuttons.classList.add('titlebuttons');
    title.appendChild(titlebuttons);

    var closebutton = document.createElement('img');
    closebutton.setAttribute('class', 'titlebutton');
    closebutton.src = '/webdata/close-icon.png';
    closebutton.draggable = false;
    closebutton.title = 'Close';
    closebutton.alt = closebutton.title;
    titlebuttons.appendChild(closebutton);

    closebutton.addEventListener(
        'click',
        function() {
            cmd = {
                Command: 'stream cmd',
                Metadata: {
                    stream: stream,
                    'stream cmd': 'kill'
                }
            };
            ws.send(JSON.stringify(cmd));
        }
    );

    // Run control
    var runctldiv = document.createElement('div');
    runctldiv.style.overflow = 'hidden';

    var runstop = document.createElement('button');
    runstop.setAttribute('class', 'control red');
    runstop.innerHTML = 'Stop Run';
    runctldiv.appendChild(runstop);

    var runstart = document.createElement('button');
    runstart.id = 'stream ' + stream + ' run start';
    runstart.setAttribute('class', 'control green');
    runstart.innerHTML = 'Start Run';
    runctldiv.appendChild(runstart);

    var resourcelabel = document.createElement('label');
    resourcelabel.innerHTML = 'Destination Resource';
    //resourcelabel.for = stream + ' resourceselect';
    resourcelabel.classList.add('control');
    runctldiv.appendChild(resourcelabel);
    var resourceselect = document.createElement('select');
    for (var resourceName in dataResources) {
        if (dataResources.hasOwnProperty(resourceName)) {
            var opt = document.createElement('option');
            opt.value = resourceName;
            opt.innerHTML = dataResources[resourceName].name;
            resourceselect.appendChild(opt);

            if (getCookie('resource') === resourceName) {
                resourceselect.selectedIndex = resourceselect.childElementCount - 1;
            }
        }
    }
    resourceselect.classList.add('control');
    resourceselect.id = stream + ' resourceselect';
    runctldiv.appendChild(resourceselect);

    var rundesc = document.createElement('textarea');
    rundesc.setAttribute('class', 'control');
    rundesc.setAttribute('placeholder', 'Run Description');
    runctldiv.appendChild(rundesc);

    runstart.addEventListener(
        'click',
        function() {
            var resourcename = resourceselect.options[resourceselect.selectedIndex].value;
            var resource = dataResources[resourcename];
            var exp = new Date();
            exp.setTime(exp.getTime() + 31536000000); // expire in 365 days
            document.cookie = 'resource=' + resourcename + '; expires=' + exp.toUTCString() + ';';

            cmd = {
                Command: 'stream cmd',
                Metadata: {
                    stream: stream,
                    'stream cmd': 'start run',
                    url: resource['url'],
                    credentials: resource['credentials'],
                    Description: rundesc.value
                }
            };
            ws.send(JSON.stringify(cmd));
        }
    );

    runstop.addEventListener(
        'click',
        function() {
            cmd = {
                Command: 'stream cmd',
                Metadata: {
                    stream: stream,
                    'stream cmd': 'stop run'
                }
            };
            ws.send(JSON.stringify(cmd));
        }
    );

    // Stream status
    var statusdiv = document.createElement('div');
    var statustablediv = document.createElement('div');
    statustablediv.setAttribute('class', 'statustable');
    statustablediv.id = 'stream ' + stream + ' status table';
    statusdiv.appendChild(statustablediv);

    // Data sources
    var datadiv = document.createElement('div');

    var normsourcetitle = document.createElement('div');
    normsourcetitle.classList.add('headerelement');
    normsourcetitle.innerHTML = 'Normal';
    datadiv.appendChild(normsourcetitle);
    var normsourcediv = document.createElement('div');
    normsourcediv.id = 'stream ' + stream + ' data sources';
    datadiv.appendChild(normsourcediv);

    var advsourcetitle = document.createElement('div');
    advsourcetitle.classList.add('headerelement');
    advsourcetitle.innerHTML = 'Advanced';
    datadiv.appendChild(advsourcetitle);
    var advsourcediv = document.createElement('div');
    advsourcediv.id = 'stream ' + stream + ' adv data sources';
    datadiv.appendChild(advsourcediv);

    fillControlTabs(box, [{
        name: 'Run Control',
        element: runctldiv
    }, {
        name: 'Data',
        element: datadiv
    }, {
        name: 'Status',
        element: statusdiv
    }]);

    var off = box.offsetWidth;
    box.classList.remove('hidden');
}
