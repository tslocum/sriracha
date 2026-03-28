var mouseX = 0;
var mouseY = 0;
var haveFocus = false;
var highlightedPost = null;
var blinkTitle = false;
var originalTitle = "";
var viewThreadID = 0;
var newRepliesCount = 0;
var newReplyID = 0;
var postCache = {};

// verbose is a flag which enables verbose logging.
const verbose = false;

const touchScreen = 'ontouchstart' in window ||
    (window.DocumentTouch && document instanceof window.DocumentTouch) ||
    navigator.maxTouchPoints > 0 ||
    window.navigator.msMaxTouchPoints > 0;

function updateTitle() {
    if (originalTitle == "") {
        originalTitle = document.title;
    }

    if (!blinkTitle) {
        document.title = originalTitle;
        return;
    }

    if (document.title == originalTitle) {
        document.title = newRepliesCount + " new";
        if (newRepliesCount == 1) {
            document.title += " reply";
        } else {
            document.title += " replies";
        }
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
    var deleted = false;
    return fetch(url, {cache: 'no-cache'}).then(function(response) {
        if (response.status == 404) {
            deleted = true;
            return;
        } else if (!response.ok) {
            return;
        }
        return response.text();
    }).then(function(body) {
        if (verbose) {
            console.log('fetched ' + url);
        }
        if (body == "") {
            return;
        }

        var container;
        var posts = [];
        var forum = document.getElementsByClassName('forumpost').length > 0;
        if (forum) {
            if (verbose) {
                console.log('detected forum board');
            }
            threadElements = document.getElementsByClassName('thread');
            if (threadElements.length > 0) {
                container = threadElements[0];
            }
        }
        if (!container) {
            var ops = document.getElementsByClassName('op');
            if (ops.length > 0) {
                posts.push(ops[0]);
            }
            for (const post of document.getElementsByClassName('reply')) {
                posts.push(post);
            }
            if (posts.length > 0) {
                if (verbose) {
                    console.log('setting container via elements with class op or reply');
                }
                container = posts[posts.length - 1].parentElement.parentElement.parentElement.parentElement;
            }
            if (!container) {
                posts = document.getElementsByClassName('forumpost');
                if (posts.length > 0) {
                    forum = true;
                    if (verbose) {
                        console.log('setting container via elements with class forumpost');
                    }
                    container = posts[posts.length - 1].parentElement.parentElement.parentElement;
                } else {
                    var ops = document.getElementsByClassName('op');
                    if (ops.length > 0) {
                        if (verbose) {
                            console.log('setting container via elements with class op');
                        }
                        container = ops[0].parentElement;
                    }
                }
            }
            if (!container) {
                if (append) {
                    console.log('warning: fetched ' + url + ' but could not find thread container, falling back to appending replies to document body. as a result, auto-refreshed replies will have style issues.');
                }
                container = document.body;
            }
        }

        var doc = (new DOMParser).parseFromString(body, 'text/html');
        var posts = doc.getElementsByClassName("post");
        var newPosts = [];
        for (const post of posts) {
            if (post.id != "" && !document.getElementById(post.id)) {
                if (verbose) {
                    console.log('found new post', post);
                }
                newPosts.push(post);
            }
        }
        if (newPosts.length == 0) {
            return;
        }
        for (const post of newPosts) {
            postCache[post.id] = post;
            if (append) {
                if (forum) { // Append forum reply.
                    var tr = doc.createElement('tr');
                    tr.appendChild(post);

                    container.appendChild(tr);
                } else { // Append imageboard reply.
                    var table = doc.createElement('table');
                    var tbody = doc.createElement('tbody');
                    table.appendChild(tbody);
                    var tr = doc.createElement('tr');
                    tbody.appendChild(tr);

                    var td = doc.createElement('td');
                    td.classList.add('doubledash');

                    tr.appendChild(td);
                    tr.appendChild(post);

                    container.appendChild(table);
                }
            }
        }
        setPostAttributes(container);
        if (append && !haveFocus) {
            if (newReplyID == 0) {
                newReplyID = newPosts[0].id;
            }
            newRepliesCount += newPosts.length;
            if (!blinkTitle) {
                blinkTitle = true;
                updateTitle();
            }
        }
    }).catch(function(err) {
        console.log('Failed to fetch thread (' + url + '):', err);
    }).finally(function() {
        if (deleted) {
            if (append) {
                var postdeleted = document.getElementById("postdeleted");
                if (postdeleted) {
                    postdeleted.style.display = "table-cell";
                }
            }
            return;
        }
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
        if (!srcFile || !thumbFile || !srcFile.dataset) {
            return true;
        }

        var expandHTML = srcFile.dataset.expand;
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
    var doc = document.documentElement;
    var vw = Math.max(doc.clientWidth || 0, window.innerWidth || 0);
    var vh = Math.max(doc.clientHeight || 0, window.innerHeight || 0);
    var vl = (window.pageXOffset || doc.scrollLeft) - (doc.clientLeft || 0);
    var vt = (window.pageYOffset || doc.scrollTop)  - (doc.clientTop || 0);
    var rect = el.getBoundingClientRect();

    var preview = document.getElementById('ref' + el.getAttribute('refID'));
    if (!preview) {
        closePostPreview();
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
                var thread = postCache['thread' + threadID];
                var fetching = postCache['fetch' + threadID];
                if (thread || fetching) {
                    if (verbose) {
                        if (thread) {
                            console.log('thread page already cached');
                        }
                        if (fetching) {
                            console.log('fetch already in progress');
                        }
                    }

                    var msg = "Loading...";
                    var fetched = postCache['fetched' + threadID];
                    if (fetched) {
                        msg = '<span style="color: red;font-weight: bold;">Post deleted.</span>';
                    }

                    var preview = document.createElement('div');
                    preview.id = 'ref' + el.getAttribute('refID');
                    preview.setAttribute('refID', el.getAttribute('refID'));
                    preview.className = 'hoverpost';
                    preview.innerHTML = msg;
                    preview.style.left = vl + rect.left + 'px';
                    preview.style.top = vt + rect.bottom + 'px';
                    document.body.append(preview);
                    return;
                }
                if (fetching) {
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

                var preview = document.createElement('div');
                preview.id = 'ref' + el.getAttribute('refID');
                preview.setAttribute('refID', el.getAttribute('refID'));
                preview.className = 'hoverpost';
                preview.innerHTML = 'Loading...';
                preview.style.left = vl + rect.left + 'px';
                preview.style.top = vt + rect.bottom + 'px';
                document.body.append(preview);

                postCache['thread' + threadID] = true;
                postCache['fetch' + threadID] = true;
                fetchPosts(url, false).then(function() {
                    postCache['fetched' + threadID] = true;
                    post = postCache[postID];
                    if (post && post.innerHTML) {
                        // Preview fetched post.
                        closePostPreview();
                        previewPost(el);
                    } else {
                        preview.innerHTML = '<span style="color: red;font-weight: bold;">Post deleted.</span>';
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

    var pr = preview.getBoundingClientRect();
    var pw = pr.right - pr.left;
    var ph = pr.bottom - pr.top;

    var px = vl+rect.left;
    var py = vt+rect.bottom;
    var offset = false;
    if (py > vt + vh - ph) {
        py = vt + vh - ph;
        offset = true;
    }
    if (py < vt ) {
        py = vt;
        offset = true;
    }
    if (offset) {
        px += rect.right-rect.left;
    }
    if (px > vl + vw - pw) {
        px = vl + vw - pw;
    }
    if (px < vl) {
        px = vl;
    }
    preview.style.left = px + 'px';
    preview.style.top = py + 'px';
    if (touchScreen && px + pw < rect.right) {
        preview.style.right = (vl + vw - rect.right) + 'px';
    }
}

function closePostPreview() {
    var previews = document.getElementsByClassName('hoverpost');
    for (const preview of previews) {
        preview.remove();
    }
}

function setPostAttributes(element) {
    var base_url = window.location.pathname;
    var resIndex = base_url.indexOf('/res/');
    if (resIndex != -1) {
        base_url = base_url.substring(0, resIndex) + '/';
    }
    element.querySelectorAll('a').forEach((el, i) => {
        var postID = 0;
        var m = null;
        if (el.getAttribute('href')) {
            m = el.getAttribute('href').match(/.*\/([0-9]+)\.html#([0-9]+)/i);
            if (m) {
                postID = m[2];
                if (m[1] == viewThreadID) {
                    el.setAttribute("href", "#" + postID);
                }
            }
        }
        if (postID == 0 && el.getAttribute('href')) {
            m = el.getAttribute('href').match(/\#([0-9]+)/i);
            if (m) {
                postID = m[1];
            }
        }
        if (postID == 0) {
            return;
        }

        if (el.innerHTML == 'No.') {
            if (element != document) {
                element.setAttribute('postID', postID);
                element.setAttribute('postLink', el.getAttribute('href'))
                element.classList.add('post');
            }
        } else if (el.getAttribute('refID') == undefined) {
            var m2 = el.innerHTML.match(/^\&gt\;\&gt\;[0-9]+/i);
            if (m2 == null) {
                return;
            }
            if (touchScreen) {
                el.classList.add("touchreflink")
            }
            el.setAttribute('refID', postID);
            el.addEventListener("mouseenter", function(e) {
                previewPost(el);
            });
            el.addEventListener("mouseleave", function(e) {
                closePostPreview();
            });
            var pressTime;
            el.addEventListener("touchstart", function(e) {
                e.preventDefault();
                pressTime = new Date().getTime();
                previewPost(el);
            });
            el.addEventListener("touchend", function(e) {
                e.preventDefault();
                var now = new Date().getTime();
                closePostPreview();
                if (now - pressTime < 200) {
                    el.click()
                }
            });
            el.addEventListener("touchcancel", function(e) {
                e.preventDefault();
                var now = new Date().getTime();
                closePostPreview();
                if (now - pressTime < 200) {
                    el.click()
                }
            });
            if (touchScreen) {
                el.addEventListener("contextmenu", function(e) {
                    e.preventDefault();
                });
            }
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
    if (newReplyID != 0) {
        window.location.hash = newReplyID;
        newReplyID = 0;
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

    var result = window.location.pathname.match(/.*\/res\/([0-9]+)\.html$/);
    if (result && result.length == 2) {
        viewThreadID = result[1];
    }

    setPostAttributes(document);

    if (typeof autoRefreshDelay === 'undefined' || viewThreadID == 0) {
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
