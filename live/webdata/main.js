// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

document.ondragover = function(event) {
    event.preventDefault();
    event.stopPropagation();
}
document.ondrop = function(event) {
    event.preventDefault();
    event.stopPropagation();
    if (dragsource !== null) {
        if (dragsource.compatshows.length > 0) {
            var cmd = {
                Command: 'stream cmd',
                Metadata: {
                    stream: dragsource.stream,
                    'stream cmd': 'new show',
                    type: dragsource.compatshows[0],
                    source: dragsource.innerHTML
                }
            }
            ws.send(JSON.stringify(cmd));
        }
    } else if (dragrun !== null) {
        var cmd = {
            Command: 'play run',
            Metadata: {
                url: dragrun.resource.url + '/' + dragrun.name,
                credentials: dragrun.resource.credentials
            }
        }
        ws.send(JSON.stringify(cmd));
    }
}

var sidebar = document.getElementById('sidebar');
sidebar.ondragover = function(event) {
    event.stopPropagation();
}

function fillControlTabs(box, tabs) {
    var tabsdiv = document.createElement('div');
    tabsdiv.setAttribute('class', 'tab');
    box.appendChild(tabsdiv);

    var hideAll = function() {
        var children = box.children;
        for (var i = 0; i < children.length; i++) {
            if (children[i].classList.contains('controlboxcolumn')) {
                children[i].style.display = 'none';
            }
        }

        var children = tabsdiv.children;
        for (var i = 0; i < children.length; i++) {
            children[i].classList.remove('active');
        }
    }

    var makeTabButton = function(name, element, hideAll) {
        var button = document.createElement('button');
        button.innerHTML = name;
        button.setAttribute('class', 'tablink');
        button.addEventListener(
            'click',
            function() {
                hideAll();
                element.style.display = 'block';
                button.classList.add('active');
            }
        );
        return button;
    }

    var firstButton = null;
    for (var i = 0; i < tabs.length; i++) {
        var tab = tabs[i];
        box.appendChild(tab.element);
        tab.element.classList.add('controlboxcolumn');

        var button = makeTabButton(tab.name, tab.element, hideAll);
        tabsdiv.appendChild(button);

        if (i === 0) {
            firstButton = button;
        }
    }

    if (firstButton !== null) {
        firstButton.click();
    }
}

function setMeterWidth(meter, width) {
    meter.style.width = width;
    var radius = parseFloat(getComputedStyle(meter, null).borderRadius)
    if (meter.getBoundingClientRect().width < 2 * radius) {
        meter.style.width = (2 * radius).toString() + 'px';
    }
}

function shrinkRemoveBox(e) {
    e.classList.add('hidden');
    setTimeout(
        function() {
            var p = e.parentNode;
            if (p !== null) {
                p.removeChild(e);
            }
        },
        250
    );
}

function getCookie(name) {
    var value = '; ' + document.cookie;
    var parts = value.split('; ' + name + '=');
    if (parts.length == 2) return parts.pop().split(';').shift();
    return '';
}
