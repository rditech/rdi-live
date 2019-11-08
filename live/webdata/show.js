// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

var parser = new DOMParser();

function handleShowFrame(msg, stream, id) {
    var stream = msg.Metadata['stream name']
    var listE = document.getElementById('stream ' + stream + ' list element');
    if (listE === null || !listE.classList.contains('enabledlistelement')) {
        return;
    }

    var showId = msg.Metadata['show id'];
    var id = 'stream ' + stream + ' show ' + showId;
    var outerdiv = document.getElementById(id);
    if (outerdiv === null || outerdiv.classList.contains('hidden')) {
        outerdiv = makeShowBox(msg, stream, id, showId);
    } else if (outerdiv.classList.contains('swapping') || outerdiv.classList.contains('paused')) {
        return;
    }

    var showdiv = outerdiv.childNodes[2];
    if (msg.Metadata['is png'] === 'true') {
        if (!showdiv.firstChild) {
            var png = document.createElement('img');
            png.setAttribute('width', '100%');
            var url = 'data:image/png;base64,' + msg.Payload;
            png.src = url;
            showdiv.appendChild(png);
        } else {
            var png = showdiv.firstChild;
            var url = 'data:image/png;base64,' + msg.Payload;
            png.src = url;
        }
    } else {
        if (!showdiv.firstChild) {
            var svg = parser.parseFromString(atob(msg.Payload), 'image/svg+xml').documentElement;
            svg.setAttribute('width', '100%');
            svg.removeAttribute('height');
            svg.setAttribute('viewBox', '0 0 365 230');
            showdiv.appendChild(svg);
        } else {
            var svg = showdiv.firstChild;
            var tempSvg = parser.parseFromString(atob(msg.Payload), 'image/svg+xml').documentElement;
            svg.innerHTML = tempSvg.innerHTML;
        }
    }

    delete msg.Metadata['show id'];
    var params = msg.Metadata

    var settings = outerdiv.childNodes[1];
    var setting = settings.childNodes;
    for (var i = 0; i < setting.length; i++) {
        var param = setting[i].id;
        if (params.hasOwnProperty(param)) {
            updateSetting(stream, showId, setting[i], param, params[param]);
            delete params[param];
        }
    }

    for (var param in params) {
        var setting = document.createElement('div');
        setting.id = param;
        setting.setAttribute('class', 'setting');
        updateSetting(stream, showId, setting, param, params[param], settings);
        settings.appendChild(setting);
    }
}

function handleShowClose(msg) {
    var id = 'stream ' + msg.Metadata.stream + ' show ' + msg.Metadata['show id'];
    var e = document.getElementById(id);
    if (e != null) {
        shrinkRemoveBox(e);
    }
}

var dragbox = null;

