function liftBan(id) {
    var reason = prompt('Reason for lifting ban #' + id + ':');
    if (reason === null) {
        return false;
    }
    document.getElementById('reason' + id).value = reason;
    return true;
}

function setAllBoards(enable) {
    var boards = document.getElementsByName('boards');
    for (var i = 0; i < boards.length; i++) {
        boards[i].checked = enable;
    }
    return false;
}

function setAllUploads(enable) {
    var uploads = document.getElementsByName('uploads');
    for (var i = 0; i < uploads.length; i++) {
        uploads[i].checked = enable;
    }
    return false;
}
