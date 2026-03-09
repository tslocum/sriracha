var mouseX = 0;
var mouseY = 0;
var haveFocus = false;
var highlightedPost = null;
var blinkTitle = false;
var originalTitle = "";
var newRepliesCount = 0;
var postCache = {};

// verbose is a flag which enables verbose logging.
const verbose = false;

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

function unsubscribeAll() {
    var subs = document.querySelectorAll("[name^=sub]");
    if (!subs) {
        return;
    }
    for (var i = 0; i < subs.length; i++) {
        var tag = subs[i].tagName.toLowerCase();
        if (tag == "select") {
            subs[i].value = "1";
        } else if (tag == "input") {
            subs[i].checked = true;
        }
    }
    return false;
}

function fetchPosts(url, append) {
    if (verbose) {
        console.log('fetching ' + url + '...');
    }
    return fetch(url).then(function(resp) {
        return resp.text();
    }).then(function(body) {
        if (verbose) {
            console.log('fetched ' + url);
        }
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
            console.log('fetched ' + url + ' but could not find container');
            console.log(body);
            return;
        }

        var doc = (new DOMParser).parseFromString(body, 'text/html');
        var replies = doc.getElementsByClassName('reply');
        var newReplies = [];
        for (const reply of replies) {
            if (reply.id != "" && !document.getElementById(reply.id)) {
                if (verbose) {
                    console.log('found new reply', reply);
                }
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

            postCache[reply.id] = reply;
            if (append) {
                container.appendChild(table);
            }
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
        console.log('Failed to fetch thread (' + url + '):', err);
    }).finally(function() {
        if (append) {
            setTimeout(function() { fetchPosts(window.location.href, true); }, autoRefreshDelay*1000);
        }
    });
}

function quotePost(postID) {
    var message = document.getElementById("message");
    if (!message) {
        return false;
    }
    var details = document.getElementById("postdetails");
    if (details) {
        details.open = true;
    }
    message.value = message.value + '>>' + postID + "\n";
    message.focus();
    if (details) {
        details.scrollIntoView();
        return false;
    }
    var postform = document.getElementById("postform");
    if (postform) {
        postform.scrollIntoView();
    }
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

function previewPost(el) {
    var preview = document.getElementById('ref' + el.getAttribute('refID'));
    if (!preview) {
        var refpost = document.getElementById(el.getAttribute('refID'));
        if (!refpost || !refpost.innerHTML || refpost.innerHTML == undefined) {
            if (verbose) {
                console.log('preview missing post No.' + el.getAttribute('refID'));
            }
            var m = el.getAttribute('href').match(/([0-9]+)\.html\#([0-9]+)/i);
            if (m == null) {
                if (verbose) {
                    console.log('failed to preview post: thread URL does not match expected pattern');
                }
                return;
            }
            var threadID = m[1];
            var postID = m[2];
            if (verbose) {
                console.log('preview thread No.' + threadID + ' post No.' + postID);
            }
            var post = postCache[postID];
            if (post) {
                if (verbose) {
                    console.log('post is cached');
                }
                refpost = post;
            } else {
                // Check if thread res page is already cached.
                var thread = postCache['thread' + threadID];
                if (thread) {
                    if (verbose) {
                        console.log('thread page is cached');
                    }
                    return;
                }
                // Fetch thread res page.
                var url = el.getAttribute('href');
                var hash = url.indexOf('#');
                if (hash != -1) {
                    url = url.substring(0, hash);
                }
                if (verbose) {
                    console.log('fetching thread page ' + url + '...');
                }
                postCache['thread' + threadID] = true;
                fetchPosts(url, false).then(function() {
                    post = postCache[postID];
                    if (post && post.innerHTML) {
                        // Preview fetched post.
                        previewPost(el);
                    }
                });
                return;
            }
        }
        if (refpost && refpost.innerHTML && refpost.innerHTML != undefined) {
            var preview = document.createElement('div');
            preview.id = 'ref' + el.getAttribute('refID');
            preview.setAttribute('refID', el.getAttribute('refID'));
            preview.className = 'hoverpost';
            preview.innerHTML = refpost.innerHTML;
            postCache[el.getAttribute('refID')] = refpost;
        } else {
            // Post no longer exists.
            return;
        }
        document.body.append(preview);
    }
    var doc = document.documentElement;
    var vw = Math.max(doc.clientWidth || 0, window.innerWidth || 0);
    var vh = Math.max(doc.clientHeight || 0, window.innerHeight || 0);
    var vl = (window.pageXOffset || doc.scrollLeft) - (doc.clientLeft || 0);
    var vt = (window.pageYOffset || doc.scrollTop)  - (doc.clientTop || 0);

    var rect = el.getBoundingClientRect();
    var px = rect.right+vl+7;
    if (px + preview.offsetWidth > vw + vl) {
        px = vw + vl - preview.offsetWidth
    }
    var py = rect.top+vt+(rect.bottom-rect.top)/2;
    if (py + preview.offsetHeight > vh + vt) {
        py = vh + vt - preview.offsetHeight
    }
    preview.style.left = px + 'px';
    preview.style.top = py + 'px';
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
                previewPost(el);
            });
            el.addEventListener("mouseleave", function(e) {
                var preview = document.getElementById('ref' + el.getAttribute('refID'));
                if (preview) {
                    preview.remove();
                }
            });
        }
    });
}