function makeShowBox(msg, stream, id, showId) {
    var outerdiv = document.createElement('div');
    outerdiv.classList.add('box', 'hidden');
    outerdiv.draggable = true;
    showpanel.insertBefore(outerdiv, showpanel.childNodes[0]);
    var title = document.createElement('div');
    title.setAttribute('class', 'showtitle');
    outerdiv.appendChild(title);
    var titletext = document.createElement('div');
    title.appendChild(titletext);
    var showtype = msg.Metadata['show type'];
    titletext.innerHTML = stream;
    titletext.classList.add('showtitletext');
    var titlebuttons = document.createElement('div');
    titlebuttons.classList.add('titlebuttons');
    title.appendChild(titlebuttons);

    var url = null;
    var savebutton = document.createElement('a');
    savebutton.href = url;
    savebutton.download = stream;
    savebutton.draggable = false;
    var saveimg = document.createElement('img');
    saveimg.setAttribute('class', 'titlebutton');
    saveimg.src = '/webdata/save-icon.png';
    saveimg.draggable = false;
    savebutton.title = 'Save';
    savebutton.alt = savebutton.title;
    savebutton.appendChild(saveimg);
    titlebuttons.appendChild(savebutton);

    var settingsbutton = document.createElement('img');
    settingsbutton.setAttribute('class', 'titlebutton');
    settingsbutton.src = '/webdata/settings-icon.png';
    settingsbutton.draggable = false;
    settingsbutton.title = 'Settings';
    settingsbutton.alt = settingsbutton.title;
    titlebuttons.appendChild(settingsbutton);

    var settings = document.createElement('div');
    settings.setAttribute('class', 'settings');
    outerdiv.appendChild(settings);

    settings.addEventListener('mouseenter', function() {
        this.parentNode.setAttribute("draggable", false);
    });
    settings.addEventListener('mouseleave', function() {
        this.parentNode.setAttribute("draggable", true);
    });

    settingsbutton.addEventListener('click',
        function(event) {
            if (settings.style.display === 'block') {
                settings.style.display = 'none';
            } else {
                settings.style.display = 'block';
            }
            event.stopPropagation();
        }
    )

    var showdiv = document.createElement('div');
    outerdiv.appendChild(showdiv);
    showdiv.setAttribute('class', 'plot');

    if (msg.Metadata['is png'] === 'true') {
        savebutton.download += '.png';
        savebutton.addEventListener('click',
            function(event) {
                if (showdiv.firstChild) {
                    savebutton.href = showdiv.firstChild.src;
                }
                event.stopPropagation();
            }
        )
    } else {
        savebutton.download += '.svg';
        savebutton.addEventListener('click',
            function(event) {
                var blob = new Blob([showdiv.innerHTML]);
                url = URL.createObjectURL(blob);
                savebutton.href = url;
                event.stopPropagation();
            }
        )
    }

    var entercount = 0;
    outerdiv.ondragstart = function(event) {
        event.dataTransfer.setData('text', titletext.innerHTML)
        outerdiv.classList.add('swapping');
        dragbox = outerdiv;
    }
    outerdiv.ondragend = function(event) {
        outerdiv.classList.remove('swapping');
        dragbox = null;
    }
    outerdiv.ondragover = function(event) {
        event.preventDefault();
        event.stopPropagation();
    }
    outerdiv.ondragenter = function(event) {
        if (outerdiv != dragbox) {
            if (entercount === 0) {
                outerdiv.classList.add('swapping');
            }
            entercount++;
        }
    }
    outerdiv.ondragleave = function(event) {
        if (outerdiv != dragbox) {
            entercount--;
            if (entercount === 0) {
                outerdiv.classList.remove('swapping');
            }
        }
    }
    outerdiv.ondrop = function(event) {
        event.preventDefault();
        entercount = 0;
        outerdiv.classList.remove('swapping');

        if (dragbox !== null) {
            var parent = dragbox.parentNode;
            var nextSibling = dragbox.nextSibling;
            if (nextSibling == outerdiv) {
                nextSibling = dragbox;
            }
            parent.replaceChild(dragbox, outerdiv);
            parent.insertBefore(outerdiv, nextSibling);
        } else if (dragsource !== null) {
            var compat = false;
            for (var i = 0; i < dragsource.compatshows.length; i++) {
                var thisshowtype = dragsource.compatshows[i];
                if (thisshowtype === showtype) {
                    compat = true;
                    break;
                }
            }

            if (compat && dragsource.stream === stream) {
                var cmd = {
                    Command: 'stream cmd',
                    Metadata: {
                        stream: stream,
                        'stream cmd': 'map source',
                        'show id': showId,
                        'source': dragsource.innerHTML
                    }
                }
                ws.send(JSON.stringify(cmd));

                event.stopPropagation();
            }
        }
    }

    var pausebutton = document.createElement('img');
    pausebutton.setAttribute('class', 'titlebutton');
    pausebutton.src = '/webdata/pause-icon.png';
    pausebutton.draggable = false;
    pausebutton.title = 'Pause';
    pausebutton.alt = pausebutton.title;
    titlebuttons.appendChild(pausebutton);

    pausebutton.addEventListener(
        'click',
        function() {
            if (outerdiv.classList.contains('paused')) {
                outerdiv.classList.remove('paused');
                pausebutton.src = '/webdata/pause-icon.png';
                pausebutton.title = 'Pause';
                pausebutton.alt = pausebutton.title;
            } else {
                outerdiv.classList.add('paused');
                pausebutton.src = '/webdata/play-icon.png';
                pausebutton.title = 'Resume';
                pausebutton.alt = pausebutton.title;
            }
        }
    )

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
                    'stream cmd': 'rm show',
                    'show id': showId
                }
            };
            ws.send(JSON.stringify(cmd));
        }
    );

    var off = outerdiv.offsetWidth;
    outerdiv.classList.remove('hidden');

    outerdiv.id = id;
    return outerdiv
}

