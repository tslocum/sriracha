var refreshPaused = false;
var refreshTimeout = {};
var enableNotifications = false;
var currentNotification = null;
var setUnloadHandler = false;
var haveFocus = false;
var blinkTitle = false;
var originalTitle = "";
var viewThreadID = 0;
var newRepliesCount = 0;
var newReplyID = 0;
var postCache = {};

var threadStatusNormal = 0;
var threadStatusDelayed = 1;
var threadStatusPaused = 2;
var threadStatusDeleted = 3;

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

function setStatusIndicator(status) {
    var showNormal = false;
    var showDelayed = false;
    var showPaused = false;
    var showDeleted = false;
    if (status == threadStatusNormal) {
        showNormal = true;
    } else if (status == threadStatusDelayed) {
        showDelayed = true;
    } else if (status == threadStatusPaused) {
        showPaused = true;
    } else {
        showDeleted = true;
    }
    var threadStatus = document.getElementById("threadstatus");
    if (threadStatus) {
        threadStatus.style.display = "inline-block";
    }
    var statusNormal = document.getElementById("threadstatusnormal");
    if (statusNormal) {
        if (showNormal) {
            statusNormal.style.display = "inline-block";
        } else {
            statusNormal.style.display = "none";
        }
    }
    var statusDelayed = document.getElementById("threadstatusdelayed");
    if (statusDelayed) {
        if (showDelayed) {
            statusDelayed.style.display = "inline-block";
        } else {
            statusDelayed.style.display = "none";
        }
    }
    var statusPaused = document.getElementById("threadstatuspaused");
    if (statusPaused) {
        if (showPaused) {
            statusPaused.style.display = "inline-block";
        } else {
            statusPaused.style.display = "none";
        }
    }
    var statusDeleted = document.getElementById("threadstatusdeleted");
    if (statusDeleted) {
        if (showDeleted) {
            statusDeleted.style.display = "table-cell";
            var notificationsOff = document.getElementById("notificationsoff");
            if (notificationsOff) {
                notificationsOff.remove();
            }
            var notificationsOn = document.getElementById("notificationson");
            if (notificationsOn) {
                notificationsOn.remove();
            }
        } else {
            statusDeleted.style.display = "none";
        }
    }
}

function toggleAutoRefresh() {
    refreshPaused = !refreshPaused;

    if (refreshPaused) {
        clearTimeout(refreshTimeout);
        setStatusIndicator(threadStatusPaused);
    } else {
        setStatusIndicator(threadStatusNormal);
        fetchPosts(window.location.href, true);
    }
    return false;
}