function openGallery() {
    var extensions = ["bmp", "gif", "jpg", "png", "svg", "tif"];
    var lightbox = new FsLightbox();
    if (!lightbox) {
        alert("Failed to open gallery: Failed to load lightbox library.");
        return;
    }
    lightbox.props.sources = [];
    var thumbnails = document.getElementsByClassName('thumb');
    for (const thumbnail of thumbnails) {
        var parent = thumbnail.parentElement;
        if (!parent || parent.tagName.toLowerCase() != "a") {
            continue;
        }
        var href = parent.href;
        if (!href) {
            continue;
        }
        var ext = href.split('.').pop().toLowerCase();
        if (!extensions.includes(ext)) {
            continue;
        }
        lightbox.props.sources.push(href);
    }
    if (lightbox.props.sources.length == 0) {
        alert("Failed to open gallery: No expandable media.");
        return
    }
    lightbox.open();
}

function getCookie(cname) {
    var name = cname + "=";
    var decodedCookie = decodeURIComponent(document.cookie);
    var ca = decodedCookie.split(';');
    for (var i = 0; i < ca.length; i++) {
        var c = ca[i];
        while (c.charAt(0) == ' ') {
            c = c.substring(1);
        }
        if (c.indexOf(name) == 0) {
            return c.substring(name.length, c.length);
        }
    }
    return "";
}

function setStyle(style) {
    document.cookie = 'sriracha_style=' + style + '; expires=Tue, 19 Jan 2038 03:14:07 UTC; path=/; SameSite=Strict';

    var stylesheet = document.getElementById('mainStylesheet');
    if (!stylesheet) {
        return;
    }
    stylesheet.href = '/static/css/' + style + '.css';
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

function onDragOver(e) {
    const files = [...e.dataTransfer.items].filter(
        (item) => item.kind === "file",
    );
    if (files.length > 0) {
        e.preventDefault();
    }
}

function onDrop(e) {
    e.preventDefault();
    var fileInputs = document.getElementsByName("file");
    if (!fileInputs || fileInputs.length == 0) {
        return;
    }
    fileInputs[0].files = e.dataTransfer.files;
}

function onDOMContentLoaded(e) {
    var style = getCookie("sriracha_style");
    if (style) {
        setStyle(style);
    }

    var switchStyle = document.getElementById('switchStyle');
    if (switchStyle) {
        switchStyle.addEventListener("change", function(e) {
            if (this.value == "") {
                return;
            }
            setStyle(this.value);
            this.value = "";
        });
    }

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
    setTimeout(function() { fetchPosts(window.location.href, true); }, autoRefreshDelay*1000);
}

document.addEventListener("dragover", onDragOver);
window.addEventListener("dragover", onDragOver);

document.addEventListener("drop", onDrop);
window.addEventListener("drop", onDrop);

window.addEventListener("focus", onFocus);
window.addEventListener("blur", onBlur);
window.addEventListener("mousemove", onMouseMove);
window.addEventListener("DOMContentLoaded", onDOMContentLoaded);
