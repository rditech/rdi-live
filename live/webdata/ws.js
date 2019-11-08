// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

function genClientWsUri() {
    var loc = window.location,
        uri;
    if (loc.protocol === 'https:') {
        uri = 'wss:';
    } else {
        uri = 'ws:';
    }
    uri += '//' + loc.host;
    uri += loc.pathname + 'client';
    return uri;
}

var loc = genClientWsUri();
var ws = new WebSocket(loc);

wsonopen = function() {
    connectionicon.className = 'active';
    connectionicon.alt = 'connected to server (click to disconnect)';
    connectionicon.title = connectionicon.alt;

    var cmd = {
        Command: 'list streams'
    }
    ws.send(JSON.stringify(cmd));

    var cmd = {
        Command: 'get nickname'
    }
    ws.send(JSON.stringify(cmd));

    var refresh = function() {
        if (ws.readyState === WebSocket.OPEN) {
            refreshDataResources();
            setTimeout(refresh, 10000);
        }
    }
    refresh();
}

wsonclose = function() {
    connectionicon.className = 'inactive';
    connectionicon.alt = 'not connected to server (refresh to reconnect)';
    connectionicon.title = connectionicon.alt;
    cpuusagemeter.style.width = '0%';
    cpuusagetext.innerHTML = 'CPU Usage';
    memusagemeter.style.width = '0%';
    memusagetext.innerHTML = 'Mem Usage';
}

wsonmessage = function(event) {
    var msg = JSON.parse(event.data);
    switch (msg.Type) {
        case 'stream announce':
            handleStreamAnnounce(msg);
            break;
        case 'stream close':
            handleStreamClose(msg);
            break;
        case 'show frame':
            handleShowFrame(msg);
            break;
        case 'show close':
            handleShowClose(msg);
            break;
        case 'stream sub':
            handleSubscribe(msg);
            break;
        case 'stream unsub':
            handleUnsubscribe(msg);
            break;
        case 'stream status':
            handleStreamStatus(msg);
            break;
        case 'source announce':
            handleSourceAnnounce(msg);
            break;
        case 'nickname':
            handleNickname(msg);
            break;
        case 'system status':
            handleSystemStatus(msg);
            break;
        case 'run list':
            handleRunList(msg);
            break;
        case 'run meta':
            handleRunMeta(msg);
            break;
        case 'player failure':
            handlePlayerFailure(msg);
            break;
        default:
            console.log('unhandled message');
            console.log(msg);
    }
}

ws.onopen = wsonopen;
ws.onclose = wsonclose;
ws.onmessage = wsonmessage;

connectionicon.addEventListener('click',
    function() {
        if (ws.readyState === WebSocket.OPEN) {
            ws.close(1000)
        }
    }
);

function handleNickname(msg) {
    var name = msg.Metadata.name;
    if (name === 'nobody') {
        return;
    }

    nickname.innerHTML = 'logged in as ' + name + ' (';
    var logoutLink = document.createElement('a');
    logoutLink.href = '/logout';
    logoutLink.innerHTML = 'logout';
    nickname.appendChild(logoutLink);
    nickname.innerHTML += ')';
}

function handleSystemStatus(msg) {
    if ('usage' in msg.Metadata) {
        var usage = (parseFloat(msg.Metadata.usage) * 100).toFixed(1) + '%';
        cpuusagetext.innerHTML = 'CPU Usage: ' + usage;
        setMeterWidth(cpuusagemeter, usage);
    }

    if ('mem alloc' in msg.Metadata && 'mem sys' in msg.Metadata) {
        var alloc = parseFloat(msg.Metadata['mem alloc']);
        var sys = parseFloat(msg.Metadata['mem sys']);
        memusagetext.innerHTML = 'Mem Usage: ' + alloc.toFixed(0) + '/' + sys.toFixed(0) + ' MB';
        var usage = (alloc / sys * 100).toFixed(1) + '%';
        setMeterWidth(memusagemeter, usage);
    }
}