function toggleNotifications() {
    var requestPermission = false;
    if (Notification.permission === "denied") {
        enableNotifications = false;
    } else {
        enableNotifications = !enableNotifications;
        if (enableNotifications && Notification.permission !== "granted") {
            enableNotifications = false;
            requestPermission = true;
        }
    }

    var showDisable = false;
    var showEnable = false;
    if (enableNotifications) {
        showEnable = true;
    } else {
        showDisable = true;
    }
    var notificationsOff = document.getElementById("notificationsoff");
    if (notificationsOff) {
        if (showDisable) {
            notificationsOff.style.display = "inline-block";
        } else {
            notificationsOff.style.display = "none";
        }
    }
    var notificationsOn = document.getElementById("notificationson");
    if (notificationsOn) {
        if (showEnable) {
            notificationsOn.style.display = "inline-block";
        } else {
            notificationsOn.style.display = "none";
        }
    }

    if (requestPermission) {
        Notification.requestPermission().then((permission) => {
            if (permission === "granted") {
                toggleNotifications();
            }
        });
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
        } else if (append) {
            setStatusIndicator(threadStatusNormal);
        }

        var container;
        var posts = [];
        var forum = document.getElementsByClassName('forumpost').length > 0;
        if (forum) {
            if (verbose) {
                console.log('detected forum board');
            }
        }
        threadElements = document.getElementsByClassName('thread');
        if (threadElements.length > 0) {
            container = threadElements[0];
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
                if (enableNotifications && Notification.permission === "granted") {
                    if (currentNotification) {
                        currentNotification.close();
                    }
                    const options = {
                        body: window.location.pathname,
                        renotify: true,
                        requireInteraction: false,
                    };
                    var title = newPosts.length + " new post"
                    if (newPosts.length != 1) {
                        title += "s";
                    }
                    currentNotification = new Notification(title, options);
                    if (!setUnloadHandler) {
                        window.addEventListener('unload', function(e) {
                            currentNotification.close();
                        });
                        setUnloadHandler = true;
                    }
                }
            }
            newRepliesCount += newPosts.length;
            if (!blinkTitle) {
                blinkTitle = true;
                updateTitle();
            }
        }
    }).catch(function(err) {
        console.log('Failed to fetch thread (' + url + '):', err);
        if (append) {
            setStatusIndicator(threadStatusDelayed);
        }
    }).finally(function() {
        if (deleted) {
            if (append) {
                setStatusIndicator(threadStatusDeleted);
            }
            return;
        }
        if (append) {
           refreshTimeout = setTimeout(function() { fetchPosts(window.location.href, true); }, autoRefreshDelay*1000);
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
    if (touchScreen) {
        const thumbOffset = 50;
        rect = new DOMRect(rect.x, rect.y+thumbOffset, rect.width, rect.height)
    }

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
                        // Verify loading message is still visible. If it was hidden, the reflink is no longer being previewed.
                        if (document.getElementById('ref' + postID) === null) {
                            return;
                        }
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
                el.addEventListener("dblclick", function(e) {
                    e.preventDefault();
                    window.location.href = el.href;
                });
                el.addEventListener("click", function(e) {
                    e.preventDefault();
                });
                el.classList.add("touchreflink")
            }
            el.setAttribute('refID', postID);
            el.addEventListener("mouseenter", function(e) {
                previewPost(el);
            });
            el.addEventListener("mouseleave", function(e) {
                closePostPreview();
            });
            el.addEventListener("touchstart", function(e) {
                previewPost(el);
            });
            el.addEventListener("touchend", function(e) {
                closePostPreview();
            });
            el.addEventListener("touchcancel", function(e) {
                closePostPreview();
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
    var styleName = style;
    if (style.endsWith('/flex')) {
        styleName = style.substring(0, style.length-5);

        var flexStyle = document.createElement('style');
        flexStyle.id = 'flexStyle';
        flexStyle.textContent = '.thread { display: flex; flex-wrap: wrap; }';
        document.head.appendChild(flexStyle);
    } else {
        var flexStyle = document.getElementById('flexStyle');
        if (flexStyle) {
            flexStyle.remove();
        }
    }
    document.cookie = 'sriracha_style=' + style + '; expires=Tue, 19 Jan 2038 03:14:07 UTC; path=/; SameSite=Strict';

    var stylesheet = document.getElementById('mainStylesheet');
    if (!stylesheet) {
        return;
    }
    stylesheet.href = '/static/css/' + styleName + '.css';
}

function loadStyle() {
    var style = getCookie("sriracha_style");
    if (style) {
        setStyle(style);
    }
}

function formatFileSize(size) {
    var i = size == 0 ? 0 : Math.floor(Math.log(size) / Math.log(1000));
    return +((size / Math.pow(1000, i)).toFixed(2)) * 1 + ' ' + ['B', 'KB', 'MB', 'GB', 'TB'][i];
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
    if (currentNotification) {
        currentNotification.close();
    }
}

function onBlur(e) {
    newRepliesCount = 0;
    haveFocus = false;
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

function onPaste(e) {
    var fileInputs = document.getElementsByName("file");
    if (!fileInputs || fileInputs.length == 0) {
        return;
    }
    var items = (event.clipboardData  || event.originalEvent.clipboardData).items;
    var dt = new DataTransfer();
    for (const item of items) {
        if (item.kind == 'file') {
            dt.items.add(item.getAsFile());
        }
    }
    if (dt.items.length == 0) {
        return;
    }
    var msg = "Paste file";
    if (dt.items.length != 1) {
        msg += "s";
    }
    msg += "?";
    if (!confirm(msg)) {
        return;
    }
    fileInputs[0].files = dt.files;
}

function onSubmit(e) {
    var maxSizeElement = document.getElementById("maxFileSize");
    if (!maxSizeElement) {
        return true;
    }
    var maxFileSize = parseInt(maxSizeElement.value);
    if (maxFileSize <= 0) {
        return true;
    }
    var maxCountElement = document.getElementById("maxFileCount");
    if (!maxCountElement) {
        return true;
    }
    var maxFileCount = parseInt(maxCountElement.value);
    if (maxFileCount <= 0) {
        return true;
    }
    var fileInputs = document.getElementsByName("file");
    if (!fileInputs || fileInputs.length == 0) {
        return true;
    } else if (fileInputs[0].files && fileInputs[0].files.length > maxFileCount) {
        e.preventDefault();
        var info = maxFileCount + " file";
        if (maxFileCount != 1) {
            info += "s";
        }
        alert("Error: Only " + info + " may be uploaded at once.");
        return false;
    }
    for (const file of fileInputs[0].files) {
        if (file.size > maxFileSize) {
            e.preventDefault();
            alert("Error: " + file.name + " (" + formatFileSize(file.size) + ") is too large. Please upload a file " + formatFileSize(maxFileSize) + " or smaller.");
            return false;
        }
    }
    return true;
}

function onDOMContentLoaded(e) {
    // Parse thread ID.
    var result = window.location.pathname.match(/.*\/res\/([0-9]+)\.html$/);
    if (result && result.length == 2) {
        viewThreadID = result[1];
    }

    // Display thread status icons.
    if (autoRefreshDelay && autoRefreshDelay > 0 && viewThreadID > 0) {
        if ("Notification" in window) {
            var permission = Notification.permission;
            if (permission !== "denied") {
                if (permission === "granted") {
                    enableNotifications = true;
                    var notificationsOn = document.getElementById("notificationson");
                    if (notificationsOn) {
                        notificationsOn.style.display = "inline-block";
                    }
                } else {
                    var notificationsOff = document.getElementById("notificationsoff");
                    if (notificationsOff) {
                        notificationsOff.style.display = "inline-block";
                    }
                }
            }
        }
        setStatusIndicator(threadStatusNormal);
        refreshTimeout = setTimeout(function() { fetchPosts(window.location.href, true); }, autoRefreshDelay*1000);
    }

    // Handle style change.
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

    // Quote post.
    if (window.location.hash) {
        var match = window.location.hash.match(/^#q[0-9]+$/i);
        if (match !== null) {
            var quotePostID = match[0].substr(2);
            if (quotePostID) {
                quotePost(quotePostID);
            }
        }
    }

    // Set post attributes and handle reflink hover previews.
    setPostAttributes(document);

    // Validate posts before they are submitted.
    var postForm = document.getElementById("postform");
    if (postForm) {
        postForm.addEventListener("submit", onSubmit)
    }
}

document.addEventListener("dragover", onDragOver);
window.addEventListener("dragover", onDragOver);

document.addEventListener("drop", onDrop);
window.addEventListener("drop", onDrop);

window.addEventListener("focus", onFocus);
window.addEventListener("blur", onBlur);

document.addEventListener("paste", onPaste);

window.addEventListener("DOMContentLoaded", onDOMContentLoaded);
