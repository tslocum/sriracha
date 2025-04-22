function expandFile(e, id) {
    if (e == undefined || e.which == undefined || e.which == 1) {
        var srcFile = document.querySelector("#file" + id);
        var thumbFile = document.querySelector("#thumbfile" + id);
        if (!srcFile || !thumbFile) {
            return;
        }

        if (thumbFile.getAttribute('expanded') != 'true') {
            thumbFile.setAttribute('expanded', 'true');

            srcFile.style.display = "none";
            srcFile.innerHTML = decodeURIComponent(document.querySelector("#expand" + id).innerHTML);

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
