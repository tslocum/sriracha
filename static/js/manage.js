function liftBan(id) {
    var reason = prompt('Reason for lifting ban #' + id + ':');
    if (reason === null) {
        return false;
    }
    document.getElementById('reason' + id).value = reason;
    return true;
}

function setAllUploads(enabled) {
    var uploads = document.getElementsByName('uploads');
    for (var i = 0; i < uploads.length; i++) {
        uploads[i].checked = enabled;
    }
    return false;
}
