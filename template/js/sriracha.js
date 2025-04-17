function expandFile(e, id) {
    if (e == undefined || e.which == undefined || e.which == 1) {
        var message = "expand " + id;
        if (document.querySelector("#thumbfile" + id).getAttribute('expanded') != 'true') {
            document.querySelector("#thumbfile" + id).setAttribute('expanded', 'true');
            document.querySelector("#file" + id).style.display = "none";
            document.querySelector("#file" + id).innerHTML = decodeURIComponent(document.querySelector("#expand" + id).innerHTML);
            setTimeout(function (id) {
                return function () {
                    document.querySelector("#thumbfile" + id).style.display = "none";
                    document.querySelector("#file" + id).style.display = "block";
                }
            }(id), 100);
        } else {
            document.querySelector("#file" + id).style.display = "none";
            document.querySelector("#file" + id).innerHTML = "";
            document.querySelector("#thumbfile" + id).style.display = "block";
            document.querySelector("#thumbfile" + id).setAttribute('expanded', 'false');
        }

        return false;
    }

    return true;
}
