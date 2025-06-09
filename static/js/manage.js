function liftBan(id) {
    var reason = prompt('Reason for lifting ban #' + id + ':');
    if (reason === null) {
        return false;
    }
    document.getElementById('reason' + id).value = reason;
    return true;
}