function updateSetting(stream, showId, setting, param, value, settings) {
    var cmd = {
        Command: 'stream cmd',
        Metadata: {
            stream: stream,
            'stream cmd': 'show cmd',
            'show id': showId,
            'show cmd': 'set params'
        }
    }

    switch (param) {
        case 'logscale':
            if (setting.innerHTML == '') {
                var checkbox = document.createElement('input');
                checkbox.id = 'checkbox';
                checkbox.type = 'checkbox';
                setting.appendChild(checkbox);

                var label = document.createElement('label');
                label.for = 'checkbox';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Logarithmic scale';
                setting.appendChild(label);

                checkbox.addEventListener(
                    'change',
                    function() {
                        if (checkbox.checked) {
                            cmd.Metadata[param] = 'true';
                        } else {
                            cmd.Metadata[param] = 'false';
                        }
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.firstChild === document.activeElement) {
                break;
            }
            if (value === 'true') {
                setting.firstChild.checked = true;
            } else {
                setting.firstChild.checked = false;
            }
            break;
        case 'autorange':
            if (setting.innerHTML == '') {
                var checkbox = document.createElement('input');
                checkbox.id = 'checkbox';
                checkbox.type = 'checkbox';
                setting.appendChild(checkbox);

                var label = document.createElement('label');
                label.for = 'checkbox';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Autorange vertical axis';
                setting.appendChild(label);

                checkbox.addEventListener(
                    'change',
                    function() {
                        if (checkbox.checked) {
                            cmd.Metadata[param] = 'true';
                        } else {
                            cmd.Metadata[param] = 'false';
                        }
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.firstChild === document.activeElement) {
                break;
            }
            if (value === 'true') {
                setting.firstChild.checked = true;
            } else {
                setting.firstChild.checked = false;
            }
            break;
        case 'magnitude':
            if (setting.innerHTML == '') {
                var checkbox = document.createElement('input');
                checkbox.id = 'checkbox';
                checkbox.type = 'checkbox';
                setting.appendChild(checkbox);

                var label = document.createElement('label');
                label.for = 'checkbox';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Magnitude';
                setting.appendChild(label);

                checkbox.addEventListener(
                    'change',
                    function() {
                        if (checkbox.checked) {
                            cmd.Metadata[param] = 'true';
                        } else {
                            cmd.Metadata[param] = 'false';
                        }
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.firstChild === document.activeElement) {
                break;
            }
            if (value === 'true') {
                setting.firstChild.checked = true;
            } else {
                setting.firstChild.checked = false;
            }
            break;
        case 'min':
            if (setting.innerHTML == '') {
                var label = document.createElement('label');
                label.for = 'number';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Min';
                setting.appendChild(label);

                var number = document.createElement('input');
                number.id = 'number';
                number.type = 'number';
                number.step = '1e-9';
                setting.appendChild(number);

                number.addEventListener(
                    'change',
                    function() {
                        var c = {
                            Command: 'stream cmd',
                            Metadata: {
                                stream: stream,
                                'stream cmd': 'show cmd',
                                'show id': showId,
                                'show cmd': 'set params',
                                autorange: 'false'
                            }
                        }
                        ws.send(JSON.stringify(c));

                        cmd.Metadata[param] = number.value;
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.childNodes[1] === document.activeElement) {
                break;
            }
            if (setting.childNodes[1].value != value) {
                setting.childNodes[1].value = value;
            }
            break;
        case 'max':
            if (setting.innerHTML == '') {
                var label = document.createElement('label');
                label.for = 'number';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Max';
                setting.appendChild(label);

                var number = document.createElement('input');
                number.id = 'number';
                number.type = 'number';
                number.step = '1e-9';
                setting.appendChild(number);

                number.addEventListener(
                    'change',
                    function() {
                        var c = {
                            Command: 'stream cmd',
                            Metadata: {
                                stream: stream,
                                'stream cmd': 'show cmd',
                                'show id': showId,
                                'show cmd': 'set params',
                                autorange: 'false'
                            }
                        }
                        ws.send(JSON.stringify(c));

                        cmd.Metadata[param] = number.value;
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.childNodes[1] === document.activeElement) {
                break;
            }
            if (setting.childNodes[1].value != value) {
                setting.childNodes[1].value = value;
            }
            break;
        case 'alpha':
            if (setting.innerHTML == '') {
                var label = document.createElement('label');
                label.for = 'number';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Alpha';
                setting.appendChild(label);

                var number = document.createElement('input');
                number.id = 'number';
                number.type = 'number';
                number.min = '0.0001';
                number.step = '0.0001';
                number.max = '1.0';
                setting.appendChild(number);

                number.addEventListener(
                    'change',
                    function() {
                        cmd.Metadata[param] = number.value;
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.childNodes[1] === document.activeElement) {
                break;
            }
            if (setting.childNodes[1].value != value) {
                setting.childNodes[1].value = value;
            }
            break;
        case 'nsample':
            if (setting.innerHTML == '') {
                var label = document.createElement('label');
                label.for = 'number';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Num. points';
                setting.appendChild(label);

                var number = document.createElement('input');
                number.id = 'number';
                number.type = 'number';
                number.min = '2';
                number.step = '1';
                setting.appendChild(number);

                number.addEventListener(
                    'change',
                    function() {
                        cmd.Metadata[param] = number.value;
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.childNodes[1] === document.activeElement) {
                break;
            }
            if (setting.childNodes[1].value != value) {
                setting.childNodes[1].value = value;
            }
            break;
        case 'downsample':
            if (setting.innerHTML == '') {
                var label = document.createElement('label');
                label.for = 'number';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Downsample';
                setting.appendChild(label);

                var number = document.createElement('input');
                number.id = 'number';
                number.type = 'number';
                number.min = '1';
                number.step = '1';
                setting.appendChild(number);

                number.addEventListener(
                    'change',
                    function() {
                        cmd.Metadata[param] = number.value;
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.childNodes[1] === document.activeElement) {
                break;
            }
            if (setting.childNodes[1].value != value) {
                setting.childNodes[1].value = value;
            }
            break;
        case 'trigger':
            if (setting.innerHTML == '') {
                var label = document.createElement('label');
                label.for = 'text';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Trigger';
                setting.appendChild(label);

                var trigger = document.createElement('input');
                trigger.id = 'text';
                trigger.type = 'text';
                setting.appendChild(trigger);

                trigger.addEventListener(
                    'change',
                    function() {
                        cmd.Metadata[param] = trigger.value;
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.childNodes[1] === document.activeElement) {
                break;
            }
            if (setting.childNodes[1].value != value) {
                setting.childNodes[1].value = value;
            }
            break;
        case 'triglevel':
            if (setting.innerHTML == '') {
                var label = document.createElement('label');
                label.for = 'number';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Trigger level';
                setting.appendChild(label);

                var number = document.createElement('input');
                number.id = 'number';
                number.type = 'number';
                number.step = '1e-9';
                setting.appendChild(number);

                number.addEventListener(
                    'change',
                    function() {
                        cmd.Metadata[param] = number.value;
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.childNodes[1] === document.activeElement) {
                break;
            }
            if (setting.childNodes[1].value != value) {
                setting.childNodes[1].value = value;
            }
            break;
        case 'trigleadsample':
            if (setting.innerHTML == '') {
                var label = document.createElement('label');
                label.for = 'number';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Trigger leading samples';
                setting.appendChild(label);

                var number = document.createElement('input');
                number.id = 'number';
                number.type = 'number';
                number.min = '0';
                number.step = '1';
                setting.appendChild(number);

                number.addEventListener(
                    'change',
                    function() {
                        cmd.Metadata[param] = number.value;
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.childNodes[1] === document.activeElement) {
                break;
            }
            if (setting.childNodes[1].value != value) {
                setting.childNodes[1].value = value;
            }
            break;
        case 'trigfall':
            if (setting.innerHTML == '') {
                var checkbox = document.createElement('input');
                checkbox.id = 'checkbox';
                checkbox.type = 'checkbox';
                setting.appendChild(checkbox);

                var label = document.createElement('label');
                label.for = 'checkbox';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Trigger on falling slope';
                setting.appendChild(label);

                checkbox.addEventListener(
                    'change',
                    function() {
                        if (checkbox.checked) {
                            cmd.Metadata[param] = 'true';
                        } else {
                            cmd.Metadata[param] = 'false';
                        }
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.firstChild === document.activeElement) {
                break;
            }
            if (value === 'true') {
                setting.firstChild.checked = true;
            } else {
                setting.firstChild.checked = false;
            }
            break;
        case 'min x':
            if (setting.innerHTML == '') {
                var label = document.createElement('label');
                label.for = 'number';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Min X';
                setting.appendChild(label);

                var number = document.createElement('input');
                number.id = 'number';
                number.type = 'number';
                number.step = '1';
                setting.appendChild(number);

                number.addEventListener(
                    'change',
                    function() {
                        cmd.Metadata[param] = number.value;
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.childNodes[1] === document.activeElement) {
                break;
            }
            if (setting.childNodes[1].value != value) {
                setting.childNodes[1].value = value;
            }
            break;
        case 'max x':
            if (setting.innerHTML == '') {
                var label = document.createElement('label');
                label.for = 'number';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Max X';
                setting.appendChild(label);

                var number = document.createElement('input');
                number.id = 'number';
                number.type = 'number';
                number.step = '1';
                setting.appendChild(number);

                number.addEventListener(
                    'change',
                    function() {
                        cmd.Metadata[param] = number.value;
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.childNodes[1] === document.activeElement) {
                break;
            }
            if (setting.childNodes[1].value != value) {
                setting.childNodes[1].value = value;
            }
            break;
        case 'min y':
            if (setting.innerHTML == '') {
                var label = document.createElement('label');
                label.for = 'number';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Min Y';
                setting.appendChild(label);

                var number = document.createElement('input');
                number.id = 'number';
                number.type = 'number';
                number.step = '1';
                setting.appendChild(number);

                number.addEventListener(
                    'change',
                    function() {
                        cmd.Metadata[param] = number.value;
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.childNodes[1] === document.activeElement) {
                break;
            }
            if (setting.childNodes[1].value != value) {
                setting.childNodes[1].value = value;
            }
            break;
        case 'max y':
            if (setting.innerHTML == '') {
                var label = document.createElement('label');
                label.for = 'number';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Max Y';
                setting.appendChild(label);

                var number = document.createElement('input');
                number.id = 'number';
                number.type = 'number';
                number.step = '1';
                setting.appendChild(number);

                number.addEventListener(
                    'change',
                    function() {
                        cmd.Metadata[param] = number.value;
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.childNodes[1] === document.activeElement) {
                break;
            }
            if (setting.childNodes[1].value != value) {
                setting.childNodes[1].value = value;
            }
            break;
        case 'reset':
            if (setting.innerHTML == '') {
                var button = document.createElement('button')
                button.innerHTML = 'Reset';
                setting.appendChild(button);

                button.addEventListener(
                    'click',
                    function() {
                        cmd.Metadata[param] = '';
                        ws.send(JSON.stringify(cmd));
                    }
                )
            }
            break;
        case 'nbins x':
            if (setting.innerHTML == '') {
                var label = document.createElement('label');
                label.for = 'number';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Num. bins x';
                setting.appendChild(label);

                var number = document.createElement('input');
                number.id = 'number';
                number.type = 'number';
                number.min = '1';
                number.step = '1';
                setting.appendChild(number);

                number.addEventListener(
                    'change',
                    function() {
                        cmd.Metadata[param] = number.value;
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.childNodes[1] === document.activeElement) {
                break;
            }
            if (setting.childNodes[1].value != value) {
                setting.childNodes[1].value = value;
            }
            break;
        case 'nbins y':
            if (setting.innerHTML == '') {
                var label = document.createElement('label');
                label.for = 'number';
                label.setAttribute('class', 'setting');
                label.innerHTML = 'Num. bins y';
                setting.appendChild(label);

                var number = document.createElement('input');
                number.id = 'number';
                number.type = 'number';
                number.min = '1';
                number.step = '1';
                setting.appendChild(number);

                number.addEventListener(
                    'change',
                    function() {
                        cmd.Metadata[param] = number.value;
                        ws.send(JSON.stringify(cmd));
                    }
                );
            }
            if (setting.childNodes[1] === document.activeElement) {
                break;
            }
            if (setting.childNodes[1].value != value) {
                setting.childNodes[1].value = value;
            }
            break;
        default:
    }
}

function getTime() {
    var date = new Date;
    return date.getUTCHours() + ':' + date.getUTCMinutes() + ':' + date.getUTCSeconds()
}
