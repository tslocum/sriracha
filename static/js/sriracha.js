var mouseX = 0;
var mouseY = 0;
var haveFocus = false;
var originalTitle = "";
var newRepliesCount = 0;

function updateTitle() {
    if (originalTitle == "") {
        originalTitle = document.title;
    }

    if (!blinkTitle) {
        document.title = originalTitle;
        return;
    }

    if (document.title == originalTitle) {
        document.title = "(" + newRepliesCount + " new)";
    } else {
        document.title = originalTitle;
    }

    setTimeout(updateTitle, 2000);
}

function refreshReplies() {
    fetch(window.location.href).then(function(resp) {
        return resp.text();
    }).then(function(body) {
        var container;
        var replies = document.getElementsByClassName('reply');
        if (replies.length > 0) {
            container = replies[replies.length - 1].parentElement.parentElement.parentElement.parentElement;
        } else {
            var ops = document.getElementsByClassName('op');
            if (ops.length > 0) {
                container = ops[0].parentElement;
            }
        }
        if (!container) {
            return;
        }

        var doc = (new DOMParser).parseFromString(body, 'text/html');
        var replies = doc.getElementsByClassName('reply');
        var newReplies = [];
        for (const reply of replies) {
            if (reply.id != "" && !document.getElementById(reply.id)) {
                newReplies.push(reply);
            }
        }
        if (newReplies.length == 0) {
            return;
        }
        for (const reply of newReplies) {
            var table = doc.createElement('table');
            var tbody = doc.createElement('tbody');
            table.appendChild(tbody);
            var tr = doc.createElement('tr');
            tbody.appendChild(tr);

            var td = doc.createElement('td');
            td.classList.add('doubledash');
            td.innerHTML = "&#0168;";

            tr.appendChild(td);
            tr.appendChild(reply);

            container.appendChild(table);
        }
        setPostAttributes(container);
        if (!haveFocus) {
            newRepliesCount += newReplies.length;
            if (!blinkTitle) {
                blinkTitle = true;
                updateTitle();
            }
        }
    }).catch(function(err) {
        console.log('Failed to refresh thread:', err);
    }).finally(function() {
        setTimeout(refreshReplies, autoRefreshDelay*1000);
    });
}

function quotePost(postID) {
    var message = document.getElementById("message");
    if (!message) {
        return false;
    }
    message.value = message.value + '>>' + postID + "\n";
    message.focus();
    return false;
}

function expandFile(e, id) {
    if (e == undefined || e.which == undefined || e.which == 1) {
        var srcFile = document.querySelector("#file" + id);
        var thumbFile = document.querySelector("#thumbfile" + id);
        if (!srcFile || !thumbFile) {
            return true;
        }

        var expandHTML = document.querySelector("#expand" + id).innerHTML;
        if (!expandHTML) {
            return true;
        }

        if (thumbFile.getAttribute('expanded') != 'true') {
            thumbFile.setAttribute('expanded', 'true');

            srcFile.style.display = "none";
            srcFile.innerHTML = decodeURIComponent(expandHTML);

            setTimeout(function (id) {
                return function () {
                    thumbFile.style.display = "none";
                    srcFile.style.display = "block";
                }
            }(id), 100);
        } else {
            srcFile.style.display = "none";
            srcFile.innerHTML = "";

            thumbFile.style.display = "block";
            thumbFile.setAttribute('expanded', 'false');
        }

        return false;
    }

    return true;
}

function setPostAttributes(element) {
    var base_url = window.location.pathname;
    var resIndex = base_url.indexOf('/res/');
    if (resIndex != -1) {
        base_url = base_url.substring(0, resIndex) + '/';
    }
    element.querySelectorAll('a').forEach((el, i) => {
        var m = null;
        if (el.getAttribute('href')) {
            m = el.getAttribute('href').match(/.*\/[0-9]+?#([0-9]+)/i);
        }
        if (m == null && el.getAttribute('href')) {
            m = el.getAttribute('href').match(/\#([0-9]+)/i);
        }
        if (m == null) {
            return;
        }

        if (el.innerHTML == 'No.') {
            if (element != document) {
                element.setAttribute('postID', m[1]);
                element.setAttribute('postLink', el.getAttribute('href'))
                element.classList.add('post');
            }
        } else if (el.getAttribute('refID') == undefined) {
            var m2 = el.innerHTML.match(/^\&gt\;\&gt\;[0-9]+/i);
            if (m2 == null) {
                return;
            }
            el.setAttribute('refID', m[1]);
            el.addEventListener("mouseenter", function(e) {
                var preview = document.getElementById('ref' + el.getAttribute('refID'));
                if (!preview) {
                    var refpost = document.getElementById('post' + el.getAttribute('refID'));
                    if (refpost && refpost.innerHTML && refpost.innerHTML != undefined) {
                        var preview = document.createElement('div');
                        preview.id = 'ref' + el.getAttribute('refID');
                        preview.style.position = 'absolute';
                        preview.style.textAlign = 'left';
                        preview.setAttribute('refID', el.getAttribute('refID'));
                        preview.className = 'hoverpost';
                        preview.innerHTML = refpost.innerHTML;
                        if (refpost.tagName.toLowerCase() == 'td') {
                            preview.classList.add('reply');
                        }
                    } else {
                        return;
                    }
                    document.body.append(preview);
                }
                preview.style.left =  mouseX+14 + 'px';
                preview.style.top = mouseY+7 + 'px';
            });
            el.addEventListener("mouseleave", function(e) {
                document.getElementById('ref' + el.getAttribute('refID')).remove();
            });
        }
    });
}

function onFocus(e) {
    newRepliesCount = 0;
    blinkTitle = false;
    haveFocus = true;
    if (originalTitle != "") {
        document.title = originalTitle;
    }
}

function onBlur(e) {
    newRepliesCount = 0;
    haveFocus = false;
}

function onMouseMove(e) {
    mouseX = e.pageX;
    mouseY = e.pageY;
}

function onLoad(e) {
    if (window.location.hash) {
        var match = window.location.hash.match(/^#q[0-9]+$/i);
        if (match !== null) {
            var quotePostID = match[0].substr(2);
            if (quotePostID) {
                quotePost(quotePostID);
            }
        }
    }

    setPostAttributes(document);

    if (typeof autoRefreshDelay === 'undefined') {
        return;
    }

    var result = window.location.pathname.match(/.*\/res\/([0-9]+)\.html$/);
    if (!result || result.length < 2) {
        return;
    }

    setTimeout(refreshReplies, autoRefreshDelay*1000);
}

window.addEventListener("focus", onFocus);
window.addEventListener("blur", onBlur);
window.addEventListener("mousemove", onMouseMove);
window.addEventListener("load", onLoad);
