// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

var dataDirResource = {
    name: 'Data Directory',
    url: 'file:///mnt/data'
}

var dataResources = {
    'Data Directory': dataDirResource,
};

function refreshDataResources() {
    for (var resourceName in dataResources) {
        if (dataResources.hasOwnProperty(resourceName)) {
            var cmd = {
                Command: 'ls',
                Metadata: dataResources[resourceName]
            }
            ws.send(JSON.stringify(cmd));
        }
    }
}

function createDataManager() {
    box = document.createElement('div');
    box.classList.add('box', 'hidden');
    box.ondragover = function(event) {
        event.stopPropagation();
    }
    controlpanel.appendChild(box);

    var title = document.createElement('div');
    title.setAttribute('class', 'controltitle');
    box.appendChild(title);
    var titletext = document.createElement('div');
    title.appendChild(titletext);
    titletext.innerHTML = 'Data Manager';

    var runs = document.createElement('div');
    runs.id = 'datamgrrunslist';

    var resources = document.createElement('div');
    resources.id = 'datamgrresources';
    for (var resourceName in dataResources) {
        if (dataResources.hasOwnProperty(resourceName)) {
            var resource = document.createElement('div');
            resource.classList.add('listelement');
            resource.innerHTML = dataResources[resourceName].name + ' (' + dataResources[resourceName].url + ')';
            resources.appendChild(resource);
        }
    }

    fillControlTabs(box, [{
        name: 'Runs',
        element: runs
    }, {
        name: 'Resources',
        element: resources
    }]);

    var e = document.createElement('div');
    e.classList.add('sidebarlistelement');
    e.innerHTML = 'Data Manager';
    managerlist.appendChild(e);

    e.addEventListener('click',
        function() {
            if (e.classList.contains('enabledlistelement')) {
                e.classList.remove('enabledlistelement');
                box.classList.add('hidden');
            } else {
                e.classList.add('enabledlistelement');
                box.classList.remove('hidden')
            }
        }
    );
}

createDataManager();

var dragrun = null;

function handleRunList(msg) {
    if (!'status' in msg.Metadata) {
        console.log('no status in run list');
        return;
    }

    if (msg.Metadata['status'] === 'failure') {
        console.log(msg.Metadata['status'], atob(msg.Payload));
        return;
    }

    var resourceRunListId = msg.Metadata['name'] + ' run list';
    var resourceRuns = document.getElementById(resourceRunListId);
    if (resourceRuns === null) {
        resourceRuns = document.createElement('div');
        resourceRuns.id = resourceRunListId;
        var runs = document.getElementById('datamgrrunslist');
        runs.appendChild(resourceRuns);

        var resourceHeader = document.createElement('div');
        resourceHeader.classList.add('headerelement');
        resourceHeader.innerHTML = msg.Metadata['name'];
        resourceRuns.appendChild(resourceHeader);
    }

    var setListeners = function(run) {
        run.ondragstart = function(event) {
            event.dataTransfer.setData('text', run.innerHTML)
            dragrun = run;
        }
        run.ondragend = function(event) {
            dragrun = null;
        }

        run.addEventListener(
            'click',
            function() {
                if (run.classList.contains('active')) {
                    run.classList.remove('active');

                    var children = run.children
                    for (var i = 0; i < children.length; i++) {
                        var child = children[i];
                        if (child.classList.contains('runmeta')) {
                            run.removeChild(child);
                        }
                    }
                } else {
                    run.classList.add('active');

                    var cmd = {
                        Command: 'get meta',
                        Metadata: {
                            url: run.resource.url + '/' + run.name,
                            credentials: run.resource.credentials
                        }
                    }
                    ws.send(JSON.stringify(cmd));
                }
            }
        );
    }

    var list = JSON.parse(atob(msg.Payload));
    if (list !== null) {
        for (var i = 0; i < list.length; i++) {
            var run = document.createElement('div');
            run.classList.add('listelement', 'indented', 'interactive');
            run.innerHTML = list[i].Name;
            run.draggable = true;
            run.name = list[i].Name;
            run.resource = dataResources[msg.Metadata['name']];
            run.id = run.resource.url + '/' + run.name;
            if (document.getElementById(run.id) === null) {
                resourceRuns.appendChild(run);
            }

            setListeners(run);
        }
    }
}

function handlePlayerFailure(msg) {
    console.log(msg.Metadata['url'], atob(msg.Payload));
}

function handleRunMeta(msg) {
    var run = document.getElementById(msg.Metadata['url']);
    if (run === null || !run.classList.contains('active')) {
        return
    }

    var meta = JSON.parse(atob(msg.Payload))
    var desc = document.createElement('div');
    desc.classList.add('runmeta');
    run.appendChild(desc);
    if (meta.hasOwnProperty('Description')) {
        desc.innerHTML = 'Description: ' + atob(meta['Description']);
    } else {
        desc.innerHTML = 'no description';
    }
}
